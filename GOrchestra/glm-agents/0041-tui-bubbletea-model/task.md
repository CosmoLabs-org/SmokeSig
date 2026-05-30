Create the TUI reporter adapter and the main Bubbletea model.

These are the core TUI files. All files go in internal/tui/ with //go:build tui tag.

FILE 1: internal/tui/reporter.go

This adapts the reporter.Reporter interface to send Bubbletea messages:

```go
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
type rerunResultMsg struct{ Name string; Data reporter.TestResultData }

type TUIReporter struct {
    send func(tea.Msg)
}

func NewTUIReporter(send func(tea.Msg)) *TUIReporter {
    return &TUIReporter{send: send}
}

func (r *TUIReporter) PrereqStart(name string)               { r.send(prereqStartMsg{Name: name}) }
func (r *TUIReporter) PrereqResult(d reporter.PrereqResultData) { r.send(prereqResultMsg{Data: d}) }
func (r *TUIReporter) TestStart(name string)                  { r.send(testStartMsg{Name: name}) }
func (r *TUIReporter) TestResult(d reporter.TestResultData)    { r.send(testResultMsg{Data: d}) }
func (r *TUIReporter) Summary(d reporter.SuiteResultData)      { r.send(summaryMsg{Data: d}) }
```

FILE 2: internal/tui/model.go

The main Bubbletea model. Implements tea.Model (Init, Update, View).

State:
- tests []testItem — accumulated results. testItem has: name string, started bool, result *reporter.TestResultData
- cursor int — selected index in filtered view
- expanded map[int]bool — which real indices are expanded
- filter filterMode — iota: filterAll, filterPassed, filterFailed, filterSkipped
- searchMode bool, searchText string
- filtered []int — indices into tests[] after applying filter+search
- running bool (starts true)
- summary *reporter.SuiteResultData
- rerunning map[string]bool
- width, height int
- rerunFunc func(string) reporter.TestResultData
- err error

Constructor: NewModel() returns model with running=true, expanded/rerunning maps initialized, filter=filterAll.

SetRerunFunc(fn): stores the re-run callback.

Init(): return nil.

Update(msg tea.Msg):
- tea.WindowSizeMsg: store w/h
- testStartMsg: append testItem{name, true, nil}, recompute filtered
- testResultMsg: find by name, set result, recompute filtered
- summaryMsg: set summary, running=false
- RunnerDoneMsg: set err if non-nil, running=false
- rerunResultMsg: find by name, replace result, delete from rerunning, recompute
- tea.KeyMsg: handle keys per keyMap. For rerun: only if !running && selected test has result && !result.Passed && !rerunning[name] — then set rerunning[name]=true, return tea.Cmd that calls rerunFunc and sends rerunResultMsg.

View():
- If err != nil: show error and quit hint
- Header: "SmokeSig — {summary.Project}    {passed}✓ {failed}✗ {skipped}⊘" or "{completed}/{total} running" if running
- Separator line
- Test list: for each index in filtered, render the test row. Use passStyle/failStyle/skipStyle icons matching terminal.go. Cursor row gets cursorStyle. If expanded[realIndex], render assertion details indented. If rerunning[name], show spinner icon.
- Separator line
- Footer: key hints on left, filter label on right. If searchMode, show "/searchText" with cursor.

Helper recomputeFiltered():
- Walk tests, apply filter mode (filterPassed keeps result!=nil && result.Passed, etc)
- Apply searchText as case-insensitive substring on name
- Store matching indices in filtered, clamp cursor

Helper formatDuration(d time.Duration) string — same logic as terminal.go.

The View should handle terminal width gracefully — truncate long names, right-align durations.

Verify:
  go build -tags tui ./internal/tui/...

Commit via: ccs commit-batch --message "feat(tui): bubbletea model and reporter adapter (FEAT-051)"
