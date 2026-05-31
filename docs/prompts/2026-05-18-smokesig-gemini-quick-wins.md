---
branch: master
completed: "2026-05-30"
created: "2026-05-18T12:00:00-03:00"
goals_completed: 3
goals_total: 3
priority: high
related_prompts:
    - docs/prompts/2026-04-30-gemini-phase1-continuation.md
    - docs/prompts/2026-05-05-smokesig-docs.md
requires_reading:
    - CLAUDE.md
    - internal/runner/assertion_file.go
    - internal/runner/assertion_file_size_test.go
    - internal/reporter/github.go
    - internal/reporter/push.go
    - internal/runner/runner.go
    - internal/schema/schema.go
schema_version: 1
status: COMPLETED
tags:
    - gemini
    - phase1
    - quick-wins
    - implementation
title: SmokeSig Gemini Phase 1 Quick Wins — Next Implementation Session
---

# SmokeSig Gemini Phase 1 Quick Wins — Continuation Prompt

## File Scope

```yaml
files_modified:
  - internal/runner/runner.go
  - internal/schema/schema.go
files_created: []
```

## Context

SmokeSig is post-rename (cosmo-smoke -> SmokeSig), post-v0.20.1, and post-Backstage reporter. The project has 76/81 completed roadmap items with strong momentum. The last session was a docs/integration session that wired SmokeSig into CCS as an external dependency and expanded the roadmap with 5 new items (ROAD-082 through ROAD-086). No source code was changed in SmokeSig itself during that session. The next big unlock is CCS integration (ROAD-082, p95) but that is blocked waiting for CCS to review the edits made to their repo. Meanwhile, the highest-value unblocked work is ROAD-077 (p90): Gemini Phase 1 Quick Wins. The three FEATs that comprise it (FEAT-037, FEAT-038, FEAT-039) are all marked done/closed in the issue tracker already, meaning they were implemented in prior sessions. However, ROAD-077 remains in `exploring` status on the roadmap, which may indicate incomplete integration or follow-up polish. Verify status before starting new work -- if Phase 1 is truly done, shift to ROAD-078 (Phase 2) or ROAD-084 (Push Reporter).

## What Got Done (Previous Session)

- **CCS Dependency Wiring**: Edited 4 CCS files (smoke.go, rebuild.go, project registry) to reference `smokesig` binary instead of old `cosmo-smoke`. Sent as FB-010 for CCS review. Creates ROAD-082 (p95).
- **USAGE.md Rewrite**: Rewrote SmokeSig's USAGE.md for the new branding, full command set, and assertion catalog.
- **Roadmap Expansion**: Added 5 new roadmap items: ROAD-082 (CCS integration, p95), ROAD-083 (portfolio adoption, p70), ROAD-084 (push reporter, p55), ROAD-085 (TUI runner, p40), ROAD-086 (Wasm plugins, p25).
- **FEAT-052 Filed**: Tracked the CCS wiring work as an open issue.
- **Feedback Sent**: FB-010 to CCS project with full details of the integration edits.

## Goals

### [x] 1. Verify Gemini Phase 1 Completion and Polish
**Model:** `sonnet` | **Files:** `internal/runner/assertion_file.go`, `internal/reporter/github.go`, `internal/runner/runner.go`, `internal/schema/schema.go`

ROAD-077 is in `exploring` status but all three child issues (FEAT-037 file_size, FEAT-038 test chaining, FEAT-039 GHA reporter) are marked done/closed. This discrepancy needs investigation:

1. Read `internal/runner/assertion_file.go` and `internal/runner/assertion_file_size_test.go` -- verify `file_size` assertion exists with `min_bytes`/`max_bytes` fields and is wired into `checkAssertion()` dispatch.
2. Read `internal/runner/runner.go` -- verify test chaining works: `extract:` field on `json_field`/`stdout_matches` assertions populates `{{ .Vars.name }}` for subsequent tests, chained tests run sequentially.
3. Read `internal/reporter/github.go` -- verify `--format gha` writes markdown to `$GITHUB_STEP_SUMMARY` and emits `::error`/`::warning` annotations.
4. Run `go test ./internal/runner/ -run TestFileSize -v` and `go test ./internal/reporter/ -run TestGitHub -v` -- confirm tests pass.
5. If all three are genuinely complete and tested, update ROAD-077 status to `completed` via `ccs roadmap update ROAD-077 --status completed`.

If any feature is incomplete or has gaps, create specific goals below for the missing pieces before proceeding to Goal 2.

**Acceptance:** `go test ./internal/... -count=1` passes. ROAD-077 status is either `completed` or has concrete remaining-work goals filed.

### [x] 2. Push Reporter Verification and Enhancement (ROAD-084)
**Model:** `glm-turbo` | **Files:** `internal/reporter/push.go`, `internal/reporter/push_test.go`, `cmd/run.go`

