---
brainstorm: docs/brainstorming/2026-05-29-wasm-plugin-system.md
created: "2026-05-29T12:00:00-03:00"
issue: FEAT-048
status: PENDING
deliverables:
  - id: P-01
    title: "Plugin types and schema integration (PluginEntry, Expect.Plugin, Settings additions)"
  - id: P-02
    title: "PluginManager core: compile, cache, instantiate, evaluate, close"
  - id: P-03
    title: "ABI layer: WASI stdin/stdout primary (v1) and shared-memory performance path (v2)"
  - id: P-04
    title: "Host functions: HTTP, env, time with capability gating"
  - id: P-05
    title: "Sandbox: capability enforcement, memory limits, timeout"
  - id: P-06
    title: "Runner integration: plugin evaluation in assertion block with sorted iteration"
  - id: P-07
    title: "Validation integration: plugin references, file existence, export probing"
  - id: P-08
    title: "Config merge semantics: includes last-wins for plugins, monorepo path resolution"
  - id: P-09
    title: "Schema export: plugin metadata in smokesig schema output"
  - id: P-10
    title: "Test fixtures: .wasm binaries in testdata/ with build script"
  - id: P-11
    title: "Debug mode: SMOKESIG_PLUGIN_DEBUG=1 logging"
  - id: P-12
    title: "Documentation: plugin authoring guide, ABI reference"
---

# FEAT-048: Wasm Plugin System — Implementation Plan

## Goal

Add a WebAssembly plugin system to SmokeSig that lets users write custom assertions in any language that compiles to Wasm (Rust, TinyGo, AssemblyScript, Zig, C), deploy `.wasm` files alongside their config, and reference them by name in `expect.plugin`. Uses `tetratelabs/wazero` (pure Go, zero CGO, zero transitive deps). Always included in the binary (no build tag).

## Review Feedback Incorporated

1. **Map iteration order (Issue 1)**: Plugin names from `Expect.Plugin` are sorted before iteration to ensure deterministic output ordering across runs. Implemented in the runner integration (P-06).

2. **Env security (Issue 2)**: `context.env` in plugin input JSON is always `{}` (empty object). Plugins that need environment variables must use the `host_env_get` host function, which requires the `env` capability grant. No ambient env access.

3. **SDK scope (Issue 3)**: v1 uses WASI stdin/stdout as the primary ABI (version 1) for maximum language compatibility without requiring SDKs. The shared-memory ABI (version 2) is the performance path for plugins that export `smokesig_alloc`/`smokesig_dealloc`/`evaluate`. WASI mode reads JSON from stdin, writes result to stdout. Both work without any SDK.

4. **allow_failure interaction**: Plugin assertions are evaluated like any other assertion. When `allow_failure: true` is set on the test, plugin failures are marked `AllowedFailure` transparently — no special handling needed.

5. **--dry-run semantics**: In dry-run mode, plugin assertions are skipped entirely (same as built-in assertions). The existing dry-run short-circuit in `runTestOnce()` at line 355 already handles this — it returns before the assertion evaluation block.

6. **Config includes merge**: `plugins:` uses last-wins merge semantics in `MergeConfigs()` — overlay plugins replace base plugins by name (same map key). New overlay plugins are added. Base-only plugins are preserved. This matches how `Settings` fields merge.

7. **Monorepo plugin paths**: Plugin `path` values are resolved relative to the sub-config's directory, consistent with SmokeSig's existing `config-dir-relative` convention used by `file_exists`, `credential_check`, etc.

---

## File Structure

| Action | File | Purpose |
|--------|------|---------|
| **create** | `internal/plugin/types.go` | PluginInput, PluginOutput, PluginDetail, PluginConfig, error types |
| **create** | `internal/plugin/manager.go` | PluginManager: load, compile, cache (SHA-256), evaluate, close |
| **create** | `internal/plugin/manager_test.go` | Unit tests with .wasm test fixtures |
| **create** | `internal/plugin/abi.go` | ABI version detection, memory protocol, WASI fallback |
| **create** | `internal/plugin/abi_test.go` | ABI negotiation tests, JSON round-trip |
| **create** | `internal/plugin/host.go` | Host function implementations (http, env, time) |
| **create** | `internal/plugin/host_test.go` | Host function tests with mock modules |
| **create** | `internal/plugin/sandbox.go` | Capability checking, memory limit config |
| **create** | `internal/plugin/sandbox_test.go` | Capability denial tests |
| **create** | `internal/plugin/testdata/build.sh` | Script to compile test .wasm fixtures from .wat/.go sources |
| **create** | `internal/plugin/testdata/pass.wasm` | Minimal plugin that always passes (pre-compiled) |
| **create** | `internal/plugin/testdata/fail.wasm` | Minimal plugin that always fails (pre-compiled) |
| **create** | `internal/plugin/testdata/crash.wasm` | Plugin that traps (unreachable instruction) |
| **create** | `internal/plugin/testdata/timeout.wasm` | Plugin with infinite loop |
| **create** | `internal/plugin/testdata/abi2.wasm` | Memory-ABI plugin (v2 performance path) |
| **create** | `internal/plugin/testdata/echo.wasm` | Plugin that echoes config back (for integration tests) |
| **modify** | `internal/schema/schema.go` | Add `Plugins` to SmokeConfig, `Plugin` to Expect, `AllowPluginExec` to Settings |
| **modify** | `internal/schema/validate.go` | Validate plugin references, file existence, add to hasStandaloneAssertions |
| **modify** | `internal/schema/export.go` | Add plugin section to ExportSchema output |
| **modify** | `internal/schema/remote.go` | Add plugins merge in MergeConfigs (last-wins) |
| **modify** | `internal/runner/runner.go` | Add `plugins` field to Runner, init/close lifecycle, evaluation block |
| **modify** | `go.mod` | Add `github.com/tetratelabs/wazero` |
| **modify** | `cmd/validate.go` | Add `--check-plugins` flag |
| **create** | `docs/guides/plugin-authoring.md` | Plugin authoring guide with examples |

---

## Task 1: Test Fixture Build System (P-10)

**Files**: `internal/plugin/testdata/build.sh`, `internal/plugin/testdata/*.wat`

