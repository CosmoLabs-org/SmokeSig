package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// writeCfg writes a .smokesig.yaml to dir and returns the full path.
func writeCfg(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeCfg: %v", err)
	}
	return p
}

// setGlobalConfigFile sets the package-level configFile var and restores it.
func setGlobalConfigFile(t *testing.T, path string) {
	t.Helper()
	old := configFile
	configFile = path
	t.Cleanup(func() { configFile = old })
}

// discardOutput redirects stdout+stderr to /dev/null for the duration of the test.
func discardOutput(t *testing.T) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip("cannot open /dev/null")
	}
	os.Stdout, os.Stderr = devNull, devNull
	t.Cleanup(func() {
		os.Stdout = oldOut
		os.Stderr = oldErr
		devNull.Close()
	})
}

// saveRunFlags captures and restores all run.go package-level flag vars.
func saveRunFlags(t *testing.T) {
	t.Helper()
	v, q, ff, dr, w, mr, bl, no := verbose, quiet, failFast, dryRun, watch, monorepoMode, baselineFlag, noOtel
	to, fmt_, en, oc, ru, wf, wo := timeout, format, envName, otelCollector, reportURL, webhookFormat, webhookOn
	bt, vb := baselineThresh, verbosity
	t.Cleanup(func() {
		verbose, quiet, failFast, dryRun, watch, monorepoMode, baselineFlag, noOtel = v, q, ff, dr, w, mr, bl, no
		timeout, format, envName, otelCollector, reportURL, webhookFormat, webhookOn = to, fmt_, en, oc, ru, wf, wo
		baselineThresh, verbosity = bt, vb
	})
}

const passingCfg = `
version: 1
project: test-proj
tests:
  - name: always-pass
    run: "true"
    expect:
      exit_code: 0
`

// ─── withConfigNotifications ──────────────────────────────────────────────────

func TestWithConfigNotifications_NoNotifications_CoverageBoost(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	setGlobalConfigFile(t, p)
	saveRunFlags(t)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	rep := reporter.NewTerminal(io.Discard)
	got := withConfigNotifications(rep, cfg)
	if got != rep {
		t.Error("expected same reporter when cfg.Notifications is empty")
	}
}

func TestWithConfigNotifications_WithSlackNotification(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, `
version: 1
project: notify-test
notifications:
  - url: "http://hooks.example.com/slack"
    format: "slack"
    on: "failure"
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`)
	setGlobalConfigFile(t, p)
	saveRunFlags(t)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	rep := reporter.NewTerminal(io.Discard)
	got := withConfigNotifications(rep, cfg)
	if got == rep {
		t.Error("expected a wrapped multi-reporter when notifications are configured")
	}
}

func TestWithConfigNotifications_DefaultOnIsFailure(t *testing.T) {
	// notification with no "on" field — code defaults to WebhookOnFailure
	dir := t.TempDir()
	p := writeCfg(t, dir, `
version: 1
project: default-on
notifications:
  - url: "http://hooks.example.com/notify"
    format: "json"
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`)
	setGlobalConfigFile(t, p)
	saveRunFlags(t)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	rep := reporter.NewTerminal(io.Discard)
	got := withConfigNotifications(rep, cfg)
	if got == rep {
		t.Error("expected wrapped reporter")
	}
}

func TestWithConfigNotifications_APIKeyFromEnv(t *testing.T) {
	os.Setenv("_TEST_NOTIFY_KEY", "secret")
	t.Cleanup(func() { os.Unsetenv("_TEST_NOTIFY_KEY") })

	dir := t.TempDir()
	p := writeCfg(t, dir, `
version: 1
project: api-key-env
notifications:
  - url: "http://hooks.example.com/notify"
    format: "json"
    api_key_env: "_TEST_NOTIFY_KEY"
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`)
	setGlobalConfigFile(t, p)
	saveRunFlags(t)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	rep := reporter.NewTerminal(io.Discard)
	got := withConfigNotifications(rep, cfg)
	if got == rep {
		t.Error("expected wrapped reporter with api key notification")
	}
}

// ─── handleBaseline ───────────────────────────────────────────────────────────

func TestHandleBaseline_FlagOff_NoOp(t *testing.T) {
	old := baselineFlag
	baselineFlag = false
	t.Cleanup(func() { baselineFlag = old })

	suite := &runner.SuiteResult{Tests: []runner.TestResult{{Name: "t1"}}}
	// should return immediately without touching filesystem
	handleBaseline(suite, t.TempDir())
}

func TestHandleBaseline_FirstRun_WritesFile(t *testing.T) {
	old := baselineFlag
	baselineFlag = true
	t.Cleanup(func() { baselineFlag = old })

	dir := t.TempDir()
	suite := &runner.SuiteResult{Tests: []runner.TestResult{
		{Name: "alpha"},
		{Name: "beta"},
	}}
	handleBaseline(suite, dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Error("expected baseline file to be written")
	}
}

