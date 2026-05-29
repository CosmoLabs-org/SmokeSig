package detector

import (
	"os"
	"path/filepath"
	"testing"
)

// --- hasDepInPackageJSON coverage ---

// TestHasDepInPackageJSON_MissingFile verifies false when package.json doesn't exist.
func TestHasDepInPackageJSON_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if hasDepInPackageJSON(dir, "react-native") {
		t.Error("expected false for missing package.json")
	}
}

// TestHasDepInPackageJSON_InvalidJSON verifies false when package.json is invalid JSON.
func TestHasDepInPackageJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{invalid json`), 0644); err != nil {
		t.Fatal(err)
	}
	if hasDepInPackageJSON(dir, "react-native") {
		t.Error("expected false for invalid JSON")
	}
}

// TestHasDepInPackageJSON_MissingDep verifies false when dependency not present.
func TestHasDepInPackageJSON_MissingDep(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"dependencies":{"express":"^4.0.0"},"devDependencies":{"jest":"^29.0.0"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	if hasDepInPackageJSON(dir, "react-native") {
		t.Error("expected false when dep not listed")
	}
}

// TestHasDepInPackageJSON_InDependencies verifies true when dep is in dependencies.
func TestHasDepInPackageJSON_InDependencies(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"dependencies":{"react-native":"^0.73.0"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	if !hasDepInPackageJSON(dir, "react-native") {
		t.Error("expected true when dep in dependencies")
	}
}

// TestHasDepInPackageJSON_InDevDependencies verifies true when dep is in devDependencies.
func TestHasDepInPackageJSON_InDevDependencies(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"devDependencies":{"react-native":"^0.73.0"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	if !hasDepInPackageJSON(dir, "react-native") {
		t.Error("expected true when dep in devDependencies")
	}
}

// --- hasFlutterDep coverage ---

// TestHasFlutterDep_MissingFile verifies false when pubspec.yaml doesn't exist.
func TestHasFlutterDep_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if hasFlutterDep(dir) {
		t.Error("expected false for missing pubspec.yaml")
	}
}

// TestHasFlutterDep_MissingFlutterSDK verifies false when pubspec.yaml exists but has no flutter SDK dep.
func TestHasFlutterDep_MissingFlutterSDK(t *testing.T) {
	dir := t.TempDir()
	pubspec := []byte("name: myapp\ndependencies:\n  http: ^1.0.0\n")
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), pubspec, 0644); err != nil {
		t.Fatal(err)
	}
	if hasFlutterDep(dir) {
		t.Error("expected false when no 'sdk: flutter' in pubspec.yaml")
	}
}

// TestHasFlutterDep_WithFlutterSDK verifies true when pubspec.yaml has flutter SDK dep.
func TestHasFlutterDep_WithFlutterSDK(t *testing.T) {
	dir := t.TempDir()
	pubspec := []byte("name: myapp\ndependencies:\n  flutter:\n    sdk: flutter\n")
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), pubspec, 0644); err != nil {
		t.Fatal(err)
	}
	if !hasFlutterDep(dir) {
		t.Error("expected true when pubspec.yaml has 'sdk: flutter'")
	}
}

// --- detectNodeCLI coverage ---

// TestDetectNodeCLI_MissingPackageJSON verifies nil when package.json doesn't exist.
func TestDetectNodeCLI_MissingPackageJSON(t *testing.T) {
	dir := t.TempDir()
	result := detectNodeCLI(dir)
	if result != nil {
		t.Errorf("expected nil for missing package.json, got %v", result)
	}
}

// TestDetectNodeCLI_InvalidJSON verifies nil when package.json is invalid JSON.
func TestDetectNodeCLI_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{bad json`), 0644); err != nil {
		t.Fatal(err)
	}
	result := detectNodeCLI(dir)
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
}

// TestDetectNodeCLI_NoBinField verifies nil when package.json has no bin field.
func TestDetectNodeCLI_NoBinField(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"name":"mytool","version":"1.0.0"}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	result := detectNodeCLI(dir)
	if result != nil {
		t.Errorf("expected nil when no bin field, got %v", result)
	}
}

// TestDetectNodeCLI_WithBinStringAndName verifies CLIInfo returned when bin is a string and name is set.
func TestDetectNodeCLI_WithBinStringAndName(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"name":"mycli","bin":"./bin/mycli.js"}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	result := detectNodeCLI(dir)
	if result == nil {
		t.Fatal("expected CLIInfo for package.json with bin string, got nil")
	}
	if result.Binary != "mycli" {
		t.Errorf("binary = %q, want %q", result.Binary, "mycli")
	}
}

