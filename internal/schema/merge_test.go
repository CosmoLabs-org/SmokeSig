package schema

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMergeEnvAppendsTests(t *testing.T) {
	dir := t.TempDir()

	// Write base config
	baseYAML := `
project: myapp
tests:
  - name: base-test
    run: echo base
    expect:
      exit_code: 0
`
	basePath := filepath.Join(dir, ".smokesig.yaml")
	os.WriteFile(basePath, []byte(baseYAML), 0644)

	// Write env config
	envYAML := `
tests:
  - name: staging-test
    run: echo staging
    expect:
      exit_code: 0
`
	envPath := filepath.Join(dir, "staging.smokesig.yaml")
	os.WriteFile(envPath, []byte(envYAML), 0644)

	// Load and merge
	base, err := Load(basePath)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeEnv(base, envPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(merged.Tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(merged.Tests))
	}
	if merged.Tests[0].Name != "base-test" {
		t.Errorf("first test = %q, want base-test", merged.Tests[0].Name)
	}
	if merged.Tests[1].Name != "staging-test" {
		t.Errorf("second test = %q, want staging-test", merged.Tests[1].Name)
	}
}

func TestMergeEnvOverridesSettings(t *testing.T) {
	dir := t.TempDir()

	baseYAML := `
project: myapp
settings:
  timeout: 10s
tests:
  - name: base
    run: "true"
    expect:
      exit_code: 0
`
	basePath := filepath.Join(dir, ".smokesig.yaml")
	os.WriteFile(basePath, []byte(baseYAML), 0644)

	envYAML := `
settings:
  timeout: 60s
  fail_fast: true
tests:
  - name: env
    run: "true"
    expect:
      exit_code: 0
`
	envPath := filepath.Join(dir, "production.smokesig.yaml")
	os.WriteFile(envPath, []byte(envYAML), 0644)

	base, _ := Load(basePath)
	merged, err := MergeEnv(base, envPath)
	if err != nil {
		t.Fatal(err)
	}

	if merged.Settings.Timeout.Duration != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", merged.Settings.Timeout.Duration)
	}
	if !merged.Settings.FailFast {
		t.Error("fail_fast should be true")
	}
}

func TestMergeEnvMissingFile(t *testing.T) {
	base := &SmokeConfig{Tests: []Test{{Name: "t", Run: "true"}}}
	_, err := MergeEnv(base, "/nonexistent/staging.smokesig.yaml")
	if err == nil {
		t.Error("should fail on missing env file")
	}
}

func TestMergeEnvPrependsPrereqs(t *testing.T) {
	dir := t.TempDir()

	baseYAML := `
prerequisites:
  - name: base-prereq
    check: "true"
tests:
  - name: base
    run: "true"
    expect:
      exit_code: 0
`
	basePath := filepath.Join(dir, ".smokesig.yaml")
	os.WriteFile(basePath, []byte(baseYAML), 0644)

	envYAML := `
prerequisites:
  - name: env-prereq
    check: "true"
tests: []
`
	envPath := filepath.Join(dir, "ci.smokesig.yaml")
	os.WriteFile(envPath, []byte(envYAML), 0644)

	base, _ := Load(basePath)
	merged, err := MergeEnv(base, envPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(merged.Prereqs) != 2 {
		t.Fatalf("expected 2 prereqs, got %d", len(merged.Prereqs))
	}
	// Env prereqs prepended
	if merged.Prereqs[0].Name != "env-prereq" {
		t.Errorf("first prereq = %q, want env-prereq", merged.Prereqs[0].Name)
	}
}
