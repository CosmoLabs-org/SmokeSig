---
brainstorm: docs/brainstorming/2026-05-29-oidc-cloud-integration.md
created: "2026-05-29T12:00:00-03:00"
issue: FEAT-049
status: PENDING
deliverables:
  - id: P-01
    title: "AuthConfig and AuthProfile schema types on SmokeConfig and Test"
  - id: P-02
    title: "Auth config validation rules in validate.go"
  - id: P-03
    title: "internal/auth package: interfaces, Credentials, zero-on-close"
  - id: P-04
    title: "CI environment auto-detection (GitHub Actions, GitLab CI, CircleCI)"
  - id: P-05
    title: "AWS STS AssumeRoleWithWebIdentity via raw HTTP (encoding/xml)"
  - id: P-06
    title: "GCP STS two-step token exchange via raw HTTP (encoding/json)"
  - id: P-07
    title: "Token clock skew validation (local exp check before STS call)"
  - id: P-08
    title: "Credential injection into runner: env vars for run commands, AuthContext for assertions"
  - id: P-09
    title: "Per-test auth profile override (test.auth field)"
  - id: P-10
    title: "Watch mode TTL-aware re-exchange"
  - id: P-11
    title: "Credential masking in reporter output"
  - id: P-12
    title: "Monorepo auth inheritance (root auth inherited, sub-config overrides)"
  - id: P-13
    title: "Auth failure as synthetic prereq failure (error propagation to reporters)"
  - id: P-14
    title: "smokesig schema output updated with auth types"
  - id: P-15
    title: "Unit tests: CI detection, AWS/GCP parsing, validation, masking, zeroing"
  - id: P-16
    title: "Integration tests: mock STS httptest servers, full exchange flow"
  - id: P-17
    title: "Documentation: CLAUDE.md assertion table, YAML examples, limitations"
---

# FEAT-049: OIDC Integration for Cloud Role Assumption — Implementation Plan

## Goal

Add OIDC-based cloud authentication to SmokeSig so CI pipelines can assume AWS/GCP roles via short-lived tokens instead of long-lived credentials. Config-level `auth:` section with named profiles, automatic CI token detection, raw HTTP token exchange (zero cloud SDK deps), and credential injection into test execution.

## Review Feedback Incorporated

The following reviewer feedback (score 8/10) has been incorporated into this plan:

1. **No build tag** -- OIDC uses only stdlib (`net/http`, `encoding/xml`, `encoding/json`). No `-tags oidc` gate. Always compiled. The gRPC precedent does not apply (gRPC adds ~50MB via google.golang.org/grpc; OIDC adds ~600 LOC of pure stdlib).

2. **SigV4 gap documented** -- v1 OIDC injects env vars only. `run:` commands and `k8s_resource` (shells out to kubectl) consume them. Standalone HTTP assertions like `CheckS3Bucket` use raw `net/http` without AWS Signature V4 signing -- they remain anonymous. SigV4 is a v2 item.

3. **GCP default: env var, not temp file** -- Default to `CLOUDSDK_AUTH_ACCESS_TOKEN` env var injection (no temp file, no crash-path leak). Fall back to temp keyfile only when user sets `gcp_credential_format: keyfile`. Register SIGTERM handler for keyfile cleanup.

4. **Token clock skew validation** -- Check OIDC token `exp` claim locally before making STS call. Fail fast with clear error if token is already expired or within 30s of expiry.

5. **Auth failure as synthetic prereq** -- Auth exchange errors surface as a synthetic failed prerequisite in reporter output, not as a test failure. All tests are blocked but reporters render the failure correctly.

6. **Monorepo auth inheritance** -- Root config `auth:` is inherited by sub-configs. Sub-config `auth:` overrides the root entirely (no merge). Profile name conflicts: sub-config wins.

7. **CI provider API versioning** -- Document GitHub/GitLab/CircleCI OIDC token endpoint versions as maintenance surface in code comments.

8. **Assertion signatures internal-only** -- No public API changes. `AuthContext` is passed internally within the runner package.

---

## File Layout

```
internal/auth/
├── auth.go              # AuthProvider interface, Credentials struct, AuthContext, Zero()
├── provider_aws.go      # AWS STS AssumeRoleWithWebIdentity exchange
├── provider_gcp.go      # GCP STS two-step exchange
├── detect.go            # CI environment auto-detection, OIDC token acquisition
├── exchange.go          # Orchestrator: detect CI -> acquire token -> exchange -> cache
├── masking.go           # Credential masking utilities
├── auth_test.go         # Config validation, caching, TTL, fallback, masking, zeroing
├── detect_test.go       # CI detection with env var manipulation
├── provider_aws_test.go # Mock STS server, request validation, XML response parsing
├── provider_gcp_test.go # Mock STS server, two-step exchange, env var / keyfile
```

No build tags. All files compile unconditionally.

---

## Task 1: Schema Types (P-01)

**Files:** `internal/schema/schema.go`

Add `AuthConfig` and `AuthProfile` structs, plus the `Auth` field on `SmokeConfig` and `Test`.

```go
// AuthConfig defines OIDC-based cloud authentication for smoke tests.
type AuthConfig struct {
    Profiles []AuthProfile `yaml:"profiles"`
    Fallback string        `yaml:"fallback,omitempty"` // "env" | "fail" (default: "fail")
}

// AuthProfile defines a single cloud auth provider configuration.
type AuthProfile struct {
    Name     string `yaml:"name,omitempty"`     // profile name (default: "default")
    Provider string `yaml:"provider"`           // "aws" | "gcp"
    RoleARN  string `yaml:"role_arn,omitempty"` // AWS: arn:aws:iam::ACCOUNT:role/ROLE
    Audience string `yaml:"audience,omitempty"` // OIDC audience (AWS default: "sts.amazonaws.com")
    Region   string `yaml:"region,omitempty"`   // AWS region for regional STS endpoint

    // GCP-specific
    WorkloadIdentityProvider string `yaml:"workload_identity_provider,omitempty"`
    ServiceAccountEmail      string `yaml:"service_account_email,omitempty"`
    GCPCredentialFormat      string `yaml:"gcp_credential_format,omitempty"` // "env" (default) | "keyfile"

    // Advanced
    TokenEnv        string `yaml:"token_env,omitempty"`        // custom env var containing OIDC token
    SessionDuration string `yaml:"session_duration,omitempty"` // AWS: "1h" default, must be 15m-12h
}
```

Changes to existing structs:

```go
type SmokeConfig struct {
    // ... existing fields ...
    Auth AuthConfig `yaml:"auth,omitempty"` // NEW — after OTel
}

type Test struct {
    // ... existing fields ...
    Auth string `yaml:"auth,omitempty"` // NEW — profile name override
}
```

- [ ] Add `AuthConfig`, `AuthProfile` structs to `schema.go`
- [ ] Add `Auth AuthConfig` field to `SmokeConfig` (after `OTel`)
- [ ] Add `Auth string` field to `Test` (after `Retry`)

**Test:** `go test ./internal/schema/ -run TestParse -v` -- verify YAML with `auth:` section parses correctly.

**Commit:** `feat(schema): add AuthConfig and AuthProfile types for OIDC auth (FEAT-049)`

