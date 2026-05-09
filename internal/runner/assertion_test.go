package runner

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// ---------------------------------------------------------------------------
// CheckExitCode
// ---------------------------------------------------------------------------

func TestCheckExitCode_Pass(t *testing.T) {
	r := CheckExitCode(0, 0)
	if !r.Passed {
		t.Errorf("expected pass, got fail: actual=%s expected=%s", r.Actual, r.Expected)
	}
	if r.Type != "exit_code" {
		t.Errorf("expected type 'exit_code', got %q", r.Type)
	}
}

func TestCheckExitCode_Fail(t *testing.T) {
	r := CheckExitCode(1, 0)
	if r.Passed {
		t.Errorf("expected fail, got pass")
	}
	if r.Actual != "1" || r.Expected != "0" {
		t.Errorf("unexpected values: actual=%s expected=%s", r.Actual, r.Expected)
	}
}

func TestCheckExitCode_NonZeroExpected(t *testing.T) {
	r := CheckExitCode(2, 2)
	if !r.Passed {
		t.Errorf("expected pass for exit code 2 == 2")
	}
}

func TestCheckExitCode_LargeCode(t *testing.T) {
	r := CheckExitCode(127, 127)
	if !r.Passed {
		t.Errorf("expected pass for exit code 127 == 127")
	}
}

// ---------------------------------------------------------------------------
// CheckStdoutContains
// ---------------------------------------------------------------------------

func TestCheckStdoutContains_Pass(t *testing.T) {
	r := CheckStdoutContains("hello world", "world")
	if !r.Passed {
		t.Errorf("expected pass")
	}
	if r.Type != "stdout_contains" {
		t.Errorf("expected type 'stdout_contains', got %q", r.Type)
	}
}

func TestCheckStdoutContains_Fail(t *testing.T) {
	r := CheckStdoutContains("hello world", "missing")
	if r.Passed {
		t.Errorf("expected fail")
	}
}

func TestCheckStdoutContains_EmptyStdout(t *testing.T) {
	r := CheckStdoutContains("", "anything")
	if r.Passed {
		t.Errorf("expected fail for empty stdout")
	}
}

func TestCheckStdoutContains_EmptySubstr(t *testing.T) {
	// Empty substring is always contained in any string (Go strings.Contains behavior)
	r := CheckStdoutContains("hello", "")
	if !r.Passed {
		t.Errorf("expected pass: empty substring is contained in any string")
	}
}

func TestCheckStdoutContains_BothEmpty(t *testing.T) {
	r := CheckStdoutContains("", "")
	if !r.Passed {
		t.Errorf("expected pass: empty substring in empty string")
	}
}

func TestCheckStdoutContains_Multiline(t *testing.T) {
	stdout := "line1\nline2\nline3"
	r := CheckStdoutContains(stdout, "line2")
	if !r.Passed {
		t.Errorf("expected pass for multiline stdout")
	}
}

func TestCheckStdoutContains_ExactMatch(t *testing.T) {
	r := CheckStdoutContains("exact", "exact")
	if !r.Passed {
		t.Errorf("expected pass for exact match")
	}
}

// ---------------------------------------------------------------------------
// CheckStdoutMatches
// ---------------------------------------------------------------------------

func TestCheckStdoutMatches_Pass(t *testing.T) {
	r := CheckStdoutMatches("version 1.2.3", `version \d+\.\d+\.\d+`)
	if !r.Passed {
		t.Errorf("expected pass")
	}
	if r.Type != "stdout_matches" {
		t.Errorf("expected type 'stdout_matches', got %q", r.Type)
	}
}

func TestCheckStdoutMatches_Fail(t *testing.T) {
	r := CheckStdoutMatches("hello world", `^\d+$`)
	if r.Passed {
		t.Errorf("expected fail")
	}
}

func TestCheckStdoutMatches_EmptyStdout(t *testing.T) {
	r := CheckStdoutMatches("", `\w+`)
	if r.Passed {
		t.Errorf("expected fail for empty stdout against word pattern")
	}
}

func TestCheckStdoutMatches_EmptyStdoutMatchesEmptyPattern(t *testing.T) {
	r := CheckStdoutMatches("", "")
	if !r.Passed {
		t.Errorf("expected pass: empty pattern matches empty string")
	}
}

func TestCheckStdoutMatches_InvalidRegex(t *testing.T) {
	r := CheckStdoutMatches("some output", `[invalid`)
	if r.Passed {
		t.Errorf("expected fail for invalid regex")
	}
	// Actual should describe the error
	if r.Actual == "some output" {
		t.Errorf("expected Actual to contain error description, got stdout instead")
	}
}

func TestCheckStdoutMatches_AnchoredPattern(t *testing.T) {
	r := CheckStdoutMatches("ok", `^ok$`)
	if !r.Passed {
		t.Errorf("expected pass for anchored exact match")
	}
}

