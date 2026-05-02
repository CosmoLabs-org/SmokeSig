---
branch: master
created: "2026-05-02"
status: ACTIVE
schema_version: 1
requires_reading:
  - docs/planning-mode/2026-04-30-phase1-test-chaining.md
  - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
  - internal/runner/runner.go
  - internal/schema/schema.go
  - internal/reporter/reporter.go
  - cmd/run.go
related_prompts:
  - docs/prompts/2026-04-30-phase2-remaining-continuation.md
tags: [feat-038, feat-045, ecosystem, continuation]
title: "Continuation: Ecosystem Features — FEAT-038, FEAT-045, Doc Audit"
---

# Continuation: Ecosystem Features — FEAT-038, FEAT-045, Doc Audit

**Date**: 2026-05-02
**Branch**: master (clean, pushed to origin)
**Version**: v0.18.0
**Tests**: 1034 passing, 11 packages

---

## Current State

cosmo-smoke v0.18.0 on master. Working tree is clean and fully pushed to origin.

**Completed in recent sessions:**
- FEAT-044 (smoke stress command) — DONE, merged
- FEAT-036 (v1.0 release readiness: goreleaser, Docker, Homebrew) — DONE, merged
- FEAT-037 (file_size assertion) — DONE, merged

**Open features (8 total, all from Gemini ecosystem feedback brainstorm):**

| ID | Title | Stage | Size |
|----|-------|-------|------|
| FEAT-038 | Test chaining with data extraction (BR-08) | Plan written, deferred 1x | ~500 LOC |
| FEAT-045 | Auto-Add Generator / smoke observe (BR-13) | Brainstormed, needs plan | ~1,080 LOC |
| FEAT-046 | Detector-observer integration (BR-15) | Brainstormed | ~150 LOC |
| FEAT-047 | Simulator/emulator health assertions (BR-12) | Brainstormed | ~200 LOC |
| FEAT-048 | Wasm plugin system for custom assertions (BR-07) | Brainstormed | ~800 LOC |
| FEAT-049 | OIDC integration for cloud role assumption (BR-04) | Brainstormed | ~500 LOC |
| FEAT-050 | Backstage.io schema output reporter (BR-05) | Brainstormed | ~150 LOC |
| FEAT-051 | Interactive TUI test runner (BR-09) | Brainstormed | ~400 LOC |

**Housekeeping items:**
- 16 commits since v0.18.0 tag (release check suggests v0.19.0)
- No staged changelog entries (unreleased.yaml is empty)
- 9 command READMEs missing (stress, init, run, serve, validate, schema, version, mcp, migrate)
- No unreleased changelog entries despite FEAT-036 and FEAT-044 completions

## Goal

**Primary**: Implement FEAT-038 (test chaining with data extraction) — the biggest functional gap for API testing workflows.

**Secondary**: If time permits, write the implementation plan for FEAT-045 (auto-add generator / smoke observe).

**Housekeeping**: Stage changelog entries for FEAT-036 and FEAT-044. Address command README gaps.

## Pre-Flight

Run these BEFORE starting any implementation work:

```bash
# 1. Verify clean working tree
git status --short

# 2. Confirm build passes
go build ./...

# 3. Quick test sanity check
go test ./... -count=1 -timeout 60s 2>&1 | tail -5
```

## Execution Plan

### Phase 1: Housekeeping (do this first, ~10 min)

1. **Stage changelog entries** for recently completed features:
   ```bash
   ccs changelog add "FEAT-036: Distribution tooling — goreleaser, Docker, Homebrew" --type added
   ccs changelog add "FEAT-044: Flakiness detector — smoke stress command with worker pool" --type added
   ```

2. **Check command READMEs** — determine which are genuinely missing vs. covered by USAGE.md:
   ```bash
   ls READMEs/commands/ 2>/dev/null
   ls cmd/*/
   ```

### Phase 2: FEAT-038 — Test Chaining with Data Extraction

FEAT-038 has a complete plan at `docs/planning-mode/2026-04-30-phase1-test-chaining.md` with six deliverables. It has been deferred once already — this session should execute it.

**What it does**: Enables extracting values from one test (e.g., JWT token from login response) and injecting them into subsequent tests via `{{ .Vars.token }}` templating. Chained tests run sequentially. Sensitive vars are masked in output. This is the biggest functional gap for API testing workflows.

**Plan deliverables:**

| ID | Deliverable | Description |
|----|-------------|-------------|
| P-01 | Variable store and extraction interface | New `VarStore` type with thread-safe get/set, `.Vars` namespace |
| P-02 | `extract:` field on assertions | Add `extract` to json_field, stdout_matches, http assertions |
| P-03 | Variable resolution in Go templates | Extend template engine to resolve `{{ .Vars.name }}` at runtime |
| P-04 | Sequential execution for chained test groups | Ordered execution when `depends_on:` or `extract:` is present |
| P-05 | Sensitive variable masking | Mask marked vars in reporter output |
| P-06 | Full chain lifecycle tests | End-to-end: extract → resolve → assert cycle |

**TDD approach:**
1. Write tests for P-01 (VarStore) — watch them fail
2. Implement VarStore — watch tests pass
3. Write tests for P-02 (extract field) — watch them fail
4. Add extract field to schema and assertion evaluation
5. Continue through P-03, P-04, P-05 in same pattern
6. P-06 (lifecycle tests) validates the full chain

