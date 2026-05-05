package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// --- filterTests extended coverage ---

func TestExtended_FilterTests_MultipleIncludeTags(t *testing.T) {
	tests := []schema.Test{
		{Name: "build-unit", Tags: []string{"build", "unit"}},
		{Name: "build-integration", Tags: []string{"build", "integration"}},
		{Name: "deploy", Tags: []string{"deploy"}},
		{Name: "untagged"},
	}

	got := filterTests(tests, []string{"build", "deploy"}, nil)
	if len(got) != 3 {
		t.Fatalf("got %d tests, want 3", len(got))
	}

	names := make(map[string]bool)
	for _, tt := range got {
		names[tt.Name] = true
	}
	for _, want := range []string{"build-unit", "build-integration", "deploy"} {
		if !names[want] {
			t.Errorf("missing test %q in filtered results", want)
		}
	}
	if names["untagged"] {
		t.Error("untagged test should not be included when include tags are specified")
	}
}

func TestExtended_FilterTests_IncludeCaseInsensitive(t *testing.T) {
	tests := []schema.Test{
		{Name: "lower", Tags: []string{"build"}},
		{Name: "upper", Tags: []string{"BUILD"}},
	}

	got := filterTests(tests, []string{"Build"}, nil)
	if len(got) != 2 {
		t.Errorf("got %d tests, want 2 (case-insensitive match)", len(got))
	}
}

func TestExtended_FilterTests_MultipleIncludeNoMatch(t *testing.T) {
	tests := []schema.Test{
		{Name: "a", Tags: []string{"unit"}},
		{Name: "b", Tags: []string{"integration"}},
	}

	got := filterTests(tests, []string{"smoke", "e2e"}, nil)
	if len(got) != 0 {
		t.Errorf("got %d tests, want 0 (no tags match)", len(got))
	}
}

func TestExtended_FilterTests_ExcludeOnly_MultipleTags(t *testing.T) {
	tests := []schema.Test{
		{Name: "fast", Tags: []string{"fast"}},
		{Name: "slow", Tags: []string{"slow"}},
		{Name: "flaky", Tags: []string{"flaky"}},
		{Name: "untagged"},
	}

	got := filterTests(tests, nil, []string{"slow", "flaky"})
	if len(got) != 2 {
		t.Fatalf("got %d tests, want 2", len(got))
	}

	names := make(map[string]bool)
	for _, tt := range got {
		names[tt.Name] = true
	}
	if !names["fast"] {
		t.Error("missing 'fast' test")
	}
	if !names["untagged"] {
		t.Error("missing 'untagged' test")
	}
}

func TestExtended_FilterTests_ExcludeOnly_ExcludesAll(t *testing.T) {
	tests := []schema.Test{
		{Name: "a", Tags: []string{"skip"}},
		{Name: "b", Tags: []string{"skip", "also-skip"}},
	}

	got := filterTests(tests, nil, []string{"skip"})
	if len(got) != 0 {
		t.Errorf("got %d tests, want 0 (all excluded)", len(got))
	}
}

func TestExtended_FilterTests_IncludeAndExclude(t *testing.T) {
	tests := []schema.Test{
		{Name: "build-fast", Tags: []string{"build", "fast"}},
		{Name: "build-slow", Tags: []string{"build", "slow"}},
		{Name: "test-fast", Tags: []string{"test", "fast"}},
	}

	// Include "build" but exclude "slow"
	got := filterTests(tests, []string{"build"}, []string{"slow"})
	if len(got) != 1 {
		t.Fatalf("got %d tests, want 1", len(got))
	}
	if got[0].Name != "build-fast" {
		t.Errorf("got %q, want build-fast", got[0].Name)
	}
}

// --- runTest with allow_failure ---

