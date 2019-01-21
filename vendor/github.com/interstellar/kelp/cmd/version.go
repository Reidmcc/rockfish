package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Version and build information",
	Run: func(ccmd *cobra.Command, args []string) {
		fmt.Printf("  version: %s\n", version)
		fmt.Printf("  git branch: %s\n", gitBranch)
		fmt.Printf("  git hash: %s\n", gitHash)
		fmt.Printf("  build date: %s\n", buildDate)
		fmt.Printf("  GOOS: %s\n", runtime.GOOS)
		fmt.Printf("  GOARCH: %s\n", runtime.GOARCH)
	},
}
