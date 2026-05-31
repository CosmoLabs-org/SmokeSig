---
brainstorm_ref: docs/brainstorming/2026-05-30-oidc-cloud-role-assumption.md
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
covers_plan_deliverables:
    - P-01
    - P-02
    - P-03
    - P-04
    - P-05
    - P-06
    - P-07
created: "2026-05-30T12:00:00-03:00"
goals_completed: 7
goals_total: 7
id: P-2026-05-30-oidc-cloud-role-assumption
plan_ref: docs/planning-mode/2026-05-30-oidc-cloud-role-assumption.md
priority: medium
related_prompts: []
requires_reading:
    - docs/brainstorming/2026-05-30-oidc-cloud-role-assumption.md
    - docs/planning-mode/2026-05-30-oidc-cloud-role-assumption.md
schema_version: 1
status: COMPLETED
tags: []
title: 'FEAT-049: OIDC Cloud Role Assumption — Full Implementation'
---

# FEAT-049: OIDC Cloud Role Assumption — Full Implementation

## BEFORE Starting — Required Reading

**You MUST read these files in full before writing any code. They are the gospel truth of what must be implemented.**

Read in order:

1. **`docs/brainstorming/2026-05-30-oidc-cloud-role-assumption.md`** — design rationale and decision history.
2. **`docs/planning-mode/2026-05-30-oidc-cloud-role-assumption.md`** — implementation plan with deliverables and file scope.

Loading is enforced by `ccs prompts load-context` — the command will error if any file is missing.

## Context

Add OIDC-based cloud role assumption so smoke tests can access AWS/GCP/Azure resources using temporary credentials from CI providers. All three cloud exchanges use raw HTTP (no SDK deps), keeping SmokeSig dependency-free. New `internal/auth/` package with `auth:` config section. ~575 LOC total.

## Execution Strategy

3 waves, mix of parallel GLM and sequential:

- **Wave 1** (parallel): G-01 schema + G-02 CI detection — independent, no shared files
- **Wave 2** (parallel): G-03 AWS + G-04 GCP + G-05 Azure — each is a separate file pair
- **Wave 3** (sequential): G-06 core auth (depends on provider types) → G-07 runner wiring

## Goals

### [x] G-01 AuthConfig schema types + YAML parsing
**Model:** `glm-turbo` | **Files:** `internal/schema/schema.go`
Covers P-01.

### [x] G-02 CI environment auto-detection (GitHub Actions, GitLab CI)
**Model:** `glm-turbo` | **Files:** `internal/auth/ci.go`, `internal/auth/ci_test.go`
Covers P-02.

### [x] G-03 AWS STS AssumeRoleWithWebIdentity exchange
**Model:** `glm-turbo` | **Files:** `internal/auth/aws.go`, `internal/auth/aws_test.go`
Covers P-03.

### [x] G-04 GCP Workload Identity Federation two-step exchange
**Model:** `glm-turbo` | **Files:** `internal/auth/gcp.go`, `internal/auth/gcp_test.go`
Covers P-04.

### [x] G-05 Azure AD federated token exchange
**Model:** `glm-turbo` | **Files:** `internal/auth/azure.go`, `internal/auth/azure_test.go`
Covers P-05.

### [x] G-06 AuthProvider interface, ResolveAll, env injection
**Model:** `sonnet` | **Files:** `internal/auth/auth.go`, `internal/auth/auth_test.go`
Covers P-06. Depends on G-03, G-04, G-05 (uses provider types).

### [x] G-07 Runner integration — resolve auth before test execution
**Model:** `glm-turbo` | **Files:** `internal/runner/runner.go`
Covers P-07. Depends on G-01 and G-06.

## Related

- Brainstorm: `docs/brainstorming/2026-05-30-oidc-cloud-role-assumption.md`
- Plan: `docs/planning-mode/2026-05-30-oidc-cloud-role-assumption.md`
