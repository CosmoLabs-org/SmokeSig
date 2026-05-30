package schema

import (
	"strings"
	"testing"
)

func TestValidatePluginReferencesRegistered(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Plugins: map[string]PluginEntry{
			"alpha": {Path: "alpha.wasm"},
		},
		Tests: []Test{
			{
				Name: "uses registered plugin",
				Run:  "echo ok",
				Expect: Expect{
					Plugin: map[string]interface{}{
						"alpha": map[string]interface{}{"key": "val"},
					},
				},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestValidatePluginUnregistered(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Plugins: map[string]PluginEntry{
			"alpha": {Path: "alpha.wasm"},
		},
		Tests: []Test{
			{
				Name: "uses unregistered plugin",
				Run:  "echo ok",
				Expect: Expect{
					Plugin: map[string]interface{}{
						"beta": map[string]interface{}{},
					},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unregistered plugin")
	}
	if !strings.Contains(err.Error(), "unregistered plugin \"beta\"") {
		t.Errorf("expected unregistered plugin error, got: %v", err)
	}
}

func TestValidatePluginPathRequired(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Plugins: map[string]PluginEntry{
			"empty": {Path: ""},
		},
		Tests: []Test{
			{Name: "t1", Run: "echo ok"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for empty plugin path")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("expected path required error, got: %v", err)
	}
}

func TestValidatePluginInvalidCapability(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Plugins: map[string]PluginEntry{
			"bad": {Path: "bad.wasm", Capabilities: []string{"teleport"}},
		},
		Tests: []Test{
			{Name: "t1", Run: "echo ok"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid capability")
	}
	if !strings.Contains(err.Error(), "unknown capability \"teleport\"") {
		t.Errorf("expected unknown capability error, got: %v", err)
	}
}

func TestValidatePluginExecRequiresGlobal(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Plugins: map[string]PluginEntry{
			"runner": {Path: "runner.wasm", Capabilities: []string{"exec"}},
		},
		Settings: Settings{AllowPluginExec: false},
		Tests: []Test{
			{Name: "t1", Run: "echo ok"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for exec without allow_plugin_exec")
	}
	if !strings.Contains(err.Error(), "exec capability requires settings.allow_plugin_exec") {
		t.Errorf("expected exec requires allow_plugin_exec error, got: %v", err)
	}

	// With AllowPluginExec=true should pass
	cfg.Settings.AllowPluginExec = true
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid config with allow_plugin_exec=true, got: %v", err)
	}
}

func TestValidatePluginStandalone(t *testing.T) {
	// Plugin assertions should work as standalone (no run command)
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Plugins: map[string]PluginEntry{
			"checker": {Path: "checker.wasm", Capabilities: []string{"network"}},
		},
		Tests: []Test{
			{
				Name: "standalone plugin",
				Expect: Expect{
					Plugin: map[string]interface{}{
						"checker": map[string]interface{}{"host": "localhost"},
					},
				},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("expected plugin-only test to be valid (standalone), got: %v", err)
	}
}

func TestMergeConfigsPlugins(t *testing.T) {
	base := SmokeConfig{
		Version: 1,
		Project: "base",
		Plugins: map[string]PluginEntry{
			"alpha": {Path: "alpha.wasm", Version: "1.0"},
			"beta":  {Path: "beta.wasm"},
		},
		Tests: []Test{{Name: "t1", Run: "echo ok"}},
	}

	overlay := SmokeConfig{
		Version: 1,
		Project: "overlay",
		Plugins: map[string]PluginEntry{
			"alpha":   {Path: "alpha-v2.wasm", Version: "2.0"}, // override
			"charlie": {Path: "charlie.wasm"},                   // new
		},
	}

	merged := MergeConfigs(base, overlay)

	if len(merged.Plugins) != 3 {
		t.Fatalf("expected 3 plugins after merge, got: %d", len(merged.Plugins))
	}
	if merged.Plugins["alpha"].Version != "2.0" {
		t.Errorf("expected alpha version 2.0 (last-wins), got: %s", merged.Plugins["alpha"].Version)
	}
	if merged.Plugins["beta"].Path != "beta.wasm" {
		t.Errorf("expected beta preserved from base, got: %s", merged.Plugins["beta"].Path)
	}
	if merged.Plugins["charlie"].Path != "charlie.wasm" {
		t.Errorf("expected charlie from overlay, got: %s", merged.Plugins["charlie"].Path)
	}
}

func TestMergeConfigsPluginSettings(t *testing.T) {
	base := SmokeConfig{
		Version:  1,
		Project:  "base",
		Settings: Settings{AllowPluginExec: false, PluginMemoryMB: 8},
		Tests:    []Test{{Name: "t1", Run: "echo ok"}},
	}

	overlay := SmokeConfig{
		Settings: Settings{AllowPluginExec: true, PluginMemoryMB: 32},
	}

	merged := MergeConfigs(base, overlay)
	if !merged.Settings.AllowPluginExec {
		t.Error("expected AllowPluginExec=true from overlay")
	}
	if merged.Settings.PluginMemoryMB != 32 {
		t.Errorf("expected PluginMemoryMB=32, got: %d", merged.Settings.PluginMemoryMB)
	}
}

func TestExportSchemaWithPlugins(t *testing.T) {
	plugins := map[string]PluginEntry{
		"beta":  {Path: "beta.wasm", Version: "1.0", Capabilities: []string{"network"}},
		"alpha": {Path: "alpha.wasm", Version: "2.0"},
	}

	out := ExportSchemaWithPlugins(plugins)
	if len(out.PluginAssertions) != 2 {
		t.Fatalf("expected 2 plugin assertions, got: %d", len(out.PluginAssertions))
	}
	// Should be sorted
	if out.PluginAssertions[0].Name != "alpha" {
		t.Errorf("expected first plugin 'alpha', got: %s", out.PluginAssertions[0].Name)
	}
	if out.PluginAssertions[1].Name != "beta" {
		t.Errorf("expected second plugin 'beta', got: %s", out.PluginAssertions[1].Name)
	}
	if out.PluginAssertions[1].Version != "1.0" {
		t.Errorf("expected beta version 1.0, got: %s", out.PluginAssertions[1].Version)
	}
}