The push reporter scaffolding already exists in `internal/reporter/push.go` with tests in `push_test.go`. ROAD-084 is in `captured` status. Verify what's already built and determine what's missing:

1. **Read `internal/reporter/push.go`** -- verify it implements the `Reporter` interface, sends HTTP POST to `--report-url`, includes auth header when `--report-api-key` is set, and handles timeouts gracefully.
2. **Read `internal/reporter/push_test.go`** -- verify test coverage for success, auth, timeout, and server error cases.
3. **Read `cmd/run.go`** -- verify `--report-url` and `--report-api-key` flags wire the push reporter into the multi-reporter pipeline.
4. **Run `go test ./internal/reporter/ -run TestPush -v`** -- confirm all tests pass.
5. **If complete**: Update ROAD-084 to `completed` via `ccs roadmap update ROAD-084 --status completed`.
6. **If gaps found**: Scope the missing pieces (e.g., Slack/PagerDuty formatting, retry logic) and implement them. File specific follow-up goals.

**Acceptance:** `go test ./internal/reporter/ -run TestPush -v` passes. ROAD-084 status updated based on findings.

### [x] 3. ROAD-078 Exploration — Gemini Phase 2 Lifecycle Hooks
**Model:** `sonnet` | **Files:** `internal/schema/schema.go`, `internal/runner/runner.go`

If Goals 1 and 2 are complete, begin exploring Phase 2 (ROAD-078). The two Phase 2 items from the Gemini brainstorm are:

- **FEAT-040 (setup/teardown lifecycle hooks)**: Marked `done`. Verify implementation in runner -- global and per-test `setup:`/`teardown:` commands that run before/after tests.
- **FEAT-041 (background commands with `wait_for_port`)**: Marked `done`. Verify implementation -- `background: true` on test `run:` commands that spawn processes and `wait_for_port:` assertion to gate subsequent tests.

Read the implementation, run tests, and if both are genuinely complete, update ROAD-078 to `completed` and identify what remains for the Gemini roadmap phases (ROAD-079/080/081 are all `captured` status -- none started).

If either FEAT-040 or FEAT-041 is incomplete, scope the remaining work and create a concrete implementation goal.

**Acceptance:** `go test ./internal/runner/ -v -count=1` passes. ROAD-078 status updated based on findings.

## Carry-Over Tasks

No carry-over tasks from the previous session. All work was completed cleanly.

## Carry-Overs

1. **docs/prompts/2026-05-05-smokesig-docs.md** -- Previous docs session prompt. Work completed (rename cleanup, USAGE.md, backstage reporter docs).

## Where We're Headed

The project is at a natural inflection point. The core tool is mature: 39 assertion types, 31 project detectors, 8 output formats, monorepo support, OTel integration, test chaining. The roadmap has 76/81 items done. The remaining open issues (FEAT-045 through FEAT-052) are all advanced features -- observe command, TUI, Wasm plugins, OIDC -- that are phase 4/5 ecosystem work.

The near-term trajectory is:

1. **CCS integration lands** (ROAD-082) -- unblocks portfolio-wide adoption (ROAD-083). This is the single highest-priority item but blocked externally.
2. **Push reporter ships** (ROAD-084) -- fills a gap in CI/CD workflows where teams need results pushed to Slack/PagerDuty/webhooks rather than polling files.
3. **Gemini phases close out** -- Phase 1 (ROAD-077) and Phase 2 (ROAD-078) verification, then move to Phase 3 Quality Tooling (ROAD-079) if the roadmap items there are still relevant.
4. **Portfolio adoption** (ROAD-083) -- once CCS integration lands, the big unlock: wire `.smokesig.yaml` configs across CosmoLabs' ~95-project portfolio.

The project has strong momentum. Every session closes multiple roadmap items. The challenge shifts from "what to build next" to "what to prioritize among many good options." The push reporter is the highest-value unblocked work right now because it completes the CI/CD story -- run tests, get results pushed to your team's tools.

## GLM Dispatch Rules

When goals involve dispatching subagents:

1. **ALWAYS** use `ccs glm-agent exec` for GLM agents (routes through queue with retry logic)
2. **NEVER** use Agent tool with `model:sonnet` or `model:haiku` for GLM work (bypasses queue, risks 429 rate limits)
3. Agent tool with `model:opus` is fine for Opus subagents
4. For parallel work: use `/glm-sprint` or `ccs glm-agent exec-batch`

## Priority Order

1. Goal 1 -- Verify Phase 1 status before building anything (prevents duplicate work)
2. Goal 2 -- Push Reporter is highest-value unblocked implementation work
3. Goal 3 -- Phase 2 verification (stretch goal, can carry to next session)