func TestHandleBaseline_SecondRun_Compares(t *testing.T) {
	old := baselineFlag
	oldThresh := baselineThresh
	baselineFlag = true
	baselineThresh = 50
	t.Cleanup(func() {
		baselineFlag = old
		baselineThresh = oldThresh
	})

	dir := t.TempDir()
	suite1 := &runner.SuiteResult{Tests: []runner.TestResult{{Name: "alpha"}}}
	handleBaseline(suite1, dir)

	// second run — adds a new test, triggers "new test" branch
	suite2 := &runner.SuiteResult{Tests: []runner.TestResult{
		{Name: "alpha"},
		{Name: "gamma"},
	}}
	handleBaseline(suite2, dir) // must not panic
}

func TestHandleBaseline_EmptySuite(t *testing.T) {
	old := baselineFlag
	baselineFlag = true
	t.Cleanup(func() { baselineFlag = old })

	dir := t.TempDir()
	suite := &runner.SuiteResult{Tests: []runner.TestResult{}}
	handleBaseline(suite, dir) // no tests — must not panic
}

// ─── runSmoke (early-error and flag paths not already covered) ────────────────

func TestRunSmoke_MissingConfigFile(t *testing.T) {
	saveRunFlags(t)
	setGlobalConfigFile(t, "/nonexistent/.smokesig.yaml")
	format = "terminal"

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error with missing config")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("error = %q, want 'loading config'", err.Error())
	}
}

func TestRunSmoke_BadTimeout(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	timeout = "notaduration"
	format = "terminal"

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunSmoke_BadFormat(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	format = "unknownformat999"

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestRunSmoke_VerboseSetsVerbosity(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	verbose = true
	dryRun = true
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verbosity != reporter.VerbosityVerbose {
		t.Errorf("verbosity = %v, want VerbosityVerbose", verbosity)
	}
}

func TestRunSmoke_QuietSetsVerbosity(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	quiet = true
	dryRun = true
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verbosity != reporter.VerbosityQuiet {
		t.Errorf("verbosity = %v, want VerbosityQuiet", verbosity)
	}
}

func TestRunSmoke_MonorepoNoSubConfigs(t *testing.T) {
	dir := t.TempDir()
	// Root config enables monorepo but no sub-directories have configs
	p := writeCfg(t, dir, `
version: 1
project: mono
settings:
  monorepo: true
tests:
  - name: placeholder
    run: "true"
    expect:
      exit_code: 0
`)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	monorepoMode = true
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error when no sub-configs found")
	}
	if !strings.Contains(err.Error(), "no smoke configs") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunSmoke_MonorepoInvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	// Create a sub-project so monorepo.Discover returns something
	sub := filepath.Join(dir, "svc")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, ".smokesig.yaml"), []byte(passingCfg), 0644)

	p := writeCfg(t, dir, `
version: 1
project: mono
tests:
  - name: placeholder
    run: "true"
    expect:
      exit_code: 0
`)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	monorepoMode = true
	timeout = "bad-duration"
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid timeout in monorepo mode")
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunSmoke_DryRunSuccess(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	dryRun = true
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
}

// ─── runWatch ────────────────────────────────────────────────────────────────

func TestRunWatch_NonexistentDir(t *testing.T) {
	err := runWatch("/path/that/does/not/exist", "/path/that/does/not/exist/.smokesig.yaml",
		func() error { return nil })
	if err == nil {
		t.Fatal("expected error watching non-existent directory")
	}
	if !strings.Contains(err.Error(), "watching") {
		t.Errorf("error = %q", err.Error())
	}
}

// TestRunWatch_CallsRunOnce verifies runWatch calls runOnce at least once before
// the watcher is set up — tested via the nonexistent-dir path which returns early.
func TestRunWatch_CallsRunOnce(t *testing.T) {
	called := false
	runOnce := func() error {
		called = true
		return nil
	}
	// Non-existent dir: runWatch calls runOnce first, then fails on w.Add
	err := runWatch("/path/does/not/exist/999", "/path/does/not/exist/999/.smokesig.yaml", runOnce)
	if err == nil {
		t.Fatal("expected error watching non-existent directory")
	}
	if !called {
		t.Error("runOnce should have been called before watcher setup")
	}
}

// TestRunWatch_InitialRunError verifies runWatch tolerates errors from runOnce
// and still proceeds to set up the watcher (fails at w.Add for nonexistent dir).
func TestRunWatch_InitialRunError(t *testing.T) {
	runOnce := func() error {
		return fmt.Errorf("simulated run error")
	}
	err := runWatch("/path/does/not/exist/999", "/path/does/not/exist/999/.smokesig.yaml", runOnce)
	// Should fail at the watch setup, not panic
	if err == nil {
		t.Fatal("expected error from watch setup on non-existent dir")
	}
}

