package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"gopkg.in/yaml.v3"
)

// TestInit_EmptyDir creates a .smokesig.yaml in an empty directory.
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

	data, err := os.ReadFile(filepath.Join(dir, ".smokesig.yaml"))
	if err != nil {
		t.Fatalf("reading .smokesig.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smokesig.yaml: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Project != filepath.Base(dir) {
		t.Errorf("expected project %q, got %q", filepath.Base(dir), cfg.Project)
	}
}

// TestInit_ForceOverwrite overwrites an existing .smokesig.yaml when --force is set.
func TestInit_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create an initial config
	if err := os.WriteFile(".smokesig.yaml", []byte("version: 1\nproject: old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Without --force it should fail
	forceOverwrite = false
	fromRunning = ""
	if err := runInit(nil, nil); err == nil {
		t.Fatal("expected error when .smokesig.yaml exists without --force")
	}

	// With --force it should succeed
	forceOverwrite = true
	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit with --force failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".smokesig.yaml"))
	if err != nil {
		t.Fatalf("reading .smokesig.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smokesig.yaml: %v", err)
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

	data, err := os.ReadFile(filepath.Join(dir, ".smokesig.yaml"))
	if err != nil {
		t.Fatalf("reading .smokesig.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smokesig.yaml: %v", err)
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

// TestCountProcessTests_None — no tests have PortListening set; expect 0.
func TestCountProcessTests_None(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{
			{Name: "build", Run: "go build ./..."},
			{Name: "version", Run: "go version"},
		},
	}
	if got := countProcessTests(cfg); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// TestCountProcessTests_Some — 2 of 3 tests have PortListening; expect 2.
func TestCountProcessTests_Some(t *testing.T) {
	port1 := schema.PortCheck{Port: 8080}
	port2 := schema.PortCheck{Port: 5432}
	cfg := &schema.SmokeConfig{
		Tests: []schema.Test{
			{Name: "web", Expect: schema.Expect{PortListening: &port1}},
			{Name: "no-port", Run: "echo hi"},
			{Name: "db", Expect: schema.Expect{PortListening: &port2}},
		},
	}
	if got := countProcessTests(cfg); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

// TestCountProcessTests_Empty — config with no tests; expect 0.
func TestCountProcessTests_Empty(t *testing.T) {
	cfg := &schema.SmokeConfig{}
	if got := countProcessTests(cfg); got != 0 {
		t.Errorf("expected 0, got %d", got)
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

	data, err := os.ReadFile(filepath.Join(dir, ".smokesig.yaml"))
	if err != nil {
		t.Fatalf("reading .smokesig.yaml: %v", err)
	}

	var cfg schema.SmokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing .smokesig.yaml: %v", err)
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
