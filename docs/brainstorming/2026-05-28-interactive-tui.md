---
title: "Interactive TUI with Bubbletea"
created: "2026-05-28T02:30:00-03:00"
status: APPROVED
tags: [tui, bubbletea, interactive, dx]
deliverables:
  - id: BR-01
    title: "TUI Bubbletea model with state management (model.go)"
  - id: BR-02
    title: "Reporter-to-tea.Msg adapter bridge (reporter.go)"
  - id: BR-03
    title: "Lipgloss TUI styles (styles.go)"
  - id: BR-04
    title: "Key binding definitions (keys.go)"
  - id: BR-05
    title: "Runner.RunSingle method for single-test re-run"
  - id: BR-06
    title: "Build-tagged cmd wiring (run_tui.go + run_tui_stub.go)"
  - id: BR-07
    title: "Model unit tests (model_test.go)"
---

# Interactive TUI with Bubbletea — Design Doc

## Status: APPROVED

## Problem

SmokeSig's terminal output is write-once: results scroll past and you can't interact with them. For projects with 20+ tests, finding failures means scrolling back. Re-running a single failure means re-running the entire suite. There's no way to filter, search, or drill into assertion details without piping to a file and grepping.

## Decision Record

| Question | Answer |
|----------|--------|
| Scope of v1 | All 5 features: navigate, expand/collapse, re-run failures, filter by status, live search |
| Entry point | `--tui` flag on `smokesig run` |
| Build tag | `//go:build tui` — Bubbletea stays optional, preserves minimal-dep philosophy |
| Watch mode | Not in v1 — single-run only |
| Dependency | `github.com/charmbracelet/bubbletea` + `github.com/charmbracelet/bubbles` (same Charm ecosystem as existing lipgloss) |

## Architecture

### How It Works

The TUI is a Bubbletea `tea.Program` that receives test results as messages from the runner. The runner executes in a background goroutine; its Reporter adapter converts `TestStart`/`TestResult` calls into `tea.Msg` values sent via `p.Send()`.

```
┌──────────────────┐     tea.Msg      ┌──────────────────┐
│  Runner goroutine │ ──────────────▸ │  Bubbletea loop  │
│  (runs tests)     │                  │  (renders TUI)   │
│                   │ ◂────────────── │                   │
│  RunSingle(name)  │   re-run req    │  User presses 'r' │
└──────────────────┘                  └──────────────────┘
```

### Build Tag Approach

All TUI code lives behind `//go:build tui`:

- `internal/tui/*.go` — model, reporter adapter, styles, keys
- `cmd/run_tui.go` — registers `--tui` flag, wires TUI reporter + program
- `cmd/run_tui_stub.go` (`//go:build !tui`) — prints error if `--tui` used without build tag

Users build with `go build -tags tui` to include it. Default binary is unchanged.

### Package Structure

```
internal/tui/
├── model.go       # tea.Model — state, Init, Update, View (~200 lines)
├── reporter.go    # Reporter interface adapter → tea.Msg bridge (~60 lines)
├── styles.go      # lipgloss styles for TUI elements (~30 lines)
├── keys.go        # key bindings via bubbles/key (~40 lines)
├── model_test.go  # Update logic tests (~200 lines)
```

### Model State

```go
type model struct {
    tests      []testItem              // accumulated test results
    cursor     int                     // current selection index
    expanded   map[int]bool            // expanded detail view per test
    filter     filterMode              // all | passed | failed | skipped
    searchMode bool                    // typing in search bar
    searchText string                  // current search query
    filtered   []int                   // visible indices after filter+search
    running    bool                    // suite still executing
    summary    *reporter.SuiteResultData // populated after suite completes
    rerunning  map[string]bool         // test names currently re-running
    width      int                     // terminal width (from WindowSizeMsg)
    height     int                     // terminal height
    err        error                   // fatal error from runner

    rerunFunc  func(string) reporter.TestResultData // callback to re-run a single test
}

type testItem struct {
    name     string
    started  bool                     // TestStart received
    result   *reporter.TestResultData // nil while running
}

type filterMode int
const (
    filterAll filterMode = iota
    filterPassed
    filterFailed
    filterSkipped
)
```

### Message Types

```go
type prereqStartMsg struct{ name string }
type prereqResultMsg struct{ data reporter.PrereqResultData }
type testStartMsg struct{ name string }
type testResultMsg struct{ data reporter.TestResultData }
type summaryMsg struct{ data reporter.SuiteResultData }
type runnerDoneMsg struct{ err error }
type rerunResultMsg struct{ name string; data reporter.TestResultData }
```

### TUI Reporter Adapter