---

## Task 2: Auth Config Validation (P-02)

**Files:** `internal/schema/validate.go`

Add validation rules for the `auth:` section. Insert after the existing `OTel` validation block (line ~197) and before notifications validation.

```go
// --- Auth validation ---
if len(cfg.Auth.Profiles) > 0 {
    if cfg.Auth.Fallback != "" && cfg.Auth.Fallback != "env" && cfg.Auth.Fallback != "fail" {
        errs = append(errs, fmt.Sprintf("auth.fallback: invalid value %q (must be env or fail)", cfg.Auth.Fallback))
    }
    profileNames := make(map[string]bool)
    for i, p := range cfg.Auth.Profiles {
        prefix := fmt.Sprintf("auth.profiles[%d]", i)
        name := p.Name
        if name == "" {
            name = "default"
        }
        if profileNames[name] {
            errs = append(errs, fmt.Sprintf("auth.profiles: duplicate name %q", name))
        }
        profileNames[name] = true

        if p.Provider != "aws" && p.Provider != "gcp" {
            errs = append(errs, fmt.Sprintf("%s: unsupported provider %q (must be aws or gcp)", prefix, p.Provider))
        }
        if p.Provider == "aws" {
            if p.RoleARN == "" {
                errs = append(errs, fmt.Sprintf("%s: aws provider requires role_arn", prefix))
            } else if matched, _ := regexp.MatchString(`^arn:aws:iam::\d+:role/.+$`, p.RoleARN); !matched {
                errs = append(errs, fmt.Sprintf("%s: invalid role_arn format (expected arn:aws:iam::ACCOUNT:role/NAME)", prefix))
            }
        }
        if p.Provider == "gcp" {
            if p.WorkloadIdentityProvider == "" {
                errs = append(errs, fmt.Sprintf("%s: gcp provider requires workload_identity_provider", prefix))
            }
            if p.ServiceAccountEmail == "" {
                errs = append(errs, fmt.Sprintf("%s: gcp provider requires service_account_email", prefix))
            }
            if p.GCPCredentialFormat != "" && p.GCPCredentialFormat != "env" && p.GCPCredentialFormat != "keyfile" {
                errs = append(errs, fmt.Sprintf("%s: gcp_credential_format must be env or keyfile", prefix))
            }
        }
        if p.SessionDuration != "" {
            d, err := time.ParseDuration(p.SessionDuration)
            if err != nil {
                errs = append(errs, fmt.Sprintf("%s: invalid session_duration %q", prefix, p.SessionDuration))
            } else if p.Provider == "aws" && (d < 15*time.Minute || d > 12*time.Hour) {
                errs = append(errs, fmt.Sprintf("%s: session_duration must be between 15m and 12h for AWS", prefix))
            }
        }
    }

    // Validate test auth profile references
    for i, t := range cfg.Tests {
        if t.Auth != "" && !profileNames[t.Auth] {
            errs = append(errs, fmt.Sprintf("tests[%d]: auth profile %q not found", i, t.Auth))
        }
    }
}
```

- [ ] Add `"time"` to imports in `validate.go` (note: `"regexp"` is already imported)
- [ ] Add auth validation block after OTel validation (line ~197), before notifications validation (line ~199)
- [ ] Add test auth profile reference validation inside the existing test loop (lines 35-191)

**Test:** `go test ./internal/schema/ -run TestValidate -v` -- add test cases for all auth validation rules.

**Commit:** `feat(schema): add auth config validation rules (FEAT-049)`

---

## Task 3: Auth Package Core (P-03)

**Files:** `internal/auth/auth.go`

The core types and interfaces for the auth module.

```go
package auth

import (
    "fmt"
    "os"
    "sync"
    "time"
)

// Provider identifies a cloud provider.
type Provider string

const (
    ProviderAWS Provider = "aws"
    ProviderGCP Provider = "gcp"
)

// Credentials holds temporary cloud credentials obtained via OIDC exchange.
// Byte slices (not strings) enable zeroing on cleanup.
type Credentials struct {
    Provider       Provider
    ProfileName    string
    AccessKeyID    []byte // AWS
    SecretAccessKey []byte // AWS
    SessionToken   []byte // AWS
    AccessToken    []byte // GCP
    Expiration     time.Time
    // GCP keyfile path (only when gcp_credential_format=keyfile)
    KeyfilePath    string
}

// Expired returns true if credentials have expired or are within the skew window.
func (c *Credentials) Expired(skew time.Duration) bool {
    if c.Expiration.IsZero() {
        return false
    }
    return time.Now().Add(skew).After(c.Expiration)
}

// Zero overwrites all credential byte slices with zeros.
// Best-effort: Go's GC may have copied the data, but this closes the obvious window.
func (c *Credentials) Zero() {
    for i := range c.AccessKeyID {
        c.AccessKeyID[i] = 0
    }
    for i := range c.SecretAccessKey {
        c.SecretAccessKey[i] = 0
    }
    for i := range c.SessionToken {
        c.SessionToken[i] = 0
    }
    for i := range c.AccessToken {
        c.AccessToken[i] = 0
    }
}

// String returns a redacted representation (first 4 chars of key ID only).
func (c *Credentials) String() string {
    switch c.Provider {
    case ProviderAWS:
        id := string(c.AccessKeyID)
        if len(id) > 4 {
            id = id[:4] + "***"
        }
        return fmt.Sprintf("aws:%s", id)
    case ProviderGCP:
        tok := string(c.AccessToken)
        if len(tok) > 4 {
            tok = tok[:4] + "***"
        }
        return fmt.Sprintf("gcp:%s", tok)
    default:
        return "unknown"
    }
}

// EnvVars returns the environment variables to set for these credentials.
func (c *Credentials) EnvVars() map[string]string {
    vars := make(map[string]string)
    switch c.Provider {
    case ProviderAWS:
        vars["AWS_ACCESS_KEY_ID"] = string(c.AccessKeyID)
        vars["AWS_SECRET_ACCESS_KEY"] = string(c.SecretAccessKey)
        vars["AWS_SESSION_TOKEN"] = string(c.SessionToken)
    case ProviderGCP:
        vars["CLOUDSDK_AUTH_ACCESS_TOKEN"] = string(c.AccessToken)
        if c.KeyfilePath != "" {
            vars["GOOGLE_APPLICATION_CREDENTIALS"] = c.KeyfilePath
        }
    }
    return vars
}

// AuthContext carries resolved credentials for the current test execution.
// Passed internally within the runner package — not a public API.
type AuthContext struct {
    mu       sync.RWMutex
    profiles map[string]*Credentials
    active   string // currently active profile name
}

// NewAuthContext creates an empty auth context.
func NewAuthContext() *AuthContext {
    return &AuthContext{
        profiles: make(map[string]*Credentials),
    }
}

// Set stores credentials for a named profile.
func (ac *AuthContext) Set(name string, creds *Credentials) {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    ac.profiles[name] = creds
}

// Get returns credentials for a named profile, or nil if not found.
func (ac *AuthContext) Get(name string) *Credentials {
    ac.mu.RLock()
    defer ac.mu.RUnlock()
    return ac.profiles[name]
}

// Active returns the currently active credentials (default profile or overridden).
func (ac *AuthContext) Active() *Credentials {
    ac.mu.RLock()
    defer ac.mu.RUnlock()
    if ac.active != "" {
        return ac.profiles[ac.active]
    }
    return ac.profiles["default"]
}

// SetActive sets the active profile name.
func (ac *AuthContext) SetActive(name string) {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    ac.active = name
}

// CleanupKeyfiles removes any GCP temp keyfiles from all profiles.
func (ac *AuthContext) CleanupKeyfiles() {
    ac.mu.RLock()
    defer ac.mu.RUnlock()
    for _, c := range ac.profiles {
        if c.KeyfilePath != "" {
            os.Remove(c.KeyfilePath)
        }
    }
}

// ZeroAll zeroes all stored credentials.
func (ac *AuthContext) ZeroAll() {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    for _, c := range ac.profiles {
        c.Zero()
    }
}
```