func TestCheckStdoutMatches_AnchoredPatternFail(t *testing.T) {
	r := CheckStdoutMatches("ok extra", `^ok$`)
	if r.Passed {
		t.Errorf("expected fail: anchored pattern should not match with extra content on same line")
	}
}

func TestCheckStdoutMatches_CaseInsensitiveNotDefault(t *testing.T) {
	// By default regex is case-sensitive
	r := CheckStdoutMatches("Hello", `hello`)
	if r.Passed {
		t.Errorf("expected fail: regex is case-sensitive by default")
	}
}

func TestCheckStdoutMatches_CaseFlagOverride(t *testing.T) {
	r := CheckStdoutMatches("Hello", `(?i)hello`)
	if !r.Passed {
		t.Errorf("expected pass with case-insensitive flag")
	}
}

// ---------------------------------------------------------------------------
// CheckStderrContains
// ---------------------------------------------------------------------------

func TestCheckStderrContains_Pass(t *testing.T) {
	r := CheckStderrContains("error: connection refused", "connection refused")
	if !r.Passed {
		t.Errorf("expected pass")
	}
	if r.Type != "stderr_contains" {
		t.Errorf("expected type 'stderr_contains', got %q", r.Type)
	}
}

func TestCheckStderrContains_Fail(t *testing.T) {
	r := CheckStderrContains("error: timeout", "connection refused")
	if r.Passed {
		t.Errorf("expected fail")
	}
}

func TestCheckStderrContains_EmptyStderr(t *testing.T) {
	r := CheckStderrContains("", "error")
	if r.Passed {
		t.Errorf("expected fail for empty stderr")
	}
}

func TestCheckStderrContains_EmptySubstr(t *testing.T) {
	r := CheckStderrContains("some error", "")
	if !r.Passed {
		t.Errorf("expected pass: empty substring is always contained")
	}
}

// ---------------------------------------------------------------------------
// CheckFileExists
// ---------------------------------------------------------------------------

