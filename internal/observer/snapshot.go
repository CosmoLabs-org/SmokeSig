package observer

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

var skipDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"__pycache__": true,
}

// TakeSnapshot walks dir recursively and returns a map of relative path to FileSnapshot.
// It skips .git, node_modules, and __pycache__ directories.
func TakeSnapshot(dir string) (map[string]FileSnapshot, error) {
	result := make(map[string]FileSnapshot)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		result[rel] = FileSnapshot{
			Path: rel,
			Size: info.Size(),
			Hash: hash,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// hashFile reads up to 8KB of the file and returns the hex-encoded first 8 bytes of its SHA-256 hash.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyN(h, f, 8192); err != nil && err != io.EOF {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8]), nil
}

// DiffSnapshots returns files present in after but not in before, or present in both
// but with different Hash values. Returns a slice of FileSnapshot from after.
func DiffSnapshots(before, after map[string]FileSnapshot) []FileSnapshot {
	var diff []FileSnapshot
	for path, af := range after {
		bf, exists := before[path]
		if !exists || bf.Hash != af.Hash {
			diff = append(diff, af)
		}
	}
	return diff
}
