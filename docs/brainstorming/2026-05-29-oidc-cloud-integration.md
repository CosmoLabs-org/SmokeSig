---
date: "2026-05-29T00:00:00-03:00"
source: FEAT-049 dedicated brainstorm
status: brainstorm
issue: FEAT-049
related:
  - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
deliverables:
  - id: BR-01
    title: "AuthConfig schema and provider model (SmokeConfig.Auth)"
  - id: BR-02
    title: "CI environment auto-detection (GitHub Actions, GitLab CI, CircleCI)"
  - id: BR-03
    title: "AWS STS token exchange via raw HTTP (no SDK)"
  - id: BR-04
    title: "GCP STS token exchange via raw HTTP (no SDK)"
  - id: BR-05
    title: "Credential injection into process environment and assertion context"
  - id: BR-06
    title: "Always-compiled OIDC (no build tag — pure stdlib)"
  - id: BR-07
    title: "Credential caching with TTL-aware refresh"
  - id: BR-08
    title: "Security controls: masking, no-log, memory zeroing"
  - id: BR-09
    title: "Testing strategy: mock STS endpoint, CI detection fakes"
  - id: BR-10
    title: "Validation and error reporting for auth config"
---

# FEAT-049: OIDC Integration for Cloud Role Assumption

## Problem

SmokeSig has several assertion types that touch cloud resources: `s3_bucket` (S3/MinIO bucket accessibility), `k8s_resource` (Kubernetes resource state via kubectl), `http` (authenticated cloud API endpoints), `url_reachable` and `service_reachable` (cloud service connectivity). Today, these assertions require pre-configured credentials in the environment (e.g., `AWS_ACCESS_KEY_ID`, `KUBECONFIG`). This creates two problems:

1. **Security anti-pattern.** Long-lived credentials stored as CI secrets are a lateral-movement risk. If any CI secret leaks, the blast radius is the full permissions of those credentials. OIDC federation limits credentials to short-lived tokens scoped to a single job run.

2. **Operational friction.** Every project that needs cloud smoke tests must manually configure cloud credentials in CI, rotate them, and ensure they reach the right tests. OIDC eliminates credential provisioning entirely -- the CI provider IS the identity.

The gap was first identified in BR-04 of the Gemini ecosystem feedback analysis (2026-04-29). That note sketched the idea at ~5 lines. This document expands it into a full design.

## Design Decisions

### Decision 1: Config-Level vs Per-Test Auth

**Options:**

A. **Global `auth:` section** -- all tests inherit credentials. Simple. Matches how `otel:` works today.

B. **Per-test `auth:` field** -- each test references a named auth profile. Flexible. Allows different roles per test.

C. **Hybrid** -- global `auth:` defines profiles, tests optionally override with `auth: profile-name`.

**Decision: Option C (Hybrid).** The global section defines named auth profiles. If only one profile exists, all cloud-touching assertions use it implicitly. If multiple profiles exist, tests reference them by name. This mirrors how real-world CI works: most projects have one AWS account, but some have prod + staging.

Config-level auth also means credentials are exchanged once at suite startup (in the `before_all` lifecycle phase semantically), not per-test. This is important because STS token exchange adds 200-500ms latency, and doing it per-test in a 50-test suite would add 10-25s.

### Decision 2: Provider Support (v1 Scope)