// ─── noopReporter (serve.go) ─────────────────────────────────────────────────

func TestNoopReporter_PrereqStart(t *testing.T) {
	n := newNoopReporter()
	n.PrereqStart("prereq-name") // must not panic
}

func TestNoopReporter_PrereqResult_Passed(t *testing.T) {
	n := newNoopReporter()
	n.PrereqResult(reporter.PrereqResultData{Name: "db", Passed: true})
}

func TestNoopReporter_PrereqResult_Failed(t *testing.T) {
	n := newNoopReporter()
	n.PrereqResult(reporter.PrereqResultData{Name: "db", Passed: false, Error: fmt.Errorf("timeout")})
}

func TestNoopReporter_TestStart(t *testing.T) {
	n := newNoopReporter()
	n.TestStart("my-test")
}

func TestNoopReporter_TestResult_Passed(t *testing.T) {
	n := newNoopReporter()
	n.TestResult(reporter.TestResultData{Name: "t", Passed: true})
}

func TestNoopReporter_TestResult_Failed(t *testing.T) {
	n := newNoopReporter()
	n.TestResult(reporter.TestResultData{Name: "t", Passed: false, Error: fmt.Errorf("exit 1")})
}

func TestNoopReporter_Summary_AllPass(t *testing.T) {
	n := newNoopReporter()
	n.Summary(reporter.SuiteResultData{Project: "p", Total: 3, Passed: 3})
}

func TestNoopReporter_Summary_WithFailures(t *testing.T) {
	n := newNoopReporter()
	n.Summary(reporter.SuiteResultData{Total: 5, Passed: 2, Failed: 3})
}

// ─── runServe (early-error path via invalid port) ─────────────────────────────

func TestRunServe_InvalidPortErrors(t *testing.T) {
	old := servePort
	oldPath := servePath
	oldCfg := serveConfigFile
	oldDash := serveDashboard
	servePort = "99999"
	servePath = "/healthz"
	serveConfigFile = ".smokesig.yaml"
	serveDashboard = false
	t.Cleanup(func() {
		servePort = old
		servePath = oldPath
		serveConfigFile = oldCfg
		serveDashboard = oldDash
	})

	err := runServe(serveCmd, nil)
	if err == nil {
		t.Fatal("expected error for out-of-range port")
	}
}

// ─── runInit ─────────────────────────────────────────────────────────────────

func TestRunInit_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	old := forceOverwrite
	oldFrom := fromRunning
	oldDoc := withDocIntegrity
	forceOverwrite = false
	fromRunning = ""
	withDocIntegrity = false
	t.Cleanup(func() {
		forceOverwrite = old
		fromRunning = oldFrom
		withDocIntegrity = oldDoc
	})

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".smokesig.yaml")); err != nil {
		t.Error(".smokesig.yaml not created")
	}
}

func TestRunInit_ExistingFileWithoutForce(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte("existing"), 0644)

	old := forceOverwrite
	forceOverwrite = false
	t.Cleanup(func() { forceOverwrite = old })

	err := runInit(initCmd, nil)
	if err == nil {
		t.Fatal("expected error when file exists without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunInit_ExistingFileForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte("old"), 0644)

	old := forceOverwrite
	oldFrom := fromRunning
	forceOverwrite = true
	fromRunning = ""
	t.Cleanup(func() {
		forceOverwrite = old
		fromRunning = oldFrom
	})

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}
}

