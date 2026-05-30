package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/baseline"
	"github.com/CosmoLabs-org/SmokeSig/internal/monorepo"
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// withOTelExport wraps the reporter with OTel telemetry export if configured.
func withOTelExport(rep reporter.Reporter, cfg *schema.SmokeConfig) reporter.Reporter {
	if !cfg.OTel.Enabled {
		return rep
	}
	exportURL := cfg.OTel.ExportURL
	if exportURL == "" && cfg.OTel.JaegerURL != "" {
		exportURL = cfg.OTel.JaegerURL + "/v1/traces"
	}
	if exportURL == "" {
		return rep
	}
	if _, err := url.Parse(exportURL); err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid otel export url: %v\n", err)
		return rep
	}
	service := cfg.OTel.ServiceName
	if service == "" {
		service = cfg.Project
	}
	otel := reporter.NewOTelReporter(exportURL, service, cfg.OTel.ExportHeaders)
	return reporter.NewMultiReporter(rep, otel)
}

// buildReporter creates a chained reporter from the format string, wrapping with
// OTel export and push report if configured. Returns the reporter and any opened
// files that must be closed (in reverse order) after the run completes.
func buildReporter(formatStr string, cfg *schema.SmokeConfig) (reporter.Reporter, func(), error) {
	rep, closers, err := reporter.ChainWithVerbosity(formatStr, os.Stdout, verbosity)
	if err != nil {
		return nil, nil, err
	}
	rep = withOTelExport(rep, cfg)
	rep = withPushReport(rep)
	rep = withConfigNotifications(rep, cfg)
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i].Close(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: closing reporter: %v\n", err)
			}
		}
	}
	return rep, closeAll, nil
}

// withPushReport wraps the reporter with a PushReporter or WebhookReporter
// depending on whether --webhook-format is set.
func withPushReport(rep reporter.Reporter) reporter.Reporter {
	if reportURL == "" {
		return rep
	}
	if _, err := url.Parse(reportURL); err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid report-url: %v\n", err)
		return rep
	}
	if webhookFormat != "" {
		wh := reporter.NewWebhookReporter(
			reportURL,
			reportAPIKey,
			reporter.WebhookFormat(webhookFormat),
			reporter.WebhookCondition(webhookOn),
		)
		return reporter.NewMultiReporter(rep, wh)
	}
	push := reporter.NewPushReporter(reportURL, reportAPIKey)
	return reporter.NewMultiReporter(rep, push)
}

// withConfigNotifications wraps the reporter with WebhookReporters from
// config-file notification entries.
func withConfigNotifications(rep reporter.Reporter, cfg *schema.SmokeConfig) reporter.Reporter {
	if len(cfg.Notifications) == 0 {
		return rep
	}
	reporters := []reporter.Reporter{rep}
	for _, n := range cfg.Notifications {
		apiKey := ""
		if n.APIKeyEnv != "" {
			apiKey = os.Getenv(n.APIKeyEnv)
		}
		on := reporter.WebhookCondition(n.On)
		if on == "" {
			on = reporter.WebhookOnFailure
		}
		wh := reporter.NewWebhookReporter(n.URL, apiKey, reporter.WebhookFormat(n.Format), on)
		reporters = append(reporters, wh)
	}
	if len(reporters) == 1 {
		return rep
	}
	return reporter.NewMultiReporter(reporters...)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run smoke tests",
	Long:  "Execute smoke tests defined in .smokesig.yaml",
	RunE:  runSmoke,
}

// runContext carries shared state for the interactive TUI entry point.
type runContext struct {
	cfg       *schema.SmokeConfig
	configDir string
	opts      runner.RunOptions
	formatStr string
	watch     bool
}

var (
	configFile     string
	tags           []string
	excludeTags    []string
	format         string
	failFast       bool
	timeout        string
	dryRun         bool
	watch          bool
	envName        string
	monorepoMode   bool
	otelCollector  string
	noOtel         bool
	noAuth         bool
	reportURL      string
	reportAPIKey   string
	webhookFormat  string
	webhookOn      string
	baselineFlag   bool
	baselineThresh float64
	verbose        bool
	quiet          bool
	verbosity      reporter.Verbosity
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&configFile, "file", "f", "", "Config file path (default: .smokesig.yaml, falls back to .smoke.yaml)")
	runCmd.Flags().StringSliceVar(&tags, "tag", nil, "Include only tests with these tags")
	runCmd.Flags().StringSliceVar(&excludeTags, "exclude-tag", nil, "Exclude tests with these tags")
	runCmd.Flags().StringVar(&format, "format", "terminal", "Output format(s), comma-separated (terminal,json,junit,tap,prometheus,gha,backstage)")
	runCmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop on first failure")
	runCmd.Flags().StringVar(&timeout, "timeout", "", "Per-test timeout override (e.g. 30s)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "List tests without running")
	runCmd.Flags().BoolVar(&watch, "watch", false, "Re-run tests when files change (Ctrl+C to exit)")
	runCmd.Flags().StringVar(&envName, "env", "", "Load environment-specific config (e.g. staging loads staging.smokesig.yaml)")
	runCmd.Flags().BoolVar(&monorepoMode, "monorepo", false, "Auto-discover .smokesig.yaml in subdirectories")
	runCmd.Flags().StringVar(&otelCollector, "otel-collector", "", "Override otel.jaeger_url and enable tracing")
	runCmd.Flags().BoolVar(&noOtel, "no-otel", false, "Disable otel trace propagation for this run")
	runCmd.Flags().BoolVar(&noAuth, "no-auth", false, "Disable OIDC auth for this run")
	runCmd.Flags().StringVar(&reportURL, "report-url", "", "POST results to this URL after run")
	runCmd.Flags().StringVar(&reportAPIKey, "report-api-key", "", "API key for report-url endpoint (X-API-Key header)")
	runCmd.Flags().StringVar(&webhookFormat, "webhook-format", "", "Webhook payload format: slack, pagerduty, or json (requires --report-url)")
	runCmd.Flags().StringVar(&webhookOn, "webhook-on", "failure", "When to send webhook: failure, always, or change")
	runCmd.Flags().BoolVar(&baselineFlag, "baseline", false, "Save and compare test timings against baseline")
	runCmd.Flags().Float64Var(&baselineThresh, "baseline-threshold", 50, "Regression threshold %% (flag if current > baseline * (1+threshold/100))")
	runCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show assertion details, timing per assertion, and full error context")
	runCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Minimal output: only failures and final summary")
	runCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")
}

