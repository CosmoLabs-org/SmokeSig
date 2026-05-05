package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const banner = `
   _____ __  __  ____  _  ______
  / ____|  \/  |/ __ \| |/ / __ \
 | (___ | \  / | |  | | ' / |__) |
  \___ \| |\/| | |  | |  <|  ___/
  ____) | |  | | |__| | . \ |____
 |_____/|_|  |_|\____/|_|\_\_____|

  Universal Smoke Test Runner
`

var rootCmd = &cobra.Command{
	Use:   "smoke",
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
