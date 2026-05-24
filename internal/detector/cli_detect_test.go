package detector

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// --- DetectCLI tests ---

func TestDetectCLI_GoWithCmdDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/mytool\ngo 1.22\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0755)

	info := DetectCLI(dir, []ProjectType{Go})
	if info == nil {
		t.Fatal("expected CLI detection for Go project with cmd/ dir")
	}
	if info.Binary != "./mytool" {
		t.Errorf("binary = %q, want %q", info.Binary, "./mytool")
	}
}

func TestDetectCLI_GoWithMainGo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/cli-app\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	info := DetectCLI(dir, []ProjectType{Go})
	if info == nil {
		t.Fatal("expected CLI detection for Go project with main.go")
	}
	if info.Binary != "./cli-app" {
		t.Errorf("binary = %q, want %q", info.Binary, "./cli-app")
	}
}

func TestDetectCLI_GoSimpleModule(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module mybin\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	info := DetectCLI(dir, []ProjectType{Go})
	if info == nil {
		t.Fatal("expected CLI detection")
	}
	if info.Binary != "./mybin" {
		t.Errorf("binary = %q, want %q", info.Binary, "./mybin")
	}
}

func TestDetectCLI_GoLibraryOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/lib\ngo 1.22\n"), 0644)
	// No cmd/ directory and no main.go — it's a library

	info := DetectCLI(dir, []ProjectType{Go})
	if info != nil {
		t.Errorf("expected no CLI for Go library, got %+v", info)
	}
}

func TestDetectCLI_NodeWithBin(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name": "my-cli", "bin": "./dist/cli.js"}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	info := DetectCLI(dir, []ProjectType{Node})
	if info == nil {
		t.Fatal("expected CLI detection for Node project with bin field")
	}
	if info.Binary != "my-cli" {
		t.Errorf("binary = %q, want %q", info.Binary, "my-cli")
	}
}

func TestDetectCLI_NodeWithBinObject(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name": "multi-bin", "bin": {"tool-a": "./a.js", "tool-b": "./b.js"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	info := DetectCLI(dir, []ProjectType{Node})
	if info == nil {
		t.Fatal("expected CLI detection for Node project with bin object")
	}
	if info.Binary != "multi-bin" {
		t.Errorf("binary = %q, want %q", info.Binary, "multi-bin")
	}
}

func TestDetectCLI_NodeNoBin(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name": "my-lib", "main": "index.js"}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	info := DetectCLI(dir, []ProjectType{Node})
	if info != nil {
		t.Errorf("expected no CLI for Node library, got %+v", info)
	}
}

func TestDetectCLI_NodeEmptyBin(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name": "my-lib", "bin": {}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	info := DetectCLI(dir, []ProjectType{Node})
	if info != nil {
		t.Errorf("expected no CLI for empty bin object, got %+v", info)
	}
}

func TestDetectCLI_PythonWithScripts(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
name = "my-python-cli"
version = "1.0.0"

[project.scripts]
mycli = "my_python_cli:main"
`
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644)

	info := DetectCLI(dir, []ProjectType{Python})
	if info == nil {
		t.Fatal("expected CLI detection for Python project with [project.scripts]")
	}
	if info.Binary != "my-python-cli" {
		t.Errorf("binary = %q, want %q", info.Binary, "my-python-cli")
	}
}

func TestDetectCLI_PythonNoScripts(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
name = "my-lib"
version = "1.0.0"
`
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644)

	info := DetectCLI(dir, []ProjectType{Python})
	if info != nil {
		t.Errorf("expected no CLI for Python library, got %+v", info)
	}
}

func TestDetectCLI_RustWithBin(t *testing.T) {
	dir := t.TempDir()
	cargo := `[package]
name = "my-rust-cli"
version = "0.1.0"

[[bin]]
name = "mycli"
path = "src/main.rs"
`
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644)

	info := DetectCLI(dir, []ProjectType{Rust})
	if info == nil {
		t.Fatal("expected CLI detection for Rust project with [[bin]]")
	}
	if info.Binary != "./my-rust-cli" {
		t.Errorf("binary = %q, want %q", info.Binary, "./my-rust-cli")
	}
}

