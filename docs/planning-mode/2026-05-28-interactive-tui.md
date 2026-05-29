---
brainstorm_ref: docs/brainstorming/2026-05-28-interactive-tui.md
completed: "2026-05-29"
created: "2026-05-28T02:32:00-03:00"
deliverables:
    - id: P-01
      title: Add Bubbletea + Bubbles dependencies to go.mod
    - id: P-02
      title: Runner.RunSingle method for single-test re-execution
    - id: P-03
      title: TUI model with full Update/View cycle
    - id: P-04
      title: Reporter adapter bridging runner events to tea.Msg
    - id: P-05
      title: Lipgloss styles and key bindings
    - id: P-06
      title: Build-tagged cmd wiring + stub
    - id: P-07
      title: Model unit tests for all interactive features
goals_completed: 0
goals_total: 0
issue: FEAT-051
related_prompts: []
requires_reading: []
roadmap: ROAD-085
schema_version: 1
status: COMPLETED
tags:
    - tui
    - bubbletea
    - implementation
title: Interactive TUI with Bubbletea — Implementation Plan
---

# Interactive TUI with Bubbletea — Implementation Plan

Design: `docs/brainstorming/2026-05-28-interactive-tui.md`
Issue: FEAT-051 | Roadmap: ROAD-085

## File Scope

### New Files
| File | Purpose | Build Tag |
|------|---------|-----------|
| `internal/tui/model.go` | Bubbletea model — state, Init, Update, View | `//go:build tui` |
| `internal/tui/reporter.go` | Reporter interface → tea.Msg bridge | `//go:build tui` |
| `internal/tui/styles.go` | Lipgloss styles for TUI elements | `//go:build tui` |
| `internal/tui/keys.go` | Key bindings via bubbles/key | `//go:build tui` |
| `internal/tui/model_test.go` | Update logic tests | `//go:build tui` |
| `cmd/run_tui.go` | Register `--tui` flag, create TUI program | `//go:build tui` |
| `cmd/run_tui_stub.go` | Declare `useTUI = false` when tag absent | `//go:build !tui` |

### Modified Files
| File | Change |
|------|--------|
| `internal/runner/runner.go` | Add `RunSingle(name string, opts RunOptions) (*TestResult, error)` method |
| `internal/runner/runner_test.go` | Test for `RunSingle` |
| `cmd/run.go` | Check `useTUI` before building reporter; wire TUI program when true |
| `go.mod` / `go.sum` | Add `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles` |

## Implementation Steps

### Step 1: Add dependencies + RunSingle (P-01, P-02)

**1a. Add Bubbletea dependency**
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
```

**1b. Add `RunSingle` to runner**

In `internal/runner/runner.go`, add after `RunMonorepo`:

```go
func (r *Runner) RunSingle(name string, opts RunOptions) (*TestResult, error) {
    for _, t := range r.Config.Tests {
        if t.Name == name {
            result := r.runTestWithHooks(t, opts)
            return &result, nil
        }
    }
    return nil, fmt.Errorf("test %q not found", name)
}
```

**1c. Test RunSingle** in `internal/runner/runner_test.go`:
- Happy path: known test name returns result
- Error path: unknown test name returns error

### Step 2: TUI styles and key bindings (P-05)

**2a. `internal/tui/styles.go`** — reuse the same color values as `terminal.go`:
- `passStyle` (green), `failStyle` (red), `skipStyle` (yellow), `dimStyle` (gray)
- `headerStyle` — bold + cyan for the top bar
- `cursorStyle` — reverse for highlighted row
- `footerStyle` — dim for key hints
- `searchStyle` — for the search bar highlight

**2b. `internal/tui/keys.go`** — define key.Binding values:
- `Up`/`k`, `Down`/`j` — cursor movement
- `Enter`/`Space` — toggle expand
- `r` — re-run (context: only when cursor is on a failed test and suite is done)
- `Tab` — cycle filter
- `/` — enter search
- `Esc` — exit search / clear filter
- `q`/`Ctrl+C` — quit

### Step 3: Reporter adapter (P-04)

**`internal/tui/reporter.go`**:

```go
type TUIReporter struct {
    send func(tea.Msg)
}

