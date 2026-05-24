# SmokeSig Specification

Technical specification for SmokeSig, the universal smoke test runner. This document covers the `.smokesig.yaml` configuration format, all 45 assertion types, CLI commands, output formats, and runtime behavior.

---

## Overview

SmokeSig is a standalone Go binary that reads `.smokesig.yaml` and runs lightweight smoke tests. It is designed for rapid infrastructure validation across diverse project types --- verifying that services compile, start, respond, and connect to their dependencies.

Key design principles:

- **Config-driven**: All tests are declared in YAML. No test code to write.
- **Pure assertions**: All assertion types are side-effect-free functions.
- **All errors at once**: Validation reports every error, not just the first.
- **Config-dir-relative**: Commands execute from the config file's directory, not the working directory.
- **Minimal dependencies**: Cobra, Lipgloss, yaml.v3, gjson. No Viper, no Bubbletea.

---

## Configuration Format

The configuration file is `.smokesig.yaml`, placed at the project root. Legacy `.smoke.yaml` is accepted with a deprecation warning.

Go template syntax is supported: `{{ .Env.VAR_NAME }}` expands environment variables at load time.

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | integer | yes | Schema version. Must be `1`. |
| `project` | string | yes | Project name. Used in output headers and telemetry. |
| `description` | string | no | Human-readable description. |
| `extends` | string | no | URL to a remote base config (http, https, or file scheme). Local config overlays the remote. |
| `includes` | []string | no | List of local YAML files to merge. Paths are relative to the config file. Max depth: 10. |
| `settings` | Settings | no | Global defaults for test behavior. |
| `otel` | OTelConfig | no | OpenTelemetry trace propagation and export. |
| `prerequisites` | []Prerequisite | no | Commands that must pass before any tests run. |
| `lifecycle` | LifecycleConfig | no | Setup/teardown hooks (before_all, after_all, before_each, after_each). |
| `tests` | []Test | yes | List of smoke tests. At least one is required. |

**Minimal valid config:**

```yaml
version: 1
project: my-app
tests:
  - name: "Starts"
    run: "./my-app --help"
    expect:
      exit_code: 0
```

---

### Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `30s` | Default timeout per test. Overridable per test and via CLI. |
| `fail_fast` | bool | `false` | Stop after the first test failure. |
| `parallel` | bool | `false` | Run tests concurrently. |
| `monorepo` | bool | `false` | Auto-discover `.smokesig.yaml` in subdirectories. |
| `monorepo_exclude` | []string | `[]` | Directory patterns to skip during monorepo discovery. |

```yaml
settings:
  timeout: 30s
  fail_fast: true
  parallel: false
  monorepo: false
  monorepo_exclude: [node_modules, vendor]
```

**Duration format:** Go duration strings --- `30s`, `2m`, `1m30s`, `500ms`.

---

### Prerequisites

Prerequisites run sequentially before any tests. If any check fails, the entire run aborts.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Human-readable name shown in output. |
| `check` | string | yes | Shell command. Must exit `0` to pass. |
| `hint` | string | no | Help message shown on failure (e.g., install instructions). |

```yaml
prerequisites:
  - name: "Docker running"
    check: "docker info"
    hint: "Start Docker Desktop before running these tests"

  - name: "Go 1.21+"
    check: "go version"
    hint: "Install Go from https://go.dev"
```

---

### Test Schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Human-readable test name. |
| `run` | string | conditional | Shell command to execute. Not required if the test uses only standalone assertions (network/storage checks). |
| `expect` | Expect | yes | One or more assertions. All must pass. |
| `tags` | []string | no | Labels for filtering with `--tag` / `--exclude-tag`. |
| `timeout` | duration | no | Per-test timeout. Overrides `settings.timeout`. |
| `cleanup` | string | no | Shell command run after the test, pass or fail. Exit code ignored. |
| `allow_failure` | bool | no | If `true`, test failure does not fail the suite. Reported as allowed failure. |
| `retry` | RetryPolicy | no | Automatic retry configuration for flaky tests. |
| `skip_if` | SkipIf | no | Conditions under which the test is skipped. |

```yaml
tests:
  - name: "Builds binary"
    run: "go build -o ./bin/app ./..."
    expect:
      exit_code: 0
    tags: [build]
    timeout: 120s
    cleanup: "rm -f ./bin/app"
    allow_failure: false
```

---

### Retry Policy

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `count` | int | yes | Number of retry attempts. Must be >= 1. |
| `backoff` | duration | yes | Delay between retries. Exponential backoff applied. Must be > 0. |
| `retry_on_trace_only` | bool | no | Skip retry if the `otel_trace` assertion already confirmed the trace was received. Requires `otel_trace` in the same test. |

```yaml
tests:
  - name: "Flaky endpoint"
    run: "curl -f http://localhost:8080/health"
    expect:
      exit_code: 0
    retry:
      count: 3
      backoff: 2s
```

---

### Skip Conditions

| Field | Type | Description |
|-------|------|-------------|
| `env_unset` | string | Skip if this environment variable is not set. |
| `env_equals` | object | Skip if env var matches a specific value. Fields: `var`, `value`. |
| `file_missing` | string | Skip if this file does not exist (relative to config dir). |

```yaml
tests:
  - name: "Integration test"
    run: "make integration"
    expect:
      exit_code: 0
    skip_if:
      env_unset: "CI"
```

