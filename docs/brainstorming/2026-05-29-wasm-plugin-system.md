---
date: "2026-05-29T10:00:00-03:00"
source: brainstorm session
status: brainstorm
issue: FEAT-048
related:
  - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
deliverables:
  - id: BR-01
    title: "ABI contract: WASI stdin/stdout primary (v1), shared-memory deferred (v2)"
  - id: BR-02
    title: "Plugin manifest schema and discovery mechanism"
  - id: BR-03
    title: "PluginManager lifecycle: compile, cache, instantiate, teardown"
  - id: BR-04
    title: "Sandbox policy: capability-based resource access model"
  - id: BR-05
    title: "Error taxonomy: crash vs fail vs timeout vs ABI mismatch"
  - id: BR-06
    title: "Schema and runner integration: Expect.Plugin field, evaluation wiring"
  - id: BR-07
    title: "Reporter integration: plugin assertions in TestResultData/AssertionDetail"
  - id: BR-08
    title: "Build tag strategy: always-included vs -tags wasm"
  - id: BR-09
    title: "Plugin testing story: local dev, debug, CI"
  - id: BR-10
    title: "Reference plugin examples: Rust, Go (TinyGo), AssemblyScript"
  - id: BR-11
    title: "Performance model: compilation cache, instance pooling, memory limits"
  - id: BR-12
    title: "Validation integration: plugin existence checks, ABI version probing"
---

# FEAT-048: Wasm Plugin System for Custom Assertions

## Problem

SmokeSig currently has 45+ built-in assertion types, each following the same implementation pattern: a field on the `Expect` struct in `internal/schema/schema.go`, a nil-check + `Check*()` call in the 350-line assertion evaluation block in `internal/runner/runner.go` (lines 442-813), an entry in `hasStandaloneAssertions()` in `internal/schema/validate.go`, and corresponding test coverage. Adding a new assertion type requires a PR to SmokeSig, a release cycle, and adoption by downstream consumers.

This creates two scaling problems:

1. **Contributor friction**: Every domain-specific assertion (Consul health, Vault seal status, RabbitMQ queues, custom internal APIs) must go through the SmokeSig release pipeline, even when only one team needs it.

2. **Struct bloat**: The `Expect` struct has 44 typed pointer fields. Each new assertion adds a field, a check type to `ExportSchema()`, a case in the runner evaluation block, and a boolean term in `hasStandaloneAssertions()`. The evaluation block is already a 350-line if-chain. This pattern does not scale to 100+ assertion types.

Wasm plugins let users write custom assertions in any language that compiles to WebAssembly (Rust, Go via TinyGo, AssemblyScript, C/C++, Zig), deploy them as `.wasm` files alongside their `.smokesig.yaml`, and reference them by name. SmokeSig evaluates them in a sandboxed runtime with deterministic resource limits.

**No other smoke test tool offers this.** This is a blue-ocean ecosystem play.

## Prior Art (BR-07 from Gemini Ecosystem Feedback)

The initial concept was captured in `docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md` as BR-07. Key decisions from that document:

- Runtime: `tetratelabs/wazero` (zero-dep, pure Go, no CGO) -- confirmed
- ABI: JSON over memory buffers -- needs refinement (see Decision 1)
- YAML: `plugins:` top-level section maps names to `.wasm` paths -- confirmed direction
- Caching: `PluginManager` avoids recompilation per test -- confirmed
- Estimate: ~800 lines -- updated below
- Open questions: plugin versioning, sandboxing details, error taxonomy, WASI alternative

This document resolves all open questions and produces a complete implementation spec.

---

## Design Decisions

### Decision 1: ABI Choice (BR-01)

**Question**: How do the host (SmokeSig Go binary) and guest (Wasm plugin) exchange data?

**Options analyzed**:

| Option | Mechanism | Pros | Cons |
|--------|-----------|------|------|
| A. JSON-over-memory | Host allocates Wasm memory, writes JSON, calls `evaluate(ptr, len)`. Guest returns `(ptr, len)` to result JSON. | Fast (no I/O syscalls), wazero has good memory APIs | Raw pointer arithmetic in guest code, error-prone for plugin authors |
| B. WASI stdin/stdout | Host writes JSON to stdin pipe, guest reads stdin, writes result to stdout | Dead simple for plugin authors (`read stdin, write stdout`), standard WASI | I/O overhead per invocation, harder to enforce timeout mid-read |
| C. Hybrid: shared-memory with helper SDK | Same as A, but SmokeSig publishes tiny guest-side SDKs (Rust crate, npm package) that hide the pointer math | Fast + ergonomic for plugin authors | Maintenance burden: SDKs for each language |

**Decision: Option B (WASI stdin/stdout) as v1 primary ABI. Option C (shared-memory with SDK) deferred to v2.**

