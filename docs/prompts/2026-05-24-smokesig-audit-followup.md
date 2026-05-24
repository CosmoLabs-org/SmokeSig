---
created: "2026-05-24T12:00:00-03:00"
goals_completed: 9
goals_total: 9
priority: high
related_prompts: []
requires_reading:
    - docs/audit/2026-05-24-smokesig/brief.md
    - docs/audit/2026-05-24-smokesig/risk-map.md
    - docs/audit/2026-05-24-smokesig/upgrade-plan.md
schema_version: 1
started: "2026-05-24"
status: STARTED
tags: []
title: 'Audit Followup: SmokeSig 2026-05-24'
type: audit-followup
---

# Audit Followup: SmokeSig 2026-05-24

## Goal
Continue Phase 1 (Documentation, CI hardening, UX polish) from the audit upgrade plan. Phase 0 critical fixes are complete as of v0.20.1.

## Completed This Session (v0.20.1)
- Complete cosmo-smoke → SmokeSig rename (.goreleaser.yml, Dockerfile, smoke.yml, SPEC.md, STABILITY.md, FEATURES.md, examples/) ✅
- Fix Docker Compose `append` no-op — assertion_docker.go:65 ✅
- Add DeepLink to hasStandaloneAssertions — validate.go:226 ✅
- Update version fallback 0.13.0 → 0.21.1 — cmd/version.go:10 ✅
- Fix MCP assertion count 29 → 40 — internal/mcp/server.go:196 ✅
- Add non-root USER to Dockerfile ✅
- Add HTTP server timeouts to serve.go ✅
- Use subtle.ConstantTimeCompare for API key — handler.go:30 ✅
- Rebuild issue index (auto-rebuilt to 64 items) ✅
- Update --format help to include gha, backstage — run.go:109 ✅
- IPv6 fix with net.JoinHostPort ✅
- Race condition fixes ✅
- 12 missing types added to ExportSchema ✅
- Test coverage improvements ✅

## Remaining Work

### Bugs
1. **BUG-004** Fix SMTP double-handshake — assertion_smtp.go:38-62 (sends EHLO twice; first response consumed and discarded before second exchange begins)

### Documentation
2. Rewrite SPEC.md from scratch covering all 40 assertion types (current version is outdated, references old name and missing ~11 types)
3. Document MCP server (tools, resources, prompts), Dashboard API endpoints, and lifecycle hooks in dedicated docs
4. Update FEATURES.md to v0.21.1 (add all new assertion types and features added since last update)

### CI / Quality
5. Add golangci-lint + security scanning (gosec or similar) to CI pipeline
6. Clean CHANGELOG corruption — 8 malformed/duplicate entries detected

### UX / Flags
7. Add --verbose / --quiet flags to `smokesig run` for output verbosity control
8. Add warning on push reporter and OTel reporter failures — BUG-008 (currently fails silently, user sees no indication the push/export failed)

### Content / Marketing
9. Content strategy: terminal screenshot for README hero, comparison pages (vs. other smoke test tools)
