//go:build tui

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

type filterMode int

const (
	filterAll     filterMode = iota
	filterPassed
	filterFailed
	filterSkipped
)

type testItem struct {
	name    string
	started bool
	result  *reporter.TestResultData
}

type model struct {
	tests      []testItem
	cursor     int
	expanded   map[int]bool
	filter     filterMode
	searchMode bool
	searchText string
	filtered   []int
	running    bool
	summary    *reporter.SuiteResultData
	rerunning  map[string]bool
	width      int
	height     int
	rerunFunc  func(string) reporter.TestResultData
	err        error
}

// NewModel returns a Bubbletea model ready to receive test events.
func NewModel() model {
	return model{
		running:   true,
		expanded:  make(map[int]bool),
		rerunning: make(map[string]bool),
		filter:    filterAll,
	}
}

// SetRerunFunc stores the callback used to re-run a single test by name.
func (m *model) SetRerunFunc(fn func(string) reporter.TestResultData) {
	m.rerunFunc = fn
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case prereqStartMsg:
		// prereqs are not rendered in the test list

	case prereqResultMsg:
		// prereqs are not rendered in the test list

	case testStartMsg:
		m.tests = append(m.tests, testItem{name: msg.Name, started: true})
		m.recomputeFiltered()

	case testResultMsg:
		for i := range m.tests {
			if m.tests[i].name == msg.Data.Name {
				m.tests[i].result = &msg.Data
				break
			}
		}
		m.recomputeFiltered()

	case summaryMsg:
		m.summary = &msg.Data
		m.running = false

	case RunnerDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		m.running = false

	case rerunResultMsg:
		for i := range m.tests {
			if m.tests[i].name == msg.Name {
				m.tests[i].result = &msg.Data
				break
			}
		}
		delete(m.rerunning, msg.Name)
		m.recomputeFiltered()

	case tea.KeyMsg:
		switch {
		case keyMatches(msg, keys.Quit):
			return m, tea.Quit

		case keyMatches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case keyMatches(msg, keys.Down):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case keyMatches(msg, keys.Expand):
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				idx := m.filtered[m.cursor]
				m.expanded[idx] = !m.expanded[idx]
			}

		case keyMatches(msg, keys.Rerun):
			if !m.running && m.rerunFunc != nil && len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				idx := m.filtered[m.cursor]
				item := m.tests[idx]
				if item.result != nil && !item.result.Passed && !m.rerunning[item.name] {
					name := item.name
					m.rerunning[name] = true
					return m, func() tea.Msg {
						return rerunResultMsg{Name: name, Data: m.rerunFunc(name)}
					}
				}
			}

		case keyMatches(msg, keys.Filter):
			m.filter = (m.filter + 1) % (filterSkipped + 1)
			m.recomputeFiltered()

		case keyMatches(msg, keys.Search):
			m.searchMode = true
			m.searchText = ""

		case keyMatches(msg, keys.Escape):
			if m.searchMode {
				m.searchMode = false
				m.searchText = ""
				m.recomputeFiltered()
			}

		default:
			if m.searchMode {
				switch msg.String() {
				case "backspace":
					if len(m.searchText) > 0 {
						m.searchText = m.searchText[:len(m.searchText)-1]
					}
				case "enter":
					m.searchMode = false
				default:
					if len(msg.String()) == 1 {
						m.searchText += msg.String()
					}
				}
				m.recomputeFiltered()
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	if m.err != nil {
		b.WriteString(failStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Press q to quit"))
		return b.String()
	}

	// Header
	m.writeHeader(&b)
	b.WriteString(separator(m.width) + "\n")

	// Test list
	availHeight := m.height - 4 // header + separator + separator + footer
	if availHeight < 1 {
		availHeight = len(m.filtered)
	}
	start, end := m.visibleRange(availHeight)
	for i := start; i < end; i++ {
		idx := m.filtered[i]
		item := m.tests[idx]
		m.writeTestRow(&b, i, idx, item)
	}

	b.WriteString(separator(m.width) + "\n")

	// Footer
	m.writeFooter(&b)

	return b.String()
}

func (m model) writeHeader(b *strings.Builder) {
	if m.summary != nil {
		s := m.summary
		header := fmt.Sprintf("SmokeSig — %s    %s✓ %s✗ %s⊘",
			s.Project,
			passStyle.Render(fmt.Sprintf("%d", s.Passed)),
			failStyle.Render(fmt.Sprintf("%d", s.Failed)),
			skipStyle.Render(fmt.Sprintf("%d", s.Skipped)),
		)
		b.WriteString(headerStyle.Render(header))
	} else {
		completed := 0
		for _, t := range m.tests {
			if t.result != nil {
				completed++
			}
		}
		header := fmt.Sprintf("SmokeSig    %d/%d running", completed, len(m.tests))
		b.WriteString(headerStyle.Render(header))
	}
	b.WriteString("\n")
}

func (m model) writeTestRow(b *strings.Builder, viewIdx, realIdx int, item testItem) {
	icon := dimStyle.Render("●")
	name := item.name
	duration := ""

	if item.result != nil {
		r := item.result
		duration = formatDuration(r.Duration)
		switch {
		case r.Skipped:
			icon = skipStyle.Render("⊘")
		case r.Passed:
			icon = passStyle.Render("✓")
		case r.AllowedFailure:
			icon = skipStyle.Render("~")
		default:
			icon = failStyle.Render("✗")
		}
	} else if !item.started {
		icon = dimStyle.Render("○")
	}

	if m.rerunning[item.name] {
		icon = "⟳"
	}

	row := fmt.Sprintf("  %s %s", icon, name)
	if duration != "" {
		pad := m.width - lipgloss.Width(row) - len(duration)
		if pad < 1 {
			pad = 1
		}
		row += strings.Repeat(" ", pad) + dimStyle.Render(duration)
	}

	// Truncate to width
	if m.width > 0 && lipgloss.Width(row) > m.width {
		row = row[:m.width-1] + "…"
	}

	// Cursor highlight
	if viewIdx == m.cursor {
		row = cursorStyle.Render(row)
	}
	b.WriteString(row + "\n")

	// Expanded assertion details
	if m.expanded[realIdx] && item.result != nil {
		for _, a := range item.result.Assertions {
			style := passStyle
			if !a.Passed {
				style = failStyle
			}
			line := fmt.Sprintf("      %s expected %s, got %s", a.Type+":", a.Expected, a.Actual)
			if m.width > 0 && len(line) > m.width {
				line = line[:m.width-1] + "…"
			}
			b.WriteString(style.Render(line) + "\n")
		}
		if item.result.Error != nil {
			errLine := fmt.Sprintf("      error: %s", item.result.Error)
			b.WriteString(failStyle.Render(errLine) + "\n")
		}
	}
}

func (m model) writeFooter(b *strings.Builder) {
	var left, right string

	left = "↑/k ↓/j ⏎ ⇥ r q"
	if m.searchMode {
		right = searchStyle.Render("/"+m.searchText) + "▏"
	} else {
		switch m.filter {
		case filterAll:
			right = "all"
		case filterPassed:
			right = passStyle.Render("passed")
		case filterFailed:
			right = failStyle.Render("failed")
		case filterSkipped:
			right = skipStyle.Render("skipped")
		}
	}

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 2 {
		pad = 2
	}
	b.WriteString(footerStyle.Render(left) + strings.Repeat(" ", pad) + right + "\n")
}

func (m *model) recomputeFiltered() {
	m.filtered = m.filtered[:0]
	for i, t := range m.tests {
		if !m.passesFilter(t) {
			continue
		}
		if m.searchText != "" && !strings.Contains(strings.ToLower(t.name), strings.ToLower(m.searchText)) {
			continue
		}
		m.filtered = append(m.filtered, i)
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m model) passesFilter(t testItem) bool {
	if m.filter == filterAll {
		return true
	}
	if t.result == nil {
		return m.filter == filterAll
	}
	switch m.filter {
	case filterPassed:
		return t.result.Passed && !t.result.Skipped
	case filterFailed:
		return !t.result.Passed && !t.result.Skipped && !t.result.AllowedFailure
	case filterSkipped:
		return t.result.Skipped
	}
	return true
}

func (m model) visibleRange(maxLines int) (int, int) {
	n := len(m.filtered)
	if n <= maxLines {
		return 0, n
	}
	start := m.cursor - maxLines/2
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > n {
		end = n
		start = end - maxLines
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func separator(width int) string {
	if width <= 0 {
		width = 80
	}
	return dimStyle.Render(strings.Repeat("─", width))
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("(%dµs)", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("(%dms)", d.Milliseconds())
	}
	return fmt.Sprintf("(%.1fs)", d.Seconds())
}

// keyMatches checks if a key message matches a binding.
func keyMatches(msg tea.KeyMsg, b key.Binding) bool {
	for _, k := range b.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}
