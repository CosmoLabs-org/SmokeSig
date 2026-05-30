package reporter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingReporter captures every method call for verification.
type recordingReporter struct {
	mu            sync.Mutex
	prereqStarts  []string
	prereqResults []PrereqResultData
	testStarts    []string
	testResults   []TestResultData
	summaries     []SuiteResultData
}

func (r *recordingReporter) PrereqStart(name string) {
	r.mu.Lock()
	r.prereqStarts = append(r.prereqStarts, name)
	r.mu.Unlock()
}

func (r *recordingReporter) PrereqResult(d PrereqResultData) {
	r.mu.Lock()
	r.prereqResults = append(r.prereqResults, d)
	r.mu.Unlock()
}

func (r *recordingReporter) TestStart(name string) {
	r.mu.Lock()
	r.testStarts = append(r.testStarts, name)
	r.mu.Unlock()
}

func (r *recordingReporter) TestResult(d TestResultData) {
	r.mu.Lock()
	r.testResults = append(r.testResults, d)
	r.mu.Unlock()
}

func (r *recordingReporter) Summary(d SuiteResultData) {
	r.mu.Lock()
	r.summaries = append(r.summaries, d)
	r.mu.Unlock()
}

func TestChain_SingleFormat_NoFilesCreated(t *testing.T) {
	rep, closers, err := Chain("terminal", os.Stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers, got %d", len(closers))
	}
}

func TestChain_MultipleFormats_CreatesFiles(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	rep, closers, err := Chain("terminal,json", os.Stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
	if len(closers) != 1 {
		t.Fatalf("expected 1 closer, got %d", len(closers))
	}
	if _, err := os.Stat(filepath.Join(tmp, "smoke-results.json")); err != nil {
		t.Fatalf("expected smoke-results.json to exist: %v", err)
	}
	for _, c := range closers {
		c.Close()
	}
}

func TestChain_DeduplicatesFormats(t *testing.T) {
	var buf bytes.Buffer
	rep, closers, err := Chain("json,json,json", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers (single format after dedup), got %d", len(closers))
	}
	_ = rep
}

func TestChain_UnknownFormat_ReturnsError(t *testing.T) {
	_, _, err := Chain("xml", os.Stdout)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Fatalf("error should mention unknown format: %v", err)
	}
}

func TestChain_EmptyFormat_ReturnsError(t *testing.T) {
	_, _, err := Chain("", os.Stdout)
	if err == nil {
		t.Fatal("expected error for empty format")
	}
}

func TestChain_CommasOnly_ReturnsError(t *testing.T) {
	_, _, err := Chain(",,,", os.Stdout)
	if err == nil {
		t.Fatal("expected error for commas-only format")
	}
}

func TestChain_CaseInsensitive(t *testing.T) {
	rep, closers, err := Chain("JSON", os.Stdout)
	if err != nil {
		t.Fatalf("case-insensitive match should work: %v", err)
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers for single format, got %d", len(closers))
	}
	_ = rep
}

func TestChain_WhitespaceTrimmed(t *testing.T) {
	var buf bytes.Buffer
	rep, closers, err := Chain(" json , terminal ", &buf)
	if err != nil {
		t.Fatalf("whitespace trimming should work: %v", err)
	}
	if len(closers) != 1 {
		t.Fatalf("expected 1 closer (terminal to file), got %d", len(closers))
	}
	_ = rep
	for _, c := range closers {
		c.Close()
	}
}

func TestChain_TrailingComma(t *testing.T) {
	rep, closers, err := Chain("json,", os.Stdout)
	if err != nil {
		t.Fatalf("trailing comma should be handled: %v", err)
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers, got %d", len(closers))
	}
	_ = rep
}

func TestChain_FileNaming(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	tests := []struct {
		format   string
		filename string
	}{
		{"json", "smoke-results.json"},
		{"junit", "smoke-junit.xml"},
		{"prometheus", "smoke-metrics.prom"},
		{"tap", "smoke-tap.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			_, closers, err := Chain("terminal,"+tc.format, os.Stdout)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			path := filepath.Join(tmp, tc.filename)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected %s to exist: %v", tc.filename, err)
			}
			for _, c := range closers {
				c.Close()
			}
		})
	}
}

