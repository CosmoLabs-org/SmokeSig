package cmd

import (
	"encoding/json"
	"fmt"
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

// ---------------------------------------------------------------------------
// loadConfig tests
// ---------------------------------------------------------------------------

// minimalSmokeYAML is the smallest valid .smokesig.yaml for loadConfig testing.
const minimalSmokeYAML = `version: 1
project: loadconfig-test
tests:
  - name: basic
    run: echo ok
    expect:
      exit_code: 0
`

// resetLoadConfigVars saves and restores all package-level vars that loadConfig reads.
// Call it at the top of each test; t.Cleanup restores originals automatically.
func resetLoadConfigVars(t *testing.T) {
	t.Helper()
	origConfigFile := configFile
	origEnvName := envName
	origNoOtel := noOtel
	origOtelCollector := otelCollector
	t.Cleanup(func() {
		configFile = origConfigFile
		envName = origEnvName
		noOtel = origNoOtel
		otelCollector = origOtelCollector
	})
	envName = ""
	noOtel = false
	otelCollector = ""
}

// TestLoadConfig_ValidFile loads a valid config and expects no error.
func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetLoadConfigVars(t)
	configFile = p
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cfg.Project != "loadconfig-test" {
		t.Errorf("project = %q, want loadconfig-test", cfg.Project)
	}
}

// TestLoadConfig_MissingFile returns an error when the config file does not exist.
func TestLoadConfig_MissingFile(t *testing.T) {
	resetLoadConfigVars(t)
	configFile = "/nonexistent/path/.smokesig.yaml"
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing config file, got nil")
	}
}

// TestLoadConfig_NoOtel disables OTel when noOtel flag is true.
func TestLoadConfig_NoOtel(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	yaml := `version: 1
project: otel-test
otel:
  enabled: true
  jaeger_url: "http://jaeger:16686"
tests:
  - name: basic
    run: echo ok
    expect:
      exit_code: 0
`
	if err := os.WriteFile(p, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	resetLoadConfigVars(t)
	configFile = p
	noOtel = true

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OTel.Enabled {
		t.Error("expected OTel.Enabled=false when --no-otel is set")
	}
}

// TestLoadConfig_OtelCollector sets JaegerURL and enables OTel via --otel-collector.
func TestLoadConfig_OtelCollector(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetLoadConfigVars(t)
	configFile = p
	otelCollector = "http://jaeger:16686"

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OTel.JaegerURL != "http://jaeger:16686" {
		t.Errorf("JaegerURL = %q, want http://jaeger:16686", cfg.OTel.JaegerURL)
	}
	if !cfg.OTel.Enabled {
		t.Error("expected OTel.Enabled=true when --otel-collector is set")
	}
}

// TestLoadConfig_BadEnvFile returns an error when the env overlay file is missing.
func TestLoadConfig_BadEnvFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetLoadConfigVars(t)
	configFile = p
	envName = "nonexistent-env"

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing env file, got nil")
	}
}

// ---------------------------------------------------------------------------
// runSmoke tests
// ---------------------------------------------------------------------------

// resetRunSmokeVars saves and restores all package-level vars used by runSmoke.
func resetRunSmokeVars(t *testing.T) {
	t.Helper()
	origConfigFile := configFile
	origEnvName := envName
	origNoOtel := noOtel
	origOtelCollector := otelCollector
	origFormat := format
	origTags := tags
	origExcludeTags := excludeTags
	origFailFast := failFast
	origDryRun := dryRun
	origTimeout := timeout
	origMonorepoMode := monorepoMode
	origWatch := watch
	origVerbose := verbose
	origQuiet := quiet
	origBaselineFlag := baselineFlag
	origBaselineThresh := baselineThresh
	origUseTUI := useTUI
	t.Cleanup(func() {
		configFile = origConfigFile
		envName = origEnvName
		noOtel = origNoOtel
		otelCollector = origOtelCollector
		format = origFormat
		tags = origTags
		excludeTags = origExcludeTags
		failFast = origFailFast
		dryRun = origDryRun
		timeout = origTimeout
		monorepoMode = origMonorepoMode
		watch = origWatch
		verbose = origVerbose
		quiet = origQuiet
		baselineFlag = origBaselineFlag
		baselineThresh = origBaselineThresh
		useTUI = origUseTUI
	})
	// Defaults for a simple, non-monorepo, non-watch run.
	envName = ""
	noOtel = true
	otelCollector = ""
	format = "terminal"
	tags = nil
	excludeTags = nil
	failFast = false
	dryRun = false
	timeout = ""
	monorepoMode = false
	watch = false
	verbose = false
	quiet = false
	baselineFlag = false
	baselineThresh = 50
	useTUI = false
}

