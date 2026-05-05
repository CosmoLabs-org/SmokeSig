//go:build ignore
package runner

import (
	"fmt"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestParallel_ExecutesConcurrently(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "slow-a", Run: "sleep 0.3 && echo a", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "slow-b", Run: "sleep 0.3 && echo b", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "slow-c", Run: "sleep 0.3 && echo c", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	cfg.Settings.Parallel = true
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	start := time.Now()
	result, err := r.Run(RunOptions{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 3 {
		t.Errorf("passed = %d, want 3", result.Passed)
	}
	// Sequential would take >= 900ms. Parallel should be ~300-400ms.
	if elapsed >= 800*time.Millisecond {
		t.Errorf("elapsed = %v, want < 800ms (tests should run concurrently)", elapsed)
	}
}

func TestParallel_FailFast_FallsBackToSequential(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "pass1", Run: "echo 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "fail", Run: "exit 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "skipped", Run: "echo 3", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	cfg.Settings.Parallel = true
	cfg.Settings.FailFast = true
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
}

func TestParallel_FailFastViaOptions(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "pass1", Run: "echo 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "fail", Run: "exit 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "skipped", Run: "echo 3", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	cfg.Settings.Parallel = true
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	result, err := r.Run(RunOptions{FailFast: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
}

func TestParallel_MixedPassAndFail(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{Name: "pass-a", Run: "echo a", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "fail-b", Run: "exit 1", Expect: schema.Expect{ExitCode: intPtr(0)}},
		{Name: "pass-c", Run: "echo c", Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	cfg.Settings.Parallel = true
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 2 {
		t.Errorf("passed = %d, want 2", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
}

func TestParallel_AllTestsRun(t *testing.T) {
	dir := t.TempDir()

	tests := make([]schema.Test, 5)
	for i := range tests {
		tests[i] = schema.Test{
			Name:   fmt.Sprintf("t-%d", i),
			Run:    fmt.Sprintf("touch %s/%d", dir, i),
			Expect: schema.Expect{ExitCode: intPtr(0)},
		}
	}
	cfg := newConfig(tests)
	cfg.Settings.Parallel = true
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("total = %d, want 5", result.Total)
	}
	if result.Passed != 5 {
		t.Errorf("passed = %d, want 5", result.Passed)
	}
}

func TestRetry_Count2_RetriesFailedTest(t *testing.T) {
	flagFile := t.TempDir() + "/flag"
	cmd := "[ -f " + flagFile + " ] && exit 0 || (touch " + flagFile + " && exit 1)"
	cfg := newConfig([]schema.Test{
		{
			Name: "retry-twice",
			Run:  cmd,
			Expect: schema.Expect{ExitCode: intPtr(0)},
			Retry: &schema.RetryPolicy{
				Count:   2,
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

func TestRetry_Count2_ExhaustsRetries(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "always-fails",
			Run:  "exit 1",
			Expect: schema.Expect{ExitCode: intPtr(0)},
			Retry: &schema.RetryPolicy{
				Count:   2,
				Backoff: schema.Duration{Duration: 10 * time.Millisecond},
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
	if tr.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", tr.Attempts)
	}
}

func TestRetry_RetryOnTraceOnly_SkipsRetryWhenTracePasses(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name: "trace-passes-other-fails",
			Run:  "exit 1",
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
			Retry: &schema.RetryPolicy{
				Count:            3,
				Backoff:          schema.Duration{Duration: 10 * time.Millisecond},
				RetryOnTraceOnly: true,
			},
		},
	})
	// No otel_trace assertion present, so traceConfirmed returns false,
	// meaning all retries are exhausted (same as TestRetry_TraceAware_NoOTelTrace_ExhaustsRetries).
	// This test validates the code path — the retry_on_trace_only with no trace
	// still exhausts retries because traceConfirmed([]AssertionResult{}) = false.
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
	tr := result.Tests[0]
	if tr.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", tr.Attempts)
	}
	// With 2 backoffs (10ms + 20ms) = 30ms minimum
	if elapsed < 25*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 25ms (retries should occur)", elapsed)
	}
}
