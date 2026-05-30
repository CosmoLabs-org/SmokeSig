package baseline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Nonexistent(t *testing.T) {
	f, err := Load("/nonexistent/baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Error("expected empty file for missing path")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	f := File{
		"test_a": {DurationMs: 100, Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		"test_b": {DurationMs: 200, Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded))
	}
	if loaded["test_a"].DurationMs != 100 {
		t.Errorf("test_a duration = %d, want 100", loaded["test_a"].DurationMs)
	}
}

func TestCompare_NoBaseline(t *testing.T) {
	f := File{}
	current := map[string]int64{"test_a": 100, "test_b": 200}
	results := f.Compare(current, 50)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.NewTest {
			t.Errorf("%s: expected NewTest=true", r.Name)
		}
	}
}

func TestCompare_Regressions(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := File{
		"fast_test":  {DurationMs: 100, Timestamp: ts},
		"slow_test":  {DurationMs: 500, Timestamp: ts},
		"stable_test": {DurationMs: 300, Timestamp: ts},
	}

	current := map[string]int64{
		"fast_test":   180,  // +80% → regressed (>50% threshold)
		"slow_test":   600,  // +20% → not regressed
		"stable_test": 300,  // 0% → not regressed
		"new_test":    400,  // new
	}

	results := f.Compare(current, 50)

	regressed := 0
	newTests := 0
	for _, r := range results {
		if r.Regressed {
			regressed++
			if r.Name != "fast_test" {
				t.Errorf("unexpected regression: %s (%.1f%%)", r.Name, r.DeltaPct)
			}
			if r.DeltaMs != 80 {
				t.Errorf("delta = %d, want 80", r.DeltaMs)
			}
		}
		if r.NewTest {
			newTests++
			if r.Name != "new_test" {
				t.Errorf("unexpected new test: %s", r.Name)
			}
		}
	}
	if regressed != 1 {
		t.Errorf("regressions = %d, want 1", regressed)
	}
	if newTests != 1 {
		t.Errorf("new tests = %d, want 1", newTests)
	}
}

func TestCompare_ZeroBaseline(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := File{
		"zero_test": {DurationMs: 0, Timestamp: ts},
	}
	current := map[string]int64{"zero_test": 100}

	results := f.Compare(current, 50)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	// Zero baseline: delta_pct should be 0 (avoid div-by-zero), no regression
	if results[0].Regressed {
		t.Error("zero baseline should not flag regression")
	}
}

func TestUpdate(t *testing.T) {
	f := File{
		"old_test": {DurationMs: 100, Timestamp: time.Now()},
	}
	current := map[string]int64{
		"old_test": 150,
		"new_test": 200,
	}
	f.Update(current)

	if f["old_test"].DurationMs != 150 {
		t.Errorf("old_test = %d, want 150", f["old_test"].DurationMs)
	}
	if f["new_test"].DurationMs != 200 {
		t.Errorf("new_test = %d, want 200", f["new_test"].DurationMs)
	}
}

func TestSave_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Error("expected empty file for empty content")
	}
}

func TestLoad_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for corrupted JSON, got nil")
	}
}

func TestLoad_ReadError(t *testing.T) {
	// Use a path where the parent exists as a regular file — os.ReadFile will
	// fail with a non-IsNotExist error (it's a directory entry conflict).
	dir := t.TempDir()
	// Create a regular file, then try to read a "child" of it.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// blocker/baseline.json cannot be read because blocker is a file, not a dir.
	path := filepath.Join(blocker, "baseline.json")
	_, err := Load(path)
	if err == nil {
		t.Error("expected error when parent path is a file, got nil")
	}
}

func TestSave_WriteError(t *testing.T) {
	// Write to a path whose parent is a regular file — os.WriteFile will fail.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "baseline.json")
	f := File{
		"test_a": {DurationMs: 100, Timestamp: time.Now()},
	}
	err := f.Save(path)
	if err == nil {
		t.Error("expected error saving to invalid path, got nil")
	}
}

func TestSave_EmptyMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	f := File{}
	if err := f.Save(path); err != nil {
		t.Fatalf("unexpected error saving empty baseline: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected empty map after save/load, got %d entries", len(loaded))
	}
}
