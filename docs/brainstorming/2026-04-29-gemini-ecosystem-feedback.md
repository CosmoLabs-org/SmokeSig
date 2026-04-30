---
date: 2026-04-29
source: Gemini AI competitive analysis
status: brainstorm
deliverables:
  - id: BR-01
    title: "GitHub Actions native output reporter (--format gha)"
  - id: BR-02
    title: "Flakiness detector command (smoke stress)"
  - id: BR-03
    title: "Remote config inheritance (extends: URL)"
  - id: BR-04
    title: "OIDC integration for cloud role assumption (opt-in build tag)"
  - id: BR-05
    title: "Backstage.io schema output reporter"
  - id: BR-06
    title: "Official GitHub Action wrapper"
  - id: BR-07
    title: "Wasm plugin system for custom assertions"
  - id: BR-08
    title: "Test chaining with data extraction (JWT flows etc.)"
  - id: BR-09
    title: "Interactive TUI test runner (bubbletea)"
  - id: BR-10
    title: "Setup/teardown lifecycle hooks (before_all, after_all, etc.)"
  - id: BR-11
    title: "Bundle size and file size threshold assertions"
  - id: BR-12
    title: "Simulator/emulator health assertions (iOS/Android)"
  - id: BR-13
    title: "Auto-Add Generator (observe command, generate assertions from live execution)"
  - id: BR-14
    title: "Background commands with wait_for_port (multi-process orchestration)"
  - id: BR-15
    title: "Detector-observer integration (stack-aware test generation)"
---

# Gemini Ecosystem Feedback — Gap Analysis & New Ideas

## Executive Summary

Gemini's feedback is a solid ecosystem survey but suffers from one critical flaw: **it assumes cosmo-smoke only has 5 basic assertion types** (`exit_code`, `stdout/stderr`, `file_exists`, `env_exists`). In reality, cosmo-smoke v0.15.0 has **39 assertion types**, a dashboard, OpenTelemetry integration, watch mode, and `smoke init` with 31 project type detectors.

**The real takeaway**: Gemini didn't know what we already have, so it suggested building what we've already built. The genuinely valuable new ideas are the ones that go beyond what exists.

---

## What Gemini Got Wrong (Already Implemented)

| Gemini Suggestion | cosmo-smoke Already Has | Notes |
|---|---|---|
| HTTP checks (`http_status`, `http_body`, `http_header`) | `http` assertion with `status_code`, `body_contains`, `body_matches`, `header_contains` | Full HTTP assertion since early versions |
| JSON parsing (`json_path`) | `json_field` with gjson `path`, `equals`, `contains`, `matches` | gjson-based, very capable |
| Network/Ports (`tcp_listen`, `udp_listen`) | `port_listening` with `protocol: tcp/udp`, `host` | Both protocols supported |
| Database connectivity (`sql_ping`) | `redis_ping`, `postgres_ping`, `mysql_ping`, `mongo_ping`, `kafka_broker`, `ldap_bind`, `memcached_version` | 7 wire-protocol DB checks, no drivers needed |
| Timeouts/Latency (`execution_time`) | `response_time_ms` | Per-test threshold |
| Security/Certificates (`tls_validity`) | `ssl_cert` with `min_days_remaining`, `allow_self_signed` | Full TLS cert assertion |
| OpenTelemetry/OTLP | Full OTel integration: trace propagation, `otel_trace` assertion, OTLP export, Jaeger/Tempo/Honeycomb/Datadog backends | Enterprise-grade |
| Config inheritance (`extends`) | `includes:` directive | Local file inclusion |
| Environment substitution (`${VAR:-default}`) | Go templates `{{ .Env.FOO }}` | Template-based |
| YAML Anchors | yaml.v3 parser handles anchors natively | Works out of the box |
| Watch mode (`cosmo-smoke watch`) | `smoke run --watch` with fsnotify, 500ms debounce, OTel trace health tracking | Plus sliding window health alerts |
| Sidecar health endpoint (`cosmo-smoke serve`) | `smoke serve --dashboard --port 8080` with SQLite storage, API handlers, embedded UI | Full dashboard, not just healthz |
| AI auto-scaffolding (`cosmo-smoke generate --ai`) | `smoke init` with 31 project type detectors + tailored templates | Zero-config detection, no LLM needed |
| JUnit output | `--format junit` (plus json, tap, prometheus, terminal) | 5 output formats |
| `allow_failure` for flaky tests | `allow_failure: true` on any test | Plus `retry:` with exponential backoff |

**Score: 15/16 "suggestions" already implemented.** The one thing Gemini got right that we don't have is remote config inheritance (URL-based `extends`).

---

## Genuinely New Ideas Worth Exploring

These are the ideas from Gemini's analysis that cosmo-smoke does NOT have and that could add real value:

### Tier 1 — High Value, Feasible

#### 1. GitHub Actions Native Output (`$GITHUB_STEP_SUMMARY`)
**What**: A `--format github` or `--format gha` mode that writes markdown to `$GITHUB_STEP_SUMMARY` and emits `::error`/`::warning` annotations for failed tests.

**Why it matters**: GitHub Actions is the dominant CI platform. Native integration means failed smoke tests show up as inline PR annotations without any extra tooling. This is a "first 5 minutes" experience — someone adds cosmo-smoke to their Actions workflow and immediately sees results in the PR UI.

**Implementation sketch**:
- New reporter: `internal/reporter/github.go`
- On init, check `$GITHUB_ACTIONS` env var
- Write markdown summary to `$GITHUB_STEP_SUMMARY`
- Emit `::error file=X,line=Y,title=Smoke Test Failed::message` for each failure
- Problem matcher JSON file for regex-based annotation

**Effort**: ~200 lines. Low risk. High visibility.

#### 2. Flakiness Detector (`smoke stress`)
**What**: `smoke stress <test-name> --count 100 --parallel 10` — runs a single test N times with configurable parallelism, reports pass rate and timing distribution.