For v1, the primary ABI is WASI stdin/stdout. The host writes input JSON to the plugin's stdin, and the plugin writes its result JSON to stdout. This is universal across all languages that compile to Wasm -- plugin authors use stdin/stdout, which requires no SDK, no pointer arithmetic, and no language-specific tooling. A Rust plugin is just `read stdin, parse JSON, write JSON to stdout`. A TinyGo plugin is the same. Even a C plugin can do it with `fgets`/`printf`.

The v1 ABI version is `1` (WASI stdin/stdout). Plugins export `smokesig_abi_version` as a global set to `1`.

**Shared-memory ABI (v2)**: When guest-side SDKs are published (Rust crate, Go module, AS package), v2 introduces JSON-over-shared-memory for performance. Plugins opting into v2 export `smokesig_abi_version` set to `2` and use the SDK to hide `alloc`/`dealloc` pointer arithmetic. This is ~3x faster but requires an SDK -- acceptable only after SDKs are published and battle-tested.

```
// v2 example (deferred): Rust guest SDK (smokesig-plugin-sdk crate)
use smokesig_sdk::{PluginInput, PluginOutput, register};

#[register]
fn evaluate(input: PluginInput) -> PluginOutput {
    let host = input.config["host"].as_str().unwrap_or("localhost");
    // ... custom check logic ...
    PluginOutput::pass("connection OK")
}
```

The SDK hides `alloc`, `dealloc`, and pointer arithmetic. Plugin authors never touch raw memory. This is a v2 concern -- v1 plugins use stdin/stdout with no SDK dependency.

**Wire format (both modes)**:

Host-to-guest input:
```json
{
  "abi_version": 1,
  "assertion_name": "consul_health",
  "config": {
    "host": "consul.local",
    "service": "api-gateway",
    "passing_only": true
  },
  "context": {
    "test_name": "check-consul",
    "exit_code": 0,
    "stdout": "...",
    "stderr": "...",
    "duration_ms": 142,
    "env": {}
  }
}
```

**Security invariant**: `context.env` MUST always be `{}` (empty object). Plugins that need environment variables must use the `host_env_get` host function, which requires the `env` capability grant. This keeps the capability model consistent -- the host controls exactly which env vars a plugin can access, one at a time, gated by capability. Passing `os.Environ()` would leak secrets to untrusted plugins -- this is a security invariant, not an optimization.

Guest-to-host output:
```json
{
  "pass": true,
  "message": "service api-gateway has 3 healthy instances",
  "details": [
    {"type": "consul_health", "expected": "passing", "actual": "3 passing, 0 critical", "pass": true}
  ],
  "error": null
}
```

The `details` array maps directly to `[]AssertionResult` / `[]reporter.AssertionDetail`. Plugins can return multiple sub-assertions (like `CheckHTTP` and `CheckGraphQL` do today).

**v1 protocol (WASI stdin/stdout)**:

1. Host configures WASI with stdin piped to input JSON bytes
2. Host instantiates the module with WASI — the module's `_start` (main) runs
3. Plugin reads JSON from stdin, evaluates, writes result JSON to stdout
4. Host reads stdout as the result JSON
5. Module instance is closed

Guest must export: `_start` (standard WASI entrypoint), `smokesig_abi_version` (global, value `1`).

**v2 protocol (shared-memory, deferred)**:

1. Host calls guest-exported `smokesig_alloc(size) -> ptr` to allocate input buffer
2. Host writes JSON bytes to `[ptr, ptr+size)` in guest memory
3. Host calls guest-exported `evaluate(ptr, len) -> u64` where result is `(result_ptr << 32) | result_len`
4. Host reads `[result_ptr, result_ptr+result_len)` as JSON
5. Host calls guest-exported `smokesig_dealloc(ptr, len)` to free both buffers

v2 guest must export: `smokesig_alloc`, `smokesig_dealloc`, `evaluate`, `smokesig_abi_version` (global, value `2`).

### Decision 2: Plugin Discovery and Registration (BR-02)

**Question**: Where do `.wasm` files come from, and how are they registered?

**Decision: Local files only for v1. URL download deferred to v2.**

Rationale: A plugin registry (download from URL, verify checksums, cache globally) is a significant feature on its own. v1 keeps it simple: `.wasm` files live in the project, referenced by path.

**YAML config**:

```yaml
# Top-level plugins section in .smokesig.yaml
plugins:
  consul_health:
    path: "./plugins/consul-health.wasm"
    version: "1.0.0"                    # optional, for user tracking
    timeout: 5s                         # per-invocation timeout override
    capabilities: [network]             # explicit capability grant (see Decision 4)

  vault_seal:
    path: "./plugins/vault-seal.wasm"
```

