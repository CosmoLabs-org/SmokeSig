package reporter

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGitHubActions_SummaryAllPass(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:     "api-health",
		Passed:   true,
		Duration: 120 * time.Millisecond,
	})
	g.TestResult(TestResultData{
		Name:     "db-ping",
		Passed:   true,
		Duration: 45 * time.Millisecond,
	})

	summaryFile, err := os.CreateTemp("", "gha-summary-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(summaryFile.Name())
	summaryFile.Close()

	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile.Name())
	g.Summary(SuiteResultData{
		Total:    2,
		Passed:   2,
		Failed:   0,
		Duration: 165 * time.Millisecond,
		Tests: []TestResultData{
			{Name: "api-health", Passed: true, Duration: 120 * time.Millisecond},
			{Name: "db-ping", Passed: true, Duration: 45 * time.Millisecond},
		},
	})

	// Check workflow commands in buffer (should be empty for all-pass)
	if got := buf.String(); got != "" {
		t.Errorf("expected no workflow commands for passing suite, got: %q", got)
	}

	// Check step summary markdown
	content, err := os.ReadFile(summaryFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	md := string(content)

	if !strings.Contains(md, "## Smoke Test Results") {
		t.Error("summary missing header")
	}
	if !strings.Contains(md, "2/2 passed") {
		t.Errorf("summary missing pass count, got: %s", md)
	}
	if !strings.Contains(md, "api-health") {
		t.Error("summary missing test name api-health")
	}
	if !strings.Contains(md, "db-ping") {
		t.Error("summary missing test name db-ping")
	}
}

func TestGitHubActions_SummaryWithFailures(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:     "api-health",
		Passed:   true,
		Duration: 120 * time.Millisecond,
	})
	g.TestResult(TestResultData{
		Name:     "auth-endpoint",
		Passed:   false,
		Duration: 302 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
		},
	})

	summaryFile, err := os.CreateTemp("", "gha-summary-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(summaryFile.Name())
	summaryFile.Close()

	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile.Name())
	g.Summary(SuiteResultData{
		Total:    2,
		Passed:   1,
		Failed:   1,
		Duration: 422 * time.Millisecond,
		Tests: []TestResultData{
			{Name: "api-health", Passed: true, Duration: 120 * time.Millisecond},
			{Name: "auth-endpoint", Passed: false, Duration: 302 * time.Millisecond,
				Assertions: []AssertionDetail{{Type: "exit_code", Expected: "0", Actual: "1", Passed: false}}},
		},
	})

	// Check workflow commands
	output := buf.String()
	if !strings.Contains(output, "::error") {
		t.Errorf("expected ::error workflow command, got: %q", output)
	}
	if !strings.Contains(output, "auth-endpoint") {
		t.Errorf("error command should mention failed test, got: %q", output)
	}

	// Check step summary has failed section
	content, _ := os.ReadFile(summaryFile.Name())
	md := string(content)
	if !strings.Contains(md, "### Failed Tests") {
		t.Error("summary missing failed tests section")
	}
}

func TestGitHubActions_WorkflowCommandFormat(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:     "bad-test",
		Passed:   false,
		Duration: 50 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
		},
	})

	summaryFile, err := os.CreateTemp("", "gha-summary-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(summaryFile.Name())
	summaryFile.Close()

	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile.Name())
	g.Summary(SuiteResultData{
		Total:  1,
		Failed: 1,
		Tests: []TestResultData{
			{Name: "bad-test", Passed: false, Duration: 50 * time.Millisecond,
				Assertions: []AssertionDetail{{Type: "exit_code", Expected: "0", Actual: "1", Passed: false}}},
		},
	})

	output := buf.String()
	if !strings.HasPrefix(output, "::error title=Smoke Test Failed") {
		t.Errorf("workflow command has wrong prefix: %q", output)
	}
}

func TestGitHubActions_WarningForAllowedFailure(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:           "flaky-test",
		Passed:         false,
		AllowedFailure: true,
		Duration:       10 * time.Millisecond,
	})

	summaryFile, _ := os.CreateTemp("", "gha-summary-*")
	defer os.Remove(summaryFile.Name())
	summaryFile.Close()

	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile.Name())
	g.Summary(SuiteResultData{
		Total:           1,
		AllowedFailures: 1,
		Tests: []TestResultData{
			{Name: "flaky-test", Passed: false, AllowedFailure: true, Duration: 10 * time.Millisecond},
		},
	})

	output := buf.String()
	if !strings.Contains(output, "::warning") {
		t.Errorf("expected ::warning for allowed failure, got: %q", output)
	}
	if !strings.Contains(output, "flaky-test") {
		t.Errorf("warning should mention test name, got: %q", output)
	}
}

