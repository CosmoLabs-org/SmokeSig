---
status: PENDING
type: audit-followup
priority: high
---
# Audit Followup: SmokeSig 2026-05-24

## Critical Bugs to Fix First
1. Complete cosmo-smoke → SmokeSig rename in .goreleaser.yml, Dockerfile, smoke.yml, SPEC.md, STABILITY.md, FEATURES.md, examples/
2. Fix Docker Compose `append` no-op — assertion_docker.go:65
3. Fix SMTP double-handshake — assertion_smtp.go:38-62
4. Add DeepLink to hasStandaloneAssertions — validate.go:226
5. Update version fallback 0.13.0 → 0.21.1 — cmd/version.go:10
6. Fix MCP assertion count 29 → 40 — internal/mcp/server.go:196

## Quick Wins (small effort, high impact)
1. Add non-root USER to Dockerfile — effort: small
2. Add HTTP server timeouts to serve.go — effort: small
3. Use subtle.ConstantTimeCompare for API key — handler.go:30 — effort: small
4. Add 12 missing types to ExportSchema() — export.go — effort: small
5. Fix IPv6 with net.JoinHostPort — 8 files — effort: small
6. Rebuild issue index — effort: small
7. Update --format help to include gha, backstage — run.go:109 — effort: trivial

## Roadmap Items to Start
1. ROAD-082: CCS dependency integration
2. Create release workflow (.github/workflows/release.yml)
3. Rewrite SPEC.md from scratch

## Files to Read First
- docs/audit/2026-05-24-smokesig/brief.md (project context)
- docs/audit/2026-05-24-smokesig/risk-map.md (where NOT to touch without tests)
- docs/audit/2026-05-24-smokesig/upgrade-plan.md (phased action plan)