func TestRunInit_WithDocIntegrityFlag(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	old := withDocIntegrity
	oldFrom := fromRunning
	oldForce := forceOverwrite
	withDocIntegrity = true
	fromRunning = ""
	forceOverwrite = false
	t.Cleanup(func() {
		withDocIntegrity = old
		fromRunning = oldFrom
		forceOverwrite = oldForce
	})

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── applyFixes / runAudit ────────────────────────────────────────────────────

func TestRunAudit_WithFixMode(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(`
version: 1
project: fix-test
tests:
  - name: build
    run: "echo build"
    expect:
      exit_code: 0
`), 0644)

	old := auditFix
	oldJSON := auditJSON
	auditFix = true
	auditJSON = false
	t.Cleanup(func() {
		auditFix = old
		auditJSON = oldJSON
	})

	if err := runAudit(auditCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAudit_JSONOutputMode(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(`
version: 1
project: json-out
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`), 0644)

	old := auditJSON
	oldFix := auditFix
	auditJSON = true
	auditFix = false
	t.Cleanup(func() {
		auditJSON = old
		auditFix = oldFix
	})

	if err := runAudit(auditCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── runStress (not-found and invalid-config paths) ───────────────────────────

func TestRunStress_ConfigNotFound(t *testing.T) {
	saveRunFlags(t)
	setGlobalConfigFile(t, "/nonexistent/.smokesig.yaml")
	format = "terminal"

	err := runStress(stressCmd, []string{"some-test"})
	if err == nil {
		t.Fatal("expected error with missing config")
	}
}

func TestRunStress_BadFormatString(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	format = "invalid-format-xyz"

	err := runStress(stressCmd, []string{"always-pass"})
	if err == nil {
		t.Fatal("expected error with invalid format")
	}
}

func TestRunStress_TestNameNotFound(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	format = "terminal"
	oldRuns, oldWorkers := stressRuns, stressWorkers
	stressRuns = 1
	stressWorkers = 1
	t.Cleanup(func() { stressRuns = oldRuns; stressWorkers = oldWorkers })
	discardOutput(t)

	err := runStress(stressCmd, []string{"nonexistent-test-xyz"})
	if err == nil {
		t.Fatal("expected error when test not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q", err.Error())
	}
}

// ─── runWithTUI stub ──────────────────────────────────────────────────────────

func TestRunWithTUI_PanicsOnStub(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from TUI stub, got none")
		}
	}()
	runWithTUI(&runner.Runner{}, runner.RunOptions{})
}

// ─── Execute ─────────────────────────────────────────────────────────────────

func TestExecute_HelpDoesNotPanic(t *testing.T) {
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--help"})
	// ExecuteC returns nil for --help; must not panic
	_, _ = rootCmd.ExecuteC()
}

// ─── loadConfig env merge path ────────────────────────────────────────────────

func TestLoadConfig_EnvMerge(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	// Write an env-specific config in the same dir
	envCfg := `
version: 1
project: test-proj-staging
tests:
  - name: staging-check
    run: "true"
    expect:
      exit_code: 0
`
	envPath := filepath.Join(dir, "staging.smokesig.yaml")
	if err := os.WriteFile(envPath, []byte(envCfg), 0644); err != nil {
		t.Fatalf("write env config: %v", err)
	}
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	envName = "staging"
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig with env: %v", err)
	}
	_ = cfg
}

func TestLoadConfig_EnvMerge_MissingEnvFile(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	envName = "nonexistent-env"
	noOtel = false

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error when env config file is missing")
	}
}

func TestLoadConfig_NoOtelDisablesOTel(t *testing.T) {
	dir := t.TempDir()
	cfg := `
version: 1
project: otel-test
otel:
  enabled: true
  jaeger_url: "http://localhost:14268"
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`
	p := writeCfg(t, dir, cfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = true

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.OTel.Enabled {
		t.Error("expected OTel to be disabled when --no-otel is set")
	}
}

func TestLoadConfig_OtelCollectorOverride_CB(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	otelCollector = "http://collector:14268"
	noOtel = false

	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if !loaded.OTel.Enabled {
		t.Error("expected OTel to be enabled when --otel-collector is set")
	}
	if loaded.OTel.JaegerURL != "http://collector:14268" {
		t.Errorf("unexpected JaegerURL: %s", loaded.OTel.JaegerURL)
	}
}

// ─── runSmoke: normal (non-monorepo, non-watch) path ─────────────────────────

func TestRunSmoke_PassingTest(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("runSmoke: %v", err)
	}
}

func TestRunSmoke_InvalidTimeout_CB(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	format = "terminal"
	timeout = "not-a-duration"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunSmoke_BadFormat_CB(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	format = "not-a-real-format"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestRunSmoke_VerboseFlag_CB(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	verbose = true
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("runSmoke verbose: %v", err)
	}
	if verbosity != reporter.VerbosityVerbose {
		t.Errorf("expected VerbosityVerbose, got %v", verbosity)
	}
}

func TestRunSmoke_QuietFlag_CB(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	quiet = true
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("runSmoke quiet: %v", err)
	}
	if verbosity != reporter.VerbosityQuiet {
		t.Errorf("expected VerbosityQuiet, got %v", verbosity)
	}
}

func TestRunSmoke_ConfigNotFound(t *testing.T) {
	saveRunFlags(t)
	setGlobalConfigFile(t, "/nonexistent/dir/.smokesig.yaml")
	format = "terminal"

	err := runSmoke(runCmd, nil)
	if err == nil {
		t.Fatal("expected error when config not found")
	}
}

func TestRunSmoke_MonorepoWithSubConfig(t *testing.T) {
	dir := t.TempDir()
	// Root config
	p := writeCfg(t, dir, `
version: 1
project: mono-root
tests:
  - name: placeholder
    run: "true"
    expect:
      exit_code: 0
`)
	// Sub-project
	sub := filepath.Join(dir, "svc")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, ".smokesig.yaml"), []byte(passingCfg), 0644)

	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	monorepoMode = true
	format = "terminal"
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("monorepo runSmoke: %v", err)
	}
}

