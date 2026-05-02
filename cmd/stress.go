package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/reporter"
	"github.com/CosmoLabs-org/cosmo-smoke/internal/runner"
	"github.com/spf13/cobra"
)

var (
	stressRuns     int
	stressWorkers  int
	stressFailFast bool
)

var stressCmd = &cobra.Command{
	Use:   "stress <test-name>",
	Short: "Run a single test repeatedly to detect flakiness",
	Long:  "Run a single smoke test N times with configurable parallelism.\nReports pass rate, timing distribution, and deduplicated errors.",
	Args:  cobra.ExactArgs(1),
	RunE:  runStress,
}

func init() {
	rootCmd.AddCommand(stressCmd)
	stressCmd.Flags().IntVar(&stressRuns, "runs", 50, "Total number of executions")
	stressCmd.Flags().IntVar(&stressWorkers, "workers", 1, "Concurrency (1 = sequential)")
	stressCmd.Flags().BoolVar(&stressFailFast, "fail-fast", false, "Stop on first failure")
	stressCmd.Flags().StringVarP(&configFile, "file", "f", ".smoke.yaml", "Config file path")
	stressCmd.Flags().StringVar(&format, "format", "terminal", "Output format (terminal, json)")
}

func runStress(cmd *cobra.Command, args []string) error {
	testName := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	rep, closeAll, err := buildReporter(format, cfg)
	if err != nil {
		return err
	}
	defer closeAll()

	configDir := filepath.Dir(configFile)
	if configDir == "." {
		configDir = ""
	}

	r := &runner.Runner{
		Config:    cfg,
		Reporter:  rep,
		ConfigDir: configDir,
	}

	result := r.StressTest(testName, stressRuns, stressWorkers, stressFailFast)
	if result.TotalRuns == 0 {
		return fmt.Errorf("test %q not found in %s", testName, configFile)
	}

	fmt.Fprint(os.Stdout, formatStressSummary(result, cfg.Project))

	reportStressResult(rep, result, cfg.Project)

	if result.PassRate < 100.0 {
		os.Exit(1)
	}
	return nil
}

func reportStressResult(rep reporter.Reporter, result runner.StressResult, project string) {
	for i := range result.Passes {
		rep.TestResult(reporter.TestResultData{
			Name:   fmt.Sprintf("%s (run %d)", result.TestName, i+1),
			Passed: true,
		})
	}
	rep.Summary(reporter.SuiteResultData{
		Project:  project,
		Total:    result.TotalRuns,
		Passed:   result.Passes,
		Failed:   result.Failures,
		Duration: result.Duration,
	})
}

func formatStressSummary(r runner.StressResult, project string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n  Stress Test Complete: %s\n", r.TestName)
	if project != "" {
		fmt.Fprintf(&b, "  Project: %s\n", project)
	}
	fmt.Fprintf(&b, "  %s\n", strings.Repeat("-", 50))
	fmt.Fprintf(&b, "  Total Runs:    %d\n", r.TotalRuns)
	fmt.Fprintf(&b, "  Concurrency:   %d workers\n", r.Concurrency)
	fmt.Fprintf(&b, "  Duration:      %s\n", r.Duration.Round(time.Millisecond))
	fmt.Fprintf(&b, "  Reliability:   %.0f%% (%s)\n", r.PassRate, r.Reliability)
	fmt.Fprintf(&b, "  Passed:        %d/%d\n", r.Passes, r.TotalRuns)

	if len(r.ErrorGroups) > 0 {
		fmt.Fprintf(&b, "\n  Failures:\n")
		for _, eg := range r.ErrorGroups {
			fmt.Fprintf(&b, "    - [%d times]: %s\n", eg.Count, eg.Message)
		}
	}

	return b.String()
}
