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

	// Check Memory ABI first (more specific): requires smokesig_alloc, smokesig_dealloc, evaluate
	hasAlloc := hasExport(exports, "smokesig_alloc")
	hasDealloc := hasExport(exports, "smokesig_dealloc")
	hasEval := hasExport(exports, "evaluate")

	if hasAlloc && hasDealloc && hasEval {
		return ABIVersionMemory, nil
	}

	// WASI ABI (primary): requires _start entry point
	hasStart := hasExport(exports, "_start")
	if hasStart {
		return ABIVersionWASI, nil
	}

	return 0, &PluginError{
		Kind:    ErrABIMismatch,
		Message: "plugin does not export required functions (need: _start for WASI ABI, or smokesig_alloc+smokesig_dealloc+evaluate for memory ABI)",
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
		_, _ = dealloc.Call(ctx, uint64(inputPtr), uint64(len(inputJSON)))
		return nil, classifyWazeroError(err)
	}

	// Decode result pointer: (result_ptr << 32) | result_len
	packed := evalResults[0]
	resultPtr := uint32(packed >> 32)
	resultLen := uint32(packed & 0xFFFFFFFF)

	// Read result JSON from guest memory
	resultJSON, ok := mod.Memory().Read(resultPtr, resultLen)
	if !ok {
		_, _ = dealloc.Call(ctx, uint64(inputPtr), uint64(len(inputJSON)))
		return nil, &PluginError{Kind: ErrCrash, Message: fmt.Sprintf("failed to read result from guest memory at [%d, %d)", resultPtr, resultPtr+resultLen)}
	}

	// Make a copy since the memory may be invalidated after dealloc
	resultCopy := make([]byte, len(resultJSON))
	copy(resultCopy, resultJSON)

	// Free both buffers
	_, _ = dealloc.Call(ctx, uint64(inputPtr), uint64(len(inputJSON)))
	_, _ = dealloc.Call(ctx, uint64(resultPtr), uint64(resultLen))

	// Parse result
	var output PluginOutput
	if err := json.Unmarshal(resultCopy, &output); err != nil {
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
	if containsStr(msg, "unreachable") || containsStr(msg, "wasm error") || containsStr(msg, "out of bounds") {
		return &PluginError{Kind: ErrCrash, Message: fmt.Sprintf("plugin crashed: %v", err), Cause: err}
	}
	if containsStr(msg, "context deadline exceeded") || containsStr(msg, "context canceled") {
		return &PluginError{Kind: ErrTimeout, Message: fmt.Sprintf("plugin exceeded timeout: %v", err), Cause: err}
	}
	return &PluginError{Kind: ErrCrash, Message: fmt.Sprintf("plugin error: %v", err), Cause: err}
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}
