# Agent 4: Competitive Analysis Audit

## SmokeSig v0.21.1 — Competitive Positioning Audit

### 1. Feature Completeness (Score: 88/100)

**What SmokeSig covers well:**

- **39 assertion types** across 22 assertion files, covering: HTTP, gRPC, WebSocket, GraphQL, DNS, SMTP, SSL certs, databases (Redis, Postgres, MySQL, Memcached, MongoDB), message brokers (Kafka, MQTT), infrastructure (Docker, Docker Compose, K8s resources, S3), identity (LDAP), network (port, ping, NTP), credentials, deep links, OTel traces, file/process/env checks, and version verification. Each assertion is implemented as pure Go with native wire protocol implementations — no SDK dependencies.

- **Lifecycle hooks** (`internal/runner/lifecycle.go`): `before_all`, `after_all`, `before_each`, `after_each` with background process support, `wait_for_port` with exponential backoff, and `env_pass`.

- **Test chaining via VarStore** (`internal/runner/varstore.go`): Tests can extract values from stdout (via regex or JSON path) and pass them to subsequent tests. Auto-detects chains and forces sequential execution.

- **Retry with exponential backoff** (runner.go:261-287): Per-test retry with `retry_on_trace_only` for OTel-aware retries.

- **Conditional skip** (schema.go:92-103, runner.go:822-846): `skip_if` with `env_unset`, `env_equals`, `file_missing`.

**Gaps vs. user expectations:**
- No `stdout_not_contains` or negation assertions
- No `file_content_contains` / `file_content_matches`
- No SSH/remote execution
- Cannot chain multiple HTTP calls within a single test

### 2. Differentiators (Score: 92/100)

**Genuine unique features that competitors lack:**

1. **MCP Server for AI integration** (`internal/mcp/server.go`): 7 tools via Model Context Protocol for Claude Desktop. No competitor offers this.

2. **39 native wire-protocol assertions** without external dependencies. Goss has ~10 resource types. InSpec has ~100+ but requires Ruby. SmokeSig implements Kafka, MongoDB, MQTT, LDAP all in pure Go.

3. **Deep link verification** (`assertion_deeplink.go`): Two-tier — HTTP config checks + optional adb/xcrun tool checks. No other smoke test tool touches mobile deep links.

4. **OTel trace verification end-to-end** (`assertion_otel.go`): Injects W3C traceparent, verifies traces at Jaeger/Tempo/Honeycomb/Datadog. Plus TraceHealthTracker with sliding window in watch mode.

5. **31 project-type auto-detection** (`internal/detector/detector.go`): Generates tailored configs for Go, Node, Python, Rust, Java, .NET, Ruby, PHP, and 23 more.

6. **Goss migration** (`cmd/migrate.go`): Direct competitive displacement tool with `--strict`, `--stats`, `--distro`.

7. **Portfolio dashboard** (`internal/dashboard/`): SQLite-backed with REST API. Backstage reporter integration.

8. **Stress/flakiness detection** (`cmd/stress.go`): Pass rate, reliability labels, deduplicated error groups.

9. **Docker container inspection** (`internal/detector/container.go`): `smokesig init --from-running <container>` auto-generates smoke tests.

### 3. Market Gaps (Score: 68/100)

**What competitors have that SmokeSig lacks:**

1. No scheduled/cron execution — no agent mode
2. No alerting/notifications — no Slack, PagerDuty, webhook integration
3. No remote execution — Goss has `goss serve`, InSpec has SSH transport
4. No compliance/policy framework — no CIS benchmarks or HIPAA controls
5. No parallel execution with dependencies (DAG-based)
6. No test grouping/suites within config
7. No built-in secrets management — no Vault/SOPS integration
8. Limited HTTP depth — no cookies, redirect control, client certs
9. No Windows-native assertions beyond process_running

### 4. Developer Experience (Score: 85/100)

**Strengths:**
- Zero-config start with `smokesig init` auto-detection
- Beautiful Lipgloss terminal output
- 7 output formats including GitHub Actions annotations and Backstage entities
- Go template support in config
- Environment overlays with deep-merge
- Config inheritance (extends + includes)
- All-errors-at-once validation
- MCP integration for AI workflows
- 1,010 test functions — very high coverage

**Weaknesses:**
- No interactive/step-through mode
- Cryptic wire-protocol error messages
- No `--verbose` flag for passing test details
- No `--help` per assertion type

### 5. Ecosystem Integration (Score: 82/100)

**Strong:** GitHub Actions (gha format), Prometheus metrics, OTel/OTLP spans, Backstage entities, JUnit XML, TAP, Push reporting, Dashboard API, Docker assertions, Kubernetes assertions.

**Missing:** GitLab CI format, Slack/Teams/PagerDuty, Terraform provider, Helm chart for self-deployment, published container image.

### Critical Bugs Found

1. JSON injection in dashboard error handler (handler.go:61) — raw err.Error() concatenated into JSON
2. `allow_self_signed` maps to `InsecureSkipVerify` skipping ALL TLS verification (assertion_network.go:197)
3. Inconsistent indentation on FileSize assertion block (runner.go:461)
