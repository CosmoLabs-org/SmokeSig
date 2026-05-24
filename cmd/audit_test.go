package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Reset flags for test isolation.
	auditJSON = false
	auditFix = false

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditCmd_WithConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cfg := `
version: 1
project: test-project
tests:
  - name: Build
    run: echo build
    expect:
      exit_code: 0
`
	if err := os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	auditJSON = false
	auditFix = false

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditCmd_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cfg := `
version: 1
project: test-project
tests:
  - name: Build
    run: echo build
    expect:
      exit_code: 0
`
	if err := os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	auditJSON = true
	auditFix = false

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	auditJSON = false // reset
}

func TestAuditCmd_FixMode(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Create a Go project with minimal config missing build test reference.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("FOO=bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := `
version: 1
project: test-project
tests:
  - name: Runs
    run: echo hello
    expect:
      stdout_contains: hello
`
	if err := os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	auditJSON = false
	auditFix = true

	err := runAudit(auditCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the config was updated with additional tests.
	data, err := os.ReadFile(filepath.Join(dir, ".smokesig.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if len(content) <= len(cfg) {
		t.Error("expected config to grow after --fix")
	}

	auditFix = false // reset
}
