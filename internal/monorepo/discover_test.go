package monorepo

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func TestDiscover_FindsSubConfigs(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "api"), 0755)
	os.MkdirAll(filepath.Join(root, "worker"), 0755)
	os.WriteFile(filepath.Join(root, "api", ".smokesig.yaml"), []byte("version: 1\nproject: api\ntests: []\n"), 0644)
	os.WriteFile(filepath.Join(root, "worker", ".smokesig.yaml"), []byte("version: 1\nproject: worker\ntests: []\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	names := []string{filepath.Base(configs[0].Dir), filepath.Base(configs[1].Dir)}
	sort.Strings(names)
	if names[0] != "api" || names[1] != "worker" {
		t.Errorf("expected api+worker, got %v", names)
	}
}

func TestDiscover_SkipsIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0755)
	os.WriteFile(filepath.Join(root, "node_modules", "pkg", ".smokesig.yaml"), []byte("version: 1\nproject: pkg\ntests: []\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs (node_modules skipped), got %d", len(configs))
	}
}

func TestDiscover_CustomExclude(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "api"), 0755)
	os.MkdirAll(filepath.Join(root, "internal"), 0755)
	os.WriteFile(filepath.Join(root, "api", ".smokesig.yaml"), []byte("version: 1\nproject: api\ntests: []\n"), 0644)
	os.WriteFile(filepath.Join(root, "internal", ".smokesig.yaml"), []byte("version: 1\nproject: internal\ntests: []\n"), 0644)

	configs, err := Discover(root, []string{"internal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || filepath.Base(configs[0].Dir) != "api" {
		t.Errorf("expected 1 config (api only), got %v", configs)
	}
}

func TestDiscover_DeepNesting(t *testing.T) {
	root := t.TempDir()
	deepDir := filepath.Join(root, "services", "team-a", "api")
	os.MkdirAll(deepDir, 0755)
	os.WriteFile(filepath.Join(deepDir, ".smokesig.yaml"), []byte("version: 1\nproject: deep\ntests: []\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || filepath.Base(configs[0].Dir) != "api" {
		t.Errorf("expected 1 deep config, got %v", configs)
	}
}

func TestDiscover_NoSmokeFiles(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "api"), 0755)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs, got %d", len(configs))
	}
}

func TestDiscover_MultipleNestedDepths(t *testing.T) {
	// Tests configs at multiple nested depths in the same tree
	root := t.TempDir()

	// Depth 1: services/
	level1 := filepath.Join(root, "services")
	os.MkdirAll(level1, 0755)
	os.WriteFile(filepath.Join(level1, ".smokesig.yaml"), []byte("version: 1\nproject: services\ntests: []\n"), 0644)

	// Depth 2: services/api/
	level2 := filepath.Join(root, "services", "api")
	os.MkdirAll(level2, 0755)
	os.WriteFile(filepath.Join(level2, ".smokesig.yaml"), []byte("version: 1\nproject: api\ntests: []\n"), 0644)

	// Depth 3: services/api/v2/
	level3 := filepath.Join(root, "services", "api", "v2")
	os.MkdirAll(level3, 0755)
	os.WriteFile(filepath.Join(level3, ".smokesig.yaml"), []byte("version: 1\nproject: api-v2\ntests: []\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 3 {
		t.Fatalf("expected 3 configs at multiple depths, got %d", len(configs))
	}
	// Verify all three are found by collecting project dirs
	names := make(map[string]bool)
	for _, c := range configs {
		names[filepath.Base(c.Dir)] = true
	}
	for _, expected := range []string{"services", "api", "v2"} {
		if !names[expected] {
			t.Errorf("expected config in dir %q, not found. Got: %v", expected, names)
		}
	}
}

func TestDiscover_MultipleExcludedPaths(t *testing.T) {
	// Tests that multiple user-specified exclusions are all respected
	root := t.TempDir()

	for _, dir := range []string{"keep", "skip-a", "skip-b"} {
		os.MkdirAll(filepath.Join(root, dir), 0755)
		os.WriteFile(filepath.Join(root, dir, ".smokesig.yaml"), []byte("version: 1\nproject: "+dir+"\ntests: []\n"), 0644)
	}

	configs, err := Discover(root, []string{"skip-a", "skip-b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (only 'keep'), got %d: %v", len(configs), configs)
	}
	if filepath.Base(configs[0].Dir) != "keep" {
		t.Errorf("expected 'keep', got %q", filepath.Base(configs[0].Dir))
	}
}

func TestDiscover_SymlinkedConfig(t *testing.T) {
	// Tests that symlinked .smokesig.yaml files are discovered
	root := t.TempDir()

	// Real config in a separate location
	realDir := t.TempDir()
	realConfig := filepath.Join(realDir, ".smokesig.yaml")
	os.WriteFile(realConfig, []byte("version: 1\nproject: symlinked\ntests: []\n"), 0644)

	// Directory in root that has a symlink to the real config
	linkDir := filepath.Join(root, "linked-service")
	os.MkdirAll(linkDir, 0755)
	linkPath := filepath.Join(linkDir, ".smokesig.yaml")
	if err := os.Symlink(realConfig, linkPath); err != nil {
		t.Skip("symlinks not supported on this platform:", err)
	}

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (symlinked), got %d", len(configs))
	}
	if filepath.Base(configs[0].Dir) != "linked-service" {
		t.Errorf("expected 'linked-service', got %q", filepath.Base(configs[0].Dir))
	}
}

