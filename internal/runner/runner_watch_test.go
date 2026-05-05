package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// TestWatch_PrereqExecutesCommands verifies that prerequisites run before tests.
func TestWatch_PrereqExecutesCommands(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Prereqs: []schema.Prerequisite{
			{Name: "check-echo", Check: "echo hello"},
			{Name: "check-true", Check: "true"},
		},
		Tests: []schema.Test{
			{Name: "basic", Run: "echo ok", Expect: schema.Expect{ExitCode: intPtr(0)}},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
}

// TestWatch_PrereqFailureBlocksTests verifies that a failing prerequisite prevents test execution.
func TestWatch_PrereqFailureBlocksTests(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Prereqs: []schema.Prerequisite{
			{Name: "must-pass", Check: "exit 1", Hint: "install this tool"},
		},
		Tests: []schema.Test{
			{Name: "never-runs", Run: "echo ok", Expect: schema.Expect{ExitCode: intPtr(0)}},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	_, err := r.Run(RunOptions{})
	if err == nil {
		t.Fatal("expected error from failed prerequisite")
	}
}

// TestWatch_CleanupRunsAfterTest verifies cleanup command executes after the test.
func TestWatch_CleanupRunsAfterTest(t *testing.T) {
	dir := t.TempDir()
	cleanupFile := filepath.Join(dir, "cleanup-marker")

	cfg := newConfig([]schema.Test{
		{
			Name:    "with-cleanup",
			Run:     "echo test",
			Expect:  schema.Expect{ExitCode: intPtr(0)},
			Cleanup: "touch " + cleanupFile,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}

	// Wait for cleanup to complete
	time.Sleep(200 * time.Millisecond)

	if _, err := os.Stat(cleanupFile); os.IsNotExist(err) {
		t.Error("cleanup file was not created — cleanup command did not run")
	}
}

// TestWatch_CleanupRunsEvenOnTestFailure verifies cleanup runs even when the test fails.
func TestWatch_CleanupRunsEvenOnTestFailure(t *testing.T) {
	dir := t.TempDir()
	cleanupFile := filepath.Join(dir, "cleanup-on-fail")

	cfg := newConfig([]schema.Test{
		{
			Name:    "fails-but-cleans",
			Run:     "exit 1",
			Expect:  schema.Expect{ExitCode: intPtr(0)},
			Cleanup: "touch " + cleanupFile,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}

	time.Sleep(200 * time.Millisecond)

	if _, err := os.Stat(cleanupFile); os.IsNotExist(err) {
		t.Error("cleanup should still run even when test fails")
	}
}

// TestWatch_DryRunOutputsPlanWithoutExecution verifies dry-run marks tests as passed without running them.
func TestWatch_DryRunOutputsPlanWithoutExecution(t *testing.T) {
	dir := t.TempDir()
	// This test would fail if actually executed (exit 1), but dry-run should pass
	cfg := newConfig([]schema.Test{
		{Name: "would-fail", Run: "exit 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "would-also-fail", Run: "exit 2", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 2 {
		t.Errorf("passed = %d, want 2 (dry run passes all)", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0 (dry run never fails)", result.Failed)
	}
}

// TestWatch_DryRunDoesNotExecuteCommands verifies dry-run doesn't create side effects.
func TestWatch_DryRunDoesNotExecuteCommands(t *testing.T) {
	dir := t.TempDir()
	sideEffectFile := filepath.Join(dir, "should-not-exist")

	cfg := newConfig([]schema.Test{
		{
			Name:   "no-side-effect",
			Run:    "touch " + sideEffectFile,
			Expect: schema.Expect{ExitCode: intPtr(0)},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	_, err := r.Run(RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(sideEffectFile); err == nil {
		t.Error("dry-run should not execute commands — side effect file was created")
	}
}

// TestWatch_SkipIfEnvUnset skips test when env var is not set.
func TestWatch_SkipIfEnvUnset(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:   "skippable",
			Run:    "echo should-not-run",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			SkipIf: &schema.SkipIf{EnvUnset: "COSMO_WATCH_SKIP_TEST_XYZ"},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (env var is unset)", result.Skipped)
	}
	if result.Passed != 0 {
		t.Errorf("passed = %d, want 0", result.Passed)
	}
}

// TestWatch_SkipIfEnvUnsetRunsWhenSet verifies test runs when skip_if env var IS set.
func TestWatch_SkipIfEnvUnsetRunsWhenSet(t *testing.T) {
	t.Setenv("COSMO_WATCH_SKIP_RUN_TEST", "1")

	cfg := newConfig([]schema.Test{
		{
			Name:   "runs-because-set",
			Run:    "echo ok",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			SkipIf: &schema.SkipIf{EnvUnset: "COSMO_WATCH_SKIP_RUN_TEST"},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1 (env var is set, test should run)", result.Passed)
	}
	if result.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", result.Skipped)
	}
}

// TestWatch_SkipIfFileMissing skips test when file does not exist.
func TestWatch_SkipIfFileMissing(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:   "needs-file",
			Run:    "echo should-not-run",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			SkipIf: &schema.SkipIf{FileMissing: "nonexistent-file.txt"},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (file is missing)", result.Skipped)
	}
}

// TestWatch_SkipIfFileMissingRunsWhenPresent verifies test runs when skip_if file exists.
func TestWatch_SkipIfFileMissingRunsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	existingFile := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(existingFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := newConfig([]schema.Test{
		{
			Name:   "runs-because-file-exists",
			Run:    "echo ok",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			SkipIf: &schema.SkipIf{FileMissing: "present.txt"},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1 (file exists, test should run)", result.Passed)
	}
	if result.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", result.Skipped)
	}
}

// TestWatch_SkipIfMixedConditions verifies mixed skip_if conditions with multiple tests.
func TestWatch_SkipIfMixedConditions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("x"), 0644)

	cfg := newConfig([]schema.Test{
		{
			Name:   "skipped-env",
			Run:    "echo no",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			SkipIf: &schema.SkipIf{EnvUnset: "COSMO_WATCH_MIXED_XYZ"},
		},
		{
			Name:   "skipped-file",
			Run:    "echo no",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			SkipIf: &schema.SkipIf{FileMissing: "missing.txt"},
		},
		{
			Name:   "runs-ok",
			Run:    "echo yes",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			SkipIf: &schema.SkipIf{FileMissing: "exists.txt"},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 2 {
		t.Errorf("skipped = %d, want 2", result.Skipped)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
}
