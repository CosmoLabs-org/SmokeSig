---
brainstorm: docs/brainstorming/2026-05-29-interactive-tui-runner.md
created: "2026-05-29T12:00:00-03:00"
issue: FEAT-051
status: PENDING
deliverables:
  - id: P-01
    title: "Build tag setup and Bubbletea dependency"
  - id: P-02
    title: "TUI Model, state machine, and key bindings"
  - id: P-03
    title: "TUI Reporter implementing reporter.Reporter"
  - id: P-04
    title: "Runner TestNames filter for re-run support"
  - id: P-05
    title: "cmd/run_tui.go integration with --interactive flag"
  - id: P-06
    title: "Watch mode integration"
  - id: P-07
    title: "Tests and build verification"
---

# FEAT-051: Interactive TUI Test Runner — Implementation Plan

## Goal

Add a full-screen interactive TUI to SmokeSig behind a `-tags tui` build tag. The TUI provides cursor navigation through test results, expand/collapse details, filter by status, re-run individual tests or failures, and integrates with `--watch` mode. When the `tui` tag is absent, the binary is unchanged.

## Review Fixes Incorporated

These corrections from the brainstorm review are baked into this plan:

1. **`TestNames []string` on `RunOptions`** — P-04 adds name-based filtering to the runner for re-run support.
2. **No `chain_tui.go`** — TUI is a UI mode, not a format. The `--interactive` flag is handled entirely in `cmd/run_tui.go`. No registration in the reporter chain.
3. **Goroutine lifecycle** — The model ignores new re-run requests while in `RERUNNING` state. An `ERROR` display state is added for runner failures.
4. **Context-based cancellation** — **Deferred to v2.** Adding `Context context.Context` to `RunOptions` and threading it through `Run()` requires changes to every assertion executor. For v1, quit terminates the Bubbletea program (which exits the process); in-flight goroutines are cleaned up by OS process exit. The `cancelFn` field is removed from Model to avoid implying cancellation works. Future work: add `RunOptions.Context`, check `ctx.Done()` before each test in `Run()`, wire `context.WithCancel` in `runInteractive`.
5. **State machine completeness** — `RUNNING->QUITTING` and `RERUNNING->QUITTING` transitions are explicit.

---

## P-01: Build Tag Setup and Bubbletea Dependency

**Objective:** Add Bubbletea + Bubbles dependencies gated behind `//go:build tui`. Create the `internal/tui/` package skeleton.

| File | Action | Build Tag |
|------|--------|-----------|
| `go.mod` | Add `bubbletea` v1.x, `bubbles` v0.x | — |
| `internal/tui/doc.go` | Package doc + build tag | `tui` |
| `cmd/run_notui.go` | Stub: `--interactive` errors without tag | `!tui` |

### Steps

- [ ] Add dependencies:
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go mod tidy
```

- [ ] Create `internal/tui/doc.go`:
```go
//go:build tui

// Package tui provides an interactive full-screen test result viewer
// using Bubbletea. It is only included when built with -tags tui.
package tui
```

- [ ] Create `cmd/run_notui.go` (the stub for non-TUI builds):
```go
//go:build !tui

package cmd

import "fmt"

// interactive is always false when built without the tui tag.
var interactive bool

func runInteractive(_ *runContext) error {
	return fmt.Errorf("interactive mode requires building with -tags tui\n  go build -tags tui -o smokesig .")
}
```

- [ ] Verify default build is unaffected:
```bash
go build ./...
go test ./...
```

- [ ] Verify TUI build pulls dependencies:
```bash
go build -tags tui ./...
```

**Commit:** `feat(tui): add bubbletea dependency and build tag skeleton`

---

## P-02: TUI Model, State Machine, and Key Bindings

**Objective:** Implement the Bubbletea `Model` with a 5-state machine (RUNNING, RESULTS, RERUNNING, ERROR, QUITTING), key bindings, and view rendering.

| File | Action | Lines (est) |
|------|--------|-------------|
| `internal/tui/model.go` | Model struct, Init, Update, View | ~200 |
| `internal/tui/keymap.go` | Key binding definitions | ~45 |
| `internal/tui/styles.go` | Lipgloss styles (ANSI 16-color) | ~35 |
| `internal/tui/views.go` | Header, test list, detail, summary, help | ~210 |
| `internal/tui/events.go` | Custom Bubbletea message types | ~55 |

All files have `//go:build tui`.

