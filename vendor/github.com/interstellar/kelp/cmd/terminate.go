package cmd

import (
	"log"
	"net/http"

	"github.com/interstellar/kelp/model"
	"github.com/interstellar/kelp/plugins"
	"github.com/interstellar/kelp/support/utils"
	"github.com/interstellar/kelp/terminator"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/spf13/cobra"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/config"
)

var terminateCmd = &cobra.Command{
	Hidden: true,
	Use:    "terminate",
	Short:  "Monitors a Stellar Account and terminates offers across all inactive bots",
}

func init() {
	configPath := terminateCmd.Flags().StringP("conf", "c", "./terminator.cfg", "service's basic config file path")

	terminateCmd.Run = func(ccmd *cobra.Command, args []string) {
		log.Println("Starting Terminator: " + version + " [" + gitHash + "]")

		var configFile terminator.Config
		err := config.Read(*configPath, &configFile)
		utils.CheckConfigError(configFile, err, *configPath)
		err = configFile.Init()
		if err != nil {
			log.Fatal(err)
		}
		utils.LogConfig(configFile)
		log.Println("Started Terminator for account: ", *configFile.TradingAccount)

		// --- start initialization of objects ----
		client := &horizon.Client{
			URL:  configFile.HorizonURL,
			HTTP: http.DefaultClient,
		}
		sdex := plugins.MakeSDEX(
			client,
			configFile.SourceSecretSeed,
			configFile.TradingSecretSeed,
			*configFile.SourceAccount,
			*configFile.TradingAccount,
			utils.ParseNetwork(configFile.HorizonURL),
			multithreading.MakeThreadTracker(),
			-1, // not needed here
			-1, // not needed here
			false,
			nil, // not needed here
			map[model.Asset]horizon.Asset{},
		)
		terminator := terminator.MakeTerminator(client, sdex, *configFile.TradingAccount, configFile.TickIntervalSeconds, configFile.AllowInactiveMinutes)
		// --- end initialization of objects ----

		for {
			terminator.StartService()
			log.Println("Restarting terminator service")
		}
	}
}
