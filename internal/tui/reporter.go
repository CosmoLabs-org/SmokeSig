//go:build tui

package tui

import (
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	tea "github.com/charmbracelet/bubbletea"
)

// TUIReporter implements reporter.Reporter by forwarding events
// to a Bubbletea program. The runner calls these methods from its
// goroutine; Program.Send is goroutine-safe.
type TUIReporter struct {
	program *tea.Program
}

// Compile-time interface check.
var _ reporter.Reporter = (*TUIReporter)(nil)

// NewTUIReporter creates a reporter that sends events to the given program.
func NewTUIReporter(p *tea.Program) *TUIReporter {
	return &TUIReporter{program: p}
}

func (t *TUIReporter) PrereqStart(name string) {
	t.program.Send(prereqStartEvent{Name: name})
}

func (t *TUIReporter) PrereqResult(r reporter.PrereqResultData) {
	t.program.Send(prereqResultEvent{Data: r})
}

func (t *TUIReporter) TestStart(name string) {
	t.program.Send(testStartEvent{Name: name})
}

func (t *TUIReporter) TestResult(r reporter.TestResultData) {
	t.program.Send(testResultEvent{Data: r})
}

func (t *TUIReporter) Summary(s reporter.SuiteResultData) {
	t.program.Send(summaryEvent{Data: s})
}