**Why it matters**: We already have `allow_failure: true` and `retry:` for known-flaky tests. But there's no tooling to *discover* which tests are flaky. A stress command gives teams data to make informed decisions about `allow_failure` and `retry` settings.

**Implementation sketch**:
- New command: `cmd/stress.go`
- Reuse existing runner but run single test N times
- Collect: pass rate, mean/median/p95 duration, failure patterns
- Output: histogram + summary table
- Optional `--json` for programmatic consumption

**Effort**: ~300 lines. Moderate (needs parallel orchestration).

#### 3. Remote Config Inheritance (`extends: URL`)
**What**: Allow `.smoke.yaml` to inherit from a remote URL:
```yaml
extends: https://raw.githubusercontent.com/org/infra/main/base-smoke.yaml
tests:
  - name: my-custom-test
    # ...
```

**Why it matters**: CosmoLabs has ~95 projects. Today `includes:` only works for local files. A remote `extends` lets every project inherit a canonical baseline (security checks, cert validity, critical endpoint checks) from a single source of truth. Change once, propagates everywhere.

**Implementation sketch**:
- Extend schema parser to resolve `extends:` URLs at load time
- HTTP GET with caching (etag/last-modified)
- Merge strategy: remote base + local overrides
- Support `file://`, `https://`, and `git://` schemes
- Security: allowlist of domains or hash verification

**Effort**: ~400 lines. Medium complexity (merge semantics, caching, security).

### Tier 2 — Medium Value, Worth Considering

#### 4. OIDC Integration for Cloud Role Assumption
**What**: Allow smoke tests to assume AWS/GCP roles via OIDC tokens from CI providers, enabling real cloud resource assertions without stored credentials.

**Why it matters**: Tests like `s3_bucket`, `url_reachable`, `k8s_resource` often need cloud access. Hardcoded credentials in CI are a security anti-pattern. OIDC lets tests assume temporary roles.

**Implementation sketch**:
- New top-level config section: `auth:`
- Support `aws_oidc`, `gcp_oidc` providers
- Auto-detect CI environment (GitHub Actions, GitLab CI)
- Exchange OIDC token for temporary credentials
- Inject into test environment

**Effort**: ~500 lines. Higher complexity (AWS/GCP SDK deps, token exchange flows).

**Concern**: Adds dependency weight. Consider making this a plugin or opt-in build tag (like `grpc_health` uses `-tags grpc`).

#### 5. Backstage.io Schema Output
**What**: A `--format backstage` mode that emits JSON conforming to Backstage's entity annotation format, allowing smoke test health to surface in developer portals.

**Why it matters**: Organizations with Backstage deployments can see smoke test status alongside service ownership, API docs, and runbooks. Low-effort integration point.

**Implementation sketch**:
- New reporter: `internal/reporter/backstage.go`
- Map test results to Backstage HealthCheck annotations
- Output: JSON with `status: "healthy"/"unhealthy"`, individual check details

**Effort**: ~150 lines. Very low risk.

#### 6. Official GitHub Action
**What**: A published GitHub Action (`uses: cosmolabs-org/cosmo-smoke-action@v1`) that wraps `smoke run` with proper output handling, summary generation, and caching.

**Why it matters**: Reduces friction from "install binary + run command" to "one `uses:` line". Standard pattern for CI tools.

**Implementation sketch**:
- Separate repo: `cosmolabs-org/cosmo-smoke-action`
- `action.yaml` with inputs for config path, format, tags
- Downloads binary, runs smoke, captures output
- Sets `$GITHUB_STEP_SUMMARY` automatically

**Effort**: ~100 lines. Very low effort, high distribution value.

### Tier 3 — Interesting but Lower Priority

#### 7. Matrix Execution (`--env dev,staging,prod`)
**What**: Run the same test suite against multiple environments in parallel.

**Why it matters**: Useful for deployment verification pipelines. But this can already be achieved with CI matrix strategies or multiple `smoke run` invocations.

**Verdict**: Nice-to-have but not urgent. CI-native matrix is more flexible.

#### 8. S3/Parquet Output
**What**: Write test results to S3 in Parquet format for data lake aggregation.

**Why it matters**: Only relevant for organizations with mature data lake setups. The JSON output + a small ETL pipeline achieves the same result.

**Verdict**: Over-engineered for smoke tests. OTLP export to observability platforms is the better path.

#### 9. Goss-Style Observation-Based Generation
**What**: `smoke init --observe <command>` — run a command, observe its behavior, and generate assertions.

**Why it matters**: `smoke init` already detects project type and generates templates. Observation adds one more layer: "I just ran this, generate tests based on what happened."

**Verdict**: Interesting but `smoke init` + 31 project detectors covers the common case well. Observation is most useful for custom/legacy commands. Could be a v2 enhancement.

---

## What to Ask Gemini Back

The most valuable follow-up questions for Gemini:

1. **"How would you design the merge semantics for remote `extends:`?"** — Deep/overlay merge vs shallow replacement? How does Goss handle this? What about conflicts?

2. **"What's the best pattern for OIDC in smoke tests — should it be a config-level concern or a per-test concern?"** — Getting the abstraction boundary right matters.

3. **"Research the GitHub Actions problem matcher format and design a `--format gha` reporter spec."** — Concrete implementation guidance, not just "support GHA."

4. **"What assertion types are missing from cosmo-smoke that exist in no other tool?"** — The blue ocean question. What's genuinely novel vs. parity?

---

## Recommendations

**Priority order for implementation:**

| Priority | Feature | Effort | Impact |
|---|---|---|---|
| 1 | GitHub Actions native output | Low | High — first-class CI experience |
| 2 | Remote config inheritance (`extends: URL`) | Medium | High — portfolio-scale value for 95 projects |
| 3 | Flakiness detector (`smoke stress`) | Medium | Medium — quality assurance tooling |
| 4 | Official GitHub Action | Low | High — distribution + adoption |
| 5 | Backstage.io output | Low | Low-Medium — enterprise integration |

