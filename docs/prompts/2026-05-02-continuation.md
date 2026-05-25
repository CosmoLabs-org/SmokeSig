---
branch: master
completed: "2026-05-25"
covers_brainstorm_deliverables:
    - BR-13
    - BR-15
    - BR-05
created: "2026-05-02T12:00:00-03:00"
goals_completed: 0
goals_total: 7
id: P-2026-05-02-continuation
priority: high
related_prompts:
    - docs/prompts/2026-05-02-continuation.md
requires_reading:
    - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
    - docs/planning-mode/2026-04-30-phase1-test-chaining.md
    - internal/runner/runner.go
    - internal/schema/schema.go
schema_version: 1
status: ABANDONED
tags:
    - release
    - feat-045
    - feat-050
    - feat-046
    - ecosystem
    - continuation
title: 'Continuation: Post-Chaining Release & Next Feature Selection'
---

# Continuation: Post-Chaining Release & Next Feature Selection

**Date**: 2026-05-02
**Branch**: master (clean, pushed to origin)
**Version**: v0.19.0
**Tests**: 1036 passing, 11 packages

---

## Current State

cosmo-smoke v0.19.0 on master. Working tree is clean and fully pushed to origin.

**Just completed (last session):**
- FEAT-038 (test chaining with data extraction) — DONE, closed. Adds `extract:`, `depends_on:`, `sensitive_vars:`, VarStore, chain detection, sequential execution for chained tests.
- Chain detection wired into runner: tests with `depends_on:` or `extract:` are forced sequential automatically.

**7 open features remain** (all from Gemini ecosystem feedback brainstorm):

| ID | Title | Stage | Size |
|----|-------|-------|------|
| FEAT-045 | Auto-Add Generator / smoke observe (BR-13) | Brainstormed, needs plan | ~1,080 LOC |
| FEAT-046 | Detector-observer integration (BR-15) | Brainstormed | ~150 LOC |
| FEAT-047 | Simulator/emulator health assertions (BR-12) | Brainstormed | ~200 LOC |
| FEAT-048 | Wasm plugin system (BR-07) | Brainstormed | ~800 LOC |
| FEAT-049 | OIDC integration (BR-04) | Brainstormed | ~500 LOC |
| FEAT-050 | Backstage.io schema output reporter (BR-05) | Brainstormed | ~150 LOC |
| FEAT-051 | Interactive TUI test runner (BR-09) | Brainstormed | ~400 LOC |

**Staged changelog entries (5 total, ready for release):**
- FEAT-036: Distribution tooling (goreleaser, Docker, Homebrew)
- FEAT-044: Flakiness detector (smoke stress command)
- FEAT-038: Test chaining with data extraction
- FEAT-038 issue closure entry
- Chain detection wiring

**Release readiness**: 5 commits since v0.19.0 tag. Release check suggests minor bump to v0.20.0.

## Goal

**Primary**: Release v0.20.0 with all staged changelog entries. Finalize the release cleanly.

**Secondary**: Write the implementation plan for the next feature. Priority candidates:

1. **FEAT-050 (Backstage.io schema output)** — ~150 LOC, low risk, extends existing reporter pattern. Quick win that expands output format coverage (currently: terminal, json, junit, tap, prometheus, gha). Enterprise integration value.

2. **FEAT-045 (Auto-Add Generator / smoke observe)** — ~1,080 LOC, the largest remaining feature. Brainstorm has a detailed 5-batch architecture (observer engine, sanitization pipeline, filesystem diffing, port detection, YAML generation). Needs a formal plan before implementation. FEAT-046 (detector-observer integration, ~150 LOC) is a natural dependency.

**Recommendation**: Do the release first, then write the FEAT-050 plan (quick win, done in one session) or the FEAT-045 plan (ambitious, needs dedicated session for plan alone). The session should produce exactly one plan, not start implementing.

## Pre-Flight

Run these BEFORE starting any work:

```bash
# 1. Verify clean working tree
git status --short

# 2. Confirm build passes
go build ./...

# 3. Quick test sanity check
go test ./... -count=1 -timeout 60s 2>&1 | tail -5

# 4. Verify changelog
ccs changelog preview
```

## Execution Plan

### Phase 1: Release v0.20.0 (~10 min)

1. **Preview changelog** to verify all 5 entries:
   ```bash
   ccs changelog preview
   ```

2. **Finalize release**:
   ```bash
   ccs changelog finalize 0.20.0 "test-chaining-stress-distribution"
   ```

3. **Tag release**:
   ```bash
   git tag v0.20.0
   ```

4. **Verify version**:
   ```bash
   go build -ldflags "-s -w -X github.com/CosmoLabs-org/cosmo-smoke/cmd.Version=0.20.0" -o smoke . && ./smoke version
   ```

### Phase 2: Next Feature Plan (~60 min)

Pick ONE feature to plan. Do NOT implement.

**Option A: FEAT-050 (Backstage.io schema output)** — recommended if time is short