### State Machine

```
RUNNING ──────> RESULTS ──────> RERUNNING ──────> RESULTS
   |               |                |
   v               v                v
QUITTING       QUITTING         QUITTING
                   |
                   v
                 ERROR ──(any key)──> RESULTS
```

Transitions:
- `RUNNING -> RESULTS`: `summaryEvent` received (all tests done)
- `RESULTS -> RERUNNING`: user presses `r`/`R`/`a` (re-run requested)
- `RESULTS -> QUITTING`: user presses `q`/`Ctrl+C`
- `RERUNNING -> RESULTS`: new `summaryEvent` received
- `RERUNNING -> RERUNNING`: **ignored** — new re-run requests while already rerunning are dropped
- `RUNNING -> QUITTING`: user presses `q`/`Ctrl+C`
- `RERUNNING -> QUITTING`: user presses `q`/`Ctrl+C`
- `RESULTS -> ERROR`: `RerunErrorEvent` received (runner returned error)
- `ERROR -> RESULTS`: any key press dismisses error overlay

### Steps

- [ ] Create `internal/tui/events.go`:
```go
//go:build tui

package tui

import (
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

// Events sent from TUIReporter to the Bubbletea program.
type testStartEvent struct{ Name string }
type testResultEvent struct{ Data reporter.TestResultData }
type prereqStartEvent struct{ Name string }
type prereqResultEvent struct{ Data reporter.PrereqResultData }
type summaryEvent struct{ Data reporter.SuiteResultData }

// Events for re-run lifecycle.
type rerunStartEvent struct{}
type RerunErrorEvent struct{ Err error }

// Watch mode trigger.
type WatchTriggerEvent struct{}

// Window size is handled by tea.WindowSizeMsg (built-in).
```

- [ ] Create `internal/tui/keymap.go`:
```go
//go:build tui

package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Top        key.Binding
	Bottom     key.Binding
	Toggle     key.Binding
	ToggleAll  key.Binding
	Filter     key.Binding
	Rerun      key.Binding
	RerunFails key.Binding
	RerunAll   key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/Up", "move up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/Down", "move down")),
		Top:        key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g/Home", "first test")),
		Bottom:     key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/End", "last test")),
		Toggle:     key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("Enter", "expand/collapse")),
		ToggleAll:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "expand/collapse all")),
		Filter:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "cycle filter")),
		Rerun:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rerun current")),
		RerunFails: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerun failures")),
		RerunAll:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "rerun all")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
```

- [ ] Create `internal/tui/styles.go`:
```go
//go:build tui

package tui

import "github.com/charmbracelet/lipgloss"

// Styles mirror the ANSI 16-color palette from terminal.go.
var (
	stylePass    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	styleFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // red
	styleSkip    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // yellow
	styleAllowed = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))  // cyan
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")) // blue
	styleHeader  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	styleSummary = lipgloss.NewStyle().Padding(0, 1)
	styleHelp    = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)
```

- [ ] Create `internal/tui/model.go`:
```go
//go:build tui

package tui

import (
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
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

// RerunFunc runs tests by name and sends results back via the reporter channel.
// An empty slice means "run all tests."
type RerunFunc func(ctx context.Context, testNames []string) error

// Model is the Bubbletea model for the interactive TUI.
type Model struct {
	state     State
	filter    Filter
	cursor    int
	expanded  map[int]bool  // indices of expanded tests (in filtered view)
	allExpanded bool

	// Data
	results   reporter.SuiteResultData
	tests     []reporter.TestResultData // current filtered view
	prereqs   []reporter.PrereqResultData
	project   string
	runError  error

	// Accumulated results during RUNNING/RERUNNING
	pending []reporter.TestResultData

	// Capabilities
	rerunFunc RerunFunc
	// NOTE: cancelFn removed — context cancellation deferred to v2 (see review fix 4)

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
		state:    StateRunning,
		project:  project,
		expanded: make(map[int]bool),
		keys:     defaultKeyMap(),
		rerunFunc: rerunFunc,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
```

