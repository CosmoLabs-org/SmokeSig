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

func init() {
	// Prevent Cobra from printing errors itself -- we handle formatting and exit codes
	rootCmd.SilenceErrors = true
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitCodeForError(err))
	}
}
