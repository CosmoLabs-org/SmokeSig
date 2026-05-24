package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
)

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