func TestDiscover_LegacySmokeYAML(t *testing.T) {
	// Tests that legacy .smoke.yaml files are discovered
	root := t.TempDir()
	legacyDir := filepath.Join(root, "legacy-service")
	os.MkdirAll(legacyDir, 0755)
	os.WriteFile(filepath.Join(legacyDir, ".smoke.yaml"), []byte("version: 1\nproject: legacy\ntests: []\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (legacy .smoke.yaml), got %d", len(configs))
	}
	if configs[0].Project != "legacy-service" {
		t.Errorf("expected project name 'legacy-service', got %q", configs[0].Project)
	}
}

func TestDiscover_DefaultSkipDirs(t *testing.T) {
	// Tests that all default skip dirs are excluded
	root := t.TempDir()

	defaultSkipped := []string{".git", "node_modules", "vendor", "__pycache__", "dist", "build", "target", ".next", ".cache"}
	for _, dir := range defaultSkipped {
		d := filepath.Join(root, dir)
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, ".smokesig.yaml"), []byte("version: 1\nproject: skip\ntests: []\n"), 0644)
	}

	// Also add one that should be found
	keepDir := filepath.Join(root, "api")
	os.MkdirAll(keepDir, 0755)
	os.WriteFile(filepath.Join(keepDir, ".smokesig.yaml"), []byte("version: 1\nproject: api\ntests: []\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected only 1 config (api), got %d: %v", len(configs), configs)
	}
	if filepath.Base(configs[0].Dir) != "api" {
		t.Errorf("expected 'api', got %q", filepath.Base(configs[0].Dir))
	}
}

func TestDiscover_WalkErrorIsSkipped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based walk error not reliable on Windows")
	}
	root := t.TempDir()

	// Create a readable sibling that should still be discovered
	goodDir := filepath.Join(root, "good-service")
	os.MkdirAll(goodDir, 0755)
	os.WriteFile(filepath.Join(goodDir, ".smokesig.yaml"), []byte("version: 1\nproject: good\ntests: []\n"), 0644)

	// Create a directory that will trigger a walk error (unreadable)
	badDir := filepath.Join(root, "bad-service")
	os.MkdirAll(badDir, 0755)
	// Make it unreadable so WalkDir gets a permission error when trying to read its contents
	os.Chmod(badDir, 0000)
	defer os.Chmod(badDir, 0755) // restore for cleanup

	// Discover should skip the error and still find good-service
	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (good-service), got %d: %v", len(configs), configs)
	}
	if filepath.Base(configs[0].Dir) != "good-service" {
		t.Errorf("expected 'good-service', got %q", filepath.Base(configs[0].Dir))
	}
}

func TestDiscover_SubConfigFields(t *testing.T) {
	// Tests that SubConfig fields (Path, Dir, Project) are set correctly
	root := t.TempDir()
	svcDir := filepath.Join(root, "my-service")
	os.MkdirAll(svcDir, 0755)
	configFile := filepath.Join(svcDir, ".smokesig.yaml")
	os.WriteFile(configFile, []byte("version: 1\nproject: my-service\ntests: []\n"), 0644)

	configs, err := Discover(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	c := configs[0]
	if c.Path != configFile {
		t.Errorf("Path = %q, want %q", c.Path, configFile)
	}
	if c.Dir != svcDir {
		t.Errorf("Dir = %q, want %q", c.Dir, svcDir)
	}
	if c.Project != "my-service" {
		t.Errorf("Project = %q, want %q", c.Project, "my-service")
	}
}
