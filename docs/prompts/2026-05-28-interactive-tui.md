---
brainstorm_ref: docs/brainstorming/2026-05-28-interactive-tui.md
branch: master
covers_brainstorm_deliverables:
    - BR-01
    - BR-02
    - BR-03
    - BR-04
    - BR-05
    - BR-06
    - BR-07
covers_plan_deliverables:
    - P-01
    - P-02
    - P-03
    - P-04
    - P-05
    - P-06
    - P-07
created: "2026-05-28"
id: P-2026-05-28-interactive-tui
plan_ref: docs/planning-mode/2026-05-28-interactive-tui.md
priority: medium
requires_reading:
    - docs/brainstorming/2026-05-28-interactive-tui.md
    - docs/planning-mode/2026-05-28-interactive-tui.md
schema_version: 1
status: PENDING
title: Interactive TUI with Bubbletea — Full Implementation
---
# Interactive TUI with Bubbletea — Full Implementation

## BEFORE Starting — Required Reading

**You MUST read these files in full before writing any code. They are the gospel truth of what must be implemented.**

Read in order:

1. **`docs/brainstorming/2026-05-28-interactive-tui.md`** — design rationale and decision history.
2. **`docs/planning-mode/2026-05-28-interactive-tui.md`** — implementation plan with deliverables and file scope.

Loading is enforced by `ccs prompts load-context` — the command will error if any file is missing.

## Context

_Describe the session context._

## Goals

### [ ] G-01 Add Bubbletea + Bubbles dependencies to go.mod
Covers P-01.

### [ ] G-02 Runner.RunSingle method for single-test re-execution
Covers P-02.

### [ ] G-03 TUI model with full Update/View cycle
Covers P-03.

### [ ] G-04 Reporter adapter bridging runner events to tea.Msg
Covers P-04.

### [ ] G-05 Lipgloss styles and key bindings
Covers P-05.

### [ ] G-06 Build-tagged cmd wiring + stub
Covers P-06.

### [ ] G-07 Model unit tests for all interactive features
Covers P-07.

## Related

- Brainstorm: `docs/brainstorming/2026-05-28-interactive-tui.md`
- Plan: `docs/planning-mode/2026-05-28-interactive-tui.md`
