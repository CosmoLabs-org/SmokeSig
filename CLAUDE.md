# SmokeSig — Project Instructions

## Overview

Universal smoke test runner. Standalone Go binary that reads `.smokesig.yaml` and runs lightweight smoke tests. Designed for CosmoLabs' ~95-project portfolio.

**Repository**: `github.com/CosmoLabs-org/SmokeSig`
**Company**: CosmoLabs
**Version**: 0.20.1

## Architecture

```
cmd/
├── root.go          # Cobra root command with banner
├── run.go           # smokesig run — main entry point
├── validate.go      # smokesig validate — config validation without running
├── schema.go        # smokesig schema — export assertion types as JSON
├── init_cmd.go      # smokesig init — auto-detect + generate config
└── version.go       # smokesig version (ldflags-injected)
internal/
├── schema/          # SmokeConfig structs, YAML parsing, validation
├── baseline/        # Performance baseline storage and comparison
├── runner/          # Assertion engine (39 types), prereq runner, test execution
├── reporter/        # Terminal (Lipgloss) + JSON + JUnit + TAP + Prometheus + GHA + Backstage + Push reporters
├── monorepo/        # Sub-config discovery for monorepo projects
├── dashboard/       # Portfolio dashboard (SQLite storage, API handlers, embedded UI)
└── detector/        # Project type detection + template generation
```

## Key Design Decisions

- **Minimal deps**: Cobra + Lipgloss + yaml.v3 + gjson. No Viper, no Bubbletea.
- **Pure assertions**: All 29 assertion types are pure functions — no side effects.
- **Config inheritance**: `includes:` directive + Go templates (`{{ .Env.FOO }}`).
- **Config-dir-relative**: Commands execute from the config file's directory, not cwd.
- **All errors at once**: Validation returns all errors, not just the first.
- **Reporter interface**: Terminal and JSON reporters are pluggable via interface.
- **Watch mode**: `--watch` keeps smoke resident and re-runs on file changes. fsnotify-backed. 500ms debounce. When OTel is enabled, tracks trace health across runs with a sliding window (last 10 runs). Alerts when health drops below 50%.
- **Retry**: Opt-in `retry: {count, backoff, retry_on_trace_only?}` on test level. Exponential backoff. No side effects on pass-first-try. `retry_on_trace_only` skips retry when the otel_trace assertion confirms the trace was received.
- **Monorepo**: `--monorepo` flag auto-discovers `.smokesig.yaml` in subdirectories. Unlimited depth, configurable exclusions.
- **WebSocket**: Stdlib-only WebSocket client. Connect-send-expect pattern with no external deps.
- **gRPC opt-in**: gRPC health check excluded from default build. Use `-tags grpc` to include.

## Build & Test

```bash
go build ./...                    # Build
go test ./...                     # Run all tests (1045 total)
smokesig run                         # Self-smoke (6 tests)
go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=X.Y.Z" -o smokesig .
```

## Commands

```bash
smokesig run [--tag X] [--exclude-tag X] [--format terminal,json,junit,tap,prometheus,gha,backstage] [--fail-fast] [--timeout 30s] [-f path] [--dry-run] [--watch] [--monorepo] [--otel-collector URL] [--no-otel] [--report-url URL] [--report-api-key KEY] [--baseline] [--baseline-threshold 50]
smokesig validate [-f path]
smokesig schema
smokesig serve [--port 8080] [--dashboard] [--api-key KEY] [--db-path PATH]
smokesig init [--force] [--from-running CONTAINER]
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

## Output Formats

`smokesig run --format X` supports: `terminal` (default), `json`, `junit`, `tap`, `prometheus`, `gha`, `backstage`. Comma-separated for multiple: `--format terminal,json`. First format goes to stdout, rest to auto-named files (`smoke-results.json`, `smoke-junit.xml`, `smoke-metrics.prom`, `smoke-tap.txt`, `smoke-backstage.json`). The `gha` format writes markdown to `$GITHUB_STEP_SUMMARY` and emits `::error`/`::warning` workflow commands for CI annotations. The `backstage` format emits a Backstage entity annotation JSON for developer portal integration.

## Detected Project Types

31 project types with auto-detection and tailored smoke test templates:

**Languages:** Go, Node (bun/npm), Python, Rust, Java (Maven), Java (Gradle), .NET/C#, Ruby, PHP, Deno, Scala, Elixir, Swift (server), Dart (server), Zig, Haskell, Lua, C/C++ (Make), C/C++ (CMake)

**Mobile:** React Native, Flutter, iOS, Android

**Infrastructure:** Docker, Terraform, Helm, Kustomize, Serverless

**Static Sites:** Hugo, Astro, Jekyll