**Not recommended**: OIDC (add build-tag opt-in later), S3/Parquet (OTLP is the right path), Matrix execution (CI handles this).

**The meta-lesson**: Gemini's competitive analysis is solid but surface-level. It analyzed the *category* without reading the *product*. The real differentiator for cosmo-smoke isn't matching Goss/Karate feature-for-feature — it's the combination of 39 assertion types, zero-dependency wire-protocol checks, and portfolio-scale tooling (monorepo, dashboard, OTel) that no other tool offers.

---

## Batch 2: Advanced Features & DX (Gemini Round 2)

**Source**: Gemini AI — "next-level improvements for modern development flows"

### What Gemini Got Wrong (Already Implemented)

| Gemini Suggestion | cosmo-smoke Already Has |
|---|---|
| Watch mode (`--watch`) | `smoke run --watch` with fsnotify, 500ms debounce, OTel trace health sliding window |
| Dry-run mode (`--dry-run`) | `smoke run --dry-run` — lists tests without running |
| Smart retries with exponential backoff | `retry: {count, backoff, retry_on_trace_only?}` per-test, exponential backoff |
| Soft assertions (`warn_only`) | `allow_failure: true` on any test — reports failure but doesn't block |
| Process checks (`process_running`) | Built-in assertion type using `pgrep -x` / `tasklist` |
| Go template variable injection | `{{ .Env.FOO }}` in YAML configs |

**Score: 6/10 already implemented.** Gemini is still partially working from the "5 assertion types" baseline.

### Genuinely New Ideas

#### BR-07: Wasm Plugin System for Custom Assertions
**What**: Embed `wazero` (zero-dependency Wasm runtime) in the Go binary. Users write custom assertions in Rust, TS, or Go, compile to Wasm, and reference them in `.smoke.yaml`:
```yaml
tests:
  - name: custom-check
    plugin: my-plugin.wasm
    input: { key: "value" }
```

**Why it matters**: Today, every new assertion type requires a PR to cosmo-smoke. Wasm plugins let users extend without forking. This is the path to an ecosystem. No other smoke test tool does this.

**Risk**: Wasm runtime adds ~2-3MB to binary size. Plugin API surface needs careful design. `wazero` is pure Go but adds maintenance burden.

**Verdict**: **Blue ocean feature.** High complexity but genuinely novel. Consider as a v2 milestone after the core assertion library stabilizes. Would need: plugin manifest schema, assertion interface (input JSON → output JSON), sandboxing, timeout enforcement.

#### BR-08: Test Chaining with Data Extraction
**What**: Extract values from one test's output and inject into subsequent tests:
```yaml
tests:
  - name: login
    command: curl -s -X POST /auth/login
    assertions:
      - json_field:
          path: token
          extract: jwt_token    # <-- NEW: save to variable
  - name: get-profile
    command: curl -s -H "Authorization: Bearer {{ .Vars.jwt_token }}" /profile
    assertions:
      - json_field:
          path: name
          equals: "Gab"
```

**Why it matters**: Real-world smoke tests almost always need auth flows. Today, cosmo-smoke tests are stateless — each test runs independently. Adding `extract:` + variable injection enables multi-step scenarios (login → use token → verify data) which is the #1 request for API smoke testing tools.

**Design considerations**:
- Variable scope: test-level (default), suite-level (`--set var=val`), env-level
- Extraction sources: `json_field`, `stdout_matches` (capture groups), `http` response headers
- Security: extracted values should NOT appear in terminal output by default (mask sensitive vars)
- Ordering: tests with dependencies must run sequentially (disable parallel for chained groups)

**Verdict**: **High value, should be prioritized.** This is the biggest functional gap for API testing workflows. Moderate complexity (~500 lines for extraction + variable resolution).

#### BR-09: Interactive TUI Test Runner
**What**: A bubbletea-based interactive mode where developers can navigate test results, expand logs, and re-run individual failures:
```
$ smoke run --interactive
  ✅ api-health         120ms
  ✅ db-connection       45ms
  ❌ auth-endpoint      302ms  ← [enter to expand]
  ✅ cache-ping          12ms

  [r] re-run failed  [f] filter  [q] quit
```

**Why it matters**: Current Lipgloss output is pretty but static. Interactive TUI lets developers debug failures without re-running the entire suite. Jest and Vitest popularized this pattern.

**Current state**: cosmo-smoke uses Lipgloss (already in go.mod). Bubbletea would be a new dep but same ecosystem (Charmbracelet). The project explicitly avoided Bubbletea in initial design ("No Viper, no Bubbletea") for simplicity.

**Concern**: CLAUDE.md says "Minimal deps: Cobra + Lipgloss + yaml.v3 + gjson. No Viper, no Bubbletea." This was an intentional design decision. Adding Bubbletea changes the dependency philosophy.

**Verdict**: **Tempting but conflicts with minimal-dep philosophy.** Could be an opt-in build tag (`-tags tui`) like gRPC. Or could revisit the philosophy now that the core is mature (877 tests, 39 assertion types).

#### BR-10: Setup/Teardown Lifecycle Hooks
**What**: Add `before_all`, `after_all`, `before_each`, `after_each` blocks:
```yaml
setup:
  before_all:
    - command: docker compose up -d
      timeout: 30s
  after_all:
    - command: docker compose down
  before_each:
    - command: curl -s http://localhost/health
teardown:
  after_all:
    - command: docker compose down  # guaranteed even on failure
tests:
  - name: my-test
    # ...
```

**Current state**: `prerequisites:` already exists as a pre-test gate (command checks that run before the suite). But there's no teardown, no `after_all`, no `before_each`.