Test .wasm fixtures are compiled from WebAssembly Text Format (`.wat`) sources using `wat2wasm` (from the [wabt](https://github.com/WebAssembly/wabt) toolkit). WAT is the right choice for test fixtures because it produces minimal binaries (~100-500 bytes), requires no external toolchain (Rust, TinyGo), and gives exact control over exports and behavior.

For CI, pre-compiled `.wasm` files are committed to the repo. The build script is for regeneration only.

### Steps

- [ ] 1. Create `internal/plugin/testdata/` directory
- [ ] 2. Write WAT source for `pass.wat` — memory-ABI plugin that returns `{"pass":true,"message":"ok","details":[{"type":"test","expected":"pass","actual":"pass","pass":true}],"error":null}`:

```wat
;; pass.wat — Minimal memory-ABI plugin that always passes
(module
  ;; Memory exported for host access
  (memory (export "memory") 1)

  ;; ABI version 2 = memory protocol (performance path)
  (global (export "smokesig_abi_version") i32 (i32.const 2))

  ;; Static result JSON stored at offset 1024
  (data (i32.const 1024) "{\"pass\":true,\"message\":\"ok\",\"details\":[{\"type\":\"test\",\"expected\":\"pass\",\"actual\":\"pass\",\"pass\":true}],\"error\":null}")

  ;; smokesig_alloc: trivial bump allocator at offset 4096+
  (global $bump (mut i32) (i32.const 4096))
  (func (export "smokesig_alloc") (param $size i32) (result i32)
    global.get $bump
    global.get $bump
    local.get $size
    i32.add
    global.set $bump
  )

  ;; smokesig_dealloc: no-op (bump allocator)
  (func (export "smokesig_dealloc") (param $ptr i32) (param $len i32))

  ;; evaluate: ignore input, return pointer to static result
  ;; Returns (ptr << 32) | len as i64
  (func (export "evaluate") (param $ptr i32) (param $len i32) (result i64)
    i64.const 1024        ;; result_ptr
    i64.const 32
    i64.shl
    i64.const 121         ;; result_len (length of JSON above)
    i64.or
  )
)
```

- [ ] 3. Write WAT source for `fail.wat` — returns `{"pass":false,...}`
- [ ] 4. Write WAT source for `crash.wat` — executes `unreachable` instruction
- [ ] 5. Write WAT source for `timeout.wat` — infinite loop (`(loop $inf (br $inf))`)
- [ ] 6. Write WAT source for `abi2.wat` — memory-ABI plugin (exports `smokesig_abi_version=2`, `smokesig_alloc`, `smokesig_dealloc`, `evaluate` — the performance-path ABI)
- [ ] 7. Write WAT source for `echo.wat` — copies input config to output message (for integration tests)
- [ ] 8. Write `build.sh` that compiles all `.wat` to `.wasm` via `wat2wasm` (with fallback instructions if wabt not installed)
- [ ] 9. Pre-compile all `.wasm` files and commit them (CI should not depend on wabt being installed)
- [ ] 10. Verify: `file internal/plugin/testdata/*.wasm` confirms valid Wasm modules

**Test commands**:
```bash
# Verify .wasm files are valid
file internal/plugin/testdata/*.wasm
# Each should report: WebAssembly (wasm) binary module version 0x1 (MVP)
```

**Commit**: `feat(plugin): add wasm test fixtures for plugin system`

---

## Task 2: Plugin Types (P-01)

**Files**: `internal/plugin/types.go`

### Steps

- [ ] 1. Create `internal/plugin/types.go` with the following types:

```go
package plugin

import (
	"time"
)

// PluginConfig holds the runtime configuration for a loaded plugin.
type PluginConfig struct {
	Name         string
	Path         string        // absolute path to .wasm file
	Timeout      time.Duration // per-invocation timeout (0 = use default)
	Capabilities []string      // granted capabilities: network, env, time
	ABIVersion   int           // 1 = WASI, 2 = memory (detected at load time)
}

// PluginInput is the JSON payload sent from host to guest.
type PluginInput struct {
	ABIVersion    int                    `json:"abi_version"`
	AssertionName string                 `json:"assertion_name"`
	Config        interface{}            `json:"config"`
	Context       PluginContext          `json:"context"`
}

// PluginContext provides test execution context to the plugin.
type PluginContext struct {
	TestName   string            `json:"test_name"`
	ExitCode   int               `json:"exit_code"`
	Stdout     string            `json:"stdout"`
	Stderr     string            `json:"stderr"`
	DurationMs int               `json:"duration_ms"`
	Env        map[string]string `json:"env"` // always empty — use host_env_get capability
}

// PluginOutput is the JSON payload returned from guest to host.
type PluginOutput struct {
	Pass    bool           `json:"pass"`
	Message string         `json:"message"`
	Details []PluginDetail `json:"details"`
	Error   *string        `json:"error"`
}

// PluginDetail is one sub-assertion result from a plugin.
type PluginDetail struct {
	Type     string `json:"type"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Pass     bool   `json:"pass"`
}

// PluginError classifies plugin failure modes.
type PluginError struct {
	Kind    PluginErrorKind
	Message string
	Cause   error
}

func (e *PluginError) Error() string { return e.Message }
func (e *PluginError) Unwrap() error { return e.Cause }

// PluginErrorKind distinguishes plugin failure modes for assertion type naming.
type PluginErrorKind string

const (
	ErrTimeout        PluginErrorKind = "timeout"
	ErrCrash          PluginErrorKind = "crash"
	ErrABIMismatch    PluginErrorKind = "abi_error"
	ErrCapabilityDeny PluginErrorKind = "denied"
	ErrNotFound       PluginErrorKind = "not_found"
)

const (
	// DefaultTimeout is used when no per-plugin or per-test timeout is set.
	DefaultTimeout = 10 * time.Second

	// DefaultMemoryLimitMB is the per-module memory cap.
	DefaultMemoryLimitMB = 16

	// ABIVersionWASI is the WASI stdin/stdout ABI (primary, no SDK needed).
	ABIVersionWASI = 1

	// ABIVersionMemory is the shared-memory ABI (performance path).
	ABIVersionMemory = 2
)
```

- [ ] 2. Verify: `go build ./internal/plugin/`

**Commit**: `feat(plugin): add plugin type definitions`

---

## Task 3: Schema Integration (P-01)

**Files**: `internal/schema/schema.go`, `internal/schema/validate.go`, `internal/schema/export.go`, `internal/schema/remote.go`

### Steps

- [ ] 1. Add `PluginEntry` type and `Plugins` field to `SmokeConfig` in `schema.go`:

```go
// PluginEntry defines a registered Wasm plugin in the top-level plugins: section.
type PluginEntry struct {
	Path         string   `yaml:"path"`
	Version      string   `yaml:"version,omitempty"`
	Timeout      Duration `yaml:"timeout,omitempty"`
	Capabilities []string `yaml:"capabilities,omitempty"`
}
```

Add to `SmokeConfig` struct (after `Lifecycle`):
```go
	Plugins   map[string]PluginEntry `yaml:"plugins,omitempty"`
```

- [ ] 2. Add `Plugin` field to `Expect` struct (after `DocIntegrity`, before `Extract`):

```go
	// Plugin holds plugin assertion configs, keyed by registered plugin name.
	// Values are arbitrary YAML passed to the plugin as config JSON.
	Plugin map[string]interface{} `yaml:"plugin,omitempty"`
```

- [ ] 3. Add `AllowPluginExec` to `Settings` struct:

```go
	AllowPluginExec bool `yaml:"allow_plugin_exec,omitempty"`
	PluginMemoryMB  int  `yaml:"plugin_memory_mb,omitempty"` // default 16
```

- [ ] 4. Update `hasStandaloneAssertions()` in `validate.go` — add `len(e.Plugin) > 0` to the return expression (plugin assertions can run without a command)

- [ ] 5. Add plugin validation in `Validate()` in `validate.go` — after existing test validation loop, add:
  - Every key in `test.Expect.Plugin` must reference a name in `cfg.Plugins`
  - Error: `"test %q references unregistered plugin %q (available: %s)"`
  - Every `cfg.Plugins[name].Path` must be non-empty
  - Capability values must be from allowed set: `network`, `env`, `time`, `fs_read`, `exec`
  - If `exec` capability is granted but `Settings.AllowPluginExec` is false, error

- [ ] 6. Add plugins merge in `MergeConfigs()` in `remote.go` — after the Includes merge block (line ~238), add:

```go
	// Plugins: last-wins per plugin name
	if len(overlay.Plugins) > 0 {
		if merged.Plugins == nil {
			merged.Plugins = make(map[string]PluginEntry)
		}
		for name, entry := range overlay.Plugins {
			merged.Plugins[name] = entry
		}
	}
```

- [ ] 7. Add plugin metadata to `ExportSchema()` in `export.go` — extend `SchemaOutput` with a `Plugins` field (or add a generic plugin entry to `AssertionTypes`):

```go
// In SchemaOutput, add:
	PluginAssertions []PluginSchemaEntry `json:"plugin_assertions,omitempty"`

// New type:
type PluginSchemaEntry struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}
```

- [ ] 8. Verify: `go test ./internal/schema/ -count=1`
- [ ] 9. Verify: `go build ./...`

**Test commands**:
```bash
go test ./internal/schema/ -count=1 -v
go build ./...
```

**Commit**: `feat(plugin): add schema types and validation for wasm plugins`

---

## Task 4: Sandbox and Capability Enforcement (P-05)

**Files**: `internal/plugin/sandbox.go`, `internal/plugin/sandbox_test.go`

### Steps

- [ ] 1. Create `sandbox.go` with:

```go
package plugin

