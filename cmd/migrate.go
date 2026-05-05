package cmd

import (
	"fmt"
	"os"

	"github.com/CosmoLabs-org/SmokeSig/internal/migrate/goss"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
)

var (
	migrateOutput    string
	migrateOverwrite bool
	migrateStrict    bool
	migrateStats     bool
	migrateDistro    string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate from other test frameworks",
	Long:  "Migrate configuration files from other test frameworks to cosmo-smoke format.",
}

var gossCmd = &cobra.Command{
	Use:   "goss <input.yaml>",
	Short: "Migrate a Goss YAML file to .smokesig.yaml",
	Long: `Migrate a Goss YAML configuration to cosmo-smoke format.

Supports all Goss resource types. Core keys (process, port, command, file,
http, package, service) are mapped to native cosmo-smoke assertions. Other
keys are mapped via command fallback with TODO comments for attributes that
lack native support.

Examples:
  smoke migrate goss goss.yaml
  smoke migrate goss goss.yaml -o .smokesig.yaml
  smoke migrate goss goss.yaml --strict --stats
  smoke migrate goss goss.yaml --distro rpm -o smoke.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runMigrateGoss,
}

func init() {
	gossCmd.Flags().StringVarP(&migrateOutput, "output", "o", "", "Output .smokesig.yaml path (default: stdout)")
	gossCmd.Flags().BoolVar(&migrateOverwrite, "overwrite", false, "Overwrite output file if it exists")
	gossCmd.Flags().BoolVar(&migrateStrict, "strict", false, "Fail on any unmappable assertion")
	gossCmd.Flags().BoolVar(&migrateStats, "stats", false, "Print mapping stats to stderr")
	gossCmd.Flags().StringVar(&migrateDistro, "distro", "deb", "Linux distro for package commands: deb|rpm|apk")

	migrateCmd.AddCommand(gossCmd)
	rootCmd.AddCommand(migrateCmd)
}

func runMigrateGoss(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	// Validate distro flag
	switch migrateDistro {
	case "deb", "rpm", "apk":
		// valid
	default:
		return fmt.Errorf("unsupported distro %q: must be deb, rpm, or apk", migrateDistro)
	}

	// Read input
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputPath, err)
	}

	// Parse Goss YAML
	gf, err := goss.Parse(data)
	if err != nil {
		return fmt.Errorf("parsing goss file: %w", err)
	}

	// Translate to cosmo-smoke tests
	tests, warnings := goss.Translate(gf, goss.TranslateOptions{
		Distro: migrateDistro,
	})

	// Strict mode: fail if any warnings
	if migrateStrict && len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "Strict mode: %d unmappable assertions found\n", len(warnings))
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", w.GossKey, w.Resource, w.Message)
		}
		os.Exit(1)
	}

	// Emit output
	output, err := goss.Emit(tests, warnings, goss.EmitMeta{
		Source: inputPath,
	})
	if err != nil {
		return fmt.Errorf("generating output: %w", err)
	}

	// Validate emitted YAML parses back
	if _, err := schema.Parse([]byte(output)); err != nil {
		return fmt.Errorf("generated output is not valid .smokesig.yaml: %w", err)
	}

	// Print stats if requested
	if migrateStats {
		fmt.Fprintln(os.Stderr, goss.EmitStats(warnings))
	}

	// Write output
	if migrateOutput != "" {
		if _, err := os.Stat(migrateOutput); err == nil && !migrateOverwrite {
			return fmt.Errorf("output file %s already exists (use --overwrite)", migrateOutput)
		}
		if err := os.WriteFile(migrateOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote %s\n", migrateOutput)
	} else {
		fmt.Print(output)
	}

	return nil
}