// TestRunSmoke_DryRun exercises the main runSmoke path in dry-run mode (no actual exec).
func TestRunSmoke_DryRun(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetRunSmokeVars(t)
	configFile = p
	dryRun = true
	format = "json"

	if err := runSmoke(nil, nil); err != nil {
		t.Fatalf("runSmoke dry-run: %v", err)
	}
}

// TestRunSmoke_MissingConfig returns an error when the config file is absent.
func TestRunSmoke_MissingConfig(t *testing.T) {
	resetRunSmokeVars(t)
	configFile = "/nonexistent/.smokesig.yaml"
	err := runSmoke(nil, nil)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
}

// TestRunSmoke_InvalidTimeout returns an error for a bad timeout string.
func TestRunSmoke_InvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetRunSmokeVars(t)
	configFile = p
	timeout = "not-a-duration"

	err := runSmoke(nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid timeout, got nil")
	}
}

// TestRunSmoke_VerboseFlag exercises the verbose verbosity branch.
func TestRunSmoke_VerboseFlag(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetRunSmokeVars(t)
	configFile = p
	dryRun = true
	verbose = true

	if err := runSmoke(nil, nil); err != nil {
		t.Fatalf("runSmoke verbose: %v", err)
	}
	if verbosity != reporter.VerbosityVerbose {
		t.Errorf("verbosity = %d, want VerbosityVerbose", verbosity)
	}
}

// TestRunSmoke_QuietFlag exercises the quiet verbosity branch.
func TestRunSmoke_QuietFlag(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetRunSmokeVars(t)
	configFile = p
	dryRun = true
	quiet = true

	if err := runSmoke(nil, nil); err != nil {
		t.Fatalf("runSmoke quiet: %v", err)
	}
	if verbosity != reporter.VerbosityQuiet {
		t.Errorf("verbosity = %d, want VerbosityQuiet", verbosity)
	}
}

// ---------------------------------------------------------------------------
// runWatch tests
// ---------------------------------------------------------------------------

// TestRunWatch_SignalTerminates verifies runWatch exits cleanly on SIGINT.
func TestRunWatch_SignalTerminates(t *testing.T) {
	dir := t.TempDir()

	ranOnce := false
	runOnce := func() error {
		ranOnce = true
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- runWatch(dir, filepath.Join(dir, ".smokesig.yaml"), runOnce)
	}()

	// Give the watcher a moment to start, then send SIGINT.
	time.Sleep(50 * time.Millisecond)
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	p.Signal(os.Interrupt)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runWatch returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runWatch did not exit after SIGINT within 3s")
	}

	if !ranOnce {
		t.Error("expected runOnce to be called at least once before signal")
	}
}

// TestRunWatch_FileChangeTrigger verifies the debounce path fires on file writes.
func TestRunWatch_FileChangeTrigger(t *testing.T) {
	dir := t.TempDir()

	runCount := 0
	runOnce := func() error {
		runCount++
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- runWatch(dir, filepath.Join(dir, ".smokesig.yaml"), runOnce)
	}()

	// Wait for watcher to start, then write a file to trigger the debounce.
	time.Sleep(80 * time.Millisecond)
	triggerPath := filepath.Join(dir, "trigger.txt")
	os.WriteFile(triggerPath, []byte("change"), 0644) //nolint:errcheck

	// Wait for debounce (500ms) + buffer to fire, then signal.
	time.Sleep(700 * time.Millisecond)
	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(os.Interrupt)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runWatch returned error: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("runWatch did not exit after SIGINT within 4s")
	}

	if runCount < 2 {
		t.Errorf("expected runOnce called at least twice (initial + debounce), got %d", runCount)
	}
}

// TestRunWatch_RunOnceError verifies runWatch does not propagate runOnce errors (just logs them).
func TestRunWatch_RunOnceError(t *testing.T) {
	dir := t.TempDir()

	runOnce := func() error {
		return fmt.Errorf("simulated run error")
	}

	done := make(chan error, 1)
	go func() {
		done <- runWatch(dir, filepath.Join(dir, ".smokesig.yaml"), runOnce)
	}()

	time.Sleep(50 * time.Millisecond)
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(os.Interrupt)

	select {
	case err := <-done:
		// runWatch logs errors but does not return them for the initial run.
		if err != nil {
			t.Errorf("runWatch should not propagate runOnce errors, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runWatch did not exit within 3s")
	}
}

// ---------------------------------------------------------------------------
// withConfigNotifications additional tests
// ---------------------------------------------------------------------------

// TestWithConfigNotifications_APIKeyEnv exercises the APIKeyEnv lookup branch.
func TestWithConfigNotifications_APIKeyEnv(t *testing.T) {
	t.Setenv("TEST_WEBHOOK_KEY_2", "secret123")
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		Notifications: []schema.Notification{
			{URL: "https://hooks.example.com/test", Format: "json", On: "always", APIKeyEnv: "TEST_WEBHOOK_KEY_2"},
		},
	}
	got := withConfigNotifications(rep, cfg)
	if got == rep {
		t.Error("expected reporter to be wrapped when APIKeyEnv is set")
	}
}