import (
	"context"
	"fmt"
	"sort"
)

// ValidCapabilities is the set of recognized capability names.
var ValidCapabilities = map[string]bool{
	"network": true,
	"env":     true,
	"time":    true,
	"fs_read": true,
	"exec":    true,
}

// Sandbox enforces capability-based access control for a plugin instance.
type Sandbox struct {
	pluginName   string
	capabilities map[string]bool
	memoryLimitMB int
}

// NewSandbox creates a sandbox from the plugin's granted capabilities.
func NewSandbox(name string, capabilities []string, memoryLimitMB int) *Sandbox {
	caps := make(map[string]bool, len(capabilities))
	for _, c := range capabilities {
		caps[c] = true
	}
	if memoryLimitMB <= 0 {
		memoryLimitMB = DefaultMemoryLimitMB
	}
	return &Sandbox{
		pluginName:    name,
		capabilities:  caps,
		memoryLimitMB: memoryLimitMB,
	}
}

// Check returns nil if the capability is granted, or a PluginError if denied.
func (s *Sandbox) Check(capability string) error {
	if s.capabilities[capability] {
		return nil
	}
	return &PluginError{
		Kind:    ErrCapabilityDeny,
		Message: fmt.Sprintf("plugin %q called %s but capability not granted (granted: %v)", s.pluginName, capability, s.grantedList()),
	}
}

// MemoryLimitPages returns the wazero memory limit in pages (64KB each).
func (s *Sandbox) MemoryLimitPages() uint32 {
	return uint32(s.memoryLimitMB) * 16 // 1MB = 16 pages of 64KB
}

func (s *Sandbox) grantedList() []string {
	var list []string
	for c := range s.capabilities {
		list = append(list, c)
	}
	sort.Strings(list)
	return list
}
```

- [ ] 2. Create `sandbox_test.go` with tests for:
  - Granted capability passes Check
  - Ungranted capability returns PluginError with ErrCapabilityDeny
  - Empty capabilities denies everything
  - MemoryLimitPages calculation (16MB = 256 pages)
  - Default memory limit when 0 is passed

- [ ] 3. Verify: `go test ./internal/plugin/ -run TestSandbox -v`

**Commit**: `feat(plugin): add capability-based sandbox enforcement`

---

## Task 5: ABI Layer (P-03)

**Files**: `internal/plugin/abi.go`, `internal/plugin/abi_test.go`

### Steps

- [ ] 1. Create `abi.go` with ABI detection and both invocation paths:

```go
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// DetectABI probes a compiled module's exports to determine ABI version.
// Returns ABIVersionWASI (1) if _start is exported (WASI entry point — primary ABI).
// Returns ABIVersionMemory (2) if smokesig_alloc + smokesig_dealloc + evaluate are exported.
// Returns error if neither ABI is satisfied.
func DetectABI(mod wazero.CompiledModule) (int, error) {
	exports := mod.ExportedFunctions()

	// WASI ABI (primary): requires _start entry point
	hasStart := hasExport(exports, "_start")
	if hasStart {
		return ABIVersionWASI, nil
	}

	// Memory ABI (performance path): requires smokesig_alloc, smokesig_dealloc, evaluate
	hasAlloc := hasExport(exports, "smokesig_alloc")
	hasDealloc := hasExport(exports, "smokesig_dealloc")
	hasEval := hasExport(exports, "evaluate")

	if hasAlloc && hasDealloc && hasEval {
		return ABIVersionMemory, nil
	}

	return 0, &PluginError{
		Kind:    ErrABIMismatch,
		Message: fmt.Sprintf("plugin does not export required functions (need: _start for WASI ABI, or smokesig_alloc+smokesig_dealloc+evaluate for memory ABI)"),
	}
}

func hasExport(exports map[string]api.FunctionDefinition, name string) bool {
	_, ok := exports[name]
	return ok
}

// InvokeMemoryABI calls the plugin using the shared-memory protocol.
// 1. Marshal input to JSON
// 2. Call smokesig_alloc(len) -> ptr
// 3. Write JSON to [ptr, ptr+len) in guest memory
// 4. Call evaluate(ptr, len) -> (result_ptr << 32) | result_len
// 5. Read result JSON from [result_ptr, result_ptr+result_len)
// 6. Call smokesig_dealloc for both buffers
func InvokeMemoryABI(ctx context.Context, mod api.Module, input *PluginInput) (*PluginOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshaling plugin input: %w", err)
	}

	// Allocate input buffer in guest memory
	alloc := mod.ExportedFunction("smokesig_alloc")
	dealloc := mod.ExportedFunction("smokesig_dealloc")
	evaluate := mod.ExportedFunction("evaluate")

	if alloc == nil || dealloc == nil || evaluate == nil {
		return nil, &PluginError{Kind: ErrABIMismatch, Message: "missing required exports"}
	}

	results, err := alloc.Call(ctx, uint64(len(inputJSON)))
	if err != nil {
		return nil, &PluginError{Kind: ErrCrash, Message: fmt.Sprintf("smokesig_alloc failed: %v", err), Cause: err}
	}
	inputPtr := uint32(results[0])

	// Write input JSON to guest memory
	if !mod.Memory().Write(inputPtr, inputJSON) {
		return nil, &PluginError{Kind: ErrCrash, Message: "failed to write input to guest memory"}
	}

	// Call evaluate
	evalResults, err := evaluate.Call(ctx, uint64(inputPtr), uint64(len(inputJSON)))
	if err != nil {
		// Attempt cleanup
		dealloc.Call(ctx, uint64(inputPtr), uint64(len(inputJSON)))
		return nil, classifyWazeroError(err)
	}

	// Decode result pointer: (result_ptr << 32) | result_len
	packed := evalResults[0]
	resultPtr := uint32(packed >> 32)
	resultLen := uint32(packed & 0xFFFFFFFF)

	// Read result JSON from guest memory
	resultJSON, ok := mod.Memory().Read(resultPtr, resultLen)
	if !ok {
		dealloc.Call(ctx, uint64(inputPtr), uint64(len(inputJSON)))
		return nil, &PluginError{Kind: ErrCrash, Message: fmt.Sprintf("failed to read result from guest memory at [%d, %d)", resultPtr, resultPtr+resultLen)}
	}

	// Free both buffers
	dealloc.Call(ctx, uint64(inputPtr), uint64(len(inputJSON)))
	dealloc.Call(ctx, uint64(resultPtr), uint64(resultLen))

	// Parse result
	var output PluginOutput
	if err := json.Unmarshal(resultJSON, &output); err != nil {
		return nil, &PluginError{Kind: ErrABIMismatch, Message: fmt.Sprintf("invalid plugin output JSON: %v", err), Cause: err}
	}

	return &output, nil
}

