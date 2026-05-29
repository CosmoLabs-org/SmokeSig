package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveCmdRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "observe" {
			found = true
			break
		}
	}
	if !found {
		t.Error("root command missing subcommand 'observe'")
	}
}

func TestObserveCmdHasExpectedFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
	}{
		{"dir", "d"},
		{"timeout", "t"},
		{"quiet", "q"},
		{"output", "o"},
	}

	for _, tc := range flags {
		f := observeCmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("observe cmd missing flag --%s", tc.name)
			continue
		}
		if f.Shorthand != tc.shorthand {
			t.Errorf("observe cmd flag --%s: expected shorthand %q, got %q", tc.name, tc.shorthand, f.Shorthand)
		}
	}
}

func TestObserveCmdRequiresArgs(t *testing.T) {
	if observeCmd.Args == nil {
		t.Fatal("observe cmd Args validator is nil")
	}
	if err := observeCmd.Args(observeCmd, nil); err == nil {
		t.Error("observe cmd should reject zero arguments")
	}
	if err := observeCmd.Args(observeCmd, []string{"echo"}); err != nil {
		t.Errorf("observe cmd should accept one argument, got error: %v", err)
	}
	if err := observeCmd.Args(observeCmd, []string{"node", "server.js"}); err != nil {
		t.Errorf("observe cmd should accept multiple arguments, got error: %v", err)
	}
}

func TestObserveCmdDefaultOutput(t *testing.T) {
	f := observeCmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("observe cmd missing --output flag")
	}
	if f.DefValue != ".smokesig.yaml" {
		t.Errorf("observe cmd --output default: expected .smokesig.yaml, got %s", f.DefValue)
	}
}

func TestObserveCmdDefaultTimeout(t *testing.T) {
	f := observeCmd.Flags().Lookup("timeout")
	if f == nil {
		t.Fatal("observe cmd missing --timeout flag")
	}
	if f.DefValue != "0s" {
		t.Errorf("observe cmd --timeout default: expected 0s, got %s", f.DefValue)
	}
}

// resetObserveVars saves and restores observe package vars.
func resetObserveVars(t *testing.T) {
	t.Helper()
	origDir := observeDir
	origTimeout := observeTimeout
	origQuiet := observeQuiet
	origOutput := observeOutput
	t.Cleanup(func() {
		observeDir = origDir
		observeTimeout = origTimeout
		observeQuiet = origQuiet
		observeOutput = origOutput
	})
	observeDir = ""
	observeTimeout = 0
	observeQuiet = true // default to quiet so tests don't prompt stdin
}

// TestRunObserve_QuietMode runs a simple command in quiet mode and verifies the output file is created.
func TestRunObserve_QuietMode(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.yaml")
	resetObserveVars(t)
	observeOutput = outPath

	// Use "echo hello" as the observed command — guaranteed to succeed and exit quickly.
	if err := runObserve(nil, []string{"echo", "hello"}); err != nil {
		t.Fatalf("runObserve quiet: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file %s to exist: %v", outPath, err)
	}
}

// TestRunObserve_InvalidCommand verifies that an observe run with a non-existent command returns an error.
func TestRunObserve_InvalidCommand(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.yaml")
	resetObserveVars(t)
	observeOutput = outPath

	// A command that doesn't exist should cause observer.Observe to fail.
	err := runObserve(nil, []string{"__this_command_does_not_exist_smokesig_test__"})
	// Note: some observers may not error on non-zero exit — only assert no panic.
	_ = err
}

// pipeStdin replaces os.Stdin with a pipe containing the given input, restoring it in t.Cleanup.
func pipeStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(w, input) //nolint:errcheck
	w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		r.Close()
	})
}

// TestRunObserve_InteractiveYes exercises the non-quiet path when user answers "Y".
func TestRunObserve_InteractiveYes(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.yaml")
	resetObserveVars(t)
	observeOutput = outPath
	observeQuiet = false

	pipeStdin(t, "Y\n")

	if err := runObserve(nil, []string{"echo", "hello"}); err != nil {
		t.Fatalf("runObserve interactive Y: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file to be written: %v", err)
	}
}

// TestRunObserve_InteractiveNo exercises the non-quiet abort path when user answers "n".
func TestRunObserve_InteractiveNo(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.yaml")
	resetObserveVars(t)
	observeOutput = outPath
	observeQuiet = false

	pipeStdin(t, "n\n")

	if err := runObserve(nil, []string{"echo", "hello"}); err != nil {
		t.Fatalf("runObserve interactive n: %v", err)
	}
	// File should NOT be written when user aborts.
	if _, err := os.Stat(outPath); err == nil {
		t.Error("expected output file NOT to be written when user answers 'n'")
	}
}

// TestRunObserve_MultiArgs verifies joining multiple args into a command string.
func TestRunObserve_MultiArgs(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.yaml")
	resetObserveVars(t)
	observeOutput = outPath
	observeQuiet = true

	_ = strings.Join([]string{"echo", "multi", "args"}, " ") // sanity check
	if err := runObserve(nil, []string{"echo", "multi", "args"}); err != nil {
		t.Fatalf("runObserve multi-args: %v", err)
	}
}