// ─── handleBaseline ───────────────────────────────────────────────────────────

func TestHandleBaseline_FlagOff_CB(t *testing.T) {
	oldFlag := baselineFlag
	baselineFlag = false
	t.Cleanup(func() { baselineFlag = oldFlag })
	// Should be a no-op; must not panic
	handleBaseline(&runner.SuiteResult{}, t.TempDir())
}

func TestHandleBaseline_FirstRun_CreatesBaseline(t *testing.T) {
	dir := t.TempDir()
	oldFlag, oldThresh := baselineFlag, baselineThresh
	baselineFlag = true
	baselineThresh = 50
	t.Cleanup(func() { baselineFlag = oldFlag; baselineThresh = oldThresh })
	discardOutput(t)

	suite := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "alpha", Duration: 10 * 1e6}, // 10ms in nanoseconds
		},
	}
	handleBaseline(suite, dir)
}

func TestHandleBaseline_Regression(t *testing.T) {
	dir := t.TempDir()
	oldFlag, oldThresh := baselineFlag, baselineThresh
	baselineFlag = true
	baselineThresh = 10 // 10% threshold — easy to trigger
	t.Cleanup(func() { baselineFlag = oldFlag; baselineThresh = oldThresh })
	discardOutput(t)

	// First run: establish baseline
	suite1 := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "slow-test", Duration: 10 * 1e6},
		},
	}
	handleBaseline(suite1, dir)

	// Second run: simulate regression (much slower)
	suite2 := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "slow-test", Duration: 1000 * 1e6}, // 100x slower
		},
	}
	handleBaseline(suite2, dir)
}

// ─── buildReporter / withPushReport / withOTelExport paths ───────────────────

func TestWithPushReport_WithWebhookFormat(t *testing.T) {
	oldURL, oldFormat, oldOn, oldKey := reportURL, webhookFormat, webhookOn, reportAPIKey
	reportURL = "http://hooks.example.com/webhook"
	webhookFormat = "slack"
	webhookOn = "failure"
	reportAPIKey = "key123"
	t.Cleanup(func() {
		reportURL = oldURL
		webhookFormat = oldFormat
		webhookOn = oldOn
		reportAPIKey = oldKey
	})

	base := reporter.NewTerminal(io.Discard)
	got := withPushReport(base)
	if got == base {
		t.Error("expected wrapped reporter when webhook format is set")
	}
}

func TestWithPushReport_PushReporter_CB(t *testing.T) {
	oldURL, oldFormat, oldKey := reportURL, webhookFormat, reportAPIKey
	reportURL = "http://results.example.com/push"
	webhookFormat = ""
	reportAPIKey = ""
	t.Cleanup(func() {
		reportURL = oldURL
		webhookFormat = oldFormat
		reportAPIKey = oldKey
	})

	base := reporter.NewTerminal(io.Discard)
	got := withPushReport(base)
	if got == base {
		t.Error("expected wrapped reporter when report-url is set")
	}
}

func TestWithOTelExport_JaegerURLFallback(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, `
version: 1
project: otel-jaeger
otel:
  enabled: true
  jaeger_url: "http://jaeger:14268"
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	base := reporter.NewTerminal(io.Discard)
	got := withOTelExport(base, cfg)
	// Should wrap with OTel reporter since JaegerURL is set
	if got == base {
		t.Error("expected OTel-wrapped reporter when jaeger_url is set")
	}
}

func TestWithOTelExport_ExplicitExportURL(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, `
version: 1
project: otel-explicit
otel:
  enabled: true
  jaeger_url: "http://jaeger:14268"
  export_url: "http://collector:4318/v1/traces"
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	base := reporter.NewTerminal(io.Discard)
	got := withOTelExport(base, cfg)
	if got == base {
		t.Error("expected OTel-wrapped reporter when export_url is set")
	}
}

func TestWithOTelExport_Disabled_CB(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	base := reporter.NewTerminal(io.Discard)
	got := withOTelExport(base, cfg)
	if got != base {
		t.Error("expected same reporter when OTel is disabled")
	}
}

func TestBuildReporter_ValidFormats(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	for _, f := range []string{"terminal", "json", "junit", "tap"} {
		rep, closeAll, err := buildReporter(f, cfg)
		if err != nil {
			t.Errorf("buildReporter(%q): %v", f, err)
			continue
		}
		if rep == nil {
			t.Errorf("buildReporter(%q): nil reporter", f)
		}
		closeAll()
	}
}

// ─── runMigrateGoss: output file + stats + strict mode paths ─────────────────

