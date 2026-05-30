---
title: "OIDC Integration for Cloud Role Assumption"
created: "2026-05-30T02:00:00-03:00"
status: APPROVED
tags: [oidc, auth, cloud, aws, gcp, azure, ci]
issue: FEAT-049
deliverables:
  - id: BR-01
    title: "AuthProvider interface and provider registry"
  - id: BR-02
    title: "AWS STS OIDC token exchange (raw HTTP)"
  - id: BR-03
    title: "GCP Workload Identity Federation token exchange (raw HTTP)"
  - id: BR-04
    title: "Azure AD OIDC token exchange (raw HTTP)"
  - id: BR-05
    title: "CI environment auto-detection (GitHub Actions, GitLab CI)"
  - id: BR-06
    title: "Schema auth: section with multi-provider config"
  - id: BR-07
    title: "Runner integration — resolve auth before test execution"
  - id: BR-08
    title: "Unit tests for token exchange, CI detection, config parsing"
---

# OIDC Integration for Cloud Role Assumption — Design Doc

## Status: APPROVED

## Problem

Smoke tests that verify cloud resources (`s3_bucket`, `url_reachable` behind auth, `k8s_resource`) need credentials. Today, users hardcode long-lived keys in CI environment variables — a security anti-pattern that violates least-privilege and can't be rotated automatically.

OIDC (OpenID Connect) lets CI runners exchange short-lived identity tokens for temporary cloud credentials. GitHub Actions, GitLab CI, and other providers all support this natively. SmokeSig should wire this transparently so tests "just work" with temporary credentials.

## Decision Record

| Question | Answer |
|----------|--------|
| Cloud providers in v1 | AWS + GCP + Azure — all three |
| Implementation approach | Raw HTTP (no cloud SDKs) — POST to STS/token endpoints |
| Build tag | None — HTTP-only, consistent with redis_ping/postgres_ping |
| Credential injection | Environment variables — assertions are unaware of OIDC |
| CI auto-detection | GitHub Actions + GitLab CI |
| Multi-provider support | Yes — `auth.providers` is a list |
| Package location | `internal/auth/` — new package |

## Architecture

### Config Schema

```yaml
auth:
  providers:
    - type: aws_oidc
      role_arn: "arn:aws:iam::123456789012:role/smoke-test-role"
      region: "us-east-1"        # optional, default us-east-1
      session_name: "smokesig"   # optional
    - type: gcp_oidc
      project_number: "123456789"
      pool_id: "my-pool"
      provider_id: "my-provider"
      service_account: "smoke@project.iam.gserviceaccount.com"
    - type: azure_oidc
      tenant_id: "xxxx-xxxx"
      client_id: "yyyy-yyyy"
      subscription_id: "zzzz-zzzz"  # optional, for env var
```

### Package Structure

```
internal/auth/
├── auth.go          # AuthProvider interface, ResolveAll(), env injection
├── aws.go           # AWS STS AssumeRoleWithWebIdentity
├── gcp.go           # GCP STS token + service account access token
├── azure.go         # Azure AD client credentials with federated token
├── ci.go            # CI environment detection (GitHub, GitLab)
├── auth_test.go     # Core tests: config parsing, env injection, CI detection
├── aws_test.go      # AWS token exchange tests (httptest server)
├── gcp_test.go      # GCP token exchange tests
├── azure_test.go    # Azure token exchange tests
```

### Core Interface

```go
type AuthProvider interface {
    Type() string
    Resolve(ctx context.Context, oidcToken string) (map[string]string, error)
}
```

`Resolve` takes an OIDC token (from CI) and returns a map of environment variable key-value pairs to inject. Each provider knows which env vars its cloud expects:

