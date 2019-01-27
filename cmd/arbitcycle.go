package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/Reidmcc/rockfish/arbitrageur"
	"github.com/Reidmcc/rockfish/modules"
	"github.com/interstellar/kelp/plugins"
	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/config"
)

const arbitExamples = `  rockfish arbitcycle --botConf ./path/botconfig.cfg --stratConf ./path/arbitcycle.cfg`

const tradeExamples = `  kelp trade --botConf ./path/trader.cfg --strategy buysell --stratConf ./path/buysell.cfg
  kelp trade --botConf ./path/trader.cfg --strategy buysell --stratConf ./path/buysell.cfg --sim`

var tradeCmd = &cobra.Command{
	Use:     "trade",
	Short:   "Trades against the Stellar universal marketplace using the specified strategy",
	Example: tradeExamples,
}

func requiredFlag(flag string) {
	e := arbitcycleCmd.MarkFlagRequired(flag)
	if e != nil {
		panic(e)
	}
}

func hiddenFlag(flag string) {
	e := arbitcycleCmd.Flags().MarkHidden(flag)
	if e != nil {
		panic(e)
	}
}

func logPanic(l logger.Logger) {
	if r := recover(); r != nil {
		st := debug.Stack()
		l.Errorf("PANIC!! recovered to log it in the file\npanic: %v\n\n%s\n", r, string(st))
	}
}

var arbitcycleCmd = &cobra.Command{
	Use:     "arbitcycle",
	Short:   "Watches for price variations and runs favorable multi-asset trade cycles",
	Example: arbitExamples,
}

func init() {
	// short flags
	botConfigPath := arbitcycleCmd.Flags().StringP("botConf", "c", "", "(required) trading bot's basic config file path")
	stratConfigPath := arbitcycleCmd.Flags().StringP("stratConf", "f", "", "strategy config file path")
	// long-only flags
	operationalBuffer := arbitcycleCmd.Flags().Float64("operationalBuffer", 20, "buffer of native XLM to maintain beyond minimum account balance requirement")
	simMode := arbitcycleCmd.Flags().Bool("sim", false, "simulate the bot's actions without placing any trades")
	logPrefix := arbitcycleCmd.Flags().StringP("log", "l", "", "log to a file (and stdout) with this prefix for the filename")
	fixedIterations := arbitcycleCmd.Flags().Uint64("iter", 0, "only run the bot for the first N iterations (defaults value 0 runs unboundedly)")

	requiredFlag("botConf")
	hiddenFlag("operationalBuffer")
	arbitcycleCmd.Flags().SortFlags = false

	validateCliParams := func(l logger.Logger) {
		if *operationalBuffer < 0 {
			panic(fmt.Sprintf("invalid operationalBuffer argument, must be non-negative: %f", *operationalBuffer))
		}

		if *fixedIterations == 0 {
			fixedIterations = nil
			l.Info("will run unbounded iterations")
		} else {
			l.Infof("will run only %d update iterations\n", *fixedIterations)
		}
	}

	arbitcycleCmd.Run = func(ccmd *cobra.Command, args []string) {
		l := logger.MakeBasicLogger()
		var arbitConfig arbitrageur.ArbitConfig
		e := config.Read(*botConfigPath, &arbitConfig)
		utils.CheckConfigError(arbitConfig, e, *botConfigPath)
		e = arbitConfig.Init()
		if e != nil {
			logger.Fatal(l, errors.Cause(e))
		}

		utils.LogConfig(arbitConfig)

		if *stratConfigPath == "" {
			logger.Fatal(l, fmt.Errorf("arbitcycle mode needs a config file"))
		}

		var stratConfig modules.ArbitCycleConfig
		err := config.Read(*stratConfigPath, &stratConfig)
		utils.CheckConfigError(stratConfig, err, *stratConfigPath)

		if *logPrefix != "" {
			t := time.Now().Format("20060102T150405MST")
			fileName := fmt.Sprintf("%s_%s.log", *logPrefix, t)
			e = setLogFile(fileName)
			if e != nil {
				logger.Fatal(l, e)
				return
			}
			l.Infof("logging to file: %s\n", fileName)

			// we want to create a deferred recovery function here that will log panics to the log file and then exit
			defer logPanic(l)
		}

		startupMessage := "Starting Rockfish in Arbitrage Cycle mode: " + version + " [" + gitHash + "]"
		if *simMode {
			startupMessage += " (simulation mode)"
		}
		l.Info(startupMessage)

		// now that we've got the basic messages logged, validate the cli params
		validateCliParams(l)

		// only log arbitConfig file here so it can be included in the log file
		utils.LogConfig(arbitConfig)
		l.Info("")
		utils.LogConfig(stratConfig)

		// --- start initialization of objects ----
		threadTracker := multithreading.MakeThreadTracker()

		client := &horizon.Client{
			URL:  arbitConfig.HorizonURL,
			HTTP: http.DefaultClient,
		}

		dexAgent := modules.MakeDexAgent(
			client,
			arbitConfig.SourceSecretSeed,
			arbitConfig.TradingSecretSeed,
			arbitConfig.SourceAccount(),
			arbitConfig.TradingAccount(),
			utils.ParseNetwork(arbitConfig.HorizonURL),
			threadTracker,
			*operationalBuffer,
			stratConfig.MinRatio,
			stratConfig.UseBalance,
			stratConfig.StaticAmount,
			*simMode,
			l,
		)

		dexWatcher := modules.MakeDexWatcher(client, utils.ParseNetwork(arbitConfig.HorizonURL), l)

		pathFinder, e := modules.MakePathFinder(*dexWatcher, stratConfig, l)
		if e != nil {
			logger.Fatal(l, fmt.Errorf("Couldn't make Patherfinder: %s", e))
		}

		timeController := plugins.MakeIntervalTimeController(
			time.Duration(arbitConfig.TickIntervalSeconds)*time.Second,
			0,
		)

		arbitrageur := arbitrageur.MakeArbitrageur(
			*pathFinder,
			*dexWatcher,
			*dexAgent,
			timeController,
			threadTracker,
			fixedIterations,
			*simMode,
			l,
		)

		// --- end initialization of objects ---

		if stratConfig.HoldAssetCode != "XLM" {
			l.Info("validating trustlines...")
			validateTrustlines(l, client, stratConfig, arbitConfig)
			l.Info("trustlines valid")
		}

		l.Info("Starting the trader bot...")
		arbitrageur.Start()
	}
}

func validateTrustlines(l logger.Logger, client *horizon.Client, stratConfig modules.ArbitCycleConfig, arbitConfig arbitrageur.ArbitConfig) {
	account, e := client.LoadAccount(arbitConfig.TradingAccount())
	if e != nil {
		logger.Fatal(l, e)
	}

	missingTrustlines := []string{}
	if stratConfig.HoldAssetCode != "" {
		balance := utils.GetCreditBalance(account, stratConfig.HoldAssetCode, stratConfig.HoldAssetIssuer)
		if balance == nil {
			missingTrustlines = append(missingTrustlines, fmt.Sprintf("%s:%s", stratConfig.HoldAssetCode, stratConfig.HoldAssetIssuer))
		}
	}

	if len(missingTrustlines) > 0 {
		logger.Fatal(l, fmt.Errorf("error: your trading account does not have the required trustlines: %v", missingTrustlines))
	}
}

func setLogFile(fileName string) error {
	f, e := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if e != nil {
		return fmt.Errorf("failed to set log file: %s", e)
	}
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	return nil
}
