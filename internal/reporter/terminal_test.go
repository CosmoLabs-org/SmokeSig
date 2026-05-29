package reporter

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTerminal_PrereqPass(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.PrereqStart("Go installed")
	r.PrereqResult(PrereqResultData{Name: "Go installed", Passed: true, Output: "go1.26.2"})
	out := buf.String()
	if !strings.Contains(out, "Go installed") {
		t.Errorf("output missing prereq name: %q", out)
	}
	if !strings.Contains(out, "go1.26.2") {
		t.Errorf("output missing prereq output: %q", out)
	}
}

func TestTerminal_PrereqFail(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.PrereqStart("Docker")
	r.PrereqResult(PrereqResultData{Name: "Docker", Passed: false, Hint: "Install Docker"})
	out := buf.String()
	if !strings.Contains(out, "Docker") {
		t.Errorf("output missing prereq name: %q", out)
	}
	if !strings.Contains(out, "Install Docker") {
		t.Errorf("output missing hint: %q", out)
	}
}

func TestTerminal_TestPass(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.TestStart("Compiles")
	r.TestResult(TestResultData{
		Name:     "Compiles",
		Passed:   true,
		Duration: 150 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "Compiles") {
		t.Errorf("output missing test name: %q", out)
	}
	if !strings.Contains(out, "150ms") {
		t.Errorf("output missing duration: %q", out)
	}
}

func TestTerminal_TestFail(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.TestStart("Exit check")
	r.TestResult(TestResultData{
		Name:   "Exit check",
		Passed: false,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
		},
		Duration: 50 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "exit_code") {
		t.Errorf("output missing assertion type: %q", out)
	}
}

func TestTerminal_TestSkipped(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.TestStart("Skipped test")
	r.TestResult(TestResultData{Name: "Skipped test", Skipped: true, Duration: 0})
	out := buf.String()
	if !strings.Contains(out, "Skipped test") {
		t.Errorf("output missing test name: %q", out)
	}
}

func TestTerminal_Summary(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.Summary(SuiteResultData{
		Total:    5,
		Passed:   3,
		Failed:   1,
		Skipped:  1,
		Duration: 2 * time.Second,
	})
	out := buf.String()
	if !strings.Contains(out, "5 tests") {
		t.Errorf("output missing total: %q", out)
	}
	if !strings.Contains(out, "3 passed") {
		t.Errorf("output missing passed: %q", out)
	}
	if !strings.Contains(out, "1 failed") {
		t.Errorf("output missing failed: %q", out)
	}
}

func TestTerminal_QuietMode_SuppressesPassingTests(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.TestStart("Passing test")
	r.TestResult(TestResultData{
		Name:     "Passing test",
		Passed:   true,
		Duration: 100 * time.Millisecond,
	})
	out := buf.String()
	if out != "" {
		t.Errorf("quiet mode should suppress passing tests, got %q", out)
	}
}

func TestTerminal_QuietMode_ShowsFailures(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.TestStart("Failing test")
	r.TestResult(TestResultData{
		Name:   "Failing test",
		Passed: false,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
		},
		Duration: 50 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "Failing test") {
		t.Errorf("quiet mode should show failures, got %q", out)
	}
	if !strings.Contains(out, "exit_code") {
		t.Errorf("quiet mode should show failed assertion details, got %q", out)
	}
}

func TestTerminal_QuietMode_SuppressesPrereqs(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.PrereqStart("Go installed")
	r.PrereqResult(PrereqResultData{Name: "Go installed", Passed: true, Output: "go1.26"})
	out := buf.String()
	if out != "" {
		t.Errorf("quiet mode should suppress passing prereqs, got %q", out)
	}
}

func TestTerminal_QuietMode_ShowsFailedPrereqs(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.PrereqStart("Docker")
	r.PrereqResult(PrereqResultData{Name: "Docker", Passed: false, Hint: "Install Docker"})
	out := buf.String()
	if !strings.Contains(out, "Docker") {
		t.Errorf("quiet mode should show failed prereqs, got %q", out)
	}
	if !strings.Contains(out, "Install Docker") {
		t.Errorf("quiet mode should show prereq hints, got %q", out)
	}
}

func TestTerminal_QuietMode_ShowsSummary(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.Summary(SuiteResultData{
		Total:    5,
		Passed:   4,
		Failed:   1,
		Duration: 2 * time.Second,
	})
	out := buf.String()
	if !strings.Contains(out, "5 tests") {
		t.Errorf("quiet mode should show summary, got %q", out)
	}
}

func TestTerminal_QuietMode_SuppressesSkippedTests(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.TestStart("Skipped test")
	r.TestResult(TestResultData{Name: "Skipped test", Skipped: true, Duration: 0})
	out := buf.String()
	if out != "" {
		t.Errorf("quiet mode should suppress skipped tests, got %q", out)
	}
}

func TestTerminal_VerboseMode_ShowsAllAssertions(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityVerbose)
	r.TestStart("Passing test")
	r.TestResult(TestResultData{
		Name:   "Passing test",
		Passed: true,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "0", Passed: true},
			{Type: "stdout_contains", Expected: "hello", Actual: "hello world", Passed: true},
		},
		Duration: 100 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "exit_code") {
		t.Errorf("verbose mode should show all assertions for passing tests, got %q", out)
	}
	if !strings.Contains(out, "stdout_contains") {
		t.Errorf("verbose mode should show all assertions for passing tests, got %q", out)
	}
}