// InvokeWASIABI calls the plugin using stdin/stdout pipes.
// 1. Marshal input to JSON
// 2. Provide JSON as stdin to the WASI module
// 3. Run _start
// 4. Capture stdout as result JSON
func InvokeWASIABI(ctx context.Context, runtime wazero.Runtime, compiled wazero.CompiledModule, input *PluginInput) (*PluginOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshaling plugin input: %w", err)
	}

	stdin := bytes.NewReader(inputJSON)
	var stdout bytes.Buffer

	config := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithName("") // anonymous to avoid name collisions across invocations

	mod, err := runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, classifyWazeroError(err)
	}
	defer mod.Close(ctx)

	// For WASI modules, _start is called automatically during instantiation.
	// The module reads stdin, processes, writes stdout, and exits.

	resultJSON := stdout.Bytes()
	if len(resultJSON) == 0 {
		return nil, &PluginError{Kind: ErrABIMismatch, Message: "WASI plugin produced no output on stdout"}
	}

	var output PluginOutput
	if err := json.Unmarshal(resultJSON, &output); err != nil {
		return nil, &PluginError{Kind: ErrABIMismatch, Message: fmt.Sprintf("invalid plugin output JSON: %v", err), Cause: err}
	}

	return &output, nil
}

// classifyWazeroError converts a wazero error into a typed PluginError.
func classifyWazeroError(err error) *PluginError {
	msg := err.Error()
	// wazero trap errors contain "wasm error:" or "unreachable"
	if contains(msg, "unreachable") || contains(msg, "wasm error") || contains(msg, "out of bounds") {
		return &PluginError{Kind: ErrCrash, Message: fmt.Sprintf("plugin crashed: %v", err), Cause: err}
	}
	if contains(msg, "context deadline exceeded") || contains(msg, "context canceled") {
		return &PluginError{Kind: ErrTimeout, Message: fmt.Sprintf("plugin exceeded timeout: %v", err), Cause: err}
	}
	return &PluginError{Kind: ErrCrash, Message: fmt.Sprintf("plugin error: %v", err), Cause: err}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
```

- [ ] 2. Create `abi_test.go` with tests for:
  - DetectABI returns ABIVersionWASI for modules with _start (primary)
  - DetectABI returns ABIVersionMemory for modules with alloc+dealloc+evaluate
  - DetectABI returns error for modules with neither
  - InvokeMemoryABI round-trip with `pass.wasm` test fixture
  - InvokeMemoryABI returns ErrCrash for `crash.wasm`
  - classifyWazeroError correctly categorizes timeout, crash, OOB errors
  - JSON marshaling round-trip for PluginInput/PluginOutput

- [ ] 3. Verify: `go test ./internal/plugin/ -run TestABI -v`

**Commit**: `feat(plugin): add ABI detection and memory/WASI invocation protocols`

---

## Task 6: Host Functions (P-04)

**Files**: `internal/plugin/host.go`, `internal/plugin/host_test.go`

v1 implements three host function categories: `network` (HTTP GET/POST), `env` (environment variable read), and `time` (current timestamp). `fs_read` and `exec` are deferred to v2 (higher security surface).

### Steps

- [ ] 1. Create `host.go`:

```go
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const hostModuleName = "smokesig"

// HostResponse is the JSON format for host function results written back to guest memory.
type HostResponse struct {
	Status int               `json:"status,omitempty"`
	Body   string            `json:"body,omitempty"`
	Error  string            `json:"error,omitempty"`
	Value  string            `json:"value,omitempty"`
	TimeMs int64             `json:"time_ms,omitempty"`
}

// RegisterHostFunctions registers the "smokesig" host module with wazero.
// Called once in NewPluginManager. Each host function extracts the active
// sandbox from context via SandboxFromContext(ctx) for capability checks.
func RegisterHostFunctions(ctx context.Context, runtime wazero.Runtime) (wazero.CompiledModule, error) {
	builder := runtime.NewHostModuleBuilder(hostModuleName)

	builder.NewFunctionBuilder().
		WithFunc(makeHTTPGet()).
		WithParameterNames("url_ptr", "url_len", "headers_ptr", "headers_len").
		Export("host_http_get")

	builder.NewFunctionBuilder().
		WithFunc(makeHTTPPost()).
		WithParameterNames("url_ptr", "url_len", "body_ptr", "body_len", "headers_ptr", "headers_len").
		Export("host_http_post")

	builder.NewFunctionBuilder().
		WithFunc(makeEnvGet()).
		WithParameterNames("name_ptr", "name_len").
		Export("host_env_get")

	builder.NewFunctionBuilder().
		WithFunc(makeTimeNow()).
		Export("host_time_now")

	compiled, err := builder.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compiling host module: %w", err)
	}
	return compiled, nil
}

// makeHTTPGet returns a host function that performs an HTTP GET.
// Requires "network" capability. Reads URL from guest memory, writes response JSON back.
func makeHTTPGet() func(ctx context.Context, mod api.Module, urlPtr, urlLen, headersPtr, headersLen uint32) uint64 {
	return func(ctx context.Context, mod api.Module, urlPtr, urlLen, headersPtr, headersLen uint32) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return writeHostResponse(mod, HostResponse{Error: "no sandbox in context"})
		}
		if err := sandbox.Check("network"); err != nil {
			return writeHostResponse(mod, HostResponse{Error: err.Error()})
		}

		urlBytes, ok := mod.Memory().Read(urlPtr, urlLen)
		if !ok {
			return writeHostResponse(mod, HostResponse{Error: "failed to read URL from memory"})
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(urlBytes), nil)
		if err != nil {
			return writeHostResponse(mod, HostResponse{Error: fmt.Sprintf("invalid URL: %v", err)})
		}

		// Parse headers if provided
		if headersLen > 0 {
			hdrBytes, ok := mod.Memory().Read(headersPtr, headersLen)
			if ok {
				var headers map[string]string
				if json.Unmarshal(hdrBytes, &headers) == nil {
					for k, v := range headers {
						req.Header.Set(k, v)
					}
				}
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return writeHostResponse(mod, HostResponse{Error: fmt.Sprintf("HTTP GET failed: %v", err)})
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return writeHostResponse(mod, HostResponse{Status: resp.StatusCode, Body: string(body)})
	}
}

// makeHTTPPost returns a host function that performs an HTTP POST.
// Requires "network" capability.
func makeHTTPPost() func(ctx context.Context, mod api.Module, urlPtr, urlLen, bodyPtr, bodyLen, headersPtr, headersLen uint32) uint64 {
	return func(ctx context.Context, mod api.Module, urlPtr, urlLen, bodyPtr, bodyLen, headersPtr, headersLen uint32) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return writeHostResponse(mod, HostResponse{Error: "no sandbox in context"})
		}
		if err := sandbox.Check("network"); err != nil {
			return writeHostResponse(mod, HostResponse{Error: err.Error()})
		}

		urlBytes, ok := mod.Memory().Read(urlPtr, urlLen)
		if !ok {
			return writeHostResponse(mod, HostResponse{Error: "failed to read URL from memory"})
		}

		var bodyReader io.Reader
		if bodyLen > 0 {
			bodyBytes, ok := mod.Memory().Read(bodyPtr, bodyLen)
			if !ok {
				return writeHostResponse(mod, HostResponse{Error: "failed to read body from memory"})
			}
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, string(urlBytes), bodyReader)
		if err != nil {
			return writeHostResponse(mod, HostResponse{Error: fmt.Sprintf("invalid URL: %v", err)})
		}

		if headersLen > 0 {
			hdrBytes, ok := mod.Memory().Read(headersPtr, headersLen)
			if ok {
				var headers map[string]string
				if json.Unmarshal(hdrBytes, &headers) == nil {
					for k, v := range headers {
						req.Header.Set(k, v)
					}
				}
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return writeHostResponse(mod, HostResponse{Error: fmt.Sprintf("HTTP POST failed: %v", err)})
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return writeHostResponse(mod, HostResponse{Status: resp.StatusCode, Body: string(body)})
	}
}

