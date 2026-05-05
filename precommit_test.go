package main

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPrecommitHooksYAML(t *testing.T) {
	data, err := os.ReadFile(".pre-commit-hooks.yaml")
	if err != nil {
		t.Fatalf("reading .pre-commit-hooks.yaml: %v", err)
	}
	var hooks []map[string]interface{}
	if err := yaml.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("parsing YAML: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0]["id"] != "smokesig" {
		t.Errorf("hook id = %v, want smokesig", hooks[0]["id"])
	}
	if hooks[0]["entry"] != "smokesig run --fail-fast" {
		t.Errorf("hook entry = %v, want smokesig run --fail-fast", hooks[0]["entry"])
	}
}
