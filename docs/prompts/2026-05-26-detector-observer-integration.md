---
brainstorm_ref: docs/brainstorming/2026-05-26-detector-observer-integration.md
branch: master
covers_brainstorm_deliverables:
    - BR-01
    - BR-02
    - BR-03
    - BR-04
    - BR-05
covers_plan_deliverables:
    - P-01
    - P-02
    - P-03
    - P-04
created: "2026-05-26"
id: P-2026-05-26-detector-observer-integration
plan_ref: docs/planning-mode/2026-05-26-detector-observer-integration.md
priority: medium
requires_reading:
    - docs/brainstorming/2026-05-26-detector-observer-integration.md
    - docs/planning-mode/2026-05-26-detector-observer-integration.md
schema_version: 1
status: PENDING
title: 'FEAT-046: Detector-Observer Integration'
---

# FEAT-046: Detector-Observer Integration

## BEFORE Starting — Required Reading

**You MUST read these files in full before writing any code. They are the gospel truth of what must be implemented.**

Read in order:

1. **`docs/brainstorming/2026-05-26-detector-observer-integration.md`** — design rationale and decision history.
2. **`docs/planning-mode/2026-05-26-detector-observer-integration.md`** — implementation plan with deliverables and file scope.

Loading is enforced by `ccs prompts load-context` — the command will error if any file is missing.

## Context

Make `smokesig observe` stack-aware. The observer package (FEAT-045) captures raw behavior; this feature adds project type detection so it knows what to look for. Reads `portless.json` first (CCS SOP), falls back to `detector.Detect()` for stack heuristics. ~160 LOC total.

## Execution Strategy

All 4 goals are sequential (each builds on the prior). Single GLM-turbo dispatch is appropriate — bounded scope, exact code provided in plan, no design judgment needed.

```yaml
agents:
  - task: "All 4 goals — hints.go + probes change + observer wiring"
    model: glm-turbo
    files: [internal/observer/hints.go, internal/observer/hints_test.go, internal/observer/probes.go, internal/observer/observer.go]
    ready: true
```

## Goals

### [ ] G-01 StackHints type and HintsFromDir with portless reader + stack table
Covers P-01.

### [ ] G-02 ProbeEndpoints accepts extra paths
Covers P-02.

### [ ] G-03 Observer wires hints into observation pipeline
Covers P-03.

### [ ] G-04 cmd/observe auto-detects and passes hints
Covers P-04.

## Related

- Brainstorm: `docs/brainstorming/2026-05-26-detector-observer-integration.md`
- Plan: `docs/planning-mode/2026-05-26-detector-observer-integration.md`
