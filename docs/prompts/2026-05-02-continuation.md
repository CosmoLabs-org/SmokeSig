---
branch: master
completed: "2026-05-02"
created: "2026-05-02"
goals_completed: 11
goals_total: 11
related_prompts: []
requires_reading: []
schema_version: 1
status: COMPLETED
tags: []
title: 'Continuation: FEAT-044 Flakiness Detector + Push to Origin'
---

# Continuation: FEAT-044 Flakiness Detector + Push to Origin

**Date**: 2026-05-02
**Branch**: master (6 commits ahead of origin)
**Issue**: FEAT-044 (in-progress, plan written)
**Version**: v0.17.0

---

## Current State

cosmo-smoke is at v0.17.0 on master, 6 commits ahead of origin/master. The local commits include FEAT-041 (background commands with wait_for_port), FEAT-043 (GitHub Action), FEAT-037 (file_size assertion tests), and the FEAT-044 plan document.

FEAT-044 has a complete implementation plan at `docs/planning-mode/2026-05-02-flakiness-detector-stress.md` with five deliverables (P-01 through P-05). The plan adds a `smoke stress <test-name>` command that runs a single test N times with configurable parallelism, reports pass rate, timing distribution, and deduplicated errors.

No code for FEAT-044 has been written yet. The plan is TDD-structured: write failing tests first, then implement, then verify.

## Goal

1. Execute the FEAT-044 plan end-to-end, implementing all five deliverables (P-01 through P-05).
2. Push the 6 local commits to origin/master.
3. If FEAT-044 completes with time remaining, optionally begin FEAT-038 (test chaining with data extraction).

## Pre-Flight

Run these BEFORE starting any implementation work:

```bash
# 1. Read the plan (required_reading gate)
# The load-context step will verify all requires_reading files exist.

# 2. Verify clean working tree (or at least understand what's dirty)
git status --short

# 3. Confirm build passes on current state
go build ./...

# 4. Quick test sanity check
go test ./... -count=1 -timeout 60s 2>&1 | tail -5
```

## Execution Plan

### Phase 1: Push to Origin (do this first, takes 10 seconds)

```bash
git push origin master
```

This ships the 6 local commits (v0.17.0 release, FEAT-037, FEAT-041, FEAT-043, audit fixes, FEAT-044 plan). Do this before any new work so origin is up to date.

### Phase 2: Execute FEAT-044 Plan

Follow the plan at `docs/planning-mode/2026-05-02-flakiness-detector-stress.md` exactly. It is TDD-structured in three chunks:

**Chunk 1 -- Stress Engine Core**
- P-01: Stress result types, error deduplication, reliability scoring
  - Create `internal/runner/stress.go` with `StressResult`, `ErrorGroup`, `DedupErrors()`, `ReliabilityStatus()`
  - Create `internal/runner/stress_test.go` with unit tests
  - Tests first, then implementation, then verify pass
- P-02: Worker pool stress execution with atomic counters
  - Add `Runner.StressTest()` method with bounded semaphore-based concurrency
  - Uses `sync/atomic` for passes/failures, `sync.Mutex` for error collection
  - Reuses existing `Runner.runTestOnce()` for each iteration
  - Lifecycle hooks (BeforeAll/AfterAll) and cleanup wired in
  - Tests for: all-pass, with-failures, test-not-found, concurrent execution

**Chunk 2 -- CLI Command and Output**
- P-03: Cobra command wiring
  - Create `cmd/stress.go` with flags: `--runs` (default 50), `--workers` (default 1), `--fail-fast`, `--file`, `--format`
  - Create `cmd/stress_test.go` for flag validation
  - `runStress()` loads config, builds runner, calls `StressTest()`, formats output
- P-04: Terminal summary output formatting
  - `formatStressSummary()` renders: test name, run count, concurrency, duration, reliability percentage, pass/fail counts, deduplicated error groups
  - `reportStressResult()` feeds results through the existing reporter interface

**Chunk 3 -- Edge Cases and Verification**
- P-05: Edge cases, fail-fast behavior, final verification
  - Fail-fast stops after first failure
  - allow_failure tests counted correctly
  - Full test suite passes (1023+ tests)
  - Binary builds and `smoke stress --help` works

**Post-implementation:**
```bash
ccs issues update FEAT-044 --status done
ccs changelog add "FEAT-044: Flakiness detector -- smoke stress command" --type added
```

### Phase 3 (Optional): FEAT-038 Test Chaining

If FEAT-044 completes early, FEAT-038 (test chaining with data extraction) is the next priority. It has an existing plan. Enables auth flows, JWT token passing between tests. Only start this if the session has substantial time remaining -- it is a larger feature.

## Architecture Context

The stress feature integrates with existing cosmo-smoke patterns:

- **Runner reuse**: `Runner.runTestOnce()` is the existing single-test execution path. `StressTest()` wraps it N times with concurrency control.
- **Config-dir-relative**: Commands execute from the config file's directory (same as `smoke run`). The stress command respects this.
- **Reporter interface**: Results feed through the existing `reporter.Reporter` interface, supporting terminal and JSON formats.
- **No new dependencies**: Uses Go stdlib (`sync`, `sync/atomic`, `time`) plus existing Cobra/Lipgloss.

Key types to understand before implementing:
- `schema.SmokeConfig` -- top-level config with `Tests []Test` and `Lifecycle`
- `schema.Test` -- individual test definition with `Name`, `Run`, `AllowFailure`
- `runner.Runner` -- test execution engine with `Config`, `ConfigDir`, `Reporter`
- `runner.TestResult` -- per-test result with `Passed`, `Duration`, `Assertions`, `Error`
- `reporter.Reporter` -- output interface with `TestResult()` and `Summary()` methods

## Key Files

| File | Why It Matters |
|------|---------------|
| `docs/planning-mode/2026-05-02-flakiness-detector-stress.md` | The implementation plan with exact code, tests, and commit steps |
| `docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md` | Original brainstorm, contains BR-02 (flakiness detector concept) |
| `internal/runner/runner.go` | Contains `Runner.runTestOnce()` and lifecycle hook infrastructure |
| `internal/schema/schema.go` | Contains `SmokeConfig`, `Test`, and all assertion type structs |
| `internal/reporter/reporter.go` | Reporter interface definition |

## Verification Checklist

Before declaring FEAT-044 done:

- [x] All stress-specific tests pass: `go test ./internal/runner/ -run "TestStressTest_|TestDedupErrors|TestReliabilityStatus" -v`
- [x] All command tests pass: `go test ./cmd/ -run "TestStressCmd_" -v`
- [x] Full suite green: `go test ./... -count=1`
- [x] Binary builds: `go build -o smoke .`
- [x] Help output correct: `./smoke stress --help`
- [x] Manual smoke: `./smoke run` (self-smoke still passes with 6 tests)
- [x] Issue updated: `ccs issues update FEAT-044 --status done`
- [x] Changelog entry: `ccs changelog add "FEAT-044: ..." --type added`

## Warnings

- The plan references `context.Background()` in the StressTest method -- ensure `Runner` has access to lifecycle hook functions. Check `internal/runner/runner.go` for the exact signatures of `RunLifecycleHooks` and `CleanupBackgroundProcesses` before implementing.
- The `testErrorMessage` helper in the plan accesses `tr.Assertions` -- verify the `TestResult` struct has an `Assertions` field and that each assertion has `Type`, `Expected`, `Actual`, and `Passed` fields. Check `internal/runner/runner.go` for the actual struct definition.
- The `buildReporter` and `loadConfig` functions are used in `cmd/stress.go` -- these are existing helpers in other command files. Verify their signatures match the plan's usage.
