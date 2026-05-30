//go:build tui

package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	tea "github.com/charmbracelet/bubbletea"
)

func TestStateTransitions_RunningToResults(t *testing.T) {
	m := NewModel("test-project", nil)
	if m.state != StateRunning {
		t.Fatalf("initial state: got %v, want StateRunning", m.state)
	}

	// Simulate test results arriving via testResultEvent
	testResults := []reporter.TestResultData{
		{Name: "a", Passed: true, Duration: 100 * time.Millisecond},
		{Name: "b", Passed: true, Duration: 200 * time.Millisecond},
		{Name: "c", Passed: false, Duration: 300 * time.Millisecond},
	}
	var updated tea.Model
	for _, tr := range testResults {
		updated, _ = m.Update(testResultEvent{Data: tr})
		m = updated.(Model)
	}

	// RUNNING -> RESULTS on summary (Tests field intentionally empty - mirrors production)
	updated, _ = m.Update(summaryEvent{Data: reporter.SuiteResultData{
		Total: 3, Passed: 2, Failed: 1, Duration: time.Second,
	}})
	m = updated.(Model)
	if m.state != StateResults {
		t.Fatalf("after summary: got %v, want StateResults", m.state)
	}
	// Verify model populated tests from pending, not from summaryEvent.Data.Tests
	if len(m.tests) != 3 {
		t.Fatalf("expected 3 tests from pending, got %d", len(m.tests))
	}
	if len(m.pending) != 0 {
		t.Fatalf("pending should be nil after summary, got %d", len(m.pending))
	}
}

func TestStateTransitions_ResultsToQuitting(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	m.tests = []reporter.TestResultData{{Name: "a", Passed: true}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.state != StateQuitting {
		t.Fatalf("after quit: got %v, want StateQuitting", m.state)
	}
	if cmd == nil {
		t.Fatal("quit should return tea.Quit cmd")
	}
}

func TestStateTransitions_RunningToQuitting(t *testing.T) {
	m := NewModel("test", nil)
	// State is already RUNNING from NewModel

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.state != StateQuitting {
		t.Fatalf("after quit during running: got %v, want StateQuitting", m.state)
	}
	if cmd == nil {
		t.Fatal("quit should return tea.Quit cmd")
	}
}

func TestStateTransitions_ErrorDismissal(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateError
	m.runError = errors.New("something broke")

	// Any key should dismiss error and return to results
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)
	if m.state != StateResults {
		t.Fatalf("after error dismiss: got %v, want StateResults", m.state)
	}
	if m.runError != nil {
		t.Fatal("runError should be cleared after dismiss")
	}
}

func TestFilterCycle(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	allTests := []reporter.TestResultData{
		{Name: "a", Passed: true},
		{Name: "b", Passed: false},
		{Name: "c", Skipped: true},
	}
	m.results = reporter.SuiteResultData{Tests: allTests}
	m.tests = allTests

	// All -> Failed
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.filter != FilterFailed {
		t.Fatalf("first f: got %v, want FilterFailed", m.filter)
	}
	if len(m.tests) != 1 || m.tests[0].Name != "b" {
		t.Fatalf("failed filter: got %v", m.tests)
	}

	// Failed -> Passed
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.filter != FilterPassed {
		t.Fatalf("second f: got %v, want FilterPassed", m.filter)
	}
	if len(m.tests) != 1 || m.tests[0].Name != "a" {
		t.Fatalf("passed filter: got %v", m.tests)
	}

	// Passed -> Skipped
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.filter != FilterSkipped {
		t.Fatalf("third f: got %v, want FilterSkipped", m.filter)
	}
	if len(m.tests) != 1 || m.tests[0].Name != "c" {
		t.Fatalf("skipped filter: got %v", m.tests)
	}

	// Skipped -> All
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.filter != FilterAll {
		t.Fatalf("fourth f: got %v, want FilterAll", m.filter)
	}
	if len(m.tests) != 3 {
		t.Fatalf("all filter: got %d tests, want 3", len(m.tests))
	}
}

