---
brainstorm_ref: docs/brainstorming/2026-05-05-rename-to-smokesig.md
branch: master
completed: "2026-05-12"
covers_brainstorm_deliverables:
    - BR-04
covers_plan_deliverables:
    - P-04
created: "2026-05-05"
goals_completed: 4
goals_total: 4
id: P-2026-05-05-smokesig-docs
plan_ref: docs/planning-mode/2026-05-05-rename-to-smokesig.md
priority: medium
related_prompts: []
requires_reading:
    - docs/brainstorming/2026-05-05-rename-to-smokesig.md
    - docs/planning-mode/2026-05-05-rename-to-smokesig.md
schema_version: 1
status: COMPLETED
tags: []
title: 'SmokeSig: Docs Polish & Remaining Cleanup'
---

# SmokeSig: Docs Polish & Remaining Cleanup

## BEFORE Starting — Required Reading

1. **`docs/brainstorming/2026-05-05-rename-to-smokesig.md`** — rename rationale, decisions, scope.
2. **`docs/planning-mode/2026-05-05-rename-to-smokesig.md`** — original plan (P-01 through P-06 all DONE).

## Context

The repo rename from cosmo-smoke → SmokeSig is complete in code. All 1036 tests pass, binary builds as `smokesig`, GitHub repo is now `CosmoLabs-org/SmokeSig`, remote URL updated and pushed. What remains is documentation polish — the sed-based find-replace got the mechanical substitutions done but the README and CLAUDE.md need a proper human-quality rewrite, and there are scattered `cosmo-smoke` references in historical docs (CHANGELOG, Goss migration comments, test fixture strings) that should be cleaned up.

The project is on v0.20.0 with 7 open features from the Gemini ecosystem brainstorm. FEAT-050 (Backstage.io reporter) has a plan ready at `docs/planning-mode/2026-05-02-backstage-reporter.md`. That's the natural next feature after docs polish.

## What Got Done (Previous Session)

- P-01: Module path renamed in go.mod + all 83 Go files (`github.com/CosmoLabs-org/SmokeSig`)
- P-02: Config file `.smokesig.yaml` primary, `.smoke.yaml` backward compat with deprecation warning
- P-03: Binary `smokesig`, banner "SmokeSig", version output updated
- P-05: Makefile ldflags, `.pre-commit-hooks.yaml`, version registry, plugins registry
- P-06: 1036 tests passing, `go build` clean, grep audit done for Go source
- GitHub repo renamed: `cosmo-smoke` → `SmokeSig`
- Remote URL updated and pushed

## Goals

### [x] G-01 README.md full rewrite
**Model:** `sonnet` | **Files:** `README.md`

The current README had `cosmo-smoke` replaced with `SmokeSig` via sed. It needs a proper rewrite:
- Fresh title and description: "SmokeSig — Universal Smoke Test Runner by CosmoLabs"
- Updated badge URLs: `github.com/CosmoLabs-org/SmokeSig`
- Updated install instructions: `go install github.com/CosmoLabs-org/SmokeSig@latest`
- Updated pre-commit hook: `id: smokesig`
- All command examples: `smokesig run`, `smokesig validate`, etc.
- Config examples: `.smokesig.yaml`
- Keep all assertion type tables and architecture sections intact
- Add a "Migration from cosmo-smoke" section explaining `.smoke.yaml` backward compat

### [x] G-02 Remaining cosmo-smoke reference cleanup
**Model:** `glm-turbo` | **Files:** `CHANGELOG.md`, `internal/migrate/goss/translator.go`, `internal/migrate/goss/translator_test.go`, test fixtures

Grep for remaining `cosmo-smoke` references in the repo (excluding `.git/`, `GOrchestra/sessions/`, and the deprecation/fallback strings in `schema.go`/`discover.go`). Clean up:
- CHANGELOG.md: historical entries mentioning `cosmo-smoke` — leave as-is (it's history) but add a note at top
- `internal/migrate/goss/translator.go`: comments saying "Translate to cosmo-smoke" → "Translate to SmokeSig"
- `internal/migrate/goss/translator_test.go`: any `cosmo-smoke` in test strings
- Test fixture strings that still say `cosmo-smoke` in assertion names or service names

Verify: `grep -ri "cosmo-smoke" --include="*.go" . | grep -v ".git/" | grep -v "deprecated\|fallback\|legacy\|smoke.yaml"` returns zero results.

### [x] G-03 CLAUDE.md quality pass
**Model:** `sonnet` | **Files:** `CLAUDE.md`

The CLAUDE.md had sed replacements applied. Verify:
- Version number is current (0.20.0)
- All command examples say `smokesig` not `smoke`
- Repository path is `github.com/CosmoLabs-org/SmokeSig`
- No double-replaced or mangled text from sed

### [x] G-04 FEAT-050 Backstage.io reporter — begin implementation
**Model:** `glm-turbo` | **Files:** `internal/reporter/backstage.go`, `internal/reporter/backstage_test.go`, `internal/reporter/chain.go`

If docs work finishes quickly, start FEAT-050. Plan is at `docs/planning-mode/2026-05-02-backstage-reporter.md`. Steps:
1. Create `internal/reporter/backstage.go` implementing the Reporter interface (follow `github.go` pattern)
2. Register `"backstage"` in `chain.go` formats map with filename `smoke-backstage.json`
3. Create `internal/reporter/backstage_test.go` with 7 test cases from the plan

## Carry-Overs

1. **[PRIORITY] docs/prompts/2026-05-02-continuation.md** (0/7 goals) — superseded by this prompt. The v0.20.0 release and FEAT-050 plan are both done; remaining goals carried here.

## Where We're Headed

The rename unlocks a fresh identity. After docs polish, the natural next milestone is FEAT-050 (Backstage.io reporter, ~150 LOC, quick win) followed by FEAT-045 (Auto-Add Generator, ~1,080 LOC, the big one). The project is at 1036 tests, 39 assertion types, 7 output formats (soon 8 with Backstage). The Gemini ecosystem brainstorm (`docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md`) has 7 open features remaining.

## Priority Order

1. G-01 README rewrite (highest visibility)
2. G-02 Reference cleanup (completeness)
3. G-03 CLAUDE.md quality pass (correctness)
4. G-04 FEAT-050 Backstage reporter (if time permits)