The `Update` method handles all state transitions:
- On `tea.WindowSizeMsg`: store dimensions.
- On `testResultEvent`: append to `pending`, rebuild filtered view.
- On `summaryEvent`: transition to `StateResults`, populate tests from accumulated `pending` (NOT from `summaryEvent.Data.Tests` — the runner does not populate that field), apply filter, clear pending:
```go
case summaryEvent:
    m.state = StateResults
    m.results = msg.Data
    m.results.Tests = m.pending  // runner doesn't populate Tests
    m.tests = applyFilter(m.pending, m.filter)
    m.pending = nil
```
> **Critical: `SuiteResultData.Tests` is unpopulated.** The runner constructs `summaryData` at `runner.go:157-170` WITHOUT setting the `Tests` field — it is always an empty slice in production. The TUI model MUST use its own accumulated `pending` results (from `testResultEvent` messages) as the source of truth.

- On `RerunErrorEvent`: transition to `StateError`, store error.
- On key press: dispatch based on `state` + key binding. Re-run keys are **no-ops** in `RUNNING` and `RERUNNING` states. `Quit` returns `tea.Quit` (context cancellation deferred to v2 — process exit handles cleanup for v1).

The `View` method delegates to `views.go` functions based on state.

- [ ] Create `internal/tui/views.go` with functions:
  - `viewHeader(m Model) string` — project name, state indicator (RUNNING spinner / PASS/FAIL badge), test count
  - `viewTestList(m Model) string` — scrollable test list with cursor, status icons (checkmark/cross/skip/tilde), duration, expanded detail
  - `viewDetail(test reporter.TestResultData) string` — assertion list with expected/actual, error message
  - `viewSummary(m Model) string` — total/passed/failed/skipped/duration bar
  - `viewShortcuts(m Model) string` — `[f]ilter [r]erun [R]erun-fails [a]ll [q]uit [?]`
  - `viewHelp(m Model) string` — full help overlay (all key bindings)
  - `viewError(m Model) string` — error message overlay

Filter logic in a helper:
```go
func applyFilter(tests []reporter.TestResultData, f Filter) []reporter.TestResultData {
	if f == FilterAll {
		return tests
	}
	var out []reporter.TestResultData
	for _, t := range tests {
		switch f {
		case FilterFailed:
			if !t.Passed && !t.Skipped {
				out = append(out, t)
			}
		case FilterPassed:
			if t.Passed {
				out = append(out, t)
			}
		case FilterSkipped:
			if t.Skipped {
				out = append(out, t)
			}
		}
	}
	return out
}
```

Scrolling: compute visible window from `m.height` (minus header/summary/shortcut rows), clamp cursor within `[0, len(filtered)-1]`, compute scroll offset so cursor is always visible.

- [ ] Verify the package compiles under the build tag:
```bash
go build -tags tui ./internal/tui/
```

**Commit:** `feat(tui): implement model, state machine, views, and key bindings`

---

## P-03: TUI Reporter Implementing reporter.Reporter

**Objective:** Create `TUIReporter` that implements the `reporter.Reporter` interface and bridges runner events into the Bubbletea program via `tea.Program.Send()`.

| File | Action | Lines (est) |
|------|--------|-------------|
| `internal/tui/reporter.go` | Reporter implementation | ~65 |

### Steps

- [ ] Create `internal/tui/reporter.go`:
```go
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
```

Key design note: `tea.Program.Send()` is goroutine-safe (documented by Bubbletea). The runner calls reporter methods from its own goroutine (or parallel goroutines). Each `Send` enqueues a message into the Bubbletea event loop, which processes it in the `Update` method on the main goroutine. No channels or mutexes needed.

- [ ] Verify interface compliance (add to reporter_test or as a compile check):
```go
var _ reporter.Reporter = (*TUIReporter)(nil)
```

- [ ] Build check:
```bash
go build -tags tui ./internal/tui/
```

**Commit:** `feat(tui): add TUIReporter bridging runner events to bubbletea`

---

## P-04: Runner TestNames Filter for Re-Run Support

**Objective:** Add `TestNames []string` to `runner.RunOptions` so the TUI can re-run specific tests by name.

| File | Action |
|------|--------|
| `internal/runner/runner.go` | Add `TestNames` to `RunOptions`, filter in `Run()` |
| `internal/runner/runner_test.go` | Test name filtering |