- [ ] Create `internal/auth/` directory
- [ ] Create `internal/auth/auth.go` with types above
- [ ] Verify: `go build ./internal/auth/`

**Test:** `go test ./internal/auth/ -run TestCredentials -v`

**Commit:** `feat(auth): add core types, Credentials, AuthContext, and zero-on-close (FEAT-049)`

---

## Task 4: CI Environment Auto-Detection (P-04)

**Files:** `internal/auth/detect.go`

Detect CI environment and acquire OIDC token. Each CI provider exposes tokens differently.

```go
package auth

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"
)

// CIProvider identifies a CI system.
type CIProvider string

const (
    CIGitHubActions CIProvider = "github-actions"
    CIGitLabCI      CIProvider = "gitlab-ci"
    CICircleCI      CIProvider = "circleci"
    CICustom        CIProvider = "custom"
    CIUnknown       CIProvider = "unknown"
)

// DetectResult holds the detected CI environment and token acquisition method.
type DetectResult struct {
    Provider CIProvider
    Token    string
}

// DetectCI identifies the CI provider and acquires an OIDC token.
// Detection order: GitHub Actions > GitLab CI > CircleCI > custom token_env.
// Returns error if no CI environment is detected and no token_env fallback is configured.
//
// CI provider API versions (maintenance surface):
//   - GitHub Actions: OIDC token endpoint v1 (ACTIONS_ID_TOKEN_REQUEST_URL)
//   - GitLab CI: CI_JOB_JWT_V2 (JWT v2 format, introduced GitLab 15.7)
//   - CircleCI: CIRCLE_OIDC_TOKEN_V2 (OIDC v2 format)
func DetectCI(tokenEnv string, audience string) (*DetectResult, error) {
    // GitHub Actions: HTTP request to token endpoint
    if reqURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"); reqURL != "" {
        token, err := fetchGitHubToken(reqURL, audience)
        if err != nil {
            return nil, fmt.Errorf("github actions OIDC token fetch failed: %w", err)
        }
        return &DetectResult{Provider: CIGitHubActions, Token: token}, nil
    }

    // GitLab CI: token is directly in env var
    if token := os.Getenv("CI_JOB_JWT_V2"); token != "" {
        return &DetectResult{Provider: CIGitLabCI, Token: token}, nil
    }

    // CircleCI: token is directly in env var
    if token := os.Getenv("CIRCLE_OIDC_TOKEN_V2"); token != "" {
        return &DetectResult{Provider: CICircleCI, Token: token}, nil
    }

    // Custom: user-specified env var
    if tokenEnv != "" {
        if token := os.Getenv(tokenEnv); token != "" {
            return &DetectResult{Provider: CICustom, Token: token}, nil
        }
        return nil, fmt.Errorf("token_env %q is set in config but the environment variable is empty or unset", tokenEnv)
    }

    return nil, fmt.Errorf("no CI OIDC environment detected (checked: ACTIONS_ID_TOKEN_REQUEST_URL, CI_JOB_JWT_V2, CIRCLE_OIDC_TOKEN_V2); set auth.profiles[].token_env to use a custom token source")
}

// fetchGitHubToken requests an OIDC token from GitHub Actions.
func fetchGitHubToken(reqURL, audience string) (string, error) {
    bearerToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
    if bearerToken == "" {
        return "", fmt.Errorf("ACTIONS_ID_TOKEN_REQUEST_TOKEN not set (missing id-token: write permission?)")
    }

    if audience != "" {
        reqURL = reqURL + "&audience=" + audience
    }

    req, err := http.NewRequest("GET", reqURL, nil)
    if err != nil {
        return "", err
    }
    req.Header.Set("Authorization", "bearer "+bearerToken)

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("HTTP request failed: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("reading response: %w", err)
    }

    if resp.StatusCode != 200 {
        return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }

    var result struct {
        Value string `json:"value"`
    }
    if err := json.Unmarshal(body, &result); err != nil {
        return "", fmt.Errorf("parsing response JSON: %w", err)
    }

    if result.Value == "" {
        return "", fmt.Errorf("empty token in response")
    }
    return result.Value, nil
}

// ValidateTokenExp checks the OIDC JWT's exp claim locally.
// Returns error if token is expired or within skew of expiry.
// This is a fast-fail check before making the STS network call.
func ValidateTokenExp(token string, skew time.Duration) error {
    // JWT is three base64url-encoded segments separated by dots
    parts := strings.SplitN(token, ".", 3)
    if len(parts) != 3 {
        // Not a valid JWT structure; let the STS endpoint reject it
        return nil
    }

    // Decode the payload (second segment)
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        // Can't decode; let STS handle validation
        return nil
    }

    var claims struct {
        Exp int64 `json:"exp"`
    }
    if err := json.Unmarshal(payload, &claims); err != nil {
        return nil
    }

    if claims.Exp == 0 {
        return nil // No exp claim; proceed
    }

    expTime := time.Unix(claims.Exp, 0)
    if time.Now().Add(skew).After(expTime) {
        return fmt.Errorf("OIDC token expired or expiring within %v (exp: %s, now: %s)",
            skew, expTime.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
    }

    return nil
}
```

- [ ] Create `internal/auth/detect.go`
- [ ] Verify: `go build ./internal/auth/`

**Test:** `go test ./internal/auth/ -run TestDetectCI -v` -- env var manipulation tests, token exp validation.

**Commit:** `feat(auth): add CI environment auto-detection and token clock skew validation (FEAT-049)`

---

## Task 5: AWS STS Exchange (P-05)

**Files:** `internal/auth/provider_aws.go`

Raw HTTP implementation of `AssumeRoleWithWebIdentity`. Uses `encoding/xml` (stdlib) for response parsing.

