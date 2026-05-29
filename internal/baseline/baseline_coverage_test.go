package baseline

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSave_WritesFile verifies Save writes a valid JSON file to disk.
func TestSave_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	f := File{
		"test-a": {DurationMs: 100},
		"test-b": {DurationMs: 250},
	}

	if err := f.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after Save: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty file after Save")
	}
}

// TestSave_ErrorPath verifies Save returns an error when the directory doesn't exist.
func TestSave_ErrorPath(t *testing.T) {
	f := File{"test": {DurationMs: 100}}
	err := f.Save("/nonexistent/path/that/does/not/exist/baseline.json")
	if err == nil {
		t.Error("expected error when writing to nonexistent directory")
	}
}

// TestSave_RoundTrip verifies Save then Load produces identical data.
func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	original := File{
		"alpha": {DurationMs: 42},
		"beta":  {DurationMs: 99},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != len(original) {
		t.Fatalf("expected %d entries, got %d", len(original), len(loaded))
	}
	for k, v := range original {
		got, ok := loaded[k]
		if !ok {
			t.Errorf("missing key %q after round-trip", k)
			continue
		}
		if got.DurationMs != v.DurationMs {
			t.Errorf("key %q: got %d ms, want %d ms", k, got.DurationMs, v.DurationMs)
		}
	}
}

// TestSave_EmptyFileBaseline verifies Save handles an empty File without error.
func TestSave_EmptyFileBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty-baseline.json")

	f := File{}
	if err := f.Save(path); err != nil {
		t.Fatalf("Save empty File: %v", err)
	}

	// Loading empty file should return empty File
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after empty Save: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 entries, got %d", len(loaded))
	}
}
