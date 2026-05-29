package observer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Observe — non-quiet mode (passes through to real stdout/stderr)
// ---------------------------------------------------------------------------

// TestObserve_NonQuietMode exercises the !opts.Quiet branch in Observe which
// sets up MultiWriter to os.Stdout/Stderr.
func TestObserve_NonQuietMode(t *testing.T) {
	obs, err := Observe(context.Background(), ObserveOptions{
		Command: "echo nonquiet",
		Quiet:   false, // triggers the MultiWriter branch
	})
	if err != nil {
		t.Fatalf("Observe non-quiet: %v", err)
	}
	if obs.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", obs.ExitCode)
	}
	if !strings.Contains(obs.Stdout, "nonquiet") {
		t.Errorf("expected 'nonquiet' in stdout, got %q", obs.Stdout)
	}
}

// ---------------------------------------------------------------------------
// hashFile — open-error path (line 59-61)
// ---------------------------------------------------------------------------

// TestHashFile_OpenError covers the os.Open error branch by passing a path that doesn't exist.
func TestHashFile_OpenError(t *testing.T) {
	_, err := hashFile("/nonexistent/path/to/file/that/cannot/exist.txt")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// TestHashFile_Success covers the happy path for reference.
func TestHashFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world content"), 0644); err != nil {
		t.Fatal(err)
	}
	hash, err := hashFile(path)
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}
	if len(hash) != 16 { // 8 bytes → 16 hex chars
		t.Errorf("expected 16-char hex hash, got %q (len %d)", hash, len(hash))
	}
}

// ---------------------------------------------------------------------------
// TakeSnapshot — walk error path via unreadable file
// ---------------------------------------------------------------------------

// TestTakeSnapshot_FileHashError covers the hashFile error path in TakeSnapshot's
// WalkDir callback. We create a file then make it unreadable (chmod 000).
func TestTakeSnapshot_FileHashError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read any file — chmod 000 won't trigger error")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "unreadable.txt")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	// Remove read permission — hashFile's os.Open will fail.
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0644) // restore for cleanup

	_, err := TakeSnapshot(dir)
	if err == nil {
		t.Error("expected error for unreadable file in TakeSnapshot, got nil")
	}
}

// ---------------------------------------------------------------------------
// ExtractKeyPhrases — 5+ matches truncation path (line 63-65)
// ---------------------------------------------------------------------------

// TestExtractKeyPhrases_TruncatesAt5 covers the early-return when >= 5 keyword lines matched.
func TestExtractKeyPhrases_TruncatesAt5(t *testing.T) {
	// 6 lines all containing keywords — should return exactly 5
	input := strings.Join([]string{
		"server ready to accept connections",
		"listening on port 3000",
		"started background worker",
		"connected to database",
		"running migrations complete",
		"serving requests on :8080", // 6th — should be truncated
	}, "\n")

	result := ExtractKeyPhrases(input)
	if len(result) != 5 {
		t.Errorf("expected 5 phrases (truncated), got %d: %v", len(result), result)
	}
}

// ---------------------------------------------------------------------------
// Generate — HTTP probe path (line 94-109)
// ---------------------------------------------------------------------------

// TestGenerate_WithHTTPProbe covers the HTTPProbes loop in Generate.
func TestGenerate_WithHTTPProbe(t *testing.T) {
	obs := &Observation{
		Command:  "echo hello",
		ExitCode: 0,
		Stdout:   "hello\n",
		HTTPProbes: []HTTPProbeResult{
			{URL: "http://localhost:3000/", Reachable: true, StatusCode: 200},
			{URL: "http://localhost:3001/", Reachable: false}, // not reachable — skipped
		},
	}

	out, err := Generate(obs)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content := string(out)
	if !strings.Contains(content, "localhost:3000") {
		t.Errorf("expected HTTP probe for port 3000 in output, got:\n%s", content)
	}
	if strings.Contains(content, "localhost:3001") {
		t.Errorf("expected unreachable probe to be excluded, but found localhost:3001 in:\n%s", content)
	}
}

// TestGenerate_WithHTTPProbeNoStatusCode covers the h.StatusCode == 0 path (line 100-101 skipped).
func TestGenerate_WithHTTPProbeNoStatusCode(t *testing.T) {
	obs := &Observation{
		Command:  "true",
		ExitCode: 0,
		HTTPProbes: []HTTPProbeResult{
			{URL: "http://localhost:8080/", Reachable: true, StatusCode: 0},
		},
	}
	out, err := Generate(obs)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(string(out), "localhost:8080") {
		t.Errorf("expected probe URL in output, got:\n%s", out)
	}
}

// TestGenerate_WithPorts covers the obs.Ports loop in Generate.
func TestGenerate_WithPorts(t *testing.T) {
	obs := &Observation{
		Command:  "true",
		ExitCode: 0,
		Ports:    []PortBinding{{Port: 5432, Protocol: "tcp", Host: "localhost"}},
	}
	out, err := Generate(obs)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(string(out), "5432") {
		t.Errorf("expected port 5432 in generated config, got:\n%s", out)
	}
}

// TestGenerate_WithNewFiles covers the obs.NewFiles loop in Generate.
func TestGenerate_WithNewFiles(t *testing.T) {
	obs := &Observation{
		Command:  "true",
		ExitCode: 0,
		NewFiles: []FileSnapshot{{Path: "output/result.json", Size: 100}},
	}
	out, err := Generate(obs)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(string(out), "result.json") {
		t.Errorf("expected file assertion in generated config, got:\n%s", out)
	}
}