// loadConfig reads the config file, applies environment overrides and CLI flags.
func loadConfig() (*schema.SmokeConfig, error) {
	// Resolve config path: explicit -f flag takes priority, otherwise auto-detect
	if configFile == "" {
		resolved, err := schema.LoadDefaultPath()
		if err != nil {
			return nil, err
		}
		configFile = resolved // Assign back so configDir derivation works correctly
	}

	cfg, err := schema.Load(configFile)
	if err != nil {
		return nil, err
	}
	if envName != "" {
		configDir := filepath.Dir(configFile)
		envFile := filepath.Join(configDir, envName+".smokesig.yaml")
		cfg, err = schema.MergeEnv(cfg, envFile)
		if err != nil {
			return nil, err
		}
	}
	if noOtel {
		cfg.OTel.Enabled = false
	}
	if noAuth {
		cfg.Auth.Profiles = nil
	}
	if otelCollector != "" {
		cfg.OTel.JaegerURL = otelCollector
		cfg.OTel.Enabled = true
	}
	if err := schema.Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// wrapRunnerError maps runner-specific errors to typed exit code errors.
func wrapRunnerError(err error) error {
	if err == nil {
		return nil
	}
	var prereqErr *runner.ErrPrereqFailed
	if errors.As(err, &prereqErr) {
		return &PrereqError{Err: err}
	}
	return err
}

func runSmoke(cmd *cobra.Command, args []string) error {
	// Resolve verbosity from flags (mutual exclusion enforced by Cobra)
	verbosity = reporter.VerbosityNormal
	if verbose {
		verbosity = reporter.VerbosityVerbose
	} else if quiet {
		verbosity = reporter.VerbosityQuiet
	}

	cfg, err := loadConfig()
	if err != nil {
		return &ConfigError{Err: fmt.Errorf("loading config: %w", err)}
	}

	configDir := filepath.Dir(configFile)
	if !filepath.IsAbs(configDir) {
		cwd, _ := os.Getwd()
		configDir = filepath.Join(cwd, configDir)
	}

	// Parse timeout early (needed by both interactive and normal paths)
	var timeoutDur time.Duration
	if timeout != "" {
		timeoutDur, err = time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", timeout, err)
		}
	}

	// Interactive TUI mode (requires -tags tui)
	if interactive {
		return runInteractive(&runContext{
			cfg:       cfg,
			configDir: configDir,
			opts: runner.RunOptions{
				Tags:        tags,
				ExcludeTags: excludeTags,
				FailFast:    failFast,
				DryRun:      dryRun,
				Timeout:     timeoutDur,
			},
			formatStr: format,
			watch:     watch,
		})
	}

	// Check monorepo mode
	if monorepoMode || cfg.Settings.Monorepo {
		configs, err := monorepo.Discover(configDir, cfg.Settings.MonorepoExclude)
		if err != nil {
			return fmt.Errorf("discovering sub-configs: %w", err)
		}
		if len(configs) == 0 {
			return fmt.Errorf("no smoke configs found in %s", configDir)
		}

		if watch {
			health := runner.NewTraceHealthTracker(10)
			return runWatch(configDir, configFile, func() error {
				refreshed, err := loadConfig()
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠ config reload: %v\n", err)
					return nil
				}
				cfg = refreshed
				subConfigs, err := monorepo.Discover(configDir, cfg.Settings.MonorepoExclude)
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠ monorepo discover: %v\n", err)
					return nil
				}
				if len(subConfigs) == 0 {
					fmt.Fprintln(os.Stderr, "⚠ no smoke configs found in monorepo")
					return nil
				}
				rep, closeAll, err := buildReporter(format, cfg)
				if err != nil {
					return err
				}
				defer closeAll()
				r := &runner.Runner{Config: cfg, Reporter: rep, ConfigDir: configDir, TraceHealth: health}
				_, err = r.RunMonorepo(runner.RunOptions{
					Tags:        tags,
					ExcludeTags: excludeTags,
					FailFast:    failFast,
					DryRun:      dryRun,
					Timeout:     timeoutDur,
					WatchMode:   true,
				}, subConfigs)
				if err != nil {
					return err
				}
				return nil
			})
		}

		rep, closeAll, err := buildReporter(format, cfg)
		if err != nil {
			return err
		}
		defer closeAll()
		r := &runner.Runner{Config: cfg, Reporter: rep, ConfigDir: configDir}
		result, err := r.RunMonorepo(runner.RunOptions{
			Tags:        tags,
			ExcludeTags: excludeTags,
			FailFast:    failFast,
			DryRun:      dryRun,
			Timeout:     timeoutDur,
		}, configs)
		if err != nil {
			return wrapRunnerError(err)
		}
		handleBaseline(result, configDir)
		if result.Failed > 0 {
			os.Exit(ExitFail)
		}
		return nil
	}

	if watch {
		health := runner.NewTraceHealthTracker(10)
		runOnce := func() error {
			refreshed, err := loadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠ config reload: %v\n", err)
				return nil
			}
			cfg = refreshed
			rep, closeAll, err := buildReporter(format, cfg)
			if err != nil {
				return err
			}
			defer closeAll()
			r := &runner.Runner{Config: cfg, Reporter: rep, ConfigDir: configDir, TraceHealth: health}
			_, err = r.Run(runner.RunOptions{
				Tags:        tags,
				ExcludeTags: excludeTags,
				FailFast:    failFast,
				DryRun:      dryRun,
				Timeout:     timeoutDur,
				WatchMode:   true,
			})
			if err != nil {
				return err
			}
			return nil
		}
		return runWatch(configDir, configFile, runOnce)
	}

	rep, closeAll, err := buildReporter(format, cfg)
	if err != nil {
		return err
	}
	defer closeAll()

	r := &runner.Runner{Config: cfg, Reporter: rep, ConfigDir: configDir}
	result, err := r.Run(runner.RunOptions{
		Tags:        tags,
		ExcludeTags: excludeTags,
		FailFast:    failFast,
		DryRun:      dryRun,
		Timeout:     timeoutDur,
	})
	if err != nil {
		return wrapRunnerError(err)
	}
	handleBaseline(result, configDir)
	if result.Failed > 0 {
		os.Exit(ExitFail)
	}
	return nil
}