### Steps

- [ ] Add field to `RunOptions`:
```go
type RunOptions struct {
	Tags        []string
	ExcludeTags []string
	TestNames   []string  // If non-empty, only run tests with these exact names
	FailFast    bool
	DryRun      bool
	Timeout     time.Duration
}
```

- [ ] Add name-based filter function:
```go
// filterByName returns only tests whose names appear in the names slice.
// If names is empty, all tests are returned.
func filterByName(tests []schema.Test, names []string) []schema.Test {
	if len(names) == 0 {
		return tests
	}
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}
	var filtered []schema.Test
	for _, t := range tests {
		if _, ok := nameSet[t.Name]; ok {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
```

- [ ] Apply in `Run()`, after existing tag filter (line ~128):
```go
// Filter tests by tags
tests := filterTests(r.Config.Tests, opts.Tags, opts.ExcludeTags)

// Filter by specific test names (for TUI re-run)
tests = filterByName(tests, opts.TestNames)
```

- [ ] Add tests in `internal/runner/runner_test.go`:
```go
func TestFilterByName(t *testing.T) {
	tests := []schema.Test{
		{Name: "alpha"}, {Name: "beta"}, {Name: "gamma"},
	}

	// Empty names = all tests
	got := filterByName(tests, nil)
	if len(got) != 3 { t.Errorf("expected 3, got %d", len(got)) }

	// Specific names
	got = filterByName(tests, []string{"beta"})
	if len(got) != 1 || got[0].Name != "beta" { t.Errorf("expected [beta], got %v", got) }

	// Multiple names
	got = filterByName(tests, []string{"alpha", "gamma"})
	if len(got) != 2 { t.Errorf("expected 2, got %d", len(got)) }

	// Non-existent name
	got = filterByName(tests, []string{"delta"})
	if len(got) != 0 { t.Errorf("expected 0, got %d", len(got)) }
}
```

- [ ] Run tests:
```bash
go test ./internal/runner/ -run TestFilterByName -v
```

**Commit:** `feat(runner): add TestNames filter to RunOptions for re-run support`

---

## P-05: cmd/run_tui.go Integration with --interactive Flag

**Objective:** Wire the `--interactive` flag into the `run` command. When set, construct the Bubbletea program, wire TUI reporter (replacing terminal reporter slot), and manage the program lifecycle. Non-terminal formats (json, junit, etc.) still work alongside via `MultiReporter`.

| File | Action | Build Tag |
|------|--------|-----------|
| `cmd/run_tui.go` | Flag registration, `runInteractive`, TUI bootstrap | `tui` |
| `cmd/run.go` | Add early exit to `runSmoke` when `interactive` is set | — |

### Steps

- [ ] Add the `interactive` check to `runSmoke` in `cmd/run.go`. Insert after the `loadConfig` / `configDir` setup (around line 216), before monorepo/watch handling:
```go
// Interactive TUI mode (requires -tags tui)
if interactive {
    return runInteractive(&runContext{
        cfg:       cfg,
        configDir: configDir,
        opts: runner.RunOptions{
            Tags:        tags,
            ExcludeTags: excludeTags,
            FailFast:    failFast,
            DryRun:      dryRun,
            Timeout:     timeoutDur,
        },
        formatStr: format,
        watch:     watch,
    })
}
```

Add the `runContext` struct in `run.go` (used by both `run_tui.go` and `run_notui.go`):
```go
// runContext carries shared state for the interactive TUI entry point.
type runContext struct {
	cfg       *schema.SmokeConfig
	configDir string
	opts      runner.RunOptions
	formatStr string
	watch     bool
}
```

