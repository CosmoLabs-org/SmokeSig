---
date: "2026-05-29T00:00:00-03:00"
source: FEAT-051 brainstorm
status: brainstorm
issue: FEAT-051
related:
  - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
deliverables:
  - id: BR-01
    title: "Decision on dependency strategy: Bubbletea with build tag vs lipgloss-only"
  - id: BR-02
    title: "TUI reporter architecture — new Reporter implementation with full-screen model"
  - id: BR-03
    title: "Interactive result navigation — cursor movement, expand/collapse, scrolling"
  - id: BR-04
    title: "Re-run capabilities — individual test re-run, failures-only re-run, tag-filtered re-run"
  - id: BR-05
    title: "Watch mode integration — in-place TUI update on file changes"
  - id: BR-06
    title: "Keyboard shortcut map and navigation model"
  - id: BR-07
    title: "Accessibility and degradation — screen reader passthrough, no-color, pipe detection"
  - id: BR-08
    title: "Wire-up: --interactive flag, build tag gating, chain.go registration"
---

# FEAT-051: Interactive TUI Test Runner

## Problem

SmokeSig's terminal reporter is a streaming, write-once output. Once 40+ tests scroll past, the developer has to scroll back manually to find failures, re-read assertion details, or figure out which tags failed. There is no way to:

1. **Navigate results** after a run completes — failures get buried in passing output.
2. **Expand/collapse** individual test details — verbose mode dumps everything, quiet mode hides everything, there is no middle ground.
3. **Re-run a single failure** without restarting the entire suite — the only option is `--tag` filtering, which requires knowing the tag ahead of time.
4. **Filter or search** results interactively — with 45+ assertion types and growing test suites across a ~95-project portfolio, finding the relevant failure is increasingly friction-heavy.

Jest and Vitest solved this years ago with interactive watch mode. `smokesig --watch` already re-runs on file changes but outputs the same streaming text each time, making the terminal a wall of repeated output. An interactive TUI would turn `--watch` from "re-run and scroll" into "re-run and navigate."

This was identified as BR-09 in the Gemini ecosystem feedback analysis (2026-04-29).

## Design Decisions

### DD-1: Bubbletea with Build Tag (Recommended)

**Options considered:**

| Option | Pros | Cons |
|--------|------|------|
| **A: Bubbletea + `-tags tui`** | Full TUI framework, proven in Charm ecosystem (already use lipgloss), handles terminal state/resize/mouse, well-maintained | New dependency, ~5MB binary increase, needs build tag discipline |
| **B: huh (Charm forms library)** | Lighter than Bubbletea, good for prompts/forms | Designed for forms/wizards, not persistent full-screen UIs; would still pull Bubbletea as transitive dep |
| **C: Lipgloss-only (no new deps)** | Zero new dependencies, stays true to original philosophy | Must reimplement: raw terminal mode, input handling, screen clearing, resize detection, alternate screen buffer. Easily 600+ lines of plumbing that Bubbletea already handles correctly across platforms |
| **D: tcell / tview** | Powerful, widget-based | Different ecosystem from Charm (lipgloss already in tree), heavier API surface, less idiomatic with existing code |

**Decision: Option A — Bubbletea with `-tags tui` build tag.**

Rationale:

1. **Lipgloss is already a dependency.** Bubbletea is the same Charm ecosystem. The team that maintains lipgloss maintains Bubbletea. Version coordination is handled upstream.
2. **Build tag preserves zero-cost default.** `go build ./...` produces the same minimal binary. Only `go build -tags tui ./...` pulls in Bubbletea. CI, Docker, and lightweight deploys are unaffected.
3. **CLAUDE.md says the no-Bubbletea decision "can be revisited now the core is mature."** The core has 45 assertion types, 1297 tests, 8 reporter formats, watch mode, lifecycle hooks, stress testing, and monorepo support. It is mature.
4. **huh is a false economy.** It transitively depends on Bubbletea anyway (`github.com/charmbracelet/huh` imports `github.com/charmbracelet/bubbletea`). You get the dependency cost without the full-screen UI capability.
5. **Lipgloss-only is a trap.** Raw terminal mode, alternate screen buffer management, input parsing (including escape sequences across platforms), resize handling, and mouse support are each individually tricky and collectively ~800 lines of code that Bubbletea has already battle-tested. Reimplementing this violates the minimal-code philosophy more than adding a dependency does.

