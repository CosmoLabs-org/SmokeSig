package monorepo

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDiscover_NestedThreePlusLevels(t *testing.T) {
	root := t.TempDir()
	// Level 3: root/a/b/c/.smokesig.yaml
	os.MkdirAll(filepath.Join(root, "a", "b", "c"), 0755)
	os.WriteFile(filepath.Join(root, "a", "b", "c", ".smokesig.yaml"), []byte("version: 1\n"), 0644)
	// Level 4: root/x/y/z/w/.smokesig.yaml
	os.MkdirAll(filepath.Join(root, "x", "y", "z", "w"), 0755)
	os.WriteFile(filepath.Join(root, "x", "y", "z", "w", ".smokesig.yaml"), []byte("version: 1\n"), 0644)
	// Level 5: root/p/q/r/s/t/.smokesig.yaml
	os.MkdirAll(filepath.Join(root, "p", "q", "r", "s", "t"), 0755)
	os.WriteFile(filepath.Join(root, "p", "q", "r", "s", "t", ".smokesig.yaml"), []byte("version: 1\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 3 {
		t.Fatalf("expected 3 deep configs, got %d", len(configs))
	}
	names := []string{filepath.Base(configs[0].Dir), filepath.Base(configs[1].Dir), filepath.Base(configs[2].Dir)}
	sort.Strings(names)
	if names[0] != "c" || names[1] != "t" || names[2] != "w" {
		t.Errorf("expected dirs c,t,w got %v", names)
	}
}

func TestDiscover_HiddenDirsExcluded(t *testing.T) {
	root := t.TempDir()
	// .git is in defaultSkipDirs
	os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0755)
	os.WriteFile(filepath.Join(root, ".git", ".smokesig.yaml"), []byte("version: 1\n"), 0644)
	// .next is in defaultSkipDirs
	os.MkdirAll(filepath.Join(root, ".next", "static"), 0755)
	os.WriteFile(filepath.Join(root, ".next", ".smokesig.yaml"), []byte("version: 1\n"), 0644)
	// .cache is in defaultSkipDirs
	os.MkdirAll(filepath.Join(root, ".cache"), 0755)
	os.WriteFile(filepath.Join(root, ".cache", ".smokesig.yaml"), []byte("version: 1\n"), 0644)
	// Non-hidden dir with config should still be found
	os.MkdirAll(filepath.Join(root, "api"), 0755)
	os.WriteFile(filepath.Join(root, "api", ".smokesig.yaml"), []byte("version: 1\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || filepath.Base(configs[0].Dir) != "api" {
		t.Errorf("expected 1 config (api only), hidden dirs excluded, got %v", configs)
	}
}

func TestDiscover_EmptyDirsSkippedGracefully(t *testing.T) {
	root := t.TempDir()
	// Several empty directories
	os.MkdirAll(filepath.Join(root, "empty1"), 0755)
	os.MkdirAll(filepath.Join(root, "empty2", "subempty"), 0755)
	os.MkdirAll(filepath.Join(root, "empty3"), 0755)
	// One with a config
	os.MkdirAll(filepath.Join(root, "api"), 0755)
	os.WriteFile(filepath.Join(root, "api", ".smokesig.yaml"), []byte("version: 1\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || filepath.Base(configs[0].Dir) != "api" {
		t.Errorf("expected 1 config, empty dirs skipped, got %v", configs)
	}
}

func TestDiscover_ZeroConfigsReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	// Just an empty root, no subdirs at all

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs, got %d", len(configs))
	}
}

func TestDiscover_DuplicateProjectNames(t *testing.T) {
	root := t.TempDir()
	// Two dirs named "api" at different paths
	os.MkdirAll(filepath.Join(root, "api"), 0755)
	os.WriteFile(filepath.Join(root, "api", ".smokesig.yaml"), []byte("version: 1\n"), 0644)
	os.MkdirAll(filepath.Join(root, "services", "api"), 0755)
	os.WriteFile(filepath.Join(root, "services", "api", ".smokesig.yaml"), []byte("version: 1\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs with duplicate names, got %d", len(configs))
	}
	// Both should have Project == "api"
	for _, c := range configs {
		if c.Project != "api" {
			t.Errorf("expected Project 'api', got %q", c.Project)
		}
	}
	// Paths must differ
	if configs[0].Path == configs[1].Path {
		t.Error("expected different paths for duplicate project names")
	}
}