func TestDetectCLI_RustWithMainRs(t *testing.T) {
	dir := t.TempDir()
	cargo := `[package]
name = "rusttool"
version = "0.1.0"
`
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644)
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "main.rs"), []byte("fn main() {}"), 0644)

	info := DetectCLI(dir, []ProjectType{Rust})
	if info == nil {
		t.Fatal("expected CLI detection for Rust project with src/main.rs")
	}
	if info.Binary != "./rusttool" {
		t.Errorf("binary = %q, want %q", info.Binary, "./rusttool")
	}
}

func TestDetectCLI_RustLibraryOnly(t *testing.T) {
	dir := t.TempDir()
	cargo := `[package]
name = "my-lib"
version = "0.1.0"
`
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644)
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "lib.rs"), []byte("pub fn hello() {}"), 0644)

	info := DetectCLI(dir, []ProjectType{Rust})
	if info != nil {
		t.Errorf("expected no CLI for Rust library, got %+v", info)
	}
}

func TestDetectCLI_NonCLIProjectType(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Dockerfile")

	info := DetectCLI(dir, []ProjectType{Docker})
	if info != nil {
		t.Errorf("expected no CLI for Docker project, got %+v", info)
	}
}

func TestDetectCLI_NilTypes(t *testing.T) {
	dir := t.TempDir()
	info := DetectCLI(dir, nil)
	if info != nil {
		t.Errorf("expected nil for no types, got %+v", info)
	}
}

// --- DetectDocFiles tests ---

func TestDetectDocFiles_AllPresent(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "README.md")
	touch(t, dir, "CLAUDE.md")
	touch(t, dir, "SPEC.md")
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	touch(t, dir, "docs/USAGE.md")

	docs := DetectDocFiles(dir)
	if len(docs) != 4 {
		t.Fatalf("expected 4 doc files, got %d: %v", len(docs), docs)
	}
	for _, want := range []string{"README.md", "CLAUDE.md", "docs/USAGE.md", "SPEC.md"} {
		if !slices.Contains(docs, want) {
			t.Errorf("missing doc file %q in %v", want, docs)
		}
	}
}

func TestDetectDocFiles_OnlyReadme(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "README.md")

	docs := DetectDocFiles(dir)
	if len(docs) != 1 || docs[0] != "README.md" {
		t.Errorf("expected [README.md], got %v", docs)
	}
}

func TestDetectDocFiles_NonePresent(t *testing.T) {
	dir := t.TempDir()
	docs := DetectDocFiles(dir)
	if len(docs) != 0 {
		t.Errorf("expected no doc files, got %v", docs)
	}
}

// --- GenerateConfig doc_integrity integration tests ---

func TestGenerateConfig_GoCLI_IncludesDocIntegrity(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/smokesig\ngo 1.22\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# SmokeSig\n"), 0644)

	cfg := GenerateConfig(dir, []ProjectType{Go})

	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			found = true
			if test.Name != "Docs in sync" {
				t.Errorf("test name = %q, want %q", test.Name, "Docs in sync")
			}
			if test.Expect.DocIntegrity.Binary != "./smokesig" {
				t.Errorf("binary = %q, want %q", test.Expect.DocIntegrity.Binary, "./smokesig")
			}
			if !slices.Contains(test.Expect.DocIntegrity.Docs, "README.md") {
				t.Errorf("docs should contain README.md, got %v", test.Expect.DocIntegrity.Docs)
			}
			if test.Expect.DocIntegrity.CheckExamples {
				t.Error("check_examples should default to false")
			}
			if !slices.Contains(test.Expect.DocIntegrity.IgnoreCommands, "help") {
				t.Errorf("ignore_commands should contain 'help', got %v", test.Expect.DocIntegrity.IgnoreCommands)
			}
			if !slices.Contains(test.Expect.DocIntegrity.IgnoreCommands, "completion") {
				t.Errorf("ignore_commands should contain 'completion', got %v", test.Expect.DocIntegrity.IgnoreCommands)
			}
			if !slices.Contains(test.Tags, "docs") || !slices.Contains(test.Tags, "ci") {
				t.Errorf("tags should be [docs, ci], got %v", test.Tags)
			}
			break
		}
	}
	if !found {
		t.Error("expected doc_integrity test for Go CLI project with docs")
	}
}

