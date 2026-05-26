package cmd

import (
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
