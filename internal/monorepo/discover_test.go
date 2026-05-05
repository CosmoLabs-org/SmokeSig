package monorepo

import (
	"os"
	"path/filepath"
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