func TestTerminal_VerboseMode_ShowsPassingAssertionsOnFailure(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityVerbose)
	r.TestStart("Mixed test")
	r.TestResult(TestResultData{
		Name:   "Mixed test",
		Passed: false,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "0", Passed: true},
			{Type: "stdout_contains", Expected: "hello", Actual: "goodbye", Passed: false},
		},
		Duration: 50 * time.Millisecond,
	})
	out := buf.String()
	// Failed assertion shown (normal behavior)
	if !strings.Contains(out, "stdout_contains") {
		t.Errorf("verbose mode should show failed assertion, got %q", out)
	}
	// Passing assertion also shown (verbose-only behavior)
	if !strings.Contains(out, "exit_code") {
		t.Errorf("verbose mode should also show passing assertions on failure, got %q", out)
	}
}

func TestTerminal_NormalMode_HidesPassingAssertions(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf) // default = VerbosityNormal
	r.TestStart("Passing test")
	r.TestResult(TestResultData{
		Name:   "Passing test",
		Passed: true,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "0", Passed: true},
		},
		Duration: 100 * time.Millisecond,
	})
	out := buf.String()
	if strings.Contains(out, "exit_code") {
		t.Errorf("normal mode should not show assertion details for passing tests, got %q", out)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Microsecond, "(500µs)"},
		{150 * time.Millisecond, "(150ms)"},
		{2500 * time.Millisecond, "(2.5s)"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestTerminal_AllowedFailure_NormalMode(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.TestStart("Flaky test")
	r.TestResult(TestResultData{
		Name:           "Flaky test",
		Passed:         false,
		AllowedFailure: true,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
		},
		Duration: 30 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "Flaky test") {
		t.Errorf("output missing test name: %q", out)
	}
	if !strings.Contains(out, "allowed") {
		t.Errorf("output missing allowed marker: %q", out)
	}
	if !strings.Contains(out, "exit_code") {
		t.Errorf("output missing failed assertion detail: %q", out)
	}
}

func TestTerminal_AllowedFailure_WithError(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.TestResult(TestResultData{
		Name:           "Flaky with error",
		Passed:         false,
		AllowedFailure: true,
		Error:          fmt.Errorf("network timeout"),
		Duration:       10 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "network timeout") {
		t.Errorf("output missing error message: %q", out)
	}
}

func TestTerminal_AllowedFailure_QuietMode_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.TestResult(TestResultData{
		Name:           "Flaky quiet",
		Passed:         false,
		AllowedFailure: true,
		Duration:       10 * time.Millisecond,
	})
	out := buf.String()
	if out != "" {
		t.Errorf("quiet mode should suppress allowed failures, got %q", out)
	}
}

func TestTerminal_Summary_TraceHealth_Degraded(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.Summary(SuiteResultData{
		Total:          3,
		Passed:         3,
		Duration:       time.Second,
		TraceHealthPct: 40.0,
		TraceDegraded:  true,
	})
	out := buf.String()
	if !strings.Contains(out, "trace health:") {
		t.Errorf("output missing trace health label: %q", out)
	}
	if !strings.Contains(out, "40.0%") {
		t.Errorf("output missing trace health percentage: %q", out)
	}
	if !strings.Contains(out, "degraded") {
		t.Errorf("output missing degraded marker: %q", out)
	}
}

func TestTerminal_Summary_TraceHealth_Healthy(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.Summary(SuiteResultData{
		Total:          3,
		Passed:         3,
		Duration:       time.Second,
		TraceHealthPct: 90.0,
		TraceDegraded:  false,
	})
	out := buf.String()
	if !strings.Contains(out, "trace health:") {
		t.Errorf("output missing trace health label: %q", out)
	}
	if !strings.Contains(out, "90.0%") {
		t.Errorf("output missing trace health percentage: %q", out)
	}
	// Healthy trace should NOT show "degraded"
	if strings.Contains(out, "degraded") {
		t.Errorf("healthy trace should not show degraded: %q", out)
	}
}

func TestTerminal_Summary_AllowedFailures(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.Summary(SuiteResultData{
		Total:           5,
		Passed:          3,
		Failed:          1,
		AllowedFailures: 1,
		Duration:        time.Second,
	})
	out := buf.String()
	if !strings.Contains(out, "allowed-failure") {
		t.Errorf("output missing allowed-failure count: %q", out)
	}
}

func TestTerminal_Summary_ZeroTraceHealth_Hidden(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.Summary(SuiteResultData{
		Total:          3,
		Passed:         3,
		Duration:       time.Second,
		TraceHealthPct: 0, // zero means not shown
	})
	out := buf.String()
	if strings.Contains(out, "trace health:") {
		t.Errorf("zero trace health should not be shown: %q", out)
	}
}

func TestTerminal_VerboseMode_AllowedFailure(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityVerbose)
	r.TestResult(TestResultData{
		Name:           "Flaky verbose",
		Passed:         false,
		AllowedFailure: true,
		Assertions: []AssertionDetail{
			{Type: "stdout_contains", Expected: "ok", Actual: "err", Passed: false},
		},
		Duration: 10 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "stdout_contains") {
		t.Errorf("verbose mode should show assertion detail in allowed failure: %q", out)
	}
}

func TestTerminal_NoOpMethods_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	// PrereqStart and TestStart are real methods, just verify they don't panic
	r.PrereqStart("prereq-name")
	r.TestStart("test-name")
}

func TestTerminal_FailedTest_WithError(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminal(&buf)
	r.TestResult(TestResultData{
		Name:     "Broken test",
		Passed:   false,
		Error:    fmt.Errorf("connection refused"),
		Duration: 20 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "connection refused") {
		t.Errorf("output missing error message: %q", out)
	}
}

func TestTerminal_FailedTest_WithError_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalWithVerbosity(&buf, VerbosityQuiet)
	r.TestResult(TestResultData{
		Name:     "Broken test",
		Passed:   false,
		Error:    fmt.Errorf("timeout"),
		Duration: 20 * time.Millisecond,
	})
	out := buf.String()
	if !strings.Contains(out, "timeout") {
		t.Errorf("quiet mode should show error on failure: %q", out)
	}
}
