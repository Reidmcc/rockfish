package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Reidmcc/rockfish/arbitrageur"
	"github.com/Reidmcc/rockfish/modules"
	"github.com/interstellar/kelp/plugins"
	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/monitoring"
	"github.com/interstellar/kelp/support/networking"
	"github.com/interstellar/kelp/support/utils"
	"github.com/interstellar/kelp/trader"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/spf13/cobra"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/config"
)

const arbitExamples = `  rockfish arbitcycle --botConf ./path/arbitrageur.cfg --stratConf ./path/arbitcycle.cfg`

var arbitcycleCmd = &cobra.Command{
	Use:     "arbitcycle",
	Short:   "Watches for price variations and runs favorable multi-asset trade cycles",
	Example: tradeExamples,
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
		utils.CheckConfigError(&arbitConfig, e, *botConfigPath)
		e = arbitConfig.Init()
		if e != nil {
			logger.Fatal(l, e)
		}

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

		startupMessage := "Starting Kelp in Arbitrage Cycle mode: " + version + " [" + gitHash + "]"
		if *simMode {
			startupMessage += " (simulation mode)"
		}
		l.Info(startupMessage)

		// now that we've got the basic messages logged, validate the cli params
		validateCliParams(l)

		// only log botConfig file here so it can be included in the log file
		utils.LogConfig(botConfig)

		client := &horizon.Client{
			URL:  botConfig.HorizonURL,
			HTTP: http.DefaultClient,
		}

		alert, e := monitoring.MakeAlert(botConfig.AlertType, botConfig.AlertAPIKey)
		if e != nil {
			l.Infof("Unable to set up monitoring for alert type '%s' with the given API key\n", botConfig.AlertType)
		}
		// --- start initialization of objects ----
		threadTracker := multithreading.MakeThreadTracker()

		multiDex := modules.MakeMultiDex(
			client,
			arbitConfig.SourceSecretSeed,
			arbitConfig.TradingSecretSeed,
			arbitConfig.SourceAccount(),
			arbitConfig.TradingAccount(),
			utils.ParseNetwork(botConfig.HorizonURL),
			threadTracker,
			*operationalBuffer,
			*simMode,
		)

		pathManager := modules.MakePathManager(*stratConfigPath, *simMode)
		strat, e := plugins.MakeStrategy(sdex, tradingPair, &assetBase, &assetQuote, *strategy, *stratConfigPath, *simMode)
		if e != nil {
			l.Info("")
			l.Errorf("%s", e)
			// we want to delete all the offers and exit here since there is something wrong with our setup
			deleteAllOffersAndExit(l, botConfig, client, sdex)
		}

		timeController := plugins.MakeIntervalTimeController(
			time.Duration(botConfig.TickIntervalSeconds)*time.Second,
			botConfig.MaxTickDelayMillis,
		)
		bot := trader.MakeBot(
			client,
			botConfig.AssetBase(),
			botConfig.AssetQuote(),
			botConfig.TradingAccount(),
			sdex,
			strat,
			timeController,
			botConfig.DeleteCyclesThreshold,
			threadTracker,
			fixedIterations,
			dataKey,
			alert,
		)
		// --- end initialization of objects ---

		l.Info("validating trustlines...")
		validateTrustlines(l, client, &botConfig)
		l.Info("trustlines valid")

		// --- start initialization of services ---
		if botConfig.MonitoringPort != 0 {
			go func() {
				e := startMonitoringServer(l, botConfig)
				if e != nil {
					l.Info("")
					l.Info("unable to start the monitoring server or problem encountered while running server:")
					l.Errorf("%s", e)
					// we want to delete all the offers and exit here because we don't want the bot to run if monitoring isn't working
					// if monitoring is desired but not working properly, we want the bot to be shut down and guarantee that there
					// aren't outstanding offers.
					deleteAllOffersAndExit(l, botConfig, client, sdex)
				}
			}()
		}

		// --- end initialization of services ---

		l.Info("Starting the trader bot...")
		bot.Start()
	}
}

func startMonitoringServer(l logger.Logger, botConfig trader.BotConfig) error {
	serverConfig := &networking.Config{
		GoogleClientID:     botConfig.GoogleClientID,
		GoogleClientSecret: botConfig.GoogleClientSecret,
		PermittedEmails:    map[string]bool{},
	}
	// Load acceptable Google emails into the map
	for _, email := range strings.Split(botConfig.AcceptableEmails, ",") {
		serverConfig.PermittedEmails[email] = true
	}

	healthMetrics, e := monitoring.MakeMetricsRecorder(map[string]interface{}{"success": true})
	if e != nil {
		return fmt.Errorf("unable to make metrics recorder for the health endpoint: %s", e)
	}

	healthEndpoint, e := monitoring.MakeMetricsEndpoint("/health", healthMetrics, networking.NoAuth)
	if e != nil {
		return fmt.Errorf("unable to make /health endpoint: %s", e)
	}
	kelpMetrics, e := monitoring.MakeMetricsRecorder(nil)
	if e != nil {
		return fmt.Errorf("unable to make metrics recorder for the /metrics endpoint: %s", e)
	}

	metricsEndpoint, e := monitoring.MakeMetricsEndpoint("/metrics", kelpMetrics, networking.GoogleAuth)
	if e != nil {
		return fmt.Errorf("unable to make /metrics endpoint: %s", e)
	}
	server, e := networking.MakeServer(serverConfig, []networking.Endpoint{healthEndpoint, metricsEndpoint})
	if e != nil {
		return fmt.Errorf("unable to initialize the metrics server: %s", e)
	}

	l.Infof("Starting monitoring server on port %d\n", botConfig.MonitoringPort)
	return server.StartServer(botConfig.MonitoringPort, botConfig.MonitoringTLSCert, botConfig.MonitoringTLSKey)
}

func validateTrustlines(l logger.Logger, client *horizon.Client, botConfig *trader.BotConfig) {
	account, e := client.LoadAccount(botConfig.TradingAccount())
	if e != nil {
		logger.Fatal(l, e)
	}

	missingTrustlines := []string{}
	if botConfig.IssuerA != "" {
		balance := utils.GetCreditBalance(account, botConfig.AssetCodeA, botConfig.IssuerA)
		if balance == nil {
			missingTrustlines = append(missingTrustlines, fmt.Sprintf("%s:%s", botConfig.AssetCodeA, botConfig.IssuerA))
		}
	}

	if botConfig.IssuerB != "" {
		balance := utils.GetCreditBalance(account, botConfig.AssetCodeB, botConfig.IssuerB)
		if balance == nil {
			missingTrustlines = append(missingTrustlines, fmt.Sprintf("%s:%s", botConfig.AssetCodeB, botConfig.IssuerB))
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