---

### Lifecycle Hooks

Hooks run at defined lifecycle points around test execution.

| Block | When it runs |
|-------|-------------|
| `before_all` | Once before the entire test suite. |
| `after_all` | Once after the entire test suite. |
| `before_each` | Before every individual test. |
| `after_each` | After every individual test. |

Each hook is an object with:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | yes | Shell command to execute. |
| `timeout` | duration | no | Command timeout. |
| `always_run` | bool | no | Run even if previous hooks or tests failed. |
| `env_pass` | bool | no | Pass environment variables from the hook to subsequent tests. |
| `background` | bool | no | Run command in the background (e.g., start a server). Requires `wait_for_port` or a timeout. |
| `wait_for_port` | int | no | When `background: true`, wait until this port is listening before continuing. |
| `startup_timeout` | duration | no | Maximum time to wait for a background hook to become ready. |

```yaml
lifecycle:
  before_all:
    - command: "docker compose up -d"
      background: true
      wait_for_port: 5432
      startup_timeout: 30s
    - command: "make migrate"
      timeout: 10s
  after_all:
    - command: "docker compose down"
      always_run: true
  before_each:
    - command: "make seed-db"
  after_each:
    - command: "make clean-db"
```

---

### Config Inheritance

#### `includes`

Merge local YAML files into the current config. Included prerequisites are prepended (run first). Included tests are prepended. Paths are relative to the config file's directory. Maximum include depth: 10 (circular include detection).

```yaml
includes:
  - shared/common-tests.yaml
  - shared/db-prereqs.yaml
```

#### `extends`

Inherit from a remote base config via URL (http, https, or file scheme). The local config is overlaid onto the remote base --- local values take precedence. Remote configs are cached locally with ETag/Last-Modified conditional requests. Cache directory: `~/.cache/smokesig/` (or OS cache dir). Maximum remote size: 10 MB.

```yaml
extends: "https://raw.githubusercontent.com/myorg/smoke-templates/main/golang.yaml"
```

#### Environment-specific overrides

The `--env` CLI flag loads `<name>.smokesig.yaml` from the same directory and deep-merges it onto the base config. Environment settings override base settings. Environment prerequisites are prepended. Environment tests are appended.

```bash
smokesig run --env staging
# Loads .smokesig.yaml, then merges staging.smokesig.yaml on top
```

---

## Assertion Types

All assertion fields live under the `expect:` block. All fields are optional, but at least one must be present. When multiple assertions are specified, all must pass for the test to pass.

Assertions fall into two categories:
- **Command assertions** require a `run` command (they check stdout, stderr, exit code, or timing).
- **Standalone assertions** do not require `run` (they check external state like ports, HTTP endpoints, databases). When a test has only standalone assertions, the `run` field is optional.

---

### 1. exit_code

Match the process exit code.

| Field | Type | Required |
|-------|------|----------|
| `exit_code` | integer (pointer) | --- |

Omitting `exit_code` means the exit code is not checked. The value `0` asserts clean exit; any other value asserts a specific failure code.

```yaml
expect:
  exit_code: 0
```

```yaml
expect:
  exit_code: 1  # must exit with error
```

---

### 2. stdout_contains

Case-sensitive substring match on stdout.

| Field | Type | Required |
|-------|------|----------|
| `stdout_contains` | string | --- |

```yaml
expect:
  stdout_contains: "Server started on port 8080"
```

---

### 3. stdout_matches

Regular expression match on stdout. Full Go regex syntax. Matched against the complete stdout string.

| Field | Type | Required |
|-------|------|----------|
| `stdout_matches` | string | --- |

```yaml
expect:
  stdout_matches: "^v[0-9]+\\.[0-9]+\\.[0-9]+"
```

Use `(?i)` for case-insensitive matching.

---

### 4. stderr_contains

Case-sensitive substring match on stderr.

| Field | Type | Required |
|-------|------|----------|
| `stderr_contains` | string | --- |

```yaml
expect:
  stderr_contains: "flag provided but not defined"
```

---

### 5. stderr_matches

Regular expression match on stderr. Full Go regex syntax.

| Field | Type | Required |
|-------|------|----------|
| `stderr_matches` | string | --- |

```yaml
expect:
  stderr_matches: "WARN.*deprecated"
```

---

### 6. file_exists

Check that a file or directory exists, relative to the config file's directory.

| Field | Type | Required |
|-------|------|----------|
| `file_exists` | string | --- |

```yaml
expect:
  file_exists: "dist/index.html"
```

---

### 7. file_size

Check that a file exists and optionally verify its size falls within a range.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | yes | File path relative to config dir. |
| `min_bytes` | integer | no | Minimum file size in bytes. |
| `max_bytes` | integer | no | Maximum file size in bytes. Must be >= `min_bytes`. |

```yaml
expect:
  file_size:
    path: "build/bundle.js"
    min_bytes: 1024
    max_bytes: 5242880  # 5 MB
```

---

### 8. env_exists

Check that an environment variable is set (non-empty).

| Field | Type | Required |
|-------|------|----------|
| `env_exists` | string | --- |

```yaml
expect:
  env_exists: "DATABASE_URL"
```

---

### 9. port_listening