**Build tag mechanics:**

```
internal/tui/             # All files have //go:build tui
internal/tui/model.go     # Bubbletea Model
internal/tui/reporter.go  # Reporter interface implementation
internal/tui/views.go     # View rendering functions
internal/tui/keymap.go    # Key bindings

cmd/run_tui.go            # //go:build tui — registers --interactive flag
cmd/run_notui.go          # //go:build !tui — stub that errors with "build with -tags tui"
```

When `tui` tag is absent: `--interactive` flag does not exist. The binary is identical to today. No Bubbletea in `go.sum`.

When `tui` tag is present: `--interactive` flag is registered. Bubbletea is linked. Binary grows ~5MB (acceptable for a dev tool, irrelevant for CI where TUI is never used).

### DD-2: TUI Reporter vs Wrapper Architecture

**Options considered:**

| Option | Description | Verdict |
|--------|-------------|---------|
| **New TUI Reporter** | Implements `reporter.Reporter` directly. Receives events, updates Bubbletea model. | Chosen |
| **Wrapper around Terminal** | Captures Terminal reporter output, renders in TUI viewport. | Rejected — double rendering, loses structured data |

**Decision: New TUI Reporter implementing `reporter.Reporter`.**

The TUI reporter receives the same `TestStart`, `TestResult`, `PrereqResult`, `Summary` events as the terminal reporter but stores them as structured data in the Bubbletea model instead of writing formatted strings. This preserves all assertion details, timing, tags, and error information for interactive exploration.

```go
// internal/tui/reporter.go
// //go:build tui

type TUIReporter struct {
    model  *Model
    events chan tuiEvent
}

func (t *TUIReporter) TestStart(name string)           { t.events <- testStartEvent{name} }
func (t *TUIReporter) TestResult(r reporter.TestResultData) { t.events <- testResultEvent{r} }
func (t *TUIReporter) PrereqStart(name string)          { t.events <- prereqStartEvent{name} }
func (t *TUIReporter) PrereqResult(r reporter.PrereqResultData) { t.events <- prereqResultEvent{r} }
func (t *TUIReporter) Summary(s reporter.SuiteResultData) { t.events <- summaryEvent{s} }
```

The channel-based approach decouples the runner goroutine (which calls Reporter methods) from the Bubbletea event loop (which owns the terminal). Events flow: Runner -> channel -> Bubbletea `Cmd` -> `Update` -> `View`.

**MultiReporter compatibility:** The TUI reporter can be combined with JSON/JUnit/etc via `NewMultiReporter`. The TUI handles the screen; the file reporters write to disk. `--format json --interactive` works: JSON goes to `smoke-results.json`, TUI goes to the terminal. The chain.go registration detects `--interactive` and replaces the terminal reporter slot with the TUI reporter, preserving all secondary format reporters.

### DD-3: Feature Scope for v1

Interactive TUI features are grouped into three tiers:

**Tier 1 — MVP (this feature):**
- Full-screen results view after run completes
- Cursor navigation (up/down/j/k) through test list
- Expand/collapse test details (assertions, errors, timing)
- Filter: show all / failures only / passed only / skipped only
- Re-run single test under cursor
- Re-run all failures
- Quit (q/Ctrl+C)
- Summary bar (total/pass/fail/skip/duration) always visible

**Tier 2 — Fast follow (separate issues):**
- Live progress during run (spinner per test, results fill in)
- Text search across test names and assertion details (`/` to search)
- Tag filter picker (interactive tag selection from discovered tags)
- Mouse support (click to expand, scroll wheel)
- Copy assertion detail to clipboard

