---
brainstorm_ref: docs/brainstorming/2026-05-29-interactive-tui-runner.md
branch: master
completed: "2026-05-30"
covers_brainstorm_deliverables:
    - BR-01
    - BR-02
    - BR-03
    - BR-04
    - BR-05
    - BR-06
    - BR-07
    - BR-08
covers_plan_deliverables:
    - P-01
    - P-02
    - P-03
    - P-04
    - P-05
    - P-06
    - P-07
created: "2026-05-29T12:00:00-03:00"
goals_completed: 7
goals_total: 7
id: P-2026-05-29-interactive-tui-runner
plan_ref: docs/planning-mode/2026-05-29-interactive-tui-runner.md
priority: medium
related_prompts: []
requires_reading:
    - docs/brainstorming/2026-05-29-interactive-tui-runner.md
    - docs/planning-mode/2026-05-29-interactive-tui-runner.md
schema_version: 1
status: COMPLETED
tags: []
title: 'FEAT-051: Interactive TUI Test Runner'
---

# FEAT-051: Interactive TUI Test Runner

## BEFORE Starting — Required Reading

**You MUST read these files in full before writing any code. They are the gospel truth of what must be implemented.**

Read in order:

1. **`docs/brainstorming/2026-05-29-interactive-tui-runner.md`** — design rationale and decision history.
2. **`docs/planning-mode/2026-05-29-interactive-tui-runner.md`** — implementation plan with deliverables and file scope.

Loading is enforced by `ccs prompts load-context` — the command will error if any file is missing.

## Context

Add an interactive TUI mode to `smokesig run` using Bubbletea, gated behind `-tags tui`. Navigate test results, expand assertion details, re-run failures, filter by status. Implements a new `TUIReporter` conforming to `reporter.Reporter`, a `TestNames` filter on `RunOptions`, and `cmd/run_tui.go` for `--interactive` flag wiring. ~955 LOC total.

## Execution Strategy

Goals are grouped for parallel dispatch where possible:

- **Group A** (parallel): G-01 + G-04 — independent foundation work
- **Group B** (parallel, after A): G-02 + G-03 — TUI core components
- **Sequential** (after B): G-05 → G-06 → G-07

## Goals

### [x] G-01 Build tag setup and Bubbletea dependency
Covers P-01.

### [x] G-02 TUI Model, state machine, and key bindings
Covers P-02.

### [x] G-03 TUI Reporter implementing reporter.Reporter
Covers P-03.

### [x] G-04 Runner TestNames filter for re-run support
Covers P-04.

### [x] G-05 cmd/run_tui.go integration with --interactive flag
Covers P-05.

### [x] G-06 Watch mode integration
Covers P-06.

### [x] G-07 Tests and build verification
Covers P-07.

## Related

- Brainstorm: `docs/brainstorming/2026-05-29-interactive-tui-runner.md`
- Plan: `docs/planning-mode/2026-05-29-interactive-tui-runner.md`
