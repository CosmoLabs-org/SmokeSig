---
completed: "2026-04-18"
created: "2026-04-18"
status: COMPLETED
title: cosmo-smoke v0.6 — Connect and Verify
---

# cosmo-smoke v0.6 — Connect and Verify

## Why

v0.5 expanded ecosystem coverage (Goss migration). v0.6 expands assertion coverage — the "universal smoke test" needs to answer not just "does it run?" but "does it connect to everything it needs?"

CosmoLabs has ~95 projects spanning HTTP APIs, gRPC microservices, Go/Node/Python runtimes, Docker deployments, and S3-backed storage. The current 20 assertion types cover process, HTTP, database, Docker, and protocol checks. The gaps are connectivity (can the service reach its dependencies?) and environment verification (are the right tools installed?).

## Design Decisions

1. **No new dependencies.** All assertions use stdlib: `net/http`, `regexp`, `os/exec`. The "minimal deps" principle is a competitive advantage.
2. **url_reachable as primitive.** `service_reachable` and `s3_bucket` both reduce to "make an HTTP request, assert on the response." A shared `httpReachable` internal function avoids duplication.
3. **Pre-commit via framework.** `.pre-commit-hooks.yaml` file in the repo. Zero code, maximum adoption.
4. **version_check via command execution.** Runs a shell command and regex-matches stdout. No need to parse version strings — the pattern does the work.
5. **url_reachable vs http: distinct purpose.** The existing `http` assertion is for full response validation (body, headers, status). `url_reachable` is a lightweight connectivity check — "can I reach this URL?" with minimal config. Use `http` when you care about the response content. Use `url_reachable` when you just need to know the endpoint is up.
6. **service_reachable is url_reachable with semantic naming.** It exists for config readability: `service_reachable: {url: "https://api.stripe.com"}` communicates intent better than `url_reachable` for external dependencies. Will gain `depends_on` DAG support in v0.7.
7. **S3 is anonymous-only for v0.6.** Authenticated S3 access requires AWS SDK or manual SigV4 signing. For v0.6, anonymous HEAD only. Error messages on 403 explicitly state "bucket requires authentication — use env vars + Go templates in a custom http assertion for authenticated checks."
8. **version_check is Unix-only.** Uses `sh -c` which doesn't exist on Windows. This is acceptable for v0.6 since cosmo-smoke targets Linux/macOS servers. Windows support tracked as future enhancement.

## Assertion Types

### 1. url_reachable

Generic HTTP/HTTPS connectivity check.

```yaml
expect:
  url_reachable: {url: "https://example.com", timeout: 5s, status_code: 200}
```

- `url` (required): HTTP or HTTPS URL
- `timeout` (optional, default 5s): request timeout
- `status_code` (optional): expected status code. Default: any 2xx passes

Internal: `net/http.Client` with timeout. Returns `AssertionResult` with actual status code and response time.

### 2. service_reachable (ROAD-018)

Semantic wrapper for external service dependencies.

```yaml
expect:
  service_reachable: {url: "https://api.stripe.com", timeout: 5s}
```

- `url` (required): service endpoint URL
- `timeout` (optional, default 5s): request timeout

Internally delegates to `url_reachable` with `status_code` defaulting to any 2xx. The separate type exists for semantic clarity in config files and to support `depends_on` DAGs in v0.7.

### 3. s3_bucket (IDEA-027)

S3/compatible bucket accessibility check.

```yaml
expect:
  s3_bucket: {bucket: "my-bucket", region: "us-east-1", endpoint: "s3.amazonaws.com"}
```

- `bucket` (required): bucket name
- `region` (optional, default "us-east-1"): AWS region
- `endpoint` (optional, default "s3.amazonaws.com"): custom endpoint for S3-compatible storage (MinIO, GCS, etc.)

Uses path-style URL: `https://s3.amazonaws.com/my-bucket?location`. **Anonymous HEAD request only** — no auth fields in v0.6. For authenticated buckets, use the existing `http` assertion with Go template env var references for presigned URLs. Error on 403 includes explicit hint about this limitation.

### 4. version_check (ROAD-013)

Tool version verification via shell command.

```yaml
expect:
  version_check: {command: "go version", pattern: "go1\\.[0-9]+"}
```

- `command` (required): shell command to run
- `pattern` (required): Go regex to match against stdout

Runs via `os/exec` ("sh", "-c", command). Regex match on combined stdout. Fails if command exits non-zero or pattern doesn't match.

### 5. Pre-commit hook (IDEA-024)

`.pre-commit-hooks.yaml` file in repository root:

```yaml
- id: smoke
  name: cosmo-smoke
  description: Run smoke tests from .smoke.yaml
  entry: smoke run --fail-fast
  language: system
  pass_filenames: false
  always_run: true
  stages: [pre-commit]
```