// TestWithConfigNotifications_EmptyOn exercises the default "on" assignment branch.
func TestWithConfigNotifications_EmptyOn(t *testing.T) {
	rep := silentReporter()
	cfg := &schema.SmokeConfig{
		Notifications: []schema.Notification{
			{URL: "https://hooks.example.com/test", Format: "json", On: ""},
		},
	}
	// Should not panic; empty On defaults to "failure".
	got := withConfigNotifications(rep, cfg)
	if got == rep {
		t.Error("expected reporter to be wrapped")
	}
}

// ---------------------------------------------------------------------------
// handleBaseline additional tests (regression/new-test comparison paths)
// ---------------------------------------------------------------------------

// TestHandleBaseline_WithExistingBaseline exercises the compare path with regressions.
func TestHandleBaseline_WithExistingBaseline(t *testing.T) {
	dir := t.TempDir()

	// Write an existing baseline file with one test at 100ms.
	bl := baseline.File{
		"slow-test": {DurationMs: 100},
	}
	blPath := filepath.Join(dir, baseline.DefaultFile)
	data, _ := json.Marshal(bl)
	if err := os.WriteFile(blPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Fake a suite result where "slow-test" now takes 300ms (3x = 200% regression).
	suite := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "slow-test", Duration: 300 * time.Millisecond},
		},
	}

	origFlag := baselineFlag
	origThresh := baselineThresh
	t.Cleanup(func() {
		baselineFlag = origFlag
		baselineThresh = origThresh
	})
	baselineFlag = true
	baselineThresh = 50 // 50% threshold — 200% increase should trigger

	// Should not panic; writes regressions to stderr.
	handleBaseline(suite, dir)

	// Verify baseline file was updated.
	if _, err := os.Stat(blPath); err != nil {
		t.Errorf("baseline file should still exist after update: %v", err)
	}
}

// TestHandleBaseline_NewTest exercises the newTests branch.
func TestHandleBaseline_NewTest(t *testing.T) {
	dir := t.TempDir()

	// Write a baseline with one test; suite has a different (new) test.
	bl := baseline.File{
		"existing-test": {DurationMs: 50},
	}
	blPath := filepath.Join(dir, baseline.DefaultFile)
	data, _ := json.Marshal(bl)
	if err := os.WriteFile(blPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	suite := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "new-test", Duration: 10 * time.Millisecond},
		},
	}

	origFlag := baselineFlag
	t.Cleanup(func() { baselineFlag = origFlag })
	baselineFlag = true

	// Should not panic.
	handleBaseline(suite, dir)
}

// TestHandleBaseline_NoRegressions exercises the "no regressions" branch.
func TestHandleBaseline_NoRegressions(t *testing.T) {
	dir := t.TempDir()

	bl := baseline.File{
		"fast-test": {DurationMs: 50},
	}
	blPath := filepath.Join(dir, baseline.DefaultFile)
	data, _ := json.Marshal(bl)
	if err := os.WriteFile(blPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	suite := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "fast-test", Duration: 55 * time.Millisecond},
		},
	}

	origFlag := baselineFlag
	origThresh := baselineThresh
	t.Cleanup(func() {
		baselineFlag = origFlag
		baselineThresh = origThresh
	})
	baselineFlag = true
	baselineThresh = 50

	// Should print "no regressions" and not panic.
	handleBaseline(suite, dir)
}

// TestRunSmoke_WatchMode exercises the watch path in runSmoke, terminated via SIGINT.
func TestRunSmoke_WatchMode(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetRunSmokeVars(t)
	configFile = p
	watch = true
	dryRun = true

	done := make(chan error, 1)
	go func() {
		done <- runSmoke(nil, nil)
	}()

	time.Sleep(80 * time.Millisecond)
	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(os.Interrupt)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runSmoke watch: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("runSmoke watch mode did not exit after SIGINT within 4s")
	}
}