**v1: AWS STS + GCP Workload Identity Federation.** These cover the vast majority of CI-to-cloud use cases. Azure Managed Identity is architecturally different (it's VM-attached, not token-exchange) and can come in v2.

| Provider | Token Source | Exchange Endpoint | Result |
|----------|-------------|-------------------|--------|
| AWS | CI OIDC token | `sts.amazonaws.com` `AssumeRoleWithWebIdentity` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` |
| GCP | CI OIDC token | `sts.googleapis.com` token exchange + `iamcredentials.googleapis.com` | `CLOUDSDK_AUTH_ACCESS_TOKEN` (default) or `GOOGLE_APPLICATION_CREDENTIALS` (temp keyfile, opt-in via `gcp_credential_format: keyfile`) |

### Decision 3: No Cloud SDK Dependencies

**Critical decision.** SmokeSig's minimal-dep philosophy prohibits pulling in the AWS SDK (~50MB compiled) or Google Cloud SDK (~30MB). Both STS endpoints are simple REST APIs. We implement token exchange with `net/http` and `encoding/json` only.

The gRPC precedent is instructive: `grpc_health` requires the `google.golang.org/grpc` dependency and is gated behind `-tags grpc`. OIDC should follow the same pattern but can avoid heavy deps entirely by using raw HTTP.

**AWS STS** is a query-string-based API (not even JSON request bodies). The response is XML, but we only need 3 fields (`AccessKeyId`, `SecretAccessKey`, `SessionToken`). A focused XML parse or even regex extraction is sufficient.

**GCP STS** is a JSON REST API. Two calls: (1) `POST https://sts.googleapis.com/v1/token` to exchange the OIDC token for a federated access token, (2) `POST https://iamcredentials.googleapis.com/v1/.../:generateAccessToken` to get a service account access token. Both are simple JSON request/response.

### Decision 4: CI Auto-Detection Strategy

The token exchange requires an OIDC token from the CI provider. Each CI system exposes this differently:

| CI Provider | Detection Env Var | Token Acquisition |
|-------------|-------------------|-------------------|
| **GitHub Actions** | `ACTIONS_ID_TOKEN_REQUEST_URL` | `GET $ACTIONS_ID_TOKEN_REQUEST_URL&audience=X` with `Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN` |
| **GitLab CI** | `CI_JOB_JWT_V2` | Value IS the token (env var contains the JWT directly) |
| **CircleCI** | `CIRCLE_OIDC_TOKEN_V2` | Value IS the token |

Auto-detection order: check env vars in sequence. First match wins. If none match, check if `auth.token_env` is explicitly configured (user provides their own OIDC token source). If nothing works, fail with a clear error listing what was tried.

### Decision 5: Credential Injection Method

**Options:**

A. **Set `os.Setenv()`** -- global process environment. Simple but leaks to all tests including non-cloud ones.

B. **Pass via `cmd.Env`** -- per-test command environment. Scoped but doesn't help standalone assertions (no `run:` command).

C. **Inject into assertion functions** -- pass credentials as a context/parameter to `CheckS3Bucket`, `CheckHTTP`, etc.

**Decision: Option A + C combined.** Set env vars for tests with `run:` commands (they already inherit `os.Environ()` at line 421 of `runner.go`). For standalone assertions, pass credentials via an `AuthContext` parameter that assertion functions can optionally use.

The env vars are set during suite setup (after prereqs, before tests) and cleared during suite teardown. This matches how lifecycle `env_pass` works today -- the runner already has an `r.lifecycleEnv` map that gets merged into the process environment.

### v1 Limitation: Standalone Assertions

v1 OIDC only injects environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` (AWS), and `CLOUDSDK_AUTH_ACCESS_TOKEN` or `GOOGLE_APPLICATION_CREDENTIALS` (GCP). This benefits two categories of assertions:

1. **`run:` commands** -- shell commands that invoke CLIs or SDKs (e.g., `aws s3 ls`, `gcloud storage ls`). These tools read credentials from standard env vars automatically.
2. **`k8s_resource`** -- shells out to `kubectl`, which respects `AWS_*` env vars when using EKS, and GCP env vars when using GKE.

**Standalone HTTP-based assertions like `CheckS3Bucket` cannot consume AWS credentials.** These assertions use raw `net/http` to connect to cloud endpoints. AWS requires all authenticated API requests to be signed with [AWS Signature Version 4 (SigV4)](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_aws-signing.html) -- a complex request-signing protocol that involves canonical request construction, string-to-sign derivation, and HMAC-SHA256 signing key computation. Simply setting `AWS_ACCESS_KEY_ID` in the environment does nothing for raw HTTP calls; the request itself must carry a valid `Authorization` header with the SigV4 signature.

Implementing SigV4 in SmokeSig would add ~300-400 LOC of cryptographic signing logic. This is deferred to v2. In v1, users who need authenticated S3 access should use a `run:` command with the AWS CLI instead of the standalone `s3_bucket` assertion.

GCP standalone assertions are less affected because GCP APIs accept a simple `Authorization: Bearer <token>` header, which the `AuthContext` parameter can provide without additional signing logic.

### Decision 6: Credential Caching and TTL

AWS STS temporary credentials have a configurable duration (default 1h, min 15min, max 12h). GCP access tokens last 1h. For a typical smoke test suite that runs in 30s-5min, a single exchange at suite startup is sufficient.

**Strategy:** Exchange once during suite init. Store credentials in memory. No disk caching (security). If the suite runs longer than the TTL (unusual but possible in watch mode), re-exchange on the first assertion failure that looks like a 403/401.

For `--watch` mode specifically: re-exchange on each watch cycle if the cached credentials are within 5 minutes of expiry. This avoids mid-suite credential expiration.

### Decision 7: Fallback Behavior

**If OIDC exchange fails:**

1. If `auth.fallback: env` is set, fall back to whatever credentials are already in the environment (e.g., `AWS_ACCESS_KEY_ID` from CI secrets). Log a warning.
2. If `auth.fallback: fail` (default), fail the suite with a clear error. This is the safe default -- if you configured OIDC, you expect it to work.
3. If no `auth:` section exists at all, behavior is unchanged from today (tests use whatever env provides).

### Decision 8: No Build Tag — Always Compiled

**Decision: DROP the `-tags oidc` build tag entirely.** OIDC code is always compiled into the binary. No stub, no build tag gating.

**Rationale:** The gRPC precedent does NOT apply here. The gRPC build tag exists because `google.golang.org/grpc` adds ~50MB of compiled binary size and pulls in a massive dependency tree. OIDC adds ~500 LOC of pure stdlib code using only `net/http`, `encoding/json`, and `encoding/xml`. The binary size impact is negligible.

`encoding/xml` is new to SmokeSig's import graph, but it is stdlib and trivial — it adds no external dependencies and minimal binary size. The entire OIDC module is equivalent to adding another assertion type in terms of dependency footprint.

Build tags add real costs: users must remember the flag, CI configs must include it, `go test ./...` misses tagged files by default (requiring `-tags oidc` there too), and the stub/real split doubles the surface area for interface drift bugs. These costs are justified when gating ~50MB of external deps. They are not justified for 500 lines of stdlib code.

**File layout:**
```
internal/auth/
├── auth.go           # AuthProvider interface, AuthContext, shared types
├── provider.go       # Exchange() implementation, caching, TTL check
├── aws.go            # AWS STS exchange
├── gcp.go            # GCP STS exchange
├── detect.go         # CI environment detection
├── detect_test.go
├── aws_test.go
├── gcp_test.go
└── auth_test.go
```

## Architecture

### Config Schema

New top-level `auth:` field on `SmokeConfig`:

```go
// AuthConfig defines OIDC-based cloud authentication for smoke tests.
type AuthConfig struct {
    Profiles []AuthProfile `yaml:"profiles"`
    Fallback string        `yaml:"fallback,omitempty"` // "env" | "fail" (default: "fail")
}

// AuthProfile defines a single cloud auth provider configuration.
type AuthProfile struct {
    Name     string `yaml:"name"`               // profile name (default: "default")
    Provider string `yaml:"provider"`           // "aws" | "gcp"
    RoleARN  string `yaml:"role_arn,omitempty"`  // AWS: arn:aws:iam::ACCOUNT:role/ROLE
    Audience string `yaml:"audience,omitempty"`  // OIDC audience (AWS default: "sts.amazonaws.com")
    Region   string `yaml:"region,omitempty"`    // AWS region for STS endpoint

    // GCP-specific
    WorkloadIdentityProvider string `yaml:"workload_identity_provider,omitempty"` // projects/NUM/locations/global/workloadIdentityPools/POOL/providers/PROV
    ServiceAccountEmail      string `yaml:"service_account_email,omitempty"`       // sa@project.iam.gserviceaccount.com
    GCPCredentialFormat      string `yaml:"gcp_credential_format,omitempty"`       // "token" (default: CLOUDSDK_AUTH_ACCESS_TOKEN) | "keyfile" (temp GOOGLE_APPLICATION_CREDENTIALS)

    // Advanced
    TokenEnv        string `yaml:"token_env,omitempty"`        // custom env var containing OIDC token (overrides auto-detect)
    SessionDuration string `yaml:"session_duration,omitempty"` // AWS: "1h" (default), GCP: ignored
}
```

### YAML Config Examples

**Minimal AWS (GitHub Actions):**
```yaml
version: 1
project: my-service
auth:
  profiles:
    - provider: aws
      role_arn: arn:aws:iam::123456789012:role/smoke-test-role
tests:
  - name: S3 bucket accessible
    expect:
      s3_bucket:
        bucket: my-service-assets
        region: us-east-1
```

**Multi-profile (staging + prod):**
```yaml
auth:
  profiles:
    - name: staging
      provider: aws
      role_arn: arn:aws:iam::111111111111:role/smoke-staging
    - name: prod
      provider: aws
      role_arn: arn:aws:iam::222222222222:role/smoke-prod
  fallback: env
tests:
  - name: Staging S3
    auth: staging
    expect:
      s3_bucket:
        bucket: staging-assets
  - name: Prod S3
    auth: prod
    expect:
      s3_bucket:
        bucket: prod-assets
```

**GCP with GitLab CI:**
```yaml
auth:
  profiles:
    - provider: gcp
      workload_identity_provider: projects/123456/locations/global/workloadIdentityPools/ci-pool/providers/gitlab
      service_account_email: smoke-tests@myproject.iam.gserviceaccount.com
tests:
  - name: GCS bucket accessible
    expect:
      http:
        url: https://storage.googleapis.com/storage/v1/b/my-bucket
        status_code: 200
```

### Token Exchange Flow

```
┌──────────────┐     ┌─────────────────┐     ┌──────────────┐     ┌──────────────┐
│  CI Provider │     │    SmokeSig     │     │  Cloud STS   │     │ Cloud Assert │
│ (GH/GL/CC)   │     │   auth module   │     │  endpoint    │     │  (S3, K8s)   │
└──────┬───────┘     └────────┬────────┘     └──────┬───────┘     └──────┬───────┘
       │                      │                      │                    │
       │  1. OIDC token       │                      │                    │
       │  (env var or HTTP)   │                      │                    │
       │─────────────────────>│                      │                    │
       │                      │                      │                    │
       │                      │  2. POST token       │                    │
       │                      │  + role ARN          │                    │
       │                      │─────────────────────>│                    │
       │                      │                      │                    │
       │                      │  3. Temporary creds  │                    │
       │                      │  (15min-12h TTL)     │                    │
       │                      │<─────────────────────│                    │
       │                      │                      │                    │
       │                      │  4. Set env vars     │                    │
       │                      │  + AuthContext       │                    │
       │                      │──────────────────────────────────────────>│
       │                      │                      │                    │
       │                      │  5. Assertion result │                    │
       │                      │<─────────────────────────────────────────│
```

### AWS STS Exchange (Raw HTTP)

The `AssumeRoleWithWebIdentity` API is a simple query-string POST:

```
POST https://sts.amazonaws.com/ HTTP/1.1
Content-Type: application/x-www-form-urlencoded

Action=AssumeRoleWithWebIdentity
&RoleArn=arn:aws:iam::123456789012:role/smoke-test-role
&RoleSessionName=smokesig-TIMESTAMP
&WebIdentityToken=eyJhbGciOi...
&DurationSeconds=3600
&Version=2011-06-15
```

Response is XML. We parse three fields:
```xml
<AssumeRoleWithWebIdentityResponse>
  <AssumeRoleWithWebIdentityResult>
    <Credentials>
      <AccessKeyId>ASIAXXX</AccessKeyId>
      <SecretAccessKey>xxx</SecretAccessKey>
      <SessionToken>xxx</SessionToken>
      <Expiration>2026-05-29T01:00:00Z</Expiration>
    </Credentials>
  </AssumeRoleWithWebIdentityResult>
</AssumeRoleWithWebIdentityResponse>
```

We use `encoding/xml` (stdlib) for parsing. No AWS SDK required.

### GCP STS Exchange (Raw HTTP)

Two-step exchange:

**Step 1: Federate OIDC token for STS token**
```
POST https://sts.googleapis.com/v1/token HTTP/1.1
Content-Type: application/json

{
  "grantType": "urn:ietf:params:oauth:grant-type:token-exchange",
  "audience": "//iam.googleapis.com/projects/NUM/locations/global/workloadIdentityPools/POOL/providers/PROV",
  "scope": "https://www.googleapis.com/auth/cloud-platform",
  "requestedTokenType": "urn:ietf:params:oauth:token-type:access_token",
  "subjectTokenType": "urn:ietf:params:oauth:token-type:jwt",
  "subjectToken": "eyJhbGciOi..."
}
```

**Step 2: Exchange STS token for service account access token**
```
POST https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SA_EMAIL:generateAccessToken HTTP/1.1
Authorization: Bearer STS_TOKEN
Content-Type: application/json

{
  "scope": ["https://www.googleapis.com/auth/cloud-platform"],
  "lifetime": "3600s"
}
```

Response: `{"accessToken": "ya29.xxx", "expireTime": "2026-05-29T01:00:00Z"}`

For GCP, the default injection method is the `CLOUDSDK_AUTH_ACCESS_TOKEN` environment variable, which passes the access token directly without writing any file to disk. This eliminates crash-path credential leaks (no temp file to orphan) and avoids concurrency issues when parallel tests share the filesystem.

Fall back to writing a temporary JSON keyfile (setting `GOOGLE_APPLICATION_CREDENTIALS`) only when the user explicitly sets `gcp_credential_format: keyfile` in their auth profile. Some GCP client libraries require a keyfile and cannot consume `CLOUDSDK_AUTH_ACCESS_TOKEN`. When keyfile mode is used:
- The temp file is created with `0600` permissions in `os.TempDir()`
- A `defer` removes it during normal suite teardown
- A `signal.Notify` handler for `SIGTERM` and `SIGINT` is registered to clean up on abnormal termination
- **Accepted risk:** `SIGKILL` (kill -9) bypasses all signal handlers and `defer` statements -- the temp keyfile will be orphaned. This is documented but unavoidable without an external cleanup daemon, which is out of scope

### Integration Points in Runner

**SmokeConfig schema** (`internal/schema/schema.go`):
```go
type SmokeConfig struct {
    // ... existing fields ...
    Auth  AuthConfig `yaml:"auth,omitempty"`  // NEW
}

type Test struct {
    // ... existing fields ...
    Auth string `yaml:"auth,omitempty"` // NEW: profile name override
}
```

**Runner init** (`internal/runner/runner.go` in `Run()`):
- After prerequisites, before test execution
- Call `auth.Exchange(config.Auth)` to perform OIDC token exchange
- Store `*auth.Credentials` on `Runner` struct
- Set env vars (`os.Setenv`) for the suite duration
- Defer credential cleanup (env unset, memory zero; temp keyfile delete + SIGTERM/SIGINT handler if `gcp_credential_format: keyfile`)

**Assertion context**: Assertions that benefit from auth (`CheckS3Bucket`, `CheckHTTP`, `CheckK8sResource`) gain an optional `*auth.Credentials` parameter. For backward compatibility, `nil` means "use environment as-is" (current behavior). Non-nil means "use these credentials." This is additive -- no existing assertion signatures break.

**Per-test auth override**: When a test has `auth: profile-name`, the runner temporarily swaps the env vars to that profile's credentials before running the test, then restores the default profile after. This reuses the same `cmd.Env` injection pattern already at line 421.

### Credential Masking

All output paths (terminal reporter, JSON reporter, JUnit, TAP, etc.) must never log:
- OIDC tokens
- AWS access keys, secret keys, session tokens
- GCP access tokens, keyfile contents
- Any `Authorization` header values

The `***redacted***` pattern from `CheckCredential` (assertion_credential.go) is the existing precedent. The auth module uses the same approach:

1. `Credentials.String()` returns `"aws:ASIA***"` (first 4 chars only)
2. Auth exchange errors redact token values before wrapping: `"STS error: HTTP 403 (token: ***redacted***)"` 
3. Reporter integration: if a test's `Error` field contains any known credential prefix (`AKIA`, `ASIA`, `ya29.`, `eyJ`), replace with `***redacted***`

### Memory Security

Credentials are stored in a struct with a `Zero()` method that overwrites byte slices with zeros before GC. This is best-effort (Go's GC may have already copied the data), but it closes the most obvious window:

```go
type Credentials struct {
    Provider        string
    AccessKeyID     []byte // not string -- mutable for zeroing
    SecretAccessKey []byte
    SessionToken    []byte
    AccessToken     []byte // GCP
    Expiration      time.Time
}

func (c *Credentials) Zero() {
    for i := range c.AccessKeyID { c.AccessKeyID[i] = 0 }
    for i := range c.SecretAccessKey { c.SecretAccessKey[i] = 0 }
    for i := range c.SessionToken { c.SessionToken[i] = 0 }
    for i := range c.AccessToken { c.AccessToken[i] = 0 }
}
```

## Scope Boundaries

### In Scope (v1)

- `auth:` config section with profile model
- AWS STS `AssumeRoleWithWebIdentity` via raw HTTP
- GCP Workload Identity Federation via raw HTTP
- CI auto-detection: GitHub Actions, GitLab CI, CircleCI
- Env var injection for `run:` commands
- `AuthContext` parameter threading for standalone assertions
- Always compiled (no build tag — pure stdlib, no external deps)
- Credential caching for suite duration
- Watch mode re-exchange on TTL expiry
- Credential masking in all reporter outputs
- Validation of `auth:` config during `smokesig validate`
- `smokesig schema` output updated to include `auth` types

### Out of Scope (v2+)

- **Azure Managed Identity** -- different architecture (VM-attached, not OIDC token exchange). Needs a separate design.
- **HashiCorp Vault** integration -- useful but out-of-scope for CI-focused OIDC.
- **Custom OIDC providers** (Keycloak, Auth0) -- the `token_env` field is the escape hatch for these. Users can configure their own OIDC flow and point SmokeSig at the resulting token.
- **AWS SigV4 request signing** for standalone assertions (`s3_bucket`, `http` against AWS APIs) -- v1 injects env vars only; raw HTTP assertions cannot authenticate to AWS without SigV4. ~300-400 LOC deferred to v2.
- **Per-assertion auth** (e.g., HTTP header injection from auth profile) -- v1 uses env vars. Fine-grained assertion auth is a v2 feature.
- **Credential rotation during suite** (proactive) -- v1 only re-exchanges reactively (on failure or watch-cycle expiry check).
- **Audit logging** of credential exchange events to a file/endpoint.
- **AWS SSO / `aws sso login`** support -- that's a local dev workflow, not CI OIDC.

## Validation Rules

The `auth:` section gets its own validation pass in `internal/schema/validate.go`:

| Rule | Error |
|------|-------|
| `provider` must be `"aws"` or `"gcp"` | `auth.profiles[N]: unsupported provider %q` |
| AWS profile requires `role_arn` | `auth.profiles[N]: aws provider requires role_arn` |
| AWS `role_arn` must match `arn:aws:iam::\d+:role/.+` | `auth.profiles[N]: invalid role_arn format` |
| GCP profile requires `workload_identity_provider` | `auth.profiles[N]: gcp provider requires workload_identity_provider` |
| GCP profile requires `service_account_email` | `auth.profiles[N]: gcp provider requires service_account_email` |
| GCP `gcp_credential_format` must be `"token"` or `"keyfile"` or empty | `auth.profiles[N]: invalid gcp_credential_format %q` |
| Profile names must be unique | `auth.profiles: duplicate name %q` |
| Test `auth:` reference must match a profile name | `test %q: auth profile %q not found` |
| `fallback` must be `"env"` or `"fail"` or empty | `auth.fallback: invalid value %q` |
| `session_duration` must parse as `time.Duration` | `auth.profiles[N]: invalid session_duration %q` |
| AWS `session_duration` must be between 15m and 12h | `auth.profiles[N]: session_duration must be between 15m and 12h` |

All errors are collected and returned together (consistent with SmokeSig's "all errors at once" design).

## Testing Strategy

### Unit Tests (no cloud access)

1. **CI detection tests** (`detect_test.go`): Set env vars, verify correct CI provider and token source are detected. Test priority order. Test fallback to `token_env`.

2. **AWS STS parsing tests** (`aws_test.go`): Parse known-good XML responses. Parse error responses. Validate request construction (query params, headers).

3. **GCP STS parsing tests** (`gcp_test.go`): Parse known-good JSON responses. Validate request body construction for both exchange steps.

4. **Config validation tests** (`auth_test.go`): All validation rules from the table above. Invalid combinations. Missing fields.

5. **Credential masking tests**: Verify `Credentials.String()` redacts. Verify `Zero()` clears memory. Verify reporter output contains no credential material.

### Integration Tests (mock STS)

A `httptest.Server` that mimics the AWS STS and GCP STS endpoints:

```go
func TestAWSExchange_MockSTS(t *testing.T) {
    sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Validate request params
        assert(r.FormValue("Action") == "AssumeRoleWithWebIdentity")
        assert(r.FormValue("RoleArn") == "arn:aws:iam::123:role/test")
        // Return mock credentials
        w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>...mock creds...</...>`))
    }))
    defer sts.Close()

    creds, err := exchangeAWS(Config{STSEndpoint: sts.URL, ...}, "fake-oidc-token")
    require.NoError(t, err)
    assert.Equal(t, "ASIAMOCKKEY", string(creds.AccessKeyID))
}
```

This pattern tests the full exchange flow without touching real cloud APIs. The `STSEndpoint` override (internal, not exposed in YAML) makes the STS URL configurable for testing.

### End-to-End Tests (CI only)

Tagged with `//go:build integration`:
- GitHub Actions workflow that configures OIDC and runs `smokesig run` against a real S3 bucket
- Only runs in CI (requires cloud role trust policy)
- Not part of `go test ./...` default suite

## Estimated Scope

| File | LOC (approx) | Description |
|------|-------------|-------------|
| `internal/auth/auth.go` | ~50 | `AuthProvider` interface, `Credentials` struct, `AuthContext`, `Zero()` |
| `internal/auth/provider.go` | ~80 | `Exchange()` implementation, caching, TTL check |
| `internal/auth/detect.go` | ~60 | CI auto-detection, token acquisition (HTTP for GH Actions, env for others) |
| `internal/auth/aws.go` | ~90 | AWS STS request construction, XML response parsing, env var names |
| `internal/auth/gcp.go` | ~100 | GCP two-step exchange, JSON handling, temp keyfile write |
| `internal/auth/detect_test.go` | ~80 | CI detection with env var manipulation |
| `internal/auth/aws_test.go` | ~100 | Mock STS server, request validation, response parsing |
| `internal/auth/gcp_test.go` | ~100 | Mock STS server, two-step exchange, keyfile validation |
| `internal/auth/auth_test.go` | ~60 | Config validation, caching, TTL, fallback |
| `internal/schema/schema.go` | ~25 | `AuthConfig`, `AuthProfile` structs, `Auth` field on `SmokeConfig`/`Test` |
| `internal/schema/validate.go` | ~40 | Auth validation rules |
| `internal/runner/runner.go` | ~30 | Auth exchange call in `Run()`, env injection, cleanup defer |
| **Total** | **~815** | ~480 implementation + ~340 test |

## Open Questions

1. **Should `smokesig init` detect cloud resources and suggest `auth:` config?** The detector already identifies 31 project types. If it detects AWS CDK/Terraform/Pulumi files, it could pre-populate an `auth:` section. Low priority for v1 but worth noting.

2. **Should auth exchange happen in `prerequisites` or as its own phase?** Currently sketched as happening between prereqs and tests. Could also be a special prerequisite that runs automatically. The phase approach is cleaner because auth failure should block all tests, not just appear as a failed prereq.

3. **Regional STS endpoints.** AWS has regional STS endpoints (`sts.us-east-1.amazonaws.com`) that are faster and more reliable than the global endpoint. Should we default to regional when `region` is set? Probably yes, but adds a small amount of complexity.

4. **~~GCP credential file format.~~** Resolved: default to `CLOUDSDK_AUTH_ACCESS_TOKEN` (no temp file). Opt-in `gcp_credential_format: keyfile` for tools that require `GOOGLE_APPLICATION_CREDENTIALS`. Keyfile mode registers SIGTERM/SIGINT signal handler for cleanup; SIGKILL orphan risk is accepted and documented.

5. **Monorepo auth inheritance.** In `--monorepo` mode, should sub-configs inherit the root's `auth:` section? Consistent with how `settings:` merges work, but needs explicit design for profile name conflicts.