Check that a TCP or UDP port is open and listening.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `port` | int | yes | --- | Port number. |
| `protocol` | string | no | `tcp` | Protocol: `tcp` or `udp`. |
| `host` | string | no | `localhost` | Host to check. |

```yaml
expect:
  port_listening:
    port: 5432
    host: "localhost"
    protocol: "tcp"
```

---

### 10. process_running

Check that a named process is currently running. Uses `pgrep -x` on Unix, `tasklist` on Windows.

| Field | Type | Required |
|-------|------|----------|
| `process_running` | string | --- |

```yaml
expect:
  process_running: "nginx"
```

---

### 11. http

HTTP endpoint assertion. Sends a request and validates the response.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | yes | --- | Full URL to request. |
| `method` | string | no | `GET` | HTTP method. |
| `headers` | map[string]string | no | --- | Request headers. |
| `body` | string | no | --- | Request body. |
| `timeout` | duration | no | `10s` | HTTP client timeout. |
| `status_code` | integer | no | --- | Expected HTTP status code. |
| `body_contains` | string | no | --- | Response body must contain this substring. |
| `body_matches` | string | no | --- | Response body must match this regex. |
| `header_contains` | map[string]string | no | --- | Response headers must contain these key-value pairs. |

When OTel is enabled, W3C `traceparent` headers are auto-injected.

```yaml
expect:
  http:
    url: "http://localhost:8080/api/health"
    method: "GET"
    status_code: 200
    body_contains: "\"status\":\"ok\""
    header_contains:
      content-type: "application/json"
```

---

### 12. json_field

Assert on a specific JSON field in stdout using gjson path syntax.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | yes | gjson path expression (e.g., `data.items.#`, `users.0.name`). |
| `equals` | string | no | Field value must equal this string. |
| `contains` | string | no | Field value must contain this substring. |
| `matches` | string | no | Field value must match this regex. |
| `extract` | string | no | Variable name to capture the matched value for use in later tests. |

```yaml
- name: "Check API response"
  run: "curl -s http://localhost:8080/api/info"
  expect:
    json_field:
      path: "version"
      matches: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
```

---

### 13. response_time_ms

Fail if the test duration exceeds this threshold in milliseconds.

| Field | Type | Required |
|-------|------|----------|
| `response_time_ms` | integer (pointer) | --- |

```yaml
expect:
  exit_code: 0
  response_time_ms: 5000  # must complete within 5 seconds
```

---

### 14. ssl_cert

Validate a TLS certificate and check expiry.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | yes | --- | Hostname to connect to. |
| `port` | int | no | `443` | TLS port. |
| `min_days_remaining` | int | no | `0` | Minimum days until cert expires. `0` means any non-expired cert passes. |
| `allow_self_signed` | bool | no | `false` | Accept self-signed certificates. |

```yaml
expect:
  ssl_cert:
    host: "example.com"
    min_days_remaining: 30
```

---

### 15. redis_ping

Ping a Redis server using the RESP protocol. Sends `PING` and expects `+PONG`.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | no | `localhost` | Redis host. |
| `port` | int | no | `6379` | Redis port. |
| `password` | string | no | --- | AUTH password. |

```yaml
expect:
  redis_ping:
    host: "localhost"
    port: 6379
```

---

### 16. memcached_version

Connect to a Memcached server and send the `version` command. Expects a `VERSION` reply.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | no | `localhost` | Memcached host. |
| `port` | int | no | `11211` | Memcached port. |

```yaml
expect:
  memcached_version:
    host: "localhost"
    port: 11211
```

---

### 17. postgres_ping

Ping a PostgreSQL server via SSLRequest handshake. Connects and validates the protocol response byte.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | no | `localhost` | Postgres host. |
| `port` | int | no | `5432` | Postgres port. |

```yaml
expect:
  postgres_ping:
    host: "localhost"
    port: 5432
```

---

### 18. mysql_ping

Connect to a MySQL server and verify it sends a valid v10 handshake packet.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | no | `localhost` | MySQL host. |
| `port` | int | no | `3306` | MySQL port. |

```yaml
expect:
  mysql_ping:
    host: "localhost"
    port: 3306
```

---

### 19. grpc_health

Query the `grpc.health.v1.Health/Check` endpoint and expect `SERVING` status. Requires building with `-tags grpc`.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `address` | string | yes | --- | gRPC `host:port`. |
| `service` | string | no | `""` | Service name. Empty string checks overall server health. |
| `use_tls` | bool | no | `false` | Use TLS for the connection. |
| `timeout` | duration | no | `5s` | gRPC call timeout. |

When OTel is enabled, trace metadata is injected into gRPC calls.

```yaml
expect:
  grpc_health:
    address: "localhost:50051"
    service: "my.service.v1"
    timeout: 5s
```

---

### 20. websocket

Connect to a WebSocket endpoint, optionally send a message, and validate the response. Uses stdlib-only WebSocket client (no external dependencies).

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | yes | --- | WebSocket URL. Must start with `ws://` or `wss://`. |
| `send` | string | no | --- | Message to send after connection. |
| `expect_contains` | string | no | --- | Response must contain this substring. |
| `expect_matches` | string | no | --- | Response must match this regex. |
| `timeout` | duration | no | `10s` | Connection and read timeout. |
| `headers` | map[string]string | no | --- | Additional headers for the upgrade request. |

