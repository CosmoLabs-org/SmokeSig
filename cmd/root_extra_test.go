package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRootHasExpectedSubcommands(t *testing.T) {
	expected := []string{"run", "validate", "schema", "init", "version", "serve"}
	for _, name := range expected {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("root command missing subcommand %q", name)
		}
	}
}

func TestVersionOutputsVersionString(t *testing.T) {
	origVersion := Version
	Version = "test-version"
	defer func() { Version = origVersion }()

	// Capture stdout since versionCmd uses fmt.Printf
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	versionCmd.Run(versionCmd, []string{})
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "test-version") {
		t.Errorf("version output should contain version string, got: %q", out)
	}
	if !strings.HasPrefix(out, "smokesig ") {
		t.Errorf("version output should start with 'smoke ', got: %q", out)
	}
}

func TestHelpFlagProducesOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	// --help triggers SilencedError in Cobra
	_ = rootCmd.Execute()

	out := buf.String()
	if out == "" {
		t.Error("expected help output, got empty string")
	}
	if !strings.Contains(out, "smoke") {
		t.Error("help output should mention 'smoke'")
	}
	if !strings.Contains(out, "Usage") {
		t.Error("help output should contain 'Usage'")
	}
}

func TestUnknownSubcommandReturnsError(t *testing.T) {
	rootCmd.SetArgs([]string{"nonexistent-command"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown subcommand, got nil")
	}
}