No code changes. Requires `smoke` binary on PATH (users install via `go install` or download release). Users add to `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/CosmoLabs-org/cosmo-smoke
    rev: v0.6.0
    hooks:
      - id: smoke
```

## Schema Changes

New structs in `internal/schema/schema.go`:

```go
type URLReachableCheck struct {
    URL        string   `yaml:"url"`
    Timeout    Duration `yaml:"timeout,omitempty"`
    StatusCode *int     `yaml:"status_code,omitempty"`
}

type ServiceReachableCheck struct {
    URL     string `yaml:"url"`
    Timeout Duration `yaml:"timeout,omitempty"`
}

type S3BucketCheck struct {
    Bucket   string `yaml:"bucket"`
    Region   string `yaml:"region,omitempty"`
    Endpoint string `yaml:"endpoint,omitempty"`
}

type VersionCheck struct {
    Command string `yaml:"command"`
    Pattern string `yaml:"pattern"`
}
```

New fields on `Expect`:

```go
URLReachable     *URLReachableCheck     `yaml:"url_reachable,omitempty"`
ServiceReachable *ServiceReachableCheck `yaml:"service_reachable,omitempty"`
S3Bucket         *S3BucketCheck         `yaml:"s3_bucket,omitempty"`
VersionCheck     *VersionCheck          `yaml:"version_check,omitempty"`
```

## Runner Changes

New check functions in `internal/runner/assertion.go`:

- `CheckURLReachable(check *schema.URLReachableCheck) AssertionResult`
- `CheckServiceReachable(check *schema.ServiceReachableCheck) AssertionResult`
- `CheckS3Bucket(check *schema.S3BucketCheck) AssertionResult`
- `CheckVersionCheck(check *schema.VersionCheck) AssertionResult`

Internal shared helper: `httpReachable(url string, timeout time.Duration, expectedStatus int) (int, time.Duration, error)`. `expectedStatus = 0` means "any 2xx passes."

Wire in `runner.go` `runTestOnce()` alongside existing assertion checks. Timeout precedence: assertion-level timeout > test-level timeout > config-level timeout > 5s default.

## Validation Rules

Added to `schema.Validate()`:

| Field | Rule |
|-------|------|
| `url_reachable.url` | Must start with `http://` or `https://` |
| `url_reachable.timeout` | Must be positive if set |
| `service_reachable.url` | Must start with `http://` or `https://` |
| `s3_bucket.bucket` | Must be non-empty |
| `s3_bucket.region` | Must be non-empty if endpoint is not set |
| `version_check.command` | Must be non-empty |
| `version_check.pattern` | Must compile as valid Go regex |

## Error Handling

| Condition | Behavior |
|-----------|----------|
| Network timeout | `Passed: false`, `Actual: "connection timed out after Xs"` |
| DNS failure | `Passed: false`, `Actual: "DNS lookup failed: ..."` |
| Non-2xx response | `Passed: false`, `Actual: "HTTP 403 Forbidden"` |
| S3 403 | `Passed: false`, hint: "bucket may require authentication" |
| version_check command failure | `Passed: false`, `Actual: "exit code N"` |
| version_check pattern miss | `Passed: false`, `Actual: "output 'go version go1.22.0' did not match pattern 'go2\\..*'"` |

## Testing Plan

| Assertion | Test approach | Count |
|-----------|--------------|-------|
| `url_reachable` | httptest server: 2xx pass, 5xx fail, timeout, invalid URL | 4 |
| `service_reachable` | httptest server: pass via delegation, fail | 2 |
| `s3_bucket` | httptest server mocking S3 HEAD: 200 pass, 404 fail, 403 auth hint | 3 |
| `version_check` | test helper running echo commands: pattern match, pattern miss, command failure | 3 |
| Pre-commit YAML | validate YAML structure, verify hook fields | 1 |

~13 new tests. Suite total: ~246 (233 current + 13 new).

## File Scope

```yaml
files_modified:
  - internal/schema/schema.go
  - internal/runner/runner.go
  - internal/runner/assertion.go
files_created:
  - internal/runner/assertion_reachable_test.go
  - internal/runner/assertion_version_test.go
  - .pre-commit-hooks.yaml
```

## v0.6 Release Scope

Headline: "cosmo-smoke v0.6 — Connect and Verify"

- service_reachable: verify external service dependencies (ROAD-018)
- version_check: assert installed tool versions (ROAD-013)
- s3_bucket: S3/compatible bucket accessibility (IDEA-027)
- url_reachable: generic HTTP connectivity primitive
- Pre-commit hook: zero-config smoke test integration (IDEA-024)
- 246+ tests passing