1. Read BR-05 section in brainstorm (lines ~144-155)
2. Study existing reporter pattern (`internal/reporter/gha.go` is the closest analog — new format, writes to a file/endpoint)
3. Write plan to `docs/planning-mode/2026-05-02-backstage-reporter.md`
4. Enrich: `ccs prompts enrich docs/planning-mode/2026-05-02-backstage-reporter.md --apply`

Deliverables:
- P-01: Backstage reporter struct implementing Reporter interface
- P-02: Backstage entity annotation JSON format (status, checks, timestamps)
- P-03: `--format backstage` integration in cmd/run.go
- P-04: Tests for Backstage output format

**Option B: FEAT-045 (Auto-Add Generator / smoke observe)** — recommended if full session available

1. Read BR-13 (lines ~421-535) and BR-15 (lines ~837-851) in brainstorm
2. Read batch 4 interactive/silent mode design (lines ~549-601)
3. Study existing `cmd/init_cmd.go` for detector integration patterns
4. Study existing `internal/detector/` for project type detection
5. Write plan to `docs/planning-mode/2026-05-02-auto-add-generator.md`
6. Enrich: `ccs prompts enrich docs/planning-mode/2026-05-02-auto-add-generator.md --apply`

Deliverables:
- P-01: Observation engine (`internal/observer/observer.go`) — process wrapping, io.MultiWriter capture
- P-02: Sanitization pipeline (`internal/observer/sanitize.go`) — ANSI, timestamps, UUIDs, durations
- P-03: Filesystem snapshot and diffing (`internal/observer/snapshot.go`)
- P-04: Port detection with gopsutil (`internal/observer/ports.go`)
- P-05: YAML generation from observations (`internal/observer/generator.go`)
- P-06: `smoke observe` Cobra command (`cmd/observe.go`)
- P-07: Integration with detector package for stack-aware observation

### Phase 3 (Optional): Housekeeping

If both phases complete with time remaining:

1. **Command READMEs** — check which commands still lack dedicated READMEs
2. **Stress docs** — verify `docs/commands/stress.md` was written properly in last session
3. **Self-smoke** — run `./smoke run` to verify the full suite against the new binary

## Architecture Context

**Key patterns for the next features:**

- **Reporter interface** (`internal/reporter/reporter.go`): All output formats implement a `Reporter` interface. New formats (Backstage) add a file in `internal/reporter/` and register in `cmd/run.go`'s format switch. The GHA reporter (`gha.go`) is the best template — it writes to a file/endpoint rather than stdout.

- **Detector package** (`internal/detector/`): 31 project types with `DetectProject()` returning project type + metadata. Used by `smoke init`. The observe command would call `DetectProject()` to tailor observation heuristics per stack.

- **Command execution pattern**: Commands run from the config file's directory (not cwd). The runner uses `exec.Command` with configurable timeout. The observer will need the same pattern but with io.MultiWriter wrapping stdout/stderr capture.

- **VarStore** (new in FEAT-038): Thread-safe variable store with get/set, used for chain data extraction. The observer might reuse VarStore for environment variable capture from observed processes.

## Key Files

| File | Why It Matters |
|------|---------------|
| `docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md` | Source brainstorm with BR-05 (Backstage), BR-13 (observe), BR-15 (detector integration) |
| `docs/planning-mode/2026-04-30-phase1-test-chaining.md` | Previous plan — example of plan structure and deliverable format |
| `internal/runner/runner.go` | Core execution engine — VarStore, chain detection, test execution |
| `internal/schema/schema.go` | Config structs — assertion types, test definition, extract/depends_on fields |
| `internal/reporter/gha.go` | Best template for a new reporter format (Backstage) |
| `internal/reporter/reporter.go` | Reporter interface definition |
| `cmd/run.go` | Main command — format registration, flag handling |
| `cmd/init_cmd.go` | Init command — detector integration, template generation patterns |
| `internal/detector/` | Project type detection — 31 types, auto-detection logic |

## Verification Checklist

Before declaring the session complete:

- [ ] v0.20.0 released: `ccs changelog preview` shows empty unreleased
- [ ] v0.20.0 tagged: `git tag -l 'v0.20*'`
- [ ] Plan written and enriched: `ccs prompts validate docs/planning-mode/2026-05-02-*.md`
- [ ] Full suite green: `go test ./... -count=1` (1036+ tests)
- [ ] Binary builds with new version: `go build -ldflags "..." -o smoke .`

## Warnings

- **Do not implement features this session.** The goal is release + plan. Implementation happens in the next session with a fresh continuation prompt.
- **FEAT-045 is ~1,080 LOC.** Even the plan is substantial. If you choose Option B, expect the plan alone to take 30-40 minutes. Do not rush the plan — it determines implementation quality.
- **FEAT-048 (Wasm) and FEAT-049 (OIDC) are ranked "Later"** in the brainstorm priority table. Do not plan these unless the user explicitly requests them.
- **The changelog has a duplicate FEAT-038 entry** (one for the feature, one for the issue closure). Verify during release that both are appropriate or consolidate.
