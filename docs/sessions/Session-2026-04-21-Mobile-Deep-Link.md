---
created: ""
goals_completed: 0
goals_total: 0
origin: migrated by ccs prompts migrate
priority: medium
related_prompts: []
requires_reading: []
schema_version: 1
status: COMPLETED
tags: []
title: Session 2027 - 2026-04-21 - Mobile Deep Link Assertion
---

# Session 2027 - 2026-04-21 - Mobile Deep Link Assertion

## Date
2026-04-21

## Branch
master

## Summary

This session recovered from a context limit crash in the previous session (#2026), committed that session's leftover work, and then designed and shipped FEAT-013: the mobile deep link assertion. The feature adds a `deep_link` assertion type that validates mobile universal link and app link infrastructure -- the `.well-known/assetlinks.json` files that Android requires and the `apple-app-site-association` files that iOS requires -- alongside opt-in tier 2 resolution using native developer tools (adb, xcrun).

The session started with recovery. Session #2026 had attempted `/session-end` four or more times, hitting a loop where `ccs session-status` exited with code 1 and triggered a retry cycle. Using `/previous-output`, we recovered the context, committed the three outstanding work items (watch mode fixes for config reload and TraceHealth persistence, GOrchestra build ignore tags across 25 archived agent files, and the FEAT-013 brainplan docs), and got the working tree clean before starting new work.

FEAT-013 was executed via the brainplan workflow: a brainstorm doc at `docs/brainstorming/2026-04-21-mobile-deep-link-assertion.md` established the why and the architecture, then a planning doc at `docs/planning-mode/2026-04-21-mobile-deep-link-assertion.md` laid out 7 goals with file-level specifics. All 7 goals were completed:

- **G-01**: `DeepLinkCheck` struct in `internal/schema/schema.go` with fields for URL, Android package, iOS bundle ID, associated domains, opt-out toggles for assetlinks/AASA checks, and a tier selector (`auto`/`config-only`/`full-resolve`). One new validation test added, bringing schema tests to 87.

- **G-02**: Four new mobile detector types in `internal/detector/detector.go` -- ReactNative, Flutter, iOS (Xcode), and Android (Gradle) -- each with marker file heuristics (`pubspec.yaml`, `build.gradle`, `*.xcodeproj`, `app.json`/`app.config.js`). Five new detector tests, bringing detector tests to 44.

- **G-03**: Tier 1 HTTP checks in `internal/runner/assertion_deeplink.go` (184 lines). `CheckAssetlinks` fetches and parses `.well-known/assetlinks.json`, validating the `delegate_permission/common.handle_all_urls` relation for the target Android package. `CheckAASA` tries both `/apple-app-site-association` and `/.well-known/apple-app-site-association` paths, parsing the `applinks.details.appIDs` array for a matching bundle ID. Seven new tests covering success, missing files, malformed JSON, wrong package names, and both AASA paths.

- **G-04**: `CheckDeepLink` entry point with tier-based routing. For web URLs (http/https), it runs tier 1 checks first. Then, if tools are available (adb for Android, xcrun for iOS), it runs tier 2 resolution. The `config-only` tier skips tool-augmented checks entirely; `full-resolve` fails if tools are missing; `auto` (default) runs tier 2 silently when tools exist. Four new tests for tier routing logic.

- **G-05**: Runner wiring in `internal/runner/runner.go`. The `deep_link` assertion case was added to the `runTestOnce` switch, calling `CheckDeepLink` and collecting results. One integration test verifying the end-to-end pipeline.

- **G-06**: Mobile smoke init templates in `internal/detector/templates.go`. Four new templates (ReactNative, Flutter, iOS, Android) generating starter `.smoke.yaml` files with deep_link assertions appropriate to the platform.

- **G-07**: CLAUDE.md updated with the `deep_link` assertion type documentation and the four mobile detector types in the detected project types list.

The test count moved from 782 to 802 across the session -- 20 new tests. The assertion_deeplink.go file is the largest addition at 184 lines of implementation plus 173 lines of tests, reflecting the dual-platform validation logic and the tier routing system.

One piece of feedback was filed: FB-610, documenting that `ccs commit-analyze` produces false positives on Go files containing `//go:build ignore` directives (the archived GOrchestra test files), since it interprets the build tag as a meaningful code change rather than an intentional exclusion.

## Key Decisions

| Decision | Options Considered | Why This Choice |
|----------|-------------------|-----------------|
| Tier-based assertion model (auto/config-only/full-resolve) | (A) HTTP-only, (B) Always try tools, (C) Tiered | Tiered model matches the reality: most CI environments lack adb/xcrun, but local dev machines have them. `auto` does the right thing in both contexts without configuration burden |
| Pointer fields for CheckAssetlinks/CheckAASA toggles | (A) Boolean with default true, (B) Pointer to bool | Pointer fields distinguish "not specified" (nil, default true) from "explicitly disabled" (false). Critical for the opt-out semantics |
| AASA dual-path probing (both root and .well-known) | (A) Only `.well-known` path, (B) Try both paths in order | Apple supports both locations. iOS checks `.well-known` first, then root. Matching that order ensures the assertion validates what iOS actually does |
| assetlinks.json relation matching with `delegate_permission/common.handle_all_urls` | (A) Any relation match, (B) Specific relation string | The specific relation is what Android requires for App Links verification. Anything else is informational, not actionable |
| Four separate detector types rather than a single "mobile" type | (A) Generic "mobile" detector, (B) Per-framework types | Per-framework types produce accurate templates (React Native projects need different smoke configs than Flutter or native iOS) |

## Task Log

| # | Task | Status | Notes |
|---|------|--------|-------|
| 1 | Session recovery via /previous-output | completed | Recovered from context limit crash in session #2026 |
| 2 | Commit session #2026 leftover work (3 commits) | completed | Watch mode fixes, GOrchestra build ignore tags, FEAT-013 brainplan |
| 3 | G-01: DeepLinkCheck struct in schema | completed | `internal/schema/schema.go`, 1 test, 87 total schema tests |
| 4 | G-02: Mobile detector types (4 types) | completed | `internal/detector/detector.go`, 5 tests, 44 total detector tests |
| 5 | G-03: Tier 1 HTTP checks (CheckAssetlinks, CheckAASA) | completed | `internal/runner/assertion_deeplink.go`, 7 tests |
| 6 | G-04: CheckDeepLink entry with tier routing | completed | `internal/runner/assertion_deeplink.go`, 4 tests |
| 7 | G-05: Runner wiring in runTestOnce | completed | `internal/runner/runner.go`, 1 integration test, 248 runner tests |
| 8 | G-06: Mobile smoke init templates | completed | `internal/detector/templates.go`, 4 templates |
| 9 | G-07: CLAUDE.md documentation update | completed | Deep link assertion + mobile detector types |
| 10 | File FB-610 (commit-analyze false positive) | completed | `//go:build ignore` in archived agent files |

## Reference

- **Commits**: `69adf4d..e088c15` (12 commits)
- **Key feature commits**:
  - `27b1631` feat(schema): add DeepLinkCheck struct for mobile deep link assertions
  - `8045997` feat(detector): add React Native, Flutter, iOS, Android project types
  - `b7fd8c2` feat(runner): add deep link assertion with tier 1 HTTP checks and tier 2 resolution
  - `0402f91` feat(runner): wire deep_link assertion into test execution pipeline
  - `68e269f` feat(detector): add mobile project smoke init templates
  - `fac8bf6` docs: add deep_link assertion type and mobile detector types to CLAUDE.md
- **Supporting commits**:
  - `2cc0e26` fix(cmd): fix watch mode config reload and TraceHealth persistence
  - `9e82ee2` chore: add build ignore tags to archived GOrchestra agent files
  - `372d990` docs: add FEAT-013 mobile deep link brainplan and session metadata
- **Files modified**: 14 files changed, 3,566 insertions, 40 deletions
- **New files**: `internal/runner/assertion_deeplink.go` (184 lines), `internal/runner/assertion_deeplink_test.go` (173 lines)
- **Tests**: 802 passing (20 new), build clean
- **Issues filed**: FB-610 (commit-analyze false positive on `//go:build ignore`)

## Related

- [Brainstorm: Mobile Deep Link Assertion](../brainstorming/2026-04-21-mobile-deep-link-assertion.md) - Design rationale and architecture decisions
- [Plan: Mobile Deep Link Assertion](../planning-mode/2026-04-21-mobile-deep-link-assertion.md) - Goal breakdown and file-level implementation plan
- [Session 2026-04-20 - Test Coverage Hardening](Session-2026-04-20-Test-Coverage-Hardening.md) - Previous session (782 tests)
