package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const banner = `  SmokeSig
  Universal Smoke Test Runner`

var rootCmd = &cobra.Command{
	Use:   "smokesig",
	Short: "Universal smoke test runner",
	Long:  banner + "\n  Run lightweight smoke tests from .smokesig.yaml",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
