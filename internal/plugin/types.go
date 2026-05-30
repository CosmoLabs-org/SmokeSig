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
	ABIVersion    int           `json:"abi_version"`
	AssertionName string        `json:"assertion_name"`
	Config        interface{}   `json:"config"`
	Context       PluginContext `json:"context"`
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
