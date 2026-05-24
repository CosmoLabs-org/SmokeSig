---
created: "2026-05-24T12:00:00-03:00"
goals_completed: 0
goals_total: 0
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
Execute Phase 0 (Critical Fixes) from the audit upgrade plan. The audit found 10 critical/high bugs and an incomplete project rename affecting the entire release pipeline.

## Critical Bugs to Fix First
1. Complete cosmo-smoke → SmokeSig rename in .goreleaser.yml, Dockerfile, smoke.yml, SPEC.md, STABILITY.md, FEATURES.md, examples/
2. Fix Docker Compose `append` no-op — assertion_docker.go:65
3. Fix SMTP double-handshake — assertion_smtp.go:38-62
4. Add DeepLink to hasStandaloneAssertions — validate.go:226
5. Update version fallback 0.13.0 → 0.21.1 — cmd/version.go:10
6. Fix MCP assertion count 29 → 40 — internal/mcp/server.go:196

## Quick Wins
1. Add non-root USER to Dockerfile
2. Add HTTP server timeouts to serve.go
3. Use subtle.ConstantTimeCompare for API key — handler.go:30
4. Rebuild issue index with all 52 issues
5. Update --format help to include gha, backstage — run.go:109
