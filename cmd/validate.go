package cmd

import (
	"fmt"
	"os"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [-f path]",
	Short: "Validate smoke test config without running tests",
	Long:  "Load and validate .smokesig.yaml configuration. Reports all errors at once.",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			resolved, err := schema.LoadDefaultPath()
			if err != nil {
				return &ConfigError{Err: err}
			}
			file = resolved
		}
		out, err := runValidate(file)
		if err != nil {
			fmt.Fprint(os.Stderr, out)
			return &ConfigError{Err: err}
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "", "Config file path (default: .smokesig.yaml, falls back to .smoke.yaml)")
}

func runValidate(path string) (string, error) {
	cfg, err := schema.Load(path)
	if err != nil {
		return fmt.Sprintf("error: loading config: %v\n", err), err
	}

	if err := schema.Validate(cfg); err != nil {
		if ve, ok := err.(*schema.ValidationError); ok {
			var out string
			out += fmt.Sprintf("❌ %s: %d error(s)\n", path, len(ve.Errors))
			for _, e := range ve.Errors {
				out += fmt.Sprintf("  - %s\n", e)
			}
			return out, ve
		}
		return fmt.Sprintf("❌ %s: %v\n", path, err), err
	}

	return fmt.Sprintf("✅ %s: valid (%d tests)\n", path, len(cfg.Tests)), nil
}
