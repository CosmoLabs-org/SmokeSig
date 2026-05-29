package reporter

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// TestBackstageNoopMethods exercises PrereqStart, PrereqResult, TestStart
// on Backstage — all are no-op stubs that were at 0% coverage.
func TestBackstageNoopMethods(t *testing.T) {
	b := NewBackstage(&bytes.Buffer{})
	// Must not panic.
	b.PrereqStart("some-prereq")
	b.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	b.TestStart("some-test")
}

// TestBackstageSummary_AllPassed covers the healthy path through buildBackstageEntity.
func TestBackstageSummary_AllPassed(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{Name: "t1", Passed: true, Duration: 10 * time.Millisecond})
	b.Summary(SuiteResultData{Project: "proj", Total: 1, Passed: 1, Failed: 0})
	if buf.Len() == 0 {
		t.Error("expected JSON output, got empty buffer")
	}
}

// TestBackstageOverallStatus covers all branches in backstageOverallStatus.
func TestBackstageOverallStatus(t *testing.T) {
	tests := []struct {
		name  string
		tests []TestResultData
		want  string
	}{
		{"no tests", nil, "unknown"},
		{"all pass", []TestResultData{{Passed: true}}, "healthy"},
		{"hard failure", []TestResultData{{Passed: false, AllowedFailure: false}}, "unhealthy"},
		{"allowed failure only", []TestResultData{{Passed: false, AllowedFailure: true}}, "degraded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := backstageOverallStatus(tc.tests)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBackstageSummary_FailedWithError covers the msg-from-error branch.
func TestBackstageSummary_FailedWithError(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{
		Name:   "t-err",
		Passed: false,
		Error:  errors.New("boom"),
	})
	b.Summary(SuiteResultData{Project: "proj", Total: 1, Passed: 0, Failed: 1})
	out := buf.String()
	if out == "" {
		t.Error("expected output")
	}
}

// TestBackstageSummary_AllowedFailure covers the degraded-check branch.
func TestBackstageSummary_AllowedFailure(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{
		Name:           "t-allowed",
		Passed:         false,
		AllowedFailure: true,
	})
	b.Summary(SuiteResultData{Project: "proj", Total: 1, Passed: 0, Failed: 1})
	out := buf.String()
	if out == "" {
		t.Error("expected JSON output")
	}
}

// TestGitHubActionsNoopMethods exercises the no-op stubs on GitHubActions.
func TestGitHubActionsNoopMethods(t *testing.T) {
	g := NewGitHubActions(&bytes.Buffer{})
	g.PrereqStart("prereq")
	g.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	g.TestStart("test")
}

// TestGitHubActionsSummary covers Summary, buildMarkdown, emitCommands.
func TestGitHubActionsSummary(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitHubActions(&buf)
	g.TestResult(TestResultData{
		Name:   "pass",
		Passed: true,
		Duration: 5 * time.Millisecond,
	})
	g.TestResult(TestResultData{
		Name:   "fail",
		Passed: false,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Passed: false, Expected: "0", Actual: "1"},
		},
	})
	g.TestResult(TestResultData{
		Name:           "flaky",
		Passed:         false,
		AllowedFailure: true,
	})
	g.Summary(SuiteResultData{Project: "p", Total: 3, Passed: 1, Failed: 2})
	out := buf.String()
	if out == "" {
		t.Error("expected output from GitHubActions summary")
	}
}

// TestJSONNoopMethods exercises the no-op stubs on JSON reporter.
func TestJSONNoopMethods(t *testing.T) {
	j := NewJSON(&bytes.Buffer{})
	j.PrereqStart("prereq")
	j.TestStart("test")
}

// TestJSONSummary_WithPrereqAndError covers prereq + error branches in JSON.Summary.
func TestJSONSummary_WithPrereqAndError(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSON(&buf)
	j.PrereqResult(PrereqResultData{
		Name:   "docker",
		Passed: false,
		Error:  errors.New("docker not found"),
		Hint:   "install docker",
	})
	j.TestResult(TestResultData{
		Name:   "t1",
		Passed: false,
		Error:  errors.New("cmd failed"),
		Assertions: []AssertionDetail{
			{Type: "exit_code", Passed: false, Expected: "0", Actual: "2"},
		},
	})
	j.Summary(SuiteResultData{Project: "proj", Total: 1, Passed: 0, Failed: 1})
	out := buf.String()
	if out == "" {
		t.Error("expected JSON output")
	}
}

// TestJUnitNoopMethods exercises the no-op stubs on JUnit reporter.
func TestJUnitNoopMethods(t *testing.T) {
	j := NewJUnit(&bytes.Buffer{})
	j.PrereqStart("prereq")
	j.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	j.TestStart("test")
}