**Tier 3 — Future (not scoped):**
- Split pane (test list left, detail right)
- Diff view for assertion expected vs actual
- Test history across watch runs (show trend)
- Monorepo project grouping with collapsible sections
- Export filtered view to clipboard/file

**This brainstorm and implementation plan cover Tier 1 only.** Tier 2 and 3 are documented for future reference but are explicitly out of scope.

### DD-4: Watch Mode Integration

`--watch --interactive` is the primary use case. The integration model:

1. **First run:** Runner executes tests, TUI reporter collects results, Bubbletea renders full-screen results view.
2. **File change detected:** fsnotify triggers (existing debounce logic in `cmd/run.go`). A `watchRerunEvent` is sent to the Bubbletea model.
3. **Re-run phase:** Model switches to "running" state. Header shows "Re-running..." with spinner. Results are replaced in-place as new `TestResult` events arrive.
4. **Completion:** Model switches back to "results" state. Cursor position is preserved if possible (same test name). Summary bar updates.

The existing `runWatch` function in `cmd/run.go` (line 410) calls a `runOnce` closure on each file change. For TUI mode, this closure sends a message to the Bubbletea program instead of rebuilding the reporter. The Bubbletea program owns the terminal for the entire watch session.

```go
// Simplified integration sketch
func runWatchTUI(configDir, configFile string, program *tea.Program, buildOpts func() RunOptions) error {
    // Initial run via message
    program.Send(watchTriggerMsg{})
    
    // fsnotify loop sends messages instead of calling runOnce
    // ... (reuse existing debounce logic)
    for {
        select {
        case ev := <-w.Events:
            if isRelevantEvent(ev.Op) {
                program.Send(watchTriggerMsg{})
            }
        case <-sigCh:
            program.Send(tea.Quit())
            return nil
        }
    }
}
```

**Without `--watch`:** The TUI is still useful. `smokesig run --interactive` runs tests, shows results in full-screen view, allows navigation and re-run. Quit exits the process. This is the "one-shot interactive" mode.

### DD-5: Re-Run Mechanics

Re-running tests from the TUI requires access to the runner and config. The TUI model holds a reference to a `RerunFunc` callback:

```go
type RerunFunc func(testNames []string) (*SuiteResult, error)

type Model struct {
    results    reporter.SuiteResultData
    rerunFunc  RerunFunc
    // ...
}
```

The callback is constructed at startup in `cmd/run_tui.go` and captures the loaded config, config dir, and run options. When the user presses `r` (re-run current) or `R` (re-run failures), the model invokes the callback with the appropriate test name filter. The callback runs in a goroutine and sends result events back through the Bubbletea message channel.

**Tag re-filtering** uses the loaded config's test list and tag metadata. No config reload is needed — the tags are already in memory from the initial parse.

### Runner API Changes

- **`TestNames []string` field on `RunOptions`**: The runner needs name-based filtering (not just tag-based) so the TUI can re-run a specific test by name. `RerunFunc` constructs a `RunOptions` with `TestNames` populated from the cursor selection or failure list. The runner filters its test list with an exact-match check against `TestNames` before execution.
- **Lifecycle hook policy on re-run**: Single-test re-run (`r` key) skips `before_all` and `after_all` hooks — these are suite-level setup/teardown and should not fire for a single test. Full re-run (`R` key or `a` key) executes `before_all`/`after_all` normally since it constitutes a full suite run.
- **Prerequisites are NOT re-checked on re-run**: Prerequisites (e.g., "docker is running", "port 5432 is available") already passed during the initial run. Re-checking them on every re-run adds latency and is redundant — if the prerequisite was satisfied 10 seconds ago, it is still satisfied. If the user suspects a prerequisite changed, they quit and re-run the full suite.

