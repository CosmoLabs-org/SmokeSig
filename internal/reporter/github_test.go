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
