# SmokeSig — Project Instructions

## Overview

Universal smoke test runner. Standalone Go binary that reads `.smokesig.yaml` and runs lightweight smoke tests. Designed for CosmoLabs' ~95-project portfolio.

**Repository**: `github.com/CosmoLabs-org/SmokeSig`
**Company**: CosmoLabs
**Version**: 0.22.0

## Architecture

```
cmd/
├── root.go          # Cobra root command with banner
├── run.go           # smokesig run — main entry point
├── observe.go       # smokesig observe — auto-generate tests from command observation
├── stress.go        # smokesig stress — flakiness detection via repeated runs
├── validate.go      # smokesig validate — config validation without running
├── schema.go        # smokesig schema — export assertion types as JSON
├── init_cmd.go      # smokesig init — auto-detect + generate config
├── audit_cmd.go     # smokesig audit — project smoke test config health check
├── serve.go         # smokesig serve — dashboard server with REST API
├── mcp.go           # smokesig mcp — MCP server for AI integration
├── migrate.go       # smokesig migrate — Goss config migration
└── version.go       # smokesig version (ldflags-injected)
internal/
├── schema/          # SmokeConfig structs, YAML parsing, validation
├── baseline/        # Performance baseline storage and comparison
├── runner/          # Assertion engine (45 types), prereq runner, test execution, lifecycle hooks
├── reporter/        # Terminal + JSON + JUnit + TAP + Prometheus + GHA + Backstage + Push + Webhook reporters
├── observer/        # Auto-add generator — command wrapping, port detection, file snapshot, YAML generation
├── monorepo/        # Sub-config discovery for monorepo projects
├── dashboard/       # Portfolio dashboard (SQLite storage, API handlers, embedded UI)
├── auth/            # OIDC cloud auth (AWS STS, GCP WIF) — CI token exchange, credential masking
├── detector/        # Project type detection (31 types) + template generation
└── mcp/             # MCP server (7 tools) + suggestion engine
```

## Key Design Decisions

- **Minimal deps**: Cobra + Lipgloss + yaml.v3 + gjson. No Viper, no Bubbletea.
- **Pure assertions**: All 45 assertion types are pure functions — no side effects.
- **Config inheritance**: `includes:` directive + Go templates (`{{ .Env.FOO }}`).
- **Config-dir-relative**: Commands execute from the config file's directory, not cwd.
- **All errors at once**: Validation returns all errors, not just the first.
- **Reporter interface**: Terminal and JSON reporters are pluggable via interface.
- **Watch mode**: `--watch` keeps smoke resident and re-runs on file changes. fsnotify-backed. 500ms debounce. When OTel is enabled, tracks trace health across runs with a sliding window (last 10 runs). Alerts when health drops below 50%.
- **Retry**: Opt-in `retry: {count, backoff, retry_on_trace_only?}` on test level. Exponential backoff. No side effects on pass-first-try. `retry_on_trace_only` skips retry when the otel_trace assertion confirms the trace was received.
- **Monorepo**: `--monorepo` flag auto-discovers `.smokesig.yaml` in subdirectories. Unlimited depth, configurable exclusions.
- **WebSocket**: Stdlib-only WebSocket client. Connect-send-expect pattern with no external deps.
- **gRPC opt-in**: gRPC health check excluded from default build. Use `-tags grpc` to include.
- **OIDC auth**: `auth:` config section for CI-to-cloud role assumption. AWS STS + GCP Workload Identity Federation. No cloud SDK deps (raw HTTP via `net/http` + `encoding/xml` + `encoding/json`). Env var injection for `run:` commands + kubectl. Standalone HTTP assertions (e.g., `s3_bucket`) remain anonymous (no SigV4 in v1). `--no-auth` flag to disable.
- **Recursion guard**: Runner sets `SMOKESIG_RUNNING=1` on child processes. If already set, test commands matching test runner patterns (`go test`, `npm test`, `pytest`, etc.) are auto-skipped to prevent fork bombs. See BUG-012.

## Build & Test

```bash
go build ./...                    # Build
go test ./...                     # Run all tests (1297 total)
smokesig run                         # Self-smoke (6 tests)
go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=X.Y.Z" -o smokesig .
```

## Commands

