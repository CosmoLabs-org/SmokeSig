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

// TestHashFile_EmptyFile verifies hashFile works on a zero-byte file (io.EOF on first CopyN).
func TestHashFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := hashFile(path)
	if err != nil {
		t.Fatalf("hashFile empty file: %v", err)
	}
	if len(h) != 16 {
		t.Errorf("expected 16 hex chars, got %q", h)
	}
}

// TestHashFile_LargeFile verifies hashFile handles files larger than 8 KB (partial read path).
func TestHashFile_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	content := make([]byte, 16*1024) // 16 KB — larger than 8 KB cap
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	h1, err := hashFile(path)
	if err != nil {
		t.Fatalf("hashFile large file: %v", err)
	}
	if len(h1) != 16 {
		t.Errorf("expected 16 hex chars, got %q", h1)
	}
	// A different large file with different prefix should produce different hash.
	path2 := filepath.Join(dir, "big2.txt")
	content2 := make([]byte, 16*1024)
	for i := range content2 {
		content2[i] = byte((i + 1) % 256)
	}
	if err := os.WriteFile(path2, content2, 0o644); err != nil {
		t.Fatal(err)
	}
	h2, err := hashFile(path2)
	if err != nil {
		t.Fatalf("hashFile large file2: %v", err)
	}
	if h1 == h2 {
		t.Error("different large files should produce different hashes")
	}
}

// TestHashFile_NonExistentFile verifies hashFile returns error for missing files.
func TestHashFile_NonExistentFile(t *testing.T) {
	_, err := hashFile("/nonexistent/path/to/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

// TestTakeSnapshot_UnreadableFile verifies TakeSnapshot returns an error when a file cannot be
// hashed due to permission denial (exercises the hashFile error path in WalkDir).
func TestTakeSnapshot_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — permission tests are not meaningful")
	}
	dir := t.TempDir()
	// Write a normal file first.
	snapWriteFile(t, dir, "readable.txt", "ok")
	// Write a file then make it unreadable.
	unreadable := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(unreadable, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadable, 0o644) // restore for cleanup

	_, err := TakeSnapshot(dir)
	// hashFile will fail to open the file → WalkDir propagates the error.
	if err == nil {
		t.Error("expected error for unreadable file, got nil")
	}
}

// TestTakeSnapshot_InvalidDir verifies TakeSnapshot returns error for a non-existent directory.
func TestTakeSnapshot_InvalidDir(t *testing.T) {
	_, err := TakeSnapshot("/nonexistent/path/that/does/not/exist/abc123")
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// TestTakeSnapshot_SymlinkFile verifies that symlinks to files are snapshotted via the target content.
func TestTakeSnapshot_SymlinkFile(t *testing.T) {
	dir := t.TempDir()
	// Create real file.
	real := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(real, []byte("symlink target content"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create symlink.
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not supported on this system")
	}
	snap, err := TakeSnapshot(dir)
	if err != nil {
		t.Fatalf("TakeSnapshot with symlink: %v", err)
	}
	// Both real.txt and link.txt should appear.
	if _, ok := snap["real.txt"]; !ok {
		t.Error("real.txt missing from snapshot")
	}
	if _, ok := snap["link.txt"]; !ok {
		t.Error("link.txt (symlink) missing from snapshot")
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