When OTel is enabled, W3C `traceparent` headers are auto-injected into the upgrade request.

```yaml
expect:
  websocket:
    url: "ws://localhost:8080/ws"
    send: '{"type":"ping"}'
    expect_contains: "pong"
    timeout: 5s
```

---

### 21. docker_container_running

Check that a named Docker container is currently running.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Container name (exact match). |

```yaml
expect:
  docker_container_running:
    name: "my-api-server"
```

---

### 22. docker_image_exists

Check that a Docker image exists locally.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image` | string | yes | Image name (with optional tag). |

```yaml
expect:
  docker_image_exists:
    image: "my-app:latest"
```

---

### 23. url_reachable

Verify an HTTP/HTTPS endpoint is accessible. Simpler than the full `http` assertion --- designed for connectivity checks.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | yes | --- | URL to check. Must start with `http://` or `https://`. |
| `timeout` | duration | no | `10s` | Connection timeout. |
| `status_code` | integer | no | --- | Expected HTTP status code. |

```yaml
expect:
  url_reachable:
    url: "https://api.example.com/health"
    timeout: 5s
    status_code: 200
```

---

### 24. service_reachable

Check that an external service dependency is accessible. Similar to `url_reachable` but semantically indicates an external dependency.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | yes | --- | Service URL. Must start with `http://` or `https://`. |
| `timeout` | duration | no | `10s` | Connection timeout. |

```yaml
expect:
  service_reachable:
    url: "https://auth.provider.com/.well-known/openid-configuration"
    timeout: 10s
```

---

### 25. s3_bucket

Check that an S3-compatible bucket is accessible via anonymous HEAD request.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `bucket` | string | yes | --- | Bucket name. |
| `region` | string | no | `us-east-1` | AWS region. |
| `endpoint` | string | no | --- | Custom endpoint for S3-compatible services (MinIO, etc.). |

```yaml
expect:
  s3_bucket:
    bucket: "my-assets"
    region: "eu-west-1"
```

```yaml
expect:
  s3_bucket:
    bucket: "local-data"
    endpoint: "http://localhost:9000"  # MinIO
```

---

### 26. version_check

Verify an installed tool's version matches a regex pattern. Runs the command and matches stdout against the pattern.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | yes | Shell command that prints a version string (e.g., `node --version`). |
| `pattern` | string | yes | Go regex pattern to match against stdout. Must be valid regex. |

```yaml
expect:
  version_check:
    command: "node --version"
    pattern: "^v(1[8-9]|[2-9][0-9])\\."  # Node 18+
```

---

### 27. otel_trace

Verify that a distributed trace arrived at a trace collector. Supports multiple backends. When OTel is enabled globally, W3C `traceparent` headers are auto-injected into HTTP, gRPC, and WebSocket assertions.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `backend` | string | no | `jaeger` | Trace backend: `jaeger`, `tempo`, `honeycomb`, or `datadog`. |
| `jaeger_url` | string | conditional | --- | Collector URL. Required unless `otel.jaeger_url` is set globally. Must start with `http://` or `https://`. |
| `service_name` | string | no | project name | Service name to search for in traces. |
| `min_spans` | int | no | `0` | Minimum number of spans expected. Must be >= 0. |
| `timeout` | duration | no | `10s` | How long to wait for the trace to arrive. |
| `api_key` | string | conditional | --- | API key. Required for `honeycomb` and `datadog` backends. |
| `dd_app_key` | string | no | --- | Datadog application key (optional, for enhanced queries). |

```yaml
expect:
  otel_trace:
    backend: "jaeger"
    jaeger_url: "http://localhost:16686"
    service_name: "my-api"
    min_spans: 3
    timeout: 15s
```

---

### 28. credential_check

Verify a credential is accessible without leaking its value in output.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | yes | Source type: `env` (environment variable), `file` (file path), or `exec` (shell command). |
| `name` | string | yes | The env var name, file path, or command to execute. |
| `contains` | string | no | Credential value must contain this substring. |

```yaml
expect:
  credential_check:
    source: "env"
    name: "API_KEY"
```

```yaml
expect:
  credential_check:
    source: "file"
    name: "/run/secrets/db-password"
    contains: "postgres"
```

---

### 29. graphql

Verify a GraphQL endpoint is introspectable and/or returns expected data.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | yes | --- | GraphQL endpoint URL. |
| `query` | string | no | introspection query | Custom GraphQL query. Defaults to full introspection. |
| `status_code` | integer | no | `200` | Expected HTTP status code. |
| `expect_types` | []string | no | --- | Type names that must exist in the schema. |
| `expect_contains` | string | no | --- | Response body must contain this substring. |
| `timeout` | duration | no | `10s` | HTTP client timeout. |

```yaml
expect:
  graphql:
    url: "http://localhost:4000/graphql"
    expect_types: ["User", "Post", "Query"]
```

```yaml
expect:
  graphql:
    url: "http://localhost:4000/graphql"
    query: "{ __typename }"
    expect_contains: "Query"
```

---

### 30. deep_link