func TestRerunIgnoredDuringRerunning(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateRerunning

	// r key should be ignored while rerunning (not in RESULTS state)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(Model)
	if m.state != StateRerunning {
		t.Fatalf("rerun during rerunning should stay: got %v", m.state)
	}
	if cmd != nil {
		t.Fatal("should not spawn cmd during rerunning")
	}
}

func TestCursorBounds(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	m.tests = []reporter.TestResultData{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	m.cursor = 0

	// Up at top stays at 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.cursor != 0 {
		t.Fatalf("cursor above 0: got %d", m.cursor)
	}

	// Down to bottom
	for i := 0; i < 5; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = updated.(Model)
	}
	if m.cursor != 2 {
		t.Fatalf("cursor past end: got %d, want 2", m.cursor)
	}

	// Go to top with 'g'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	if m.cursor != 0 {
		t.Fatalf("g should go to top: got %d", m.cursor)
	}

	// Go to bottom with 'G'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = updated.(Model)
	if m.cursor != 2 {
		t.Fatalf("G should go to bottom: got %d, want 2", m.cursor)
	}
}

func TestToggleExpand(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	m.tests = []reporter.TestResultData{
		{Name: "a", Assertions: []reporter.AssertionDetail{{Type: "http", Passed: true}}},
	}
	m.cursor = 0

	// Toggle expand
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.expanded[0] {
		t.Fatal("enter should expand current test")
	}

	// Toggle collapse
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.expanded[0] {
		t.Fatal("enter again should collapse current test")
	}
}

func TestToggleAll(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	m.tests = []reporter.TestResultData{
		{Name: "a"}, {Name: "b"},
	}

	// Tab to expand all
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if !m.allExpanded {
		t.Fatal("tab should set allExpanded")
	}

	// Tab again to collapse all
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.allExpanded {
		t.Fatal("tab again should unset allExpanded")
	}
}

func TestWatchTriggerInResults(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	m.results = reporter.SuiteResultData{Tests: []reporter.TestResultData{{Name: "a"}}}
	m.tests = m.results.Tests
	called := false
	m.rerunFunc = func(_ context.Context, names []string) error {
		called = true
		return nil
	}

	updated, cmd := m.Update(WatchTriggerEvent{})
	m = updated.(Model)
	if m.state != StateRerunning {
		t.Fatalf("watch trigger in results: got %v, want StateRerunning", m.state)
	}
	if cmd == nil {
		t.Fatal("should spawn rerun cmd")
	}
	// Execute the cmd to verify it calls rerunFunc
	cmd()
	if !called {
		t.Fatal("rerunFunc should have been called")
	}
}

func TestWatchTriggerIgnoredDuringRunning(t *testing.T) {
	m := NewModel("test", nil)
	// State is RUNNING from NewModel

	updated, cmd := m.Update(WatchTriggerEvent{})
	m = updated.(Model)
	if m.state != StateRunning {
		t.Fatalf("watch trigger during running should be ignored: got %v", m.state)
	}
	if cmd != nil {
		t.Fatal("should not spawn cmd during running")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := NewModel("test", nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	if m.width != 120 || m.height != 40 {
		t.Fatalf("window size: got %dx%d, want 120x40", m.width, m.height)
	}
}

func TestHelpToggle(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if !m.showHelp {
		t.Fatal("? should toggle help on")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.showHelp {
		t.Fatal("? again should toggle help off")
	}
}

func TestPendingAccumulatesDuringRunning(t *testing.T) {
	m := NewModel("test", nil)
	// State is RUNNING

	// Send test results
	updated, _ := m.Update(testResultEvent{Data: reporter.TestResultData{Name: "x", Passed: true}})
	m = updated.(Model)
	updated, _ = m.Update(testResultEvent{Data: reporter.TestResultData{Name: "y", Passed: false}})
	m = updated.(Model)

	if len(m.pending) != 2 {
		t.Fatalf("pending should accumulate: got %d, want 2", len(m.pending))
	}
	if len(m.tests) != 2 {
		t.Fatalf("tests should reflect pending during running: got %d, want 2", len(m.tests))
	}
}

func TestViewQuitting(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateQuitting
	m.width = 80
	m.height = 24
	if v := m.View(); v != "" {
		t.Fatalf("quitting view should be empty, got %q", v)
	}
}