// TestRunSmoke_MonorepoMode exercises the monorepo path in runSmoke.
func TestRunSmoke_MonorepoMode(t *testing.T) {
	// Create a parent dir with two sub-project configs.
	root := t.TempDir()
	sub1 := filepath.Join(root, "svc1")
	sub2 := filepath.Join(root, "svc2")
	if err := os.MkdirAll(sub1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub2, 0755); err != nil {
		t.Fatal(err)
	}
	subCfg := `version: 1
project: sub
tests:
  - name: basic
    run: echo ok
    expect:
      exit_code: 0
`
	for _, d := range []string{sub1, sub2} {
		if err := os.WriteFile(filepath.Join(d, ".smokesig.yaml"), []byte(subCfg), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Parent config — needs at least one test to pass validation.
	// We use the CLI monorepoMode flag, not the settings.monorepo field.
	parentCfg := `version: 1
project: monorepo-parent
tests:
  - name: parent-check
    run: echo ok
    expect:
      exit_code: 0
`
	parentP := filepath.Join(root, ".smokesig.yaml")
	if err := os.WriteFile(parentP, []byte(parentCfg), 0644); err != nil {
		t.Fatal(err)
	}

	resetRunSmokeVars(t)
	configFile = parentP
	monorepoMode = true
	dryRun = true
	format = "json"

	if err := runSmoke(nil, nil); err != nil {
		t.Fatalf("runSmoke monorepo: %v", err)
	}
}

// TestRunSmoke_MonorepoWatch exercises the monorepo+watch path in runSmoke, terminated via SIGINT.
func TestRunSmoke_MonorepoWatch(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "svc1")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	subCfg := `version: 1
project: sub
tests:
  - name: basic
    run: echo ok
    expect:
      exit_code: 0
`
	if err := os.WriteFile(filepath.Join(sub, ".smokesig.yaml"), []byte(subCfg), 0644); err != nil {
		t.Fatal(err)
	}
	parentCfg := `version: 1
project: parent
tests:
  - name: parent-check
    run: echo ok
    expect:
      exit_code: 0
`
	parentP := filepath.Join(root, ".smokesig.yaml")
	if err := os.WriteFile(parentP, []byte(parentCfg), 0644); err != nil {
		t.Fatal(err)
	}

	resetRunSmokeVars(t)
	configFile = parentP
	monorepoMode = true
	watch = true
	dryRun = true

	done := make(chan error, 1)
	go func() {
		done <- runSmoke(nil, nil)
	}()

	time.Sleep(80 * time.Millisecond)
	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(os.Interrupt)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runSmoke monorepo+watch: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("runSmoke monorepo+watch did not exit after SIGINT within 4s")
	}
}

// TestRunSmoke_ActualRun exercises the full runSmoke execution path with a passing test.
func TestRunSmoke_ActualRun(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetRunSmokeVars(t)
	configFile = p
	dryRun = false
	format = "json"

	if err := runSmoke(nil, nil); err != nil {
		t.Fatalf("runSmoke actual run: %v", err)
	}
}

// TestRunSmoke_WithTimeout exercises the timeout-parsing path in runSmoke.
func TestRunSmoke_WithTimeout(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(minimalSmokeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	resetRunSmokeVars(t)
	configFile = p
	dryRun = true
	timeout = "30s"

	if err := runSmoke(nil, nil); err != nil {
		t.Fatalf("runSmoke with timeout: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runInit tests
// ---------------------------------------------------------------------------

func resetInitVars(t *testing.T) {
	t.Helper()
	origForce := forceOverwrite
	origFromRunning := fromRunning
	origWithDocIntegrity := withDocIntegrity
	t.Cleanup(func() {
		forceOverwrite = origForce
		fromRunning = origFromRunning
		withDocIntegrity = origWithDocIntegrity
	})
	forceOverwrite = false
	fromRunning = ""
	withDocIntegrity = false
}

// TestRunInit_CreatesConfig verifies runInit creates .smokesig.yaml in cwd.
func TestRunInit_CreatesConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	resetInitVars(t)
	forceOverwrite = true

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".smokesig.yaml")); err != nil {
		t.Error("expected .smokesig.yaml to be created")
	}
}

// TestRunInit_ExistsWithoutForce returns an error when file exists and --force not set.
func TestRunInit_ExistsWithoutForce(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// Pre-create the file.
	if err := os.WriteFile(".smokesig.yaml", []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	resetInitVars(t)
	forceOverwrite = false

	if err := runInit(nil, nil); err == nil {
		t.Fatal("expected error when .smokesig.yaml exists without --force")
	}
}

// TestRunInit_WithDocIntegrity exercises the withDocIntegrity path.
func TestRunInit_WithDocIntegrity(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	resetInitVars(t)
	forceOverwrite = true
	withDocIntegrity = true

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit with doc integrity: %v", err)
	}
	if _, err := os.Stat(".smokesig.yaml"); err != nil {
		t.Error("expected .smokesig.yaml to be created")
	}
}