Verify mobile deep link and universal link configuration. Two-tier system: Tier 1 uses HTTP/config checks (zero dependencies). Tier 2 uses `adb`/`xcrun` when available for full device resolution.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | yes | --- | Deep link URL to verify. |
| `android_package` | string | no | --- | Expected Android package name. |
| `ios_bundle_id` | string | no | --- | Expected iOS bundle ID. |
| `ios_associated_domains` | []string | no | --- | Expected Associated Domains entries. |
| `check_assetlinks` | bool | no | `true` | Verify Android `assetlinks.json` at `/.well-known/`. |
| `check_aasa` | bool | no | `true` | Verify Apple `apple-app-site-association` file. |
| `tier` | string | no | `auto` | Resolution tier: `auto`, `config-only`, or `full-resolve`. |

```yaml
expect:
  deep_link:
    url: "https://app.example.com/invite/123"
    android_package: "com.example.app"
    ios_bundle_id: "com.example.app"
    check_assetlinks: true
    check_aasa: true
```

---

### 31. dns_resolve

Verify DNS resolution for a hostname.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `hostname` | string | yes | --- | Hostname to resolve. |
| `record_type` | string | no | `A` | DNS record type: `A`, `AAAA`, `TXT`, `MX`, `CNAME`. |
| `expected_ip` | string | no | --- | Expected IP address in the response. |
| `timeout` | duration | no | `5s` | DNS query timeout. |

```yaml
expect:
  dns_resolve:
    hostname: "api.example.com"
    record_type: "A"
    expected_ip: "203.0.113.10"
```

---

### 32. smtp_ping

Verify an SMTP server is accepting connections. Connects and performs an EHLO handshake.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | yes | --- | SMTP server hostname. |
| `port` | int | no | `25` | SMTP port. |
| `timeout` | duration | no | `10s` | Connection timeout. |

```yaml
expect:
  smtp_ping:
    host: "smtp.example.com"
    port: 587
```

---

### 33. docker_compose_healthy

Verify Docker Compose services are running and healthy.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `compose_file` | string | no | `docker-compose.yml` | Path to compose file relative to config dir. |
| `services` | []string | no | all | Specific services to check. Empty means all. |
| `timeout` | duration | no | `30s` | How long to wait for health checks. |

```yaml
expect:
  docker_compose_healthy:
    compose_file: "docker-compose.yml"
    services: ["api", "db", "redis"]
    timeout: 60s
```

---

### 34. ping

Verify a host responds to ICMP echo requests via the system `ping` command.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | yes | --- | Hostname or IP to ping. |
| `count` | int | no | `1` | Number of echo requests. |
| `timeout` | duration | no | `10s` | Ping timeout. |

```yaml
expect:
  ping:
    host: "192.168.1.1"
    count: 3
    timeout: 5s
```

---

### 35. mongo_ping

Verify a MongoDB server responds to the `isMaster` wire protocol command.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | no | `localhost` | MongoDB host. |
| `port` | int | no | `27017` | MongoDB port. |
| `username` | string | no | --- | Authentication username. |
| `password_env` | string | no | --- | Environment variable containing the password. Value is never logged. |

```yaml
expect:
  mongo_ping:
    host: "localhost"
    port: 27017
```

---

### 36. kafka_broker

Verify a Kafka broker responds to a metadata request using the wire protocol.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `brokers` | []string | yes | --- | List of broker addresses (`host:port`). |
| `topic` | string | no | --- | Specific topic to query metadata for. |
| `timeout` | duration | no | `10s` | Connection timeout. |

```yaml
expect:
  kafka_broker:
    brokers: ["localhost:9092"]
    topic: "events"
    timeout: 5s
```

---

### 37. ldap_bind

Verify an LDAP server accepts bind requests. Uses ASN.1 BER encoding for the wire protocol.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | yes | --- | LDAP server hostname. |
| `port` | int | no | `389` | LDAP port (389 for plain, 636 for TLS). |
| `bind_dn` | string | no | --- | Distinguished name for bind. Empty for anonymous bind. |
| `password_env` | string | no | --- | Environment variable containing the bind password. |
| `use_tls` | bool | no | `false` | Use TLS (LDAPS). |
| `timeout` | duration | no | `10s` | Connection timeout. |

```yaml
expect:
  ldap_bind:
    host: "ldap.example.com"
    port: 636
    bind_dn: "cn=readonly,dc=example,dc=com"
    password_env: "LDAP_PASSWORD"
    use_tls: true
```

---

### 38. mqtt_ping

Verify an MQTT broker accepts connections. Sends CONNECT and expects CONNACK using the MQTT wire protocol.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `broker` | string | yes | --- | Broker address (`host:port`). |
| `client_id` | string | no | auto-generated | MQTT client ID. |
| `username` | string | no | --- | Authentication username. |
| `password_env` | string | no | --- | Environment variable containing the password. |
| `timeout` | duration | no | `10s` | Connection timeout. |

```yaml
expect:
  mqtt_ping:
    broker: "localhost:1883"
    client_id: "smokesig-test"
```

---

### 39. ntp_check

Verify an NTP server responds with valid time data and check clock offset.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `server` | string | no | `pool.ntp.org` | NTP server hostname. |
| `max_offset_ms` | int | no | --- | Maximum acceptable clock offset in milliseconds. |
| `timeout` | duration | no | `5s` | UDP timeout. |

```yaml
expect:
  ntp_check:
    server: "time.google.com"
    max_offset_ms: 100
```

---

### 40. k8s_resource