### DD-6: Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `Enter` / `Space` | Toggle expand/collapse test details |
| `f` | Cycle filter: all -> failures -> passed -> skipped -> all |
| `r` | Re-run test under cursor |
| `R` | Re-run all failed tests |
| `a` | Re-run all tests |
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help overlay |
| `Home` / `g` | Jump to first test |
| `End` / `G` | Jump to last test |
| `Tab` | Expand all / Collapse all (toggle) |

These are vim-influenced (j/k/g/G) which aligns with the developer audience. Arrow keys work for non-vim users.

### DD-7: Accessibility and Degradation

1. **Pipe detection:** If stdout is not a TTY (`!term.IsTerminal(fd)`), `--interactive` is silently ignored and falls back to the normal terminal reporter. This prevents broken output when piping to `less`, `grep`, or a file.
2. **NO_COLOR / TERM=dumb:** Respected. The TUI renders without color when `NO_COLOR` is set (lipgloss already handles this). Under `TERM=dumb`, `--interactive` falls back to terminal reporter.
3. **Screen reader passthrough:** The TUI summary bar and test list are rendered as plain text lines (no box-drawing characters in the core list). Bubbletea's alternate screen buffer is used, which screen readers generally handle by reading the final buffer state. The `?` help overlay uses simple text layout, not decorative borders.
4. **Minimum terminal size:** The TUI requires 60 columns and 10 rows. Below that, it falls back to terminal reporter with a warning.
5. **High contrast:** Uses the same ANSI 16-color palette as the terminal reporter (colors 1-8). These map to the user's terminal theme, so high-contrast terminal themes automatically produce high-contrast TUI output.

## Architecture

### Package Layout

```
internal/tui/
    model.go         // Bubbletea Model (Init, Update, View)
    reporter.go      // TUIReporter implementing reporter.Reporter
    views.go         // View helper functions (header, test list, detail, summary, help)
    keymap.go        // Key binding definitions
    styles.go        // Lipgloss styles (reuses terminal.go color palette)
    events.go        // Custom Bubbletea messages (test events, watch events, rerun events)

cmd/
    run_tui.go       // //go:build tui — flag registration, TUI bootstrap, watch integration
    run_notui.go     // //go:build !tui — error stub
```

### Data Flow

```
.smokesig.yaml
     |
     v
schema.Load() -> schema.SmokeConfig
     |
     v
runner.Runner{Reporter: tui.TUIReporter}
     |
     | (calls Reporter methods on test completion)
     v
TUIReporter.TestResult(data)
     |
     | (sends via channel)
     v
tea.Program event loop
     |
     v
Model.Update() -> stores in []TestResultData
     |
     v
Model.View() -> renders full-screen output
     |
     ^ (user input: j/k/Enter/r/R)
     |
     | (re-run request)
     v
RerunFunc([]string) -> runner.Runner.Run()
     |
     | (new results via channel)
     v
Model.Update() -> replaces results
```

### State Machine

The TUI model operates in three states:

```
RUNNING -> RESULTS -> RERUNNING -> RESULTS
             |
             v
           QUITTING
```

- **RUNNING**: Tests are executing. Header shows progress spinner and count ("Running 12/40..."). Test results appear as they arrive (list grows). Navigation is active (user can scroll through completed tests while others are still running).
- **RESULTS**: All tests complete. Full navigation, expand/collapse, filtering, re-run available. Summary bar shows final counts.
- **RERUNNING**: A re-run was triggered (user pressed `r`/`R`/`a` or watch mode detected changes). Header shows "Re-running..." with spinner. Previous results remain visible but dimmed. New results replace old ones as they arrive.
- **QUITTING**: `q`/`Ctrl+C` pressed. Bubbletea exits, alternate screen buffer is restored.

Missing state transitions that must be added:

- **RUNNING -> QUITTING**: `Ctrl+C` during the initial test run cancels execution and exits cleanly.
- **RERUNNING -> QUITTING**: `q` or `Ctrl+C` during a re-run cancels the re-run goroutine and exits.

### Concurrency Model