func TestCheckFileExists_AbsolutePathExists(t *testing.T) {
	tmp, err := os.CreateTemp("", "smoke-assert-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	r := CheckFileExists(tmp.Name(), "/some/config/dir")
	if !r.Passed {
		t.Errorf("expected pass for existing absolute path %s", tmp.Name())
	}
	if r.Type != "file_exists" {
		t.Errorf("expected type 'file_exists', got %q", r.Type)
	}
}

func TestCheckFileExists_AbsolutePathMissing(t *testing.T) {
	r := CheckFileExists("/nonexistent/path/to/file.txt", "/some/config/dir")
	if r.Passed {
		t.Errorf("expected fail for nonexistent absolute path")
	}
}

func TestCheckFileExists_RelativePathExists(t *testing.T) {
	dir := t.TempDir()
	filename := "output.txt"
	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	r := CheckFileExists(filename, dir)
	if !r.Passed {
		t.Errorf("expected pass for relative path resolved against configDir: %s", fullPath)
	}
	// Expected and Actual should both be the resolved absolute path
	if r.Expected != fullPath {
		t.Errorf("expected resolved path %q, got %q", fullPath, r.Expected)
	}
}

func TestCheckFileExists_RelativePathMissing(t *testing.T) {
	dir := t.TempDir()
	r := CheckFileExists("ghost.txt", dir)
	if r.Passed {
		t.Errorf("expected fail for missing relative path")
	}
}

func TestCheckFileExists_RelativeSubdirExists(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	fname := filepath.Join(subdir, "result.json")
	if err := os.WriteFile(fname, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	r := CheckFileExists("sub/result.json", dir)
	if !r.Passed {
		t.Errorf("expected pass for nested relative path")
	}
}

func TestCheckFileExists_AbsolutePathIgnoresConfigDir(t *testing.T) {
	// When given an absolute path, configDir should be ignored entirely
	tmp, err := os.CreateTemp("", "smoke-abs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	r := CheckFileExists(tmp.Name(), "/completely/unrelated/dir")
	if !r.Passed {
		t.Errorf("expected pass: absolute path should ignore configDir")
	}
	if r.Expected != tmp.Name() {
		t.Errorf("expected resolved path to equal original absolute path, got %q", r.Expected)
	}
}

func TestCheckFileExists_EmptyConfigDir(t *testing.T) {
	// Relative path with empty configDir resolves to just the filename (relative to cwd)
	// This should not panic
	r := CheckFileExists("nonexistent.txt", "")
	// Result depends on whether file exists in cwd, but it must not panic
	_ = r
}

// ---------------------------------------------------------------------------
// CheckFileSize
// ---------------------------------------------------------------------------

func TestCheckFileSize_WithinRange(t *testing.T) {
	tmp := t.TempDir()
	f, _ := os.CreateTemp(tmp, "testfile")
	f.Write(make([]byte, 1024))
	f.Close()

	min := int64(512)
	max := int64(2048)
	r := CheckFileSize(&schema.FileSizeCheck{Path: f.Name(), MinBytes: &min, MaxBytes: &max}, "")
	if !r.Passed {
		t.Errorf("expected pass, got: expected=%s actual=%s", r.Expected, r.Actual)
	}
	if r.Type != "file_size" {
		t.Errorf("expected type 'file_size', got %q", r.Type)
	}
}

func TestCheckFileSize_ExceedsMax(t *testing.T) {
	tmp := t.TempDir()
	f, _ := os.CreateTemp(tmp, "testfile")
	f.Write(make([]byte, 2048))
	f.Close()

	max := int64(1024)
	r := CheckFileSize(&schema.FileSizeCheck{Path: f.Name(), MaxBytes: &max}, "")
	if r.Passed {
		t.Fatal("expected fail")
	}
	if r.Actual != "2.0KB" {
		t.Errorf("expected actual '2.0KB', got %q", r.Actual)
	}
}

func TestCheckFileSize_BelowMin(t *testing.T) {
	tmp := t.TempDir()
	f, _ := os.CreateTemp(tmp, "testfile")
	f.Write(make([]byte, 100))
	f.Close()

	min := int64(512)
	r := CheckFileSize(&schema.FileSizeCheck{Path: f.Name(), MinBytes: &min}, "")
	if r.Passed {
		t.Fatal("expected fail")
	}
	if r.Expected != ">=512B" {
		t.Errorf("expected '>=512B', got %q", r.Expected)
	}
}

func TestCheckFileSize_FileNotFound(t *testing.T) {
	r := CheckFileSize(&schema.FileSizeCheck{Path: "/nonexistent/file.txt"}, "")
	if r.Passed {
		t.Fatal("expected fail for missing file")
	}
}

func TestCheckFileSize_NoThresholds(t *testing.T) {
	tmp := t.TempDir()
	f, _ := os.CreateTemp(tmp, "testfile")
	f.Write(make([]byte, 42))
	f.Close()

	r := CheckFileSize(&schema.FileSizeCheck{Path: f.Name()}, "")
	if !r.Passed {
		t.Fatal("expected pass (existence check only)")
	}
}

func TestCheckFileSize_MinOnly(t *testing.T) {
	tmp := t.TempDir()
	f, _ := os.CreateTemp(tmp, "testfile")
	f.Write(make([]byte, 2048))
	f.Close()

	min := int64(1024)
	r := CheckFileSize(&schema.FileSizeCheck{Path: f.Name(), MinBytes: &min}, "")
	if !r.Passed {
		t.Errorf("expected pass, got: expected=%s actual=%s", r.Expected, r.Actual)
	}
}

func TestCheckFileSize_MaxOnly(t *testing.T) {
	tmp := t.TempDir()
	f, _ := os.CreateTemp(tmp, "testfile")
	f.Write(make([]byte, 100))
	f.Close()

	max := int64(1024)
	r := CheckFileSize(&schema.FileSizeCheck{Path: f.Name(), MaxBytes: &max}, "")
	if !r.Passed {
		t.Errorf("expected pass, got: expected=%s actual=%s", r.Expected, r.Actual)
	}
}

// ---------------------------------------------------------------------------
// CheckStderrMatches
// ---------------------------------------------------------------------------

func TestCheckStderrMatches_Pass(t *testing.T) {
	r := CheckStderrMatches("error: line 42: undefined", `line \d+`)
	if !r.Passed {
		t.Errorf("expected pass")
	}
	if r.Type != "stderr_matches" {
		t.Errorf("expected type 'stderr_matches', got %q", r.Type)
	}
}

func TestCheckStderrMatches_Fail(t *testing.T) {
	r := CheckStderrMatches("error: unknown", `line \d+`)
	if r.Passed {
		t.Errorf("expected fail")
	}
}

func TestCheckStderrMatches_InvalidRegex(t *testing.T) {
	r := CheckStderrMatches("some error", "[invalid(regex")
	if r.Passed {
		t.Errorf("expected fail for invalid regex")
	}
	if r.Actual == "" {
		t.Errorf("expected error message in Actual")
	}
}

func TestCheckStderrMatches_EmptyStderr(t *testing.T) {
	r := CheckStderrMatches("", "error")
	if r.Passed {
		t.Errorf("expected fail for empty stderr")
	}
}

func TestCheckStderrMatches_EmptyPattern(t *testing.T) {
	r := CheckStderrMatches("some error", "")
	if !r.Passed {
		t.Errorf("expected pass: empty pattern matches everything")
	}
}

// ---------------------------------------------------------------------------
// CheckEnvExists
// ---------------------------------------------------------------------------

func TestCheckEnvExists_Pass(t *testing.T) {
	os.Setenv("SMOKE_TEST_VAR", "value123")
	defer os.Unsetenv("SMOKE_TEST_VAR")

	r := CheckEnvExists("SMOKE_TEST_VAR")
	if !r.Passed {
		t.Errorf("expected pass for set env var")
	}
	if r.Type != "env_exists" {
		t.Errorf("expected type 'env_exists', got %q", r.Type)
	}
	if r.Actual != "value123" {
		t.Errorf("expected Actual to be 'value123', got %q", r.Actual)
	}
}

func TestCheckEnvExists_Fail(t *testing.T) {
	os.Unsetenv("SMOKE_NONEXISTENT_VAR")

	r := CheckEnvExists("SMOKE_NONEXISTENT_VAR")
	if r.Passed {
		t.Errorf("expected fail for unset env var")
	}
	if r.Actual != "" {
		t.Errorf("expected empty Actual for unset var, got %q", r.Actual)
	}
}

func TestCheckEnvExists_EmptyValue(t *testing.T) {
	os.Setenv("SMOKE_EMPTY_VAR", "")
	defer os.Unsetenv("SMOKE_EMPTY_VAR")

	r := CheckEnvExists("SMOKE_EMPTY_VAR")
	if r.Passed {
		t.Errorf("expected fail: empty string should count as not set")
	}
}

// ---------------------------------------------------------------------------
// CheckPortListening
// ---------------------------------------------------------------------------

func TestCheckPortListening_Fail(t *testing.T) {
	r := CheckPortListening(59999, "tcp", "localhost")
	if r.Passed {
		t.Errorf("expected fail for closed port")
	}
	if r.Type != "port_listening" {
		t.Errorf("expected type 'port_listening', got %q", r.Type)
	}
}

// ---------------------------------------------------------------------------
// CheckProcessRunning
// ---------------------------------------------------------------------------

func TestCheckProcessRunning_ExistingProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pgrep not available on Windows; tasklist path covered via manual verification")
	}
	// Spawn a long-running 'sleep' process so we have a predictable target
	// for pgrep -x. Both macOS and Linux ship /bin/sleep.
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	// Give the OS a moment to register the process in the process table.
	time.Sleep(50 * time.Millisecond)
	r := CheckProcessRunning("sleep")
	if !r.Passed {
		t.Errorf("expected pass for running 'sleep' process, got actual=%s", r.Actual)
	}
	if r.Type != "process_running" {
		t.Errorf("expected type 'process_running', got %q", r.Type)
	}
}

func TestCheckProcessRunning_MissingProcess(t *testing.T) {
	r := CheckProcessRunning("definitely-not-a-real-process-xyz123")
	if r.Passed {
		t.Errorf("expected fail for non-existent process")
	}
	if r.Actual != "not found" {
		t.Errorf("expected actual 'not found', got %q", r.Actual)
	}
	if r.Type != "process_running" {
		t.Errorf("expected type 'process_running', got %q", r.Type)
	}
}

func TestCheckProcessRunning_EmptyName(t *testing.T) {
	r := CheckProcessRunning("")
	if r.Passed {
		t.Errorf("expected fail for empty process name")
	}
	if r.Type != "process_running" {
		t.Errorf("expected type 'process_running', got %q", r.Type)
	}
}

// ---------------------------------------------------------------------------
// CheckHTTP
// ---------------------------------------------------------------------------

func TestCheckHTTP_StatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	status := 200
	results := CheckHTTP(&schema.HTTPCheck{
		URL:        srv.URL,
		StatusCode: &status,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected status check to pass")
	}
	if results[0].Type != "http_status" {
		t.Errorf("expected type 'http_status', got %q", results[0].Type)
	}
}

func TestCheckHTTP_StatusCode_Fail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	status := 200
	results := CheckHTTP(&schema.HTTPCheck{
		URL:        srv.URL,
		StatusCode: &status,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Errorf("expected status check to fail for 404")
	}
}

func TestCheckHTTP_BodyContains(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer srv.Close()

	results := CheckHTTP(&schema.HTTPCheck{
		URL:          srv.URL,
		BodyContains: "healthy",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected body_contains to pass")
	}
}

func TestCheckHTTP_BodyMatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`version: 1.2.3`))
	}))
	defer srv.Close()

	results := CheckHTTP(&schema.HTTPCheck{
		URL:         srv.URL,
		BodyMatches: `version: \d+\.\d+\.\d+`,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected body_matches to pass")
	}
}