Verify a Kubernetes resource exists and optionally meets a condition. Uses `kubectl` under the hood.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `context` | string | no | current context | Kubernetes context name. |
| `namespace` | string | yes | --- | Kubernetes namespace. |
| `kind` | string | yes | --- | Resource kind (e.g., `Deployment`, `Pod`, `Service`). |
| `name` | string | yes | --- | Resource name. |
| `condition` | string | no | --- | kubectl wait condition (e.g., `Available`, `Ready`). |
| `timeout` | duration | no | `30s` | kubectl wait timeout. |

```yaml
expect:
  k8s_resource:
    namespace: "production"
    kind: "Deployment"
    name: "api-server"
    condition: "Available"
    timeout: 60s
```

---

### 41. ios_simulator

Check if an iOS simulator is booted. Runs `xcrun simctl list devices -j`, parses the JSON output, and filters by device name and OS version. Standalone assertion --- does not require `run`.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `device_name` | string | no | --- | Filter by simulator device name (e.g., `iPhone 15 Pro`). |
| `os` | string | no | --- | Filter by iOS version (e.g., `17.0`). |
| `timeout` | duration | no | `10s` | Timeout for the `xcrun simctl` command. |

```yaml
expect:
  ios_simulator:
    device_name: "iPhone 15 Pro"
    os: "17.0"
    timeout: 15s
```

---

### 42. android_emulator

Check if an Android emulator has finished booting. Runs `adb shell getprop sys.boot_completed` and verifies the output is `1`. Standalone assertion --- does not require `run`.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `serial` | string | no | --- | ADB serial for a specific device (e.g., `emulator-5554`). When omitted, uses the default connected device. |
| `timeout` | duration | no | `10s` | Timeout for the `adb` command. |

```yaml
expect:
  android_emulator:
    serial: "emulator-5554"
    timeout: 10s
```

---

### 43. doc_integrity

Check if CLI documentation is in sync with actual commands and flags. Runs `binary --help` to discover subcommands, `binary <cmd> --help` for each subcommand's flags, then parses the specified markdown documentation files for references. Reports: undocumented commands, stale doc references, undocumented flags, and failed doc examples. Standalone assertion --- does not require `run`.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `binary` | string | yes | --- | Path to the CLI binary to inspect. |
| `docs` | []string | yes | --- | List of documentation files to scan for command/flag references. |
| `check_examples` | bool | no | `false` | Run fenced code blocks from docs as shell commands and verify they succeed. |
| `ignore_commands` | []string | no | `[]` | Subcommands to exclude from the sync check (e.g., `help`, `completion`). |
| `timeout` | duration | no | `30s` | Timeout for binary inspection and example execution. |

```yaml
expect:
  doc_integrity:
    binary: ./bin/myapp
    docs:
      - README.md
      - CLAUDE.md
    check_examples: true
    ignore_commands:
      - help
      - completion
```

---

### Variable Extraction

The top-level `extract` field on `expect` captures a value from `stdout_matches` for use in subsequent tests.

| Field | Type | Description |
|-------|------|-------------|
| `extract` | string | Variable name to capture from the first match group of `stdout_matches`. |

```yaml
- name: "Get token"
  run: "curl -s http://localhost:8080/token"
  expect:
    stdout_matches: "token: (.+)"
    extract: "AUTH_TOKEN"
```

The `json_field` assertion also supports its own `extract` field to capture a specific JSON value.

---

### Combining Assertions

Multiple assertions in a single `expect` block must all pass:

```yaml
expect:
  exit_code: 0
  stdout_contains: "OK"
  response_time_ms: 5000
```

---

## CLI Commands

### `smokesig run`

Execute smoke tests.

```
smokesig run [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f`, `--file` | string | `.smokesig.yaml` | Config file path. |
| `--tag` | []string | --- | Include only tests with these tags. Repeatable. |
| `--exclude-tag` | []string | --- | Exclude tests with these tags. Repeatable. |
| `--format` | string | `terminal` | Output format(s), comma-separated. |
| `--fail-fast` | bool | `false` | Stop on first failure. |
| `--timeout` | duration | --- | Per-test timeout override (overrides config and settings). |
| `--dry-run` | bool | `false` | List tests without running them. |
| `--watch` | bool | `false` | Re-run tests on file changes. 500ms debounce. Ctrl+C to exit. |
| `--env` | string | --- | Load environment-specific config overlay. |
| `--monorepo` | bool | `false` | Auto-discover `.smokesig.yaml` in subdirectories. |
| `--otel-collector` | string | --- | Override `otel.jaeger_url` and enable tracing. |
| `--no-otel` | bool | `false` | Disable OTel trace propagation for this run. |
| `--report-url` | string | --- | POST results to this URL after run. |
| `--report-api-key` | string | --- | API key for `--report-url` (sent as `X-API-Key` header). |
| `--baseline` | bool | `false` | Save and compare test timings against baseline. |
| `--baseline-threshold` | float | `50` | Regression threshold percentage. Flags if current > baseline * (1 + threshold/100). |

### `smokesig validate`

Validate config without running tests. Reports all errors at once.

```
smokesig validate [-f path]
```

### `smokesig init`

Auto-detect project type and generate a `.smokesig.yaml` configuration. Supports 31 project types.

```
smokesig init [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f`, `--force` | bool | `false` | Overwrite existing `.smokesig.yaml`. |
| `--from-running` | string | --- | Generate config by inspecting a running Docker container. |

