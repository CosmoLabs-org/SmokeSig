Create the build-tagged cmd wiring files and model unit tests.

FILE 1: cmd/run_tui_stub.go (//go:build !tui)

```go
//go:build !tui

package cmd

var useTUI bool
```

FILE 2: cmd/run_tui.go (//go:build tui)

```go
//go:build tui

package cmd

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/CosmoLabs-org/SmokeSig/internal/reporter"
    "github.com/CosmoLabs-org/SmokeSig/internal/runner"
    "github.com/CosmoLabs-org/SmokeSig/internal/tui"
)

var useTUI bool

func init() {
    runCmd.Flags().BoolVar(&useTUI, "tui", false, "interactive TUI mode")
}

func runWithTUI(rn *runner.Runner, opts runner.RunOptions) error {
    m := tui.NewModel()
    p := tea.NewProgram(m, tea.WithAltScreen())

    rep := tui.NewTUIReporter(p.Send)
    rn.Reporter = rep
    m.SetRerunFunc(func(name string) reporter.TestResultData {
        result, err := rn.RunSingle(name, opts)
        if err != nil {
            return reporter.TestResultData{Name: name, Passed: false, Error: err}
        }
        return toReporterResult(*result)
    })

    go func() {
        _, err := rn.Run(opts)
        p.Send(tui.RunnerDoneMsg{Err: err})
    }()

    _, err := p.Run()
    return err
}
```

NOTE: The toReporterResult function already exists in cmd/run.go at line 831 (approximately).
It converts runner.TestResult to reporter.TestResultData. It's already accessible from cmd/run_tui.go
since both are in the cmd package. Do NOT redeclare it.

NOTE: model.SetRerunFunc takes a pointer receiver (*model). NewModel returns model (value).
You need to take a pointer: `m := tui.NewModel()` then `m.SetRerunFunc(...)`. Since
SetRerunFunc is on *model, Go auto-takes the address. But the tea.NewProgram call needs
a tea.Model interface — which model (value receiver) satisfies. So: create m as value,
call SetRerunFunc on &m (or on m since Go auto-refs), pass m to tea.NewProgram. Check that
this compiles by reading the actual method signatures in internal/tui/model.go.

FILE 3: Modify cmd/run.go — In the runSmoke function, find where the runner is created
(`rn := &runner.Runner{...}`) and the reporter is built. Add a check for useTUI BEFORE
the buildReporter call. The pattern is:

```go
if useTUI {
    return runWithTUI(rn, opts)
}
```

Place this after `rn` is created but before `buildReporter`. Read cmd/run.go to find the
exact insertion point — look for the runner.Runner struct literal and the buildReporter call.

FILE 4: internal/tui/model_test.go (//go:build tui)

Test the Update logic by constructing model states and sending messages directly.
Do NOT use tea.NewProgram — call m.Update(msg) directly and check the returned model.

Tests to write:
- TestCursorMovement: create model, send testStartMsg for 3 tests, then testResultMsg for all 3.
  Send Down key twice (cursor=2), send Up (cursor=1). Assert cursor values.
- TestExpandCollapse: model with 1 completed test. Send Enter → expanded[0]=true.
  Send Enter again → expanded[0]=false.
- TestFilterCycling: model with 3 tests: 1 pass, 1 fail, 1 skip. Tab cycles
  filterAll(3 visible)→filterPassed(1)→filterFailed(1)→filterSkipped(1)→filterAll(3).
- TestSearchMode: send "/" key, verify searchMode=true. Send runes "a","p","i",
  verify searchText="api". Send Esc, verify searchMode=false and searchText="".
- TestRerunDisabledWhileRunning: model with running=true, send "r" — no rerun.
- TestRerunOnFailed: model with running=false, cursor on failed test, rerunFunc set.
  Send "r" — verify rerunning[name]=true and a tea.Cmd is returned.
- TestTestResultUpdates: send testStartMsg{Name:"t1"} then testResultMsg with t1 passing.
  Verify len(tests)==1 and tests[0].result.Passed==true.
- TestWindowResize: send tea.WindowSizeMsg{Width:120, Height:40}, verify stored.
- TestQuitKey: send KeyMsg for "q", verify it returns a tea.Quit batch command (non-nil cmd).

Import tea "github.com/charmbracelet/bubbletea" and use tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
for key simulation. For special keys use tea.KeyMsg{Type: tea.KeyUp} etc.

Actually, the simplest approach for key messages in bubbletea tests:
  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}} for regular keys
  tea.KeyMsg{Type: tea.KeyUp} for arrow keys
  tea.KeyMsg{Type: tea.KeyEnter} for enter
  tea.KeyMsg{Type: tea.KeyTab} for tab
  tea.KeyMsg{Type: tea.KeyEsc} for escape

Read internal/tui/model.go to understand the exact types and method signatures before writing tests.

Verify:
  go test -tags tui ./internal/tui/ -v
  go build -tags tui ./...

Commit via: ccs commit-batch --message "feat(tui): cmd wiring and model unit tests (FEAT-051)"
