package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// TestServeCommand_Exists verifies the serve sub-command is registered on root.
func TestServeCommand_Exists(t *testing.T) {
	var found *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Use == "serve" {
			found = sub
			break
		}
	}
	if found == nil {
		t.Fatal("serve command not registered on rootCmd")
	}
	if found.Flags().Lookup("port") == nil {
		t.Error("serve command missing --port flag")
	}
	if found.Flags().Lookup("path") == nil {
		t.Error("serve command missing --path flag")
	}
	if found.Flags().Lookup("file") == nil {
		t.Error("serve command missing --file/-f flag")
	}
}

// writeTempConfig writes a minimal .smokesig.yaml to a temp dir and returns the path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return p
}

// TestServeHandler_Healthy checks that a passing test suite yields HTTP 200
// with status "healthy" and the correct counts.
func TestServeHandler_Healthy(t *testing.T) {
	// A test that always passes: run `true` (exit code 0).
	cfg := `
version: 1
project: test-healthy
tests:
  - name: always passes
    run: "true"
    expect:
      exit_code: 0
`
	cfgPath := writeTempConfig(t, cfg)
	handler := buildHandler(cfgPath)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "healthy" {
		t.Errorf("expected status=healthy, got %q", resp.Status)
	}
	if resp.Tests.Total != 1 || resp.Tests.Passed != 1 || resp.Tests.Failed != 0 {
		t.Errorf("unexpected counts: %+v", resp.Tests)
	}
}

// TestServeHandler_Unhealthy checks that a failing test suite yields HTTP 503
// with status "unhealthy" and failed > 0.
func TestServeHandler_Unhealthy(t *testing.T) {
	// A test that always fails: run `false` (exit code 1) but assert exit_code 0.
	cfg := `
version: 1
project: test-unhealthy
tests:
  - name: always fails
    run: "false"
    expect:
      exit_code: 0
`
	cfgPath := writeTempConfig(t, cfg)
	handler := buildHandler(cfgPath)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected HTTP 503, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "unhealthy" {
		t.Errorf("expected status=unhealthy, got %q", resp.Status)
	}
	if resp.Tests.Failed == 0 {
		t.Errorf("expected failed > 0, got %+v", resp.Tests)
	}
}
