---
brainstorm_ref: docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
branch: master
completed: "2026-05-02"
covers_brainstorm_deliverables:
    - BR-01
    - BR-08
    - BR-11
created: "2026-04-30"
goals_completed: 3
goals_total: 3
id: P-2026-04-30-gemini-phase1-continuation
implemented_commits:
    - d60e4ebabd798337890ac7ac92ade7af27081fae
plan_ref: docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md
priority: medium
related_prompts: []
requires_reading:
    - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
    - docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md
schema_version: 1
status: COMPLETED
tags: []
title: Gemini Feedback Phase 1 — FEAT-039 GHA Reporter + Commits
---

# Gemini Feedback Phase 1 — FEAT-039 GHA Reporter + Commits

## BEFORE Starting — Required Reading

Read in order:

1. **`docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md`** — 947-line brainstorm covering 15 deliverables from Gemini AI competitive analysis. Focus on BR-01 (GHA reporter) section.
2. **`docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md`** — Implementation plan for FEAT-039 with 4 deliverables (P-01 through P-04).

## File Scope

```yaml
files_modified:
  - internal/runner/assertion_file.go
  - internal/runner/runner.go
  - internal/schema/schema.go
  - internal/schema/validate.go
  - internal/schema/validate_test.go
files_created:
  - internal/runner/assertion_file_size_test.go
  - internal/runner/chain_test.go
  - internal/runner/varstore.go
  - internal/runner/varstore_test.go
  - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
  - docs/planning-mode/2026-04-30-phase1-file-size-assertion.md
  - docs/planning-mode/2026-04-30-phase1-test-chaining.md
  - docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md
  - docs/issues/FEAT-037.yaml through FEAT-051.yaml
  - docs/roadmap/items/ROAD-077.yaml through ROAD-081.yaml
files_to_create_next_session:
  - internal/reporter/github.go
  - internal/reporter/github_test.go
  - docs/github/problem-matcher.json
```

## Context

This session was a deep brainstorming session fueled by 7 rounds of Gemini AI feedback about cosmo-smoke's competitive positioning. The key insight: Gemini assumed cosmo-smoke only had 5 basic assertion types, when it actually has 39. Of 30+ suggestions, only ~10 were genuinely new.

The brainstorming produced a 947-line doc with 15 deliverables across 5 phases (~5,510 lines total). We filed 5 roadmap items (ROAD-077 through ROAD-081) and 15 feature issues (FEAT-037 through FEAT-051), all cross-linked to the brainstorming doc.

Then we implemented the first two Phase 1 features with TDD:
- **FEAT-037 (file_size assertion)**: ~110 lines. New `FileSizeCheck` struct with min/max byte thresholds, human-readable formatting (B/KB/MB). 16 new tests. Complete.
- **FEAT-038 (test chaining + data extraction)**: ~500 lines. `VarStore` with template resolution (`{{ .Vars.X }}`), `extract:` on `json_field` and `stdout_matches`, secret masking, chain detection. 32 new tests. Complete.

**Nothing is committed yet.** All changes are uncommitted working tree modifications and new files. First task next session: commit everything, then build FEAT-039.

## GLM Dispatch Rules

1. **ALWAYS** use `ccs glm-agent exec` for GLM agents (routes through queue with retry)
2. **NEVER** use Agent tool with `model:sonnet`/`model:haiku` for GLM work (bypasses queue)
3. Agent tool with `model:opus` is fine for Opus subagents
4. For parallel work: use `/glm-sprint` or `ccs glm-agent exec-batch`

## What Got Done

**Brainstorming & Planning:**
- Processed 7 batches of Gemini AI feedback, triaged each against actual codebase
- Created 947-line brainstorming doc with 15 deliverables across 5 phases
- Filed ROAD-077 through ROAD-081 (5 roadmap items, Phase 1-5)
- Filed FEAT-037 through FEAT-051 (15 feature issues)
- Wrote 3 implementation plans for Phase 1 (file size, test chaining, GHA reporter)

**Implementation (TDD):**
- FEAT-037: `file_size` assertion — schema struct, runner dispatch, validation, 16 tests
- FEAT-038: test chaining — VarStore, extract from json_field/stdout_matches, template resolution, chain detection, secret masking, 32 tests

## Goals

### [x] G-01 Commit all session work
**Model:** `opus` | **Priority:** 1

Before any new code, commit the uncommitted work from this session:

**Group 1 — Brainstorming, roadmap, issues, plans (docs):**
- `docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md` (new)
- `docs/planning-mode/2026-04-30-phase1-*.md` (3 new)
- `docs/roadmap/items/ROAD-07*.yaml`, `ROAD-08*.yaml` (5 new)
- `docs/issues/FEAT-03*.yaml` through `FEAT-051.yaml` (15 new)
- `docs/roadmap/index.yaml` (modified)
- `docs/issues.yaml` (modified)

