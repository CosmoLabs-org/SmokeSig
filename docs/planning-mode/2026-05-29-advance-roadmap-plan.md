---
created: "2026-05-29T14:00:00-03:00"
status: PENDING
title: "Roadmap Acceleration Plan — May 2026"
tags: [acceleration, planning]
---

# Roadmap Acceleration Plan — May 2026

## Project State

| Metric | Value |
|--------|-------|
| **Project** | SmokeSig v0.24.0 |
| **Open issues** | 4 features, 0 bugs, 0 tasks |
| **Coverage** | runner 91.3%, cmd 85.2%, all others 94%+ |
| **Active worktrees** | 33 (all stale — from prior GLM/agent batches) |
| **Roadmap** | 7/15 foundation complete, 7 roadmap items captured |
| **Test count** | 1,297+ tests |

## Candidate Analysis

### FEAT-052: Wire SmokeSig as external dependency
- **Score: 82/100**
- Priority: High (unblocks FEAT-054)
- Complexity: Simple — CCS-side changes only, not SmokeSig code
- Parallelizable: N/A (requires CCS repo, not SmokeSig)
- **Disposition: SKIP — this is CCS-side work, not SmokeSig**

### FEAT-054: CCS dependency integration
- **Score: 78/100**
- Priority: High (downstream of FEAT-052)
- Complexity: Simple — CCS-side
- Parallelizable: N/A (CCS repo)
- **Disposition: SKIP — CCS-side work, blocked by FEAT-052**

### FEAT-049: OIDC integration for cloud role assumption
- **Score: 55/100**
- Priority: Medium (nice-to-have for enterprise CI)
- Complexity: Complex (~500 lines, new auth subsystem)
- Has plan: No — only brainstorm reference
- Parallelizable: Independent (new `internal/auth/` package)
- **Disposition: NEEDS BRAINPLAN — complex without implementation plan**

### FEAT-048: Wasm plugin system for custom assertions
- **Score: 35/100**
- Priority: Low (blue ocean, no roadmap urgency)
- Complexity: Complex (~800 lines, wazero runtime)
- Has plan: No
- Parallelizable: Independent (new `internal/wasm/` package)
- **Disposition: NEEDS BRAINPLAN — largest feature, lowest priority**

## Dispatchable Work (SmokeSig-internal)

Since all 4 open features are either CCS-side or need planning first, the highest-value acceleration targets are **coverage + quality hardening**:

### Batch 1: Coverage Push (parallel, independent)

| # | Target | Current | Goal | Files | Model | Class |
|---|--------|---------|------|-------|-------|-------|
| 1 | cmd package coverage | 85.2% | 90%+ | `cmd/*_test.go` | GLM 5.1 | indep |
| 2 | dashboard coverage | 94.1% | 97%+ | `internal/dashboard/*_test.go` | GLM 5.1 | indep |
| 3 | detector coverage | 94.5% | 97%+ | `internal/detector/*_test.go` | GLM 5.1 | indep |

### Batch 2: Maintenance (sequential)

| # | Task | Description | Model |
|---|------|-------------|-------|
| 4 | Stale worktree cleanup | 33 abandoned worktrees consuming disk | Manual (`ccs kill --all`) |
| 5 | Commit pending test files | 2 untracked + 1 new coverage_boost5 | Session (commit-all) |

### Batch 3: Feature Planning (sequential, Opus)

| # | Issue | Action | Model |
|---|-------|--------|-------|
| 6 | FEAT-049 | `/brainplan` → implementation plan | Opus |
| 7 | FEAT-048 | `/brainplan` → implementation plan (lower priority) | Opus |

## Recommended Next Actions

1. **Commit current work** — 3 untracked test files + prompt updates
2. **Clean stale worktrees** — `ccs kill --all` to free 33 abandoned worktrees
3. **Dispatch coverage batch** — 3 GLM agents in parallel for cmd/dashboard/detector
4. **Plan FEAT-049** — `/brainplan "OIDC integration for cloud role assumption"` when ready for next feature

## Sequential Queue

None — all dispatchable items are independent (different packages).

## Skipped Items

| ID | Reason |
|----|--------|
| FEAT-052 | CCS repo work, not SmokeSig |
| FEAT-054 | CCS repo work, blocked by FEAT-052 |
| FEAT-048 | Complex, needs plan, lowest priority |
| ROAD-086 | Same as FEAT-048 |
| ROAD-083 | Portfolio-wide adoption — operational, not code |