func TestCheckHTTP_HeaderContains(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	results := CheckHTTP(&schema.HTTPCheck{
		URL:            srv.URL,
		HeaderContains: map[string]string{"Content-Type": "json"},
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected header_contains to pass")
	}
}

func TestCheckHTTP_POST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	status := 201
	results := CheckHTTP(&schema.HTTPCheck{
		URL:        srv.URL,
		Method:     "POST",
		Body:       `{"name":"test"}`,
		StatusCode: &status,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected POST with 201 to pass")
	}
}

func TestCheckHTTP_InvalidURL(t *testing.T) {
	results := CheckHTTP(&schema.HTTPCheck{
		URL: "http://localhost:99999/nonexistent",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Errorf("expected request to fail for invalid URL")
	}
	if results[0].Type != "http_request" {
		t.Errorf("expected type 'http_request', got %q", results[0].Type)
	}
}

func TestCheckHTTP_MultipleAssertions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer srv.Close()

	status := 200
	results := CheckHTTP(&schema.HTTPCheck{
		URL:            srv.URL,
		StatusCode:     &status,
		BodyContains:   "OK",
		HeaderContains: map[string]string{"X-Custom": "value"},
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results (status, body, header), got %d", len(results))
	}
	for i, r := range results {
		if !r.Passed {
			t.Errorf("result %d (%s) failed", i, r.Type)
		}
	}
}

// ---------------------------------------------------------------------------
// CheckJSONField
// ---------------------------------------------------------------------------

func TestCheckJSONField_Equals(t *testing.T) {
	json := `{"version": "1.2.3", "status": "healthy"}`
	results := CheckJSONField(json, &schema.JSONFieldCheck{
		Path:   "version",
		Equals: "1.2.3",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected equals check to pass")
	}
	if results[0].Type != "json_field_equals" {
		t.Errorf("expected type 'json_field_equals', got %q", results[0].Type)
	}
}

func TestCheckJSONField_Equals_Fail(t *testing.T) {
	json := `{"version": "1.2.3"}`
	results := CheckJSONField(json, &schema.JSONFieldCheck{
		Path:   "version",
		Equals: "2.0.0",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Errorf("expected equals check to fail")
	}
}

func TestCheckJSONField_Contains(t *testing.T) {
	json := `{"message": "hello world"}`
	results := CheckJSONField(json, &schema.JSONFieldCheck{
		Path:     "message",
		Contains: "world",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected contains check to pass")
	}
}

func TestCheckJSONField_Matches(t *testing.T) {
	json := `{"version": "v1.2.3"}`
	results := CheckJSONField(json, &schema.JSONFieldCheck{
		Path:    "version",
		Matches: `v\d+\.\d+\.\d+`,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected matches check to pass")
	}
}

func TestCheckJSONField_NestedPath(t *testing.T) {
	json := `{"data": {"items": [{"name": "first"}, {"name": "second"}]}}`
	results := CheckJSONField(json, &schema.JSONFieldCheck{
		Path:   "data.items.0.name",
		Equals: "first",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected nested path check to pass")
	}
}

func TestCheckJSONField_NotFound(t *testing.T) {
	json := `{"foo": "bar"}`
	results := CheckJSONField(json, &schema.JSONFieldCheck{
		Path:   "nonexistent",
		Equals: "anything",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Errorf("expected check to fail for missing field")
	}
	if results[0].Actual != "field not found" {
		t.Errorf("expected 'field not found', got %q", results[0].Actual)
	}
}

func TestCheckJSONField_InvalidJSON(t *testing.T) {
	results := CheckJSONField("not json", &schema.JSONFieldCheck{
		Path:   "anything",
		Equals: "anything",
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Errorf("expected check to fail for invalid JSON")
	}
	if results[0].Actual != "invalid JSON" {
		t.Errorf("expected 'invalid JSON', got %q", results[0].Actual)
	}
}

func TestCheckJSONField_MultipleChecks(t *testing.T) {
	json := `{"version": "1.2.3"}`
	results := CheckJSONField(json, &schema.JSONFieldCheck{
		Path:     "version",
		Equals:   "1.2.3",
		Contains: "2.3",
		Matches:  `\d+\.\d+\.\d+`,
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.Passed {
			t.Errorf("result %d (%s) failed", i, r.Type)
		}
	}
}

// ---------------------------------------------------------------------------
// CheckResponseTime
// ---------------------------------------------------------------------------

func TestCheckResponseTime_PassUnder(t *testing.T) {
	r := CheckResponseTime(50, 100)
	if !r.Passed {
		t.Errorf("expected pass when actual(50ms) < threshold(100ms): actual=%s expected=%s", r.Actual, r.Expected)
	}
	if r.Type != "response_time_ms" {
		t.Errorf("expected type 'response_time_ms', got %q", r.Type)
	}
}

func TestCheckResponseTime_PassEqual(t *testing.T) {
	r := CheckResponseTime(100, 100)
	if !r.Passed {
		t.Errorf("expected pass when actual(100ms) == threshold(100ms): actual=%s expected=%s", r.Actual, r.Expected)
	}
}

func TestCheckResponseTime_FailOver(t *testing.T) {
	r := CheckResponseTime(200, 100)
	if r.Passed {
		t.Errorf("expected fail when actual(200ms) > threshold(100ms)")
	}
	if r.Actual != "200ms" {
		t.Errorf("expected actual='200ms', got %q", r.Actual)
	}
	if r.Expected != "<= 100ms" {
		t.Errorf("expected expected='<= 100ms', got %q", r.Expected)
	}
}

// ---------------------------------------------------------------------------
// CheckSSLCert
// ---------------------------------------------------------------------------

// TestCheckSSLCert_Pass_AllowSelfSigned starts a local TLS test server (self-signed)
// and verifies that CheckSSLCert passes when AllowSelfSigned is true.
func TestCheckSSLCert_Pass_AllowSelfSigned(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse test server addr: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	r := CheckSSLCert(&schema.SSLCertCheck{
		Host:            host,
		Port:            port,
		AllowSelfSigned: true,
	})
	if !r.Passed {
		t.Errorf("expected pass with AllowSelfSigned=true, got fail: actual=%s", r.Actual)
	}
	if r.Type != "ssl_cert" {
		t.Errorf("expected type 'ssl_cert', got %q", r.Type)
	}
}

// TestCheckSSLCert_Fail_SelfSignedRejected verifies that a self-signed cert is rejected
// when AllowSelfSigned is false (the default).
func TestCheckSSLCert_Fail_SelfSignedRejected(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse test server addr: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	r := CheckSSLCert(&schema.SSLCertCheck{
		Host:            host,
		Port:            port,
		AllowSelfSigned: false,
	})
	if r.Passed {
		t.Errorf("expected fail for self-signed cert without AllowSelfSigned, got pass")
	}
}

// TestCheckSSLCert_Fail_UnreachableHost verifies that a connection failure returns a failed result.
func TestCheckSSLCert_Fail_UnreachableHost(t *testing.T) {
	r := CheckSSLCert(&schema.SSLCertCheck{
		Host:            "127.0.0.1",
		Port:            19999,
		AllowSelfSigned: true,
	})
	if r.Passed {
		t.Errorf("expected fail for unreachable host, got pass")
	}
	if r.Type != "ssl_cert" {
		t.Errorf("expected type 'ssl_cert', got %q", r.Type)
	}
}

// TestCheckSSLCert_Pass_MinDaysRemaining verifies that a long-lived cert satisfies the threshold.
func TestCheckSSLCert_Pass_MinDaysRemaining(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse test server addr: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	r := CheckSSLCert(&schema.SSLCertCheck{
		Host:             host,
		Port:             port,
		AllowSelfSigned:  true,
		MinDaysRemaining: 30,
	})
	if !r.Passed {
		t.Errorf("expected pass with 30-day threshold on long-lived cert, got fail: actual=%s", r.Actual)
	}
}

// ---------------------------------------------------------------------------
// Helpers: fake TCP servers
// ---------------------------------------------------------------------------

// startFakeRedis spins up a TCP listener that reads one request and replies
// with the given bytes.
func startFakeRedis(t *testing.T, reply string) (string, int) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lis.Close() })
	go func() {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		conn.Read(buf)            //nolint:errcheck
		conn.Write([]byte(reply)) //nolint:errcheck
	}()
	host, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// startFakeMemcached spins up a TCP listener that reads one request and
// replies with the given bytes.
func startFakeMemcached(t *testing.T, reply string) (string, int) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lis.Close() })
	go func() {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		conn.Read(buf)            //nolint:errcheck
		conn.Write([]byte(reply)) //nolint:errcheck
	}()
	host, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// ---------------------------------------------------------------------------
// CheckRedisPing
// ---------------------------------------------------------------------------

func TestCheckRedisPing_Pass(t *testing.T) {
	host, port := startFakeRedis(t, "+PONG\r\n")
	r := CheckRedisPing(&schema.RedisCheck{Host: host, Port: port})
	if !r.Passed {
		t.Errorf("expected pass, got fail: actual=%s", r.Actual)
	}
	if r.Type != "redis_ping" {
		t.Errorf("expected type 'redis_ping', got %q", r.Type)
	}
	if r.Actual != "PONG" {
		t.Errorf("expected Actual='PONG', got %q", r.Actual)
	}
}

func TestCheckRedisPing_WrongReply(t *testing.T) {
	host, port := startFakeRedis(t, "-ERR unknown\r\n")
	r := CheckRedisPing(&schema.RedisCheck{Host: host, Port: port})
	if r.Passed {
		t.Errorf("expected fail, got pass")
	}
	if !strings.Contains(r.Actual, "ERR") {
		t.Errorf("expected actual to contain 'ERR', got %q", r.Actual)
	}
}

func TestCheckRedisPing_Unreachable(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	lis.Close()

	r := CheckRedisPing(&schema.RedisCheck{Host: "127.0.0.1", Port: port})
	if r.Passed {
		t.Errorf("expected fail for unreachable address, got pass")
	}
}

// ---------------------------------------------------------------------------
// CheckMemcachedVersion
// ---------------------------------------------------------------------------

func TestCheckMemcachedVersion_Pass(t *testing.T) {
	host, port := startFakeMemcached(t, "VERSION 1.6.0\r\n")
	r := CheckMemcachedVersion(&schema.MemcachedCheck{Host: host, Port: port})
	if !r.Passed {
		t.Errorf("expected pass, got fail: actual=%s", r.Actual)
	}
	if r.Type != "memcached_version" {
		t.Errorf("expected type 'memcached_version', got %q", r.Type)
	}
	if !strings.HasPrefix(r.Actual, "VERSION") {
		t.Errorf("expected Actual to start with 'VERSION', got %q", r.Actual)
	}
}

func TestCheckMemcachedVersion_GarbageReply(t *testing.T) {
	host, port := startFakeMemcached(t, "NOPE garbage\r\n")
	r := CheckMemcachedVersion(&schema.MemcachedCheck{Host: host, Port: port})
	if r.Passed {
		t.Errorf("expected fail, got pass")
	}
	if r.Expected != "VERSION ..." {
		t.Errorf("expected Expected='VERSION ...', got %q", r.Expected)
	}
}

func TestCheckMemcachedVersion_Unreachable(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	lis.Close()

	r := CheckMemcachedVersion(&schema.MemcachedCheck{Host: "127.0.0.1", Port: port})
	if r.Passed {
		t.Errorf("expected fail for unreachable address, got pass")
	}
}

// ---------------------------------------------------------------------------
// Helpers: fake Postgres and MySQL TCP servers
// ---------------------------------------------------------------------------

// startFakePostgres listens once, reads the SSLRequest, replies with a single byte.
func startFakePostgres(t *testing.T, reply byte) (string, int) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lis.Close() })
	go func() {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 8)
		conn.Read(buf)            //nolint:errcheck
		conn.Write([]byte{reply}) //nolint:errcheck
	}()
	host, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// startFakeMySQL listens once and immediately writes a handshake packet.
func startFakeMySQL(t *testing.T, protocolVersion byte) (string, int) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lis.Close() })
	go func() {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// 3-byte len, 1-byte seq, 1-byte protocol version + padding
		pkt := []byte{0x0a, 0x00, 0x00, 0x00, protocolVersion, 0x00, 0x00, 0x00, 0x00, 0x00}
		conn.Write(pkt) //nolint:errcheck
	}()
	host, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// ---------------------------------------------------------------------------
// CheckPostgresPing
// ---------------------------------------------------------------------------

func TestCheckPostgresPing_Pass_SSLSupported(t *testing.T) {
	host, port := startFakePostgres(t, 'S')
	r := CheckPostgresPing(&schema.PostgresCheck{Host: host, Port: port})
	if !r.Passed {
		t.Errorf("expected pass, got fail: actual=%s", r.Actual)
	}
	if r.Type != "postgres_ping" {
		t.Errorf("expected type 'postgres_ping', got %q", r.Type)
	}
	if r.Actual != "S" {
		t.Errorf("expected Actual='S', got %q", r.Actual)
	}
}

func TestCheckPostgresPing_Pass_SSLNotSupported(t *testing.T) {
	host, port := startFakePostgres(t, 'N')
	r := CheckPostgresPing(&schema.PostgresCheck{Host: host, Port: port})
	if !r.Passed {
		t.Errorf("expected pass, got fail: actual=%s", r.Actual)
	}
	if r.Actual != "N" {
		t.Errorf("expected Actual='N', got %q", r.Actual)
	}
}

func TestCheckPostgresPing_Pass_ErrorReply(t *testing.T) {
	host, port := startFakePostgres(t, 'E')
	r := CheckPostgresPing(&schema.PostgresCheck{Host: host, Port: port})
	if !r.Passed {
		t.Errorf("expected pass for 'E' reply, got fail: actual=%s", r.Actual)
	}
	if r.Actual != "E" {
		t.Errorf("expected Actual='E', got %q", r.Actual)
	}
}

func TestCheckPostgresPing_GarbageReply(t *testing.T) {
	host, port := startFakePostgres(t, 0xff)
	r := CheckPostgresPing(&schema.PostgresCheck{Host: host, Port: port})
	if r.Passed {
		t.Errorf("expected fail for garbage reply, got pass")
	}
	if !strings.Contains(r.Actual, "0xff") {
		t.Errorf("expected Actual to contain '0xff', got %q", r.Actual)
	}
}

func TestCheckPostgresPing_Unreachable(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	lis.Close()

	r := CheckPostgresPing(&schema.PostgresCheck{Host: "127.0.0.1", Port: port})
	if r.Passed {
		t.Errorf("expected fail for unreachable address, got pass")
	}
}

// ---------------------------------------------------------------------------
// CheckMySQLPing
// ---------------------------------------------------------------------------

func TestCheckMySQLPing_Pass_V10(t *testing.T) {
	host, port := startFakeMySQL(t, 0x0a)
	r := CheckMySQLPing(&schema.MySQLCheck{Host: host, Port: port})
	if !r.Passed {
		t.Errorf("expected pass, got fail: actual=%s", r.Actual)
	}
	if r.Type != "mysql_ping" {
		t.Errorf("expected type 'mysql_ping', got %q", r.Type)
	}
	if r.Actual != "v10" {
		t.Errorf("expected Actual='v10', got %q", r.Actual)
	}
}

func TestCheckMySQLPing_WrongProtocolVersion(t *testing.T) {
	host, port := startFakeMySQL(t, 0x09)
	r := CheckMySQLPing(&schema.MySQLCheck{Host: host, Port: port})
	if r.Passed {
		t.Errorf("expected fail for protocol version 0x09, got pass")
	}
	if !strings.Contains(r.Actual, "0x09") {
		t.Errorf("expected Actual to contain '0x09', got %q", r.Actual)
	}
}

func TestCheckMySQLPing_Unreachable(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)
	lis.Close()

	r := CheckMySQLPing(&schema.MySQLCheck{Host: "127.0.0.1", Port: port})
	if r.Passed {
		t.Errorf("expected fail for unreachable address, got pass")
	}
}

