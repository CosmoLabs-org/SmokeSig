//go:build tui

package tui

import (
	"context"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// State represents the TUI state machine.
type State int

const (
	StateRunning  State = iota
	StateResults
	StateRerunning
	StateError
	StateQuitting
)

// Filter controls which tests are visible.
type Filter int

const (
	FilterAll Filter = iota
	FilterFailed
	FilterPassed
	FilterSkipped
)

// RerunFunc runs tests by name and sends results back via the reporter.
// An empty slice means "run all tests."
type RerunFunc func(ctx context.Context, testNames []string) error

// Model is the Bubbletea model for the interactive TUI.
type Model struct {
	state       State
	filter      Filter
	cursor      int
	expanded    map[int]bool // indices of expanded tests (in filtered view)
	allExpanded bool

	// Data
	results  reporter.SuiteResultData
	tests    []reporter.TestResultData // current filtered view
	prereqs  []reporter.PrereqResultData
	project  string
	runError error

	// Accumulated results during RUNNING/RERUNNING
	pending []reporter.TestResultData

	// Capabilities
	rerunFunc RerunFunc

	// Layout
	width  int
	height int

	// UI state
	showHelp bool
	keys     keyMap
}

// NewModel creates a TUI model. rerunFunc is called to re-run tests.
func NewModel(project string, rerunFunc RerunFunc) Model {
	return Model{
		state:     StateRunning,
		project:   project,
		expanded:  make(map[int]bool),
		keys:      defaultKeyMap(),
		rerunFunc: rerunFunc,
	}
}

// SetRerunFunc sets the re-run callback after program creation.
func (m *Model) SetRerunFunc(fn RerunFunc) {
	m.rerunFunc = fn
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case testStartEvent:
		return m, nil

	case testResultEvent:
		m.pending = append(m.pending, msg.Data)
		if m.state == StateRunning || m.state == StateRerunning {
			m.tests = applyFilter(m.pending, m.filter)
		}
		return m, nil

	case prereqStartEvent:
		return m, nil

	case prereqResultEvent:
		m.prereqs = append(m.prereqs, msg.Data)
		return m, nil

	case summaryEvent:
		m.state = StateResults
		m.results = msg.Data
		m.results.Tests = m.pending // runner doesn't populate Tests
		m.tests = applyFilter(m.pending, m.filter)
		m.pending = nil
		m.clampCursor()
		return m, nil

	case RerunErrorEvent:
		m.state = StateError
		m.runError = msg.Err
		return m, nil

	case WatchTriggerEvent:
		return m.handleWatchTrigger()

	case rerunStartEvent:
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// In error state, any key dismisses the overlay and returns to results
	if m.state == StateError {
		m.state = StateResults
		m.runError = nil
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.state = StateQuitting
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil
	}

	// Navigation and action keys only work in RESULTS state
	if m.state != StateResults {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.tests)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		if len(m.tests) > 0 {
			m.cursor = len(m.tests) - 1
		}
		return m, nil

	case key.Matches(msg, m.keys.Toggle):
		m.expanded[m.cursor] = !m.expanded[m.cursor]
		return m, nil

	case key.Matches(msg, m.keys.ToggleAll):
		m.allExpanded = !m.allExpanded
		m.expanded = make(map[int]bool)
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		m.cycleFilter()
		return m, nil

	case key.Matches(msg, m.keys.Rerun):
		return m.startRerun(rerunCurrent)

	case key.Matches(msg, m.keys.RerunFails):
		return m.startRerun(rerunFailed)

	case key.Matches(msg, m.keys.RerunAll):
		return m.startRerun(rerunAll)
	}

	return m, nil
}

type rerunMode int

const (
	rerunCurrent rerunMode = iota
	rerunFailed
	rerunAll
)

func (m Model) startRerun(mode rerunMode) (tea.Model, tea.Cmd) {
	if m.rerunFunc == nil {
		return m, nil
	}

	var names []string
	switch mode {
	case rerunCurrent:
		if m.cursor < len(m.tests) {
			names = []string{m.tests[m.cursor].Name}
		}
	case rerunFailed:
		// Use the full result set (results.Tests), not the filtered view
		for _, t := range m.results.Tests {
			if !t.Passed && !t.Skipped {
				names = append(names, t.Name)
			}
		}
		if len(names) == 0 {
			return m, nil // nothing to re-run
		}
	case rerunAll:
		// empty names = run all
	}

	m.state = StateRerunning
	m.pending = nil
	m.expanded = make(map[int]bool)
	m.allExpanded = false

	rerunFn := m.rerunFunc
	return m, func() tea.Msg {
		if err := rerunFn(context.Background(), names); err != nil {
			return RerunErrorEvent{Err: err}
		}
		return nil
	}
}

func (m *Model) cycleFilter() {
	m.filter = (m.filter + 1) % 4 // All -> Failed -> Passed -> Skipped -> All
	allTests := m.results.Tests
	if m.state == StateRunning || m.state == StateRerunning {
		allTests = m.pending
	}
	m.tests = applyFilter(allTests, m.filter)
	m.cursor = 0
	m.expanded = make(map[int]bool)
	m.allExpanded = false
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.tests) {
		m.cursor = len(m.tests) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) handleWatchTrigger() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateResults, StateError:
		// Start a re-run on file change
		m.state = StateRerunning
		m.pending = nil
		m.expanded = make(map[int]bool)
		m.allExpanded = false
		rerunFn := m.rerunFunc
		if rerunFn == nil {
			return m, nil
		}
		return m, func() tea.Msg {
			if err := rerunFn(context.Background(), nil); err != nil {
				return RerunErrorEvent{Err: err}
			}
			return nil
		}
	default:
		// RUNNING or RERUNNING: ignore (tests already in progress)
		return m, nil
	}
}

func (m Model) View() string {
	if m.state == StateQuitting {
		return ""
	}

	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	if m.showHelp {
		return viewHelpFull(m)
	}

	if m.state == StateError {
		return viewError(m)
	}

	header := viewHeader(m)
	summary := viewSummary(m)
	shortcuts := viewShortcuts(m)

	// Calculate available height for test list
	headerLines := countLines(header)
	summaryLines := countLines(summary)
	shortcutLines := countLines(shortcuts)
	listHeight := m.height - headerLines - summaryLines - shortcutLines
	if listHeight < 1 {
		listHeight = 1
	}

	list := viewTestList(m, listHeight)

	return header + "\n" + list + "\n" + summary + "\n" + shortcuts
}
