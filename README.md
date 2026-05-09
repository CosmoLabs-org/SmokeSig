# SmokeSig

[![Go Reference](https://pkg.go.dev/badge/github.com/CosmoLabs-org/SmokeSig.svg)](https://pkg.go.dev/github.com/CosmoLabs-org/SmokeSig) [![Go Report Card](https://goreportcard.com/badge/github.com/CosmoLabs-org/SmokeSig)](https://goreportcard.com/report/github.com/CosmoLabs-org/SmokeSig) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Universal smoke test runner. Define lightweight "does it turn on?" checks in `.smokesig.yaml` and run them with a single command — on any project, in any language.

## Install

**Go install:**
```bash
go install github.com/CosmoLabs-org/SmokeSig@latest
```

**Build from source:**
```bash
git clone https://github.com/CosmoLabs-org/SmokeSig
cd SmokeSig
go build -o smokesig .
```

**Pre-commit hook:**
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/CosmoLabs-org/SmokeSig
    rev: v0.18.0
    hooks:
      - id: smokesig
```

## Quick Start

```bash
# 1. Generate a config for your project
smokesig init

# 2. Run all tests
smokesig run

# 3. Run only tagged tests
smokesig run --tag build

# 4. CI-friendly JSON output
smokesig run --format json

# 5. Watch mode — re-run on file changes
smokesig run --watch
```

## Example .smokesig.yaml

```yaml
version: 1
project: my-api
description: "Smoke tests for my-api"

settings:
  timeout: 30s
  fail_fast: true

prerequisites:
  - name: "Go installed"
    check: "go version"
    hint: "Install Go from https://go.dev"

tests:
  - name: "Compiles"
    run: "go build -o ./bin/api ./..."
    expect:
      exit_code: 0
    tags: [build]
    timeout: 60s
    cleanup: "rm -f ./bin/api"

  - name: "Health endpoint responds"
    run: "echo check"
    expect:
      http: { url: "http://localhost:8080/health", status_code: 200 }
    tags: [runtime]

  - name: "Redis is reachable"
    run: "echo check"
    expect:
      redis_ping: {}
    tags: [infra]

  - name: "API responds within 500ms"
    run: "echo check"
    expect:
      url_reachable: { url: "http://localhost:8080", timeout: 1s }
      response_time_ms: 500
    tags: [perf]

  - name: "Go version is 1.22+"
    run: "echo check"
    expect:
      version_check: { command: "go version", pattern: "go1\\.2[2-9]" }
    tags: [env]

  - name: "Config file exists"
    run: "echo check"
    expect:
      file_exists: "config.yaml"
    tags: [structure]

  - name: "Flaky external API"
    run: "curl -sf https://api.example.com/health"
    expect:
      exit_code: 0
    retry:
      count: 3
      backoff: 2s
    allow_failure: true
    skip_if:
      env_unset: "CI"
```

## Assertion Types

All assertions are optional and combinable within a single `expect` block.

### Process Assertions

| Type | Field | Description |
|------|-------|-------------|
| Exit code | `exit_code: <int>` | Exact process exit code match |
| Stdout substring | `stdout_contains: <string>` | Substring present in stdout |
| Stdout regex | `stdout_matches: <string>` | Go regex match against stdout |
| Stderr substring | `stderr_contains: <string>` | Substring present in stderr |
| Stderr regex | `stderr_matches: <string>` | Go regex match against stderr |
| Response time | `response_time_ms: <int>` | Test duration must not exceed threshold (ms) |

### File & Environment

| Type | Field | Description |
|------|-------|-------------|
| File exists | `file_exists: <path>` | Path exists relative to config file directory |
| Env variable | `env_exists: <string>` | Environment variable is set (non-empty) |

### Network Assertions

| Type | Field | Description |
|------|-------|-------------|
| Port listening | `port_listening: {port, protocol?, host?}` | TCP/UDP port is open |
| Process running | `process_running: <string>` | Named process is running (pgrep -x / tasklist) |
| HTTP check | `http: {url, method?, status_code?, body_contains?, body_matches?, header_contains?}` | Full HTTP endpoint validation |
| URL reachable | `url_reachable: {url, timeout?, status_code?}` | Lightweight HTTP/HTTPS connectivity check |
| Service reachable | `service_reachable: {url, timeout?}` | External service dependency check |
| SSL certificate | `ssl_cert: {host, port?, min_days_remaining?, allow_self_signed?}` | TLS cert validity + expiry threshold |
| gRPC health | `grpc_health: {address, service?, use_tls?, timeout?}` | grpc.health.v1 Health/Check returns SERVING |

### Database & Protocol

| Type | Field | Description |
|------|-------|-------------|
| Redis | `redis_ping: {host?, port?, password?}` | Redis PING returns +PONG |
| Memcached | `memcached_version: {host?, port?}` | Memcached `version` returns VERSION |
| PostgreSQL | `postgres_ping: {host?, port?}` | Postgres SSLRequest handshake valid |
| MySQL | `mysql_ping: {host?, port?}` | MySQL v10 handshake packet valid |
| MongoDB | `mongo_ping: {host?, port?, username?, password_env?}` | MongoDB isMaster wire protocol check |
| Kafka | `kafka_broker: {brokers, topic?, timeout?}` | Kafka metadata request wire protocol |
| LDAP | `ldap_bind: {host, port?, bind_dn?, password_env?, use_tls?, timeout?}` | LDAP bind request (ASN.1 BER) |
| MQTT | `mqtt_ping: {broker, client_id?, username?, password_env?, timeout?}` | MQTT CONNECT/CONNACK wire protocol |
| NTP | `ntp_check: {server?, max_offset_ms?, timeout?}` | NTP time sync verification (UDP) |

### Storage & Docker

| Type | Field | Description |
|------|-------|-------------|
| S3 bucket | `s3_bucket: {bucket, region?, endpoint?}` | S3-compatible bucket accessibility (anonymous HEAD) |
| Docker container | `docker_container_running: {name}` | Named Docker container is running |
| Docker image | `docker_image_exists: {image}` | Docker image exists locally |
| Docker Compose | `docker_compose_healthy: {compose_file?, services?, timeout?}` | Docker Compose service health |
| ICMP ping | `ping: {host, count?, timeout?}` | ICMP echo via system ping command |
| Kubernetes | `k8s_resource: {context?, namespace, kind, name, condition?, timeout?}` | K8s resource state via kubectl |

### Observability & APIs

| Type | Field | Description |
|------|-------|-------------|
| OpenTelemetry trace | `otel_trace: {jaeger_url, service_name?, min_spans?, timeout?}` | Trace verification with W3C traceparent propagation |
| GraphQL | `graphql: {url, query?, status_code?, expect_types?, expect_contains?}` | GraphQL introspection assertion |
| WebSocket | `websocket: {url, send?, expect_contains?, expect_matches?}` | WebSocket connect-send-expect |

### Mobile & Web

| Type | Field | Description |
|------|-------|-------------|
| Deep link | `deep_link: {url, android_package?, ios_bundle_id?}` | Mobile deep link / universal link verification |
| DNS resolve | `dns_resolve: {hostname, record_type?, expected_ip?}` | DNS resolution check (A, AAAA, TXT, MX, CNAME) |

### Messaging & Mail

| Type | Field | Description |
|------|-------|-------------|
| SMTP | `smtp_ping: {host, port?, timeout?}` | SMTP server connectivity + EHLO handshake |

### Tool Verification

| Type | Field | Description |
|------|-------|-------------|
| Version check | `version_check: {command, pattern}` | Shell command output matches regex |
| JSON field | `json_field: {path, equals?, contains?, matches?}` | JSONPath assertion on stdout |
| Credential check | `credential_check: {source, name, contains?}` | Verify credential accessible without leaking value |

### Test Modifiers

| Modifier | Description |
|----------|-------------|
| `allow_failure: true` | Test passes even if assertions fail (for flaky/optional checks) |
| `retry: {count, backoff}` | Retry flaky tests with exponential backoff |
| `skip_if: {env_unset, env_equals, file_missing}` | Conditionally skip a test |
| `tags: [...]` | Tag tests for selective runs |
| `timeout: <dur>` | Per-test timeout override |

## CLI Reference

```
smokesig run [flags]
  -f, --file string          Config file (default ".smokesig.yaml")
      --tag strings          Run only tests with these tags
      --exclude-tag strings  Skip tests with these tags
      --format string        Output format: terminal|json|junit|tap|prometheus (default "terminal")
      --fail-fast            Stop on first failure
      --timeout string       Per-test timeout override (e.g. "30s")
      --dry-run              List matching tests without running them
      --watch                Re-run tests on file changes

smokesig init [flags]
  -f, --force                Overwrite existing .smokesig.yaml

smokesig version
```

## Auto-Detection

`smokesig init` inspects the current directory and generates a starter config. Supports **31 project types** across languages, mobile, infrastructure, and static sites.

### Languages & Frameworks

| Marker file | Type | Tests generated |
|-------------|------|----------------|
| `go.mod` | Go | compile, test |
| `package.json` | Node (bun/npm) | install, lint (if script exists) |
| `pyproject.toml` / `requirements.txt` / `setup.py` | Python | import check |
| `Cargo.toml` | Rust | build, test |
| `pom.xml` | Java (Maven) | compile, test |
| `build.gradle` + `package.json` | Java (Gradle) | build, test |
| `*.csproj` / `*.sln` | .NET/C# | build, test |
| `Gemfile` | Ruby | bundle install, rake (if Rakefile) |
| `composer.json` | PHP | composer install, syntax lint |
| `deno.json` / `deno.jsonc` | Deno | type check, test |
| `build.sbt` | Scala | compile, test |
| `mix.exs` | Elixir | deps, compile, test |
| `Package.swift` (no xcodeproj) | Swift (server) | build, test |
| `pubspec.yaml` (no flutter) | Dart (server) | pub get, test |
| `build.zig` | Zig | build, test |
| `stack.yaml` / `*.cabal` | Haskell | build, test |
| `*.rockspec` | Lua | build (luarocks) |
| `Makefile` | C/C++ (Make) | make |
| `CMakeLists.txt` | C/C++ (CMake) | configure, build |

### Mobile

| Marker file | Type | Tests generated |
|-------------|------|----------------|
| `app.json` + `react-native` dep | React Native | deep link config |
| `pubspec.yaml` + `sdk: flutter` | Flutter | universal link config |
| `*.xcodeproj` / `*.xcworkspace` | iOS | universal link config |
| `build.gradle` (no go.mod/package.json) | Android | universal link config |

### Infrastructure & DevOps

| Marker file | Type | Tests generated |
|-------------|------|----------------|
| `Dockerfile` / `docker-compose.yml` | Docker | docker build |
| `*.tf` | Terraform | validate, fmt check |
| `Chart.yaml` | Helm | lint, template render |
| `kustomization.yaml` | Kustomize | render manifests |
| `serverless.yml` | Serverless | validate config |

### Static Sites & SSG

| Marker file | Type | Tests generated |
|-------------|------|----------------|
| `hugo.toml` / `hugo.yaml` / `config.toml` + `content/` | Hugo | site build |
| `astro.config.*` | Astro | type check, build |
| `_config.yml` + `Gemfile` | Jekyll | site build |

## CI/CD Integration

### GitHub Actions (Reusable Workflow)

Reference the reusable workflow from any repo:

```yaml
jobs:
  smoke:
    uses: CosmoLabs-org/SmokeSig/.github/workflows/smoke.yml@v1
    with:
      smoke-version: "latest"       # or pin: "v0.18.0"
      working-directory: "."         # dir containing .smokesig.yaml
      tags: "smoke"                  # optional tag filter
      fail-fast: true
```

Results are uploaded as a `smoke-results` artifact (JSON) on every run, even on failure.

### GitLab CI

```yaml
smoke:
  stage: test
  image: golang:1.23
  before_script:
    - go install github.com/CosmoLabs-org/SmokeSig@latest
  script:
    - smokesig run --format junit > smoke-junit.xml
    - smokesig run --format json > smoke-results.json
  artifacts:
    when: always
    reports:
      junit: smoke-junit.xml
    paths:
      - smoke-results.json
```

### Docker-based CI (Any Platform)

```dockerfile
FROM golang:1.23 AS smoke
RUN go install github.com/CosmoLabs-org/SmokeSig@latest
WORKDIR /app
COPY . .
CMD ["smokesig", "run", "--fail-fast"]
```

### Centralized Result Collection

Push results to a dashboard endpoint:

```bash
smokesig run --format json --report-url https://dashboard.example.com/api/results --report-api-key $API_KEY
```

### Exit Code Semantics

| Code | Meaning | CI Behavior |
|------|---------|-------------|
| `0` | All tests passed | Pipeline continues |
| `1` | One or more tests failed | Pipeline fails |
| `2` | Config error or invalid arguments | Pipeline fails |

### JUnit for CI Ingestion

```bash
smokesig run --format junit  # writes smoke-junit.xml
```

Most CI platforms (GitHub Actions, GitLab CI, Jenkins, CircleCI) natively ingest JUnit XML for test result visualization.

## Output Formats

`smokesig run --format X` supports: `terminal` (default), `json`, `junit`, `tap`, `prometheus`, `gha`.
Comma-separated for multiple: `--format terminal,json`. First format goes to stdout, rest to auto-named files.

## Migration from cosmo-smoke

SmokeSig was previously named `cosmo-smoke`. The migration is straightforward:

1. **Config file**: Rename `.smoke.yaml` to `.smokesig.yaml`. The old name still works but prints a deprecation warning.
2. **Binary**: Replace `smoke` with `smokesig` in scripts, CI configs, and aliases.
3. **Go import**: Change `github.com/CosmoLabs-org/cosmo-smoke` to `github.com/CosmoLabs-org/SmokeSig`.
4. **Pre-commit hook**: Change `id: smoke` to `id: smokesig`.

No assertion types, config structure, or subcommand names changed.

## License

MIT