```go
package auth

import (
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
)

// awsSTSResponse is the XML structure returned by AssumeRoleWithWebIdentity.
type awsSTSResponse struct {
    XMLName xml.Name `xml:"AssumeRoleWithWebIdentityResponse"`
    Result  struct {
        Credentials struct {
            AccessKeyID     string `xml:"AccessKeyId"`
            SecretAccessKey string `xml:"SecretAccessKey"`
            SessionToken    string `xml:"SessionToken"`
            Expiration      string `xml:"Expiration"`
        } `xml:"Credentials"`
    } `xml:"AssumeRoleWithWebIdentityResult"`
}

// awsSTSErrorResponse parses STS error responses.
type awsSTSErrorResponse struct {
    XMLName xml.Name `xml:"ErrorResponse"`
    Error   struct {
        Code    string `xml:"Code"`
        Message string `xml:"Message"`
    } `xml:"Error"`
}

// ExchangeAWS performs AssumeRoleWithWebIdentity against AWS STS.
// stsEndpoint is overridable for testing (empty = production endpoint).
func ExchangeAWS(roleARN, audience, region, sessionDuration, oidcToken, stsEndpoint string) (*Credentials, error) {
    if stsEndpoint == "" {
        if region != "" {
            stsEndpoint = fmt.Sprintf("https://sts.%s.amazonaws.com/", region)
        } else {
            stsEndpoint = "https://sts.amazonaws.com/"
        }
    }

    if audience == "" {
        audience = "sts.amazonaws.com"
    }

    duration := "3600" // 1 hour default
    if sessionDuration != "" {
        d, err := time.ParseDuration(sessionDuration)
        if err == nil {
            duration = fmt.Sprintf("%d", int(d.Seconds()))
        }
    }

    sessionName := fmt.Sprintf("smokesig-%d", time.Now().Unix())

    params := url.Values{
        "Action":           {"AssumeRoleWithWebIdentity"},
        "Version":          {"2011-06-15"},
        "RoleArn":          {roleARN},
        "RoleSessionName":  {sessionName},
        "WebIdentityToken": {oidcToken},
        "DurationSeconds":  {duration},
    }

    resp, err := http.PostForm(stsEndpoint, params)
    if err != nil {
        return nil, fmt.Errorf("AWS STS request failed: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("reading STS response: %w", err)
    }

    if resp.StatusCode != 200 {
        var stsErr awsSTSErrorResponse
        if xml.Unmarshal(body, &stsErr) == nil && stsErr.Error.Message != "" {
            return nil, fmt.Errorf("AWS STS error (HTTP %d): %s — %s (token: ***redacted***)",
                resp.StatusCode, stsErr.Error.Code, stsErr.Error.Message)
        }
        return nil, fmt.Errorf("AWS STS error (HTTP %d): %s (token: ***redacted***)",
            resp.StatusCode, string(body))
    }

    var stsResp awsSTSResponse
    if err := xml.Unmarshal(body, &stsResp); err != nil {
        return nil, fmt.Errorf("parsing STS XML response: %w", err)
    }

    creds := stsResp.Result.Credentials
    if creds.AccessKeyID == "" {
        return nil, fmt.Errorf("AWS STS returned empty credentials")
    }

    expiration, _ := time.Parse(time.RFC3339, creds.Expiration)

    return &Credentials{
        Provider:       ProviderAWS,
        AccessKeyID:    []byte(creds.AccessKeyID),
        SecretAccessKey: []byte(creds.SecretAccessKey),
        SessionToken:   []byte(creds.SessionToken),
        Expiration:     expiration,
    }, nil
}
```

Note: The auth package accepts individual parameters rather than a profile struct to avoid importing the schema package (keeps auth testable in isolation). The exchange orchestrator (Task 7) maps from `schema.AuthProfile` fields to function args. The function signature used above is:
```go
func ExchangeAWS(roleARN, audience, region, sessionDuration, oidcToken, stsEndpoint string) (*Credentials, error)
```
Update the function body to use these parameter names instead of `profile.RoleARN`, etc.

- [ ] Create `internal/auth/provider_aws.go`
- [ ] Verify: `go build ./internal/auth/`

**Test:** `go test ./internal/auth/ -run TestExchangeAWS -v` -- mock STS httptest server, valid response parsing, error response parsing, empty credentials.

**Commit:** `feat(auth): add AWS STS AssumeRoleWithWebIdentity exchange (FEAT-049)`

---

## Task 6: GCP STS Exchange (P-06)

**Files:** `internal/auth/provider_gcp.go`

Two-step exchange: (1) federate OIDC token for STS access token, (2) exchange for service account access token. Default to `CLOUDSDK_AUTH_ACCESS_TOKEN` env var injection; temp keyfile only when `gcp_credential_format: keyfile`.

```go
package auth

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"
)

// gcpSTSResponse is returned by sts.googleapis.com/v1/token.
type gcpSTSResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    ExpiresIn   int    `json:"expires_in"`
}

// gcpIAMResponse is returned by iamcredentials generateAccessToken.
type gcpIAMResponse struct {
    AccessToken string `json:"accessToken"`
    ExpireTime  string `json:"expireTime"`
}

// ExchangeGCP performs the two-step GCP Workload Identity Federation exchange.
// stsEndpoint and iamEndpoint are overridable for testing (empty = production).
// credentialFormat: "env" (default, sets CLOUDSDK_AUTH_ACCESS_TOKEN) or "keyfile" (writes temp JSON).
func ExchangeGCP(
    workloadIdentityProvider, serviceAccountEmail, oidcToken string,
    credentialFormat string,
    stsEndpoint, iamEndpoint string,
) (*Credentials, error) {
    if stsEndpoint == "" {
        stsEndpoint = "https://sts.googleapis.com/v1/token"
    }

    // Step 1: Exchange OIDC token for federated access token
    stsBody := map[string]string{
        "grantType":          "urn:ietf:params:oauth:grant-type:token-exchange",
        "audience":           "//iam.googleapis.com/" + workloadIdentityProvider,
        "scope":              "https://www.googleapis.com/auth/cloud-platform",
        "requestedTokenType": "urn:ietf:params:oauth:token-type:access_token",
        "subjectTokenType":   "urn:ietf:params:oauth:token-type:jwt",
        "subjectToken":       oidcToken,
    }
    stsJSON, _ := json.Marshal(stsBody)

    stsResp, err := http.Post(stsEndpoint, "application/json", bytes.NewReader(stsJSON))
    if err != nil {
        return nil, fmt.Errorf("GCP STS request failed: %w", err)
    }
    defer stsResp.Body.Close()

    stsRespBody, _ := io.ReadAll(stsResp.Body)
    if stsResp.StatusCode != 200 {
        return nil, fmt.Errorf("GCP STS error (HTTP %d): %s (token: ***redacted***)",
            stsResp.StatusCode, string(stsRespBody))
    }

    var stsResult gcpSTSResponse
    if err := json.Unmarshal(stsRespBody, &stsResult); err != nil {
        return nil, fmt.Errorf("parsing GCP STS response: %w", err)
    }

    // Step 2: Exchange federated token for service account access token
    if iamEndpoint == "" {
        iamEndpoint = fmt.Sprintf(
            "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
            serviceAccountEmail,
        )
    }

    iamBody := map[string]interface{}{
        "scope":    []string{"https://www.googleapis.com/auth/cloud-platform"},
        "lifetime": "3600s",
    }
    iamJSON, _ := json.Marshal(iamBody)

    iamReq, _ := http.NewRequest("POST", iamEndpoint, bytes.NewReader(iamJSON))
    iamReq.Header.Set("Authorization", "Bearer "+stsResult.AccessToken)
    iamReq.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 10 * time.Second}
    iamResp, err := client.Do(iamReq)
    if err != nil {
        return nil, fmt.Errorf("GCP IAM request failed: %w", err)
    }
    defer iamResp.Body.Close()

    iamRespBody, _ := io.ReadAll(iamResp.Body)
    if iamResp.StatusCode != 200 {
        return nil, fmt.Errorf("GCP IAM error (HTTP %d): %s (token: ***redacted***)",
            iamResp.StatusCode, string(iamRespBody))
    }

    var iamResult gcpIAMResponse
    if err := json.Unmarshal(iamRespBody, &iamResult); err != nil {
        return nil, fmt.Errorf("parsing GCP IAM response: %w", err)
    }

    expiration, _ := time.Parse(time.RFC3339, iamResult.ExpireTime)

    creds := &Credentials{
        Provider:    ProviderGCP,
        AccessToken: []byte(iamResult.AccessToken),
        Expiration:  expiration,
    }

    // If keyfile format requested, write temp credentials file
    if credentialFormat == "keyfile" {
        keyfilePath, cleanup, err := writeGCPKeyfile(iamResult.AccessToken, expiration)
        if err != nil {
            return nil, fmt.Errorf("writing GCP temp keyfile: %w", err)
        }
        creds.KeyfilePath = keyfilePath
        // Register SIGTERM handler for cleanup.
        // NOTE: The goroutine blocks until a signal is received. This is acceptable
        // because there is at most one keyfile per GCP profile per suite run, and
        // the goroutine exits on process termination. The AuthContext.CleanupKeyfiles()
        // method handles normal teardown; this handler covers abnormal termination.
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
        go func() {
            <-sigCh
            cleanup()
            signal.Stop(sigCh) // Unregister to avoid stacking handlers on refresh
        }()
    }

    return creds, nil
}

// writeGCPKeyfile writes a temporary GCP credentials JSON file.
// Returns the file path and a cleanup function.
func writeGCPKeyfile(accessToken string, expiration time.Time) (string, func(), error) {
    tmpDir := os.TempDir()
    keyfilePath := filepath.Join(tmpDir, fmt.Sprintf("smokesig-gcp-%d.json", time.Now().UnixNano()))

    keyfileContent := map[string]interface{}{
        "type":         "external_account",
        "access_token": accessToken,
        "expiry":       expiration.Format(time.RFC3339),
    }
    data, _ := json.MarshalIndent(keyfileContent, "", "  ")

    if err := os.WriteFile(keyfilePath, data, 0600); err != nil {
        return "", nil, err
    }

    cleanup := func() {
        os.Remove(keyfilePath)
    }

    return keyfilePath, cleanup, nil
}
```