**Detected project types:** Go, Node (bun/npm), Python, Rust, Java (Maven/Gradle), .NET/C#, Ruby, PHP, Deno, Scala, Elixir, Swift (server), Dart (server), Zig, Haskell, Lua, C/C++ (Make/CMake), React Native, Flutter, iOS, Android, Docker, Terraform, Helm, Kustomize, Serverless, Hugo, Astro, Jekyll.

### `smokesig schema`

Export the assertion type schema as structured JSON. Useful for editor integrations and tooling.

```
smokesig schema
```

### `smokesig serve`

Start an HTTP server that runs smoke tests on each request. Designed for container health probes.

```
smokesig serve [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-p`, `--port` | string | `8080` | Port to listen on. |
| `--path` | string | `/healthz` | Health endpoint path. |
| `-f`, `--file` | string | `.smokesig.yaml` | Config file path. |
| `--dashboard` | bool | `false` | Enable dashboard aggregation mode with SQLite storage. |
| `--api-key` | string | --- | API key for `POST /api/results` (checked via `X-API-Key` header). |
| `--db-path` | string | `smoke-dashboard.db` | SQLite database path for dashboard. |

**Health endpoint response:**
```json
{
  "status": "healthy",
  "tests": { "total": 6, "passed": 6, "failed": 0 },
  "duration_ms": 1234
}
```

Returns `200 OK` when all tests pass, `503 Service Unavailable` when any test fails.

### `smokesig stress`

Run a single test repeatedly to detect flakiness. Reports pass rate, timing distribution, and deduplicated errors.

```
smokesig stress <test-name> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--runs` | int | `50` | Total number of executions. |
| `--workers` | int | `1` | Concurrency (1 = sequential). |
| `--fail-fast` | bool | `false` | Stop on first failure. |
| `-f`, `--file` | string | `.smokesig.yaml` | Config file path. |
| `--format` | string | `terminal` | Output format. |

### `smokesig migrate goss`

Migrate a Goss YAML file to `.smokesig.yaml` format. Supports all Goss resource types with native assertion mapping where possible and command fallback for others.

```
smokesig migrate goss <input.yaml> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o`, `--output` | string | stdout | Output file path. |
| `--overwrite` | bool | `false` | Overwrite output file if it exists. |
| `--strict` | bool | `false` | Fail on any unmappable assertion. |
| `--stats` | bool | `false` | Print mapping statistics to stderr. |
| `--distro` | string | `deb` | Linux distro for package commands: `deb`, `rpm`, or `apk`. |

### `smokesig mcp`

Start an MCP (Model Context Protocol) server for Claude Desktop integration. Exposes smoke test operations as tools over stdio.

```
smokesig mcp
```

### `smokesig version`

Print the version string.

```
smokesig version
```

---

## Output Formats

Controlled via `--format`. Multiple formats can be comma-separated: `--format terminal,json,junit`. The first format writes to stdout; additional formats write to auto-named files.

| Format | File | Description |
|--------|------|-------------|
| `terminal` | stdout | Human-readable colored output (Lipgloss). Default. |
| `json` | `smoke-results.json` | Machine-readable JSON with full test details. |
| `junit` | `smoke-junit.xml` | JUnit XML compatible with CI systems (GitHub Actions, Jenkins, GitLab CI). |
| `tap` | `smoke-tap.txt` | Test Anything Protocol output. |
| `prometheus` | `smoke-metrics.prom` | Prometheus exposition format metrics. |
| `gha` | `$GITHUB_STEP_SUMMARY` | GitHub Actions markdown summary + `::error`/`::warning` workflow annotations. |
| `backstage` | `smoke-backstage.json` | Backstage entity annotation JSON for developer portal integration. |

### JUnit XML

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="smoke" tests="6" failures="1" time="2.300">
  <testsuite name="project-name" tests="6" failures="1" skipped="0" time="2.300">
    <testcase name="Compiles" time="0.800"/>
    <testcase name="CLI works" time="0.500">
      <failure message="stdout_contains: expected &quot;Usage&quot; not found">
        Expected: Usage
        Actual:   error: unknown command
      </failure>
    </testcase>
  </testsuite>
</testsuites>
```

### Push Reporter

When `--report-url` is set, results are POSTed as JSON to the specified URL after the run completes. The `--report-api-key` value is sent as an `X-API-Key` header.

---

## OpenTelemetry Integration

Configure OTel in the `otel` block:

```yaml
otel:
  enabled: true
  jaeger_url: "http://jaeger:16686"
  service_name: "my-api"
  trace_propagation: true
  export_url: "http://collector:4318/v1/traces"
  export_headers:
    Authorization: "Bearer token"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | bool | no | Enable OTel integration. |
| `jaeger_url` | string | conditional | Collector URL. Required when `enabled: true`. |
| `service_name` | string | no | Service name for traces. Defaults to `project`. |
| `trace_propagation` | bool | no | Auto-inject W3C `traceparent` headers into HTTP, gRPC, and WebSocket assertions. |
| `export_url` | string | no | OTLP HTTP endpoint for emitting test result telemetry. Defaults to `jaeger_url + /v1/traces`. |
| `export_headers` | map[string]string | no | Additional headers for OTLP export (e.g., authentication). |

