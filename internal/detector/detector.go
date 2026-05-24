package detector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ProjectType identifies the kind of project detected.
type ProjectType string

const (
	Go          ProjectType = "go"
	Node        ProjectType = "node"
	Python      ProjectType = "python"
	Docker      ProjectType = "docker"
	Rust        ProjectType = "rust"
	ReactNative ProjectType = "react-native"
	Flutter     ProjectType = "flutter"
	IOS         ProjectType = "ios"
	Android     ProjectType = "android"
	Java        ProjectType = "java"
	JavaGradle  ProjectType = "java-gradle"
	DotNet      ProjectType = "dotnet"
	Ruby        ProjectType = "ruby"
	PHP         ProjectType = "php"
	Deno        ProjectType = "deno"
	Terraform   ProjectType = "terraform"
	Helm        ProjectType = "helm"
	Kustomize   ProjectType = "kustomize"
	Serverless  ProjectType = "serverless"
	Zig         ProjectType = "zig"
	Elixir      ProjectType = "elixir"
	Scala       ProjectType = "scala"
	SwiftServer ProjectType = "swift-server"
	DartServer  ProjectType = "dart-server"
	Hugo        ProjectType = "hugo"
	Astro       ProjectType = "astro"
	Jekyll      ProjectType = "jekyll"
	Make        ProjectType = "make"
	CMake       ProjectType = "cmake"
	Haskell     ProjectType = "haskell"
	Lua         ProjectType = "lua"
)

// exists returns true if the given path exists under dir.
func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// hasGlob returns true if any file matching the glob pattern exists under dir.
func hasGlob(dir, pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	return len(matches) > 0
}

// hasType checks if a specific type is already in the list.
func hasType(types []ProjectType, want ProjectType) bool {
	return slices.Contains(types, want)
}

// hasDepInPackageJSON checks if package.json has a specific dependency.
func hasDepInPackageJSON(dir, dep string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	_, ok := pkg.Dependencies[dep]
	if !ok {
		_, ok = pkg.DevDependencies[dep]
	}
	return ok
}

// hasFlutterDep checks if pubspec.yaml has a Flutter SDK dependency.
func hasFlutterDep(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "pubspec.yaml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "sdk: flutter")
}

// Detect scans dir for project type markers and returns all detected types.
func Detect(dir string) []ProjectType {
	var types []ProjectType

	if exists(dir, "go.mod") {
		types = append(types, Go)
	}
	if exists(dir, "package.json") {
		types = append(types, Node)
	}
	if exists(dir, "pyproject.toml") || exists(dir, "requirements.txt") || exists(dir, "setup.py") {
		types = append(types, Python)
	}
	if exists(dir, "Dockerfile") || exists(dir, "docker-compose.yml") {
		types = append(types, Docker)
	}
	if exists(dir, "Cargo.toml") {
		types = append(types, Rust)
	}

	// React Native: app.json + react-native dependency or metro.config.js
	if exists(dir, "app.json") {
		if hasDepInPackageJSON(dir, "react-native") || exists(dir, "metro.config.js") {
			types = append(types, ReactNative)
		}
	}
	// Flutter: pubspec.yaml with flutter dependency
	if exists(dir, "pubspec.yaml") && hasFlutterDep(dir) {
		types = append(types, Flutter)
	}
	// iOS native: xcodeproj/xcworkspace or Podfile (skip if RN/Flutter)
	if !hasType(types, ReactNative) && !hasType(types, Flutter) {
		if hasGlob(dir, "*.xcodeproj") || hasGlob(dir, "*.xcworkspace") || exists(dir, "Podfile") {
			types = append(types, IOS)
		}
	}
	// Android native: build.gradle without Go/Node (skip if RN/Flutter)
	if !hasType(types, ReactNative) && !hasType(types, Flutter) {
		if exists(dir, "build.gradle") || exists(dir, "build.gradle.kts") {
			if !exists(dir, "go.mod") && !exists(dir, "package.json") {
				types = append(types, Android)
			}
		}
	}

	// Java/Maven: pom.xml
	if exists(dir, "pom.xml") {
		types = append(types, Java)
	}
	// Java/Gradle: build.gradle alongside package.json (Gradle-based JS projects)
	if hasType(types, Node) {
		if exists(dir, "build.gradle") || exists(dir, "build.gradle.kts") {
			types = append(types, JavaGradle)
		}
	}
	// .NET/C#: csproj or sln files
	if hasGlob(dir, "*.csproj") || hasGlob(dir, "*.sln") {
		types = append(types, DotNet)
	}
	// Ruby: Gemfile
	if exists(dir, "Gemfile") {
		types = append(types, Ruby)
	}
	// PHP: composer.json
	if exists(dir, "composer.json") {
		types = append(types, PHP)
	}
	// Deno: deno.json or deno.jsonc
	if exists(dir, "deno.json") || exists(dir, "deno.jsonc") {
		types = append(types, Deno)
	}
	// Terraform: any .tf files
	if hasGlob(dir, "*.tf") {
		types = append(types, Terraform)
	}
	// Helm: Chart.yaml
	if exists(dir, "Chart.yaml") {
		types = append(types, Helm)
	}
	// Kustomize: kustomization.yaml
	if exists(dir, "kustomization.yaml") || exists(dir, "kustomization.yml") {
		types = append(types, Kustomize)
	}
	// Serverless: serverless.yml
	if exists(dir, "serverless.yml") || exists(dir, "serverless.yaml") {
		types = append(types, Serverless)
	}
	// Zig: build.zig
	if exists(dir, "build.zig") {
		types = append(types, Zig)
	}
	// Elixir: mix.exs
	if exists(dir, "mix.exs") {
		types = append(types, Elixir)
	}
	// Scala: build.sbt
	if exists(dir, "build.sbt") {
		types = append(types, Scala)
	}
	// Swift server-side: Package.swift without xcodeproj (not iOS)
	if exists(dir, "Package.swift") && !hasGlob(dir, "*.xcodeproj") && !hasGlob(dir, "*.xcworkspace") {
		types = append(types, SwiftServer)
	}
	// Dart server: pubspec.yaml without flutter dependency
	if exists(dir, "pubspec.yaml") && !hasFlutterDep(dir) {
		types = append(types, DartServer)
	}
	// Hugo: hugo.toml, hugo.yaml, or config.toml with content/ dir
	if exists(dir, "hugo.toml") || exists(dir, "hugo.yaml") || (exists(dir, "config.toml") && exists(dir, "content")) {
		types = append(types, Hugo)
	}
	// Astro: astro.config.*
	if hasGlob(dir, "astro.config.*") {
		types = append(types, Astro)
	}
	// Jekyll: _config.yml with Gemfile
	if exists(dir, "_config.yml") && exists(dir, "Gemfile") {
		types = append(types, Jekyll)
	}
	// C/Make: Makefile
	if exists(dir, "Makefile") {
		types = append(types, Make)
	}
	// C/CMake: CMakeLists.txt
	if exists(dir, "CMakeLists.txt") {
		types = append(types, CMake)
	}
	// Haskell: stack.yaml or *.cabal
	if exists(dir, "stack.yaml") || hasGlob(dir, "*.cabal") {
		types = append(types, Haskell)
	}
	// Lua: *.rockspec
	if hasGlob(dir, "*.rockspec") {
		types = append(types, Lua)
	}

	return types
}