**Why it matters**: Smoke tests against live systems need guaranteed cleanup. The `after_all` with guaranteed execution (even on crash/failure) is the critical gap.

**Design considerations**:
- `after_all` must run even if suite panics (defer-like semantics)
- `before_each` runs before every test (useful for health checks between tests)
- Order: `before_all` → `before_each` per test → test → `after_each` per test → `after_all`
- Failure in `before_all` should skip all tests
- Failure in `after_all` should still report test results

**Verdict**: **High value, complements existing `prerequisites:` mechanism.** Natural evolution. ~400 lines to extend the existing lifecycle.

#### BR-11: Bundle Size / File Size Threshold Assertions
**What**: Assert that built artifacts exist and are within expected size ranges:
```yaml
tests:
  - name: android-apk-size
    file_size:
      path: android/app/build/outputs/apk/release/app-release.apk
      max_bytes: 50000000   # 50MB
  - name: tauri-binary
    file_size:
      path: target/release/bundle/macos/app.app
      max_bytes: 100000000
      min_bytes: 5000000
```

**Current state**: `file_exists` checks presence only. No size validation.

**Why it matters**: For Tauri/React Native projects, bundle size regressions are a real concern. Detecting a 2x size increase in CI is valuable. This is a natural extension of `file_exists`.

**Verdict**: **Low effort, high value for the use case.** ~50 lines to add a `file_size` assertion type that extends `file_exists`. Should be a quick win.

#### BR-12: Simulator/Emulator Health Assertions
**What**: Check if iOS simulators or Android emulators are booted and reachable:
```yaml
tests:
  - name: ios-simulator-ready
    simulator:
      platform: ios
      state: booted
  - name: android-emulator-ready
    simulator:
      platform: android
      state: device  # "device" = booted in adb terminology
```

**Current state**: `process_running` can check if `Simulator` or `qemu` is running, but can't verify the simulator is fully booted and ready. `version_check` could run `xcrun simctl list devices` but doesn't parse the output.

**Why it matters**: React Native test suites often fail because the simulator isn't fully booted. A dedicated assertion that checks `xcrun simctl list devices | grep Booted` (iOS) or `adb shell getprop sys.boot_completed` (Android) would be genuinely useful.

**Verdict**: **Niche but well-scoped.** ~100 lines per platform. Could be a single `simulator` assertion or two separate `ios_simulator` / `android_emulator` assertions. Worth doing if React Native is a target audience.

### Batch 2 Summary

| Deliverable | Verdict | Priority |
|---|---|---|
| BR-07 Wasm plugins | Blue ocean, v2 milestone | **Later** |
| BR-08 Test chaining + data extraction | Biggest functional gap | **High** |
| BR-09 Interactive TUI | Conflicts with minimal-dep philosophy | **Optional** (build tag?) |
| BR-10 Setup/teardown lifecycle | Natural evolution of prereqs | **High** |
| BR-11 File size assertions | Quick win | **Quick win** |
| BR-12 Simulator health | Niche but useful for RN | **Medium** |

---

## Batch 3: Auto-Add Generator Deep Dive (Gemini Round 3)

**Source**: Gemini AI — "how to architect the Auto-Add Generator in Go"

### Context

This expands on the Goss-style observation idea from Batch 1 (Tier 3, item 9). Gemini now provides concrete implementation guidance. Upgrading from "interesting but not urgent" to a more serious proposal given the detailed architecture.

### What Gemini Got Right

The technical approach is sound and pragmatic:

1. **`io.MultiWriter` for stdout capture** — stream to terminal AND buffer simultaneously. No special OS APIs needed. Cross-platform.
2. **Pre/post filesystem snapshot** (not fsnotify) — walk directory before, walk after, diff by hash. Avoids the cross-platform fsnotify mess for file *content* changes.
3. **Intelligent sanitization** — the real value-add. Auto-generated tests fail when they hardcode timestamps, UUIDs, ANSI codes. Stripping these before generating assertions is essential.
4. **`gopsutil/net` for port detection** — check if the spawned PID opened listening ports. Clever and generates `port_listening` assertions automatically.

### What Gemini Missed

1. **cosmo-smoke already has `smoke init`** with 31 project detectors. The gap isn't "can we generate tests?" (we can), it's "can we generate tests from *observed behavior* rather than *project type templates*?" The two approaches complement each other.

2. **No mention of HTTP endpoint discovery.** If the wrapped command starts a server, we could probe common paths (`/health`, `/ready`, `/api`) and auto-generate `http` assertions. More useful than just port detection.

3. **Interactive vs. silent is a false dichotomy.** The right answer is `--interactive` flag (default silent, opt-in interactive). Gemini even asks this question at the end.

4. **The real UX challenge isn't generation — it's confidence.** Developers won't trust auto-generated tests unless they can see *why* each assertion was generated. The output should include comments or a companion `.smoke.reasons.yaml` explaining the reasoning.

### BR-13: Auto-Add Generator (Detailed Design)

**Command**: `smoke observe "node server.js" [--dir ./dist] [--interactive] [--timeout 30s]`

**What it does**:
1. Takes a snapshot of the target directory (if `--dir` provided)
2. Wraps the command with `io.MultiWriter` to capture stdout/stderr
3. Monitors the process PID with `gopsutil/net` for port binding events
4. On exit (Ctrl+C or natural), analyzes the observation:
   - Exit code → `exit_code` assertion
   - Sanitized stdout → `stdout_contains` assertions (first line, last line, key phrases)
   - Sanitized stderr → `stderr_contains` if non-empty
   - New files (post-snapshot diff) → `file_exists` assertions
   - Opened ports → `port_listening` assertions
   - If HTTP port detected → probe `/health`, `/ready` → `http` assertions
5. Writes `.smoke.yaml` with generated tests