// TestDetectNodeCLI_WithBinObjectAndName verifies CLIInfo returned when bin is an object.
func TestDetectNodeCLI_WithBinObjectAndName(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"name":"myapp","bin":{"myapp":"./bin/myapp.js"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	result := detectNodeCLI(dir)
	if result == nil {
		t.Fatal("expected CLIInfo for package.json with bin object, got nil")
	}
	if result.Binary != "myapp" {
		t.Errorf("binary = %q, want %q", result.Binary, "myapp")
	}
}

// TestDetectNodeCLI_WithBinButNoName verifies binary falls back to dir name when name missing.
func TestDetectNodeCLI_WithBinButNoName(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"bin":"./cli.js"}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	result := detectNodeCLI(dir)
	if result == nil {
		t.Fatal("expected CLIInfo when bin is set but name missing")
	}
	// Should use dir base name as fallback
	if result.Binary != filepath.Base(dir) {
		t.Errorf("binary = %q, want dir base %q", result.Binary, filepath.Base(dir))
	}
}

// TestDetectNodeCLI_EmptyBinObject verifies nil when bin field is an empty object.
func TestDetectNodeCLI_EmptyBinObject(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"name":"mytool","bin":{}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	result := detectNodeCLI(dir)
	if result != nil {
		t.Errorf("expected nil for empty bin object, got %v", result)
	}
}

// --- detectPythonCLI coverage ---

// TestDetectPythonCLI_MissingFile verifies nil when pyproject.toml doesn't exist.
func TestDetectPythonCLI_MissingFile(t *testing.T) {
	dir := t.TempDir()
	result := detectPythonCLI(dir)
	if result != nil {
		t.Errorf("expected nil for missing pyproject.toml, got %v", result)
	}
}

// TestDetectPythonCLI_NoProjectScripts verifies nil when pyproject.toml has no [project.scripts].
func TestDetectPythonCLI_NoProjectScripts(t *testing.T) {
	dir := t.TempDir()
	toml := []byte("[tool.poetry]\nname = \"myapp\"\n")
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), toml, 0644); err != nil {
		t.Fatal(err)
	}
	result := detectPythonCLI(dir)
	if result != nil {
		t.Errorf("expected nil when no [project.scripts], got %v", result)
	}
}

// TestDetectPythonCLI_WithProjectScripts verifies CLIInfo when [project.scripts] is present.
func TestDetectPythonCLI_WithProjectScripts(t *testing.T) {
	dir := t.TempDir()
	toml := []byte("[project]\nname = \"mytool\"\n\n[project.scripts]\nmytool = \"mytool:main\"\n")
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), toml, 0644); err != nil {
		t.Fatal(err)
	}
	result := detectPythonCLI(dir)
	if result == nil {
		t.Fatal("expected CLIInfo for pyproject.toml with [project.scripts]")
	}
	if result.Binary != "mytool" {
		t.Errorf("binary = %q, want %q", result.Binary, "mytool")
	}
}

// --- hasPkgScript coverage ---

// TestHasPkgScript_MissingFile verifies false when package.json doesn't exist.
func TestHasPkgScript_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if hasPkgScript(dir, "lint") {
		t.Error("expected false for missing package.json")
	}
}

