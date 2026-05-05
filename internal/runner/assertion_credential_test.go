package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestCheckCredential_EnvExists(t *testing.T) {
	t.Setenv("TEST_DB_URL", "postgres://user:pass@localhost:5432/mydb")

	check := &schema.CredentialCheck{Source: "env", Name: "TEST_DB_URL"}
	result := CheckCredential(check, "")

	if !result.Passed {
		t.Errorf("expected pass for existing env var, got fail: %s", result.Actual)
	}
	if result.Actual != "***redacted***" {
		t.Errorf("expected redacted actual, got %q", result.Actual)
	}
}

func TestCheckCredential_EnvMissing(t *testing.T) {
	check := &schema.CredentialCheck{Source: "env", Name: "SURELY_MISSING_VAR_XYZ"}
	result := CheckCredential(check, "")

	if result.Passed {
		t.Error("expected fail for missing env var")
	}
}

func TestCheckCredential_EnvContains(t *testing.T) {
	t.Setenv("TEST_REDIS_URL", "redis://localhost:6379/0")

	check := &schema.CredentialCheck{Source: "env", Name: "TEST_REDIS_URL", Contains: "redis://"}
	result := CheckCredential(check, "")

	if !result.Passed {
		t.Errorf("expected pass for env var containing substring, got fail: %s", result.Actual)
	}
}

func TestCheckCredential_EnvContainsMismatch(t *testing.T) {
	t.Setenv("TEST_CACHE_URL", "memcached://localhost:11211")

	check := &schema.CredentialCheck{Source: "env", Name: "TEST_CACHE_URL", Contains: "redis://"}
	result := CheckCredential(check, "")

	if result.Passed {
		t.Error("expected fail when env var does not contain substring")
	}
}

func TestCheckCredential_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	os.WriteFile(path, []byte("apiVersion: v1\nclusters: []"), 0644)

	check := &schema.CredentialCheck{Source: "file", Name: path}
	result := CheckCredential(check, dir)

	if !result.Passed {
		t.Errorf("expected pass for existing file, got fail: %s", result.Actual)
	}
}

func TestCheckCredential_FileMissing(t *testing.T) {
	check := &schema.CredentialCheck{Source: "file", Name: "/no/such/path/creds.json"}
	result := CheckCredential(check, "")

	if result.Passed {
		t.Error("expected fail for missing file")
	}
}

func TestCheckCredential_FileContains(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("DATABASE_URL=postgres://localhost\nAPI_KEY=secret123"), 0644)

	check := &schema.CredentialCheck{Source: "file", Name: path, Contains: "DATABASE_URL="}
	result := CheckCredential(check, dir)

	if !result.Passed {
		t.Errorf("expected pass for file containing substring, got fail: %s", result.Actual)
	}
}

func TestCheckCredential_FileRelativePath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "secret.key"), []byte("key-data"), 0644)

	check := &schema.CredentialCheck{Source: "file", Name: "secret.key"}
	result := CheckCredential(check, dir)

	if !result.Passed {
		t.Errorf("expected pass for relative file path, got fail: %s", result.Actual)
	}
}

func TestCheckCredential_ExecSuccess(t *testing.T) {
	check := &schema.CredentialCheck{Source: "exec", Name: "echo hello"}
	result := CheckCredential(check, "")

	if !result.Passed {
		t.Errorf("expected pass for successful command, got fail: %s", result.Actual)
	}
}

func TestCheckCredential_ExecFailure(t *testing.T) {
	check := &schema.CredentialCheck{Source: "exec", Name: "false"}
	result := CheckCredential(check, "")

	if result.Passed {
		t.Error("expected fail for failing command")
	}
}

func TestCheckCredential_ExecContains(t *testing.T) {
	check := &schema.CredentialCheck{Source: "exec", Name: "echo my-api-key-12345", Contains: "my-api-key"}
	result := CheckCredential(check, "")

	if !result.Passed {
		t.Errorf("expected pass for exec output containing substring, got fail: %s", result.Actual)
	}
}

func TestCheckCredential_InvalidSource(t *testing.T) {
	check := &schema.CredentialCheck{Source: "invalid", Name: "whatever"}
	result := CheckCredential(check, "")

	if result.Passed {
		t.Error("expected fail for invalid source type")
	}
}