// handleBaseline compares current timings against saved baseline, reports
// regressions, and updates the baseline file.
func handleBaseline(suite *runner.SuiteResult, configDir string) {
	if !baselineFlag {
		return
	}

	current := make(map[string]int64)
	for _, tr := range suite.Tests {
		current[tr.Name] = tr.Duration.Milliseconds()
	}

	blPath := filepath.Join(configDir, baseline.DefaultFile)
	bl, err := baseline.Load(blPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ baseline: %v\n", err)
		return
	}

	if len(bl) > 0 {
		comps := bl.Compare(current, baselineThresh)
		var regressions []baseline.Comparison
		var newTests []baseline.Comparison
		for _, c := range comps {
			if c.Regressed {
				regressions = append(regressions, c)
			}
			if c.NewTest {
				newTests = append(newTests, c)
			}
		}
		if len(regressions) > 0 || len(newTests) > 0 {
			fmt.Fprintln(os.Stderr, "\n📊 Baseline Comparison:")
			for _, r := range regressions {
				fmt.Fprintf(os.Stderr, "  ⚠ %s: %dms → %dms (%+.0f%%)\n", r.Name, r.BaselineMs, r.CurrentMs, r.DeltaPct)
			}
			for _, r := range newTests {
				fmt.Fprintf(os.Stderr, "  + %s: %dms (new)\n", r.Name, r.CurrentMs)
			}
		} else {
			fmt.Fprintln(os.Stderr, "\n📊 Baseline: no regressions")
		}
	}

	bl.Update(current)
	if err := bl.Save(blPath); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ baseline save: %v\n", err)
	}
}

func runWatch(configDir, configFile string, runOnce func() error) error {
	// Run once immediately
	if err := runOnce(); err != nil {
		fmt.Fprintf(os.Stderr, "initial run error: %v\n", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer w.Close()

	if err := w.Add(configDir); err != nil {
		return fmt.Errorf("watching %s: %w", configDir, err)
	}

	fmt.Fprintln(os.Stderr, "👀 watching", configDir, "(Ctrl+C to exit)")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Debounce: coalesce rapid events into a single re-run
	debounce := 500 * time.Millisecond
	var timer *time.Timer

	trigger := func() {
		fmt.Fprintln(os.Stderr, "\n🔁 change detected, re-running…")
		if err := runOnce(); err != nil {
			fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		}
	}

	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			// Ignore chmod-only events
			if !isRelevantEvent(ev.Op) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, trigger)
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		case <-sigCh:
			fmt.Fprintln(os.Stderr, "\n✋ stopping watcher")
			return nil
		}
	}
}