func TestExtended_RunTest_AllowFailure_Failed(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:         "allowed-fail",
			Run:          "exit 1",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	tr := r.runTest(cfg.Tests[0], RunOptions{})
	if tr.Passed {
		t.Error("Passed should be false (test failed)")
	}
	if !tr.AllowedFailure {
		t.Error("AllowedFailure should be true")
	}
}

func TestExtended_RunTest_AllowFailure_Passed(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:         "allowed-pass",
			Run:          "echo ok",
			Expect:       schema.Expect{ExitCode: intPtr(0)},
			AllowFailure: true,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	tr := r.runTest(cfg.Tests[0], RunOptions{})
	if !tr.Passed {
		t.Error("Passed should be true (test succeeded)")
	}
	if tr.AllowedFailure {
		t.Error("AllowedFailure should be false when test passes")
	}
}

func TestExtended_RunTest_AllowFailure_StdoutMismatch(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:         "wrong-output",
			Run:          "echo wrong",
			Expect:       schema.Expect{StdoutContains: "expected"},
			AllowFailure: true,
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	tr := r.runTest(cfg.Tests[0], RunOptions{})
	if tr.Passed {
		t.Error("Passed should be false (stdout mismatch)")
	}
	if !tr.AllowedFailure {
		t.Error("AllowedFailure should be true")
	}
}

func TestExtended_RunTest_NoAllowFailure_FailsNormally(t *testing.T) {
	cfg := newConfig([]schema.Test{
		{
			Name:   "normal-fail",
			Run:    "exit 1",
			Expect: schema.Expect{ExitCode: intPtr(0)},
		},
	})
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}

	tr := r.runTest(cfg.Tests[0], RunOptions{})
	if tr.Passed {
		t.Error("Passed should be false")
	}
	if tr.AllowedFailure {
		t.Error("AllowedFailure should be false (no allow_failure flag)")
	}
}

// --- shouldSkip with FileMissing: absolute vs relative paths ---

func TestExtended_ShouldSkip_FileMissing_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "absolute.txt")
	os.WriteFile(absFile, []byte("data"), 0644)

	// Existing absolute path → don't skip
	si := &schema.SkipIf{FileMissing: absFile}
	if shouldSkip(si, dir) {
		t.Error("absolute path exists → should not skip")
	}

	// Missing absolute path → skip
	missing := filepath.Join(dir, "nope.txt")
	si = &schema.SkipIf{FileMissing: missing}
	if !shouldSkip(si, dir) {
		t.Error("absolute path missing → should skip")
	}
}

func TestExtended_ShouldSkip_FileMissing_RelativePath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "relative.txt"), []byte("data"), 0644)

	// Existing relative path → don't skip
	si := &schema.SkipIf{FileMissing: "relative.txt"}
	if shouldSkip(si, dir) {
		t.Error("relative file exists in configDir → should not skip")
	}

	// Missing relative path → skip
	si = &schema.SkipIf{FileMissing: "gone.txt"}
	if !shouldSkip(si, dir) {
		t.Error("relative file missing in configDir → should skip")
	}
}

func TestExtended_ShouldSkip_FileMissing_AbsoluteIgnoresConfigDir(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// File exists in dir1 but configDir is dir2 — absolute path should still find it
	absFile := filepath.Join(dir1, "target.txt")
	os.WriteFile(absFile, []byte("data"), 0644)

	si := &schema.SkipIf{FileMissing: absFile}
	if shouldSkip(si, dir2) {
		t.Error("absolute path should resolve regardless of configDir")
	}
}

func TestExtended_ShouldSkip_FileMissing_RelativeUsesConfigDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("data"), 0644)

	// Relative path "nested.txt" should NOT be found when configDir is the parent
	si := &schema.SkipIf{FileMissing: "nested.txt"}
	if !shouldSkip(si, dir) {
		t.Error("relative file is in subdir, not configDir → should skip")
	}

	// But it should be found when configDir is the subdirectory
	if shouldSkip(si, subDir) {
		t.Error("relative file exists in configDir (subDir) → should not skip")
	}
}
