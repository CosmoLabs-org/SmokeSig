//go:build tui

package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

// helper: send a message to a model and return the updated model.
func updateModel(m model, msg tea.Msg) model {
	updated, _ := m.Update(msg)
	return updated.(model)
}

func TestCursorMovement(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// Start 3 tests
	m = updateModel(m, testStartMsg{Name: "t1"})
	m = updateModel(m, testStartMsg{Name: "t2"})
	m = updateModel(m, testStartMsg{Name: "t3"})

	// Complete all 3
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "t1", Passed: true}})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "t2", Passed: true}})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "t3", Passed: true}})

	// Cursor starts at 0
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}

	// Down twice → cursor=2
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1 after first down, got %d", m.cursor)
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Fatalf("expected cursor 2 after second down, got %d", m.cursor)
	}

	// Up once → cursor=1
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1 after up, got %d", m.cursor)
	}
}

func TestExpandCollapse(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// Add one completed test
	m = updateModel(m, testStartMsg{Name: "t1"})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "t1", Passed: true}})

	// Expand
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	idx := m.filtered[0]
	if !m.expanded[idx] {
		t.Fatal("expected test to be expanded after first Enter")
	}

	// Collapse
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.expanded[idx] {
		t.Fatal("expected test to be collapsed after second Enter")
	}
}

func TestFilterCycling(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// 1 pass, 1 fail, 1 skip
	m = updateModel(m, testStartMsg{Name: "pass-test"})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "pass-test", Passed: true}})
	m = updateModel(m, testStartMsg{Name: "fail-test"})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "fail-test", Passed: false}})
	m = updateModel(m, testStartMsg{Name: "skip-test"})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "skip-test", Skipped: true}})

	// Initially: filterAll, 3 visible
	if m.filter != filterAll || len(m.filtered) != 3 {
		t.Fatalf("expected filterAll with 3 items, got filter=%d len=%d", m.filter, len(m.filtered))
	}

	// Tab → filterPassed, 1 visible
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.filter != filterPassed || len(m.filtered) != 1 {
		t.Fatalf("expected filterPassed with 1 item, got filter=%d len=%d", m.filter, len(m.filtered))
	}

	// Tab → filterFailed, 1 visible
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.filter != filterFailed || len(m.filtered) != 1 {
		t.Fatalf("expected filterFailed with 1 item, got filter=%d len=%d", m.filter, len(m.filtered))
	}

	// Tab → filterSkipped, 1 visible
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.filter != filterSkipped || len(m.filtered) != 1 {
		t.Fatalf("expected filterSkipped with 1 item, got filter=%d len=%d", m.filter, len(m.filtered))
	}

	// Tab → filterAll, 3 visible
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.filter != filterAll || len(m.filtered) != 3 {
		t.Fatalf("expected filterAll with 3 items, got filter=%d len=%d", m.filter, len(m.filtered))
	}
}

func TestSearchMode(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	m = updateModel(m, testStartMsg{Name: "api-test"})
	m = updateModel(m, testStartMsg{Name: "db-test"})

	// Enter search mode with "/"
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.searchMode {
		t.Fatal("expected searchMode=true after pressing '/'")
	}

	// Type "a", "p", "i"
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.searchText != "api" {
		t.Fatalf("expected searchText='api', got %q", m.searchText)
	}
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 filtered result matching 'api', got %d", len(m.filtered))
	}

	// Esc clears search
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.searchMode {
		t.Fatal("expected searchMode=false after Esc")
	}
	if m.searchText != "" {
		t.Fatalf("expected searchText='' after Esc, got %q", m.searchText)
	}
}

func TestRerunDisabledWhileRunning(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.running = true

	// Add a failed test and set rerun func
	m = updateModel(m, testStartMsg{Name: "t1"})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "t1", Passed: false}})
	called := false
	m.rerunFunc = func(string) reporter.TestResultData {
		called = true
		return reporter.TestResultData{Name: "t1", Passed: true}
	}

	// Press "r" — should be no-op while running
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if called {
		t.Fatal("rerunFunc should not be called while running")
	}
	if len(m.rerunning) != 0 {
		t.Fatal("no tests should be rerunning while running=true")
	}
}

func TestRerunOnFailed(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.running = false

	m = updateModel(m, testStartMsg{Name: "t1"})
	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{Name: "t1", Passed: false}})

	called := false
	m.rerunFunc = func(name string) reporter.TestResultData {
		called = true
		return reporter.TestResultData{Name: name, Passed: true}
	}

	// Cursor is on t1 (only test), press "r"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m2 := updated.(model)

	if !called && cmd == nil {
		t.Fatal("expected rerun to trigger on failed test")
	}
	if !m2.rerunning["t1"] {
		t.Fatal("expected t1 to be marked as rerunning")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Cmd to be returned for rerun")
	}
}

func TestTestResultUpdates(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	m = updateModel(m, testStartMsg{Name: "t1"})
	if len(m.tests) != 1 {
		t.Fatalf("expected 1 test after start, got %d", len(m.tests))
	}

	m = updateModel(m, testResultMsg{Data: reporter.TestResultData{
		Name:    "t1",
		Passed:  true,
		Duration: 100 * time.Millisecond,
	}})
	if len(m.tests) != 1 {
		t.Fatalf("expected 1 test after result, got %d", len(m.tests))
	}
	if m.tests[0].result == nil || !m.tests[0].result.Passed {
		t.Fatal("expected test t1 to be marked as passed")
	}
}

func TestWindowResize(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 {
		t.Fatalf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Fatalf("expected height 40, got %d", m.height)
	}
}

func TestQuitKey(t *testing.T) {
	m := NewModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on quit key")
	}
	// tea.Quit is a tea.Cmd, calling it should produce a tea.QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}