// makeEnvGet returns a host function that reads an environment variable.
// Requires "env" capability.
func makeEnvGet() func(ctx context.Context, mod api.Module, namePtr, nameLen uint32) uint64 {
	return func(ctx context.Context, mod api.Module, namePtr, nameLen uint32) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return writeHostResponse(mod, HostResponse{Error: "no sandbox in context"})
		}
		if err := sandbox.Check("env"); err != nil {
			return writeHostResponse(mod, HostResponse{Error: err.Error()})
		}

		nameBytes, ok := mod.Memory().Read(namePtr, nameLen)
		if !ok {
			return writeHostResponse(mod, HostResponse{Error: "failed to read env var name from memory"})
		}

		value := os.Getenv(string(nameBytes))
		return writeHostResponse(mod, HostResponse{Value: value})
	}
}

// makeTimeNow returns a host function that returns current time in unix millis.
// Requires "time" capability.
func makeTimeNow() func(ctx context.Context, mod api.Module) uint64 {
	return func(ctx context.Context, mod api.Module) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return 0
		}
		if err := sandbox.Check("time"); err != nil {
			return 0
		}
		return uint64(time.Now().UnixMilli())
	}
}

// writeHostResponse marshals a HostResponse to JSON and writes it to guest memory
// via smokesig_alloc, returning (ptr << 32) | len.
func writeHostResponse(mod api.Module, resp HostResponse) uint64 {
	data, err := json.Marshal(resp)
	if err != nil {
		return 0
	}

	alloc := mod.ExportedFunction("smokesig_alloc")
	if alloc == nil {
		return 0
	}

	results, err := alloc.Call(context.Background(), uint64(len(data)))
	if err != nil {
		return 0
	}
	ptr := uint32(results[0])

	if !mod.Memory().Write(ptr, data) {
		return 0
	}

	return (uint64(ptr) << 32) | uint64(len(data))
}
```

Note: The `import "bytes"` is needed in this file for `bytes.NewReader` in `makeHTTPPost`. Add it to the import block.

- [ ] 2. Create `host_test.go` with tests for:
  - Capability denial: calling host_http_get without "network" returns error response
  - Capability grant: calling host_env_get with "env" capability succeeds
  - HTTP GET against httptest.Server returns status + body
  - HTTP POST against httptest.Server sends body correctly
  - Env get reads OS environment correctly
  - Time now returns reasonable value (> 0, < future)
  - Response body capped at 1MB
  - Invalid URL returns error in response JSON

- [ ] 3. Verify: `go test ./internal/plugin/ -run TestHost -v`

**Commit**: `feat(plugin): add host functions for network, env, and time capabilities`

---

## Task 7: PluginManager Core (P-02)

**Files**: `internal/plugin/manager.go`, `internal/plugin/manager_test.go`

### Steps

- [ ] 1. Create `manager.go`:

```go
package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// PluginManager loads, caches, and evaluates Wasm plugins.
type PluginManager struct {
	runtime   wazero.Runtime
	compiled  map[string]wazero.CompiledModule  // keyed by SHA-256 of .wasm bytes
	configs   map[string]*PluginConfig           // keyed by plugin name
	sandboxes map[string]*Sandbox                // keyed by plugin name
	mu        sync.RWMutex
	debug     bool
	configDir string // base directory for resolving relative plugin paths
}

// ManagerOptions configures PluginManager creation.
type ManagerOptions struct {
	ConfigDir     string
	CacheDir      string // filesystem compilation cache (empty = no cache)
	MemoryLimitMB int    // per-module memory limit (0 = default 16MB)
	Debug         bool   // log plugin I/O to stderr
}

// NewPluginManager creates a PluginManager with a wazero runtime.
func NewPluginManager(ctx context.Context, opts ManagerOptions) (*PluginManager, error) {
	runtimeCfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true)

	if opts.CacheDir != "" {
		if err := os.MkdirAll(opts.CacheDir, 0755); err == nil {
			cache, err := wazero.NewCompilationCacheWithDir(opts.CacheDir)
			if err == nil {
				runtimeCfg = runtimeCfg.WithCompilationCache(cache)
			}
		}
	}

	rt := wazero.NewRuntimeWithConfig(ctx, runtimeCfg)

	// Instantiate WASI for ABI v1 modules
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("instantiating WASI: %w", err)
	}

	// Register host functions once (sandbox resolved per-invocation via context)
	if _, err := RegisterHostFunctions(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("registering host functions: %w", err)
	}

	return &PluginManager{
		runtime:   rt,
		compiled:  make(map[string]wazero.CompiledModule),
		configs:   make(map[string]*PluginConfig),
		sandboxes: make(map[string]*Sandbox),
		debug:     opts.Debug || os.Getenv("SMOKESIG_PLUGIN_DEBUG") == "1",
		configDir: opts.ConfigDir,
	}, nil
}

// LoadPlugin compiles and registers a plugin by name.
func (m *PluginManager) LoadPlugin(ctx context.Context, name string, entry PluginEntry, memoryLimitMB int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Resolve path relative to config directory
	wasmPath := entry.Path
	if !filepath.IsAbs(wasmPath) {
		wasmPath = filepath.Join(m.configDir, wasmPath)
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q: %v", name, err), Cause: err}
	}

	// SHA-256 for compilation cache key
	hash := sha256.Sum256(wasmBytes)
	hashStr := hex.EncodeToString(hash[:])

	// Check compilation cache
	compiled, ok := m.compiled[hashStr]
	if !ok {
		compiled, err = m.runtime.CompileModule(ctx, wasmBytes)
		if err != nil {
			return &PluginError{Kind: ErrABIMismatch, Message: fmt.Sprintf("plugin %q: compilation failed: %v", name, err), Cause: err}
		}
		m.compiled[hashStr] = compiled
	}

	// Detect ABI version
	abiVersion, err := DetectABI(compiled)
	if err != nil {
		return err
	}

	timeout := entry.Timeout.Duration
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	config := &PluginConfig{
		Name:         name,
		Path:         wasmPath,
		Timeout:      timeout,
		Capabilities: entry.Capabilities,
		ABIVersion:   abiVersion,
	}
	m.configs[name] = config

	sandbox := NewSandbox(name, entry.Capabilities, memoryLimitMB)
	m.sandboxes[name] = sandbox

	if m.debug {
		fmt.Fprintf(os.Stderr, "[plugin-debug] loaded %q (ABI v%d, hash %s, caps %v)\n", name, abiVersion, hashStr[:12], entry.Capabilities)
	}

	return nil
}

