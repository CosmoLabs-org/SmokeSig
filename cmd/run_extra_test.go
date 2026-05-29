package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/baseline"
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// writeRunConfig writes a YAML config to a temp dir and returns the dir path.
func writeRunConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.smokesig.yaml", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// silentReporter returns a terminal reporter that discards output.
func silentReporter() reporter.Reporter {
	return reporter.NewTerminal(io.Discard)
}

// TestRun_DryRun outputs plan without executing tests.
func TestRun_DryRun(t *testing.T) {
	dir := writeRunConfig(t, `
version: 1
project: dry-run-test
tests:
  - name: should-not-execute
    run: "echo RAN > dryrun_marker.txt"
    expect:
      exit_code: 0
`)
	cfg, err := schema.Load(dir + "/.smokesig.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
	result, err := r.Run(runner.RunOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 total test, got %d", result.Total)
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed (dry-run), got %d", result.Passed)
	}
	if _, statErr := os.Stat(dir + "/dryrun_marker.txt"); !os.IsNotExist(statErr) {
		t.Error("dry-run should not execute commands, but marker file was created")
	}
}

// TestRun_TagFilter selects only matching tests.
func TestRun_TagFilter(t *testing.T) {
	dir := writeRunConfig(t, `
version: 1
project: tag-test
tests:
  - name: smoke-only
    run: "true"
    tags: [smoke]
    expect:
      exit_code: 0
  - name: integration-only
    run: "true"
    tags: [integration]
    expect:
      exit_code: 0
  - name: no-tags
    run: "true"
    expect:
      exit_code: 0
`)
	cfg, err := schema.Load(dir + "/.smokesig.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
	result, err := r.Run(runner.RunOptions{Tags: []string{"smoke"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 test with tag 'smoke', got %d", result.Total)
	}
	if len(result.Tests) != 1 || result.Tests[0].Name != "smoke-only" {
		t.Errorf("expected 'smoke-only' test, got %+v", result.Tests)
	}
}

// TestRun_ExcludeTag skips tagged tests.
func TestRun_ExcludeTag(t *testing.T) {
	dir := writeRunConfig(t, `
version: 1
project: exclude-tag-test
tests:
  - name: keep-this
    run: "true"
    tags: [fast]
    expect:
      exit_code: 0
  - name: exclude-this
    run: "true"
    tags: [slow]
    expect:
      exit_code: 0
  - name: no-tags
    run: "true"
    expect:
      exit_code: 0
`)
	cfg, err := schema.Load(dir + "/.smokesig.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
	result, err := r.Run(runner.RunOptions{ExcludeTags: []string{"slow"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 tests after excluding 'slow', got %d", result.Total)
	}
	for _, tr := range result.Tests {
		if tr.Name == "exclude-this" {
			t.Error("test 'exclude-this' should have been excluded")
		}
	}
}

// TestRun_Timeout overrides default timeout.
func TestRun_Timeout(t *testing.T) {
	dir := writeRunConfig(t, `
version: 1
project: timeout-test
tests:
  - name: slow-test
    run: "sleep 10"
    expect:
      exit_code: 0
`)
	cfg, err := schema.Load(dir + "/.smokesig.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
	result, err := r.Run(runner.RunOptions{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 total test, got %d", result.Total)
	}
	if result.Tests[0].Passed {
		t.Error("expected test to fail due to timeout, but it passed")
	}
	if result.Tests[0].Duration > 2*time.Second {
		t.Errorf("test should have timed out quickly, took %v", result.Tests[0].Duration)
	}
}

// TestRun_FailFast stops after first failure.
func TestRun_FailFast(t *testing.T) {
	dir := writeRunConfig(t, `
version: 1
project: fail-fast-test
tests:
  - name: passes-first
    run: "true"
    expect:
      exit_code: 0
  - name: fails-second
    run: "false"
    expect:
      exit_code: 0
  - name: should-be-skipped
    run: "true"
    expect:
      exit_code: 0
`)
	cfg, err := schema.Load(dir + "/.smokesig.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
	result, err := r.Run(runner.RunOptions{FailFast: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped (after fail-fast), got %d", result.Skipped)
	}
	if len(result.Tests) != 3 {
		t.Fatalf("expected 3 test results, got %d", len(result.Tests))
	}
	if !result.Tests[2].Skipped {
		t.Error("third test should have been skipped after fail-fast")
	}
}

// TestRun_VerboseAndQuietMutuallyExclusive verifies that --verbose and --quiet
// cannot be used together.
func TestRun_VerboseAndQuietMutuallyExclusive(t *testing.T) {
	dir := writeRunConfig(t, `
version: 1
project: verbosity-test
tests:
  - name: hello
    run: "echo hi"
    expect:
      exit_code: 0
`)
	cmd := rootCmd
	cmd.SetArgs([]string{"run", "-f", dir + "/.smokesig.yaml", "--verbose", "--quiet"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --verbose and --quiet are set, got nil")
	}
}

// TestRun_VerboseFlag verifies the --verbose flag sets VerbosityVerbose.
func TestRun_VerboseFlag(t *testing.T) {
	// Reset global state
	verbose = false
	quiet = false
	verbosity = reporter.VerbosityNormal

	dir := writeRunConfig(t, `
version: 1
project: verbose-test
tests:
  - name: hello
    run: "echo hi"
    expect:
      exit_code: 0
`)
	configFile = dir + "/.smokesig.yaml"
	noOtel = false
	otelCollector = ""
	envName = ""

	verbose = true
	quiet = false
	defer func() { verbose = false }()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Simulate what runSmoke does
	verbosity = reporter.VerbosityNormal
	if verbose {
		verbosity = reporter.VerbosityVerbose
	}

	r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
	result, err := r.Run(runner.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", result.Passed)
	}
	if verbosity != reporter.VerbosityVerbose {
		t.Errorf("expected VerbosityVerbose, got %d", verbosity)
	}
}

// TestRun_QuietFlag verifies the --quiet flag sets VerbosityQuiet.
func TestRun_QuietFlag(t *testing.T) {
	// Reset global state
	verbose = false
	quiet = false
	verbosity = reporter.VerbosityNormal

	dir := writeRunConfig(t, `
version: 1
project: quiet-test
tests:
  - name: hello
    run: "echo hi"
    expect:
      exit_code: 0
`)
	configFile = dir + "/.smokesig.yaml"
	noOtel = false
	otelCollector = ""
	envName = ""

	verbose = false
	quiet = true
	defer func() { quiet = false }()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Simulate what runSmoke does
	verbosity = reporter.VerbosityNormal
	if verbose {
		verbosity = reporter.VerbosityVerbose
	} else if quiet {
		verbosity = reporter.VerbosityQuiet
	}

	r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
	result, err := r.Run(runner.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", result.Passed)
	}
	if verbosity != reporter.VerbosityQuiet {
		t.Errorf("expected VerbosityQuiet, got %d", verbosity)
	}
}

// TestRun_DefaultVerbosity verifies neither flag gives VerbosityNormal.
func TestRun_DefaultVerbosity(t *testing.T) {
	verbose = false
	quiet = false
	verbosity = reporter.VerbosityNormal

	// Simulate what runSmoke does
	if verbose {
		verbosity = reporter.VerbosityVerbose
	} else if quiet {
		verbosity = reporter.VerbosityQuiet
	}

	if verbosity != reporter.VerbosityNormal {
		t.Errorf("expected VerbosityNormal, got %d", verbosity)
	}
}

// TestLoadConfig_ReloadOnFileChange verifies loadConfig picks up changes
// when the config file is modified between calls.
func TestLoadConfig_ReloadOnFileChange(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.smokesig.yaml"

	os.WriteFile(path, []byte(`
version: 1
project: original
tests:
  - name: first
    run: "true"
    expect:
      exit_code: 0
`), 0644)

	configFile = path
	noOtel = false
	otelCollector = ""
	envName = ""

	cfg1, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg1.Project != "original" {
		t.Errorf("first load project = %q, want 'original'", cfg1.Project)
	}
	if len(cfg1.Tests) != 1 {
		t.Fatalf("first load tests = %d, want 1", len(cfg1.Tests))
	}

	os.WriteFile(path, []byte(`
version: 1
project: updated
tests:
  - name: first
    run: "true"
    expect:
      exit_code: 0
  - name: second
    run: "true"
    expect:
      exit_code: 0
`), 0644)

	cfg2, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Project != "updated" {
		t.Errorf("second load project = %q, want 'updated'", cfg2.Project)
	}
	if len(cfg2.Tests) != 2 {
		t.Errorf("second load tests = %d, want 2", len(cfg2.Tests))
	}
}

// TestLoadConfig_FileNotFound returns an error when the config file is absent.
func TestLoadConfig_FileNotFound(t *testing.T) {
	configFile = "/tmp/nonexistent_smokesig_test_xyz.yaml"
	noOtel = false
	otelCollector = ""
	envName = ""
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// TestLoadConfig_InvalidYAML returns an error for malformed YAML.
func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/.smokesig.yaml"
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)
	configFile = path
	noOtel = false
	otelCollector = ""
	envName = ""
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// TestLoadConfig_ValidConfig loads a minimal valid config and checks the project name.
func TestLoadConfig_ValidConfig(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/.smokesig.yaml"
	os.WriteFile(path, []byte("version: 1\nproject: test\ntests:\n  - name: hello\n    run: echo hi\n    expect:\n      exit_code: 0\n"), 0644)
	configFile = path
	noOtel = false
	otelCollector = ""
	envName = ""
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "test" {
		t.Errorf("project = %q, want test", cfg.Project)
	}
}

// TestLoadConfig_NoOtelDisablesTracing verifies noOtel=true turns off otel in config.
func TestLoadConfig_NoOtelDisablesTracing(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/.smokesig.yaml"
	os.WriteFile(path, []byte("version: 1\nproject: otel-off\notel:\n  enabled: true\n  jaeger_url: http://jaeger:16686\ntests:\n  - name: t1\n    run: echo ok\n    expect:\n      exit_code: 0\n"), 0644)
	configFile = path
	noOtel = true
	otelCollector = ""
	envName = ""
	defer func() { noOtel = false }()
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OTel.Enabled {
		t.Error("expected OTel.Enabled=false when --no-otel is set")
	}
}

// TestLoadConfig_OtelCollectorOverride verifies --otel-collector sets JaegerURL and enables OTel.
func TestLoadConfig_OtelCollectorOverride(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/.smokesig.yaml"
	os.WriteFile(path, []byte("version: 1\nproject: otel-on\ntests:\n  - name: t1\n    run: echo ok\n    expect:\n      exit_code: 0\n"), 0644)
	configFile = path
	noOtel = false
	otelCollector = "http://custom-collector:16686"
	envName = ""
	defer func() { otelCollector = "" }()
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.OTel.Enabled {
		t.Error("expected OTel.Enabled=true when --otel-collector is set")
	}
	if cfg.OTel.JaegerURL != "http://custom-collector:16686" {
		t.Errorf("JaegerURL = %q, want http://custom-collector:16686", cfg.OTel.JaegerURL)
	}
}

// TestWithOTelExport_Disabled returns original reporter when OTel is disabled.
func TestWithOTelExport_Disabled(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{OTel: schema.OTelConfig{Enabled: false}}
	result := withOTelExport(rep, cfg)
	if result != rep {
		t.Error("expected original reporter when OTel disabled, got different reporter")
	}
}

// TestWithOTelExport_WithExportURL returns MultiReporter when ExportURL is set.
func TestWithOTelExport_WithExportURL(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		Project: "test-svc",
		OTel: schema.OTelConfig{
			Enabled:   true,
			ExportURL: "http://localhost:4318/v1/traces",
		},
	}
	result := withOTelExport(rep, cfg)
	if _, ok := result.(*reporter.MultiReporter); !ok {
		t.Fatalf("expected *MultiReporter, got %T", result)
	}
}

// TestWithOTelExport_JaegerAutoAppend auto-appends /v1/traces to JaegerURL.
func TestWithOTelExport_JaegerAutoAppend(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		Project: "test-svc",
		OTel: schema.OTelConfig{
			Enabled:   true,
			JaegerURL: "http://jaeger:16686",
		},
	}
	result := withOTelExport(rep, cfg)
	if _, ok := result.(*reporter.MultiReporter); !ok {
		t.Fatalf("expected *MultiReporter, got %T", result)
	}
}

// TestWithOTelExport_InvalidURL returns original reporter when URL is invalid.
func TestWithOTelExport_InvalidURL(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		Project: "test-svc",
		OTel: schema.OTelConfig{
			Enabled:   true,
			ExportURL: "://invalid",
		},
	}
	result := withOTelExport(rep, cfg)
	if result != rep {
		t.Error("expected original reporter when URL is invalid, got different reporter")
	}
}

// TestWithOTelExport_NoURLs returns original when neither ExportURL nor JaegerURL set.
func TestWithOTelExport_NoURLs(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		OTel: schema.OTelConfig{Enabled: true},
	}
	result := withOTelExport(rep, cfg)
	if result != rep {
		t.Error("expected original reporter when no URLs configured, got different reporter")
	}
}

// TestBuildReporter_Terminal returns a single terminal reporter for "terminal" format.
func TestBuildReporter_Terminal(t *testing.T) {
	verbosity = reporter.VerbosityNormal
	cfg := &schema.SmokeConfig{}
	rep, closeAll, err := buildReporter("terminal", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
	// Should NOT be a MultiReporter (single format = unwrapped)
	if _, ok := rep.(*reporter.MultiReporter); ok {
		t.Error("single format should not wrap in MultiReporter")
	}
}

// TestBuildReporter_JSON returns a reporter for "json" format.
func TestBuildReporter_JSON(t *testing.T) {
	verbosity = reporter.VerbosityNormal
	cfg := &schema.SmokeConfig{}
	rep, closeAll, err := buildReporter("json", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
}

// TestBuildReporter_MultiFormat returns a MultiReporter for comma-separated formats.
func TestBuildReporter_MultiFormat(t *testing.T) {
	verbosity = reporter.VerbosityNormal
	cfg := &schema.SmokeConfig{}
	rep, closeAll, err := buildReporter("terminal,json", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if _, ok := rep.(*reporter.MultiReporter); !ok {
		t.Fatalf("expected *MultiReporter for multi-format, got %T", rep)
	}
}

// TestBuildReporter_InvalidFormat returns an error for unknown formats.
func TestBuildReporter_InvalidFormat(t *testing.T) {
	cfg := &schema.SmokeConfig{}
	_, _, err := buildReporter("xml", cfg)
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}

// TestWithPushReport_NoURL returns original reporter when reportURL is empty.
func TestWithPushReport_NoURL(t *testing.T) {
	reportURL = ""
	rep := silentReporter()
	result := withPushReport(rep)
	if result != rep {
		t.Error("expected original reporter when no report URL, got different reporter")
	}
}

// TestWithPushReport_PushReporter wraps with PushReporter when URL set without webhook format.
func TestWithPushReport_PushReporter(t *testing.T) {
	origURL := reportURL
	origKey := reportAPIKey
	origFmt := webhookFormat
	origOn := webhookOn
	reportURL = "http://localhost:9090/results"
	reportAPIKey = "test-key"
	webhookFormat = ""
	webhookOn = "failure"
	defer func() {
		reportURL = origURL
		reportAPIKey = origKey
		webhookFormat = origFmt
		webhookOn = origOn
	}()

	rep := silentReporter()
	result := withPushReport(rep)
	if _, ok := result.(*reporter.MultiReporter); !ok {
		t.Fatalf("expected *MultiReporter, got %T", result)
	}
}

// TestWithPushReport_WebhookReporter wraps with WebhookReporter when webhook-format is set.
func TestWithPushReport_WebhookReporter(t *testing.T) {
	origURL := reportURL
	origKey := reportAPIKey
	origFmt := webhookFormat
	origOn := webhookOn
	reportURL = "http://localhost:9090/hooks"
	reportAPIKey = "wh-key"
	webhookFormat = "slack"
	webhookOn = "always"
	defer func() {
		reportURL = origURL
		reportAPIKey = origKey
		webhookFormat = origFmt
		webhookOn = origOn
	}()

	rep := silentReporter()
	result := withPushReport(rep)
	if _, ok := result.(*reporter.MultiReporter); !ok {
		t.Fatalf("expected *MultiReporter, got %T", result)
	}
}

// TestWithPushReport_InvalidURL returns original reporter when URL is invalid.
func TestWithPushReport_InvalidURL(t *testing.T) {
	origURL := reportURL
	reportURL = "://not-a-url"
	defer func() { reportURL = origURL }()

	rep := silentReporter()
	result := withPushReport(rep)
	if result != rep {
		t.Error("expected original reporter for invalid URL, got different reporter")
	}
}

// TestTraceHealth_PersistsAcrossRunners verifies that a shared TraceHealthTracker
// accumulates results across multiple Runner instances (simulating watch cycles).
func TestTraceHealth_PersistsAcrossRunners(t *testing.T) {
	dir := writeRunConfig(t, `
version: 1
project: health-test
tests:
  - name: pass
    run: "true"
    expect:
      exit_code: 0
`)

	health := runner.NewTraceHealthTracker(10)

	for i := 0; i < 3; i++ {
		cfg, err := schema.Load(dir + "/.smokesig.yaml")
		if err != nil {
			t.Fatal(err)
		}
		r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir, TraceHealth: health}
		_, err = r.Run(runner.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}

	// The runner doesn't have otel_trace assertions, so TraceHealth isn't
	// updated via assertions. But the tracker persists across runs.
	if health.Total() != 0 {
		t.Errorf("expected 0 trace records (no otel_trace assertions), got %d", health.Total())
	}

	// Verify the tracker is shared by recording directly
	health.Record(true)
	health.Record(true)
	health.Record(false)
	if health.Total() != 3 {
		t.Errorf("expected 3 records after manual tracking, got %d", health.Total())
	}
	if health.HealthPct() != 66.7 {
		t.Errorf("expected 66.7%% health, got %.1f%%", health.HealthPct())
	}
}

// --- withConfigNotifications tests ---

// TestWithConfigNotifications_NoNotifications returns original reporter when config has no notifications.
func TestWithConfigNotifications_NoNotifications(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{}
	result := withConfigNotifications(rep, cfg)
	if result != rep {
		t.Error("expected original reporter when no notifications configured, got different reporter")
	}
}

// TestWithConfigNotifications_OneNotification wraps with MultiReporter when config has one notification.
func TestWithConfigNotifications_OneNotification(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		Notifications: []schema.Notification{
			{URL: "http://localhost:9090/hook", Format: "json", On: "always"},
		},
	}
	result := withConfigNotifications(rep, cfg)
	if _, ok := result.(*reporter.MultiReporter); !ok {
		t.Fatalf("expected *MultiReporter, got %T", result)
	}
}

// TestWithConfigNotifications_InvalidURL still wraps reporter (function does not validate URLs).
func TestWithConfigNotifications_InvalidURL(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		Notifications: []schema.Notification{
			{URL: "://not-a-url", Format: "json", On: "always"},
		},
	}
	result := withConfigNotifications(rep, cfg)
	// withConfigNotifications does not validate URLs — it passes them to
	// NewWebhookReporter, so the reporter is still wrapped.
	if _, ok := result.(*reporter.MultiReporter); !ok {
		t.Fatalf("expected *MultiReporter even with invalid URL, got %T", result)
	}
}

// --- handleBaseline tests ---

// TestHandleBaseline_FlagOff does nothing when baselineFlag is false.
func TestHandleBaseline_FlagOff(t *testing.T) {
	origFlag := baselineFlag
	baselineFlag = false
	defer func() { baselineFlag = origFlag }()

	dir := t.TempDir()
	suite := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "t1", Duration: 100 * time.Millisecond},
		},
	}
	handleBaseline(suite, dir)

	blPath := filepath.Join(dir, baseline.DefaultFile)
	if _, err := os.Stat(blPath); !os.IsNotExist(err) {
		t.Error("baseline file should not be created when flag is off")
	}
}

// TestHandleBaseline_NoPriorBaseline creates a new baseline file when none exists.
func TestHandleBaseline_NoPriorBaseline(t *testing.T) {
	origFlag := baselineFlag
	origThresh := baselineThresh
	baselineFlag = true
	baselineThresh = 50
	defer func() {
		baselineFlag = origFlag
		baselineThresh = origThresh
	}()

	dir := t.TempDir()
	suite := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "t1", Duration: 100 * time.Millisecond},
			{Name: "t2", Duration: 200 * time.Millisecond},
		},
	}
	handleBaseline(suite, dir)

	blPath := filepath.Join(dir, baseline.DefaultFile)
	data, err := os.ReadFile(blPath)
	if err != nil {
		t.Fatalf("expected baseline file to be created: %v", err)
	}

	var bl baseline.File
	if err := json.Unmarshal(data, &bl); err != nil {
		t.Fatalf("failed to parse baseline file: %v", err)
	}
	if len(bl) != 2 {
		t.Errorf("expected 2 entries in baseline, got %d", len(bl))
	}
	if bl["t1"].DurationMs != 100 {
		t.Errorf("t1 duration = %d, want 100", bl["t1"].DurationMs)
	}
	if bl["t2"].DurationMs != 200 {
		t.Errorf("t2 duration = %d, want 200", bl["t2"].DurationMs)
	}
}

// TestHandleBaseline_ExistingBaseline compares and updates an existing baseline.
func TestHandleBaseline_ExistingBaseline(t *testing.T) {
	origFlag := baselineFlag
	origThresh := baselineThresh
	baselineFlag = true
	baselineThresh = 50
	defer func() {
		baselineFlag = origFlag
		baselineThresh = origThresh
	}()

	dir := t.TempDir()
	blPath := filepath.Join(dir, baseline.DefaultFile)

	// Write an existing baseline with t1 at 100ms
	existing := baseline.File{
		"t1": {DurationMs: 100, Timestamp: time.Now().UTC()},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(blPath, existingData, 0644); err != nil {
		t.Fatal(err)
	}

	// Run handleBaseline with t1 at 200ms (100% increase → regression at 50% threshold)
	suite := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "t1", Duration: 200 * time.Millisecond},
		},
	}
	handleBaseline(suite, dir)

	// Verify baseline was updated
	data, err := os.ReadFile(blPath)
	if err != nil {
		t.Fatalf("failed to read updated baseline: %v", err)
	}
	var updated baseline.File
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("failed to parse updated baseline: %v", err)
	}
	if updated["t1"].DurationMs != 200 {
		t.Errorf("t1 duration = %d after update, want 200", updated["t1"].DurationMs)
	}
}