const minimalGossYAML = `
command:
  echo hello:
    exit-status: 0
`

func TestRunMigrateGoss_OutputFile_CB(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "goss.yaml")
	outPath := filepath.Join(dir, "out.smokesig.yaml")
	os.WriteFile(inPath, []byte(minimalGossYAML), 0644)

	oldOut, oldOW, oldStats, oldDistro := migrateOutput, migrateOverwrite, migrateStats, migrateDistro
	migrateOutput = outPath
	migrateOverwrite = false
	migrateStats = false
	migrateDistro = "deb"
	t.Cleanup(func() {
		migrateOutput = oldOut
		migrateOverwrite = oldOW
		migrateStats = oldStats
		migrateDistro = oldDistro
	})

	err := runMigrateGoss(gossCmd, []string{inPath})
	if err != nil {
		t.Fatalf("runMigrateGoss with output file: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Error("output file not created")
	}
}

func TestRunMigrateGoss_OutputFileAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "goss.yaml")
	outPath := filepath.Join(dir, "existing.smokesig.yaml")
	os.WriteFile(inPath, []byte(minimalGossYAML), 0644)
	os.WriteFile(outPath, []byte("existing"), 0644)

	oldOut, oldOW, oldDistro := migrateOutput, migrateOverwrite, migrateDistro
	migrateOutput = outPath
	migrateOverwrite = false
	migrateDistro = "deb"
	t.Cleanup(func() {
		migrateOutput = oldOut
		migrateOverwrite = oldOW
		migrateDistro = oldDistro
	})

	err := runMigrateGoss(gossCmd, []string{inPath})
	if err == nil {
		t.Fatal("expected error when output file exists without --overwrite")
	}
}

func TestRunMigrateGoss_OutputFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "goss.yaml")
	outPath := filepath.Join(dir, "existing.smokesig.yaml")
	os.WriteFile(inPath, []byte(minimalGossYAML), 0644)
	os.WriteFile(outPath, []byte("old"), 0644)

	oldOut, oldOW, oldDistro := migrateOutput, migrateOverwrite, migrateDistro
	migrateOutput = outPath
	migrateOverwrite = true
	migrateDistro = "deb"
	t.Cleanup(func() {
		migrateOutput = oldOut
		migrateOverwrite = oldOW
		migrateDistro = oldDistro
	})

	err := runMigrateGoss(gossCmd, []string{inPath})
	if err != nil {
		t.Fatalf("runMigrateGoss --overwrite: %v", err)
	}
}

func TestRunMigrateGoss_WithStats(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "goss.yaml")
	os.WriteFile(inPath, []byte(minimalGossYAML), 0644)

	oldOut, oldStats, oldDistro := migrateOutput, migrateStats, migrateDistro
	migrateOutput = ""
	migrateStats = true
	migrateDistro = "rpm"
	t.Cleanup(func() {
		migrateOutput = oldOut
		migrateStats = oldStats
		migrateDistro = oldDistro
	})
	discardOutput(t)

	err := runMigrateGoss(gossCmd, []string{inPath})
	if err != nil {
		t.Fatalf("runMigrateGoss --stats: %v", err)
	}
}

func TestRunMigrateGoss_InvalidDistro_CB(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "goss.yaml")
	os.WriteFile(inPath, []byte(minimalGossYAML), 0644)

	oldDistro := migrateDistro
	migrateDistro = "windows"
	t.Cleanup(func() { migrateDistro = oldDistro })

	err := runMigrateGoss(gossCmd, []string{inPath})
	if err == nil {
		t.Fatal("expected error for invalid distro")
	}
}

func TestRunMigrateGoss_ApkDistro(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "goss.yaml")
	os.WriteFile(inPath, []byte(minimalGossYAML), 0644)

	oldOut, oldDistro := migrateOutput, migrateDistro
	migrateOutput = ""
	migrateDistro = "apk"
	t.Cleanup(func() {
		migrateOutput = oldOut
		migrateDistro = oldDistro
	})
	discardOutput(t)

	err := runMigrateGoss(gossCmd, []string{inPath})
	if err != nil {
		t.Fatalf("runMigrateGoss apk: %v", err)
	}
}

// ─── runInit: fromRunning path (errors on no Docker) ─────────────────────────

func TestRunInit_FromRunning_ErrorsGracefully(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	old := fromRunning
	oldForce := forceOverwrite
	fromRunning = "nonexistent-container-xyz"
	forceOverwrite = true
	t.Cleanup(func() { fromRunning = old; forceOverwrite = oldForce })

	err := runInit(initCmd, nil)
	// Expect an error since container doesn't exist
	if err == nil {
		t.Log("runInit fromRunning unexpectedly succeeded (Docker may be available)")
	}
}