**Group 2 — FEAT-037 file_size assertion (code):**
- `internal/schema/schema.go` — FileSizeCheck struct + Expect.FileSize field
- `internal/schema/validate.go` — file_size validation + hasStandaloneAssertions
- `internal/schema/validate_test.go` — 4 validation tests
- `internal/runner/assertion_file.go` — CheckFileSize + formatBytes + fileSizeRange
- `internal/runner/assertion_file_size_test.go` (new) — 12 assertion tests
- `internal/runner/runner.go` — file_size dispatch

**Group 3 — FEAT-038 test chaining with data extraction (code):**
- `internal/schema/schema.go` — JSONFieldCheck.Extract + Expect.Extract fields
- `internal/runner/varstore.go` (new) — VarStore, extractFromJSON, extractFromRegex, processExtracts, detectChains, findVarReferences
- `internal/runner/varstore_test.go` (new) — 28 VarStore/extract/chain tests
- `internal/runner/chain_test.go` (new) — 4 integration tests
- `internal/runner/runner.go` — Vars field on Runner, var resolution in runTestOnce, processExtracts call

Use `/commit-all` or `ccs commit-batch` for grouped commits. Conventional commit format.

### [x] G-02 Build FEAT-039 GitHub Actions native output reporter
**Model:** `glm-turbo` | **Files:** `internal/reporter/github.go`, `internal/reporter/github_test.go`, `docs/github/problem-matcher.json`
**Plan:** `docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md`

Follow TDD (invoke `superpowers:test-driven-development` first). Implementation:

**Step 1 — RED: Write tests in `internal/reporter/github_test.go`:**
- `TestGitHubActions_SummaryAllPass` — generates markdown table with pass results
- `TestGitHubActions_SummaryWithFailures` — includes failed test section
- `TestGitHubActions_WorkflowCommands` — emits `::error title=Smoke Test Failed::...` for failures
- `TestGitHubActions_WarningForAllowedFailure` — emits `::warning` for allow_failure tests
- `TestGitHubActions_NoStepSummaryEnv` — graceful fallback to stdout when `$GITHUB_STEP_SUMMARY` not set
- `TestGitHubActions_EmptySuite` — handles zero tests

**Step 2 — GREEN: Implement in `internal/reporter/github.go`:**
```go
type GitHubActions struct {
    summaryPath string // $GITHUB_STEP_SUMMARY
    w           io.Writer
    tests       []ghaTestResult
}
func NewGitHubActions(w io.Writer) *GitHubActions
func (g *GitHubActions) PrereqStart(name string)
func (g *GitHubActions) PrereqResult(r reporter.PrereqResultData)
func (g *GitHubActions) TestStart(name string)
func (g *GitHubActions) TestResult(r reporter.TestResultData)
func (g *GitHubActions) Summary(s reporter.SuiteResultData) error
```
- `TestResult` collects results in `g.tests`
- `Summary` writes markdown table to `$GITHUB_STEP_SUMMARY` file + emits workflow commands via `fmt.Fprintf(g.w, "::error ...")`

**Step 3 — Wire into format registration:**
- Modify `internal/reporter/multi.go`: add `"gha"` to valid formats, create `NewGitHubActions` when format matches

**Step 4 — Create problem matcher `docs/github/problem-matcher.json`:**
```json
{
  "problemMatcher": [{
    "owner": "cosmo-smoke",
    "pattern": [{
      "regexp": "^::error title=Smoke Test Failed,file=(.+?)::(.+)$",
      "file": 1, "message": 2
    }]
  }]
}
```

**Acceptance:** `go test ./internal/reporter/ -run TestGitHubActions -v` passes. `go test ./...` passes full suite (currently 961 tests).

### [x] G-03 Update issue FEAT-039 status after implementation
**Model:** `opus`

After G-02 passes all tests:
```bash
ccs issues update FEAT-039 --status done
```

## Where We're Headed

**Phase 1 is 2/3 done** (FEAT-037 + FEAT-038 shipped, FEAT-039 next). After FEAT-039, Phase 1 is complete.

**Phase 2 (Lifecycle & Orchestration)** is next on the roadmap:
- FEAT-040: Setup/teardown lifecycle hooks (extends existing `prerequisites:`)
- FEAT-041: Background commands with `wait_for_port`
- FEAT-042: Remote config inheritance (`extends: URL`)
- FEAT-043: Official GitHub Action wrapper

The big unlock after Phase 1 is complete: **test chaining + lifecycle hooks together** enable real multi-step smoke test scenarios (start Docker → wait for port → login → extract token → verify API). That's the workflow most users actually need.

## Priority Order
1. G-01 (commit) — must happen before any new work
2. G-02 (FEAT-039 GHA reporter) — completes Phase 1
3. G-03 (update issue) — housekeeping
