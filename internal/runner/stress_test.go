package runner

import (
	"testing"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/reporter"
	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

type nopReporter struct{}

func (nopReporter) PrereqStart(string)             {}
func (nopReporter) PrereqResult(reporter.PrereqResultData) {}
func (nopReporter) TestStart(string)               {}
func (nopReporter) TestResult(reporter.TestResultData)     {}
func (nopReporter) Summary(reporter.SuiteResultData)       {}

func TestDedupErrors_GroupsIdentical(t *testing.T) {
	errors := []string{
		"exit_code expected 0, got 1",
		"exit_code expected 0, got 1",
		"exit_code expected 0, got 1",
		"stdout: missing \"database connected\"",
	}
	grouped := DedupErrors(errors)
	if len(grouped) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(grouped))
	}
	if grouped[0].Count != 3 {
		t.Errorf("expected first group count 3, got %d", grouped[0].Count)
	}
	if grouped[0].Message != "exit_code expected 0, got 1" {
		t.Errorf("unexpected first group message: %s", grouped[0].Message)
	}
}

func TestDedupErrors_Empty(t *testing.T) {
	grouped := DedupErrors(nil)
	if len(grouped) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(grouped))
	}
}

func TestReliabilityStatus(t *testing.T) {
	tests := []struct {
		rate     float64
		expected string
	}{
		{100.0, "Stable"},
		{99.0, "Flaky"},
		{95.0, "Flaky"},
		{94.9, "Unreliable"},
		{50.0, "Unreliable"},
		{0.0, "Unreliable"},
	}
	for _, tt := range tests {
		got := ReliabilityStatus(tt.rate)
		if got != tt.expected {
			t.Errorf("ReliabilityStatus(%.1f) = %q, want %q", tt.rate, got, tt.expected)
		}
	}
}

func TestStressTest_AllPass(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "always-passes", Run: "true"},
			},
		},
		Reporter:  nopReporter{},
		ConfigDir: "",
	}
	result := r.StressTest("always-passes", 10, 1, false)
	if result.TotalRuns != 10 {
		t.Errorf("expected 10 runs, got %d", result.TotalRuns)
	}
	if result.Passes != 10 {
		t.Errorf("expected 10 passes, got %d", result.Passes)
	}
	if result.Failures != 0 {
		t.Errorf("expected 0 failures, got %d", result.Failures)
	}
	if result.PassRate != 100.0 {
		t.Errorf("expected 100%% pass rate, got %.1f%%", result.PassRate)
	}
	if result.Reliability != "Stable" {
		t.Errorf("expected Stable, got %s", result.Reliability)
	}
}

func TestStressTest_WithFailures(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "sometimes-fails", Run: "sh -c 'exit $((RANDOM % 3))'"},
			},
		},
		Reporter:  nopReporter{},
		ConfigDir: "",
	}
	result := r.StressTest("sometimes-fails", 20, 1, false)
	if result.TotalRuns != 20 {
		t.Errorf("expected 20 runs, got %d", result.TotalRuns)
	}
	if result.Passes+result.Failures != 20 {
		t.Errorf("passes(%d) + failures(%d) != 20", result.Passes, result.Failures)
	}
}

func TestStressTest_TestNotFound(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{Tests: nil},
	}
	result := r.StressTest("nonexistent", 5, 1, false)
	if result.TotalRuns != 0 {
		t.Errorf("expected 0 runs for missing test, got %d", result.TotalRuns)
	}
}

func TestStressTest_Concurrent(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "pass", Run: "true"},
			},
		},
		Reporter: nopReporter{},
	}
	result := r.StressTest("pass", 20, 5, false)
	if result.Concurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", result.Concurrency)
	}
	if result.Passes != 20 {
		t.Errorf("expected 20 passes, got %d", result.Passes)
	}
}

func TestStressTest_FailFast(t *testing.T) {
	exitCode := 0
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "always-fails", Run: "false", Expect: schema.Expect{ExitCode: &exitCode}},
			},
		},
		Reporter: nopReporter{},
	}
	result := r.StressTest("always-fails", 100, 1, true)
	if result.TotalRuns > 2 {
		t.Errorf("expected at most 2 runs with fail-fast, got %d", result.TotalRuns)
	}
	if result.Failures < 1 {
		t.Errorf("expected at least 1 failure, got %d", result.Failures)
	}
	if result.TotalRuns == 100 {
		t.Error("fail-fast did not stop early")
	}
}

func TestStressTest_AllowsFailure(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "pass", Run: "true", AllowFailure: true},
			},
		},
		Reporter: nopReporter{},
	}
	result := r.StressTest("pass", 5, 1, false)
	if result.Passes != 5 {
		t.Errorf("expected 5 passes, got %d", result.Passes)
	}
}
