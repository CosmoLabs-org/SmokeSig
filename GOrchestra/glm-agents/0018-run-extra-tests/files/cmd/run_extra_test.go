//go:build ignore
package cmd

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// writeRunConfig writes a YAML config to a temp dir and returns the dir path.
func writeRunConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.smoke.yaml", []byte(content), 0644); err != nil {
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
	cfg, err := schema.Load(dir + "/.smoke.yaml")
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
	cfg, err := schema.Load(dir + "/.smoke.yaml")
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
	cfg, err := schema.Load(dir + "/.smoke.yaml")
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
	cfg, err := schema.Load(dir + "/.smoke.yaml")
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
	cfg, err := schema.Load(dir + "/.smoke.yaml")
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
