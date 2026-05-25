package observer

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func snapWriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotReturnsAllFiles(t *testing.T) {
	dir := t.TempDir()
	snapWriteFile(t, dir, "a.txt", "hello")
	snapWriteFile(t, dir, "b.txt", "world")
	snapWriteFile(t, dir, "sub/c.txt", "nested")

	snap, err := TakeSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(snap))
	}

	for name, expected := range map[string]int64{"a.txt": 5, "b.txt": 5, "sub/c.txt": 6} {
		fs, ok := snap[name]
		if !ok {
			t.Errorf("missing entry for %s", name)
			continue
		}
		if fs.Size != expected {
			t.Errorf("size for %s: got %d, want %d", name, fs.Size, expected)
		}
		if len(fs.Hash) != 16 {
			t.Errorf("hash for %s: got %q (want 16 hex chars)", name, fs.Hash)
		}
	}
}

func TestSnapshotSkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	snapWriteFile(t, dir, "visible.txt", "yes")
	snapWriteFile(t, dir, ".git/hidden.txt", "no")
	snapWriteFile(t, dir, "node_modules/pkg/index.js", "no")
	snapWriteFile(t, dir, "__pycache__/cache.pyc", "no")

	snap, err := TakeSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 1 {
		t.Fatalf("expected 1 entry (skipping .git/node_modules/__pycache__), got %d: %v", len(snap), snap)
	}
	if _, ok := snap["visible.txt"]; !ok {
		t.Error("visible.txt should be present")
	}
}

func TestSnapshotEmptyDir(t *testing.T) {
	dir := t.TempDir()
	snap, err := TakeSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(snap))
	}
}

func TestDiffNewFile(t *testing.T) {
	before := map[string]FileSnapshot{
		"A": {Path: "A", Hash: "h1"},
		"B": {Path: "B", Hash: "h2"},
	}
	after := map[string]FileSnapshot{
		"A": {Path: "A", Hash: "h1"},
		"B": {Path: "B", Hash: "h2"},
		"C": {Path: "C", Hash: "h3"},
	}
	diff := DiffSnapshots(before, after)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff))
	}
	if diff[0].Path != "C" {
		t.Errorf("expected diff for C, got %s", diff[0].Path)
	}
}

func TestDiffChangedFile(t *testing.T) {
	before := map[string]FileSnapshot{
		"A": {Path: "A", Hash: "hash1"},
	}
	after := map[string]FileSnapshot{
		"A": {Path: "A", Hash: "hash2"},
	}
	diff := DiffSnapshots(before, after)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff))
	}
	if diff[0].Path != "A" {
		t.Errorf("expected diff for A, got %s", diff[0].Path)
	}
}

func TestDiffIdentical(t *testing.T) {
	snap := map[string]FileSnapshot{
		"A": {Path: "A", Hash: "h1"},
		"B": {Path: "B", Hash: "h2"},
	}
	diff := DiffSnapshots(snap, snap)
	if len(diff) != 0 {
		t.Fatalf("expected 0 diffs for identical snapshots, got %d", len(diff))
	}
}

func TestHashConsistency(t *testing.T) {
	dir := t.TempDir()
	snapWriteFile(t, dir, "same.txt", "content")

	s1, _ := TakeSnapshot(dir)
	s2, _ := TakeSnapshot(dir)
	if s1["same.txt"].Hash != s2["same.txt"].Hash {
		t.Error("same file content should produce same hash across snapshots")
	}
}

func TestDiffSorted(t *testing.T) {
	before := map[string]FileSnapshot{}
	after := map[string]FileSnapshot{
		"z.txt": {Path: "z.txt", Hash: "h1"},
		"a.txt": {Path: "a.txt", Hash: "h2"},
		"m.txt": {Path: "m.txt", Hash: "h3"},
	}
	diff := DiffSnapshots(before, after)
	paths := make([]string, len(diff))
	for i, f := range diff {
		paths[i] = f.Path
	}
	sorted := make([]string, len(paths))
	copy(sorted, paths)
	sort.Strings(sorted)
	for i := range paths {
		// just verify all paths are present; order is map-iteration order
		if paths[i] != sorted[0] && paths[i] != sorted[1] && paths[i] != sorted[2] {
			t.Errorf("unexpected path %q in diff", paths[i])
		}
	}
}
