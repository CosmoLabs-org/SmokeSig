---
brainstorm_ref: docs/brainstorming/2026-05-29-oidc-cloud-integration.md
branch: master
completed: "2026-05-30"
covers_brainstorm_deliverables:
    - BR-01
    - BR-02
    - BR-03
    - BR-04
    - BR-05
    - BR-06
    - BR-07
    - BR-08
    - BR-09
    - BR-10
covers_plan_deliverables:
    - P-01
    - P-02
    - P-03
    - P-04
    - P-05
    - P-06
    - P-07
    - P-08
    - P-09
    - P-10
    - P-11
    - P-12
    - P-13
    - P-14
    - P-15
    - P-16
    - P-17
created: "2026-05-29T12:00:00-03:00"
goals_completed: 17
goals_total: 17
id: P-2026-05-29-oidc-cloud-integration
plan_ref: docs/planning-mode/2026-05-29-oidc-cloud-integration.md
priority: medium
related_prompts: []
requires_reading:
    - docs/brainstorming/2026-05-29-oidc-cloud-integration.md
    - docs/planning-mode/2026-05-29-oidc-cloud-integration.md
schema_version: 1
status: COMPLETED
tags: []
title: 'FEAT-049: OIDC Cloud Integration'
---

# FEAT-049: OIDC Cloud Integration

## BEFORE Starting — Required Reading

**You MUST read these files in full before writing any code. They are the gospel truth of what must be implemented.**

Read in order:

1. **`docs/brainstorming/2026-05-29-oidc-cloud-integration.md`** — design rationale and decision history.
2. **`docs/planning-mode/2026-05-29-oidc-cloud-integration.md`** — implementation plan with deliverables and file scope.

Loading is enforced by `ccs prompts load-context` — the command will error if any file is missing.

## Context

Add OIDC-based cloud role assumption for CI smoke tests. Exchange CI provider OIDC tokens for temporary AWS/GCP credentials via raw HTTP (no cloud SDKs). Auto-detect GitHub Actions, GitLab CI, CircleCI. Always compiled (pure stdlib, no build tag). v1 limitation: credentials only benefit `run:` commands and `k8s_resource`; standalone assertions like `s3_bucket` remain anonymous (SigV4 is v2). ~1,115 LOC total.

## Execution Strategy

Sequential with natural grouping:

- **Foundation** (sequential): G-01 → G-02 → G-03 → G-04
- **Providers** (parallel after G-04): G-05 + G-06
- **Integration** (sequential after providers): G-07 → G-08 → G-09 → G-10 → G-11
- **Edge cases** (after integration): G-12 → G-13 → remaining goals

## Goals

### [x] G-01 AuthConfig and AuthProfile schema types on SmokeConfig and Test
Covers P-01.

### [x] G-02 Auth config validation rules in validate.go
Covers P-02.

### [x] G-03 internal/auth package: interfaces, Credentials, zero-on-close
Covers P-03.

### [x] G-04 CI environment auto-detection (GitHub Actions, GitLab CI, CircleCI)
Covers P-04.

### [x] G-05 AWS STS AssumeRoleWithWebIdentity via raw HTTP (encoding/xml)
Covers P-05.

### [x] G-06 GCP STS two-step token exchange via raw HTTP (encoding/json)
Covers P-06.

### [x] G-07 Token clock skew validation (local exp check before STS call)
Covers P-07.

### [x] G-08 Credential injection into runner: env vars for run commands, AuthContext for assertions
Covers P-08.

### [x] G-09 Per-test auth profile override (test.auth field)
Covers P-09.

### [x] G-10 Watch mode TTL-aware re-exchange
Covers P-10.

### [x] G-11 Credential masking in reporter output
Covers P-11.

### [x] G-12 Monorepo auth inheritance (root auth inherited, sub-config overrides)
Covers P-12.

### [x] G-13 Auth failure as synthetic prereq failure (error propagation to reporters)
Covers P-13.

### [x] G-14 smokesig schema output updated with auth types
Covers P-14.

### [x] G-15 Unit tests: CI detection, AWS/GCP parsing, validation, masking, zeroing
Covers P-15.

### [x] G-16 Integration tests: mock STS httptest servers, full exchange flow
Covers P-16.

### [x] G-17 Documentation: CLAUDE.md assertion table, YAML examples, limitations
Covers P-17.

## Related

- Brainstorm: `docs/brainstorming/2026-05-29-oidc-cloud-integration.md`
- Plan: `docs/planning-mode/2026-05-29-oidc-cloud-integration.md`
