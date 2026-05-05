package monorepo

import (
	"os"
	"path/filepath"
)

// SubConfig represents a discovered .smokesig.yaml file.
type SubConfig struct {
	Path    string // absolute path to .smokesig.yaml
	Dir     string // directory containing it
	Project string // directory name as fallback project name
}

// defaultSkipDirs are always excluded from discovery.
var defaultSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"__pycache__": true, "dist": true, "build": true,
	"target": true, ".next": true, ".cache": true,
}

// Discover walks root and finds all .smokesig.yaml (and legacy .smoke.yaml) files in subdirectories.
// Returns discovered configs sorted by path. Does not include root's own config.
func Discover(root string, exclude []string) ([]SubConfig, error) {
	excludeSet := make(map[string]bool)
	for _, d := range exclude {
		excludeSet[filepath.Clean(d)] = true
	}

	var configs []SubConfig
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors, keep walking
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()

		// Skip default dirs
		if defaultSkipDirs[name] {
			return filepath.SkipDir
		}
		// Skip user-excluded dirs (match relative path)
		rel, _ := filepath.Rel(root, path)
		if excludeSet[filepath.Clean(rel)] {
			return filepath.SkipDir
		}
		// Skip root dir itself
		if path == root {
			return nil
		}

		configPath := filepath.Join(path, ".smoke.yaml")
		if _, err := os.Stat(configPath); err == nil {
			configs = append(configs, SubConfig{
				Path:    configPath,
				Dir:     path,
				Project: name,
			})
		}
		return nil
	})
	return configs, err
}
