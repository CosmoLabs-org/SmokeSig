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

// TestWriteHealthError verifies writeHealthError sets the correct status code,
// Content-Type header, and JSON body.
func TestWriteHealthError(t *testing.T) {
	w := httptest.NewRecorder()
	writeHealthError(w, 500, "test error")

	if w.Code != 500 {
		t.Errorf("status code = %d, want 500", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "test error" {
		t.Errorf("error field = %q, want 'test error'", resp["error"])
	}
}

// TestWriteHealthError_404 exercises a 404 error code path.
func TestWriteHealthError_404(t *testing.T) {
	w := httptest.NewRecorder()
	writeHealthError(w, http.StatusNotFound, "not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want 404", w.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "not found" {
		t.Errorf("error field = %q, want 'not found'", resp["error"])
	}
}

// TestBuildHandler_PostAlsoRuns verifies that buildHandler runs smoke tests
// regardless of HTTP method (no method restriction in the handler).
func TestBuildHandler_PostAlsoRuns(t *testing.T) {
	cfgPath := writeTempConfig(t, `
version: 1
project: handler-post
tests:
  - name: pass
    run: "true"
    expect:
      exit_code: 0
`)
	handler := buildHandler(cfgPath)

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	// Handler runs tests on any method — POST should also get a valid JSON response.
	if w.Code != http.StatusOK {
		t.Errorf("POST: status = %d, want 200 — body: %s", w.Code, w.Body.String())
	}
}

// TestBuildHandler_MissingConfig verifies that a missing config file yields a
// non-200 error response (500) rather than a panic.
func TestBuildHandler_MissingConfig(t *testing.T) {
	handler := buildHandler("/tmp/nonexistent_smokesig_serve_config.yaml")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 status for missing config, got 200")
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected non-empty error field in response body")
	}
}

// TestBuildHandler_HealthyConfig exercises the full 200 path with a passing config.
func TestBuildHandler_HealthyConfig(t *testing.T) {
	cfgPath := writeTempConfig(t, `
version: 1
project: handler-healthy
tests:
  - name: echo-test
    run: "echo hello"
    expect:
      exit_code: 0
`)
	handler := buildHandler(cfgPath)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health status = %d, want 200 — body: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