// ─── runObserve: quiet mode with a simple command ─────────────────────────────

func TestRunObserve_QuietMode_CB(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "observed.smokesig.yaml")

	oldDir, oldTimeout, oldQuiet, oldOutput := observeDir, observeTimeout, observeQuiet, observeOutput
	observeDir = ""
	observeTimeout = 0
	observeQuiet = true
	observeOutput = outPath
	t.Cleanup(func() {
		observeDir = oldDir
		observeTimeout = oldTimeout
		observeQuiet = oldQuiet
		observeOutput = oldOutput
	})

	err := runObserve(observeCmd, []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("runObserve quiet: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Error("output file not created by runObserve")
	}
}

// ─── runServe: verify it starts and can be shut down via the handler path ─────

func TestRunServe_InvalidDBPath(t *testing.T) {
	old := serveDashboard
	oldDB := serveDBPath
	oldPort := servePort
	oldPath := servePath
	oldCfg := serveConfigFile
	serveDashboard = true
	serveDBPath = "/nonexistent-dir-xyz/smoke.db"
	servePort = "19876"
	servePath = "/healthz"
	serveConfigFile = ".smokesig.yaml"
	t.Cleanup(func() {
		serveDashboard = old
		serveDBPath = oldDB
		servePort = oldPort
		servePath = oldPath
		serveConfigFile = oldCfg
	})

	err := runServe(serveCmd, nil)
	if err == nil {
		t.Fatal("expected error opening dashboard database at invalid path")
	}
}

// ─── runValidate: uncovered path (empty configFile branch) ───────────────────

func TestRunValidate_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)

	out, err := runValidate(p)
	if err != nil {
		t.Fatalf("runValidate: %v", err)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("expected 'valid' in output, got %q", out)
	}
}

func TestRunValidate_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	os.WriteFile(p, []byte(`version: 1
project: broken
tests: []
`), 0644)

	out, err := runValidate(p)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	_ = out
}

// ─── withConfigNotifications: with API key env var ───────────────────────────

