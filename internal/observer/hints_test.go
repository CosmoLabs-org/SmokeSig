package observer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPortlessJSON_FlatFormat(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"port":4650}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	port := readPortlessPort(dir)
	if port != 4650 {
		t.Fatalf("expected 4650, got %d", port)
	}
}

func TestReadPortlessJSON_AppsFormat_NoPort(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"apps":{"web":{}}}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	port := readPortlessPort(dir)
	if port != 0 {
		t.Fatalf("expected 0 for apps-only format, got %d", port)
	}
}

func TestReadPortlessJSON_Missing(t *testing.T) {
	dir := t.TempDir()
	port := readPortlessPort(dir)
	if port != 0 {
		t.Fatalf("expected 0 for missing file, got %d", port)
	}
}

func TestHintsFromDir_PortlessOverridesStack(t *testing.T) {
	dir := t.TempDir()
	// Write portless.json with custom port.
	err := os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"port":9999}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Write go.mod so detector detects Go.
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	hints := HintsFromDir(dir)
	if len(hints.ExpectedPorts) == 0 {
		t.Fatal("expected at least one port")
	}
	if hints.ExpectedPorts[0] != 9999 {
		t.Fatalf("expected first port 9999 (portless), got %d", hints.ExpectedPorts[0])
	}
}

func TestHintsFromDir_GoProject(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	hints := HintsFromDir(dir)
	if !containsInt(hints.ExpectedPorts, 8080) {
		t.Fatalf("expected port 8080 in %v", hints.ExpectedPorts)
	}
	found := false
	for _, p := range hints.ExtraProbePaths {
		if p == "/metrics" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected /metrics in paths %v", hints.ExtraProbePaths)
	}
}

func TestHintsFromDir_NodeProject(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	hints := HintsFromDir(dir)
	if !containsInt(hints.ExpectedPorts, 3000) {
		t.Fatalf("expected port 3000 in %v", hints.ExpectedPorts)
	}
}

func TestHintsFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	hints := HintsFromDir(dir)
	if len(hints.ExpectedPorts) != 0 {
		t.Fatalf("expected no ports, got %v", hints.ExpectedPorts)
	}
	if len(hints.ExtraProbePaths) != 0 {
		t.Fatalf("expected no paths, got %v", hints.ExtraProbePaths)
	}
}

func TestHintsFromDir_PortlessMergesWithStack(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"port":9999}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	hints := HintsFromDir(dir)
	if len(hints.ExpectedPorts) < 2 {
		t.Fatalf("expected >= 2 ports (portless + stack), got %d: %v", len(hints.ExpectedPorts), hints.ExpectedPorts)
	}
	if len(hints.ExtraProbePaths) == 0 {
		t.Fatal("expected probe paths from stack")
	}
}
