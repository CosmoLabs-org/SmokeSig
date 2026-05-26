# SmokeSig Features

Universal smoke test runner for any project, any language.

**Version**: 0.22.0 | **Status**: Stable | **License**: MIT

---

## Feature Status Legend

| Icon | Meaning |
|------|---------|
| ✅ | Implemented and stable |
| ⭐ | Key differentiator |

---

## Core Runner

| Feature | Status | Description |
|---------|--------|-------------|
| **YAML config** | ✅ | Single `.smokesig.yaml` file defines all tests |
| **45 assertion types** | ✅ ⭐ | Process, file, env, network, database, Docker, storage, tool verification, mobile, infrastructure, messaging |
| **Multiple assertions per test** | ✅ | All assertions in an `expect` block must pass |
| **Prerequisites** | ✅ | Pre-flight checks that abort the run if they fail |
| **Lifecycle hooks** | ✅ | `before_all`, `after_all`, `before_each`, `after_each` with background process support |
| **Per-test cleanup** | ✅ | `cleanup` command runs after each test regardless of pass/fail |
| **Per-test timeout** | ✅ | `timeout` field overrides the global default |
| **Global settings** | ✅ | `timeout`, `fail_fast`, `parallel` in `settings` block |
| **Tag filtering** | ✅ | `--tag` and `--exclude-tag` flags for selective runs |
| **Fail-fast mode** | ✅ | `--fail-fast` flag or `settings.fail_fast` stops on first failure |
| **Dry run** | ✅ | `--dry-run` lists matching tests without executing |
| **Parallel execution** | ✅ | `settings.parallel: true` runs tests concurrently |
| **Watch mode** | ✅ | `--watch` re-runs tests on file changes (fsnotify, 500ms debounce) |
| **Retry flaky tests** | ✅ | `retry: {count, backoff, retry_on_trace_only}` with exponential backoff |
| **Allow failure** | ✅ | `allow_failure: true` marks test as passing even on assertion failure |
| **Conditional execution** | ✅ | `skip_if: {env_unset, env_equals, file_missing}` to skip tests conditionally |
| **Variable extraction** | ✅ | `extract` field captures stdout matches into named variables |
| **Stress testing** | ✅ | `smokesig stress <test>` runs a single test N times to detect flakiness |

---

## Assertion Types

### Process & Output

| Type | Status | Description |
|------|--------|-------------|
| `exit_code` | ✅ | Exact process exit code match |
| `stdout_contains` | ✅ | Substring present in stdout |
| `stdout_matches` | ✅ | Go regex match against stdout |
| `stderr_contains` | ✅ | Substring present in stderr |
| `stderr_matches` | ✅ | Go regex match against stderr |
| `response_time_ms` | ✅ | Test duration must not exceed threshold (ms) |
| `json_field` | ✅ | JSONPath assertion on stdout (equals/contains/matches/extract) |

### File & Environment

| Type | Status | Description |
|------|--------|-------------|
| `file_exists` | ✅ | Path exists relative to config file directory |
| `file_size` | ✅ | File exists with optional min/max byte size thresholds |
| `env_exists` | ✅ | Environment variable is set (non-empty) |
| `credential_check` | ✅ | Credential accessible without leaking value (env\|file\|exec) |

### Network & Connectivity

| Type | Status | Description |
|------|--------|-------------|
| `port_listening` | ✅ | TCP/UDP port is open |
| `process_running` | ✅ | Named process is running (pgrep -x / tasklist) |
| `http` | ✅ ⭐ | Full HTTP endpoint validation (method, status, body, headers, timeout) |
| `url_reachable` | ✅ | Lightweight HTTP/HTTPS connectivity check |
| `service_reachable` | ✅ | External service dependency check (semantic naming) |
| `ssl_cert` | ✅ | TLS cert validity + expiry threshold + self-signed option |
| `websocket` | ✅ | Connect/send/expect pattern for real-time apps (custom headers) |
| `dns_resolve` | ✅ | DNS resolution check (A, AAAA, TXT, MX, CNAME record types) |
| `smtp_ping` | ✅ | SMTP server connectivity + EHLO handshake |
| `ping` | ✅ | ICMP echo via system ping command (configurable count) |