- [ ] Create `cmd/run_tui.go`:
```go
//go:build tui

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func init() {
	runCmd.Flags().BoolVar(&interactive, "interactive", false,
		"Full-screen TUI for navigating results and re-running tests")
}

func runInteractive(rc *runContext) error {
	// TTY guard: fall back to terminal reporter if stdout is not a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, "warning: --interactive ignored (stdout is not a TTY)")
		interactive = false
		return runSmoke(runCmd, nil)
	}

	// TERM=dumb guard
	if os.Getenv("TERM") == "dumb" {
		fmt.Fprintln(os.Stderr, "warning: --interactive ignored (TERM=dumb)")
		interactive = false
		return runSmoke(runCmd, nil)
	}

	// Build rerun function that the TUI model calls
	rerunFunc := func(ctx context.Context, testNames []string) error {
		opts := rc.opts
		opts.TestNames = testNames
		r := &runner.Runner{
			Config:    rc.cfg,
			ConfigDir: rc.configDir,
			// Reporter is set below after program creation
		}
		// Reporter will be set by the caller (the TUI wires it)
		_, err := r.Run(opts)
		return err
	}

	// Create the Bubbletea model
	model := tui.NewModel(rc.cfg.Project, rerunFunc)

	// Create the Bubbletea program with alternate screen
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Create TUI reporter that sends events to the program
	tuiRep := tui.NewTUIReporter(program)

	// Build secondary reporters (json, junit, etc.) excluding terminal
	var finalRep reporter.Reporter = tuiRep
	if rc.formatStr != "" && rc.formatStr != "terminal" {
		// ChainWithVerbosity returns (Reporter, []io.Closer, error)
		secondaryRep, closers, err := reporter.ChainWithVerbosity(
			rc.formatStr, os.Stdout, verbosity,
		)
		if err == nil && secondaryRep != nil {
			finalRep = reporter.NewMultiReporter(tuiRep, secondaryRep)
			defer func() {
				for i := len(closers) - 1; i >= 0; i-- {
					closers[i].Close()
				}
			}()
		}
	}

	// Wire the reporter into a runner and start tests in a goroutine
	r := &runner.Runner{
		Config:    rc.cfg,
		Reporter:  finalRep,
		ConfigDir: rc.configDir,
	}

	go func() {
		if _, err := r.Run(rc.opts); err != nil {
			program.Send(tui.RerunErrorEvent{Err: err})
		}
	}()

	// Run the Bubbletea program (blocks until quit)
	_, err := program.Run()
	return err
}
```

Note: The `rerunFunc` needs the TUI reporter wired in. The actual implementation will capture the `program` reference and create a new `TUIReporter` for each re-run, or reuse the existing one (since `Send` is goroutine-safe). The exact wiring is:

```go
rerunFunc := func(ctx context.Context, testNames []string) error {
    opts := rc.opts
    opts.TestNames = testNames
    r := &runner.Runner{
        Config:    rc.cfg,
        Reporter:  tuiRep, // same reporter, sends to same program
        ConfigDir: rc.configDir,
    }
    _, err := r.Run(opts)
    return err
}
```

This requires constructing `rerunFunc` after `tuiRep` exists. Use a closure variable or a two-phase init (model sets rerunFunc after program creation). The model should have a `SetRerunFunc` method.

- [ ] Add to `Model`:
```go
func (m *Model) SetRerunFunc(fn RerunFunc) {
	m.rerunFunc = fn
}
```

- [ ] Verify `RerunErrorEvent` and `WatchTriggerEvent` are exported in `events.go` (done in P-02 — `cmd/run_tui.go` references them as `tui.RerunErrorEvent` / `tui.WatchTriggerEvent`).

- [ ] Verify build:
```bash
go build -tags tui ./...
```

- [ ] Verify non-TUI build (no `--interactive` flag visible):
```bash
go build ./...
./smokesig run --help | grep -c interactive  # should be 0
```

**Commit:** `feat(tui): wire --interactive flag with bubbletea program lifecycle`

---

## P-06: Watch Mode Integration

**Objective:** When `--watch --interactive` is used together, the TUI owns the terminal for the entire watch session. File change events send a `watchTriggerEvent` to the Bubbletea program instead of calling `runOnce`.

| File | Action |
|------|--------|
| `cmd/run_tui.go` | Add `runWatchInteractive` function |
| `internal/tui/model.go` | Handle `WatchTriggerEvent` in `Update` |

### Steps

- [ ] Add `runWatchInteractive` to `cmd/run_tui.go`:
```go
func runWatchInteractive(rc *runContext, program *tea.Program, configDir string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer w.Close()

	if err := w.Add(configDir); err != nil {
		return fmt.Errorf("watching %s: %w", configDir, err)
	}

	debounce := 500 * time.Millisecond
	var timer *time.Timer

	// The Bubbletea program runs on the main goroutine.
	// This function runs in a separate goroutine, sending watch events.
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isRelevantEvent(ev.Op) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, func() {
				program.Send(tui.WatchTriggerEvent{})
			})
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			program.Send(tui.RerunErrorEvent{Err: fmt.Errorf("watch: %w", err)})
		}
	}
}
```

