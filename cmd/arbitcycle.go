package cmd

import (
	"fmt"
	"time"

	"github.com/Reidmcc/rockfish/arbitrageur"
	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/spf13/cobra"
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
	//stratConfigPath := arbitcycleCmd.Flags().StringP("stratConf", "f", "", "strategy config file path")
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

		startupMessage := "Starting Kelp Trader: " + version + " [" + gitHash + "]"
		if *simMode {
			startupMessage += " (simulation mode)"
		}
		l.Info(startupMessage)

		// now that we've got the basic messages logged, validate the cli params
		validateCliParams(l)
	}
}