// TestHasPkgScript_InvalidJSON verifies false when package.json has invalid JSON.
func TestHasPkgScript_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{invalid`), 0644); err != nil {
		t.Fatal(err)
	}
	if hasPkgScript(dir, "lint") {
		t.Error("expected false for invalid JSON")
	}
}

// TestHasPkgScript_ScriptPresent verifies true when the script key exists.
func TestHasPkgScript_ScriptPresent(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"scripts":{"lint":"eslint .","build":"tsc"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	if !hasPkgScript(dir, "lint") {
		t.Error("expected true when lint script present")
	}
}

// TestHasPkgScript_ScriptAbsent verifies false when the script key is not present.
func TestHasPkgScript_ScriptAbsent(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"scripts":{"build":"tsc"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	if hasPkgScript(dir, "lint") {
		t.Error("expected false when lint script absent")
	}
}

// --- GenerateConfigWithOptions coverage ---

// TestGenerateConfigWithOptions_CustomProjectName verifies the project name is always the dir base.
func TestGenerateConfigWithOptions_CustomProjectName(t *testing.T) {
	dir := t.TempDir()
	cfg := GenerateConfigWithOptions(dir, []ProjectType{Go}, ConfigOptions{})
	if cfg.Project != filepath.Base(dir) {
		t.Errorf("project = %q, want %q", cfg.Project, filepath.Base(dir))
	}
}

// TestGenerateConfigWithOptions_WithDocIntegrityOption verifies WithDocIntegrity option is honored.
func TestGenerateConfigWithOptions_WithDocIntegrityOption(t *testing.T) {
	dir := t.TempDir()
	// Create a Go project with cmd/ dir so CLI detection fires
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/mytool\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0755); err != nil {
		t.Fatal(err)
	}
	// DetectDocFiles requires at least one doc file to add the test
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# mytool\n"), 0644); err != nil {
		t.Fatal(err)
	}
	types := []ProjectType{Go}
	cfg := GenerateConfigWithOptions(dir, types, ConfigOptions{WithDocIntegrity: true})
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// With WithDocIntegrity, there should be a doc_integrity test
	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a doc_integrity test when WithDocIntegrity=true, CLI detected, and doc file present")
	}
}

// TestGenerateConfigWithOptions_MultipleTypes verifies config has tests for each type.
func TestGenerateConfigWithOptions_MultipleTypes(t *testing.T) {
	dir := t.TempDir()
	cfg := GenerateConfigWithOptions(dir, []ProjectType{Go, Python, Docker}, ConfigOptions{})
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// Go: 1 prereq + 2 tests, Python: 1 prereq + 1 test, Docker: 0 prereqs + 1 test
	if len(cfg.Prereqs) < 2 {
		t.Errorf("expected at least 2 prereqs for Go+Python, got %d", len(cfg.Prereqs))
	}
	if len(cfg.Tests) < 4 {
		t.Errorf("expected at least 4 tests for Go+Python+Docker, got %d", len(cfg.Tests))
	}
}

// TestGenerateConfigWithOptions_NodeWithBun verifies bun commands used when bun.lock exists.
func TestGenerateConfigWithOptions_NodeWithBun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"lint":"eslint ."}, "name":"myapp"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bun.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := GenerateConfigWithOptions(dir, []ProjectType{Node}, ConfigOptions{})
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// Should have bun prereq
	foundBun := false
	for _, p := range cfg.Prereqs {
		if p.Name == "Bun installed" {
			foundBun = true
			break
		}
	}
	if !foundBun {
		t.Error("expected 'Bun installed' prereq when bun.lock exists")
	}
	// Should have bun lint command
	foundLint := false
	for _, test := range cfg.Tests {
		if test.Name == "Lint" && test.Run == "bun run lint" {
			foundLint = true
			break
		}
	}
	if !foundLint {
		t.Error("expected 'bun run lint' test when bun.lock and lint script present")
	}
}

// TestDetectFlutterProjectWithPubspec verifies Flutter is detected when pubspec.yaml has flutter SDK.
func TestDetectFlutterProjectWithPubspec(t *testing.T) {
	dir := t.TempDir()
	pubspec := []byte("name: myflutter\ndependencies:\n  flutter:\n    sdk: flutter\n")
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), pubspec, 0644); err != nil {
		t.Fatal(err)
	}
	types := Detect(dir)
	found := false
	for _, tp := range types {
		if tp == Flutter {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Flutter in detected types, got %v", types)
	}
}

// TestDetectPubspecWithoutFlutter verifies pubspec.yaml without flutter SDK does not produce Flutter type.
func TestDetectPubspecWithoutFlutter(t *testing.T) {
	dir := t.TempDir()
	pubspec := []byte("name: mydart\ndependencies:\n  http: ^1.0.0\n")
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), pubspec, 0644); err != nil {
		t.Fatal(err)
	}
	types := Detect(dir)
	for _, tp := range types {
		if tp == Flutter {
			t.Errorf("unexpected Flutter type without 'sdk: flutter' in pubspec.yaml")
		}
	}
}

// TestDetectReactNativeWithDep verifies ReactNative detection via package.json dep + app.json.
func TestDetectReactNativeWithDep(t *testing.T) {
	dir := t.TempDir()
	pkg := []byte(`{"dependencies":{"react-native":"^0.73.0"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkg, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.json"), []byte(`{"name":"myrnapp"}`), 0644); err != nil {
		t.Fatal(err)
	}
	types := Detect(dir)
	foundRN := false
	for _, tp := range types {
		if tp == ReactNative {
			foundRN = true
			break
		}
	}
	if !foundRN {
		t.Errorf("expected ReactNative type, got %v", types)
	}
}