```bash
smokesig run [--tag X] [--exclude-tag X] [--format terminal,json,junit,tap,prometheus,gha,backstage] [--fail-fast] [--timeout 30s] [-f path] [--dry-run] [--watch] [--monorepo] [--otel-collector URL] [--no-otel] [--no-auth] [--report-url URL] [--report-api-key KEY] [--webhook-format slack|pagerduty|json] [--webhook-on failure|always|change] [--baseline] [--baseline-threshold 50]
smokesig validate [-f path]
smokesig audit [-f path] [--json] [--fix]
smokesig schema
smokesig stress <test> [--runs N] [--workers N] [--fail-fast]
smokesig serve [--port 8080] [--dashboard] [--api-key KEY] [--db-path PATH]
smokesig init [--force] [--from-running CONTAINER] [--with-doc-integrity]
smokesig observe [command] [--dir DIR] [--timeout DURATION] [--quiet] [--output PATH]
smokesig migrate goss <file>
smokesig mcp
smokesig version
```

## Assertion Types

| Type | Field | Description |
|------|-------|-------------|
| exit_code | `*int` | Process exit code match |
| stdout_contains | `string` | Substring match on stdout |
| stdout_matches | `string` | Regex match on stdout |
| stderr_contains | `string` | Substring match on stderr |
| stderr_matches | `string` | Regex match on stderr |
| file_exists | `string` | File exists relative to config dir |
| env_exists | `string` | Environment variable exists |
| port_listening | `{port, protocol?, host?}` | TCP/UDP port is open |
| process_running | `string` | Named process currently running (pgrep -x / tasklist) |
| http | `{url, method?, status_code?, body_contains?, body_matches?, header_contains?}` | HTTP endpoint check |
| json_field | `{path, equals?, contains?, matches?}` | JSONPath assertion on stdout |
| response_time_ms | `*int` | Test duration must not exceed this threshold |
| ssl_cert | `{host, port?, min_days_remaining?, allow_self_signed?}` | TLS cert valid + expiry threshold |
| redis_ping | `{host?, port?, password?}` | Redis PING returns +PONG (RESP protocol) |
| memcached_version | `{host?, port?}` | Memcached `version` command returns VERSION |
| postgres_ping | `{host?, port?}` | Postgres server SSLRequest handshake returns valid protocol byte |
| mysql_ping | `{host?, port?}` | MySQL server sends valid v10 handshake packet on connection |
| grpc_health | `{address, service?, use_tls?, timeout?}` | grpc.health.v1 Health/Check returns SERVING (requires `-tags grpc`) |
| websocket | `{url, send?, expect_contains?, expect_matches?, timeout?}` | WebSocket connect-send-expect assertion |
| docker_container_running | `{name}` | Named Docker container is currently running |
| docker_image_exists | `{image}` | Docker image exists locally |
| url_reachable | `{url, timeout?, status_code?}` | HTTP/HTTPS connectivity check |
| service_reachable | `{url, timeout?}` | External service dependency check |
| s3_bucket | `{bucket, region?, endpoint?}` | S3-compatible bucket accessibility (anonymous HEAD) |
| version_check | `{command, pattern}` | Tool version verification via shell command + regex |
| otel_trace | `{backend?, jaeger_url, service_name?, min_spans?, timeout?, api_key?, dd_app_key?}` | Trace verification with W3C traceparent propagation. Backends: jaeger (default), tempo, honeycomb, datadog |
| credential_check | `{source, name, contains?}` | Credential accessible without leaking value (env\|file\|exec) |
| graphql | `{url, query?, status_code?, expect_types?, expect_contains?, timeout?}` | GraphQL introspection assertion |
| deep_link | `{url, android_package?, ios_bundle_id?, ios_associated_domains?, check_assetlinks?, check_aasa?, tier?}` | Mobile deep link / universal link verification (two-tier: HTTP config + tool-augmented resolution) |
| dns_resolve | `{hostname, record_type?, expected_ip?, timeout?}` | DNS resolution check (A, AAAA, TXT, MX, CNAME) |
| smtp_ping | `{host, port?, timeout?}` | SMTP server connectivity + EHLO handshake |
| docker_compose_healthy | `{compose_file?, services?, timeout?}` | Docker Compose service health check |
| ping | `{host, count?, timeout?}` | ICMP echo via system ping command |
| mongo_ping | `{host?, port?, username?, password_env?}` | MongoDB isMaster wire protocol check |
| kafka_broker | `{brokers, topic?, timeout?}` | Kafka metadata request wire protocol check |
| ldap_bind | `{host, port?, bind_dn?, password_env?, use_tls?, timeout?}` | LDAP bind request (ASN.1 BER) |
| mqtt_ping | `{broker, client_id?, username?, password_env?, timeout?}` | MQTT CONNECT/CONNACK wire protocol check |
| ntp_check | `{server?, max_offset_ms?, timeout?}` | NTP time sync verification (UDP) |
| k8s_resource | `{context?, namespace, kind, name, condition?, timeout?}` | Kubernetes resource state via kubectl |
| ios_simulator | `{device_name?, os?, timeout?}` | Check if an iOS simulator is booted (xcrun simctl) |
| android_emulator | `{serial?, timeout?}` | Check if an Android emulator has finished booting (adb) |
| doc_integrity | `{binary, docs, check_examples?, ignore_commands?, timeout?}` | CLI documentation sync check (commands, flags, examples) |