**Intelligent Sanitization Pipeline**:
1. Strip ANSI escape codes (regex: `\x1b\[[0-9;]*[a-zA-Z]`)
2. Strip timestamps (`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)
3. Strip UUIDs (`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
4. Strip durations (`\d+\.\d+s` → `...s` or just extract the pattern)
5. Extract first and last meaningful lines (skip blank lines, separators)
6. For long output: find lines containing keywords like "ready", "listening", "started", "complete", "done", "error", "fail"
7. Never generate `stdout_matches` (regex) — always `stdout_contains` (substring)
8. Cap assertions at ~5 per test to avoid overwhelming the generated config

**Filesystem Diffing**:
```go
type FileSnapshot struct {
    Path string
    Size int64
    Hash string // SHA-256 first 8 bytes
}

func Snapshot(dir string) map[string]FileSnapshot { /* filepath.Walk + sha256 */ }
func Diff(before, after map[string]FileSnapshot) []FileSnapshot { /* new or changed files */ }
```

**Port Detection**:
```go
func WatchPorts(pid int32, stop <-chan struct{}) []PortBinding {
    ticker := time.NewTicker(500 * time.Millisecond)
    var seen []PortBinding
    for {
        select {
        case <-ticker.C:
            conns, _ := net.ConnectionsPid("tcp", pid)
            // collect new LISTEN connections
        case <-stop:
            return seen
        }
    }
}
```

**Generated Output Example**:
```yaml
# Generated by: smoke observe "node server.js"
# Observations: exit_code=0, stdout_lines=12, files_created=2, ports_opened=1
tests:
  - name: node-server-exit
    command: node server.js
    timeout: 30s
    assertions:
      - exit_code: 0
      - stdout_contains: "listening on port"
      - stdout_contains: "server started"
  - name: node-server-port
    assertions:
      - port_listening:
          port: 3000
          protocol: tcp
  - name: node-server-health
    assertions:
      - http:
          url: http://localhost:3000/health
          status_code: 200
  - name: node-server-output-files
    assertions:
      - file_exists: dist/bundle.js
      - file_exists: dist/index.html
```

**Interactive Mode** (`--interactive`):
When flag is set, after observation completes, present each detected assertion for confirmation:
```
👀 Observation complete! Detected:

  1. exit_code: 0  →  Include? [Y/n/a(ll)]
  2. stdout contains "listening on port"  →  Include? [Y/n]
  3. port 3000/tcp listening  →  Include? [Y/n]
  4. file dist/bundle.js created (142KB)  →  Include? [Y/n]
  5. HTTP GET http://localhost:3000/health → 200  →  Include? [Y/n]

  [a] accept all  [s] skip all  [e] edit
```

**Implementation Estimate**:
- Core observation engine: ~400 lines (`internal/observer/observer.go`)
- Sanitization pipeline: ~150 lines (`internal/observer/sanitize.go`)
- Filesystem diffing: ~100 lines (`internal/observer/snapshot.go`)
- Port detection: ~80 lines (`internal/observer/ports.go`)
- Command wrapper: ~100 lines (modify existing `exec.Command` pattern)
- Interactive mode: ~150 lines (simple prompts, not bubbletea)
- YAML generation: ~100 lines
- **Total: ~1,080 lines** + new dep `gopsutil`

**Risk Assessment**:
- `gopsutil` is a new dependency (but well-maintained, 10k+ stars)
- Process wrapping with graceful Ctrl+C handling needs careful signal propagation
- Cross-platform filesystem diffing (Windows path separators, symlinks)
- Sanitization will always have edge cases — needs iterative refinement

**Verdict**: **Upgraded from Tier 3 to Tier 2.** The detailed implementation sketch makes this more concrete. Still a significant effort (~1,000 lines) but the value proposition is clear: zero-to-testing in 30 seconds. Best paired with existing `smoke init` — `init` for template-based generation, `observe` for behavior-based generation.

### Batch 3 Summary

| Deliverable | Verdict | Change |
|---|---|---|
| BR-13 Auto-Add Generator | Upgraded to Tier 2, ~1,080 lines | Was Tier 3 in batch 1 |

**Key design decisions to resolve**:
1. New command `smoke observe` or subcommand of `smoke init` (`smoke init --observe`)?
2. `gopsutil` acceptable as a dependency? (violates minimal-dep philosophy but pragmatic)
3. Interactive mode: simple yes/no prompts or full bubbletea TUI?
4. Should observation also detect HTTP endpoints automatically, or only ports?

### BR-13 Design Refinement: Interactive vs Silent Mode (Batch 4)

**Source**: Gemini AI — dual-mode "Auto-Add" generator UX design

**Key new insight**: Use Charmbracelet `huh` (not `bubbletea`) for interactive prompts. `huh` is a purpose-built form/survey library — multi-select checklists, confirmations, text inputs. Much lighter than full bubbletea. Better fit for the "approve these assertions" UX pattern.

**Updated CLI design**:
- `smoke observe "node server.js"` — interactive by default (human in terminal)
- `smoke observe "node server.js" --quiet` — silent, accept all, write directly (CI/pipes)
- `smoke observe "node server.js" --output custom.yaml` — specify target file

**Interactive UX flow** (using `huh`):
1. Observation completes, terminal clears bottom half
2. Multi-select checklist renders with `huh.NewMultiSelect[string]()`
3. Smart defaults: exit_code/always selected, source maps/unselected (known flaky)
4. Spacebar to toggle, Enter to confirm
5. Generates YAML with only selected assertions

**Silent mode safety**:
- Auto-generated comments in YAML explain what was detected
- Each assertion gets a `# Detected: <reason>` comment
- Header comment includes command, timestamp, observation duration

**Architecture** (Strategy Pattern — Gemini got this right):
```go
type SpecGenerator interface {
    Generate(observations *ExecutionDelta) ([]Assertion, error)
}

type InteractiveGenerator struct { /* wraps huh form */ }
type SilentGenerator struct { /* writes all sanitized assertions directly */ }
```

**Dependency assessment**: `huh` (~15KB compiled) vs `bubbletea` (~50KB). `huh` is much lighter and purpose-fit. However, it still adds a Charmbracelet dep. Options:
1. Add `huh` as a main dependency (simplest, but breaks minimal-dep philosophy)
2. Build-tag opt-in (`-tags interactive`) like gRPC (keeps core slim)
3. Use simple `fmt.Scanln` prompts instead (zero deps, ugly but functional)
4. Add `huh` only to the `observe` command (scoped dep, not in hot path)

**Recommendation**: Option 4. The `observe` command is already a "batteries-included" feature (requires `gopsutil` too). Adding `huh` alongside it is reasonable. The core runner stays minimal.

**Smart default rules for interactive mode**:

| Detected Behavior | Default | Reason |
|---|---|---|
| exit_code: 0 | Selected | Almost always wanted |
| stdout key phrases | Selected | Core value |
| Port listening | Selected | Core value |
| New files (non-map) | Selected | Core value |
| New files (.map, .lock) | Unselected | Regenerated each build, flaky |
| HTTP endpoint probe | Selected | If detected, high confidence |
| stderr content | Unselected | May contain warnings that vary |

---

## Batch 5: Wasm Plugin + Lifecycle Hooks Architecture (Gemini Round 4)

**Source**: Gemini AI — detailed blueprints for Wasm extensibility and lifecycle management

### What Gemini Still Gets Wrong

Gemini's context says "the core engine already handles basic assertions (exit code, stdout, file existence)." Still working from the outdated 5-assertion baseline. 39 assertion types, OTel integration, and 31 project detectors exist.

### BR-07 Design Refinement: Wasm Plugin Architecture

**Runtime**: `tetratelabs/wazero` — zero-dependency, pure Go, no CGO. Confirmed as the right choice.

**ABI (Host-Guest Interface)**:
- JSON over memory buffers (not WASI)
- Host → Guest: serialize test context + assertion config as JSON, write to Wasm memory, call `evaluate()`
- Guest → Host: return pointer/size to JSON result `{pass: bool, message: string}`
- Gemini's approach is sound but raw pointer arithmetic is error-prone. Consider WASI as alternative for simpler stdin/stdout I/O.

**YAML specification** (Gemini's proposal):
```yaml
plugins:
  custom_db_check: "./plugins/postgres_ping.wasm"
tests:
  check_database:
    command: "docker-compose up -d db"
    expect:
      custom_db_check:
        host: "localhost"
        port: 5432
        timeout: "5s"
```

**Assessment**: Clean API surface. The `plugins:` top-level section registers Wasm modules, then they're usable as assertion types by name. Plugin caching in `PluginManager` avoids recompilation per test.

**Missing from Gemini's design**:
1. **Plugin versioning** — how does `plugins:` handle versioned Wasm files?
2. **Sandboxing** — what resources can Wasm access? Network? Filesystem? Time limit?
3. **Error taxonomy** — plugin crash vs assertion fail vs timeout
4. **WASI alternative** — raw memory buffers vs WASI stdin/stdout. WASI is simpler but slightly more overhead.

**Updated implementation estimate**: ~800 lines (was ~500). The caching, memory management, and error handling add significant surface area.

### BR-10 Design Refinement: Lifecycle Hooks

**Core architectural insight from Gemini**: Use `context.Context` as the backbone. Every hook/test gets a context with timeout. Cancellation triggers immediate teardown.

**New concept: Environment Passing** (Gemini's most valuable contribution):
- `before_all` command outputs key-value pairs (e.g., `echo "PORT=8080"`)
- Engine captures stdout, parses `KEY=VALUE` pairs
- Injects into `os.Environ()` context for subsequent tests
- This solves the "dynamic port assignment" problem elegantly

**Signal handling** (critical for CI):
- `os/signal` listener for `SIGINT`, `SIGTERM`
- Intercept → execute `after_all` hooks → exit gracefully
- `always_run: true` flag on hooks ensures cleanup even on abort

**YAML specification** (Gemini's proposal):
```yaml
setup:
  before_all:
    - command: "docker-compose up -d"
      timeout: "30s"
  after_all:
    - command: "docker-compose down"
      always_run: true
```

**Tension with existing `prerequisites:` pattern**:
- Current: `prerequisites:` runs commands before the suite, fails fast if any don't pass
- Proposed: `setup.before_all:` runs commands before the suite, with timeout and env capture
- These overlap significantly. Options:
  1. Replace `prerequisites:` with `setup.before_all:` (breaking change)
  2. Keep both, document that `prerequisites:` is for validation, `setup.before_all:` is for provisioning
  3. Merge: extend `prerequisites:` with `always_run`, `env_pass`, and `after_all` support

**Recommendation**: Option 3. Extend the existing `prerequisites:` mechanism rather than introducing a parallel `setup:` section. Less conceptual overhead, backward compatible.

**Execution order** (confirmed):
```
before_all → (before_each → test → after_each)×N → after_all
```

**Updated implementation estimate**: ~500 lines (extending existing `prereq.go`).

---

## Batch 6: Flakiness Detector / Stress Testing (Gemini Round 5)

**Source**: Gemini AI — architectural blueprint for `smoke stress` command

### BR-02 Design Refinement: Stress Testing Engine

This is a genuinely excellent blueprint. Gemini nailed the hard problems:

**1. Worker Pool vs Naive Parallelism**
- Bounded worker pool with configurable concurrency
- `--workers 1` = sequential (safe for tests with shared state like port binding)
- `--workers 5` = 5 concurrent goroutines
- Prevents port exhaustion and accidental DoS on local services

**CLI design**:
```bash
smoke stress <test-name> --runs 100 --workers 5 --fail-fast=false
```

| Flag | Default | Purpose |
|---|---|---|
| `--runs` | 50 | Total executions |
| `--workers` | 1 | Concurrency (1 = sequential) |
| `--fail-fast` | false | Stop on first failure |

**2. Concurrency Safety**
- `sync/atomic` for counters (TotalRuns, Passes, Fails) — lock-free
- Buffered `chan TestResult` for result collection
- **Error deduplication**: group identical failure messages. If same error occurs 15 times, show `[15 times]: exit_code expected 0, got 1` — not 15 lines. This is a genuinely smart UX decision.

**3. Lifecycle Integration**
- `before_all` runs once (start Docker, etc.)
- `before_each` + `after_each` run per iteration (reset state between runs)
- `after_all` runs once (cleanup)
- This is consistent with the BR-10 lifecycle design

**4. Terminal UX**
- Progress bar: `[========>---] 75/100 Runs | ✅ 72 | ❌ 3`
- Suppress stdout/stderr during runs (send to `io.Discard` or buffer)
- Only preserve output from **failed** runs for the summary
- Use Charmbracelet `bubbles/progress` or `huh/spinner`

**5. Summary Output**
```
🔥 Stress Test Complete: check_api_health
--------------------------------------------------
Total Runs: 100
Concurrency: 5 workers
Duration:   14.2s
Reliability: 95% (Flaky ⚠️)

❌ Failures Detected (5 runs):
  - [3 times]: exit_code expected 0, got 1
  - [2 times]: stdout: missing "database connected" (Timeout)

Tip: Try increasing the wait time in your before_each hook.
```

**Reliability thresholds**:
| Rate | Status |
|---|---|
| 100% | ✅ Stable |
| 95-99% | ⚠️ Flaky |
| <95% | ❌ Unreliable |

**What Gemini missed**:
1. **No mention of `--json` output** for programmatic consumption in CI
2. **No mention of exit code semantics** — should `smoke stress` exit 0 for stable, 1 for flaky, 2 for unreliable?
3. **Heat map output** — could show failure timing patterns (all failures in first 10 runs? clustered? random?)
4. **`--compare` flag** — compare two configs or two versions of the same test

**Implementation estimate**:
- `cmd/stress.go`: ~150 lines (Cobra command, flag parsing)
- `internal/runner/stress.go`: ~400 lines (worker pool, result collection, dedup)
- Progress bar: ~80 lines (bubbles/progress or simple spinner)
- Summary formatting: ~100 lines
- **Total: ~730 lines**

**Dependency note**: Progress bar adds either `bubbles` or `huh`. Same minimal-dep tension as BR-13. Could use simple `\r` overwrites for zero-dep progress.

### Batch 5-6 Summary

| Deliverable | Update | Key Addition |
|---|---|---|
| BR-07 Wasm plugins | Design refined | ABI details, plugin caching, ~800 lines |
| BR-10 Lifecycle hooks | Design refined | Context backbone, env passing, signal handling, ~500 lines |
| BR-02 Stress testing | Design refined | Worker pools, error dedup, reliability scoring, ~730 lines |

**Cumulative line estimate for all three**: ~2,030 lines. Significant but well-scoped.

---

## Batch 7: Final — Stack-Specific DX, Multi-Process, Data Contract (Gemini Final)

**Source**: Gemini AI — "final architectural directives" for cross-platform development

### What Gemini Got Wrong (Already Implemented)

| Gemini Suggestion | cosmo-smoke Already Has |
|---|---|
| Stack-specific `detector` package | `internal/detector/` with 31 project types including Tauri, React Native, Next.js, Flutter, iOS, Android |
| Structured JSON reporter (`reporter/json.go`) | `internal/reporter/json.go` — full JSON output with test results, durations, statuses |
| Dashboard for visualization | `smoke serve --dashboard` with SQLite storage, API handlers, embedded UI |
| Project type auto-detection for scaffolding | `smoke init` detects project and generates tailored templates |

**Score: 4/6 already implemented.** Gemini's final batch is the weakest on novelty — most of it exists.

### Genuinely New Ideas

#### BR-14: Background Commands with `wait_for_port`

**What**: Lifecycle hooks that support background process execution with port-readiness polling:
```yaml
setup:
  before_all:
    - command: "npm run tauri dev"
      background: true
      wait_for_port: 1420
      timeout: "30s"
```

**Why it matters**: The #1 pain point for smoke testing dev servers is "when is it ready?" Arbitrary `sleep` is fragile. Port polling until a TCP connection succeeds is deterministic. This is critical for:
- Tauri dev (port 1420)
- React Native Metro (port 8081)
- Next.js dev (port 3000)
- Any service that needs startup time

**Implementation sketch**:
- Extend lifecycle command schema with `background: bool`, `wait_for_port: int`, `startup_timeout: duration`
- `background: true` → `exec.Command.Start()` (non-blocking) instead of `Run()`
- `wait_for_port` → poll TCP connection in a loop with exponential backoff (50ms, 100ms, 200ms... up to timeout)
- Store PID for cleanup in `after_all`
- If `wait_for_port` timeout elapses → fail `before_all`, skip all tests

**Effort**: ~200 lines. Reuses existing `port_listening` assertion logic internally.

**Standalone utility**: `wait_for_port` should also be available as a top-level config option (not just in hooks):
```yaml
wait_for:
  - port: 5432
    host: localhost
    timeout: 10s
  - port: 1420
    timeout: 30s
```

#### BR-15: Detector-Observer Integration

**What**: When `smoke observe` runs, use the existing detector package to identify the project type, then tailor observation heuristics:

| Detected Stack | Auto-observe targets |
|---|---|
| Tauri | Watch `target/release/bundle/`, port 1420, Rust + Vite processes |
| React Native | Watch Metro port 8081, `android/app/build/`, `ios/build/` |
| Next.js | Watch `.next/` directory, port 3000, Node process |
| Go | Watch binary output, `go test` exit codes |
| Docker | Watch container startup, health check endpoints |

**Current gap**: `smoke init` uses detectors for template generation. `smoke observe` (BR-13) doesn't use them yet. The integration is: detect project → know what to observe → generate better assertions.

**Effort**: ~150 lines of integration code, reusing existing `detector.DetectProject()` output.

### Enhanced JSON Schema (Already Partially Implemented)

Gemini proposes a phases-aware JSON schema:
```json
{
  "suite": "cosmo-smoke-tauri-run",
  "status": "passed",
  "duration_ms": 14502,
  "phases": {
    "setup": { "duration_ms": 4200, "status": "passed" },
    "tests": [...]
  }
}
```

**Current state**: JSON reporter exists but doesn't have `phases` structure. Tests are flat. Adding phases would require:
1. Track `before_all`/`after_all` durations as phase entries
2. Add `type` field to tests (standard, wasm_plugin, etc.)
3. Add `metrics` map for arbitrary key-value data

**Verdict**: Nice enhancement to existing JSON reporter. ~100 lines to add phase tracking when lifecycle hooks are implemented (BR-10). Not a standalone feature — it follows from BR-10.

### Batch 7 Summary

| Deliverable | Verdict | Priority |
|---|---|---|
| BR-14 Background commands + wait_for_port | Genuinely valuable for multi-process setups | **High** |
| BR-15 Detector-observer integration | Natural coupling of existing systems | **Medium** |
| Enhanced JSON schema with phases | Follows from BR-10 lifecycle implementation | **Low** (derivative) |

---

## Final Consolidated Analysis

### All Deliverables Ranked by Priority

| Rank | ID | Feature | Effort | Impact | Dependencies |
|---|---|---|---|---|---|
| 1 | BR-08 | Test chaining + data extraction | ~500 lines | **Critical** — enables auth flows | None |
| 2 | BR-14 | Background commands + wait_for_port | ~200 lines | **High** — solves dev server problem | BR-10 (lifecycle) |
| 3 | BR-10 | Setup/teardown lifecycle hooks | ~500 lines | **High** — extends existing prereqs | None |
| 4 | BR-11 | File size assertions | ~50 lines | **Quick win** — extends file_exists | None |
| 5 | BR-01 | GitHub Actions native output | ~200 lines | **High** — CI visibility | None |
| 6 | BR-02 | Flakiness detector (smoke stress) | ~730 lines | **Medium** — quality tooling | BR-10 (lifecycle) |
| 7 | BR-03 | Remote config inheritance (extends: URL) | ~400 lines | **High** — portfolio-scale | None |
| 8 | BR-06 | Official GitHub Action | ~100 lines | **High** — distribution | BR-01 (GHA output) |
| 9 | BR-13 | Auto-Add Generator (smoke observe) | ~1,080 lines | **Medium** — DX, zero-to-testing | BR-15 (detector integration) |
| 10 | BR-15 | Detector-observer integration | ~150 lines | **Medium** — enhances BR-13 | BR-13 |
| 11 | BR-12 | Simulator/emulator health | ~200 lines | **Low** — niche RN/Tauri | None |
| 12 | BR-05 | Backstage.io output | ~150 lines | **Low** — enterprise | None |
| 13 | BR-09 | Interactive TUI | ~400 lines | **Low** — conflicts with philosophy | New dep (huh/bubbletea) |
| 14 | BR-07 | Wasm plugin system | ~800 lines | **Later** — ecosystem play | wazero dep |
| 15 | BR-04 | OIDC integration | ~500 lines | **Later** — enterprise security | AWS/GCP SDK |

### Recommended Implementation Phases

**Phase 1 — Quick Wins + Core Gaps** (~750 lines)
- BR-11: File size assertions (50 lines)
- BR-08: Test chaining + data extraction (500 lines)
- BR-01: GitHub Actions output (200 lines)

**Phase 2 — Lifecycle & Orchestration** (~1,200 lines)
- BR-10: Setup/teardown lifecycle (500 lines)
- BR-14: Background commands + wait_for_port (200 lines)
- BR-03: Remote config inheritance (400 lines)
- BR-06: Official GitHub Action (100 lines)

**Phase 3 — Quality Tooling** (~730 lines)
- BR-02: Flakiness detector / smoke stress (730 lines)

**Phase 4 — DX Features** (~1,380 lines)
- BR-13: Auto-Add Generator (1,080 lines)
- BR-15: Detector-observer integration (150 lines)
- BR-12: Simulator health (150 lines)

**Phase 5 — Ecosystem & Enterprise** (~1,450 lines)
- BR-07: Wasm plugins (800 lines)
- BR-04: OIDC (500 lines)
- BR-05: Backstage.io (150 lines)

**Total estimated: ~5,510 lines across all phases**

### Gemini Feedback Quality Score

| Batch | New Ideas | Already Implemented | Quality |
|---|---|---|---|
| Batch 1 (Ecosystem) | 4/16 | 15/16 | Good landscape, stale product knowledge |
| Batch 2 (Advanced DX) | 6/10 | 6/10 | Better — stateful chaining was genuinely new |
| Batch 3 (Auto-Add deep dive) | 1/1 | 0/1 | Excellent — detailed implementation guidance |
| Batch 4 (Interactive vs Silent) | 0/1 | 0/1 | Refinement only — `huh` suggestion was useful |
| Batch 5 (Wasm + Lifecycle) | 2/2 | 0/2 | Strong architecture, good Go patterns |
| Batch 6 (Stress Testing) | 1/1 | 0/1 | Best blueprint — worker pools, error dedup |
| Batch 7 (Final) | 2/6 | 4/6 | Weakest batch — most already existed |

**Overall**: Gemini produced 15 deliverable ideas across 7 batches. Of these, ~10 are genuinely new. The strongest contributions were the stress testing architecture (batch 6), the Auto-Add generator design (batch 3), and test chaining with data extraction (batch 2). The weakest contributions assumed cosmo-smoke only has basic assertion types.
