//go:build tui

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

func TestApplyFilter(t *testing.T) {
	tests := []reporter.TestResultData{
		{Name: "pass1", Passed: true},
		{Name: "pass2", Passed: true},
		{Name: "fail1", Passed: false},
		{Name: "skip1", Skipped: true},
	}

	if got := applyFilter(tests, FilterAll); len(got) != 4 {
		t.Errorf("FilterAll: got %d, want 4", len(got))
	}
	if got := applyFilter(tests, FilterFailed); len(got) != 1 {
		t.Errorf("FilterFailed: got %d, want 1", len(got))
	}
	if got := applyFilter(tests, FilterPassed); len(got) != 2 {
		t.Errorf("FilterPassed: got %d, want 2", len(got))
	}
	if got := applyFilter(tests, FilterSkipped); len(got) != 1 {
		t.Errorf("FilterSkipped: got %d, want 1", len(got))
	}
}

func TestApplyFilterEmpty(t *testing.T) {
	var tests []reporter.TestResultData
	if got := applyFilter(tests, FilterFailed); got != nil {
		t.Errorf("FilterFailed on empty: got %v, want nil", got)
	}
}

func TestViewSummaryContainsCounts(t *testing.T) {
	m := NewModel("test-project", nil)
	m.state = StateResults
	m.width = 80
	m.results = reporter.SuiteResultData{
		Total: 10, Passed: 8, Failed: 1, Skipped: 1,
		Duration: 2 * time.Second,
	}

	view := viewSummary(m)
	for _, want := range []string{"10", "8", "1"} {
		if !strings.Contains(view, want) {
			t.Errorf("summary missing %q: %s", want, view)
		}
	}
}

func TestViewHeaderShowsProject(t *testing.T) {
	m := NewModel("my-project", nil)
	m.state = StateResults
	m.width = 80
	m.tests = []reporter.TestResultData{{Name: "a", Passed: true}}
	m.results = reporter.SuiteResultData{Passed: 1}

	header := viewHeader(m)
	if !strings.Contains(header, "my-project") {
		t.Errorf("header should contain project name, got: %s", header)
	}
}

func TestViewHeaderRunning(t *testing.T) {
	m := NewModel("proj", nil)
	m.state = StateRunning
	m.width = 80

	header := viewHeader(m)
	if !strings.Contains(header, "RUNNING") {
		t.Errorf("header should say RUNNING, got: %s", header)
	}
}

func TestViewShortcutsResults(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults

	shortcuts := viewShortcuts(m)
	// Shortcuts contain bracketed keys like [f]ilter, [r]erun, [q]uit
	for _, want := range []string{"[f]ilter", "[r]erun", "[q]uit"} {
		if !strings.Contains(shortcuts, want) {
			t.Errorf("shortcuts should contain %q, got: %s", want, shortcuts)
		}
	}
}

func TestViewShortcutsRunning(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateRunning

	shortcuts := viewShortcuts(m)
	if !strings.Contains(shortcuts, "[q]uit") {
		t.Errorf("shortcuts during running should contain [q]uit, got: %s", shortcuts)
	}
	if strings.Contains(shortcuts, "[f]ilter") {
		t.Errorf("shortcuts during running should not contain [f]ilter, got: %s", shortcuts)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"one line", 1},
		{"line1\nline2", 2},
		{"a\nb\nc", 3},
	}
	for _, tt := range tests {
		if got := countLines(tt.input); got != tt.want {
			t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestStatusIcon(t *testing.T) {
	pass := statusIcon(reporter.TestResultData{Passed: true})
	if !strings.Contains(pass, "✓") {
		t.Errorf("pass icon should contain checkmark, got: %s", pass)
	}

	fail := statusIcon(reporter.TestResultData{Passed: false})
	if !strings.Contains(fail, "✗") {
		t.Errorf("fail icon should contain cross, got: %s", fail)
	}

	skip := statusIcon(reporter.TestResultData{Skipped: true})
	if !strings.Contains(skip, "⊘") {
		t.Errorf("skip icon should contain circle, got: %s", skip)
	}

	allowed := statusIcon(reporter.TestResultData{AllowedFailure: true})
	if !strings.Contains(allowed, "~") {
		t.Errorf("allowed icon should contain tilde, got: %s", allowed)
	}
}

func TestViewTestListEmpty(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	m.tests = nil
	m.width = 80

	list := viewTestList(m, 10)
	if !strings.Contains(list, "No tests") {
		t.Errorf("empty test list should show message, got: %s", list)
	}
}

func TestViewTestListWaiting(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateRunning
	m.tests = nil
	m.width = 80

	list := viewTestList(m, 10)
	if !strings.Contains(list, "Waiting") {
		t.Errorf("running with no results should show waiting, got: %s", list)
	}
}
