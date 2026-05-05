package cmd

import (
	"os"
	"testing"
)

func TestValidateCmd_ValidConfig(t *testing.T) {
	cfg := `
version: 1
project: test-project
tests:
  - name: hello
    run: echo hello
    expect:
      exit_code: 0
`
	dir := t.TempDir()
	path := dir + "/.smokesig.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v\n%s", err, out)
	}
	if out == "" {
		t.Error("expected some output")
	}
}

func TestValidateCmd_InvalidConfig(t *testing.T) {
	cfg := `
version: 2
project: ""
tests: []
`
	dir := t.TempDir()
	path := dir + "/.smokesig.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if out == "" {
		t.Error("expected error output")
	}
}

func TestValidateCmd_FileNotFound(t *testing.T) {
	_, err := runValidate("/nonexistent/.smokesig.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateCmd_AllErrors(t *testing.T) {
	cfg := `
version: 3
tests:
  - run: echo hi
`
	dir := t.TempDir()
	path := dir + "/.smokesig.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Should report multiple errors: version, project, test name
	if len(out) < 50 {
		t.Errorf("expected multi-error output, got: %s", out)
	}
}