func TestGitHubActions_NoStepSummaryEnv(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:     "test-1",
		Passed:   true,
		Duration: 10 * time.Millisecond,
	})

	t.Setenv("GITHUB_STEP_SUMMARY", "")
	g.Summary(SuiteResultData{
		Total:    1,
		Passed:   1,
		Duration: 10 * time.Millisecond,
		Tests: []TestResultData{
			{Name: "test-1", Passed: true, Duration: 10 * time.Millisecond},
		},
	})

	// With no GITHUB_STEP_SUMMARY, summary should go to the writer (stdout fallback)
	output := buf.String()
	if !strings.Contains(output, "## Smoke Test Results") {
		t.Errorf("summary should fall back to writer when no step summary env, got: %q", output)
	}
}

func TestGitHubActions_EmptySuite(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	summaryFile, _ := os.CreateTemp("", "gha-summary-*")
	defer os.Remove(summaryFile.Name())
	summaryFile.Close()

	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile.Name())
	g.Summary(SuiteResultData{
		Total:    0,
		Passed:   0,
		Failed:   0,
		Duration: 0,
	})

	content, _ := os.ReadFile(summaryFile.Name())
	md := string(content)
	if !strings.Contains(md, "0/0 passed") {
		t.Errorf("empty suite should show 0/0, got: %s", md)
	}
}

func TestGitHubActions_WriteSummary_FallbackOnBadPath(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:     "check",
		Passed:   true,
		Duration: 10 * time.Millisecond,
	})

	// Set GITHUB_STEP_SUMMARY to a path that can't be created
	t.Setenv("GITHUB_STEP_SUMMARY", "/nonexistent-dir/summary.md")
	g.Summary(SuiteResultData{
		Total:    1,
		Passed:   1,
		Duration: 10 * time.Millisecond,
	})

	// Should fall back to writer
	output := buf.String()
	if !strings.Contains(output, "## Smoke Test Results") {
		t.Errorf("should fall back to writer on bad summary path, got: %q", output)
	}
}

func TestGitHubActions_MixedResults_AllSections(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:     "pass-test",
		Passed:   true,
		Duration: 50 * time.Millisecond,
	})
	g.TestResult(TestResultData{
		Name:           "flaky-test",
		Passed:         false,
		AllowedFailure: true,
		Duration:       20 * time.Millisecond,
	})
	g.TestResult(TestResultData{
		Name:     "fail-test",
		Passed:   false,
		Duration: 100 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "2", Passed: false},
		},
	})

	t.Setenv("GITHUB_STEP_SUMMARY", "")
	g.Summary(SuiteResultData{
		Total:           3,
		Passed:          1,
		Failed:          1,
		AllowedFailures: 1,
		Duration:        170 * time.Millisecond,
	})

	output := buf.String()
	if !strings.Contains(output, "### Failed Tests") {
		t.Errorf("mixed results should include failed tests section: %q", output)
	}
	if !strings.Contains(output, "### Allowed Failures") {
		t.Errorf("mixed results should include allowed failures section: %q", output)
	}
	if !strings.Contains(output, "flaky-test") {
		t.Errorf("allowed failures section should mention flaky-test: %q", output)
	}
	if !strings.Contains(output, "::error") {
		t.Errorf("workflow command should be emitted for fail-test: %q", output)
	}
	if !strings.Contains(output, "::warning") {
		t.Errorf("workflow command should be emitted for flaky-test: %q", output)
	}
}

func TestGitHubActions_NoAssertions_FailureByName(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)

	g.TestResult(TestResultData{
		Name:       "fail-no-assertions",
		Passed:     false,
		Duration:   30 * time.Millisecond,
		Assertions: nil, // no assertions
	})

	t.Setenv("GITHUB_STEP_SUMMARY", "")
	g.Summary(SuiteResultData{Total: 1, Failed: 1, Duration: 30 * time.Millisecond})

	output := buf.String()
	if !strings.Contains(output, "fail-no-assertions") {
		t.Errorf("should emit error command with test name when no assertions: %q", output)
	}
}

func TestFormatGhaDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0ms"},                           // sub-millisecond: zero
		{500 * time.Microsecond, "0ms"},      // sub-millisecond
		{150 * time.Millisecond, "150ms"},    // under 1s
		{2500 * time.Millisecond, "2.5s"},    // over 1s
		{1 * time.Second, "1.0s"},            // exactly 1s
	}
	for _, tt := range tests {
		got := formatGhaDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatGhaDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestGitHubActions_NoOpMethods_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)
	// PrereqStart, PrereqResult, TestStart are no-ops but must not panic
	g.PrereqStart("prereq-name")
	g.PrereqResult(PrereqResultData{Name: "prereq-name", Passed: true})
	g.TestStart("test-name")
}
