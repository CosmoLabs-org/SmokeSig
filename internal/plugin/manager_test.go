package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestNewPluginManager(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)
}

func TestLoadPluginPass(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	err = pm.LoadPlugin(ctx, "pass", schema.PluginEntry{
		Path: "pass.wasm",
	}, 16)
	if err != nil {
		t.Fatalf("LoadPlugin pass.wasm failed: %v", err)
	}

	// Should detect Memory ABI
	pm.mu.RLock()
	cfg := pm.configs["pass"]
	pm.mu.RUnlock()
	if cfg.ABIVersion != ABIVersionMemory {
		t.Errorf("expected ABI version %d, got %d", ABIVersionMemory, cfg.ABIVersion)
	}
}

func TestLoadPluginNotFound(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	err = pm.LoadPlugin(ctx, "nonexistent", schema.PluginEntry{
		Path: "nonexistent.wasm",
	}, 16)
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
	pe, ok := err.(*PluginError)
	if !ok {
		t.Fatalf("expected PluginError, got: %T", err)
	}
	if pe.Kind != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", pe.Kind)
	}
}

func TestEvaluatePass(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	err = pm.LoadPlugin(ctx, "pass", schema.PluginEntry{Path: "pass.wasm"}, 16)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	output, err := pm.Evaluate(ctx, "pass", PluginInput{
		AssertionName: "pass",
		Config:        map[string]interface{}{"key": "value"},
		Context: PluginContext{
			TestName: "test-1",
			Env:      map[string]string{},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !output.Pass {
		t.Error("expected pass=true")
	}
	if output.Message != "ok" {
		t.Errorf("expected message 'ok', got: %q", output.Message)
	}
	if len(output.Details) != 1 {
		t.Fatalf("expected 1 detail, got: %d", len(output.Details))
	}
	if !output.Details[0].Pass {
		t.Error("expected detail.pass=true")
	}
}

func TestEvaluateFail(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	err = pm.LoadPlugin(ctx, "fail", schema.PluginEntry{Path: "fail.wasm"}, 16)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	output, err := pm.Evaluate(ctx, "fail", PluginInput{
		AssertionName: "fail",
		Config:        nil,
		Context: PluginContext{
			TestName: "test-fail",
			Env:      map[string]string{},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if output.Pass {
		t.Error("expected pass=false")
	}
	if len(output.Details) != 1 {
		t.Fatalf("expected 1 detail, got: %d", len(output.Details))
	}
	if output.Details[0].Pass {
		t.Error("expected detail.pass=false")
	}
}

func TestEvaluateCrash(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	err = pm.LoadPlugin(ctx, "crash", schema.PluginEntry{Path: "crash.wasm"}, 16)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	_, err = pm.Evaluate(ctx, "crash", PluginInput{
		AssertionName: "crash",
		Context:       PluginContext{Env: map[string]string{}},
	})
	if err == nil {
		t.Fatal("expected error from crash plugin")
	}
	pe, ok := err.(*PluginError)
	if !ok {
		t.Fatalf("expected PluginError, got: %T", err)
	}
	if pe.Kind != ErrCrash {
		t.Errorf("expected ErrCrash, got: %v", pe.Kind)
	}
}

func TestEvaluateTimeout(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	err = pm.LoadPlugin(ctx, "timeout", schema.PluginEntry{
		Path:    "timeout.wasm",
		Timeout: schema.Duration{Duration: 100 * 1e6}, // 100ms
	}, 16)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	_, err = pm.Evaluate(ctx, "timeout", PluginInput{
		AssertionName: "timeout",
		Context:       PluginContext{Env: map[string]string{}},
	})
	if err == nil {
		t.Fatal("expected error from timeout plugin")
	}
	pe, ok := err.(*PluginError)
	if !ok {
		t.Fatalf("expected PluginError, got: %T", err)
	}
	if pe.Kind != ErrTimeout && pe.Kind != ErrCrash {
		// wazero may report timeout as crash depending on timing
		t.Errorf("expected ErrTimeout or ErrCrash, got: %v", pe.Kind)
	}
}

func TestEvaluateNotLoaded(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	_, err = pm.Evaluate(ctx, "nonexistent", PluginInput{})
	if err == nil {
		t.Fatal("expected error for unloaded plugin")
	}
	pe, ok := err.(*PluginError)
	if !ok {
		t.Fatalf("expected PluginError, got: %T", err)
	}
	if pe.Kind != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", pe.Kind)
	}
}

func TestPluginNames(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	// Load in non-alphabetical order
	pm.LoadPlugin(ctx, "charlie", schema.PluginEntry{Path: "pass.wasm"}, 16)
	pm.LoadPlugin(ctx, "alpha", schema.PluginEntry{Path: "pass.wasm"}, 16)
	pm.LoadPlugin(ctx, "bravo", schema.PluginEntry{Path: "pass.wasm"}, 16)

	names := pm.PluginNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got: %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "bravo" || names[2] != "charlie" {
		t.Errorf("expected sorted names [alpha bravo charlie], got: %v", names)
	}
}

func TestSHA256Caching(t *testing.T) {
	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	// Load the same .wasm twice under different names
	pm.LoadPlugin(ctx, "first", schema.PluginEntry{Path: "pass.wasm"}, 16)
	pm.LoadPlugin(ctx, "second", schema.PluginEntry{Path: "pass.wasm"}, 16)

	// Should have only 1 compiled module (same hash)
	pm.mu.RLock()
	compiledCount := len(pm.compiled)
	pm.mu.RUnlock()
	if compiledCount != 1 {
		t.Errorf("expected 1 compiled module (shared by hash), got: %d", compiledCount)
	}
}

func TestDebugMode(t *testing.T) {
	// Just verify it doesn't crash with debug enabled
	os.Setenv("SMOKESIG_PLUGIN_DEBUG", "1")
	defer os.Unsetenv("SMOKESIG_PLUGIN_DEBUG")

	ctx := context.Background()
	pm, err := NewPluginManager(ctx, ManagerOptions{
		ConfigDir: testdataDir(),
	})
	if err != nil {
		t.Fatalf("NewPluginManager failed: %v", err)
	}
	defer pm.Close(ctx)

	pm.LoadPlugin(ctx, "pass", schema.PluginEntry{Path: "pass.wasm"}, 16)
	pm.Evaluate(ctx, "pass", PluginInput{
		AssertionName: "pass",
		Context:       PluginContext{Env: map[string]string{}},
	})
}