func TestChain_ThreeFormats_CreatesAllFiles(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	rep, closers, err := Chain("terminal,json,prometheus", os.Stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
	if len(closers) != 2 {
		t.Fatalf("expected 2 closers (json+prometheus), got %d", len(closers))
	}
	for _, name := range []string{"smoke-results.json", "smoke-metrics.prom"} {
		if _, err := os.Stat(filepath.Join(tmp, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
	for _, c := range closers {
		c.Close()
	}
}

func TestMultiReporter_FansOutToAllReporters(t *testing.T) {
	r1 := &recordingReporter{}
	r2 := &recordingReporter{}
	r3 := &recordingReporter{}

	multi := NewMultiReporter(r1, r2, r3)

	multi.PrereqStart("docker")
	multi.PrereqResult(PrereqResultData{Name: "docker", Passed: true, Output: "running"})
	multi.TestStart("build")
	multi.TestResult(TestResultData{
		Name:     "build",
		Passed:   true,
		Duration: 100 * time.Millisecond,
	})
	multi.Summary(SuiteResultData{
		Project: "test-project",
		Total:   1,
		Passed:  1,
	})

	for i, rec := range []*recordingReporter{r1, r2, r3} {
		if len(rec.prereqStarts) != 1 || rec.prereqStarts[0] != "docker" {
			t.Errorf("reporter %d: prereqStarts = %v", i, rec.prereqStarts)
		}
		if len(rec.prereqResults) != 1 || rec.prereqResults[0].Name != "docker" {
			t.Errorf("reporter %d: prereqResults = %v", i, rec.prereqResults)
		}
		if len(rec.testStarts) != 1 || rec.testStarts[0] != "build" {
			t.Errorf("reporter %d: testStarts = %v", i, rec.testStarts)
		}
		if len(rec.testResults) != 1 || rec.testResults[0].Name != "build" {
			t.Errorf("reporter %d: testResults = %v", i, rec.testResults)
		}
		if rec.testResults[0].Duration != 100*time.Millisecond {
			t.Errorf("reporter %d: duration = %v", i, rec.testResults[0].Duration)
		}
		if len(rec.summaries) != 1 || rec.summaries[0].Project != "test-project" {
			t.Errorf("reporter %d: summaries = %v", i, rec.summaries)
		}
	}
}

func TestMultiReporter_SameEventData(t *testing.T) {
	r1 := &recordingReporter{}
	r2 := &recordingReporter{}

	multi := NewMultiReporter(r1, r2)

	td := TestResultData{
		Name:     "identical-check",
		Passed:   false,
		Duration: 250 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "stdout_contains", Expected: "hello", Actual: "world", Passed: false},
		},
	}
	multi.TestResult(td)

	if len(r1.testResults) != 1 || len(r2.testResults) != 1 {
		t.Fatal("both reporters should have exactly 1 test result")
	}
	if r1.testResults[0].Name != r2.testResults[0].Name {
		t.Errorf("name mismatch: %q vs %q", r1.testResults[0].Name, r2.testResults[0].Name)
	}
	if r1.testResults[0].Passed != r2.testResults[0].Passed {
		t.Errorf("passed mismatch: %v vs %v", r1.testResults[0].Passed, r2.testResults[0].Passed)
	}
	if r1.testResults[0].Duration != r2.testResults[0].Duration {
		t.Errorf("duration mismatch: %v vs %v", r1.testResults[0].Duration, r2.testResults[0].Duration)
	}
	if len(r1.testResults[0].Assertions) != len(r2.testResults[0].Assertions) {
		t.Errorf("assertion count mismatch: %d vs %d", len(r1.testResults[0].Assertions), len(r2.testResults[0].Assertions))
	}
}

func TestMultiReporter_EmptyList_NoPanics(t *testing.T) {
	multi := NewMultiReporter()

	// None of these should panic
	multi.PrereqStart("noop")
	multi.PrereqResult(PrereqResultData{Name: "noop", Passed: true})
	multi.TestStart("noop")
	multi.TestResult(TestResultData{Name: "noop", Passed: true})
	multi.Summary(SuiteResultData{Project: "empty", Total: 0})
}

func TestChainWithVerbosity_UnknownFormat_ReturnsError(t *testing.T) {
	_, _, err := ChainWithVerbosity("unknown-format", os.Stdout, VerbosityNormal)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown-format") {
		t.Errorf("error should mention unknown format: %v", err)
	}
}

func TestChainWithVerbosity_VerboseTerminal(t *testing.T) {
	var buf bytes.Buffer
	rep, closers, err := ChainWithVerbosity("terminal", &buf, VerbosityVerbose)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers, got %d", len(closers))
	}
	// Verify verbose mode propagated — verbose shows assertions for passing tests
	rep.TestResult(TestResultData{
		Name:   "test",
		Passed: true,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "0", Passed: true},
		},
		Duration: 10 * time.Millisecond,
	})
	if !strings.Contains(buf.String(), "exit_code") {
		t.Errorf("verbose terminal should show assertion details for passing tests, got %q", buf.String())
	}
}

func TestChainWithVerbosity_QuietTerminal(t *testing.T) {
	var buf bytes.Buffer
	rep, closers, err := ChainWithVerbosity("terminal", &buf, VerbosityQuiet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers, got %d", len(closers))
	}
	rep.TestResult(TestResultData{
		Name:     "quiet-pass",
		Passed:   true,
		Duration: 10 * time.Millisecond,
	})
	if buf.String() != "" {
		t.Errorf("quiet terminal should suppress passing tests, got %q", buf.String())
	}
}

func TestChainWithVerbosity_EmptyFormat_ReturnsError(t *testing.T) {
	_, _, err := ChainWithVerbosity("", os.Stdout, VerbosityNormal)
	if err == nil {
		t.Fatal("expected error for empty format")
	}
}

func TestChainWithVerbosity_FileCreateError_CleansUpAndReturnsError(t *testing.T) {
	// Use a read-only temp dir so os.Create fails for the second format
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	// Make the dir read-only so file creation fails
	if err := os.Chmod(tmp, 0555); err != nil {
		t.Skip("cannot make dir read-only:", err)
	}
	defer os.Chmod(tmp, 0755)

	// "terminal,json" — terminal goes to stdout (ok), json tries to create a file (fails)
	_, closers, err := ChainWithVerbosity("terminal,json", os.Stdout, VerbosityNormal)
	if err == nil {
		// Close any opened files before failing
		for _, c := range closers {
			c.Close()
		}
		t.Fatal("expected error when file creation fails")
	}
	if len(closers) != 0 {
		t.Errorf("expected 0 closers after error cleanup, got %d", len(closers))
	}
}