// Evaluate runs a plugin assertion and returns the result.
func (m *PluginManager) Evaluate(ctx context.Context, name string, input PluginInput) (*PluginOutput, error) {
	m.mu.RLock()
	config, ok := m.configs[name]
	sandbox := m.sandboxes[name]
	m.mu.RUnlock()

	if !ok {
		return nil, &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q not loaded", name)}
	}

	// Apply timeout
	timeout := config.Timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Set ABI version from detected config
	input.ABIVersion = config.ABIVersion

	// Inject sandbox into context for host function dispatch
	ctx = ContextWithSandbox(ctx, sandbox)

	if m.debug {
		inputJSON, _ := json.Marshal(input)
		fmt.Fprintf(os.Stderr, "[plugin-debug] evaluate %q input: %s\n", name, string(inputJSON))
	}

	var output *PluginOutput
	var err error

	// Find compiled module by looking up config path hash
	wasmBytes, readErr := os.ReadFile(config.Path)
	if readErr != nil {
		return nil, &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q .wasm file disappeared: %v", name, readErr)}
	}
	hash := sha256.Sum256(wasmBytes)
	hashStr := hex.EncodeToString(hash[:])

	m.mu.RLock()
	compiled, ok := m.compiled[hashStr]
	m.mu.RUnlock()
	if !ok {
		return nil, &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q not compiled", name)}
	}

	switch config.ABIVersion {
	case ABIVersionMemory:
		// Instantiate a fresh module for this invocation
		modConfig := wazero.NewModuleConfig().WithName("")
		mod, instErr := m.runtime.InstantiateModule(ctx, compiled, modConfig)
		if instErr != nil {
			return nil, classifyWazeroError(instErr)
		}
		defer mod.Close(ctx)
		output, err = InvokeMemoryABI(ctx, mod, &input)

	case ABIVersionWASI:
		output, err = InvokeWASIABI(ctx, m.runtime, compiled, &input)

	default:
		return nil, &PluginError{Kind: ErrABIMismatch, Message: fmt.Sprintf("unsupported ABI version %d", config.ABIVersion)}
	}

	if m.debug && output != nil {
		outputJSON, _ := json.Marshal(output)
		fmt.Fprintf(os.Stderr, "[plugin-debug] evaluate %q output: %s\n", name, string(outputJSON))
	}

	return output, err
}

// Close releases all compiled modules and the wazero runtime.
func (m *PluginManager) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}

// PluginNames returns the sorted list of loaded plugin names.
func (m *PluginManager) PluginNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.configs))
	for name := range m.configs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

Note: add `"context"`, `"encoding/json"`, and `"sort"` to the import block.

- [ ] 2. Refactor host module registration to context-based dispatch. Register host functions ONCE in `NewPluginManager` (not per-plugin), since wazero only allows one module per name. Each host function closure extracts the active sandbox from context:

```go
// Add to sandbox.go:
type sandboxKey struct{}

// SandboxFromContext retrieves the active sandbox from context.
func SandboxFromContext(ctx context.Context) *Sandbox {
	s, _ := ctx.Value(sandboxKey{}).(*Sandbox)
	return s
}

// ContextWithSandbox stores a sandbox in context for host function dispatch.
func ContextWithSandbox(ctx context.Context, s *Sandbox) context.Context {
	return context.WithValue(ctx, sandboxKey{}, s)
}
```

In `RegisterHostFunctions`, change all `sandbox` references to extract from context:
```go
func RegisterHostFunctions(ctx context.Context, runtime wazero.Runtime) (wazero.CompiledModule, error) {
	// ... each host function closure does:
	sandbox := SandboxFromContext(ctx)
	if sandbox == nil {
		return writeHostResponse(mod, HostResponse{Error: "no sandbox in context"})
	}
	if err := sandbox.Check("network"); err != nil { ... }
```

In `Evaluate`, wrap the context before invocation:
```go
	ctx = ContextWithSandbox(ctx, sandbox)
```

In `NewPluginManager`, call `RegisterHostFunctions` once (no sandbox param):
```go
	if _, err := RegisterHostFunctions(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("registering host functions: %w", err)
	}
```

Remove the `RegisterHostFunctions` call from `LoadPlugin` entirely.

- [ ] 3. Create `manager_test.go` with tests for:
  - `NewPluginManager` creates runtime successfully
  - `LoadPlugin` with `pass.wasm` succeeds, detects ABI v2 (memory)
  - `LoadPlugin` with nonexistent path returns ErrNotFound
  - `Evaluate` with `pass.wasm` returns passing result
  - `Evaluate` with `fail.wasm` returns failing result
  - `Evaluate` with `crash.wasm` returns ErrCrash
  - `Evaluate` with `timeout.wasm` returns ErrTimeout (use short timeout)
  - `Evaluate` with unknown plugin name returns ErrNotFound
  - `Close` succeeds without error
  - `PluginNames` returns sorted list
  - Debug mode logs to stderr (capture stderr)
  - SHA-256 caching: loading same .wasm bytes twice reuses compiled module

- [ ] 4. Verify: `go test ./internal/plugin/ -v -count=1`

**Test commands**:
```bash
go test ./internal/plugin/ -v -count=1
go test ./internal/plugin/ -run TestPluginManager -v
```

**Commit**: `feat(plugin): add PluginManager with compile cache and evaluate lifecycle`

---

## Task 8: Runner Integration (P-06)

**Files**: `internal/runner/runner.go`

### Steps

- [ ] 1. Add import for `"github.com/CosmoLabs-org/SmokeSig/internal/plugin"` and `"sort"`

- [ ] 2. Add `plugins *plugin.PluginManager` field to the `Runner` struct (after `lifecycleMu`)

- [ ] 3. In `Run()` method, after the recursion check and before prerequisite execution, add plugin initialization:

```go
	// Initialize plugin manager if plugins are configured
	if len(r.Config.Plugins) > 0 {
		cacheDir := ""
		if home, err := os.UserCacheDir(); err == nil {
			cacheDir = filepath.Join(home, "smokesig", "wasm")
		}
		memoryMB := r.Config.Settings.PluginMemoryMB
		if memoryMB <= 0 {
			memoryMB = plugin.DefaultMemoryLimitMB
		}
		pm, err := plugin.NewPluginManager(context.Background(), plugin.ManagerOptions{
			ConfigDir:     r.ConfigDir,
			CacheDir:      cacheDir,
			MemoryLimitMB: memoryMB,
			Debug:         os.Getenv("SMOKESIG_PLUGIN_DEBUG") == "1",
		})
		if err != nil {
			return nil, fmt.Errorf("initializing plugin manager: %w", err)
		}
		defer pm.Close(context.Background())
		r.plugins = pm

		// Load all plugins — sorted for deterministic error ordering
		pluginNames := make([]string, 0, len(r.Config.Plugins))
		for name := range r.Config.Plugins {
			pluginNames = append(pluginNames, name)
		}
		sort.Strings(pluginNames)
		for _, name := range pluginNames {
			entry := r.Config.Plugins[name]
			if err := pm.LoadPlugin(context.Background(), name, entry, memoryMB); err != nil {
				return nil, fmt.Errorf("loading plugin %q: %w", name, err)
			}
		}
	}
```

- [ ] 4. In `runTestOnce()`, after the `DocIntegrity` assertion block (line ~803) and before `duration := time.Since(start)` (line ~805), add plugin evaluation with **sorted iteration**:

