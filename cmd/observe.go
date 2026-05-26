package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/observer"
	"github.com/spf13/cobra"
)

var (
	observeDir     string
	observeTimeout time.Duration
	observeQuiet   bool
	observeOutput  string
)

var observeCmd = &cobra.Command{
	Use:   "observe [command]",
	Short: "Observe a command and generate smoke tests from its behavior",
	Long:  "Run a command, capture its output, detect ports and file changes, then generate a .smokesig.yaml with appropriate assertions.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runObserve,
}

func init() {
	rootCmd.AddCommand(observeCmd)
	observeCmd.Flags().StringVarP(&observeDir, "dir", "d", "", "Directory to monitor for file changes")
	observeCmd.Flags().DurationVarP(&observeTimeout, "timeout", "t", 0, "Timeout for the observed command")
	observeCmd.Flags().BoolVarP(&observeQuiet, "quiet", "q", false, "Non-interactive mode (accept all, no terminal output)")
	observeCmd.Flags().StringVarP(&observeOutput, "output", "o", ".smokesig.yaml", "Output file path")
}

func runObserve(cmd *cobra.Command, args []string) error {
	command := strings.Join(args, " ")

	opts := observer.ObserveOptions{
		Command: command,
		Dir:     observeDir,
		Timeout: observeTimeout,
		Quiet:   observeQuiet,
		Output:  observeOutput,
	}

	obs, err := observer.Observe(context.Background(), opts)
	if err != nil {
		return fmt.Errorf("observe: %w", err)
	}

	yaml, err := observer.Generate(obs)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	if !observeQuiet {
		fmt.Fprintf(os.Stdout, "\n  Generated smoke tests for: %s\n", command)
		fmt.Fprintf(os.Stdout, "  Exit code: %d\n", obs.ExitCode)
		fmt.Fprintf(os.Stdout, "  Duration:  %s\n", obs.Duration.Round(time.Millisecond))
		if len(obs.Ports) > 0 {
			fmt.Fprintf(os.Stdout, "  Ports:     %d detected\n", len(obs.Ports))
		}
		if len(obs.NewFiles) > 0 {
			fmt.Fprintf(os.Stdout, "  Files:     %d created\n", len(obs.NewFiles))
		}
		fmt.Fprintf(os.Stdout, "\n  Write to %s? [Y/n]: ", observeOutput)

		var answer string
		fmt.Scanln(&answer)
		if strings.EqualFold(answer, "n") {
			fmt.Fprintln(os.Stdout, "  Aborted.")
			return nil
		}
	}

	if err := os.WriteFile(observeOutput, yaml, 0644); err != nil {
		return fmt.Errorf("write %s: %w", observeOutput, err)
	}

	fmt.Fprintf(os.Stdout, "  Written to %s\n", observeOutput)
	return nil
}