// ---------------------------------------------------------------------------
// CheckDockerContainerRunning
// ---------------------------------------------------------------------------

func TestCheckDockerContainerRunning_NotFound(t *testing.T) {
	r := CheckDockerContainerRunning(&schema.DockerContainerCheck{Name: "SmokeSig-nonexistent-container-xyz"})
	if r.Passed {
		t.Error("expected fail for non-existent container, got pass")
	}
	if r.Type != "docker_container_running" {
		t.Errorf("expected type 'docker_container_running', got %q", r.Type)
	}
}

func TestCheckDockerContainerRunning_Pass(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("docker daemon not available")
	}
	// This test requires a running container named "smokesig-test-alpine".
	// Start one with: docker run -d --name smokesig-test-alpine --rm alpine sleep 300
	out, err := exec.Command("docker", "ps", "-q", "--filter", "name=smokesig-test-alpine").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Skip("test container 'smokesig-test-alpine' not running")
	}
	r := CheckDockerContainerRunning(&schema.DockerContainerCheck{Name: "smokesig-test-alpine"})
	if !r.Passed {
		t.Errorf("expected pass, got fail: actual=%s", r.Actual)
	}
}

// ---------------------------------------------------------------------------
// CheckDockerImageExists
// ---------------------------------------------------------------------------

