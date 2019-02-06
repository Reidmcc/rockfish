package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

// build flags
var version string
var buildDate string
var gitBranch string
var gitHash string

const rootShort = "Rockfish is an open-source arbitrage bot for the Stellar universal marketplace."
const rootLong = `Rockfish is an open-source arbitrage bot for the Stellar universal marketplace (https://stellar.org).

Learn more about Stellar : https://www.stellar.org
Learn more about Rockfish: https://github.com/Reidmcc/rockfish
Learn more about Kelp    : https://github.com/interstellar/kelp`
const examples = arbitExamples

// RootCmd is the main command for this repo
var RootCmd = &cobra.Command{
	Use:     "rockfish",
	Short:   rootShort,
	Long:    rootLong,
	Example: examples,
	Run: func(ccmd *cobra.Command, args []string) {
		intro := `
		_______  _______  _______  _        _______ _________ _______          
		(  ____ )(  ___  )(  ____ \| \    /\(  ____ \\__   __/(  ____ \|\     /|
		| (    )|| (   ) || (    \/|  \  / /| (    \/   ) (   | (    \/| )   ( |
		| (____)|| |   | || |      |  (_/ / | (__       | |   | (_____ | (___) |
		|     __)| |   | || |      |   _ (  |  __)      | |   (_____  )|  ___  |
		| (\ (   | |   | || |      |  ( \ \ | (         | |         ) || (   ) |
		| ) \ \__| (___) || (____/\|  /  \ \| )      ___) (___/\____) || )   ( |
		|/   \__/(_______)(_______/|_/    \/|/       \_______/\_______)|/     \|
										` + version + `	
					THANKS, KELP!

`
		fmt.Println(intro)
		e := ccmd.Help()
		if e != nil {
			log.Fatal(e)
		}
	},
}

func init() {
	validateBuild()

	RootCmd.AddCommand(arbitcycleCmd)
	RootCmd.AddCommand(versionCmd)
}

func validateBuild() {
	if version == "" || buildDate == "" || gitBranch == "" || gitHash == "" {
		fmt.Println("version information not included, please build using the build script (scripts/build.sh)")
		os.Exit(1)
	}
}