| Provider | Env Vars Set |
|----------|-------------|
| `aws_oidc` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION` |
| `gcp_oidc` | `GOOGLE_APPLICATION_CREDENTIALS` (writes temp JSON), `CLOUDSDK_CORE_PROJECT` |
| `azure_oidc` | `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_FEDERATED_TOKEN`, `AZURE_SUBSCRIPTION_ID` |

### CI Auto-Detection

```go
func DetectCIToken() (token string, provider string, err error)
```

Detection order:
1. **GitHub Actions**: `ACTIONS_ID_TOKEN_REQUEST_URL` + `ACTIONS_ID_TOKEN_REQUEST_TOKEN` → POST to request URL → returns JWT
2. **GitLab CI**: `CI_JOB_JWT_V2` env var (directly available, no HTTP needed)
3. **Explicit**: `SMOKESIG_OIDC_TOKEN` env var (user provides token directly)
4. **None**: Return empty — auth section is ignored gracefully

### Token Exchange Flows

**AWS STS** (BR-02):
```
POST https://sts.amazonaws.com/?Action=AssumeRoleWithWebIdentity
  &RoleArn=<role_arn>
  &RoleSessionName=<session_name>
  &WebIdentityToken=<oidc_token>
  &Version=2011-06-15
→ XML response with <AccessKeyId>, <SecretAccessKey>, <SessionToken>
```

**GCP Workload Identity** (BR-03):
```
Step 1: POST https://sts.googleapis.com/v1/token
  grant_type=urn:ietf:params:oauth:grant-type:token-exchange
  &subject_token=<oidc_token>
  &subject_token_type=urn:ietf:params:oauth:token-type:jwt
  &audience=//iam.googleapis.com/projects/<num>/locations/global/workloadIdentityPools/<pool>/providers/<provider>
→ JSON { "access_token": "..." }

Step 2: POST https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/<sa>:generateAccessToken
  Authorization: Bearer <step1_token>
  { "scope": ["https://www.googleapis.com/auth/cloud-platform"] }
→ JSON { "accessToken": "...", "expireTime": "..." }
```

**Azure AD** (BR-04):
```
POST https://login.microsoftonline.com/<tenant>/oauth2/v2.0/token
  client_id=<client_id>
  &client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer
  &client_assertion=<oidc_token>
  &grant_type=client_credentials
  &scope=https://management.azure.com/.default
→ JSON { "access_token": "...", "expires_in": ... }
```

### Runner Integration (BR-07)

In `runner.go`, before test execution begins (after config load, before `before_all` hooks):

```go
if cfg.Auth != nil && len(cfg.Auth.Providers) > 0 {
    envVars, err := auth.ResolveAll(ctx, cfg.Auth.Providers)
    if err != nil {
        // Non-fatal: log warning, continue without auth
        // Tests that need auth will fail on their own
    }
    for k, v := range envVars {
        os.Setenv(k, v)
    }
    defer auth.Cleanup() // Remove temp files (GCP creds JSON)
}
```

This runs once per suite, not per test. Environment variables are process-global, which is the correct behavior — cloud SDKs read them from the environment.

### Error Handling

- **No CI token available**: Skip auth silently — tests that don't need auth still run
- **Token exchange fails**: Log warning with provider type and HTTP status, continue
- **Partial success**: If 2/3 providers succeed, inject what worked, warn about failures
- **Token expired mid-run**: Not handled in v1 — temp creds last 1h minimum, smoke suites run in seconds

## Estimated Scope

| File | Lines | Description |
|------|-------|-------------|
| `internal/auth/auth.go` | ~60 | Interface, ResolveAll, env injection |
| `internal/auth/aws.go` | ~80 | STS XML parsing, HTTP call |
| `internal/auth/gcp.go` | ~90 | Two-step token exchange |
| `internal/auth/azure.go` | ~60 | Single POST token exchange |
| `internal/auth/ci.go` | ~50 | GitHub/GitLab detection |
| `internal/schema/schema.go` | ~20 | AuthConfig struct addition |
| `internal/runner/runner.go` | ~15 | ResolveAll wiring |
| Tests | ~200 | httptest-based exchange tests |
| **Total** | **~575** | |

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| AWS STS returns XML (fragile parsing) | Use `encoding/xml` with strict struct — well-defined schema |
| GCP two-step exchange is complex | Each step is a simple POST — test independently |
| CI tokens might not have required permissions | Clear error messages: "OIDC token lacks audience X for provider Y" |
| Tests can't exercise real OIDC without CI | httptest servers simulate each cloud endpoint — 100% testable locally |

## Out of Scope (v2)

- Custom OIDC providers beyond the big 3
- Credential caching across runs
- CircleCI / Bitbucket / Azure DevOps CI detection
- Per-test provider selection (all tests share the suite-level auth)
- Credential rotation during long-running suites
