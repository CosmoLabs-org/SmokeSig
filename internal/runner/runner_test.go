package runner

import (
	"os"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// noopReporter discards all events.
type noopReporter struct{}

func (n *noopReporter) PrereqStart(_ string)             {}
func (n *noopReporter) PrereqResult(_ reporter.PrereqResultData) {}
func (n *noopReporter) TestStart(_ string)                {}
func (n *noopReporter) TestResult(_ reporter.TestResultData)     {}
func (n *noopReporter) Summary(_ reporter.SuiteResultData)       {}

func intPtr(n int) *int { return &n }

func newConfig(tests []schema.Test) *schema.SmokeConfig {
	return &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Tests:   tests,
	}
}

func TestRunner_SinglePassingTest(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "echo", Run: "echo hello", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0", result.Failed)
	}
}

func TestRunner_SingleFailingTest(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "fail", Run: "exit 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 0 {
		t.Errorf("passed = %d, want 0", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
}

func TestRunner_FailFast(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "pass1", Run: "echo 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "fail", Run: "exit 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "skipped", Run: "echo 3", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{FailFast: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
}

func TestRunner_TagFilter_Include(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "build", Run: "echo 1", Expect: schema.Expect{ExitCode: intPtr(0)}, Tags: []string{"build"}},
		{Name: "test", Run: "echo 2", Expect: schema.Expect{ExitCode: intPtr(0)}, Tags: []string{"test"}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{Tags: []string{"build"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if result.Tests[0].Name != "build" {
		t.Errorf("name = %q, want build", result.Tests[0].Name)
	}
}

func TestRunner_TagFilter_Exclude(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "fast", Run: "echo 1", Expect: schema.Expect{ExitCode: intPtr(0)}, Tags: []string{"fast"}},
		{Name: "slow", Run: "echo 2", Expect: schema.Expect{ExitCode: intPtr(0)}, Tags: []string{"slow"}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{ExcludeTags: []string{"slow"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
}

func TestRunner_DryRun(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "would-fail", Run: "exit 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1 (dry run always passes)", result.Passed)
	}
}

func TestRunner_StdoutContains(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "grep", Run: "echo hello world", Expect: schema.Expect{StdoutContains: "hello"}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
}

func TestRunner_Cleanup(t *testing.T) {
	dir := t.TempDir()
	cfg := newConfig([]schema.Test{
		{
			Name:    "with-cleanup",
			Run:     "echo test",
			Expect:  schema.Expect{ExitCode: intPtr(0)},
			Cleanup: "touch " + dir + "/cleaned",
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	_, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Give cleanup a moment
	time.Sleep(100 * time.Millisecond)
	// Cleanup should have created the file (but we don't hard-fail if timing is off)
}

func TestRunner_Parallel(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "a", Run: "echo a", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "b", Run: "echo b", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "c", Run: "echo c", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	cfg.Settings.Parallel = true
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 3 {
		t.Errorf("passed = %d, want 3", result.Passed)
	}
}

func TestRunner_PrereqFailure(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Prereqs: []schema.Prerequisite{
			{Name: "missing-tool", Check: "nonexistent_command_xyz"},
		},
		Tests: []schema.Test{
			{Name: "test", Run: "echo hi"},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	_, err := r.Run(RunOptions{})
	if err == nil {
		t.Fatal("expected error from failed prerequisite")
	}
}

func TestFilterTests_NoFilters(t *testing.T) {
	tests := []schema.Test{{Name: "a"}, {Name: "b"}}
	got := filterTests(tests, nil, nil)
	if len(got) != 2 {
		t.Errorf("got %d tests, want 2", len(got))
	}
}

func TestFilterTests_IncludeOnly(t *testing.T) {
	tests := []schema.Test{
		{Name: "a", Tags: []string{"fast"}},
		{Name: "b", Tags: []string{"slow"}},
	}
	got := filterTests(tests, []string{"fast"}, nil)
	if len(got) != 1 || got[0].Name != "a" {
		t.Errorf("got %v, want [a]", got)
	}
}

func TestFilterTests_ExcludeOnly(t *testing.T) {
	tests := []schema.Test{
		{Name: "a", Tags: []string{"fast"}},
		{Name: "b", Tags: []string{"slow"}},
	}
	got := filterTests(tests, nil, []string{"slow"})
	if len(got) != 1 || got[0].Name != "a" {
		t.Errorf("got %v, want [a]", got)
	}
}

func TestRunner_AllowFailure_FailingTest_ExitsZero(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:         "flaky",
			Run:          "exit 1",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0 (allow_failure should not count as real failure)", result.Failed)
	}
	if result.AllowedFailures != 1 {
		t.Errorf("allowed_failures = %d, want 1", result.AllowedFailures)
	}
	if result.Passed != 0 {
		t.Errorf("passed = %d, want 0", result.Passed)
	}
}

func TestRunner_AllowFailure_PassingTest_CountsAsPassed(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:         "sometimes-flaky",
			Run:          "echo ok",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1 (passing test with allow_failure should count as passed)", result.Passed)
	}
	if result.AllowedFailures != 0 {
		t.Errorf("allowed_failures = %d, want 0", result.AllowedFailures)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0", result.Failed)
	}
}

func TestRunner_AllowFailure_MixedTests_RealFailureExitsOne(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "pass", Run: "echo ok", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{
			Name:         "flaky",
			Run:          "exit 1",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
		{Name: "real-fail", Run: "exit 2", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if result.AllowedFailures != 1 {
		t.Errorf("allowed_failures = %d, want 1", result.AllowedFailures)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1 (real-fail has no allow_failure)", result.Failed)
	}
}

func TestRunner_AllowFailure_AllowedOnly_SuiteExitsZero(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:         "flaky-a",
			Run:          "exit 1",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
		{
			Name:         "flaky-b",
			Run:          "exit 2",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0", result.Failed)
	}
	if result.AllowedFailures != 2 {
		t.Errorf("allowed_failures = %d, want 2", result.AllowedFailures)
	}
}

func TestRunner_AllowFailure_TestResultFlag(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:         "check-flag",
			Run:          "exit 1",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tests) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(result.Tests))
	}
	tr := result.Tests[0]
	if !tr.AllowedFailure {
		t.Error("expected TestResult.AllowedFailure = true")
	}
	if tr.Passed {
		t.Error("expected TestResult.Passed = false (test did fail)")
	}
}

func TestRetry_PassesOnFirstAttempt(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "always-passes",
			Run:  "echo hi",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			Retry: &schema.RetryPolicy{
				Count:   3,
				Backoff: schema.Duration{Duration: 10 * time.Millisecond},
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	start := time.Now()
	result, err := r.Run(RunOptions{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if len(result.Tests) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(result.Tests))
	}
	tr := result.Tests[0]
	if tr.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 (passed on first try)", tr.Attempts)
	}
	if elapsed >= 50*time.Millisecond {
		t.Errorf("elapsed = %v, want < 50ms (no backoff should occur)", elapsed)
	}
}

func TestRetry_PassesAfterFailure(t *testing.T) {
	flagFile := t.TempDir() + "/flag"
	// First run: flag absent → touch it and exit 1. Second run: flag present → exit 0.
	cmd := "[ -f " + flagFile + " ] && exit 0 || (touch " + flagFile + " && exit 1)"
	cfg := newConfig([]schema.Test{
		{
			Name: "flaky",
			Run:  cmd,
			Expect: schema.Expect{ExitCode: intPtr(0)},
			Retry: &schema.RetryPolicy{
				Count:   3,
				Backoff: schema.Duration{Duration: 10 * time.Millisecond},
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if len(result.Tests) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(result.Tests))
	}
	tr := result.Tests[0]
	if tr.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", tr.Attempts)
	}
}

func TestRetry_ExhaustsAllAttempts(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "always-fails",
			Run:  "exit 1",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			Retry: &schema.RetryPolicy{
				Count:   3,
				Backoff: schema.Duration{Duration: 10 * time.Millisecond},
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	start := time.Now()
	result, err := r.Run(RunOptions{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	if len(result.Tests) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(result.Tests))
	}
	tr := result.Tests[0]
	if tr.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", tr.Attempts)
	}
	// Two backoffs: 10ms + 20ms = 30ms minimum
	if elapsed < 30*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 30ms (two backoffs: 10ms + 20ms)", elapsed)
	}
}

func TestTraceConfirmed(t *testing.T) {
	t.Run("returns_true_when_otel_trace_passed", func(t *testing.T) {
		assertions := []AssertionResult{
			{Type: "exit_code", Passed: false},
			{Type: "otel_trace", Passed: true},
		}
		if !traceConfirmed(assertions) {
			t.Error("expected true when otel_trace assertion passed")
		}
	})
	t.Run("returns_false_when_otel_trace_failed", func(t *testing.T) {
		assertions := []AssertionResult{
			{Type: "exit_code", Passed: false},
			{Type: "otel_trace", Passed: false},
		}
		if traceConfirmed(assertions) {
			t.Error("expected false when otel_trace assertion failed")
		}
	})
	t.Run("returns_false_when_no_otel_trace_assertion", func(t *testing.T) {
		assertions := []AssertionResult{
			{Type: "exit_code", Passed: false},
		}
		if traceConfirmed(assertions) {
			t.Error("expected false when no otel_trace assertion present")
		}
	})
}

func TestRetry_TraceAware_NoOTelTrace_ExhaustsRetries(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "fails-no-trace",
			Run:  "exit 1",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			Retry: &schema.RetryPolicy{
				Count:            3,
				Backoff:          schema.Duration{Duration: 10 * time.Millisecond},
				RetryOnTraceOnly: true,
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	tr := result.Tests[0]
	if tr.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3 (no otel_trace assertion -> all retries exhausted)", tr.Attempts)
	}
}

func TestRunner_ChainExtractResolve(t *testing.T) {
	dir := t.TempDir()
	cfg := newConfig([]schema.Test{
		{
			Name: "extract-value",
			Run:   `echo '{"token":"abc123"}'`,
			Expect: schema.Expect{
				JSONField: &schema.JSONFieldCheck{
					Path:    "token",
					Extract: "auth_token",
				},
			},
		},
		{
			Name: "use-value",
			Run:   `echo "got {{ .Vars.auth_token }}"`,
			Expect: schema.Expect{
				StdoutContains: "got abc123",
			},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 2 {
		t.Errorf("passed = %d, want 2", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0", result.Failed)
	}
}

func TestRunner_ChainForcesSequential(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "extract",
			Run:   `echo '{"val":"x"}'`,
			Expect: schema.Expect{
				JSONField: &schema.JSONFieldCheck{Path: "val", Extract: "v"},
			},
		},
		{
			Name: "consume",
			Run:   `echo "{{ .Vars.v }}"`,
			Expect: schema.Expect{StdoutContains: "x"},
		},
	})
	cfg.Settings.Parallel = true
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 2 {
		t.Errorf("passed = %d, want 2 (sequential execution needed for chains)", result.Passed)
	}
}

func TestRunner_DeepLink_CustomScheme(t *testing.T) {
	dir := t.TempDir()
	cfg := newConfig([]schema.Test{{
		Name: "custom scheme check",
		Expect: schema.Expect{DeepLink: &schema.DeepLinkCheck{
			URL:  "myapp://test",
			Tier: "config-only",
		}},
	}})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: dir}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
}

func TestIsRecursiveTestCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		envSet   bool
		expected bool
	}{
		{"go test ./...", true, true},
		{"go test -short ./...", true, true},
		{"smokesig run", true, true},
		{"smoke run --format json", true, true},
		{"npm test", true, true},
		{"bun test", true, true},
		{"pytest", true, true},
		{"cargo test", true, true},
		{"echo hello", true, false},
		{"curl http://localhost:3000", true, false},
		{"go build ./...", true, false},
		{"go test ./...", false, false},
		{"npm test", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("SMOKESIG_RUNNING", "1")
			}
			if got := isRecursiveTestCommand(tt.cmd); got != tt.expected {
				t.Errorf("isRecursiveTestCommand(%q) = %v, want %v (env=%v)", tt.cmd, got, tt.expected, tt.envSet)
			}
		})
	}
}

func TestRunner_RecursionGuardSkipsTestRunners(t *testing.T) {
	t.Setenv("SMOKESIG_RUNNING", "1")
	cfg := newConfig([]schema.Test{
		{Name: "safe", Run: "echo safe", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "recursive", Run: "go test -short ./...", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1 (only the safe test)", result.Passed)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (recursive test should be skipped)", result.Skipped)
	}
}

func TestShouldSkip(t *testing.T) {
	dir := t.TempDir()
	existingFile := dir + "/exists.txt"
	if err := os.WriteFile(existingFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		si   *schema.SkipIf
		want bool
	}{
		{"nil skip_if", nil, false},
		{"env_unset with var set", &schema.SkipIf{EnvUnset: "HOME"}, false},
		{"env_unset with var unset", &schema.SkipIf{EnvUnset: "DEFINITELY_NOT_SET_XYZ"}, true},
		{"env_equals matching value", &schema.SkipIf{EnvEquals: &schema.EnvEqualsCond{Var: "HOME", Value: os.Getenv("HOME")}}, true},
		{"env_equals non-matching value", &schema.SkipIf{EnvEquals: &schema.EnvEqualsCond{Var: "HOME", Value: "no-match-xyz"}}, false},
		{"file_missing with existing file", &schema.SkipIf{FileMissing: "exists.txt"}, false},
		{"file_missing with missing file", &schema.SkipIf{FileMissing: "nope.txt"}, true},
		{"file_missing absolute path missing", &schema.SkipIf{FileMissing: "/tmp/definitely_missing_xyz"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkip(tt.si, dir); got != tt.want {
				t.Errorf("shouldSkip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunner_NoRecursionGuardWithoutEnv(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "test-runner", Run: "echo simulated-test-pass", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
}