func TestCheckDockerImageExists_NotFound(t *testing.T) {
	r := CheckDockerImageExists(&schema.DockerImageCheck{Image: "SmokeSig-nonexistent-image-xyz:latest"})
	if r.Passed {
		t.Error("expected fail for non-existent image, got pass")
	}
	if r.Type != "docker_image_exists" {
		t.Errorf("expected type 'docker_image_exists', got %q", r.Type)
	}
}

func TestCheckDockerImageExists_Pass(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("docker daemon not available")
	}
	// Pull a tiny image if not present
	exec.Command("docker", "pull", "alpine:latest").Run() //nolint:errcheck
	r := CheckDockerImageExists(&schema.DockerImageCheck{Image: "alpine:latest"})
	if !r.Passed {
		t.Errorf("expected pass for alpine:latest, got fail: actual=%s", r.Actual)
	}
}

func TestCheckHTTP_TraceparentInjected(t *testing.T) {
	var receivedTraceparent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceparent = r.Header.Get("traceparent")
		w.WriteHeader(200)
	}))
	defer ts.Close()

	check := &schema.HTTPCheck{
		URL:        ts.URL,
		StatusCode: intPtr(200),
	}
	span := NewTraceContext().NewSpan()
	results := CheckHTTPWithTrace(check, span)
	for _, r := range results {
		if !r.Passed {
			t.Errorf("unexpected failure: %v", r)
		}
	}
	if receivedTraceparent == "" {
		t.Error("expected traceparent header to be injected")
	}
	expected := span.Traceparent()
	if receivedTraceparent != expected {
		t.Errorf("traceparent = %q, want %q", receivedTraceparent, expected)
	}
}
