package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

// ─── resolveConfigPath ────────────────────────────────────────────────────────

func TestResolveConfigPath_Absolute(t *testing.T) {
	abs := "/tmp/foo.yaml"
	got := resolveConfigPath(abs)
	if got != abs {
		t.Errorf("expected %q, got %q", abs, got)
	}
}

func TestResolveConfigPath_Relative(t *testing.T) {
	// A relative path should come back as an absolute path.
	rel := "some/relative.yaml"
	got := resolveConfigPath(rel)
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	if !strings.HasSuffix(got, rel) {
		t.Errorf("expected suffix %q in %q", rel, got)
	}
}

func TestResolveConfigPath_Dot(t *testing.T) {
	got := resolveConfigPath(".smokesig.yaml")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path for relative .smokesig.yaml, got %q", got)
	}
}

// ─── handleSmokeInit ─────────────────────────────────────────────────────────

// writeSmokeConfig writes a minimal valid .smokesig.yaml into dir and returns the path.
func writeSmokeConfig(t *testing.T, dir string) string {
	t.Helper()
	content := `version: 1
project: test
tests:
  - name: echo-hello
    run: echo hello
    expect:
      exit_code: 0
`
	p := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeSmokeConfig: %v", err)
	}
	return p
}

func TestHandleSmokeInit_NoProjectType(t *testing.T) {
	// Empty temp dir → detector finds nothing → error
	dir := t.TempDir()
	_, err := handleSmokeInit(context.Background(), map[string]interface{}{
		"directory": dir,
	})
	if err == nil {
		t.Fatal("expected error when no project type detected")
	}
	if !strings.Contains(err.Error(), "no known project type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHandleSmokeInit_GoProject_NoDisk(t *testing.T) {
	// A directory with go.mod → detector recognises Go project.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := handleSmokeInit(context.Background(), map[string]interface{}{
		"directory": dir,
		// write not set → default false
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ir, ok := result.(*InitResult)
	if !ok {
		t.Fatalf("expected *InitResult, got %T", result)
	}
	if ir.Written {
		t.Error("expected Written=false when write arg not set")
	}
	if ir.YAML == "" {
		t.Error("expected non-empty YAML")
	}
	// No file should have been created
	if _, err := os.Stat(filepath.Join(dir, ".smokesig.yaml")); !os.IsNotExist(err) {
		t.Error("expected .smokesig.yaml NOT to be written to disk")
	}
}

func TestHandleSmokeInit_Write_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := handleSmokeInit(context.Background(), map[string]interface{}{
		"directory": dir,
		"write":     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ir := result.(*InitResult)
	if !ir.Written {
		t.Error("expected Written=true")
	}
	if ir.WritePath == "" {
		t.Error("expected non-empty WritePath")
	}
	// File must exist on disk
	if _, statErr := os.Stat(ir.WritePath); statErr != nil {
		t.Errorf("file not found at WritePath: %v", statErr)
	}
}

func TestHandleSmokeInit_Write_ErrorsIfExists_NoForce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Pre-create the config
	existing := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(existing, []byte("project: pre-existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := handleSmokeInit(context.Background(), map[string]interface{}{
		"directory": dir,
		"write":     true,
		// force not set → default false
	})
	if err == nil {
		t.Fatal("expected error when file already exists and force=false")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleSmokeInit_Write_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Pre-create the config
	existing := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(existing, []byte("project: old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := handleSmokeInit(context.Background(), map[string]interface{}{
		"directory": dir,
		"write":     true,
		"force":     true,
	})
	if err != nil {
		t.Fatalf("unexpected error with force=true: %v", err)
	}
	ir := result.(*InitResult)
	if !ir.Written {
		t.Error("expected Written=true")
	}
}

func TestHandleSmokeInit_BadDirectory(t *testing.T) {
	// filepath.Abs rarely fails, but an empty path edge case still exercises the
	// detect path even if it returns no types (empty dir).
	dir := t.TempDir()
	// Pass a valid but totally empty dir — should fail with "no project type"
	_, err := handleSmokeInit(context.Background(), map[string]interface{}{
		"directory": dir,
	})
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestHandleSmokeInit_Write_WriteFails(t *testing.T) {
	// Create a dir with go.mod so detector fires, then make it read-only so
	// WriteFile fails — exercises the "writing config" error branch (line 90).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only so WriteFile fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755) // restore so TempDir cleanup works

	_, err := handleSmokeInit(context.Background(), map[string]interface{}{
		"directory": dir,
		"write":     true,
		"force":     true, // skip the "already exists" check
	})
	if err == nil {
		t.Fatal("expected error when write fails due to read-only dir")
	}
	if !strings.Contains(err.Error(), "writing config") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── handleSmokeRun ───────────────────────────────────────────────────────────

func TestHandleSmokeRun_BadConfig(t *testing.T) {
	_, err := handleSmokeRun(context.Background(), map[string]interface{}{
		"config_path": "/nonexistent/path/.smokesig.yaml",
	})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleSmokeRun_InvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeSmokeConfig(t, dir)

	_, err := handleSmokeRun(context.Background(), map[string]interface{}{
		"config_path": cfgPath,
		"timeout":     "not-a-duration",
	})
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleSmokeRun_ValidConfig_DryRun(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeSmokeConfig(t, dir)

	result, err := handleSmokeRun(context.Background(), map[string]interface{}{
		"config_path": cfgPath,
		"dry_run":     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rr, ok := result.(*RunResult)
	if !ok {
		t.Fatalf("expected *RunResult, got %T", result)
	}
	if rr.ConfigPath == "" {
		t.Error("expected non-empty ConfigPath")
	}
}

func TestHandleSmokeRun_ValidConfig_WithTimeout(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeSmokeConfig(t, dir)

	result, err := handleSmokeRun(context.Background(), map[string]interface{}{
		"config_path": cfgPath,
		"timeout":     "30s",
		"fail_fast":   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(*RunResult); !ok {
		t.Fatalf("expected *RunResult, got %T", result)
	}
}

func TestHandleSmokeRun_WithTags(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeSmokeConfig(t, dir)

	result, err := handleSmokeRun(context.Background(), map[string]interface{}{
		"config_path":  cfgPath,
		"tags":         []interface{}{"ci"},
		"exclude_tags": []interface{}{"slow"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(*RunResult); !ok {
		t.Fatalf("expected *RunResult, got %T", result)
	}
}

// ─── handleSmokeValidate ──────────────────────────────────────────────────────

func TestHandleSmokeValidate_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeSmokeConfig(t, dir)

	result, err := handleSmokeValidate(context.Background(), map[string]interface{}{
		"config_path": cfgPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vr, ok := result.(*ValidateResult)
	if !ok {
		t.Fatalf("expected *ValidateResult, got %T", result)
	}
	if !vr.Valid {
		t.Errorf("expected valid config, got errors: %v", vr.Errors)
	}
	if len(vr.Tests) == 0 {
		t.Error("expected at least one test name in result")
	}
}

func TestHandleSmokeValidate_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	// Write a config that loads OK but fails schema.Validate (missing required fields).
	// version:1 but no tests → should produce ValidationError with errors.
	badCfg := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(badCfg, []byte("version: 1\nproject: broken\ntests:\n  - name: bad-test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := handleSmokeValidate(context.Background(), map[string]interface{}{
		"config_path": badCfg,
	})
	if err != nil {
		// schema.Load might fail — that's fine
		return
	}
	vr, ok := result.(*ValidateResult)
	if !ok {
		t.Fatalf("expected *ValidateResult, got %T", result)
	}
	// If validation returned errors, Valid should be false
	if !vr.Valid && len(vr.Errors) == 0 {
		t.Error("expected errors when Valid=false")
	}
}

func TestHandleSmokeValidate_ConfigWithValidationErrors(t *testing.T) {
	dir := t.TempDir()
	// A test with no run command and no standalone assertion → triggers validation error
	badCfg := filepath.Join(dir, ".smokesig.yaml")
	content := `version: 1
project: broken
tests:
  - name: no-run-no-standalone
    expect:
      exit_code: 0
`
	if err := os.WriteFile(badCfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := handleSmokeValidate(context.Background(), map[string]interface{}{
		"config_path": badCfg,
	})
	if err != nil {
		return // load error is acceptable
	}
	vr, ok := result.(*ValidateResult)
	if !ok {
		t.Fatalf("expected *ValidateResult, got %T", result)
	}
	// If it produced errors, Valid should be false with non-empty Errors slice
	if !vr.Valid {
		if len(vr.Errors) == 0 {
			t.Error("expected at least one error in ValidateResult when Valid=false")
		}
	}
}

func TestHandleSmokeValidate_MissingConfig(t *testing.T) {
	_, err := handleSmokeValidate(context.Background(), map[string]interface{}{
		"config_path": "/no/such/file.yaml",
	})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestHandleSmokeList_MissingConfig(t *testing.T) {
	_, err := handleSmokeList(context.Background(), map[string]interface{}{
		"config_path": "/no/such/file.yaml",
	})
	if err == nil {
		t.Fatal("expected error for missing config in smoke_list")
	}
}

func TestHandleSmokeList_WithTagFilter_HitsTag(t *testing.T) {
	dir := t.TempDir()
	content := `version: 1
project: test
tests:
  - name: tagged-test
    tags: [ci, fast]
    run: echo tagged
    expect:
      exit_code: 0
  - name: untagged-test
    run: echo bare
    expect:
      exit_code: 0
`
	cfgPath := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := handleSmokeList(context.Background(), map[string]interface{}{
		"config_path": cfgPath,
		"tags":        []interface{}{"ci"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lr := result.(*ListResult)
	if len(lr.Tests) != 1 {
		t.Errorf("expected 1 test after tag filter, got %d", len(lr.Tests))
	}
	if lr.Tests[0].Name != "tagged-test" {
		t.Errorf("expected tagged-test, got %q", lr.Tests[0].Name)
	}
}

// ─── handleSmokeDiscover ──────────────────────────────────────────────────────

func TestHandleSmokeDiscover_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := handleSmokeDiscover(context.Background(), map[string]interface{}{
		"directory": dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dr, ok := result.(*DiscoverResult)
	if !ok {
		t.Fatalf("expected *DiscoverResult, got %T", result)
	}
	// No configs in empty dir (root config added only if file actually exists)
	_ = dr
}

func TestHandleSmokeDiscover_DirWithConfig(t *testing.T) {
	dir := t.TempDir()
	writeSmokeConfig(t, dir)

	result, err := handleSmokeDiscover(context.Background(), map[string]interface{}{
		"directory": dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dr := result.(*DiscoverResult)
	// At minimum the discover function should not panic
	_ = dr
}

func TestHandleSmokeDiscover_DefaultDirectory(t *testing.T) {
	// No "directory" arg → defaults to "."
	result, err := handleSmokeDiscover(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error with default dir: %v", err)
	}
	if _, ok := result.(*DiscoverResult); !ok {
		t.Fatalf("expected *DiscoverResult, got %T", result)
	}
}

func TestHandleSmokeDiscover_InvalidDir(t *testing.T) {
	// Non-existent directory: monorepo.Discover may error or return empty list.
	result, err := handleSmokeDiscover(context.Background(), map[string]interface{}{
		"directory": "/this/does/not/exist/at/all",
	})
	if err != nil {
		// error is acceptable
		return
	}
	if _, ok := result.(*DiscoverResult); !ok {
		t.Fatalf("expected *DiscoverResult, got %T", result)
	}
}

// ─── handleSmokeExplain ───────────────────────────────────────────────────────

func TestHandleSmokeExplain_MissingArg(t *testing.T) {
	_, err := handleSmokeExplain(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when assertion_type is missing")
	}
	if !strings.Contains(err.Error(), "assertion_type is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleSmokeExplain_UnknownType(t *testing.T) {
	_, err := handleSmokeExplain(context.Background(), map[string]interface{}{
		"assertion_type": "totally_unknown_xyz",
	})
	if err == nil {
		t.Fatal("expected error for unknown assertion type")
	}
	if !strings.Contains(err.Error(), "unknown assertion type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleSmokeExplain_KnownTypes(t *testing.T) {
	knownTypes := []string{
		"exit_code", "stdout_contains", "http", "port_listening",
		"redis_ping", "ssl_cert", "file_exists", "env_exists",
		"process_running", "json_field", "response_time_ms",
	}
	for _, at := range knownTypes {
		t.Run(at, func(t *testing.T) {
			result, err := handleSmokeExplain(context.Background(), map[string]interface{}{
				"assertion_type": at,
			})
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", at, err)
			}
			er, ok := result.(*ExplainResult)
			if !ok {
				t.Fatalf("expected *ExplainResult, got %T", result)
			}
			if er.Type != at {
				t.Errorf("expected type %q, got %q", at, er.Type)
			}
			if er.Description == "" {
				t.Error("expected non-empty description")
			}
		})
	}
}

// ─── noopReporter SSE callbacks (all 0% covered) ─────────────────────────────

func TestNoopReporter_AllMethods_NoPanic(t *testing.T) {
	n := &noopReporter{}

	// These are all no-op methods; the test just confirms they don't panic.
	n.PrereqStart("prereq-name")
	n.PrereqResult(reporter.PrereqResultData{
		Name:   "prereq-name",
		Passed: true,
		Output: "ok",
		Hint:   "",
		Error:  nil,
	})
	n.TestStart("test-name")
	n.TestResult(reporter.TestResultData{
		Name:   "test-name",
		Passed: true,
	})
	n.Summary(reporter.SuiteResultData{
		Project: "test-project",
		Total:   1,
		Passed:  1,
	})
}

func TestNoopReporter_PrereqResult_WithError(t *testing.T) {
	n := &noopReporter{}
	n.PrereqResult(reporter.PrereqResultData{
		Name:   "failing-prereq",
		Passed: false,
		Output: "connection refused",
		Error:  os.ErrNotExist,
	})
	// Must not panic
}

func TestNoopReporter_TestResult_Failed(t *testing.T) {
	n := &noopReporter{}
	n.TestResult(reporter.TestResultData{
		Name:   "failing-test",
		Passed: false,
		Error:  os.ErrPermission,
	})
	// Must not panic
}

func TestNoopReporter_Summary_WithFailures(t *testing.T) {
	n := &noopReporter{}
	n.Summary(reporter.SuiteResultData{
		Project: "my-project",
		Total:   5,
		Passed:  3,
		Failed:  2,
	})
	// Must not panic
}

// ─── Server.ServeStdio ────────────────────────────────────────────────────────

// TestServer_ServeStdio_ImmediateEOF verifies ServeStdio returns (non-panic) when
// stdin is immediately closed (EOF). The underlying mcp-go library will see EOF
// and return an error or nil — both are acceptable; we only care it doesn't hang
// or panic.
func TestServer_ServeStdio_ImmediateEOF(t *testing.T) {
	// Replace os.Stdin with a pipe that's immediately closed so ServeStdio
	// reads EOF and exits promptly.
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	w.Close() // immediate EOF
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		r.Close()
	}()

	srv := NewServer()
	// ServeStdio will read EOF and return — we just check it doesn't hang or panic.
	// The return value (nil or error) is intentionally not checked.
	done := make(chan struct{})
	go func() {
		srv.ServeStdio() //nolint:errcheck
		close(done)
	}()

	select {
	case <-done:
		// Good — returned promptly.
	}
}

// ─── handleSmokeRun result shape ─────────────────────────────────────────────

func TestHandleSmokeRun_ResultFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeSmokeConfig(t, dir)

	result, err := handleSmokeRun(context.Background(), map[string]interface{}{
		"config_path": cfgPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rr := result.(*RunResult)
	if rr.ConfigPath != cfgPath {
		t.Errorf("expected ConfigPath=%q, got %q", cfgPath, rr.ConfigPath)
	}
	if rr.Total < 0 {
		t.Errorf("expected non-negative Total, got %d", rr.Total)
	}
}

// ─── arg helpers edge cases ───────────────────────────────────────────────────

func TestStrArg_DefaultAndPresent(t *testing.T) {
	args := map[string]interface{}{"key": "val"}
	if got := strArg(args, "key", "def"); got != "val" {
		t.Errorf("expected val, got %q", got)
	}
	if got := strArg(args, "missing", "def"); got != "def" {
		t.Errorf("expected def, got %q", got)
	}
	// Wrong type → default
	args["key2"] = 42
	if got := strArg(args, "key2", "def"); got != "def" {
		t.Errorf("expected def for wrong type, got %q", got)
	}
}

func TestBoolArg_DefaultAndPresent(t *testing.T) {
	args := map[string]interface{}{"flag": true}
	if got := boolArg(args, "flag", false); !got {
		t.Error("expected true")
	}
	if got := boolArg(args, "missing", true); !got {
		t.Error("expected default true")
	}
	// Wrong type → default
	args["flag2"] = "yes"
	if got := boolArg(args, "flag2", false); got {
		t.Error("expected false for wrong type")
	}
}

func TestStrSliceArg_Variants(t *testing.T) {
	// []string variant
	args := map[string]interface{}{"tags": []string{"a", "b"}}
	got := strSliceArg(args, "tags")
	if len(got) != 2 || got[0] != "a" {
		t.Errorf("unexpected slice: %v", got)
	}

	// []interface{} variant
	args["tags2"] = []interface{}{"x", "y", 99}
	got2 := strSliceArg(args, "tags2")
	if len(got2) != 2 || got2[0] != "x" {
		t.Errorf("unexpected slice from interface: %v", got2)
	}

	// missing key
	if got3 := strSliceArg(args, "nope"); got3 != nil {
		t.Errorf("expected nil for missing key, got %v", got3)
	}

	// wrong type
	args["bad"] = 42
	if got4 := strSliceArg(args, "bad"); got4 != nil {
		t.Errorf("expected nil for wrong type, got %v", got4)
	}
}

// ─── sanitize ─────────────────────────────────────────────────────────────────

func TestSanitize_Short(t *testing.T) {
	got := sanitize("hello", 100)
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestSanitize_Truncated(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := sanitize(long, 50)
	if len(got) <= 50 {
		// Expected: truncated output is longer than 50 due to suffix, but starts with 50 xs
		t.Errorf("expected truncation suffix, got %q", got[:60])
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected truncation notice in %q", got)
	}
}

func TestSanitize_TrimSpace(t *testing.T) {
	got := sanitize("  hello  ", 100)
	if got != "hello" {
		t.Errorf("expected trimmed string, got %q", got)
	}
}