Plus `allow_failure: true` on Test for flaky/allowed-failure tests.

## OpenTelemetry Integration

```yaml
otel:
  enabled: true
  jaeger_url: "http://jaeger:16686"
  service_name: "SmokeSig"
  trace_propagation: true
```

When enabled, W3C `traceparent` headers are auto-injected into HTTP, gRPC, and WebSocket assertions. The `otel_trace` assertion verifies traces arrived at a collector (supports Jaeger, Tempo, Honeycomb, Datadog backends).

Smoke test results are also exported as OTLP telemetry when `export_url` is configured or `jaeger_url` is set (auto-appends `/v1/traces`). Each test becomes a span with attributes for pass/fail status, duration, and assertion details.

## Webhook Notifications

```yaml
notifications:
  - url: "https://hooks.slack.com/services/T00/B00/xxx"
    format: slack
    on: failure
  - url: "https://events.pagerduty.com/v2/enqueue"
    format: pagerduty
    on: change
    api_key_env: "PAGERDUTY_ROUTING_KEY"
```

| Field | Description |
|-------|-------------|
| `url` | Webhook endpoint URL |
| `format` | `slack` (Block Kit), `pagerduty` (Events API v2), or `json` (raw JSON) |
| `on` | `failure` (default), `always`, or `change` (on status change) |
| `api_key_env` | Env var name for API key / routing key |

CLI overrides: `--webhook-format` and `--webhook-on` work with existing `--report-url` and `--report-api-key`. Slack format uses color-coded attachments and auto-detects CI URL from `GITHUB_SERVER_URL`/`GITLAB_URL`. PagerDuty uses severity `critical` when >50% tests fail, `error` otherwise, and auto-resolves on recovery.

## Audit Command

`smokesig audit` inspects the project and reports missing or outdated smoke test configuration. Checks: config exists, assertion coverage vs project type, stale references, baseline tests. Scores 0–10. `--json` outputs structured JSON for CCS consumption. `--fix` auto-applies safe recommendations.

## Init —with-doc-integrity

`smokesig init --with-doc-integrity` auto-detects CLI projects (Go `cmd/`, Node `bin`, Python `scripts`, Rust `[[bin]]`) and includes a `doc_integrity` test when a CLI binary is detected. Auto-detects which doc files exist (README.md, CLAUDE.md, SPEC.md, docs/USAGE.md). Without the flag, doc_integrity is included automatically for detected CLI projects; the flag forces inclusion.

## Output Formats

`smokesig run --format X` supports: `terminal` (default), `json`, `junit`, `tap`, `prometheus`, `gha`, `backstage`. Comma-separated for multiple: `--format terminal,json`. First format goes to stdout, rest to auto-named files (`smoke-results.json`, `smoke-junit.xml`, `smoke-metrics.prom`, `smoke-tap.txt`, `smoke-backstage.json`). The `gha` format writes markdown to `$GITHUB_STEP_SUMMARY` and emits `::error`/`::warning` workflow commands for CI annotations. The `backstage` format emits a Backstage entity annotation JSON for developer portal integration.

## Detected Project Types

31 project types with auto-detection and tailored smoke test templates:

**Languages:** Go, Node (bun/npm), Python, Rust, Java (Maven), Java (Gradle), .NET/C#, Ruby, PHP, Deno, Scala, Elixir, Swift (server), Dart (server), Zig, Haskell, Lua, C/C++ (Make), C/C++ (CMake)

**Mobile:** React Native, Flutter, iOS, Android

**Infrastructure:** Docker, Terraform, Helm, Kustomize, Serverless

**Static Sites:** Hugo, Astro, Jekyll