func NewTUIReporter(send func(tea.Msg)) *TUIReporter {
    return &TUIReporter{send: send}
}
```

Each Reporter method wraps the data in a typed message and calls `r.send()`. The `send` function is `program.Send` from Bubbletea.

### Step 4: TUI model (P-03) — the core

**`internal/tui/model.go`**:

**Init**: returns `nil` (no initial command — runner starts externally).

**Update** handles:
1. `tea.WindowSizeMsg` → store width/height
2. `testStartMsg` → append `testItem{name, started: true}` to tests, recompute filtered
3. `testResultMsg` → find item by name, set result, update counters, recompute filtered
4. `prereqStartMsg`/`prereqResultMsg` → store in separate prereq list (shown in header area)
5. `summaryMsg` → set `running = false`, store summary
6. `runnerDoneMsg` → set error if non-nil
7. `rerunResultMsg` → replace test result, clear rerunning flag
8. `tea.KeyMsg` — switch on key binding:
   - Up/Down: move cursor within `filtered` slice
   - Enter: toggle `expanded[cursor]`
   - `r`: if selected test failed && !running → spawn re-run tea.Cmd
   - Tab: cycle `filter`, recompute `filtered`
   - `/`: enter searchMode
   - Esc: exit searchMode or reset filter to `filterAll`
   - `q`/Ctrl+C: return `tea.Quit`
9. In searchMode, rune keys append to `searchText`, backspace removes, recompute filtered

**View** renders:
- Header line: project name + live counters (N/total, pass/fail/skip counts)
- Test list: iterate `filtered`, render each with icon + name + duration. Cursor row gets `cursorStyle`. Expanded rows show assertion details indented below.
- Footer: key hints + active filter label + search text if in searchMode

**Filtering logic** (`recomputeFiltered`):
1. Start with all test indices
2. Apply filter mode (skip items not matching pass/fail/skip status)
3. Apply search text (case-insensitive substring match on test name)
4. Store result in `filtered`; clamp cursor

### Step 5: Build-tagged cmd wiring (P-06)

**`cmd/run_tui_stub.go`** (`//go:build !tui`):
```go
package cmd
var useTUI bool
```

**`cmd/run_tui.go`** (`//go:build tui`):
```go
package cmd

import (
    "fmt"
    "os"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/CosmoLabs-org/SmokeSig/internal/tui"
    "github.com/CosmoLabs-org/SmokeSig/internal/runner"
)

var useTUI bool

func init() {
    runCmd.Flags().BoolVar(&useTUI, "tui", false, "interactive TUI mode")
}

func runWithTUI(r *runner.Runner, opts runner.RunOptions) error {
    m := tui.NewModel()
    p := tea.NewProgram(m, tea.WithAltScreen())

    rep := tui.NewTUIReporter(p.Send)
    r.Reporter = rep
    m.SetRerunFunc(func(name string) reporter.TestResultData {
        result, _ := r.RunSingle(name, opts)
        return toReporterResult(*result)
    })

    go func() {
        _, err := r.Run(opts)
        p.Send(tui.RunnerDoneMsg{Err: err})
    }()

    _, err := p.Run()
    return err
}
```

**`cmd/run.go`** modification — in the `runE` function, after config loading but before reporter construction:
```go
if useTUI {
    return runWithTUI(rn, opts)
}
```

### Step 6: Model unit tests (P-07)

**`internal/tui/model_test.go`** — test Update logic with synthetic messages:

| Test | Scenario |
|------|----------|
| `TestCursorMovement` | Send Up/Down keys, verify cursor position wraps correctly |
| `TestExpandCollapse` | Send Enter on a completed test, verify expanded state toggles |
| `TestFilterCycling` | Send Tab repeatedly, verify filter mode cycles all→passed→failed→skipped→all |
| `TestSearchMode` | Send `/`, type chars, verify filtered list narrows; Esc exits |
| `TestRerunDisabledWhileRunning` | Send `r` while `running=true`, verify no rerun spawned |
| `TestRerunOnFailedTest` | Setup completed model with a failed test, press `r`, verify rerunning flag set |
| `TestTestResultUpdatesCounters` | Send testResultMsg, verify pass/fail/skip counters update |
| `TestWindowResize` | Send WindowSizeMsg, verify width/height stored |
| `TestQuitKey` | Send `q`, verify tea.Quit returned |

## Execution Model

Steps 1-2 are independent and can run in parallel (separate files, no overlap).
Steps 3-4 depend on styles/keys (step 2) but are the core work.
Step 5 depends on model (step 4) being complete.
Step 6 can start as soon as model types exist (step 4).

**Recommended dispatch**: 4 GLM agents in 2 waves:
- Wave 1: [Step 1: deps + RunSingle] [Step 2: styles + keys]
- Wave 2: [Step 3+4: reporter + model] [Step 5+6: cmd wiring + tests]

## Verify

```bash
go build -tags tui ./...
go test -tags tui ./internal/tui/ -v
go test ./internal/runner/ -run TestRunSingle -v
```
