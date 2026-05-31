---
brainstorm_ref: docs/brainstorming/2026-05-29-ccs-dependency-wiring.md
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
covers_plan_deliverables:
    - P-01
    - P-02
    - P-03
    - P-04
    - P-05
    - P-06
created: "2026-05-29T12:00:00-03:00"
goals_completed: 6
goals_total: 6
id: P-2026-05-29-ccs-dependency-wiring
plan_ref: docs/planning-mode/2026-05-29-ccs-dependency-wiring.md
priority: medium
related_prompts: []
requires_reading:
    - docs/brainstorming/2026-05-29-ccs-dependency-wiring.md
    - docs/planning-mode/2026-05-29-ccs-dependency-wiring.md
schema_version: 1
status: COMPLETED
tags: []
title: 'FEAT-052: CCS Dependency Wiring'
---

# FEAT-052: CCS Dependency Wiring

## BEFORE Starting — Required Reading

**You MUST read these files in full before writing any code. They are the gospel truth of what must be implemented.**

Read in order:

1. **`docs/brainstorming/2026-05-29-ccs-dependency-wiring.md`** — design rationale and decision history.
2. **`docs/planning-mode/2026-05-29-ccs-dependency-wiring.md`** — implementation plan with deliverables and file scope.

Loading is enforced by `ccs prompts load-context` — the command will error if any file is missing.

## Context

Wire SmokeSig as an external CCS dependency. Fix the dead `LoadDefault()` code so `.smoke.yaml` fallback actually works. Formalize exit codes (0=pass, 1=fail, 2=config, 3=prereq). CCS-side: passthrough command, rebuild entry, binary registry, version mismatch detection. G-01 and G-02 are SmokeSig changes; G-03 through G-05 are CCS changes (documented here, implemented in CCS repo); G-06 is cross-project.

## Execution Strategy

- **SmokeSig** (parallel): G-01 + G-02
- **CCS** (parallel, after SmokeSig): G-03 + G-04
- **CCS** (after G-04): G-05
- **Cross-project** (last): G-06

## Goals

### [x] G-01 Wire LoadDefault() fallback into run/validate commands
Covers P-01.

### [x] G-02 Formalize exit code contract
Covers P-02.

### [x] G-03 CCS smoke.go passthrough command
Covers P-03.

### [x] G-04 CCS rebuild.go SmokeSig entry
Covers P-04.

### [x] G-05 CCS .binary-registry.json entry + sync check
Covers P-05.

### [x] G-06 Documentation sweep and migration guide
Covers P-06.

## Related

- Brainstorm: `docs/brainstorming/2026-05-29-ccs-dependency-wiring.md`
- Plan: `docs/planning-mode/2026-05-29-ccs-dependency-wiring.md`