Paths are resolved relative to the config file directory (consistent with SmokeSig's existing `config-dir-relative` behavior for `file_exists`, `credential_check`, etc.).

**Plugin manifest** (embedded in the .wasm or sidecar JSON):

For v1, metadata lives in the YAML config (above). For v2, plugins can embed a manifest via a `smokesig_manifest` exported function that returns JSON:

```json
{
  "name": "consul_health",
  "version": "1.0.0",
  "author": "CosmoLabs",
  "description": "Verify Consul service health",
  "abi_version": 1,
  "input_schema": {
    "type": "object",
    "required": ["service"],
    "properties": {
      "host": {"type": "string", "default": "localhost"},
      "port": {"type": "integer", "default": 8500},
      "service": {"type": "string"},
      "passing_only": {"type": "boolean", "default": true}
    }
  },
  "capabilities": ["network"]
}
```

The `input_schema` enables `smokesig validate` to type-check plugin assertion configs without running the plugin. The `capabilities` field declares what the plugin needs (see Decision 4).

### Decision 3: PluginManager Lifecycle (BR-03)

**Question**: How are Wasm modules compiled, cached, and instantiated?

```
                    ┌─────────────┐
                    │ .wasm file  │
                    └──────┬──────┘
                           │ Load (os.ReadFile)
                           ▼
                    ┌─────────────┐
                    │ wazero      │
                    │ Compile()   │ ← cached by file hash (SHA-256)
                    └──────┬──────┘
                           │ CompiledModule
                           ▼
              ┌────────────┴────────────┐
              │   Instance Pool (N=4)   │ ← pre-warmed per plugin
              │  ┌─────┐ ┌─────┐       │
              │  │ Inst │ │ Inst │ ...  │
              │  └─────┘ └─────┘       │
              └────────────┬────────────┘
                           │ Checkout
                           ▼
                    ┌─────────────┐
                    │  evaluate() │ ← per-test invocation
                    └─────────────┘
```

**PluginManager struct**:

```go
type PluginManager struct {
    runtime  wazero.Runtime
    compiled map[string]wazero.CompiledModule  // keyed by SHA-256 of .wasm bytes
    configs  map[string]PluginConfig            // keyed by plugin name
    mu       sync.RWMutex
}

type PluginConfig struct {
    Path         string
    Timeout      time.Duration
    Capabilities []string
    ABIVersion   int  // 1 = WASI stdin/stdout, 2 = shared-memory (v2)
}
```

**Lifecycle**:

1. **Init** (`NewPluginManager`): Create wazero runtime with `wazero.NewRuntimeConfig().WithCloseOnContextDone(true)`. Set memory limit (default 16MB per module).
2. **Load** (`LoadPlugin(name, config)`): Read `.wasm` bytes, SHA-256 hash, check compiled cache. If miss, `runtime.CompileModule()`. Probe for `smokesig_abi_version` export to determine ABI mode (1 = WASI stdin/stdout, 2 = shared-memory).
3. **Evaluate** (`Evaluate(name, input) -> PluginResult, error`): Instantiate module (wazero handles this efficiently for compiled modules), call `evaluate()` with context timeout, deserialize result, close instance.
4. **Close** (`Close()`): Close all compiled modules, close runtime. Called in `Runner` teardown.

**Compilation caching**: wazero supports filesystem-based compilation caching via `wazero.NewCompilationCacheWithDir(path)`. SmokeSig will use `$XDG_CACHE_HOME/smokesig/wasm/` (or `~/.cache/smokesig/wasm/`) to persist compiled artifacts across runs. This makes second-run startup near-zero for large plugins.

**Instance pooling**: Deferred to v2. wazero module instantiation is fast (~100us). Pooling adds complexity (instance state isolation, pool sizing) for marginal gain in smoke test workloads (tests run sequentially or with limited parallelism).

### Decision 4: Sandbox Policy (BR-04)

**Question**: What host resources can plugins access?

**Decision: Deny-by-default, explicit capability grants.**

Wasm modules run in a memory-isolated sandbox by default (this is inherent to WebAssembly). The question is what *host functions* SmokeSig exposes.

**Capability model**:

| Capability | What it grants | Use case |
|------------|---------------|----------|
| (none) | Pure computation only. Access to input JSON and nothing else. | Parsing, validation, math |
| `network` | `host_http_get(url, headers) -> response` and `host_http_post(url, body, headers) -> response` host functions | Consul, Vault, custom API checks |
| `env` | `host_env_get(name) -> value` host function | Reading API keys, config |
| `fs_read` | `host_fs_read(path) -> bytes` host function (relative to config dir only, no `..` escape) | Reading local files, certs |
| `exec` | `host_exec(cmd, args, timeout) -> stdout, stderr, exit_code` host function | Running local tools |
| `time` | `host_time_now() -> unix_ms` host function | Measuring external durations |

**Security rules**:

- `exec` capability requires explicit opt-in per-plugin AND a global `settings.allow_plugin_exec: true` in the config. Double gate.
- `fs_read` is jailed to the config directory. Path traversal (`../`) is rejected by the host function.
- `network` host functions respect the test's timeout. No infinite requests.
- WASI mode (ABI v1, the default) does NOT auto-grant `env` or `time` capabilities despite WASI providing those syscalls. SmokeSig configures the WASI environment with an empty env and no clock access by default. Plugins must request `env` and `time` capabilities explicitly, same as `network`, `fs_read`, and `exec`.
- Memory limit: 16MB default per module, configurable via `settings.plugin_memory_mb`.

**Host function signatures** (Go side, registered with wazero):

```go
// Registered as "smokesig" module in wazero
func hostHTTPGet(ctx context.Context, mod api.Module, urlPtr, urlLen, headersPtr, headersLen uint32) uint64
func hostHTTPPost(ctx context.Context, mod api.Module, urlPtr, urlLen, bodyPtr, bodyLen, headersPtr, headersLen uint32) uint64
func hostEnvGet(ctx context.Context, mod api.Module, namePtr, nameLen uint32) uint64
func hostFSRead(ctx context.Context, mod api.Module, pathPtr, pathLen uint32) uint64
func hostExec(ctx context.Context, mod api.Module, cmdPtr, cmdLen, argsPtr, argsLen, timeoutMs uint32) uint64
func hostTimeNow(ctx context.Context, mod api.Module) uint64
```

All host functions return `(ptr << 32) | len` pointing to a JSON response in guest memory (allocated via `smokesig_alloc`).

### Decision 5: Error Taxonomy (BR-05)

**Question**: How does SmokeSig distinguish between different plugin failure modes?

| Error Type | Cause | AssertionResult.Type | AssertionResult.Passed | Runner behavior |
|------------|-------|---------------------|----------------------|-----------------|
| **assertion_fail** | Plugin returned `{pass: false, ...}` | `"plugin:<name>"` | `false` | Normal assertion failure. Reported like any other failed assertion. |
| **assertion_pass** | Plugin returned `{pass: true, ...}` | `"plugin:<name>"` | `true` | Normal pass. |
| **plugin_timeout** | Plugin exceeded timeout | `"plugin:<name>:timeout"` | `false` | Test fails. Message: "plugin exceeded Xs timeout". |
| **plugin_crash** | Wasm trap (OOB memory, unreachable, stack overflow) | `"plugin:<name>:crash"` | `false` | Test fails. Message includes wazero trap reason. |
| **abi_mismatch** | Missing exports, wrong ABI version, malformed JSON output | `"plugin:<name>:abi_error"` | `false` | Test fails. Message identifies what's wrong (missing `evaluate` export, etc.). |
| **plugin_not_found** | `.wasm` file missing or unreadable | N/A | N/A | **Validation error** (caught by `smokesig validate`, before any test runs). |
| **capability_denied** | Plugin called host function it wasn't granted | `"plugin:<name>:denied"` | `false` | Test fails. Message: "plugin called network but capability not granted". |

**Key principle**: Plugin errors are *assertion-level*, not suite-level. A crashing plugin fails its test, not the entire suite (unless `--fail-fast` is set). This matches how built-in assertion errors (invalid regex in `stdout_matches`, unreachable host in `http`) are handled today.

### Decision 6: Schema and Runner Integration (BR-06)

**Question**: How do plugin assertions integrate with the existing `Expect` struct and evaluation block?

**Current pain point**: The `Expect` struct has 44 typed fields. Adding plugin support as another field would be wrong -- the whole point is to *avoid* modifying `Expect` for every new assertion type.

**Decision: Add a single `Plugin` map field to `Expect`.**

```go
// In internal/schema/schema.go, add to Expect struct:
type Expect struct {
    // ... existing 44 fields ...

    // Plugin holds plugin assertion configs, keyed by registered plugin name.
    // Values are arbitrary YAML that gets passed to the plugin as config JSON.
    Plugin map[string]interface{} `yaml:"plugin,omitempty"`
}
```

**YAML usage**:

```yaml
plugins:
  consul_health:
    path: "./plugins/consul-health.wasm"
    capabilities: [network]

tests:
  - name: check consul
    run: "echo ok"
    expect:
      exit_code: 0
      plugin:
        consul_health:
          service: "api-gateway"
          passing_only: true
```

This is elegant: `plugin:` is a single field on `Expect`, but it's a map -- so it supports unlimited plugin assertions per test. Each key maps to a registered plugin name, and the value is passed as the `config` field in the plugin input JSON.

**Runner integration** (addition to the evaluation block in `runner.go`):

```go
// After all built-in assertions, at ~line 803 (after DocIntegrity):
if len(t.Expect.Plugin) > 0 && r.plugins != nil {
    for name, config := range t.Expect.Plugin {
        pluginInput := PluginInput{
            ABIVersion:    1,
            AssertionName: name,
            Config:        config,
            Context: PluginContext{
                TestName:   t.Name,
                ExitCode:   exitCode,
                Stdout:     stdout.String(),
                Stderr:     stderr.String(),
                DurationMs: int(time.Since(start).Milliseconds()),
            },
        }
        result, err := r.plugins.Evaluate(ctx, name, pluginInput)
        if err != nil {
            // Plugin crash/timeout/ABI error
            assertions = append(assertions, AssertionResult{
                Type:     fmt.Sprintf("plugin:%s:%s", name, classifyError(err)),
                Expected: "plugin success",
                Actual:   err.Error(),
                Passed:   false,
            })
            allPassed = false
        } else {
            for _, detail := range result.Details {
                assertions = append(assertions, AssertionResult{
                    Type:     fmt.Sprintf("plugin:%s", detail.Type),
                    Expected: detail.Expected,
                    Actual:   detail.Actual,
                    Passed:   detail.Pass,
                })
                if !detail.Pass {
                    allPassed = false
                }
            }
        }
    }
}
```

### Deterministic Iteration

Go map iteration is non-deterministic. ALL code that iterates `Expect.Plugin` MUST collect keys, sort them with `sort.Strings()`, then iterate in sorted order. This applies to: runner evaluation, validation, schema export, and any reporter output.

```go
names := make([]string, 0, len(t.Expect.Plugin))
for name := range t.Expect.Plugin {
    names = append(names, name)
}
sort.Strings(names)
for _, name := range names {
    config := t.Expect.Plugin[name]
    // ... evaluate ...
}
```

**Runner struct change**: Add `plugins *PluginManager` field to `Runner`. Initialize in `Run()` if `Config.Plugins` is non-empty. Close in defer.

**Validation integration** (BR-12): In `validate.go`, add checks:
- Every key in `test.Expect.Plugin` must reference a name in `Config.Plugins`
- Every `Config.Plugins[name].Path` must point to an existing `.wasm` file
- If plugin exports `smokesig_manifest`, validate `input_schema` against the config values (v2)

**hasStandaloneAssertions update**: Add `len(e.Plugin) > 0` to the function (line 249 of validate.go).

### Decision 7: Reporter Integration (BR-07)

**Question**: How do plugin results flow through the reporter pipeline?

**Answer: Zero changes needed.** Plugin assertions produce `AssertionResult` structs with `Type: "plugin:<name>"`. The existing `toReporterResult()` function (line 831 of runner.go) converts these to `reporter.AssertionDetail` by copying `Type`, `Expected`, `Actual`, `Passed`. All reporters (terminal, JSON, JUnit, TAP, Prometheus, GHA, Backstage, Push, Webhook) consume `AssertionDetail` generically -- none switch on the `Type` field.

The `ExportSchema()` function in `internal/schema/export.go` will need a section for loaded plugins (name, version, capabilities, input_schema if available), but the reporter pipeline is inherently plugin-compatible.

**Terminal reporter rendering**: Plugin assertions will show as:

```
  ✓ plugin:consul_health — service api-gateway has 3 healthy instances
  ✗ plugin:vault_seal — expected sealed=false, got sealed=true
```

The `"plugin:"` prefix distinguishes them visually from built-in assertions.

### Decision 8: Build Tag Strategy (BR-08)

**Question**: Should Wasm support be behind a build tag (`-tags wasm`) or always included?

**Analysis**:

| Factor | Always included | Build tag `-tags wasm` |
|--------|----------------|----------------------|
| Binary size | +2-3MB (wazero) | +0 for default build |
| User experience | Just works | Must know to add tag |
| Precedent | gRPC uses `-tags grpc` | Consistent with existing pattern |
| Dep count | +1 (wazero, but zero transitive) | +0 for default |
| CI impact | Slightly larger binary everywhere | Only where needed |

**Decision: Always included.**

Rationale:
- wazero has **zero** transitive dependencies (pure Go, no CGO). Adding it is categorically different from adding gRPC (which pulls in protobuf, net, x/net, etc.). The `-tags grpc` precedent exists because of gRPC's dependency weight, not because of binary size.
- 2-3MB binary size increase is acceptable for a CLI tool (current binary is ~15MB with all assertion types and SQLite).
- The "just works" UX matters. A user who finds a plugin in a community repo shouldn't need to rebuild SmokeSig with special tags.
- The `plugins:` YAML section is a no-op when empty. Zero overhead when not used (wazero runtime is not initialized).

**Lazy initialization**: The `PluginManager` is only created when `Config.Plugins` has entries. If no plugins are configured, wazero is never imported at runtime (Go's init is cheap but the runtime allocation is not). This means zero performance impact for users who don't use plugins.

### Decision 9: Plugin Testing Story (BR-09)

**Question**: How do plugin authors develop, test, and debug their plugins?

**Three-layer testing approach**:

**Layer 1: Unit tests in the plugin's native language**

Plugin authors write unit tests in Rust/Go/AS that test their assertion logic with mock inputs. The SmokeSig guest SDK includes test helpers:

```rust
#[cfg(test)]
mod tests {
    use smokesig_sdk::testing::{mock_input, assert_passes, assert_fails};

    #[test]
    fn test_healthy_service() {
        let input = mock_input(json!({"service": "api", "passing_only": true}));
        let result = evaluate(input);
        assert_passes(&result, "3 healthy instances");
    }
}
```

**Layer 2: `smokesig validate --check-plugins`**

Validates that:
- `.wasm` files exist and are valid Wasm modules
- Required exports are present (`_start` and `smokesig_abi_version` for ABI v1; `smokesig_alloc`, `smokesig_dealloc`, `evaluate`, `smokesig_abi_version` for ABI v2)
- ABI version is supported (1 for v1, 2 reserved for v2)
- If `smokesig_manifest` is exported, input_schema validates against test configs
- Capabilities declared in manifest match or are a subset of those granted in YAML

This runs without executing any plugin code against real services.

**Layer 3: `smokesig run` with `--dry-run` for plugins**

In `--dry-run` mode, plugins are loaded and their ABI is validated, but `evaluate()` is not called. The test output shows `[dry-run] plugin:consul_health — would evaluate with config: {...}`.

For full integration testing, plugin authors run `smokesig run` against a local environment (Docker Compose, etc.) -- same as testing built-in assertions.

**Debug support**: Set `SMOKESIG_PLUGIN_DEBUG=1` to get:
- Plugin load/compile timing
- Input JSON logged to stderr before `evaluate()`
- Output JSON logged to stderr after `evaluate()`
- Memory allocation/deallocation tracking

### Decision 10: Reference Plugin Examples (BR-10)

**Rust (primary, recommended)**:

```rust
// plugins/consul-health/src/lib.rs
use smokesig_sdk::{register, PluginInput, PluginOutput, PluginDetail};
use serde::Deserialize;

#[derive(Deserialize)]
struct Config {
    service: String,
    #[serde(default = "default_host")]
    host: String,
    #[serde(default)]
    passing_only: bool,
}

fn default_host() -> String { "localhost:8500".into() }

#[register]
fn evaluate(input: PluginInput) -> PluginOutput {
    let config: Config = input.parse_config().unwrap_or_else(|e| {
        return PluginOutput::error(format!("invalid config: {e}"));
    });

    let url = format!("http://{}/v1/health/service/{}", config.host, config.service);
    let response = smokesig_sdk::http_get(&url, &[]).unwrap();

    if response.status != 200 {
        return PluginOutput::fail(
            format!("consul returned {}", response.status),
            vec![PluginDetail::new("consul_health", "200", &response.status.to_string(), false)],
        );
    }

    // Parse response, count healthy instances...
    let healthy = 3; // parsed from response body
    PluginOutput::pass(
        format!("service {} has {} healthy instances", config.service, healthy),
        vec![PluginDetail::new("consul_health", "passing", &format!("{} passing", healthy), true)],
    )
}
```

Build: `cargo build --target wasm32-wasi --release`

**Go (via TinyGo)**:

```go
//go:build tinygo

package main

import "github.com/cosmolabs-org/smokesig-plugin-sdk-go"

func evaluate(input smokesig.PluginInput) smokesig.PluginOutput {
    service := input.Config.String("service")
    host := input.Config.StringOr("host", "localhost:8500")

    resp, err := smokesig.HTTPGet(
        "http://"+host+"/v1/health/service/"+service, nil,
    )
    if err != nil {
        return smokesig.Error("consul unreachable: " + err.Error())
    }
    // ... parse response ...
    return smokesig.Pass("service healthy")
}

func main() {} // required by TinyGo
```

Build: `tinygo build -target wasi -o consul-health.wasm .`

**AssemblyScript**:

```typescript
// plugins/consul-health/assembly/index.ts
import { PluginInput, PluginOutput, httpGet } from "smokesig-sdk-as";

export function evaluate(input: PluginInput): PluginOutput {
  const service = input.config.getString("service");
  const host = input.config.getStringOr("host", "localhost:8500");

  const resp = httpGet(`http://${host}/v1/health/service/${service}`, []);
  if (resp.status != 200) {
    return PluginOutput.fail(`consul returned ${resp.status}`);
  }
  return PluginOutput.pass(`service ${service} is healthy`);
}
```

Build: `asc assembly/index.ts --target release --outFile consul-health.wasm`

### Decision 11: Performance Model (BR-11)

**Compilation**: wazero compiles Wasm to native machine code. First compilation of a 100KB module takes ~5-20ms. With filesystem cache (`$XDG_CACHE_HOME/smokesig/wasm/`), subsequent loads are ~1ms.

**Instantiation**: ~100-200us per module instance. Acceptable for smoke tests (typically 5-50 tests per suite).

**Execution**: Near-native speed. A plugin that does JSON parsing + HTTP request is dominated by the HTTP latency, not Wasm overhead.

**Memory**: Default 16MB per module. Configurable via `settings.plugin_memory_mb`. wazero enforces this at the runtime level -- a module that exceeds the limit traps with an OOB error.

**Timeout**: Per-plugin timeout (default: test timeout or 10s). Enforced via `context.WithTimeout()`. wazero respects context cancellation.

**Benchmarks to establish at implementation time**:
- Cold compilation: target <50ms for a 500KB module
- Warm load (cached): target <5ms
- Evaluate call overhead (excluding plugin logic): target <1ms
- Memory overhead per loaded module: measure and document

---

## Architecture

### New Package: `internal/plugin/`

```
internal/plugin/
├── manager.go       # PluginManager: load, cache, evaluate, close
├── manager_test.go  # Unit tests with embedded test .wasm modules
├── abi.go           # ABI negotiation, memory protocol, WASI fallback
├── abi_test.go
├── host.go          # Host function implementations (http, env, fs, exec, time)
├── host_test.go
├── sandbox.go       # Capability checking, resource limits
├── sandbox_test.go
├── types.go         # PluginInput, PluginOutput, PluginDetail, PluginConfig
└── testdata/
    ├── pass.wasm    # Minimal plugin that always passes
    ├── fail.wasm    # Minimal plugin that always fails
    ├── crash.wasm   # Plugin that traps (for error handling tests)
    ├── timeout.wasm # Plugin that loops forever (for timeout tests)
    ├── abi2.wasm    # WASI-mode plugin
    └── network.wasm # Plugin that calls host_http_get
```

### Modified Files

| File | Change |
|------|--------|
| `internal/schema/schema.go` | Add `Plugins map[string]PluginEntry` to `SmokeConfig`, add `Plugin map[string]interface{}` to `Expect` |
| `internal/schema/validate.go` | Validate plugin references, file existence, capability coherence |
| `internal/schema/export.go` | Add plugin metadata to `ExportSchema()` output |
| `internal/runner/runner.go` | Add `plugins *plugin.PluginManager` to `Runner`, init in `Run()`, evaluate in assertion block |
| `go.mod` | Add `github.com/tetratelabs/wazero` |
| `cmd/run.go` | No change (PluginManager is internal to Runner) |
| `cmd/validate.go` | Add `--check-plugins` flag for deep plugin validation |
| `cmd/schema.go` | Include plugin info in schema export |

### New Schema Types

```go
// In internal/schema/schema.go

// PluginEntry defines a registered Wasm plugin.
type PluginEntry struct {
    Path         string   `yaml:"path"`
    Version      string   `yaml:"version,omitempty"`
    Timeout      Duration `yaml:"timeout,omitempty"`
    Capabilities []string `yaml:"capabilities,omitempty"`
}

// In SmokeConfig:
type SmokeConfig struct {
    // ... existing fields ...
    Plugins map[string]PluginEntry `yaml:"plugins,omitempty"`
}
```

---

## YAML Config: Complete Example

```yaml
version: 1
project: my-platform

plugins:
  consul_health:
    path: "./plugins/consul-health.wasm"
    version: "1.2.0"
    timeout: 10s
    capabilities: [network]

  vault_seal:
    path: "./plugins/vault-seal.wasm"
    capabilities: [network, env]

  custom_log_check:
    path: "./plugins/log-checker.wasm"
    capabilities: [fs_read]

settings:
  allow_plugin_exec: false  # global kill-switch for exec capability

tests:
  - name: check consul service health
    run: "echo ok"
    expect:
      exit_code: 0
      plugin:
        consul_health:
          service: "api-gateway"
          host: "consul.internal:8500"
          passing_only: true

  - name: verify vault is unsealed
    run: "echo ok"
    expect:
      plugin:
        vault_seal:
          host: "vault.internal:8200"
          expected_sealed: false

  - name: verify app logs
    run: "./start-app.sh"
    expect:
      exit_code: 0
      stdout_contains: "listening on"
      plugin:
        custom_log_check:
          log_path: "./logs/app.log"
          must_contain: ["initialized", "ready"]
          must_not_contain: ["FATAL", "PANIC"]

  - name: mixed built-in and plugin
    run: "curl -s http://localhost:8080/health"
    expect:
      exit_code: 0
      http:
        url: "http://localhost:8080/health"
        status_code: 200
      plugin:
        consul_health:
          service: "web-frontend"
```

Note: plugin assertions compose naturally with built-in assertions. A test can have `exit_code`, `http`, and `plugin` assertions simultaneously.

---

## Scope Boundaries

### In Scope (v1 / FEAT-048)

- wazero integration with compilation caching
- WASI stdin/stdout ABI (version 1) -- no SDK required, universal language support
- `plugins:` YAML section with local `.wasm` file paths
- `plugin:` field on `Expect` for assertion config
- PluginManager with load, cache, evaluate, close lifecycle
- Host functions: `host_http_get`, `host_http_post`, `host_env_get`, `host_time_now`
- Capability-based sandboxing (network, env, time)
- Error taxonomy with distinct assertion types per failure mode
- `smokesig validate` checks for plugin file existence and export probing
- Plugin debug mode (`SMOKESIG_PLUGIN_DEBUG=1`)
- Test .wasm modules in `testdata/` for unit tests
- Documentation: plugin authoring guide, ABI reference

### Deferred (v2+)

- JSON-over-shared-memory ABI (version 2) for performance-sensitive plugins
- Guest-side SDKs published as packages (Rust crate, npm package, Go module) -- required for shared-memory ABI
- Plugin registry / URL download with checksum verification
- `fs_read` and `exec` host functions (higher security surface)
- Instance pooling (optimization, not needed for smoke test workloads)
- `smokesig_manifest` export for embedded plugin metadata + input schema validation
- Plugin marketplace / community index
- Hot-reload in `--watch` mode (re-compile on `.wasm` file change)
- OTel trace propagation through plugin host functions
- Plugin-to-plugin communication (shared context)

### Explicitly Out of Scope

- JavaScript/TypeScript plugins without Wasm compilation (use AssemblyScript instead)
- Dynamically linked native plugins (`.so`/`.dylib`) -- Wasm is the universal format
- Plugin sandboxing via OS-level isolation (containers, seccomp) -- Wasm memory isolation is sufficient
- Backward-incompatible changes to existing assertion types

---

## Estimated Lines of Code

| Component | Estimated LOC | Notes |
|-----------|--------------|-------|
| `internal/plugin/types.go` | ~60 | Input/output structs, config types |
| `internal/plugin/manager.go` | ~250 | Load, compile, cache, evaluate, close |
| `internal/plugin/abi.go` | ~180 | Memory protocol, WASI detection, JSON marshal/unmarshal |
| `internal/plugin/host.go` | ~200 | Host function implementations (http_get, http_post, env_get, time_now) |
| `internal/plugin/sandbox.go` | ~80 | Capability check, memory limit config |
| `internal/plugin/manager_test.go` | ~300 | Tests with embedded .wasm test fixtures |
| `internal/plugin/abi_test.go` | ~120 | ABI negotiation, memory protocol tests |
| `internal/plugin/host_test.go` | ~150 | Host function tests |
| `internal/plugin/sandbox_test.go` | ~80 | Capability denial tests |
| Schema changes (`schema.go`, `validate.go`, `export.go`) | ~80 | PluginEntry, Expect.Plugin, validation |
| Runner integration (`runner.go`) | ~40 | PluginManager init + evaluation block |
| Test .wasm fixtures (build scripts) | ~50 | Makefile/script to compile testdata .wasm |
| **Total** | **~1,390** | ~690 implementation + ~700 tests |

Updated from the original ~800 estimate. The host function implementations, capability sandboxing, and WASI fallback add significant surface area. Test coverage is substantial because plugin boundary code has many failure modes.

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| wazero API instability | Medium | Pin to specific version. wazero reached v1.0 and follows semver. |
| Binary size increase | Low | Measured at ~2.5MB. Acceptable for a CLI tool already at ~15MB. |
| Plugin ABI versioning | Medium | `smokesig_abi_version` export makes version negotiation explicit. v1 memory, v2 WASI. Future versions are additive. |
| Security: malicious plugins | Medium | Wasm memory isolation + capability model + timeout. Users choose which plugins to load. No auto-download in v1. |
| Plugin author adoption | Medium | Start with 2-3 first-party example plugins. Guest SDKs lower the barrier. |
| Performance regression on non-plugin workloads | Low | PluginManager is nil when no plugins configured. Zero overhead path. |
| Go module graph: wazero pulls in unwanted deps | None | wazero has zero Go dependencies. Verified. |

---

## Open Questions (to resolve during implementation)

1. **Monorepo plugin inheritance**: When using `--monorepo`, do sub-configs inherit parent's `plugins:` section? Likely yes, following the same pattern as `settings:` inheritance.

2. **Plugin assertion ordering**: When a test has multiple plugin assertions, are they evaluated sequentially or could they be parallel? Sequential for v1 (simpler, deterministic output ordering).

3. **Config template support**: Should `plugin:` config values support Go templates (`{{ .Env.CONSUL_HOST }}`)? Likely yes -- `processTemplate()` runs before YAML parsing, so it should work automatically.

4. **Watch mode**: When `--watch` re-runs tests, should plugins be re-compiled if the `.wasm` file changed? Yes in v2 (hot-reload). For v1, plugins are compiled once at startup.

5. **`smokesig schema` output**: Should `smokesig schema` list registered plugins and their input schemas? Yes, if `smokesig_manifest` is available. Otherwise just list plugin names and capabilities.