- [ ] Create `internal/auth/provider_gcp.go`
- [ ] Verify: `go build ./internal/auth/`

**Test:** `go test ./internal/auth/ -run TestExchangeGCP -v` -- mock both STS and IAM endpoints, verify two-step flow, test env var default vs keyfile mode.

**Commit:** `feat(auth): add GCP Workload Identity Federation exchange (FEAT-049)`

---

## Task 7: Exchange Orchestrator (P-07, P-08, P-10)

**Files:** `internal/auth/exchange.go`

The top-level orchestrator that: detects CI, validates token expiry, exchanges credentials, builds AuthContext, handles caching and TTL refresh for watch mode.

```go
package auth

import (
    "fmt"
    "os"
    "time"

    "github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

const tokenExpirySkew = 30 * time.Second
const watchRefreshWindow = 5 * time.Minute

// ExchangeAll performs OIDC token exchange for all configured auth profiles.
// Returns an AuthContext with all credentials, or error on first failure (unless fallback=env).
func ExchangeAll(authCfg schema.AuthConfig) (*AuthContext, error) {
    if len(authCfg.Profiles) == 0 {
        return nil, nil // No auth configured — not an error
    }

    ctx := NewAuthContext()
    fallback := authCfg.Fallback
    if fallback == "" {
        fallback = "fail"
    }

    for _, profile := range authCfg.Profiles {
        name := profile.Name
        if name == "" {
            name = "default"
        }

        creds, err := exchangeProfile(profile)
        if err != nil {
            if fallback == "env" {
                // Log warning, continue with whatever env provides
                fmt.Fprintf(os.Stderr, "⚠ auth profile %q: OIDC exchange failed, falling back to environment: %v\n", name, err)
                continue
            }
            return nil, fmt.Errorf("auth profile %q: %w", name, err)
        }

        creds.ProfileName = name
        ctx.Set(name, creds)
    }

    // Set the first profile as active by default
    if len(authCfg.Profiles) > 0 {
        name := authCfg.Profiles[0].Name
        if name == "" {
            name = "default"
        }
        ctx.SetActive(name)
    }

    return ctx, nil
}

// exchangeProfile detects CI, validates token, and exchanges for a single profile.
func exchangeProfile(profile schema.AuthProfile) (*Credentials, error) {
    // Detect CI and acquire OIDC token
    result, err := DetectCI(profile.TokenEnv, profile.Audience)
    if err != nil {
        return nil, err
    }

    // Validate token expiry locally before network call
    if err := ValidateTokenExp(result.Token, tokenExpirySkew); err != nil {
        return nil, err
    }

    // Exchange token with cloud provider
    switch Provider(profile.Provider) {
    case ProviderAWS:
        return ExchangeAWS(
            profile.RoleARN,
            profile.Audience,
            profile.Region,
            profile.SessionDuration,
            result.Token,
            "", // production endpoint
        )
    case ProviderGCP:
        format := profile.GCPCredentialFormat
        if format == "" {
            format = "env"
        }
        return ExchangeGCP(
            profile.WorkloadIdentityProvider,
            profile.ServiceAccountEmail,
            result.Token,
            format,
            "", "", // production endpoints
        )
    default:
        return nil, fmt.Errorf("unsupported provider %q", profile.Provider)
    }
}

// NeedsRefresh returns true if any credential is within the refresh window of expiry.
// Used by watch mode to trigger re-exchange between cycles.
func NeedsRefresh(ctx *AuthContext) bool {
    if ctx == nil {
        return false
    }
    ctx.mu.RLock()
    defer ctx.mu.RUnlock()
    for _, creds := range ctx.profiles {
        if creds.Expired(watchRefreshWindow) {
            return true
        }
    }
    return false
}

// RefreshAll re-exchanges all profiles. Used by watch mode.
func RefreshAll(authCfg schema.AuthConfig, ctx *AuthContext) error {
    if ctx == nil {
        return nil
    }
    newCtx, err := ExchangeAll(authCfg)
    if err != nil {
        return err
    }
    // Swap credentials
    ctx.mu.Lock()
    defer ctx.mu.Unlock()
    // Zero old credentials
    for _, c := range ctx.profiles {
        c.Zero()
    }
    ctx.profiles = newCtx.profiles
    ctx.active = newCtx.active
    return nil
}
```

- [ ] Create `internal/auth/exchange.go`
- [ ] Verify: `go build ./internal/auth/`

**Test:** `go test ./internal/auth/ -run TestExchangeAll -v`

**Commit:** `feat(auth): add exchange orchestrator with TTL refresh for watch mode (FEAT-049)`

---

## Task 8: Credential Masking (P-11)

**Files:** `internal/auth/masking.go`