```go
	// Plugin assertions — sorted by name for deterministic output
	if len(t.Expect.Plugin) > 0 && r.plugins != nil {
		pluginNames := make([]string, 0, len(t.Expect.Plugin))
		for name := range t.Expect.Plugin {
			pluginNames = append(pluginNames, name)
		}
		sort.Strings(pluginNames)

		for _, name := range pluginNames {
			config := t.Expect.Plugin[name]
			pluginInput := plugin.PluginInput{
				AssertionName: name,
				Config:        config,
				Context: plugin.PluginContext{
					TestName:   t.Name,
					ExitCode:   exitCode,
					Stdout:     stdout.String(),
					Stderr:     stderr.String(),
					DurationMs: int(time.Since(start).Milliseconds()),
					Env:        map[string]string{}, // always empty — use host_env_get capability
				},
			}
			result, err := r.plugins.Evaluate(ctx, name, pluginInput)
			if err != nil {
				// Plugin crash/timeout/ABI error/capability denial
				errKind := "error"
				if pe, ok := err.(*plugin.PluginError); ok {
					errKind = string(pe.Kind)
				}
				assertions = append(assertions, AssertionResult{
					Type:     fmt.Sprintf("plugin:%s:%s", name, errKind),
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
				// If plugin returned pass=false but no details, create a single assertion
				if !result.Pass && len(result.Details) == 0 {
					assertions = append(assertions, AssertionResult{
						Type:     fmt.Sprintf("plugin:%s", name),
						Expected: "pass",
						Actual:   result.Message,
						Passed:   false,
					})
					allPassed = false
				}
			}
		}
	}
```

- [ ] 5. Add `"path/filepath"` to runner.go imports if not already present

- [ ] 6. Verify: `go build ./...`
- [ ] 7. Verify: `go test ./internal/runner/ -count=1`

**Test commands**:
```bash
go build ./...
go test ./internal/runner/ -count=1 -v
```

**Commit**: `feat(plugin): integrate plugin evaluation into runner assertion block`

---

## Task 9: Validation Enhancement (P-07)

**Files**: `internal/schema/validate.go`, `cmd/validate.go`

### Steps

- [ ] 1. Add plugin reference validation in `Validate()` function — after the existing test validation loop, add a new loop:

```go
	// Validate plugin assertions reference registered plugins
	for i, t := range cfg.Tests {
		for pluginName := range t.Expect.Plugin {
			if _, ok := cfg.Plugins[pluginName]; !ok {
				available := make([]string, 0, len(cfg.Plugins))
				for name := range cfg.Plugins {
					available = append(available, name)
				}
				sort.Strings(available)
				errs = append(errs, fmt.Sprintf(
					"tests[%d] %q: references unregistered plugin %q (available: %s)",
					i, t.Name, pluginName, strings.Join(available, ", "),
				))
			}
		}
	}

	// Validate plugin entries
	for name, entry := range cfg.Plugins {
		if entry.Path == "" {
			errs = append(errs, fmt.Sprintf("plugins.%s: path is required", name))
		}
		for _, cap := range entry.Capabilities {
			if !isValidCapability(cap) {
				errs = append(errs, fmt.Sprintf("plugins.%s: unknown capability %q (valid: network, env, time, fs_read, exec)", name, cap))
			}
		}
		if hasCapability(entry.Capabilities, "exec") && !cfg.Settings.AllowPluginExec {
			errs = append(errs, fmt.Sprintf("plugins.%s: exec capability requires settings.allow_plugin_exec: true", name))
		}
	}
```

Add helper functions:

```go
func isValidCapability(cap string) bool {
	switch cap {
	case "network", "env", "time", "fs_read", "exec":
		return true
	}
	return false
}

func hasCapability(caps []string, target string) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}
```

- [ ] 2. Add `--check-plugins` flag to `cmd/validate.go` that does deep validation:
  - Verify `.wasm` files exist and are readable
  - Compile each plugin and probe exports (requires wazero, so only when flag is set)
  - Report ABI version detected, missing exports, invalid modules

- [ ] 3. Add `"sort"` and `"strings"` to validate.go imports if not present

- [ ] 4. Verify: `go test ./internal/schema/ -run TestValidate -v`
- [ ] 5. Verify: `go build ./cmd/...`

**Test commands**:
```bash
go test ./internal/schema/ -run TestValidate -v -count=1
go build ./...
```

**Commit**: `feat(plugin): add plugin validation and --check-plugins flag`

---

## Task 10: Config Merge and Monorepo Support (P-08)

**Files**: `internal/schema/remote.go`, `internal/schema/schema.go`

### Steps

- [ ] 1. In `MergeConfigs()` in `remote.go`, add plugins merge after line ~248 (after tests append, before `return merged`):

```go
	// Plugins: last-wins per plugin name (overlay replaces base by key)
	if len(overlay.Plugins) > 0 {
		if merged.Plugins == nil {
			merged.Plugins = make(map[string]PluginEntry)
		}
		for name, entry := range overlay.Plugins {
			merged.Plugins[name] = entry
		}
	}
```

- [ ] 2. In `MergeEnv()` in `schema.go`, add plugins merge (same pattern as above)

- [ ] 3. In `MergeConfigs()`, also merge new Settings fields:

```go
	if overlay.Settings.AllowPluginExec {
		merged.Settings.AllowPluginExec = true
	}
	if overlay.Settings.PluginMemoryMB > 0 {
		merged.Settings.PluginMemoryMB = overlay.Settings.PluginMemoryMB
	}
```

- [ ] 4. Monorepo path resolution: No code changes needed. The existing `loadWithDepthAndResolver()` in `remote.go` resolves each sub-config from its own directory (`configDir := filepath.Dir(path)` at line 277). Plugin `Path` values are relative to the sub-config's directory. The Runner already receives the correct `ConfigDir` per sub-config. The PluginManager resolves relative paths via `filepath.Join(m.configDir, wasmPath)`.

- [ ] 5. Add test for config merge with plugins (in existing `remote_test.go` or new test):
  - Base has plugin A, overlay has plugin B: merged has both
  - Base has plugin A v1, overlay has plugin A v2: merged has A v2 (last-wins)
  - Overlay adds exec capability but AllowPluginExec=false: validation catches it

- [ ] 6. Verify: `go test ./internal/schema/ -count=1 -v`

**Test commands**:
```bash
go test ./internal/schema/ -count=1 -v
```

**Commit**: `feat(plugin): add plugins merge semantics for includes and monorepo`

---

## Task 11: Schema Export (P-09)

**Files**: `internal/schema/export.go`

### Steps

- [ ] 1. Add `PluginSchemaEntry` type:

```go
// PluginSchemaEntry describes a registered plugin for schema export.
type PluginSchemaEntry struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}
```

- [ ] 2. Add `PluginAssertions []PluginSchemaEntry` to `SchemaOutput` struct

- [ ] 3. Update `ExportSchema()` — it currently takes no arguments. Add a variant `ExportSchemaWithPlugins(plugins map[string]PluginEntry)` that populates the `PluginAssertions` slice with sorted plugin names:

```go
func ExportSchemaWithPlugins(plugins map[string]PluginEntry) *SchemaOutput {
	out := ExportSchema()
	if len(plugins) > 0 {
		names := make([]string, 0, len(plugins))
		for name := range plugins {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			entry := plugins[name]
			out.PluginAssertions = append(out.PluginAssertions, PluginSchemaEntry{
				Name:         name,
				Version:      entry.Version,
				Capabilities: entry.Capabilities,
			})
		}
	}
	return out
}
```

- [ ] 4. Update `cmd/schema.go` to call `ExportSchemaWithPlugins` when a config file is available (load config, pass plugins), falling back to `ExportSchema()` when no config is loaded

- [ ] 5. Verify: `go build ./...`