- **Re-run debouncing**: When the model is in RERUNNING state, new re-run requests (user pressing `r`/`R`/`a` or watch-triggered `watchTriggerMsg`) are ignored. The model shows a brief "already re-running" indicator in the header (auto-clears after 1s via `tea.Tick`) rather than queueing or interrupting.
- **ERROR display state**: When `RerunFunc` returns a non-nil error (e.g., config parse failure after file change, runner panic recovery), the model transitions to an ERROR state instead of RESULTS. The header shows the error message in red. Previous results remain visible but dimmed. The user can press `a` to retry or `q` to quit. The state machine becomes:

```
RUNNING -> RESULTS -> RERUNNING -> RESULTS
             |            |
             v            v
           QUITTING     ERROR -> RERUNNING (retry)
                          |
                          v
                        QUITTING
```

- **Context-based cancellation**: `Runner.Run()` accepts a `context.Context` parameter (or wraps its internal execution with `context.WithCancel`). When the user presses `q` during RUNNING or RERUNNING, the model cancels the context, which propagates to in-flight HTTP assertions, port checks, and process waits. This prevents orphaned goroutines from leaking after quit. The Bubbletea `Update` function holds the `cancel` func and calls it on quit key before sending `tea.Quit`.

### Screen Layout

```
+----------------------------------------------------------+
| SmokeSig v0.20.1 — my-project          [PASS] 38/40 ✓   |  <- Header (project, status)
+----------------------------------------------------------+
|                                                          |
|  ✓ health-check-api                          (12ms)      |  <- Test list (scrollable)
|  ✓ database-connection                       (45ms)      |
| >✗ redis-cache-warmup                        (2.1s)  ◄  |  <- Cursor on failed test
|    redis_ping: expected +PONG, got timeout               |  <- Expanded detail
|    error: dial tcp 127.0.0.1:6379: refused               |
|  ✓ worker-queue-depth                        (89ms)      |
|  ⊘ gpu-inference (skipped: env_unset CUDA)               |
|  ~ flaky-external-api                   (1.3s allowed)   |
|  ✓ websocket-realtime                        (234ms)     |
|  ...                                                     |
|                                                          |
+----------------------------------------------------------+
| 40 tests  38 passed  1 failed  1 skipped       (4.2s)    |  <- Summary bar
| [f]ilter  [r]erun  [R]erun-fails  [a]ll  [q]uit  [?]    |  <- Shortcut bar
+----------------------------------------------------------+
```

### Integration with cmd/run_tui.go

**The TUI reporter is NOT registered as a format in `chain.go`.** Unlike JSON, JUnit, TAP, etc., the TUI reporter does not conform to the `func(io.Writer) Reporter` factory signature that `chain.go` expects. It requires a `RerunFunc` callback and a `tea.Program` reference — dependencies that the format registry cannot provide. The TUI is a UI mode, not an output format. Registering it in `chain.go` (e.g., via a `chain_tui.go` init function) would create a broken factory that cannot produce a functional reporter without its runtime dependencies.

The only integration path is through `cmd/run_tui.go`:

```go
// cmd/run_tui.go
// //go:build tui

func init() {
    runCmd.Flags().BoolVar(&interactive, "interactive", false,
        "Full-screen TUI for navigating results and re-running tests")
}
```

When `--interactive` is set, `runSmoke` delegates to `runInteractive` which constructs the Bubbletea program, wires the TUI reporter into the reporter chain (replacing the terminal slot), and manages the program lifecycle. Non-terminal formats (json, junit, etc.) still write to their files normally via MultiReporter.

## Scope Boundaries

### In Scope (Tier 1)

- Full-screen alternate-buffer TUI after test run completes
- Cursor navigation through test list with vim keys and arrows
- Expand/collapse individual test details (assertions, errors)
- Filter toggle: all / failures / passed / skipped
- Re-run: single test, all failures, all tests
- Watch mode integration (`--watch --interactive`)
- Summary bar with live counts
- Help overlay (`?`)
- Build tag gating (`-tags tui`)
- Stub file for non-TUI builds (clear error message)
- TTY detection with graceful fallback
- NO_COLOR / TERM=dumb support

