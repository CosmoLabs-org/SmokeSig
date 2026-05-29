package monorepo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscover_LegacySmokeYaml verifies that .smoke.yaml (legacy) is discovered
// when .smokesig.yaml is not present. This covers the fallback branch in Discover.
func TestDiscover_LegacySmokeYaml(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "service")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create only the legacy file (no .smokesig.yaml)
	if err := os.WriteFile(filepath.Join(subdir, ".smoke.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config from .smoke.yaml, got %d", len(configs))
	}
	if filepath.Base(configs[0].Path) != ".smoke.yaml" {
		t.Errorf("expected .smoke.yaml, got %q", filepath.Base(configs[0].Path))
	}
	if configs[0].Project != "service" {
		t.Errorf("expected project 'service', got %q", configs[0].Project)
	}
}

// TestDiscover_SmokesigPreferredOverLegacy verifies .smokesig.yaml takes precedence
// over .smoke.yaml when both exist in the same directory.
func TestDiscover_SmokesigPreferredOverLegacy(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "api")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create both files
	if err := os.WriteFile(filepath.Join(subdir, ".smokesig.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, ".smoke.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	// .smokesig.yaml should be preferred
	if filepath.Base(configs[0].Path) != ".smokesig.yaml" {
		t.Errorf("expected .smokesig.yaml to take precedence, got %q", filepath.Base(configs[0].Path))
	}
}

// TestDiscover_ExcludeSubdir verifies that user-excluded directories are skipped.
func TestDiscover_ExcludeSubdir(t *testing.T) {
	root := t.TempDir()

	// Create excluded dir with config
	excluded := filepath.Join(root, "internal")
	if err := os.MkdirAll(excluded, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(excluded, ".smokesig.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create non-excluded dir with config
	included := filepath.Join(root, "api")
	if err := os.MkdirAll(included, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(included, ".smokesig.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := Discover(root, []string{"internal"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (excluded dir skipped), got %d: %v", len(configs), configs)
	}
	if configs[0].Project != "api" {
		t.Errorf("expected project 'api', got %q", configs[0].Project)
	}
}

// TestDiscover_DefaultSkipDirs verifies that all default skip dirs are excluded.
func TestDiscover_DefaultSkipDirsAll(t *testing.T) {
	root := t.TempDir()

	// Create all defaultSkipDirs with configs inside
	skipNames := []string{"node_modules", "vendor", "__pycache__", "dist", "build", "target", ".next", ".cache"}
	for _, name := range skipNames {
		d := filepath.Join(root, name)
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, ".smokesig.yaml"), []byte("version: 1\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Add one legitimate subdir
	legit := filepath.Join(root, "src")
	if err := os.MkdirAll(legit, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legit, ".smokesig.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (all skip dirs excluded), got %d: %v", len(configs), configs)
	}
	if configs[0].Project != "src" {
		t.Errorf("expected project 'src', got %q", configs[0].Project)
	}
}

// TestDiscover_RootConfigNotIncluded verifies that the root dir's own config is NOT returned.
func TestDiscover_RootConfigNotIncluded(t *testing.T) {
	root := t.TempDir()

	// Create a config at the root level itself
	if err := os.WriteFile(filepath.Join(root, ".smokesig.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdir with its own config
	subdir := filepath.Join(root, "frontend")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, ".smokesig.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// Only the subdir config should be returned, not the root
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (root excluded), got %d", len(configs))
	}
	if configs[0].Project != "frontend" {
		t.Errorf("expected project 'frontend', got %q", configs[0].Project)
	}
}