Utilities to redact credential material from error messages and reporter output.

```go
package auth

import "strings"

// knownCredentialPrefixes are token/key prefixes that should never appear in output.
var knownCredentialPrefixes = []string{
    "AKIA",  // AWS long-lived access key
    "ASIA",  // AWS temporary access key
    "ya29.", // GCP access token
    "eyJ",   // JWT (base64 of {"alg":...)
}

// MaskCredentials replaces known credential patterns in a string with ***redacted***.
func MaskCredentials(s string) string {
    for _, prefix := range knownCredentialPrefixes {
        for {
            idx := strings.Index(s, prefix)
            if idx == -1 {
                break
            }
            // Find the end of the credential (next whitespace, quote, or end of string)
            end := idx + len(prefix)
            for end < len(s) && !isCredentialBoundary(s[end]) {
                end++
            }
            s = s[:idx] + "***redacted***" + s[end:]
        }
    }
    return s
}

func isCredentialBoundary(b byte) bool {
    return b == ' ' || b == '"' || b == '\'' || b == '\n' || b == '\r' || b == '\t' || b == ',' || b == '}' || b == ']'
}
```

- [ ] Create `internal/auth/masking.go`
- [ ] Verify: `go build ./internal/auth/`

**Test:** `go test ./internal/auth/ -run TestMaskCredentials -v`

**Commit:** `feat(auth): add credential masking utilities (FEAT-049)`

---

## Task 9: Runner Integration (P-08, P-09, P-13)

**Files:** `internal/runner/runner.go`

Integrate auth exchange into the `Run()` method. Three changes:

### 9a: Auth exchange after prerequisites, before tests

In `runner.go` `Run()`, after the prerequisite check block (line ~124) and before test filtering (line ~128), add:

```go
    // Perform OIDC auth exchange (after prereqs, before tests)
    var authCtx *auth.AuthContext
    if len(r.Config.Auth.Profiles) > 0 {
        var err error
        authCtx, err = auth.ExchangeAll(r.Config.Auth)
        if err != nil {
            // Surface auth failure as a synthetic prerequisite failure
            r.Reporter.PrereqStart("oidc-auth")
            r.Reporter.PrereqResult(reporter.PrereqResultData{
                Name:   "oidc-auth",
                Passed: false,
                Output: err.Error(),
                Hint:   "Check auth: config, CI OIDC permissions, and cloud role trust policy",
                Error:  err,
            })
            return nil, fmt.Errorf("OIDC auth exchange failed: %w", err)
        }
        // Inject credentials as env vars for run: commands
        if active := authCtx.Active(); active != nil {
            for k, v := range active.EnvVars() {
                os.Setenv(k, v)
            }
        }
        // Defer credential cleanup
        defer func() {
            if authCtx != nil {
                // Unset env vars
                if active := authCtx.Active(); active != nil {
                    for k := range active.EnvVars() {
                        os.Unsetenv(k)
                    }
                }
                // Remove GCP keyfiles via exported method (profiles is unexported)
                authCtx.CleanupKeyfiles()
                // Zero all credentials
                authCtx.ZeroAll()
            }
        }()
    }
    r.authCtx = authCtx
```

### 9b: Add `authCtx` field to Runner struct

```go
type Runner struct {
    Config        *schema.SmokeConfig
    Reporter      reporter.Reporter
    ConfigDir     string
    trace         *TraceContext
    TraceHealth   *TraceHealthTracker
    Vars          *VarStore
    lifecycleEnv  map[string]string
    lifecycleMu   sync.RWMutex
    authCtx       *auth.AuthContext // OIDC credentials for cloud assertions
}
```

### 9c: Per-test auth profile override

In `runTestOnce()`, before the `if t.Run != ""` command execution block (line ~393), swap env vars if the test has an `auth:` override. This must happen before command execution AND standalone assertions so both benefit from the override:

```go
    // Per-test auth profile override
    var restoreAuth func()
    if t.Auth != "" && r.authCtx != nil {
        profileCreds := r.authCtx.Get(t.Auth)
        if profileCreds != nil {
            // Save current env vars
            saved := make(map[string]string)
            for k := range profileCreds.EnvVars() {
                saved[k] = os.Getenv(k)
            }
            // Set profile-specific env vars
            for k, v := range profileCreds.EnvVars() {
                os.Setenv(k, v)
            }
            restoreAuth = func() {
                for k, v := range saved {
                    if v == "" {
                        os.Unsetenv(k)
                    } else {
                        os.Setenv(k, v)
                    }
                }
            }
        }
    }
    if restoreAuth != nil {
        defer restoreAuth()
    }
```

### 9d: Add import

```go
import (
    // ... existing imports ...
    "github.com/CosmoLabs-org/SmokeSig/internal/auth"
)
```

- [ ] Add `authCtx` field to `Runner` struct
- [ ] Add auth exchange block in `Run()` after prereqs
- [ ] Add per-test auth override in `runTestOnce()`
- [ ] Add `internal/auth` import
- [ ] Verify: `go build ./...`

**Test:** `go test ./internal/runner/ -run TestRun -v` -- test with no auth config (backward compat), test with mock auth (requires wiring test helpers).

**Commit:** `feat(runner): integrate OIDC auth exchange and per-test profile override (FEAT-049)`

---

## Task 10: Watch Mode TTL Refresh (P-10)

**Files:** `cmd/run.go` (watch mode is in the `runWatch` function at line ~410)

Watch mode creates a fresh `Runner` inside the `runOnce` closure on each cycle (lines ~308-332 for normal mode, ~238-271 for monorepo mode). Since `ExchangeAll` is called inside `Runner.Run()` (added in Task 9), each watch cycle already performs a fresh token exchange.

The TTL-aware optimization avoids redundant exchanges when credentials are still valid. This requires the Runner to cache and reuse credentials across watch cycles. Add a package-level credential cache in `internal/auth/exchange.go`:

```go
// cachedCtx holds credentials from the last successful exchange.
// Used by watch mode to avoid redundant STS calls.
var (
    cachedCtx   *AuthContext
    cachedMu    sync.Mutex
)

// ExchangeAllCached returns cached credentials if still valid, otherwise performs fresh exchange.
func ExchangeAllCached(authCfg schema.AuthConfig) (*AuthContext, error) {
    cachedMu.Lock()
    defer cachedMu.Unlock()

    if cachedCtx != nil && !NeedsRefresh(cachedCtx) {
        return cachedCtx, nil
    }

    ctx, err := ExchangeAll(authCfg)
    if err != nil {
        return nil, err
    }

    // Zero old cached credentials
    if cachedCtx != nil {
        cachedCtx.ZeroAll()
    }
    cachedCtx = ctx
    return ctx, nil
}
```

Then in Task 9, when `--watch` mode is active, `Runner.Run()` should call `ExchangeAllCached` instead of `ExchangeAll`. The watch mode detection can be via a new `WatchMode bool` field on `RunOptions`.

- [ ] Add `ExchangeAllCached` to `internal/auth/exchange.go`
- [ ] Add `WatchMode bool` to `RunOptions` in runner
- [ ] Set `WatchMode: true` in `cmd/run.go` watch closures (lines ~321, ~260)
- [ ] In Task 9 auth block, use `ExchangeAllCached` when `opts.WatchMode` is true
- [ ] Verify: `go build ./...`