**Test commands**:
```bash
go build ./...
go test ./internal/schema/ -run TestExport -v -count=1
```

**Commit**: `feat(plugin): add plugin metadata to schema export`

---

## Task 12: Debug Mode (P-11)

**Files**: `internal/plugin/manager.go` (already included in Task 7)

Debug logging is built into the PluginManager (Task 7). This task verifies it works end-to-end.

### Steps

- [ ] 1. Verify debug mode activates when `SMOKESIG_PLUGIN_DEBUG=1` is set
- [ ] 2. Verify output includes:
  - `[plugin-debug] loaded "name" (ABI vN, hash XXXX, caps [...])`
  - `[plugin-debug] evaluate "name" input: {...}`
  - `[plugin-debug] evaluate "name" output: {...}`
- [ ] 3. Add memory allocation tracking in debug mode — after `smokesig_alloc` calls, log `[plugin-debug] alloc %d bytes at ptr %d`
- [ ] 4. Verify debug output does NOT appear when env var is unset
- [ ] 5. Write test that captures stderr and asserts debug lines are present/absent

**Test commands**:
```bash
SMOKESIG_PLUGIN_DEBUG=1 go test ./internal/plugin/ -run TestDebug -v
```

**Commit**: `feat(plugin): verify debug mode logging for plugin system`

---

## Task 13: Documentation (P-12)

**Files**: `docs/guides/plugin-authoring.md`

### Steps

- [ ] 1. Create `docs/guides/plugin-authoring.md` with sections:
  - Overview: what plugins are, when to use them
  - Quick start: minimal WASI plugin (stdin/stdout, no SDK)
  - YAML configuration: `plugins:` section and `expect.plugin:` usage
  - ABI reference: WASI protocol (v1, primary) and memory protocol (v2, performance path)
  - Input/output JSON format with field descriptions
  - Capabilities: what each grants, how to request
  - Error handling: crash, timeout, ABI mismatch behaviors
  - Debug mode: `SMOKESIG_PLUGIN_DEBUG=1`
  - Examples: Rust (wasm32-wasi), TinyGo, WAT
  - Testing: unit tests, `smokesig validate --check-plugins`, integration testing
  - Limitations and v2 roadmap

- [ ] 2. Update CLAUDE.md assertion table to include `plugin` field

- [ ] 3. Verify: all code snippets in the guide match actual implementation

**Commit**: `docs(plugin): add plugin authoring guide and ABI reference`

---

## Task 14: Integration Test (end-to-end)

**Files**: `internal/plugin/integration_test.go` (or `internal/runner/plugin_test.go`)

### Steps

- [ ] 1. Create an integration test that:
  - Writes a `.smokesig.yaml` with a `plugins:` section referencing `pass.wasm`
  - Creates a `Runner` with the config
  - Runs a test with `expect.plugin.test_plugin: {key: value}`
  - Asserts the test passes
  - Asserts the assertion list contains `plugin:test` type entries

- [ ] 2. Create a test for plugin failure:
  - Use `fail.wasm`
  - Assert test fails
  - Assert assertion list contains the failure details

- [ ] 3. Create a test for plugin crash:
  - Use `crash.wasm`
  - Assert test fails
  - Assert assertion type is `plugin:test_plugin:crash`

- [ ] 4. Create a test for plugin timeout:
  - Use `timeout.wasm` with 100ms timeout
  - Assert test fails
  - Assert assertion type is `plugin:test_plugin:timeout`

- [ ] 5. Create a test for `allow_failure: true` with failing plugin:
  - Use `fail.wasm` with `allow_failure: true` on the test
  - Assert `TestResult.AllowedFailure` is true

- [ ] 6. Create a test for mixed built-in + plugin assertions:
  - Test with `exit_code: 0` AND `plugin.test_plugin: {}`
  - Assert both assertion types appear in results

- [ ] 7. Verify: `go test ./internal/plugin/ -v -count=1`
- [ ] 8. Verify: `go test ./... -count=1`

**Test commands**:
```bash
go test ./internal/plugin/ -v -count=1
go test ./internal/runner/ -v -count=1
go test ./... -count=1
```

**Commit**: `test(plugin): add integration tests for wasm plugin system`

---

## Task 15: Final Verification and Cleanup

### Steps

- [ ] 1. Run full test suite: `go test ./... -count=1`
- [ ] 2. Run build: `go build -o smokesig .`
- [ ] 3. Run self-smoke: `./smokesig run`
- [ ] 4. Verify binary size increase is acceptable (~2-3MB from wazero)
- [ ] 5. Run `go vet ./...` and fix any issues
- [ ] 6. Verify `go mod tidy` produces clean go.mod/go.sum
- [ ] 7. Test with a real config file that has `plugins: {}` (empty) — should be no-op
- [ ] 8. Test with a config file that has no `plugins:` section — should work identically to today

**Test commands**:
```bash
go test ./... -count=1
go build -o smokesig .
./smokesig run
ls -la smokesig  # check binary size
go vet ./...
go mod tidy
```

**Commit**: `chore(plugin): final verification and cleanup for wasm plugin system`

---

## Dependency

Single new dependency:

```
github.com/tetratelabs/wazero v1.x.x
```

- Pure Go, zero CGO, zero transitive dependencies
- MIT licensed
- v1.0+ with semver guarantees
- Adds ~2.5MB to binary size

Install: `go get github.com/tetratelabs/wazero@latest`

---

## Execution Order

Tasks are sequential (each builds on the previous):

```
Task 1  (fixtures)     ─── foundation for all tests
Task 2  (types)        ─── needed by everything
Task 3  (schema)       ─── needed by runner, validation
Task 4  (sandbox)      ─── needed by host functions, manager
Task 5  (ABI)          ─── needed by manager
Task 6  (host funcs)   ─── needed by manager
Task 7  (manager)      ─── core engine
Task 8  (runner)       ─── integration point
Task 9  (validation)   ─── config safety
Task 10 (merge)        ─── includes/monorepo support
Task 11 (schema export)─── smokesig schema command
Task 12 (debug mode)   ─── observability
Task 13 (docs)         ─── user-facing guide
Task 14 (integration)  ─── end-to-end verification
Task 15 (final)        ─── ship check
```

Tasks 1-7 can be parallelized across two worktrees:
- **Worktree A**: Tasks 1, 2, 4, 5, 6, 7 (plugin package)
- **Worktree B**: Tasks 3, 9, 10, 11 (schema package)
- **Merge point**: Task 8 (runner integration needs both)
- **Sequential**: Tasks 12-15 (verification, docs)

---

## Risk Mitigations

| Risk | Mitigation |
|------|------------|
| WAT test fixtures are fragile | Pre-compile and commit .wasm binaries. build.sh is for regeneration only. CI never depends on wabt. |
| wazero API changes | Pin to specific version in go.mod. wazero is post-1.0 with semver. |
| Host function registration conflicts (one module name) | Register host module once in `NewPluginManager`. Each host function closure calls `SandboxFromContext(ctx)` to get the active plugin's sandbox. `Evaluate` wraps context via `ContextWithSandbox(ctx, sandbox)` before invocation. |
| Plugin assertions slow down non-plugin users | PluginManager is nil when no plugins configured. Zero overhead code path. |
| Memory leaks from module instances | Each Evaluate call creates and defers Close on a fresh module instance. No pooling in v1. |
| Map iteration non-determinism | All map iterations (Plugin, Plugins) use sorted keys. Enforced in runner (P-06), validation (P-07), export (P-09), manager load (P-06). |