### Database & Protocol

| Type | Status | Description |
|------|--------|-------------|
| `redis_ping` | ✅ | Redis PING returns +PONG (RESP protocol, optional AUTH) |
| `memcached_version` | ✅ | Memcached `version` returns VERSION |
| `postgres_ping` | ✅ | Postgres SSLRequest handshake valid |
| `mysql_ping` | ✅ | MySQL v10 handshake packet valid |
| `mongo_ping` | ✅ | MongoDB isMaster wire protocol check (optional auth) |
| `grpc_health` | ✅ | grpc.health.v1 Health/Check returns SERVING (build tag: `-tags grpc`) |

### Messaging & Infrastructure

| Type | Status | Description |
|------|--------|-------------|
| `kafka_broker` | ✅ | Kafka metadata request wire protocol check |
| `mqtt_ping` | ✅ | MQTT CONNECT/CONNACK wire protocol check |
| `ldap_bind` | ✅ | LDAP bind request (ASN.1 BER, optional TLS) |
| `ntp_check` | ✅ | NTP time sync verification (configurable max offset) |
| `k8s_resource` | ✅ | Kubernetes resource state via kubectl (context, namespace, kind, condition) |

### Storage & Docker

| Type | Status | Description |
|------|--------|-------------|
| `s3_bucket` | ✅ | S3-compatible bucket accessibility (anonymous HEAD) |
| `docker_container_running` | ✅ | Named Docker container is running |
| `docker_image_exists` | ✅ | Docker image exists locally |
| `docker_compose_healthy` | ✅ | Docker Compose service health check (custom compose file, service filter) |

### Observability & API

| Type | Status | Description |
|------|--------|-------------|
| `otel_trace` | ✅ ⭐ | Trace verification with W3C traceparent (Jaeger/Tempo/Honeycomb/Datadog) |
| `graphql` | ✅ | GraphQL introspection assertion (schema types, custom queries) |
| `version_check` | ✅ | Shell command output matches regex |

### Mobile

| Type | Status | Description |
|------|--------|-------------|
| `deep_link` | ✅ ⭐ | Mobile deep link / universal link verification (Android assetlinks, iOS AASA, two-tier resolution) |
| `ios_simulator` | ✅ | Check if an iOS simulator is booted (xcrun simctl, filter by device name and OS) |
| `android_emulator` | ✅ | Check if an Android emulator has finished booting (adb sys.boot_completed) |

### Documentation & Quality

| Type | Status | Description |
|------|--------|-------------|
| `doc_integrity` | ✅ | CLI documentation sync check (undocumented commands, stale references, missing flags, example validation) |

---

## Configuration

| Feature | Status | Description |
|---------|--------|-------------|
| **Config-dir-relative paths** | ✅ ⭐ | Commands run from the config file's directory, not the caller's cwd |
| **Custom config path** | ✅ | `-f` flag to load config from any path |
| **Full validation on load** | ✅ | All errors reported at once before any test runs |
| **Go duration strings** | ✅ | Timeouts accept `30s`, `2m`, `1m30s`, etc. |
| **Shell command execution** | ✅ | All commands run via `sh -c` — pipes, redirects, and operators work |
| **Config inheritance** | ✅ | `includes:` directive to share tests across configs |
| **Config extends** | ✅ | `extends:` for base config inheritance |
| **Go templates** | ✅ | `{{ .Env.FOO }}` env var references in config values |
| **Multi-environment** | ✅ | `--env` flag loads environment-specific config overrides (e.g. `staging.smokesig.yaml`) |
| **Legacy config fallback** | ✅ | `.smoke.yaml` still loads with deprecation warning |

---

## Output & Reporting