**Key schema changes:**
```yaml
tests:
  - name: login
    run: curl -s http://api/login
    assertions:
      - json_field:
          path: ".token"
          extract: auth_token    # <-- NEW: saves to VarStore
  - name: use-api
    depends_on: [login]          # <-- NEW: sequential ordering
    run: curl -s -H "Authorization: Bearer {{ .Vars.auth_token }}" http://api/data
    assertions:
      - json_field:
          path: ".status"
          equals: "ok"
    sensitive_vars: [auth_token]  # <-- NEW: masked in output
```

**Post-implementation:**
```bash
go test ./... -count=1
ccs issues update FEAT-038 --status done
ccs changelog add "FEAT-038: Test chaining with data extraction (extract, depends_on, sensitive_vars)" --type added
```

### Phase 3 (Optional): FEAT-045 Plan

If FEAT-038 completes with substantial time remaining:

1. Read the FEAT-045 brainstorm section (BR-13 in `docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md`)
2. Write implementation plan to `docs/planning-mode/2026-05-02-auto-add-generator.md`
3. Enrich with deliverables via `ccs prompts enrich`
4. Do NOT start implementing — the plan needs review first

FEAT-045 adds `smoke observe "cmd"` which captures filesystem and port changes during command execution, then generates a `.smoke.yaml` config automatically. ~1,080 lines.

### Phase 4 (If Time): Command READMEs

Check which of the 9 commands (stress, init, run, serve, validate, schema, version, mcp, migrate) need dedicated READMEs vs. being covered by existing USAGE.md. Add brief READMEs for any that lack coverage.

## Architecture Context

**FEAT-038 touches these areas:**

- `internal/schema/schema.go` — Add `Extract string`, `DependsOn []string`, `SensitiveVars []string` fields to `Test` struct. Add `Vars map[string]string` to `SmokeConfig` or runtime state.
- `internal/runner/runner.go` — Sequential execution for chained tests. VarStore injection into template rendering. The `runTestOnce()` method needs VarStore access. Current execution is parallel-safe but unordered — chained tests need topological sort or simple sequential ordering.
- `internal/reporter/` — Masking for sensitive variables in terminal and JSON output.
- Template engine — Already supports `{{ .Env.FOO }}`. Extend to `{{ .Vars.bar }}`.

**Key existing types to understand:**
- `schema.Test` — individual test with `Name`, `Run`, `Assertions []Assertion`, `AllowFailure`
- `runner.Runner` — execution engine, holds `Config`, `ConfigDir`, `Reporter`
- `runner.TestResult` — per-test result with `Passed`, `Duration`, `Assertions`, `Error`
- `reporter.Reporter` — output interface

**Important constraint**: Chained tests break the current "all tests are independent" assumption. The runner must:
1. Detect `depends_on:` / `extract:` references and build an execution order
2. Run independent tests in parallel (current behavior), chained tests sequentially
3. Pass VarStore through the execution chain
4. Handle circular dependency detection

## Key Files

| File | Why It Matters |
|------|---------------|
| `docs/planning-mode/2026-04-30-phase1-test-chaining.md` | FEAT-038 implementation plan with P-01 through P-06 |
| `docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md` | Source brainstorm with BR-08 (chaining), BR-13 (observe), all other features |
| `internal/runner/runner.go` | Core execution engine — needs VarStore, sequential ordering |
| `internal/schema/schema.go` | Config structs — needs extract, depends_on, sensitive_vars fields |
| `internal/reporter/reporter.go` | Reporter interface — needs sensitive var masking |
| `cmd/run.go` | Main command — template rendering, config loading patterns |
| `internal/runner/stress.go` | Recent addition (FEAT-044) — example of adding runner capabilities |

## Verification Checklist

Before declaring the session complete:

- [ ] FEAT-038 tests pass: `go test ./internal/runner/ -run "TestChain|TestVarStore|TestExtract" -v`
- [ ] FEAT-038 schema tests pass: `go test ./internal/schema/ -run "TestExtract|TestDepends|TestSensitive" -v`
- [ ] Full suite green: `go test ./... -count=1` (1034+ tests)
- [ ] Binary builds: `go build -o smoke .`
- [ ] Changelog staged for FEAT-036 and FEAT-044
- [ ] Changelog staged for FEAT-038 (if completed)
- [ ] Issue updated: `ccs issues update FEAT-038 --status done`
- [ ] No regressions in self-smoke: `./smoke run`

## Warnings

- **Sequential execution is the hard part.** The current runner may execute tests concurrently. Adding `depends_on:` requires either topological sort or simpler sequential-group detection. Check how `runner.RunAll()` dispatches tests before designing the chain execution flow.
- **VarStore threading.** The VarStore must be available during template rendering (when `Run` command strings are prepared) AND during assertion evaluation (when `extract:` saves values). Check the exact call sites in `runner.go`.
- **Template injection risk.** `{{ .Vars.name }}` in command strings means user-controlled data flows into shell commands. Consider sanitization or documentation of the security implication. The existing `{{ .Env.FOO }}` has the same pattern, so follow that precedent.
- **FEAT-045 plan (Phase 3) is write-only.** Do not start implementing — the observe command is ~1,080 LOC and deserves its own focused session.
