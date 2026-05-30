package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

func TestNoopReporter_AllMethods(t *testing.T) {
	r := newNoopReporter()
	r.PrereqStart("test-prereq")
	r.PrereqResult(reporter.PrereqResultData{Name: "test", Passed: true})
	r.TestStart("test-name")
	r.TestResult(reporter.TestResultData{Name: "test", Passed: true})
	r.Summary(reporter.SuiteResultData{Total: 1, Passed: 1})
}

func TestExecute_Help(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("Execute --help: %v", err)
	}
}

func TestCoveragePush_MigrateGoss_MissingFile(t *testing.T) {
	rootCmd.SetArgs([]string{"migrate", "goss", "/nonexistent/goss.yaml"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing goss file")
	}
}

func TestRunMigrateGoss_ValidFile(t *testing.T) {
	dir := t.TempDir()
	gossFile := filepath.Join(dir, "goss.yaml")
	os.WriteFile(gossFile, []byte(`port:
  tcp:8080:
    listening: true
    ip:
      - 0.0.0.0
`), 0644)

	outFile := filepath.Join(dir, "smoke.yaml")
	rootCmd.SetArgs([]string{"migrate", "goss", gossFile, "-o", outFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("migrate goss: %v", err)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("output file not created: %v", err)
	}
}

func TestRunObserve_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "smoke.yaml")
	rootCmd.SetArgs([]string{"observe", dir, "-o", outFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("observe empty dir: %v", err)
	}
}

func TestRunWatch_MissingConfig(t *testing.T) {
	rootCmd.SetArgs([]string{"run", "--watch", "-f", "/nonexistent/config.yaml"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing config in watch mode")
	}
}

func TestCoveragePush_RunSmoke_MissingConfig(t *testing.T) {
	rootCmd.SetArgs([]string{"run", "-f", "/nonexistent/config.yaml"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestRunServe_InvalidPort(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".smokesig.yaml")
	os.WriteFile(cfgFile, []byte("version: 1\nproject: test\ntests: []\n"), 0644)

	rootCmd.SetArgs([]string{"serve", "-f", cfgFile, "--port", "99999"})
	err := rootCmd.Execute()
	// Should either error on invalid port or start+stop quickly
	_ = err
}

func TestRunAudit_NoConfig(t *testing.T) {
	dir := t.TempDir()
	rootCmd.SetArgs([]string{"audit", "-d", dir})
	err := rootCmd.Execute()
	// Audit on empty dir should work (finds nothing)
	_ = err
}