```go
type tuiReporter struct {
    send func(tea.Msg)
}

func (r *tuiReporter) PrereqStart(name string)          { r.send(prereqStartMsg{name}) }
func (r *tuiReporter) PrereqResult(d PrereqResultData)   { r.send(prereqResultMsg{d}) }
func (r *tuiReporter) TestStart(name string)             { r.send(testStartMsg{name}) }
func (r *tuiReporter) TestResult(d TestResultData)       { r.send(testResultMsg{d}) }
func (r *tuiReporter) Summary(d SuiteResultData)         { r.send(summaryMsg{d}) }
```

### Re-run Mechanism

The runner already has `runTest(t schema.Test, opts RunOptions) TestResult` which executes a single test. For TUI re-run:

1. `Runner` gets a new exported method: `RunSingle(name string, opts RunOptions) (*TestResult, error)` — finds the test by name in `Config.Tests`, runs it via `runTestWithHooks`, returns the result.
2. The TUI model holds a `rerunFunc` closure that calls `RunSingle` and sends a `rerunResultMsg`.
3. When user presses `r` on a failed test, Update spawns `rerunFunc` in a `tea.Cmd` goroutine.
4. The result replaces the old entry in `tests[]`.

### Key Bindings

| Key | Action |
|-----|--------|
| `↑`/`k` | Move cursor up |
| `↓`/`j` | Move cursor down |
| `enter`/`space` | Toggle expand/collapse |
| `r` | Re-run selected test (failed/allowed-failure only) |
| `tab` | Cycle filter: all → passed → failed → skipped → all |
| `/` | Enter search mode |
| `esc` | Exit search mode / clear filter |
| `q`/`ctrl+c` | Quit |

### Screen Layout

```
┌─────────────────────────────────────────────┐
│ SmokeSig ─ my-project        4/6 ✓ 1 ✗ 1 ● │
├─────────────────────────────────────────────┤
│   ✓ health-check                     (42ms) │
│ ▸ ✗ api-status                      (128ms) │
│   │  http: expected 200, got 503            │
│   │  body_contains: expected "ok"           │
│   ● database-ping                  running… │
│   ⊘ redis-check                   (skipped) │
│     ssl-cert                        pending  │
│     grpc-health                     pending  │
├─────────────────────────────────────────────┤
│ ↑↓ navigate  ⏎ expand  r re-run  ⇥ filter │
│ / search   q quit         [filter: all]     │
└─────────────────────────────────────────────┘
```

- Header: project name + live counters
- Test list: scrollable, cursor-highlighted, icons match terminal reporter
- Expanded view: shows assertion details inline under the selected test
- Footer: key hints + active filter label
- Re-running test shows a spinner replacing its icon

### Wiring in cmd/run_tui.go

```go
//go:build tui

package cmd

func init() {
    // Register --tui flag on runCmd
    runCmd.Flags().BoolVar(&useTUI, "tui", false, "interactive TUI mode")
}

var useTUI bool
```

In `cmd/run.go`, the reporter construction checks `useTUI` (which is false when built without the tag, since the flag isn't registered and the var defaults to false). When true, it creates the Bubbletea program with the TUI reporter instead of the terminal reporter.

The `useTUI` variable itself lives in `cmd/run_tui.go` (tagged) with a corresponding `cmd/run_tui_stub.go` (`//go:build !tui`) that declares `var useTUI = false`.

### Testing Strategy

1. **Model tests** (`model_test.go`): test Update logic with synthetic messages — cursor movement, filtering, search, expand/collapse. No real Bubbletea program needed.
2. **Reporter adapter tests**: verify each Reporter method sends the correct message type.
3. **RunSingle test** (`runner_test.go`): verify single-test execution works for an existing test name and returns error for unknown name.
4. **Integration**: manual — build with `-tags tui`, run against a real config.

### Estimated Size

| File | Lines |
|------|-------|
| `internal/tui/model.go` | ~200 |
| `internal/tui/reporter.go` | ~60 |
| `internal/tui/styles.go` | ~30 |
| `internal/tui/keys.go` | ~40 |
| `internal/tui/model_test.go` | ~200 |
| `cmd/run_tui.go` | ~50 |
| `cmd/run_tui_stub.go` | ~15 |
| Runner `RunSingle` addition | ~25 |
| **Total** | **~620** |

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Bubbletea captures stdout — breaks other reporters | TUI mode is exclusive: `--tui` disables `--format` flags (only TUI renders) |
| Re-run while suite running could conflict | Disable `r` key until `summaryMsg` received (suite done) |
| Large test suites (100+ tests) | Bubbletea viewport handles scrolling natively; pagination if needed in v2 |
| Build tag confusion for users | Clear error message in stub: "rebuild with -tags tui to enable interactive mode" |

## Out of Scope (v2)

- Watch mode integration (`--watch --tui`)
- Run history / diff between runs
- Mouse support
- Export filtered results
- Prereq display in TUI (v1 runs prereqs before TUI starts)
