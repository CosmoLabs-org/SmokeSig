---
brainstorm: docs/brainstorming/2026-05-30-oidc-cloud-role-assumption.md
created: "2026-05-30T02:15:00-03:00"
issue: FEAT-049
goals_completed: 0
goals_total: 7
related_prompts: []
requires_reading: []
schema_version: 1
status: PENDING
tags: [oidc, auth, implementation]
title: "FEAT-049: OIDC Cloud Role Assumption — Implementation Plan"
deliverables:
  - id: P-01
    title: "AuthConfig schema types + YAML parsing"
  - id: P-02
    title: "CI environment auto-detection (GitHub Actions, GitLab CI)"
  - id: P-03
    title: "AWS STS AssumeRoleWithWebIdentity exchange"
  - id: P-04
    title: "GCP Workload Identity Federation two-step exchange"
  - id: P-05
    title: "Azure AD federated token exchange"
  - id: P-06
    title: "AuthProvider interface, ResolveAll, env injection"
  - id: P-07
    title: "Runner integration — resolve auth before test execution"
---

# FEAT-049: OIDC Cloud Role Assumption — Implementation Plan

Design: `docs/brainstorming/2026-05-30-oidc-cloud-role-assumption.md`
Issue: FEAT-049

## File Structure

### New Files
| File | Purpose |
|------|---------|
| `internal/auth/auth.go` | AuthProvider interface, ResolveAll, Cleanup, env injection |
| `internal/auth/aws.go` | AWS STS token exchange |
| `internal/auth/gcp.go` | GCP two-step token exchange |
| `internal/auth/azure.go` | Azure AD token exchange |
| `internal/auth/ci.go` | CI environment auto-detection |
| `internal/auth/auth_test.go` | Core tests: registry, env injection, ResolveAll |
| `internal/auth/aws_test.go` | AWS exchange tests with httptest |
| `internal/auth/gcp_test.go` | GCP exchange tests with httptest |
| `internal/auth/azure_test.go` | Azure exchange tests with httptest |
| `internal/auth/ci_test.go` | CI detection tests |

### Modified Files
| File | Change |
|------|--------|
| `internal/schema/schema.go` | Add `AuthConfig` struct, `Auth *AuthConfig` field on SmokeConfig |
| `internal/runner/runner.go` | Call `auth.ResolveAll` before lifecycle hooks |

## Implementation Steps

### Task 1: Schema types (P-01)

**Files**: `internal/schema/schema.go`

Add after `LifecycleConfig`:

```go
type AuthConfig struct {
    Providers []AuthProviderConfig `yaml:"providers"`
}

type AuthProviderConfig struct {
    Type           string `yaml:"type"`
    RoleARN        string `yaml:"role_arn,omitempty"`
    Region         string `yaml:"region,omitempty"`
    SessionName    string `yaml:"session_name,omitempty"`
    ProjectNumber  string `yaml:"project_number,omitempty"`
    PoolID         string `yaml:"pool_id,omitempty"`
    ProviderID     string `yaml:"provider_id,omitempty"`
    ServiceAccount string `yaml:"service_account,omitempty"`
    TenantID       string `yaml:"tenant_id,omitempty"`
    ClientID       string `yaml:"client_id,omitempty"`
    SubscriptionID string `yaml:"subscription_id,omitempty"`
}
```

Add `Auth *AuthConfig \`yaml:"auth,omitempty"\`` to `SmokeConfig` struct.

Test: existing schema tests pass (`go test ./internal/schema/ -v`), config with `auth:` section parses correctly.

### Task 2: CI detection (P-02)

**Files**: `internal/auth/ci.go`, `internal/auth/ci_test.go`

```go
package auth

func DetectCIToken() (token string, provider string, err error)
```

Detection order:
1. GitHub Actions: read `ACTIONS_ID_TOKEN_REQUEST_URL` + `ACTIONS_ID_TOKEN_REQUEST_TOKEN`, POST to URL
2. GitLab CI: read `CI_JOB_JWT_V2` directly
3. Explicit: read `SMOKESIG_OIDC_TOKEN` directly
4. None: return ("", "", nil)

Tests (ci_test.go):
- `TestDetectCIToken_GitHubActions` — set env vars, mock HTTP endpoint
- `TestDetectCIToken_GitLabCI` — set `CI_JOB_JWT_V2`
- `TestDetectCIToken_Explicit` — set `SMOKESIG_OIDC_TOKEN`
- `TestDetectCIToken_NoCI` — no env vars set, returns empty

### Task 3: AWS STS exchange (P-03)

**Files**: `internal/auth/aws.go`, `internal/auth/aws_test.go`

```go
type awsOIDCProvider struct {
    RoleARN     string
    Region      string
    SessionName string
    stsURL      string // injectable for testing
}

func (p *awsOIDCProvider) Type() string { return "aws_oidc" }
func (p *awsOIDCProvider) Resolve(ctx context.Context, oidcToken string) (map[string]string, error)
```

