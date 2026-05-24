# SmokeSig v0.21.1 — Comprehensive Audit

**Date**: 2026-05-24 | **Model**: Opus 4.6 (1M) | **Agents**: 10 | **Mode**: Fresh

## Scorecard

| Area | Score | Grade | Priority |
|------|-------|-------|----------|
| Competitive | 83/100 | B+ | Maintain |
| Code Quality | 80/100 | B | Maintain |
| Frontend/UX | 78/100 | B | Medium |
| Data/Schema | 77/100 | B | Medium |
| Core Logic | 72/100 | B- | Medium |
| Roadmap Health | 63/100 | C | High |
| Documentation | 60/100 | C- | High |
| Infrastructure | 58/100 | C- | High |
| SEO/Content | 55/100 | C- | High |
| Distribution | 35/100 | D | **CRITICAL** |
| **Overall** | **66.1/100** | **C+** | **High** |

## Top 5 Strengths

| # | Strength | Score | Source |
|---|----------|-------|--------|
| 1 | 39 native wire-protocol assertion types with zero external deps | 92 | agent-4-competitive.md |
| 2 | MCP Server for AI integration — 7 tools, unique in the space | 92 | agent-4-competitive.md |
| 3 | 7 pluggable output formats (terminal, JSON, JUnit, TAP, Prometheus, GHA, Backstage) | 85 | agent-3-frontend-ux.md |
| 4 | 1,045 tests all passing with strong coverage (baseline 94%, reporter 92%) | 81 | agent-1-code-quality.md |
| 5 | 31 project type auto-detection with tailored config generation | 85 | agent-4-competitive.md |

## Critical Bugs

| # | Bug | File:Line | Severity | Source |
|---|-----|-----------|----------|--------|
| 1 | GoReleaser/Dockerfile/CI use stale `cosmo-smoke` module path — entire release pipeline broken | .goreleaser.yml:18, Dockerfile:7, smoke.yml:55 | CRITICAL | agent-5, agent-9 |
| 2 | Docker Compose `--compose-file` silently ignored (`append` no-op) | assertion_docker.go:65 | CRITICAL | agent-2 |
| 3 | SMTP double-handshake — manual greeting read + smtp.NewClient both consume greeting | assertion_smtp.go:38-62 | CRITICAL | agent-2 |
| 4 | DeepLink missing from `hasStandaloneAssertions` — tests with only deep_link: incorrectly fail validation | validate.go:226 | CRITICAL | agent-8 |
| 5 | Race condition on `r.lifecycleEnv` in parallel mode | runner.go:310 | HIGH | agent-1, agent-2 |
| 6 | Container runs as root — no USER directive | Dockerfile:13 | HIGH | agent-9 |
| 7 | API key compared with `==` — timing side-channel | handler.go:30 | HIGH | agent-9 |
| 8 | HTTP server has no timeouts — vulnerable to slowloris | serve.go:155 | HIGH | agent-9 |
| 9 | Issue index only contains 12 of 52 issues | issues/index.yaml | HIGH | agent-12 |
| 10 | Push/OTel reporters silently swallow errors — data loss | push.go:83, otel.go:76 | HIGH | agent-3 |

## Top 5 Weaknesses

| # | Weakness | Score | Source |
|---|----------|-------|--------|
| 1 | Incomplete rename: 60+ stale `cosmo-smoke` refs across release pipeline, docs, examples | 35 | agent-5, agent-6, agent-10 |
| 2 | SPEC.md documents 5 of 40 assertion types; zero MCP/Dashboard API docs | 35 | agent-10 |
| 3 | No release workflow, no pre-built binaries for 18/20 versions | 15 | agent-5 |
| 4 | No content strategy — zero blog posts, tutorials, comparison pages | 35 | agent-6 |
| 5 | HTTP server security: no timeouts, root container, timing-vulnerable auth | 52 | agent-9 |

## Cross-Agent Patterns

**Systemic Issue #1: Incomplete Rename (7 agents flagged)**
Agents 1, 2, 3, 5, 6, 9, 10 all found stale `cosmo-smoke` references. The rename touched Go source (complete) but missed: `.goreleaser.yml`, `Dockerfile`, `.github/workflows/smoke.yml`, `SPEC.md`, `STABILITY.md`, `FEATURES.md`, `examples/`, and `cmd/version.go` fallback.

