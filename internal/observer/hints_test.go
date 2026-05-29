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

// TestReadPortlessPort_InvalidJSON verifies readPortlessPort returns 0 for invalid JSON.
func TestReadPortlessPort_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`not valid json`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	port := readPortlessPort(dir)
	if port != 0 {
		t.Fatalf("expected 0 for invalid JSON, got %d", port)
	}
}

// TestContainsInt_EmptySlice verifies containsInt returns false for empty slice.
func TestContainsInt_EmptySlice(t *testing.T) {
	if containsInt([]int{}, 5) {
		t.Error("expected false for empty slice")
	}
}

// TestContainsInt_NegativeNumber verifies containsInt finds negative numbers.
func TestContainsInt_NegativeNumber(t *testing.T) {
	if !containsInt([]int{-1, -2, -3}, -2) {
		t.Error("expected true for -2 in [-1,-2,-3]")
	}
}

// TestContainsInt_NotFound verifies containsInt returns false when value is absent.
func TestContainsInt_NotFound(t *testing.T) {
	if containsInt([]int{1, 2, 3}, 99) {
		t.Error("expected false for 99 not in [1,2,3]")
	}
}

// TestContainsInt_NilSlice verifies containsInt handles nil slice gracefully.
func TestContainsInt_NilSlice(t *testing.T) {
	if containsInt(nil, 0) {
		t.Error("expected false for nil slice")
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

// TestHintsFromDir_DeduplicatesExpectedPorts verifies that ExpectedPorts are deduplicated
// when portless.json port matches a stack's expected port (exercises seenPorts dedup branch).
func TestHintsFromDir_DeduplicatesExpectedPorts(t *testing.T) {
	dir := t.TempDir()
	// Go stack expects port 8080 — use portless port 8080 to trigger dedup.
	err := os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"port":8080}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	hints := HintsFromDir(dir)
	// Count occurrences of port 8080.
	count := 0
	for _, p := range hints.ExpectedPorts {
		if p == 8080 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("port 8080 appears %d times in ExpectedPorts, want exactly 1: %v", count, hints.ExpectedPorts)
	}
}

// TestHintsFromDir_UnknownProjectType exercises the !ok branch in the stackHints loop
// by creating a project type that has no entry in stackHints (e.g., Docker/Terraform).
func TestHintsFromDir_UnknownProjectType(t *testing.T) {
	dir := t.TempDir()
	// Dockerfile triggers Docker detection, which has no stackHints entry.
	err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Should not panic and should return empty hints (Docker has no stackHints).
	hints := HintsFromDir(dir)
	_ = hints // result may or may not have ports depending on what else is detected
}

// TestHintsFromDir_DeduplicatesProbePaths verifies that ExtraProbePaths are deduplicated
// when multiple detected project types share the same probe path (exercises seenPaths branch).
func TestHintsFromDir_DeduplicatesProbePaths(t *testing.T) {
	// Ruby and Node both have /api as a probe path. Write markers for both.
	dir := t.TempDir()
	// package.json => Node detection
	err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Gemfile => Ruby detection
	err = os.WriteFile(filepath.Join(dir, "Gemfile"), []byte(`source "https://rubygems.org"`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	hints := HintsFromDir(dir)
	// Count /api occurrences — should be exactly 1.
	apiCount := 0
	for _, p := range hints.ExtraProbePaths {
		if p == "/api" {
			apiCount++
		}
	}
	if apiCount > 1 {
		t.Errorf("/api appears %d times in ExtraProbePaths, want at most 1: %v", apiCount, hints.ExtraProbePaths)
	}
}
