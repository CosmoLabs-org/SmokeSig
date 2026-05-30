package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func compileTestModule(t *testing.T, name string) wazero.CompiledModule {
	t.Helper()
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	t.Cleanup(func() { rt.Close(ctx) })

	wasmBytes := readTestdata(t, name)
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("compiling %s: %v", name, err)
	}
	return compiled
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	path := testdataDir() + "/" + name
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return data
}

func TestDetectABIMemory(t *testing.T) {
	mod := compileTestModule(t, "pass.wasm")
	abi, err := DetectABI(mod)
	if err != nil {
		t.Fatalf("DetectABI failed: %v", err)
	}
	if abi != ABIVersionMemory {
		t.Errorf("expected ABIVersionMemory (%d), got: %d", ABIVersionMemory, abi)
	}
}

func TestDetectABICrash(t *testing.T) {
	// crash.wasm also exports memory ABI functions
	mod := compileTestModule(t, "crash.wasm")
	abi, err := DetectABI(mod)
	if err != nil {
		t.Fatalf("DetectABI failed: %v", err)
	}
	if abi != ABIVersionMemory {
		t.Errorf("expected ABIVersionMemory (%d), got: %d", ABIVersionMemory, abi)
	}
}

func TestInvokeMemoryABIPass(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Need WASI for some modules (not strictly needed for memory ABI but safe)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	wasmBytes := readTestdata(t, "pass.wasm")
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	defer mod.Close(ctx)

	input := &PluginInput{
		ABIVersion:    ABIVersionMemory,
		AssertionName: "test",
		Config:        map[string]interface{}{"key": "value"},
		Context: PluginContext{
			TestName: "abi-test",
			Env:      map[string]string{},
		},
	}

	output, err := InvokeMemoryABI(ctx, mod, input)
	if err != nil {
		t.Fatalf("InvokeMemoryABI failed: %v", err)
	}
	if !output.Pass {
		t.Error("expected pass=true")
	}
	if len(output.Details) != 1 {
		t.Fatalf("expected 1 detail, got: %d", len(output.Details))
	}
}

func TestInvokeMemoryABICrash(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	wasmBytes := readTestdata(t, "crash.wasm")
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	defer mod.Close(ctx)

	input := &PluginInput{
		ABIVersion:    ABIVersionMemory,
		AssertionName: "crash",
		Context:       PluginContext{Env: map[string]string{}},
	}

	_, err = InvokeMemoryABI(ctx, mod, input)
	if err == nil {
		t.Fatal("expected error from crash module")
	}
	pe, ok := err.(*PluginError)
	if !ok {
		t.Fatalf("expected PluginError, got: %T", err)
	}
	if pe.Kind != ErrCrash {
		t.Errorf("expected ErrCrash, got: %v", pe.Kind)
	}
}

func TestClassifyWazeroError(t *testing.T) {
	tests := []struct {
		msg      string
		expected PluginErrorKind
	}{
		{"wasm error: unreachable instruction", ErrCrash},
		{"wasm error: out of bounds memory access", ErrCrash},
		{"context deadline exceeded", ErrTimeout},
		{"context canceled", ErrTimeout},
		{"some other error", ErrCrash},
	}

	for _, tt := range tests {
		err := classifyWazeroError(fmt.Errorf("%s", tt.msg))
		if err.Kind != tt.expected {
			t.Errorf("classifyWazeroError(%q): expected %v, got %v", tt.msg, tt.expected, err.Kind)
		}
	}
}

func TestPluginInputOutputJSON(t *testing.T) {
	input := PluginInput{
		ABIVersion:    1,
		AssertionName: "test",
		Config:        map[string]interface{}{"key": "value"},
		Context: PluginContext{
			TestName:   "my-test",
			ExitCode:   0,
			Stdout:     "hello",
			Stderr:     "",
			DurationMs: 100,
			Env:        map[string]string{},
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	var decoded PluginInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	if decoded.ABIVersion != 1 {
		t.Errorf("expected abi_version=1, got: %d", decoded.ABIVersion)
	}
	if decoded.AssertionName != "test" {
		t.Errorf("expected assertion_name=test, got: %s", decoded.AssertionName)
	}

	output := PluginOutput{
		Pass:    true,
		Message: "ok",
		Details: []PluginDetail{
			{Type: "check", Expected: "pass", Actual: "pass", Pass: true},
		},
	}

	data, err = json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}

	var decodedOut PluginOutput
	if err := json.Unmarshal(data, &decodedOut); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if !decodedOut.Pass {
		t.Error("expected pass=true")
	}
	if len(decodedOut.Details) != 1 {
		t.Fatalf("expected 1 detail, got: %d", len(decodedOut.Details))
	}
}
