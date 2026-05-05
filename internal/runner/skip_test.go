package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestShouldSkipEnvUnset(t *testing.T) {
	si := &schema.SkipIf{EnvUnset: "COSMO_SKIP_TEST_XYZ"}
	// Env var not set → skip
	if !shouldSkip(si, "/tmp") {
		t.Error("should skip when env var is unset")
	}

	// Set the env var → don't skip
	t.Setenv("COSMO_SKIP_TEST_XYZ", "1")
	if shouldSkip(si, "/tmp") {
		t.Error("should not skip when env var is set")
	}
}

func TestShouldSkipEnvEquals(t *testing.T) {
	si := &schema.SkipIf{EnvEquals: &schema.EnvEqualsCond{Var: "COSMO_ENV_EQ_TEST", Value: "staging"}}

	// Env var not set → don't skip (only skip if it IS equal)
	if shouldSkip(si, "/tmp") {
		t.Error("should not skip when env var is unset")
	}

	// Wrong value → don't skip
	t.Setenv("COSMO_ENV_EQ_TEST", "production")
	if shouldSkip(si, "/tmp") {
		t.Error("should not skip when env var has different value")
	}

	// Right value → skip
	t.Setenv("COSMO_ENV_EQ_TEST", "staging")
	if !shouldSkip(si, "/tmp") {
		t.Error("should skip when env var equals target value")
	}
}

func TestShouldSkipFileMissing(t *testing.T) {
	dir := t.TempDir()
	existingFile := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	// File exists → don't skip
	si := &schema.SkipIf{FileMissing: existingFile}
	if shouldSkip(si, dir) {
		t.Error("should not skip when file exists")
	}

	// File missing → skip
	si = &schema.SkipIf{FileMissing: filepath.Join(dir, "nope.txt")}
	if !shouldSkip(si, dir) {
		t.Error("should skip when file is missing")
	}

	// Relative path with configDir
	si = &schema.SkipIf{FileMissing: "exists.txt"}
	if shouldSkip(si, dir) {
		t.Error("should not skip when relative file exists in configDir")
	}

	si = &schema.SkipIf{FileMissing: "missing.txt"}
	if !shouldSkip(si, dir) {
		t.Error("should skip when relative file is missing in configDir")
	}
}

func TestShouldSkipNil(t *testing.T) {
	if shouldSkip(nil, "/tmp") {
		t.Error("nil SkipIf should not skip")
	}
}

func TestShouldSkipMultipleConditions(t *testing.T) {
	// All conditions are OR'd — any true triggers skip
	dir := t.TempDir()
	si := &schema.SkipIf{
		EnvUnset:    "COSMO_MULTI_SKIP_TEST",
		FileMissing: filepath.Join(dir, "missing.txt"),
	}
	if !shouldSkip(si, dir) {
		t.Error("should skip when any condition is true")
	}

	// Both false → don't skip
	t.Setenv("COSMO_MULTI_SKIP_TEST", "yes")
	existingFile := filepath.Join(dir, "exists.txt")
	os.WriteFile(existingFile, []byte("x"), 0644)
	si = &schema.SkipIf{
		EnvUnset:    "COSMO_MULTI_SKIP_TEST",
		FileMissing: existingFile,
	}
	if shouldSkip(si, dir) {
		t.Error("should not skip when all conditions are false")
	}
}
