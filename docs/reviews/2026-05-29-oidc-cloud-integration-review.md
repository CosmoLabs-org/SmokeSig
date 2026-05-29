---
date: "2026-05-29T14:30:00-03:00"
plan: docs/planning-mode/2026-05-29-oidc-cloud-integration.md
brainstorm: docs/brainstorming/2026-05-29-oidc-cloud-integration.md
issue: FEAT-049
reviewer: independent-review
status: fixes-applied
---

# Review: FEAT-049 OIDC Cloud Integration Plan

## Overall Score: 8.2 / 10

## Per-Dimension Scores

| Dimension | Score | Notes |
|-----------|-------|-------|
| **Completeness** | 9/10 | All 17 deliverables covered. Missing `--no-auth` flag implementation (mentioned in docs task but no code task). Fixed. |
| **Accuracy** | 7/10 | Several line number references were close but wrong. ExchangeAWS had dual function signatures (profile struct then individual params). Unexported field access across package boundary would not compile. Fixed all. |
| **Implementability** | 8/10 | Code is detailed enough to implement from. Watch mode integration was architecturally wrong (runner is local to closure, not accessible in watch loop). GCP signal handler had goroutine leak pattern. Fixed both. |
| **Review feedback incorporation** | 9/10 | All 8 reviewer feedback items incorporated: no build tag, SigV4 documented as v1 limitation, GCP defaults to env var not keyfile, token clock skew validation, auth failure as synthetic prereq, monorepo inheritance, CI API versioning comments, assertion signatures internal-only. |
| **Test coverage** | 8/10 | Comprehensive unit + integration test plan. Mock STS servers via httptest. All validation rules have test cases. Missing: test for `--no-auth` flag, test for watch mode TTL caching, test for CleanupKeyfiles method. |

## Top 3 Strengths

1. **Zero external dependencies.** The plan stays true to SmokeSig's minimal-dep philosophy. AWS STS via `encoding/xml` and GCP STS via `encoding/json` are elegant. The explicit reasoning for why the gRPC build-tag precedent does not apply (500 LOC stdlib vs 50MB grpc) is well-argued and correct.

2. **Security-conscious design throughout.** Byte slices instead of strings for credential storage (enables zeroing), `CleanupKeyfiles` with SIGTERM handler for crash-path cleanup, credential masking with known-prefix detection, redacted `String()` method, and env var cleanup in deferred teardown. The accepted SIGKILL risk is honestly documented.

3. **Thorough brainstorm-to-plan traceability.** Every brainstorm deliverable (BR-01 through BR-10) maps to specific plan deliverables (P-01 through P-17). Design decisions in the brainstorm are preserved and refined, not lost. The brainstorm's "open questions" are all resolved in the plan (GCP credential format, monorepo inheritance, regional STS endpoints).

## Top 3 Issues Found and Fixed

### Issue 1: Unexported field access across package boundary (CRITICAL -- would not compile)

**Location:** Task 9a, credential cleanup defer block

**Problem:** The plan accessed `authCtx.mu.RLock()` and iterated `authCtx.profiles` directly from the `runner` package. Both `mu` and `profiles` are lowercase (unexported) fields on `AuthContext` defined in the `auth` package. This would cause a compile error.

**Fix applied:** Added a `CleanupKeyfiles()` exported method to `AuthContext` in Task 3 that encapsulates the keyfile cleanup. Updated Task 9a defer block to call `authCtx.CleanupKeyfiles()` instead of accessing unexported fields. Added `"os"` to the auth.go imports.

### Issue 2: Watch mode TTL refresh was architecturally wrong (MODERATE -- would not work at runtime)

**Location:** Task 10

**Problem:** The plan proposed adding a TTL check using `r.authCtx` in the watch loop, but the `Runner` instance (`r`) is local to the `runOnce` closure in `cmd/run.go`. Each watch cycle creates a fresh `Runner`. The watch loop in `runWatch()` (line 410) has no access to any Runner's auth context. The plan even said "Locate watch mode entry point (likely `cmd/run.go` or `internal/runner/watch.go`)" showing uncertainty about where watch mode lives.

**Fix applied:** Rewrote Task 10 to use a package-level credential cache (`ExchangeAllCached`) in the auth package. Watch mode cycles call `ExchangeAllCached` (reuses valid credentials) instead of `ExchangeAll` (always re-exchanges). Controlled via a new `WatchMode bool` field on `RunOptions`. Correctly identified the watch loop location at `cmd/run.go` line 410.

### Issue 3: ExchangeAWS had dual conflicting function signatures (MODERATE -- confusing for implementer)

**Location:** Task 5

**Problem:** The code block defined `ExchangeAWS(profile AuthProfile, oidcToken string, stsEndpoint string)` with `profile.RoleARN`, `profile.Region`, etc. Then a correction paragraph changed the signature to individual parameters. But the code body still used `profile.X` field access. An implementer would need to reconcile two contradictory versions.

**Fix applied:** Updated the code block to use the corrected individual-parameter signature throughout. Function body now uses `roleARN`, `region`, `sessionDuration` parameter names consistently. Removed the confusing dual-definition paragraph.

## Additional Issues Fixed

| Issue | Severity | Fix |
|-------|----------|-----|
| Task 9c wrong line reference (said "line ~419" but should be before `if t.Run != ""` at line ~393) | Low | Updated to reference line ~393 with explanation that override must cover both commands and standalone assertions |
| Task 11 imprecise line reference for RunMonorepo | Low | Updated to specify exact lines 185 and 188 |
| Task 16 mentioned `--no-auth` flag but no task implemented it | Moderate | Added implementation code to Task 16a (mirrors `--no-otel` pattern in `cmd/run.go`) |
| GCP signal handler goroutine leak (each exchange spawns a goroutine blocked forever on signal channel) | Low | Added `signal.Stop(sigCh)` after cleanup to unregister, added comment explaining why one goroutine per profile is acceptable |
| Task 2 said "Add `"time"` to imports" but did not mention `"regexp"` was already imported (could confuse implementer into thinking it needs adding) | Low | Clarified that `"regexp"` is already imported in validate.go |

## Items NOT Changed (Correct as-is)

- Schema struct placement (Auth after OTel on SmokeConfig, Auth after Retry on Test) matches existing field ordering conventions
- Validation rules are comprehensive and follow the "all errors at once" pattern consistently
- The `fallback: "fail"` default is the right security choice
- CI detection priority order (GitHub > GitLab > CircleCI > custom) is reasonable
- The decision to not import schema from the auth package (individual params instead) is architecturally sound
- LOC estimates are plausible given the code shown
- Execution order dependency graph is correct
- Parallelizable task groups are accurately identified