**Commit:** `feat(runner): add credential TTL refresh in watch mode (FEAT-049)`

---

## Task 11: Monorepo Auth Inheritance (P-12)

**Files:** `internal/runner/runner.go` (in `RunMonorepo()`)

When running in monorepo mode, sub-runners should inherit the root config's `auth:` section unless the sub-config defines its own.

In `RunMonorepo()`, after loading each sub-config at line ~185 (`cfg, err := schema.Load(sc.Path)`) and before creating the sub-runner at line ~188:

```go
    // Inherit auth from root config if sub-config doesn't define its own
    if len(cfg.Auth.Profiles) == 0 && len(r.Config.Auth.Profiles) > 0 {
        cfg.Auth = r.Config.Auth
    }
```

- [ ] Add auth inheritance in `RunMonorepo()`
- [ ] Verify: `go build ./...`

**Test:** `go test ./internal/runner/ -run TestRunMonorepo -v`

**Commit:** `feat(runner): add monorepo auth inheritance (FEAT-049)`

---

## Task 12: Schema Command Update (P-14)

**Files:** `cmd/schema.go`

The `smokesig schema` command exports assertion types as JSON. Add `auth` types to the output so users can discover the auth config format.

- [ ] Add `AuthConfig` and `AuthProfile` to the schema JSON export
- [ ] Verify: `go build ./... && ./smokesig schema | jq .auth`

**Commit:** `feat(schema): include auth types in smokesig schema output (FEAT-049)`

---

## Task 13: Unit Tests (P-15)

**Files:** `internal/auth/*_test.go`

### detect_test.go

```go
func TestDetectCI_GitHubActions(t *testing.T) {
    // Set up mock HTTP server for token endpoint
    // Set ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN
    // Verify correct provider detected and token acquired
}

func TestDetectCI_GitLabCI(t *testing.T) {
    t.Setenv("CI_JOB_JWT_V2", "test-jwt-token")
    result, err := DetectCI("", "")
    require.NoError(t, err)
    assert.Equal(t, CIGitLabCI, result.Provider)
    assert.Equal(t, "test-jwt-token", result.Token)
}

func TestDetectCI_CircleCI(t *testing.T) { /* similar */ }
func TestDetectCI_Custom(t *testing.T) { /* token_env override */ }
func TestDetectCI_NoneDetected(t *testing.T) { /* error case */ }
func TestDetectCI_Priority(t *testing.T) { /* GH > GL > Circle */ }

func TestValidateTokenExp_Valid(t *testing.T) { /* token with future exp */ }
func TestValidateTokenExp_Expired(t *testing.T) { /* token with past exp */ }
func TestValidateTokenExp_WithinSkew(t *testing.T) { /* token expiring within skew */ }
func TestValidateTokenExp_NotJWT(t *testing.T) { /* non-JWT passes through */ }
```

### provider_aws_test.go

```go
func TestExchangeAWS_Success(t *testing.T) {
    sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "AssumeRoleWithWebIdentity", r.FormValue("Action"))
        assert.Equal(t, "arn:aws:iam::123:role/test", r.FormValue("RoleArn"))
        w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>
            <AssumeRoleWithWebIdentityResult>
                <Credentials>
                    <AccessKeyId>ASIAMOCKKEY123</AccessKeyId>
                    <SecretAccessKey>mocksecret</SecretAccessKey>
                    <SessionToken>mocktoken</SessionToken>
                    <Expiration>2026-05-29T13:00:00Z</Expiration>
                </Credentials>
            </AssumeRoleWithWebIdentityResult>
        </AssumeRoleWithWebIdentityResponse>`))
    }))
    defer sts.Close()

    creds, err := ExchangeAWS("arn:aws:iam::123:role/test", "", "", "", "fake-token", sts.URL)
    require.NoError(t, err)
    assert.Equal(t, "ASIAMOCKKEY123", string(creds.AccessKeyID))
}

func TestExchangeAWS_STSError(t *testing.T) { /* 403 response */ }
func TestExchangeAWS_RegionalEndpoint(t *testing.T) { /* region= sets endpoint */ }
func TestExchangeAWS_CustomDuration(t *testing.T) { /* session_duration */ }
```

### provider_gcp_test.go

```go
func TestExchangeGCP_Success_EnvVar(t *testing.T) {
    // Mock both STS and IAM endpoints
    // Verify CLOUDSDK_AUTH_ACCESS_TOKEN is the output, no keyfile
}

func TestExchangeGCP_Success_Keyfile(t *testing.T) {
    // Mock endpoints, request keyfile format
    // Verify temp file created with correct content
    // Verify cleanup function works
}

func TestExchangeGCP_STSError(t *testing.T) { /* step 1 failure */ }
func TestExchangeGCP_IAMError(t *testing.T) { /* step 2 failure */ }
```

### auth_test.go

```go
func TestCredentials_Zero(t *testing.T) {
    creds := &Credentials{
        AccessKeyID:    []byte("ASIAMOCKKEY"),
        SecretAccessKey: []byte("secret"),
    }
    creds.Zero()
    assert.Equal(t, make([]byte, len("ASIAMOCKKEY")), creds.AccessKeyID)
}

func TestCredentials_String_Redacted(t *testing.T) {
    creds := &Credentials{Provider: ProviderAWS, AccessKeyID: []byte("ASIAMOCKKEY123")}
    assert.Equal(t, "aws:ASIA***", creds.String())
}

func TestCredentials_EnvVars_AWS(t *testing.T) { /* correct env var names */ }
func TestCredentials_EnvVars_GCP_Default(t *testing.T) { /* CLOUDSDK_AUTH_ACCESS_TOKEN */ }
func TestCredentials_EnvVars_GCP_Keyfile(t *testing.T) { /* GOOGLE_APPLICATION_CREDENTIALS */ }
func TestCredentials_Expired(t *testing.T) { /* TTL check with skew */ }

func TestMaskCredentials(t *testing.T) {
    input := `error: AWS returned ASIAMOCKKEY123456 for role`
    assert.Equal(t, `error: AWS returned ***redacted*** for role`, MaskCredentials(input))
}

func TestAuthContext_ProfileManagement(t *testing.T) { /* Set, Get, Active, SetActive */ }
```

- [ ] Create all test files
- [ ] Run: `go test ./internal/auth/ -v`
- [ ] Verify all pass

**Commit:** `test(auth): add unit tests for CI detection, AWS/GCP exchange, masking, zeroing (FEAT-049)`

---

## Task 14: Integration Tests with Mock STS (P-16)

**Files:** `internal/auth/integration_test.go`

Full end-to-end exchange flow using `httptest.Server` for both AWS STS and GCP STS/IAM:

```go
func TestFullExchangeFlow_AWS(t *testing.T) {
    // 1. Set up mock STS server
    // 2. Set up mock CI env (ACTIONS_ID_TOKEN_REQUEST_URL pointing at another mock)
    // 3. Call ExchangeAll with a schema.AuthConfig
    // 4. Verify AuthContext has correct credentials
    // 5. Verify EnvVars() returns correct keys
    // 6. Verify ZeroAll() clears everything
}