- [ ] In `runInteractive`, when `rc.watch` is true, start the watcher in a goroutine before `program.Run()`:
```go
if rc.watch {
    go func() {
        if err := runWatchInteractive(rc, program, rc.configDir); err != nil {
            program.Send(tui.RerunErrorEvent{Err: err})
        }
    }()
}
```

- [ ] Handle `WatchTriggerEvent` in `Model.Update`:
  - If state is `RESULTS`: transition to `RERUNNING`, clear pending results, invoke `rerunFunc(context.Background(), nil)` in a goroutine (nil = all tests; v1 uses Background context — cancellation deferred to v2).
  - If state is `RUNNING` or `RERUNNING`: **ignore** (debounce — tests are already in progress).
  - If state is `ERROR`: transition to `RERUNNING` (recover from error on file change).

- [ ] Verify `WatchTriggerEvent` is exported in `events.go` (done in P-02).

- [ ] Manual verification (requires a real project with `.smokesig.yaml`):
```bash
go build -tags tui -o smokesig .
./smokesig run --interactive --watch
# Edit a file -> TUI should re-run in-place
```

**Commit:** `feat(tui): integrate watch mode with interactive TUI`

---

## P-07: Tests and Build Verification

**Objective:** Unit tests for model update logic, view rendering, filter function, and reporter bridge. Verify both tagged and untagged builds.

| File | Action | Build Tag |
|------|--------|-----------|
| `internal/tui/model_test.go` | State machine tests | `tui` |
| `internal/tui/views_test.go` | View rendering tests | `tui` |
| `internal/tui/reporter_test.go` | Interface compliance | `tui` |

### Steps

- [ ] Create `internal/tui/model_test.go`:
```go
//go:build tui

package tui

import (
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	tea "github.com/charmbracelet/bubbletea"
)

func TestStateTransitions(t *testing.T) {
	m := NewModel("test-project", nil)
	if m.state != StateRunning {
		t.Fatalf("initial state: got %v, want StateRunning", m.state)
	}

	// Simulate test results arriving via testResultEvent (runner doesn't populate SuiteResultData.Tests)
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

	// RUNNING -> RESULTS on summary (Tests field intentionally empty — mirrors production)
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

	// RESULTS -> QUITTING on quit key
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.state != StateQuitting {
		t.Fatalf("after quit: got %v, want StateQuitting", m.state)
	}
	if cmd == nil {
		t.Fatal("quit should return tea.Quit cmd")
	}
}

func TestFilterCycle(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateResults
	// Simulate post-summary state: results.Tests populated from pending by Update handler
	allTests := []reporter.TestResultData{
		{Name: "a", Passed: true},
		{Name: "b", Passed: false},
		{Name: "c", Skipped: true},
	}
	m.results = reporter.SuiteResultData{Tests: allTests}
	m.tests = allTests

	// Cycle: All -> Failed -> Passed -> Skipped -> All
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.filter != FilterFailed {
		t.Fatalf("first f: got %v, want FilterFailed", m.filter)
	}
	if len(m.tests) != 1 || m.tests[0].Name != "b" {
		t.Fatalf("failed filter: got %v", m.tests)
	}
}

func TestRerunIgnoredDuringRerunning(t *testing.T) {
	m := NewModel("test", nil)
	m.state = StateRerunning

	// r key should be ignored while rerunning
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
}
```