**Systemic Issue #2: Shared Mutable State (3 agents flagged)**
Agents 1, 2, 8 found `r.lifecycleEnv` and `backgroundProcesses` have no synchronization. `CheckHTTPWithTrace` mutates shared config headers.

**Systemic Issue #3: IPv6 Broken Across 8 Assertions (3 agents flagged)**
Agents 1, 2, 8 found `fmt.Sprintf("%s:%d")` produces unparseable IPv6 addresses in Redis, Postgres, MySQL, Memcached, LDAP, MongoDB, port_listening, SMTP.

**Systemic Issue #4: Silent Data Loss in Reporters (2 agents flagged)**
Agents 3, 9 found push reporter and OTel reporter silently swallow errors with no warning.

## Phased Upgrade Plan

### Phase 0: Critical Fixes (1-2 days)
1. Complete the `cosmo-smoke` → `SmokeSig` rename across all files
2. Fix Docker Compose `append` no-op (assertion_docker.go:65)
3. Fix SMTP double-handshake (assertion_smtp.go:38)
4. Add DeepLink to `hasStandaloneAssertions` (validate.go:226)
5. Update version fallback from 0.13.0 to 0.21.1 (version.go:10)
6. Fix MCP assertion count from 29 to 40 (server.go:196)

### Phase 1: Foundation (3-5 days)
1. Add release workflow (.github/workflows/release.yml) for GoReleaser
2. Add non-root USER to Dockerfile
3. Add HTTP server timeouts (serve.go)
4. Use `crypto/subtle.ConstantTimeCompare` for API key (handler.go)
5. Add mutex to `lifecycleEnv` and `backgroundProcesses`
6. Fix IPv6 with `net.JoinHostPort` across 8 assertions
7. Rebuild issue index with all 52 issues

### Phase 2: Quality (1-2 weeks)
1. Rewrite SPEC.md from scratch (document all 40 assertions)
2. Add missing 12 types to ExportSchema()
3. Add validation for 15 unvalidated assertion types
4. Add `--verbose`/`--quiet` flags
5. Add warning on push/OTel reporter failures
6. Clean CHANGELOG corruption (8 entries)
7. Update FEATURES.md to v0.21.1

### Phase 3: Growth (2-4 weeks)
1. Add terminal screenshot/GIF to README
2. Write "Why SmokeSig?" section + comparison pages
3. Document MCP server, Dashboard API, lifecycle hooks
4. Publish Docker images and create install script
5. Add golangci-lint and security scanning to CI
6. Create matrix testing (macOS, Windows)

## Metrics

| Metric | Value |
|--------|-------|
| Go lines | 35,089 |
| Test count | 1,045 |
| Code files | 178 |
| Assertion types | 39-40 |
| Output formats | 7 |
| Project detectors | 31 |
| Go vet errors | 9 |
| TODOs | 11 |
| Open issues | 7 |
| Roadmap items | 86 |
| Commit quality | 100/100 |

## Files

### Agent Reports
- [agent-1-code-quality.md](agent-1-code-quality.md)
- [agent-2-core-logic.md](agent-2-core-logic.md)
- [agent-3-frontend-ux.md](agent-3-frontend-ux.md)
- [agent-4-competitive.md](agent-4-competitive.md)
- [agent-5-distribution.md](agent-5-distribution.md)
- [agent-6-seo-content.md](agent-6-seo-content.md)
- [agent-8-data-schema.md](agent-8-data-schema.md)
- [agent-9-infrastructure.md](agent-9-infrastructure.md)
- [agent-10-documentation.md](agent-10-documentation.md)
- [agent-12-roadmap-health.md](agent-12-roadmap-health.md)

### Synthesis
- [architecture.md](architecture.md)
- [patterns.md](patterns.md)
- [risk-map.md](risk-map.md)
- [upgrade-plan.md](upgrade-plan.md)
- [brief.md](brief.md)
- [scorecard.json](scorecard.json)
