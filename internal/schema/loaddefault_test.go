package schema

import (
	"os"
	"testing"
)

func TestLoadDefaultPath_SmokesigYaml(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	if err := os.WriteFile(".smokesig.yaml", []byte("version: 1\nproject: test\ntests:\n  - name: t1\n    run: echo hi\n    expect:\n      exit_code: 0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := LoadDefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != ".smokesig.yaml" {
		t.Errorf("path = %q, want .smokesig.yaml", path)
	}
}

func TestLoadDefaultPath_FallbackSmokeYaml(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	// Only .smoke.yaml exists (no .smokesig.yaml)
	if err := os.WriteFile(".smoke.yaml", []byte("version: 1\nproject: test\ntests:\n  - name: t1\n    run: echo hi\n    expect:\n      exit_code: 0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := LoadDefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != ".smoke.yaml" {
		t.Errorf("path = %q, want .smoke.yaml", path)
	}
}

func TestLoadDefaultPath_NeitherExists(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	_, err := LoadDefaultPath()
	if err == nil {
		t.Fatal("expected error when neither config file exists")
	}
}

func TestLoadDefaultPath_PrefersSmokeSignalOverFallback(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	// Both exist -- should prefer .smokesig.yaml
	cfg := "version: 1\nproject: test\ntests:\n  - name: t1\n    run: echo hi\n    expect:\n      exit_code: 0\n"
	os.WriteFile(".smokesig.yaml", []byte(cfg), 0644)
	os.WriteFile(".smoke.yaml", []byte(cfg), 0644)

	path, err := LoadDefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != ".smokesig.yaml" {
		t.Errorf("path = %q, want .smokesig.yaml (should prefer canonical name)", path)
	}
}

func TestLoadDefault_FallbackLoadsSuccessfully(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	if err := os.WriteFile(".smoke.yaml", []byte("version: 1\nproject: fallback-test\ntests:\n  - name: t1\n    run: echo hi\n    expect:\n      exit_code: 0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "fallback-test" {
		t.Errorf("project = %q, want fallback-test", cfg.Project)
	}
}
