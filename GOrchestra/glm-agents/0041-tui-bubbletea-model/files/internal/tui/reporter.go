//go:build tui

package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

type prereqStartMsg struct{ Name string }
type prereqResultMsg struct{ Data reporter.PrereqResultData }
type testStartMsg struct{ Name string }
type testResultMsg struct{ Data reporter.TestResultData }
type summaryMsg struct{ Data reporter.SuiteResultData }
type RunnerDoneMsg struct{ Err error }
type rerunResultMsg struct {
	Name string
	Data reporter.TestResultData
}

// TUIReporter adapts the reporter.Reporter interface to send Bubbletea messages.
type TUIReporter struct {
	send func(tea.Msg)
}

// NewTUIReporter creates a reporter that forwards events as Bubbletea messages.
func NewTUIReporter(send func(tea.Msg)) *TUIReporter {
	return &TUIReporter{send: send}
}

func (r *TUIReporter) PrereqStart(name string)               { r.send(prereqStartMsg{Name: name}) }
func (r *TUIReporter) PrereqResult(d reporter.PrereqResultData) {
	r.send(prereqResultMsg{Data: d})
}
func (r *TUIReporter) TestStart(name string)                  { r.send(testStartMsg{Name: name}) }
func (r *TUIReporter) TestResult(d reporter.TestResultData)    { r.send(testResultMsg{Data: d}) }
func (r *TUIReporter) Summary(d reporter.SuiteResultData)      { r.send(summaryMsg{Data: d}) }