func TestFullExchangeFlow_GCP(t *testing.T) { /* similar with two-step mock */ }
func TestFullExchangeFlow_FallbackEnv(t *testing.T) { /* OIDC fails, fallback=env succeeds */ }
func TestFullExchangeFlow_FallbackFail(t *testing.T) { /* OIDC fails, fallback=fail errors */ }
```

- [ ] Create integration test file
- [ ] Run: `go test ./internal/auth/ -v -run TestFullExchange`

**Commit:** `test(auth): add integration tests with mock STS servers (FEAT-049)`

---

## Task 15: Validation Tests (P-15)

**Files:** `internal/schema/validate_test.go`

Add auth validation test cases to the existing validation test suite.

```go
func TestValidate_AuthProfiles(t *testing.T) {
    tests := []struct {
        name    string
        config  SmokeConfig
        wantErr string
    }{
        {name: "valid aws", config: validAWSConfig(), wantErr: ""},
        {name: "valid gcp", config: validGCPConfig(), wantErr: ""},
        {name: "missing role_arn", config: awsNoRoleARN(), wantErr: "aws provider requires role_arn"},
        {name: "invalid role_arn", config: awsBadRoleARN(), wantErr: "invalid role_arn format"},
        {name: "missing gcp provider", config: gcpNoWIP(), wantErr: "gcp provider requires workload_identity_provider"},
        {name: "missing gcp email", config: gcpNoEmail(), wantErr: "gcp provider requires service_account_email"},
        {name: "unsupported provider", config: unsupportedProvider(), wantErr: "unsupported provider"},
        {name: "duplicate name", config: duplicateProfileName(), wantErr: "duplicate name"},
        {name: "invalid fallback", config: invalidFallback(), wantErr: "invalid value"},
        {name: "bad session_duration", config: badDuration(), wantErr: "invalid session_duration"},
        {name: "duration out of range", config: durationOutOfRange(), wantErr: "between 15m and 12h"},
        {name: "test refs missing profile", config: testRefsMissing(), wantErr: "auth profile .* not found"},
        {name: "invalid gcp_credential_format", config: badGCPFormat(), wantErr: "gcp_credential_format must be env or keyfile"},
    }
    // ...
}
```

- [ ] Add auth validation tests to `validate_test.go`
- [ ] Run: `go test ./internal/schema/ -v -run TestValidate_Auth`

**Commit:** `test(schema): add auth config validation tests (FEAT-049)`

---

## Task 16: Documentation Updates (P-17)

**Files:** `CLAUDE.md`

### 16a: Add `--no-auth` CLI flag and update CLAUDE.md

No new assertion type -- auth is a config-level feature, not an assertion. Add a `--no-auth` flag to `cmd/run.go` (mirrors `--no-otel` pattern):

```go
// In var block:
noAuth bool

// In init():
runCmd.Flags().BoolVar(&noAuth, "no-auth", false, "Disable OIDC auth for this run")

// In loadConfig(), after noOtel handling:
if noAuth {
    cfg.Auth.Profiles = nil
}
```

Then update CLAUDE.md Commands section:
```
smokesig run [...existing flags...] [--no-auth]
```

### 16b: Add `auth:` to CLAUDE.md overview

Under "Key Design Decisions", add:
```
- **OIDC auth**: `auth:` config section for CI-to-cloud role assumption. AWS STS + GCP Workload Identity. No cloud SDK deps (raw HTTP). Env var injection for `run:` commands + kubectl. Standalone HTTP assertions (e.g., s3_bucket) remain anonymous (no SigV4 in v1).
```

### 16c: Limitation documentation

Add a comment block at the top of `internal/auth/auth.go`:

```go
// Package auth implements OIDC-based cloud authentication for SmokeSig.
//
// v1 Scope:
//   - AWS STS AssumeRoleWithWebIdentity (raw HTTP, no SDK)
//   - GCP Workload Identity Federation (raw HTTP, no SDK)
//   - CI auto-detection: GitHub Actions, GitLab CI, CircleCI
//
// v1 Limitations:
//   - Credential injection is env-var-only. run: commands and k8s_resource (kubectl)
//     consume them. Standalone assertions (s3_bucket, http, url_reachable) use raw
//     net/http without AWS SigV4 signing — they remain anonymous.
//   - SigV4 request signing for standalone assertions is a v2 item.
//   - Azure Managed Identity is a v2 item (different architecture: VM-attached).
//   - No per-assertion auth header injection in v1.
//
// GCP credential format:
//   - Default: CLOUDSDK_AUTH_ACCESS_TOKEN env var (no temp file)
//   - Optional: gcp_credential_format=keyfile writes a temp JSON file and sets
//     GOOGLE_APPLICATION_CREDENTIALS. SIGTERM handler registered for cleanup.
```

- [ ] Update CLAUDE.md with auth description
- [ ] Add package doc comment to `internal/auth/auth.go`

**Commit:** `docs: document OIDC auth integration and v1 limitations (FEAT-049)`

---

## Execution Order

Tasks are sequential with natural dependency chains:

```
Task 1 (schema types)
  └─> Task 2 (validation)
       └─> Task 15 (validation tests)

Task 3 (auth core types)
  └─> Task 4 (CI detection)
  └─> Task 5 (AWS provider)
  └─> Task 6 (GCP provider)
  └─> Task 8 (masking)
       └─> Task 7 (orchestrator)
            └─> Task 9 (runner integration)
                 └─> Task 10 (watch mode)
                 └─> Task 11 (monorepo inheritance)
            └─> Task 13 (unit tests)
            └─> Task 14 (integration tests)

Task 12 (schema command) — independent, after Task 1

Task 16 (docs + --no-auth flag) — after all implementation
```

**Parallelizable groups:**
- Tasks 4, 5, 6, 8 can be developed in parallel (all depend only on Task 3)
- Tasks 10, 11 can be developed in parallel (both depend on Task 9)
- Tasks 13, 14 can be developed in parallel (test writing)

---

## Verification Checklist

After all tasks complete:

- [ ] `go build ./...` succeeds (no build tags required)
- [ ] `go test ./...` passes (all 1045+ existing tests + new auth tests)
- [ ] `smokesig validate` correctly validates `auth:` configs
- [ ] `smokesig schema` includes auth types in output
- [ ] `smokesig run` with no `auth:` section behaves identically to before (backward compat)
- [ ] `smokesig run` with `auth:` section but outside CI fails with clear error
- [ ] Credential masking: no AKIA/ASIA/ya29./eyJ tokens in any output format
- [ ] Memory zeroing: `Credentials.Zero()` clears all byte slices
- [ ] GCP default uses `CLOUDSDK_AUTH_ACCESS_TOKEN`, not temp keyfile

---

## Estimated LOC

| Component | Implementation | Tests | Total |
|-----------|---------------|-------|-------|
| Schema types + validation | ~75 | ~80 | ~155 |
| internal/auth (core + detect + providers + masking + orchestrator) | ~480 | ~350 | ~830 |
| Runner integration | ~60 | ~40 | ~100 |
| Schema command + docs | ~30 | 0 | ~30 |
| **Total** | **~645** | **~470** | **~1,115** |