// HasBun returns true if the Node project uses bun (bun.lock present).
func HasBun(dir string) bool {
	return exists(dir, "bun.lock")
}

// CLIInfo describes a detected CLI binary in the project.
type CLIInfo struct {
	Binary string // path to the binary (e.g., "./myapp", "./bin/cli")
}

// DetectCLI inspects the project directory for indicators that the project
// produces a CLI binary. It checks Go, Node, Python, and Rust conventions.
// Returns nil if no CLI binary is detected.
func DetectCLI(dir string, types []ProjectType) *CLIInfo {
	for _, t := range types {
		switch t {
		case Go:
			if info := detectGoCLI(dir); info != nil {
				return info
			}
		case Node:
			if info := detectNodeCLI(dir); info != nil {
				return info
			}
		case Python:
			if info := detectPythonCLI(dir); info != nil {
				return info
			}
		case Rust:
			if info := detectRustCLI(dir); info != nil {
				return info
			}
		}
	}
	return nil
}

// detectGoCLI checks for Go CLI indicators: cmd/ directory or main.go at root.
// Binary name is derived from the go.mod module path.
func detectGoCLI(dir string) *CLIInfo {
	hasCmd := exists(dir, "cmd")
	hasMain := exists(dir, "main.go")
	if !hasCmd && !hasMain {
		return nil
	}
	// Derive binary name from go.mod module path
	binary := filepath.Base(dir)
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				mod := strings.TrimPrefix(line, "module ")
				mod = strings.TrimSpace(mod)
				if idx := strings.LastIndex(mod, "/"); idx >= 0 {
					binary = mod[idx+1:]
				} else {
					binary = mod
				}
				break
			}
		}
	}
	return &CLIInfo{Binary: "./" + binary}
}

// detectNodeCLI checks for a "bin" field in package.json.
func detectNodeCLI(dir string) *CLIInfo {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	// "bin" can be a string or an object
	var pkg struct {
		Bin  json.RawMessage `json:"bin"`
		Name string          `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Bin == nil {
		return nil
	}
	// Check if bin is a non-empty value
	raw := strings.TrimSpace(string(pkg.Bin))
	if raw == "" || raw == "null" || raw == "{}" || raw == `""` {
		return nil
	}
	binary := pkg.Name
	if binary == "" {
		binary = filepath.Base(dir)
	}
	return &CLIInfo{Binary: binary}
}

// detectPythonCLI checks for [project.scripts] in pyproject.toml.
func detectPythonCLI(dir string) *CLIInfo {
	data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return nil
	}
	content := string(data)
	if !strings.Contains(content, "[project.scripts]") {
		return nil
	}
	// Derive binary name from project name if available
	binary := filepath.Base(dir)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"'`)
				if name != "" {
					binary = name
				}
			}
			break
		}
	}
	return &CLIInfo{Binary: binary}
}

// detectRustCLI checks for [[bin]] in Cargo.toml.
func detectRustCLI(dir string) *CLIInfo {
	data, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		return nil
	}
	content := string(data)
	// Check for explicit [[bin]] section or src/main.rs (default binary)
	hasBin := strings.Contains(content, "[[bin]]")
	hasMain := exists(dir, "src/main.rs")
	if !hasBin && !hasMain {
		return nil
	}
	// Derive binary name from [package] name
	binary := filepath.Base(dir)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"'`)
				if name != "" {
					binary = name
				}
			}
			break
		}
	}
	return &CLIInfo{Binary: "./" + binary}
}

// commonDocFiles lists documentation files that doc_integrity should check.
var commonDocFiles = []string{
	"README.md",
	"CLAUDE.md",
	"docs/USAGE.md",
	"SPEC.md",
}

// DetectDocFiles returns the subset of common documentation files that
// actually exist in the given directory.
func DetectDocFiles(dir string) []string {
	var found []string
	for _, f := range commonDocFiles {
		if exists(dir, f) {
			found = append(found, f)
		}
	}
	return found
}