| Feature | Status | Description |
|---------|--------|-------------|
| **Terminal reporter** | ✅ | Styled output with pass/fail indicators (Lipgloss) |
| **JSON reporter** | ✅ | Machine-readable output for CI pipelines (`--format json`) |
| **JUnit reporter** | ✅ | JUnit XML for GitHub Actions, Jenkins, GitLab CI (`--format junit`) |
| **TAP reporter** | ✅ | Test Anything Protocol (`--format tap`) |
| **Prometheus reporter** | ✅ | Prometheus metrics (`--format prometheus`) |
| **GitHub Actions reporter** | ✅ | Markdown to `$GITHUB_STEP_SUMMARY` + `::error`/`::warning` annotations (`--format gha`) |
| **Backstage reporter** | ✅ | Backstage entity annotation JSON for developer portal integration (`--format backstage`) |
| **Multi-reporter chaining** | ✅ ⭐ | Comma-separated: `--format terminal,json,prometheus` |
| **Pluggable reporter interface** | ✅ | Clean interface for adding custom reporters |
| **Exit codes** | ✅ | `0` = all pass, `1` = failures, `2` = config/arg error |
| **Push reporter** | ✅ | Push JSON results to configurable endpoint (`--report-url`, `--report-api-key`) |
| **Webhook notifications** | ✅ ⭐ | Slack Block Kit, PagerDuty Events API v2, and raw JSON webhook notifications (`notifications:` config + `--webhook-format`, `--webhook-on` flags) |
| **OTLP telemetry export** | ✅ | Export smoke results as OTLP spans (auto or via `export_url`) |

---

## Observability

| Feature | Status | Description |
|---------|--------|-------------|
| **OpenTelemetry trace correlation** | ✅ ⭐ | W3C traceparent propagation into HTTP, gRPC, WebSocket |
| **OTLP telemetry export** | ✅ | Export smoke results as OTLP spans with custom headers |
| **Multi-backend trace verification** | ✅ | Jaeger, Tempo, Honeycomb, Datadog backends |
| **Trace-aware retry** | ✅ | Skip retries when otel_trace confirms delivery |
| **Watch mode trace health** | ✅ | Sliding window (last 10 runs) health monitoring, alerts below 50% |

---

## Advanced Features

| Feature | Status | Description |
|---------|--------|-------------|
| **Monorepo support** | ✅ | `--monorepo` auto-discovers `.smokesig.yaml` in subdirectories (unlimited depth, configurable exclusions) |
| **Stress testing** | ✅ | `smokesig stress <test> --runs 50 --workers 4` — detect flakiness with pass rate and deduplicated error reporting |
| **Performance baselines** | ✅ | `--baseline` stores and compares timing across runs with configurable regression threshold |
| **Config validation** | ✅ | `smokesig validate` — standalone config validation without running |
| **Audit command** | ✅ | `smokesig audit` — project smoke test config health check (score 0–10, `--json` for CCS, `--fix` auto-applies safe fixes) |
| **Schema export** | ✅ | `smokesig schema` — export assertion types as JSON |
| **Portfolio dashboard** | ✅ | `smokesig serve --dashboard` — SQLite storage, REST API (`/api/results`, `/api/projects`), embedded HTML UI |
| **Health endpoint** | ✅ | `smokesig serve` — `/healthz` HTTP endpoint that runs smoke tests per request (container probes) |
| **MCP server** | ✅ ⭐ | `smokesig mcp` — Claude Desktop integration (7 tools: run, init, validate, list, discover, explain, generate_test; stdio transport) |
| **Auto-add generator** | ✅ ⭐ | `smokesig observe "cmd"` — wrap any command, capture stdout/ports/files/HTTP, generate `.smokesig.yaml` automatically. Supports `--dir`, `--quiet`, `--timeout`, `--output`. |
| **Test chaining** | ✅ | `extract:` captures values from test output; `{{ .Vars.X }}` injects into subsequent tests. Automatic sensitive variable masking. |
| **Goss migration** | ✅ | `smokesig migrate goss` — import Goss test configs with distro support (deb/rpm/apk), strict mode, mapping stats |
| **Pre-commit hook** | ✅ | `.pre-commit-hooks.yaml` for zero-config integration |
| **Lifecycle hooks** | ✅ | `before_all`/`after_all`/`before_each`/`after_each` with background process support, port wait, and env passthrough |
| **Recursion guard** | ✅ | Runner sets `SMOKESIG_RUNNING=1` on child processes to prevent fork bombs when configs contain test runner commands |

