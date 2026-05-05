//go:build ignore
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"gopkg.in/yaml.v3"
)

// TestInit_EmptyDir creates a .smoke.yaml in an empty directory.
func TestInit_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	forceOverwrite = false
	fromRunning = ""

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".smoke.yaml"))
	if err != nil {
		t.Fatalf("reading .smoke.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smoke.yaml: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Project != filepath.Base(dir) {
		t.Errorf("expected project %q, got %q", filepath.Base(dir), cfg.Project)
	}
}

// TestInit_ForceOverwrite overwrites an existing .smoke.yaml when --force is set.
func TestInit_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create an initial config
	if err := os.WriteFile(".smoke.yaml", []byte("version: 1\nproject: old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Without --force it should fail
	forceOverwrite = false
	fromRunning = ""
	if err := runInit(nil, nil); err == nil {
		t.Fatal("expected error when .smoke.yaml exists without --force")
	}

	// With --force it should succeed
	forceOverwrite = true
	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit with --force failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".smoke.yaml"))
	if err != nil {
		t.Fatalf("reading .smoke.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smoke.yaml: %v", err)
	}
	if cfg.Project == "old" {
		t.Error("expected config to be overwritten, but project is still 'old'")
	}
}

// TestInit_DetectGoProject detects a Go project from go.mod.
func TestInit_DetectGoProject(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create go.mod marker
	if err := os.WriteFile("go.mod", []byte("module example.com/test\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}

	forceOverwrite = false
	fromRunning = ""

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".smoke.yaml"))
	if err != nil {
		t.Fatalf("reading .smoke.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smoke.yaml: %v", err)
	}

	// Go project should have "go build" and "go test" tests
	found := false
	for _, tc := range cfg.Tests {
		if tc.Run == "go build ./..." {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Go project to have 'go build ./...' test")
	}
}

// TestInit_DetectNodeProject detects a Node project from package.json.
func TestInit_DetectNodeProject(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create package.json marker (npm project, no bun.lock)
	pkg := `{"name": "test-app", "scripts": {"test": "jest"}}`
	if err := os.WriteFile("package.json", []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}

	forceOverwrite = false
	fromRunning = ""

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".smoke.yaml"))
	if err != nil {
		t.Fatalf("reading .smoke.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smoke.yaml: %v", err)
	}

	// Node/npm project should have "npm install" test
	found := false
	for _, tc := range cfg.Tests {
		if tc.Run == "npm install" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Node project to have 'npm install' test")
	}
}