- [ ] Create `internal/tui/views_test.go`:
```go
//go:build tui

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

func TestApplyFilter(t *testing.T) {
	tests := []reporter.TestResultData{
		{Name: "pass1", Passed: true},
		{Name: "pass2", Passed: true},
		{Name: "fail1", Passed: false},
		{Name: "skip1", Skipped: true},
	}

	if got := applyFilter(tests, FilterAll); len(got) != 4 {
		t.Errorf("FilterAll: got %d, want 4", len(got))
	}
	if got := applyFilter(tests, FilterFailed); len(got) != 1 {
		t.Errorf("FilterFailed: got %d, want 1", len(got))
	}
	if got := applyFilter(tests, FilterPassed); len(got) != 2 {
		t.Errorf("FilterPassed: got %d, want 2", len(got))
	}
	if got := applyFilter(tests, FilterSkipped); len(got) != 1 {
		t.Errorf("FilterSkipped: got %d, want 1", len(got))
	}
}

func TestViewSummaryContainsCounts(t *testing.T) {
	m := NewModel("test-project", nil)
	m.state = StateResults
	m.width = 80
	m.results = reporter.SuiteResultData{
		Total: 10, Passed: 8, Failed: 1, Skipped: 1,
		Duration: 2 * time.Second,
	}

	view := viewSummary(m)
	for _, want := range []string{"10", "8", "1"} {
		if !strings.Contains(view, want) {
			t.Errorf("summary missing %q: %s", want, view)
		}
	}
}
```

- [ ] Create `internal/tui/reporter_test.go`:
```go
//go:build tui

package tui

import "github.com/CosmoLabs-org/SmokeSig/internal/reporter"

// Compile-time interface check.
var _ reporter.Reporter = (*TUIReporter)(nil)
```

- [ ] Run all TUI tests:
```bash
go test -tags tui ./internal/tui/ -v
```

- [ ] Run existing test suite (ensure no regressions):
```bash
go test ./...
```

- [ ] Verify both builds:
```bash
# Default build — no TUI, no bubbletea linked
go build -o smokesig-default .
./smokesig-default run --help | grep -c interactive  # 0

# TUI build — bubbletea linked, --interactive available
go build -tags tui -o smokesig-tui .
./smokesig-tui run --help | grep -c interactive  # 1
```

- [ ] Verify self-smoke still passes:
```bash
./smokesig-default run
```

**Commit:** `test(tui): add model, view, filter, and reporter tests`

---

## Execution Notes

### Dependency Order

```
P-01 (skeleton) -> P-04 (runner filter, independent) -> P-02 (model) -> P-03 (reporter) -> P-05 (cmd wiring) -> P-06 (watch) -> P-07 (tests)
```

P-04 (runner `TestNames`) has no dependency on Bubbletea and can be done in parallel with P-01/P-02.

### Parallelization Opportunities

| Parallel Group | Tasks |
|----------------|-------|
| Group A | P-01 (build tag skeleton) + P-04 (runner TestNames filter) |
| Group B (after A) | P-02 (model) + P-03 (reporter) — can be developed concurrently |
| Sequential (after B) | P-05 (cmd wiring) -> P-06 (watch) -> P-07 (tests) |

### Key Files Modified (Existing)

| File | Change |
|------|--------|
| `go.mod` / `go.sum` | Bubbletea + bubbles deps |
| `internal/runner/runner.go` | `TestNames` field on `RunOptions`, `filterByName()` |
| `cmd/run.go` | `runContext` struct, `interactive` early-exit branch |

### Key Files Created (New)

| File | Build Tag |
|------|-----------|
| `internal/tui/doc.go` | `tui` |
| `internal/tui/model.go` | `tui` |
| `internal/tui/events.go` | `tui` |
| `internal/tui/keymap.go` | `tui` |
| `internal/tui/styles.go` | `tui` |
| `internal/tui/views.go` | `tui` |
| `internal/tui/reporter.go` | `tui` |
| `internal/tui/model_test.go` | `tui` |
| `internal/tui/views_test.go` | `tui` |
| `internal/tui/reporter_test.go` | `tui` |
| `cmd/run_tui.go` | `tui` |
| `cmd/run_notui.go` | `!tui` |

### Estimated LOC

| Component | Lines |
|-----------|-------|
| `internal/tui/` (6 files) | ~610 |
| `cmd/run_tui.go` + `cmd/run_notui.go` | ~100 |
| Tests (3 files) | ~200 |
| Runner change (`runner.go`) | ~20 |
| Cmd change (`run.go`) | ~20 |
| **Total** | **~950** |

### Build Commands

```bash
# Development (default, no TUI)
go build ./...
go test ./...

# With TUI
go build -tags tui -o smokesig .
go test -tags tui ./...

# Release binary with TUI
go build -tags tui -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=X.Y.Z" -o smokesig .
```