func TestGenerateConfig_GoCLI_NoDocs_SkipsDocIntegrity(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/mytool\ngo 1.22\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0755)
	// No doc files present

	cfg := GenerateConfig(dir, []ProjectType{Go})

	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			t.Error("doc_integrity test should be skipped when no doc files exist")
		}
	}
}

func TestGenerateConfig_GoLibrary_NoDocIntegrity(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/lib\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Lib\n"), 0644)
	// No cmd/ or main.go — it's a library

	cfg := GenerateConfig(dir, []ProjectType{Go})

	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			t.Error("doc_integrity test should not be included for Go libraries")
		}
	}
}

func TestGenerateConfig_NodeCLI_IncludesDocIntegrity(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name": "my-cli-tool", "bin": "./cli.js"}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My CLI\n"), 0644)

	cfg := GenerateConfig(dir, []ProjectType{Node})

	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			found = true
			if test.Expect.DocIntegrity.Binary != "my-cli-tool" {
				t.Errorf("binary = %q, want %q", test.Expect.DocIntegrity.Binary, "my-cli-tool")
			}
			break
		}
	}
	if !found {
		t.Error("expected doc_integrity test for Node CLI project")
	}
}

func TestGenerateConfig_RustCLI_IncludesDocIntegrity(t *testing.T) {
	dir := t.TempDir()
	cargo := `[package]
name = "my-rust-tool"
version = "0.1.0"

[[bin]]
name = "mytool"
`
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Tool\n"), 0644)

	cfg := GenerateConfig(dir, []ProjectType{Rust})

	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected doc_integrity test for Rust CLI project")
	}
}

func TestGenerateConfig_PythonCLI_IncludesDocIntegrity(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
name = "pytool"

[project.scripts]
pytool = "pytool:main"
`
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Tool\n"), 0644)

	cfg := GenerateConfig(dir, []ProjectType{Python})

	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected doc_integrity test for Python CLI project")
	}
}

func TestGenerateConfig_WithDocIntegrity_ForcesInclusion(t *testing.T) {
	dir := t.TempDir()
	// Docker project — not a CLI type
	touch(t, dir, "Dockerfile")
	touch(t, dir, "README.md")

	opts := ConfigOptions{WithDocIntegrity: true}
	cfg := GenerateConfigWithOptions(dir, []ProjectType{Docker}, opts)

	found := false
	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			found = true
			if !slices.Contains(test.Expect.DocIntegrity.Docs, "README.md") {
				t.Errorf("docs should include README.md, got %v", test.Expect.DocIntegrity.Docs)
			}
			break
		}
	}
	if !found {
		t.Error("expected doc_integrity test when --with-doc-integrity is set")
	}
}

func TestGenerateConfig_WithDocIntegrity_NoDocs_Skips(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Dockerfile")
	// No doc files

	opts := ConfigOptions{WithDocIntegrity: true}
	cfg := GenerateConfigWithOptions(dir, []ProjectType{Docker}, opts)

	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			t.Error("doc_integrity should be skipped even with flag when no doc files exist")
		}
	}
}

func TestGenerateConfig_MultipleDocs_AllIncluded(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tool\ngo 1.22\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0755)
	touch(t, dir, "README.md")
	touch(t, dir, "CLAUDE.md")
	touch(t, dir, "SPEC.md")

	cfg := GenerateConfig(dir, []ProjectType{Go})

	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			if len(test.Expect.DocIntegrity.Docs) != 3 {
				t.Errorf("expected 3 docs, got %d: %v", len(test.Expect.DocIntegrity.Docs), test.Expect.DocIntegrity.Docs)
			}
			return
		}
	}
	t.Error("expected doc_integrity test")
}

func TestGenerateConfig_DocIntegrity_NotDuplicated(t *testing.T) {
	// Go project that IS a CLI — doc_integrity should appear exactly once.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tool\ngo 1.22\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0755)
	touch(t, dir, "README.md")

	opts := ConfigOptions{WithDocIntegrity: true}
	cfg := GenerateConfigWithOptions(dir, []ProjectType{Go}, opts)

	count := 0
	for _, test := range cfg.Tests {
		if test.Expect.DocIntegrity != nil {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 doc_integrity test, got %d", count)
	}
}
