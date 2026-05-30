package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
)

// stressMockReporter records reporter calls for test assertions.
type stressMockReporter struct {
	testResults []reporter.TestResultData
	summary     *reporter.SuiteResultData
}

func (m *stressMockReporter) PrereqStart(name string)                {}
func (m *stressMockReporter) PrereqResult(r reporter.PrereqResultData) {}
func (m *stressMockReporter) TestStart(name string)                   {}
func (m *stressMockReporter) TestResult(r reporter.TestResultData) {
	m.testResults = append(m.testResults, r)
}
func (m *stressMockReporter) Summary(s reporter.SuiteResultData) {
	cp := s
	m.summary = &cp
}

func TestFormatStressSummary_Basic(t *testing.T) {
	r := runner.StressResult{
		TestName:    "api-health",
		TotalRuns:   10,
		Passes:      10,
		Failures:    0,
		PassRate:    100.0,
		Duration:    2 * time.Second,
		Concurrency: 1,
		Reliability: "Stable",
	}
	out := formatStressSummary(r, "")
	for _, want := range []string{"Stress Test Complete", "api-health", "10/10", "Stable", "100%"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestFormatStressSummary_WithFailures(t *testing.T) {
	r := runner.StressResult{
		TestName:    "db-ping",
		TotalRuns:   10,
		Passes:      7,
		Failures:    3,
		PassRate:    70.0,
		Duration:    5 * time.Second,
		Concurrency: 2,
		Reliability: "Flaky",
		ErrorGroups: []runner.ErrorGroup{
			{Message: "connection refused", Count: 2},
			{Message: "timeout", Count: 1},
		},
	}
	out := formatStressSummary(r, "")
	if !strings.Contains(out, "Failures:") {
		t.Errorf("expected output to contain 'Failures:' section, got:\n%s", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected output to contain error message 'connection refused', got:\n%s", out)
	}
	if !strings.Contains(out, "timeout") {
		t.Errorf("expected output to contain error message 'timeout', got:\n%s", out)
	}
	if !strings.Contains(out, "[2 times]") {
		t.Errorf("expected output to contain '[2 times]', got:\n%s", out)
	}
	if !strings.Contains(out, "[1 times]") {
		t.Errorf("expected output to contain '[1 times]', got:\n%s", out)
	}
}

func TestFormatStressSummary_WithProject(t *testing.T) {
	r := runner.StressResult{
		TestName:    "healthcheck",
		TotalRuns:   5,
		Passes:      5,
		PassRate:    100.0,
		Duration:    time.Second,
		Concurrency: 1,
		Reliability: "Stable",
	}
	out := formatStressSummary(r, "myapp")
	if !strings.Contains(out, "Project: myapp") {
		t.Errorf("expected output to contain 'Project: myapp', got:\n%s", out)
	}
}

func TestFormatStressSummary_NoProject(t *testing.T) {
	r := runner.StressResult{
		TestName:    "healthcheck",
		TotalRuns:   5,
		Passes:      5,
		PassRate:    100.0,
		Duration:    time.Second,
		Concurrency: 1,
		Reliability: "Stable",
	}
	out := formatStressSummary(r, "")
	if strings.Contains(out, "Project:") {
		t.Errorf("expected output NOT to contain 'Project:' when project is empty, got:\n%s", out)
	}
}

func TestStressCmd_Flags(t *testing.T) {
	if !stressCmd.HasFlags() {
		t.Fatal("stress command should have flags")
	}
	runs, err := stressCmd.Flags().GetInt("runs")
	if err != nil || runs != 50 {
		t.Errorf("expected --runs default 50, got %d, err %v", runs, err)
	}
	workers, err := stressCmd.Flags().GetInt("workers")
	if err != nil || workers != 1 {
		t.Errorf("expected --workers default 1, got %d, err %v", workers, err)
	}
}

func TestStressCmd_RequiresTestName(t *testing.T) {
	err := stressCmd.Args(stressCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no test name provided")
	}
}

func TestReportStressResult(t *testing.T) {
	result := runner.StressResult{
		TestName:   "healthcheck",
		TotalRuns:  5,
		Passes:     3,
		Failures:   2,
		PassRate:   60.0,
		Duration:   2 * time.Second,
		ErrorGroups: []runner.ErrorGroup{
			{Message: "connection refused", Count: 2},
		},
	}
	mock := &stressMockReporter{}
	reportStressResult(mock, result, "myapp")

	if len(mock.testResults) != 3 {
		t.Fatalf("expected 3 TestResult calls (one per pass), got %d", len(mock.testResults))
	}
	for i, tr := range mock.testResults {
		if !tr.Passed {
			t.Errorf("TestResult[%d]: expected Passed=true", i)
		}
		want := "healthcheck (run "
		if !strings.HasPrefix(tr.Name, want) {
			t.Errorf("TestResult[%d]: expected name to start with %q, got %q", i, want, tr.Name)
		}
	}
	if mock.summary == nil {
		t.Fatal("expected Summary to be called")
	}
	s := mock.summary
	if s.Project != "myapp" {
		t.Errorf("expected Project myapp, got %q", s.Project)
	}
	if s.Total != 5 {
		t.Errorf("expected Total 5, got %d", s.Total)
	}
	if s.Passed != 3 {
		t.Errorf("expected Passed 3, got %d", s.Passed)
	}
	if s.Failed != 2 {
		t.Errorf("expected Failed 2, got %d", s.Failed)
	}
}

func TestReportStressResult_AllPass(t *testing.T) {
	result := runner.StressResult{
		TestName:  "ping",
		TotalRuns: 3,
		Passes:    3,
		Failures:  0,
		PassRate:  100.0,
		Duration:  500 * time.Millisecond,
	}
	mock := &stressMockReporter{}
	reportStressResult(mock, result, "")

	if len(mock.testResults) != 3 {
		t.Fatalf("expected 3 TestResult calls, got %d", len(mock.testResults))
	}
	if mock.summary.Failed != 0 {
		t.Errorf("expected 0 failures, got %d", mock.summary.Failed)
	}
}

func TestFormatStressSummary_ZeroPassRate(t *testing.T) {
	r := runner.StressResult{
		TestName:    "failing-test",
		TotalRuns:   4,
		Passes:      0,
		Failures:    4,
		PassRate:    0.0,
		Duration:    time.Second,
		Concurrency: 2,
		Reliability: "Unreliable",
	}
	out := formatStressSummary(r, "")
	for _, want := range []string{"0/4", "0%", "Unreliable"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Failures:") {
		t.Error("expected no Failures section when ErrorGroups is empty")
	}
}

func TestFormatStressSummary_WithDedupedErrors(t *testing.T) {
	r := runner.StressResult{
		TestName:   "api",
		TotalRuns:  20,
		Passes:     15,
		Failures:   5,
		PassRate:   75.0,
		Duration:   3 * time.Second,
		Reliability: "Flaky",
		ErrorGroups: []runner.ErrorGroup{
			{Message: "ECONNREFUSED 127.0.0.1:8080", Count: 3},
			{Message: "HTTP 503 Service Unavailable", Count: 2},
		},
	}
	out := formatStressSummary(r, "testproj")
	for _, want := range []string{
		"Failures:",
		"ECONNREFUSED 127.0.0.1:8080",
		"[3 times]",
		"HTTP 503 Service Unavailable",
		"[2 times]",
		"Project: testproj",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunStress_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, ".smokesig.yaml")
	content := `version: 1
project: test
tests:
  - name: echo-test
    run: echo hello
    expect:
      exit_code: 0
      stdout_contains: hello
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigFile := configFile
	configFile = cfgPath
	defer func() { configFile = origConfigFile }()

	cmd := stressCmd
	cmd.SetArgs([]string{"echo-test", "--runs", "3", "--workers", "1", "-f", cfgPath})

	err := runStress(cmd, []string{"echo-test"})
	if err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

func TestRunStress_TestNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, ".smokesig.yaml")
	content := `version: 1
project: test
tests:
  - name: existing-test
    run: echo ok
    expect:
      exit_code: 0
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigFile := configFile
	configFile = cfgPath
	defer func() { configFile = origConfigFile }()

	cmd := stressCmd
	err := runStress(cmd, []string{"nonexistent-test"})
	if err == nil {
		t.Fatal("expected error for nonexistent test name")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}