Resolve sends POST to STS endpoint, parses XML response, returns:
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`

Tests (aws_test.go):
- `TestAWSResolve_Success` — httptest server returns valid STS XML
- `TestAWSResolve_STSError` — server returns 403 with error XML
- `TestAWSResolve_InvalidXML` — server returns garbage
- `TestAWSResolve_DefaultRegion` — empty region defaults to us-east-1
- `TestAWSResolve_DefaultSessionName` — empty session name defaults to "smokesig"

### Task 4: GCP Workload Identity exchange (P-04)

**Files**: `internal/auth/gcp.go`, `internal/auth/gcp_test.go`

```go
type gcpOIDCProvider struct {
    ProjectNumber  string
    PoolID         string
    ProviderID     string
    ServiceAccount string
    stsURL         string // injectable for testing
    iamURL         string // injectable for testing
    tempDir        string // for credentials JSON file
}

func (p *gcpOIDCProvider) Type() string { return "gcp_oidc" }
func (p *gcpOIDCProvider) Resolve(ctx context.Context, oidcToken string) (map[string]string, error)
```

Two-step: (1) exchange OIDC token for STS token, (2) exchange STS token for service account access token. Writes temp JSON creds file for `GOOGLE_APPLICATION_CREDENTIALS`.

Tests (gcp_test.go):
- `TestGCPResolve_Success` — both steps succeed
- `TestGCPResolve_STSFails` — step 1 fails
- `TestGCPResolve_IAMFails` — step 1 succeeds, step 2 fails
- `TestGCPResolve_CredsFileWritten` — verify temp JSON file contents

### Task 5: Azure AD exchange (P-05)

**Files**: `internal/auth/azure.go`, `internal/auth/azure_test.go`

```go
type azureOIDCProvider struct {
    TenantID       string
    ClientID       string
    SubscriptionID string
    loginURL       string // injectable for testing
}

func (p *azureOIDCProvider) Type() string { return "azure_oidc" }
func (p *azureOIDCProvider) Resolve(ctx context.Context, oidcToken string) (map[string]string, error)
```

Single POST to Azure AD token endpoint. Returns:
`AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_FEDERATED_TOKEN`, `AZURE_SUBSCRIPTION_ID`

Tests (azure_test.go):
- `TestAzureResolve_Success` — httptest server returns valid token
- `TestAzureResolve_AuthError` — server returns 401
- `TestAzureResolve_NoSubscription` — env var omitted when empty

### Task 6: Core auth package (P-06)

**Files**: `internal/auth/auth.go`, `internal/auth/auth_test.go`

```go
type AuthProvider interface {
    Type() string
    Resolve(ctx context.Context, oidcToken string) (map[string]string, error)
}

func NewProvider(cfg schema.AuthProviderConfig) (AuthProvider, error)
func ResolveAll(ctx context.Context, providers []schema.AuthProviderConfig) (map[string]string, error)
func Cleanup()
```

`ResolveAll`:
1. Call `DetectCIToken()` — if no token, return nil (skip gracefully)
2. For each provider config, create via `NewProvider`, call `Resolve`
3. Merge all env var maps (later providers overwrite earlier on conflict)
4. Return merged map

`Cleanup`: remove temp files (GCP creds JSON).

Tests (auth_test.go):
- `TestNewProvider_AWS` — returns awsOIDCProvider
- `TestNewProvider_GCP` — returns gcpOIDCProvider
- `TestNewProvider_Azure` — returns azureOIDCProvider
- `TestNewProvider_Unknown` — returns error
- `TestResolveAll_NoCIToken` — returns nil gracefully
- `TestResolveAll_MultiProvider` — merges env vars from 2 providers
- `TestResolveAll_PartialFailure` — one fails, others succeed, warns

### Task 7: Runner integration (P-07)

**Files**: `internal/runner/runner.go`

In `Run()`, after `r.Vars = NewVarStore()` (line 80) and before lifecycle hooks (line 83):

```go
if r.Config.Auth != nil && len(r.Config.Auth.Providers) > 0 {
    authEnv, err := auth.ResolveAll(context.Background(), r.Config.Auth.Providers)
    if err != nil {
        // Non-fatal: warn and continue
    }
    for k, v := range authEnv {
        os.Setenv(k, v)
    }
    defer auth.Cleanup()
}
```

Test: Add `TestRun_WithAuth_NoCI` to `runner_test.go` — config has auth section but no CI env, auth is skipped gracefully, tests still run.

## Execution Model

Tasks 1-5 are mostly independent (separate files, separate cloud providers). Task 6 depends on 2-5 (uses provider types). Task 7 depends on 1 and 6.

**Recommended dispatch**: 3 waves:
- Wave 1: [Task 1: schema] [Task 2: CI detection]
- Wave 2: [Task 3: AWS] [Task 4: GCP] [Task 5: Azure] (parallel, disjoint files)
- Wave 3: [Task 6: core auth] [Task 7: runner integration] (sequential)

## Verify

```bash
go build ./...
go test ./internal/auth/ -v
go test ./internal/schema/ -v
go test ./internal/runner/ -run TestRun -v
```