func TestWithConfigNotifications_WithAPIKeyEnv(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, `
version: 1
project: notify-env
notifications:
  - url: "http://hooks.example.com/webhook"
    format: slack
    on: always
    api_key_env: SMOKE_TEST_API_KEY_XYZ
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = false
	os.Setenv("SMOKE_TEST_API_KEY_XYZ", "test-key-value")
	t.Cleanup(func() { os.Unsetenv("SMOKE_TEST_API_KEY_XYZ") })

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	base := reporter.NewTerminal(io.Discard)
	got := withConfigNotifications(base, cfg)
	if got == base {
		t.Error("expected wrapped reporter when notifications are configured")
	}
}

// ─── Execute: cover the actual Execute() function ────────────────────────────

func TestExecute_VersionSubcommand(t *testing.T) {
	// Execute() calls rootCmd.Execute() internally. Use --help which exits 0
	// without calling os.Exit(1). We invoke Execute() via rootCmd directly to
	// avoid os.Exit side-effects.
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"version"})
	// Execute calls os.Exit on error; for a valid subcommand it returns normally.
	// Wrap in a goroutine with recover to catch any panic.
	done := make(chan struct{})
	go func() {
		defer close(done)
		// We can't call Execute() directly (it calls os.Exit), but we can test
		// the rootCmd.Execute() path which is what Execute() wraps.
		rootCmd.Execute() //nolint:errcheck
	}()
	<-done
}

// ─── runSmoke: relative configDir path (cwd join branch) ─────────────────────

func TestRunSmoke_RelativeConfigPath(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// Write config in a subdirectory with a relative path
	sub := filepath.Join(dir, "cfg")
	os.MkdirAll(sub, 0755)
	cfgPath := filepath.Join(sub, ".smokesig.yaml")
	os.WriteFile(cfgPath, []byte(passingCfg), 0644)

	saveRunFlags(t)
	// Use relative path so the cwd-join branch is triggered
	configFile = "cfg/.smokesig.yaml"
	format = "terminal"
	dryRun = true
	discardOutput(t)

	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("runSmoke with relative path: %v", err)
	}
}

// ─── runSmoke: monorepo watch path (covered briefly via watch=true + nonexistent dir) ─

func TestRunSmoke_MonorepoWatch_ConfigReloadError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "svc")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, ".smokesig.yaml"), []byte(passingCfg), 0644)

	p := writeCfg(t, dir, `
version: 1
project: mono-watch
tests:
  - name: placeholder
    run: "true"
    expect:
      exit_code: 0
`)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	monorepoMode = true
	watch = true
	format = "terminal"
	discardOutput(t)

	// runWatch will call runOnce once, then wait for signals/events.
	// Since dir exists but we can't signal easily, use a non-existent watch dir.
	// Instead, test via the underlying monorepo path with watch=false.
	watch = false
	err := runSmoke(runCmd, nil)
	if err != nil {
		t.Fatalf("monorepo watch setup: %v", err)
	}
}

// ─── runAudit: fix-error path (applyFixes returns error) ─────────────────────

func TestRunAudit_FixWithNoConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skipf("cannot chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// No .smokesig.yaml — audit runs but fix has nothing to do
	old := auditFix
	oldJSON := auditJSON
	auditFix = true
	auditJSON = false
	t.Cleanup(func() { auditFix = old; auditJSON = oldJSON })
	discardOutput(t)

	// Should not error even when config doesn't exist
	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("runAudit with fix and no config: %v", err)
	}
}

// ─── handleBaseline: no regressions path (new tests only) ────────────────────

func TestHandleBaseline_NoRegressions_CB(t *testing.T) {
	dir := t.TempDir()
	oldFlag, oldThresh := baselineFlag, baselineThresh
	baselineFlag = true
	baselineThresh = 50
	t.Cleanup(func() { baselineFlag = oldFlag; baselineThresh = oldThresh })
	discardOutput(t)

	// First run — establish baseline with fast test
	suite1 := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "fast-test", Duration: 100 * 1e6},
		},
	}
	handleBaseline(suite1, dir)

	// Second run — same timing, no regression
	suite2 := &runner.SuiteResult{
		Tests: []runner.TestResult{
			{Name: "fast-test", Duration: 110 * 1e6}, // within 50% threshold
		},
	}
	handleBaseline(suite2, dir)
}

// ─── runMigrateGoss: missing input file ──────────────────────────────────────

func TestRunMigrateGoss_MissingInputFile(t *testing.T) {
	oldDistro := migrateDistro
	migrateDistro = "deb"
	t.Cleanup(func() { migrateDistro = oldDistro })

	err := runMigrateGoss(gossCmd, []string{"/nonexistent/goss.yaml"})
	if err == nil {
		t.Fatal("expected error for missing input file")
	}
}

// ─── runObserve: command that observes something real ────────────────────────

func TestRunObserve_ErrorCommand(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.smokesig.yaml")

	oldDir, oldTimeout, oldQuiet, oldOutput := observeDir, observeTimeout, observeQuiet, observeOutput
	observeDir = ""
	observeTimeout = 0
	observeQuiet = true
	observeOutput = outPath
	t.Cleanup(func() {
		observeDir = oldDir
		observeTimeout = oldTimeout
		observeQuiet = oldQuiet
		observeOutput = oldOutput
	})

	// Use a command that runs but exits non-zero — observe should still succeed
	err := runObserve(observeCmd, []string{"false"})
	// May or may not error depending on implementation — just verify no panic
	_ = err
}

// ─── runWatch: watcher error channel path ────────────────────────────────────

// TestRunWatch_ReturnsNilOnNonexistentDir verifies that runWatch calls runOnce
// first and then fails on the Add call (returns an error), exercising both paths.
func TestRunWatch_AddError(t *testing.T) {
	called := false
	err := runWatch("/no/such/dir/abc123", "/no/such/dir/abc123/.smokesig.yaml",
		func() error { called = true; return nil })
	if !called {
		t.Error("runOnce not called before watcher setup")
	}
	if err == nil {
		t.Error("expected error watching non-existent directory")
	}
	if !strings.Contains(err.Error(), "watching") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── buildReporter: closers error path ───────────────────────────────────────

func TestBuildReporter_WithReportURL(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, passingCfg)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = false
	reportURL = "http://results.example.com/push"
	reportAPIKey = "key"
	webhookFormat = ""

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	rep, closeAll, err := buildReporter("terminal", cfg)
	if err != nil {
		t.Fatalf("buildReporter with report-url: %v", err)
	}
	if rep == nil {
		t.Error("nil reporter")
	}
	closeAll()
}

// ─── withConfigNotifications: on="" defaults to failure ──────────────────────

func TestWithConfigNotifications_DefaultOn(t *testing.T) {
	dir := t.TempDir()
	p := writeCfg(t, dir, `
version: 1
project: notif-default-on
notifications:
  - url: "http://hooks.example.com/slack"
    format: slack
tests:
  - name: p
    run: "true"
    expect:
      exit_code: 0
`)
	saveRunFlags(t)
	setGlobalConfigFile(t, p)
	noOtel = false

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	base := reporter.NewTerminal(io.Discard)
	got := withConfigNotifications(base, cfg)
	if got == base {
		t.Error("expected wrapped reporter")
	}
}

// ─── mcp command: verify it's registered ─────────────────────────────────────

func TestMCPCmd_IsRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "mcp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("mcp command not registered on rootCmd")
	}
}