---

## Project Detection (smokesig init)

| Feature | Status | Description |
|---------|--------|-------------|
| **31 project types** | ✅ ⭐ | Auto-detect and generate tailored smoke test templates |
| **Force overwrite** | ✅ | `--force` flag regenerates config even if one already exists |
| **From running container** | ✅ | `--from-running <container>` generates config from a running Docker container |
| **Doc integrity auto-include** | ✅ | `--with-doc-integrity` auto-detects CLI projects and includes `doc_integrity` test with auto-discovered doc files |

### Detected Project Types

**Languages (19):** Go, Node (bun/npm), Python, Rust, Java (Maven), Java (Gradle), .NET/C#, Ruby, PHP, Deno, Scala, Elixir, Swift (server), Dart (server), Zig, Haskell, Lua, C/C++ (Make), C/C++ (CMake)

**Mobile (4):** React Native, Flutter, iOS, Android

**Infrastructure (4):** Docker, Terraform, Helm, Kustomize, Serverless

**Static Sites (3):** Hugo, Astro, Jekyll

---

## CI/CD Integration

| Feature | Status | Description |
|---------|--------|-------------|
| **GitHub Actions workflow** | ✅ | Reusable workflow at `.github/workflows/smoke.yml` |
| **GitHub Actions reporter** | ✅ | `--format gha` writes `$GITHUB_STEP_SUMMARY` + CI annotations |
| **Backstage reporter** | ✅ | `--format backstage` emits developer portal entity JSON |
| **Pre-commit hook** | ✅ | `.pre-commit-hooks.yaml` for pre-commit framework |
| **JUnit XML output** | ✅ | Native CI test result ingestion |
| **JSON artifact output** | ✅ | Machine-readable results for pipelines |
| **Push reporter** | ✅ | POST results to any endpoint with API key auth |
| **Exit code gates** | ✅ | `0`/`1`/`2` semantic exit codes |

---

## Design Constraints

These are intentional limitations, not gaps:

- **No test discovery** — tests must be explicitly listed in config; no globbing
- **No secrets management** — pass secrets via Go template env var references in commands
- **Minimal dependencies** — Cobra + Lipgloss + yaml.v3 + gjson + mcp-go + gRPC; no Viper, no Bubbletea
- **S3 is anonymous-only** — authenticated access uses the `http` assertion with Go template env vars
- **version_check is Unix-only** — uses `sh -c` which doesn't exist on Windows
- **gRPC is opt-in** — excluded from default build, use `-tags grpc` to include

---

## Architecture

```
SmokeSig/
├── cmd/                # CLI commands (run, observe, stress, init, audit, validate, schema, serve, mcp, migrate, version)
├── internal/
│   ├── schema/         # SmokeConfig structs, YAML parsing, validation
│   ├── runner/         # Assertion engine (45 types), prereq runner, test execution, stress testing, lifecycle hooks
│   ├── reporter/       # Terminal + JSON + JUnit + TAP + Prometheus + GHA + Backstage + Push + Webhook + OTel reporters
│   ├── observer/       # Auto-add generator — command wrapping, port detection, file snapshot, YAML generation
│   ├── dashboard/      # SQLite storage, REST API, embedded HTML frontend
│   ├── monorepo/       # Sub-config discovery for monorepo projects
│   ├── detector/       # Project type detection (31 types) + template generation
│   ├── baseline/       # Performance baseline storage and comparison
│   ├── mcp/            # MCP server (7 tools), handlers, suggestion engine
│   └── migrate/        # Framework migration (Goss)
├── .smokesig.yaml      # Self-smoke tests for this repo
├── .pre-commit-hooks.yaml  # Pre-commit framework integration
└── .github/workflows/  # CI + reusable smoke workflow
```

---

## Quick Start

```bash
go install github.com/CosmoLabs-org/SmokeSig@latest
cd my-project
smokesig init
smokesig run
```