### Explicitly Out of Scope

- Text search / regex filter (`/` command) — Tier 2
- Mouse support — Tier 2
- Tag picker UI — Tier 2
- Split pane layout — Tier 3
- Test history / trend view — Tier 3
- Monorepo section grouping — Tier 3
- Clipboard integration — Tier 2
- Custom key binding configuration — Tier 3
- `smokesig tui` as a separate subcommand (it is a flag on `run`)

## Estimated LOC

| Component | Lines | Notes |
|-----------|-------|-------|
| `internal/tui/model.go` | ~180 | Bubbletea Model: state, update logic, window size |
| `internal/tui/reporter.go` | ~60 | Reporter interface implementation, channel bridge |
| `internal/tui/views.go` | ~200 | Header, test list, detail panel, summary bar, help |
| `internal/tui/keymap.go` | ~40 | Key binding constants and help text |
| `internal/tui/styles.go` | ~30 | Lipgloss styles (mirrors terminal.go palette) |
| `internal/tui/events.go` | ~50 | Custom Bubbletea message types |
| `cmd/run_tui.go` | ~80 | Flag registration, program bootstrap, watch wiring |
| `cmd/run_notui.go` | ~15 | Stub with build-tag error |
| Tests | ~300 | Model update logic, view rendering, reporter bridge |
| **Total** | **~955** | Excluding test code: ~655 |

The original estimate of ~400 lines assumed lipgloss-only (no framework overhead). With Bubbletea, the framework handles terminal plumbing but the model/view/update code is more structured, bringing the total to ~650 implementation + ~300 test lines.

## Dependencies

| Package | Version | Purpose | Size Impact |
|---------|---------|---------|-------------|
| `github.com/charmbracelet/bubbletea` | v1.x | TUI framework (Model-View-Update) | ~3MB binary |
| `github.com/charmbracelet/bubbles` | v0.x | Viewport, spinner components | ~1MB binary |

Both are Charm ecosystem (same org as lipgloss, already in `go.mod`). They share transitive dependencies (`x/term`, `x/ansi`, `colorprofile`) that are already in the dependency tree.

No other new dependencies. The TUI uses lipgloss styles from `internal/reporter/terminal.go`'s color palette (ANSI colors 1-8) to maintain visual consistency.

## Open Questions

1. **Should `--interactive` be the default when TTY is detected?** Probably not for v1 — explicit opt-in avoids surprises. Could become default in a later version once the TUI is battle-tested.
2. **Should the TUI persist results to disk for post-hoc review?** Out of scope for v1, but the structured data in the model could trivially serialize to JSON. The existing `--format json` already covers this use case.
3. **Monorepo grouping in TUI:** When `--monorepo` is used, should the TUI group tests by sub-project? This is Tier 3 but the data model should accommodate it from the start (the `SuiteResultData.Project` field is already per-suite).
4. **Performance with large test suites:** The SmokeSig portfolio targets ~95 projects. A monorepo run could produce 500+ test results. The viewport component from bubbles handles scrolling efficiently, but the view function should avoid re-rendering the entire list on every keystroke. Bubbletea's string-diff optimization handles this natively.

## Prior Art

| Tool | TUI Approach | Takeaway for SmokeSig |
|------|-------------|----------------------|
| Jest `--watch` | Keystroke menu at bottom, re-run by pattern/filename/failed | Keep shortcut bar visible, support re-run-failures |
| Vitest UI | Browser-based, full tree view | Too heavy; but the expand/collapse tree is the right UX pattern |
| `lazygit` | Bubbletea, full-screen panels | Proves Bubbletea handles complex TUI state well |
| `glow` (Charm) | Bubbletea, markdown viewer | Same ecosystem, validates the library choice |
| `k9s` | tcell-based Kubernetes TUI | Different ecosystem, but shows that TUI tools for infra are expected by the audience |