**Behavior:**
- When `trace_propagation: true`, W3C `traceparent` headers are automatically injected into `http`, `grpc_health`, and `websocket` assertions.
- Test results are exported as OTLP spans when `export_url` is configured or when `jaeger_url` is set (auto-appends `/v1/traces`). Each test becomes a span with attributes for pass/fail status, duration, and assertion details.
- The `otel_trace` assertion verifies traces arrived at a collector backend.

---

## Watch Mode

`smokesig run --watch` keeps the process resident and re-runs tests when files change in the config directory.

- Uses `fsnotify` for file system events.
- 500ms debounce to coalesce rapid changes.
- Filters out chmod-only events.
- Ctrl+C (SIGINT/SIGTERM) exits cleanly.
- Config is reloaded on each run (picks up YAML changes).
- When OTel is enabled, tracks trace health across runs with a sliding window (last 10 runs). Alerts when trace health drops below 50%.

---

## Monorepo Support

Enable via `--monorepo` flag or `settings.monorepo: true`. Discovers `.smokesig.yaml` files in all subdirectories (unlimited depth).

```yaml
settings:
  monorepo: true
  monorepo_exclude: [node_modules, vendor, .git]
```

Each discovered config runs independently. Results are aggregated in the final report. The `monorepo_exclude` list specifies directory names to skip during discovery.

---

## Baseline Performance Tracking

`smokesig run --baseline` saves test timings and compares against previous runs.

- Baseline file: `.smokesig-baseline.json` in the config directory.
- On first run, saves current timings.
- On subsequent runs, compares current vs. saved and flags regressions.
- `--baseline-threshold 50` (default): flags tests that take >50% longer than baseline.
- New tests are reported separately.

---

## Execution Semantics

### Shell

All commands (`run`, `check`, `cleanup`, lifecycle hooks) are executed via `sh -c "<command>"`. Shell features (pipes, redirects, `&&`, `||`) are supported.

### Working Directory

All commands execute from the directory containing the config file, not the directory where `smokesig` was invoked.

### Timeout Resolution

Per-test timeout is resolved in this order (first wins):
1. `--timeout` CLI flag
2. `test.timeout` field
3. `settings.timeout` field
4. Binary default: `30s`

### Tag Filtering

- `--tag X`: Only run tests with tag `X`. Repeatable. Tests with no tags are excluded.
- `--exclude-tag X`: Skip tests with tag `X`. Repeatable.
- Both flags can be used together.

### Prerequisites

Prerequisites always run sequentially before tests. If any prerequisite fails, the entire run halts immediately. `fail_fast` only controls test execution behavior.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All tests passed (or `--dry-run` completed successfully). |
| `1` | One or more tests failed. |
| `2` | Config error, validation error, or invalid CLI arguments. |

---

## Full Example

```yaml
version: 1
project: my-api
description: "Smoke tests for the API service"

settings:
  timeout: 30s
  fail_fast: false
  parallel: false

otel:
  enabled: true
  jaeger_url: "http://localhost:16686"
  service_name: "my-api"
  trace_propagation: true

prerequisites:
  - name: "Go toolchain"
    check: "go version"
    hint: "Install Go 1.21+ from https://go.dev"

  - name: "No port conflict"
    check: "! lsof -i :8080 -t"
    hint: "Stop any process using port 8080"

lifecycle:
  before_all:
    - command: "docker compose up -d postgres redis"
      background: true
      wait_for_port: 5432
      startup_timeout: 30s
    - command: "make migrate"
  after_all:
    - command: "docker compose down"
      always_run: true

tests:
  - name: "Compiles"
    run: "go build -o /tmp/my-api ./..."
    expect:
      exit_code: 0
    tags: [build]
    timeout: 60s
    cleanup: "rm -f /tmp/my-api"

  - name: "Version flag"
    run: "/tmp/my-api --version"
    expect:
      exit_code: 0
      stdout_matches: "^v[0-9]+\\.[0-9]+\\.[0-9]+"

  - name: "Postgres accessible"
    expect:
      postgres_ping:
        host: "localhost"
        port: 5432

  - name: "Redis accessible"
    expect:
      redis_ping:
        host: "localhost"
        port: 6379

  - name: "Health endpoint"
    expect:
      http:
        url: "http://localhost:8080/health"
        status_code: 200
        body_contains: "\"status\":\"ok\""

  - name: "TLS cert valid"
    expect:
      ssl_cert:
        host: "api.example.com"
        min_days_remaining: 30

  - name: "DNS resolves"
    expect:
      dns_resolve:
        hostname: "api.example.com"
        record_type: "A"

  - name: "Flaky endpoint"
    expect:
      http:
        url: "http://localhost:8080/slow"
        status_code: 200
    retry:
      count: 3
      backoff: 2s
    allow_failure: true

  - name: "Traces arrive"
    expect:
      otel_trace:
        jaeger_url: "http://localhost:16686"
        service_name: "my-api"
        min_spans: 1
        timeout: 15s
```

---

## Build

```bash
# Standard build
go build -o smokesig .

# Release build with version injection
go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=X.Y.Z" -o smokesig .

# With gRPC support
go build -tags grpc -o smokesig .
```

---

## Test

```bash
go test ./...     # Full suite
smokesig run      # Self-smoke tests
```
