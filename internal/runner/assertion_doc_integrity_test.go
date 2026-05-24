package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestParseHelpCommands_CobraStyle(t *testing.T) {
	helpOutput := `myapp - A CLI tool

Usage:
  myapp [command]

Available Commands:
  serve       Start the server
  migrate     Run database migrations
  version     Show version info
  completion  Generate shell completions
  help        Help about any command

Flags:
  -h, --help      help for myapp
  -v, --verbose   verbose output

Use "myapp [command] --help" for more information about a command.
`

	commands := parseHelpCommands(helpOutput)

	expected := map[string]bool{
		"serve":      false,
		"migrate":    false,
		"version":    false,
		"completion": false,
		"help":       false,
	}

	for _, cmd := range commands {
		if _, ok := expected[cmd.Name]; ok {
			expected[cmd.Name] = true
		} else {
			t.Errorf("unexpected command found: %q", cmd.Name)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected command %q not found in parsed output", name)
		}
	}
}

func TestParseHelpCommands_EmptyOutput(t *testing.T) {
	commands := parseHelpCommands("")
	if len(commands) != 0 {
		t.Errorf("expected 0 commands from empty output, got %d", len(commands))
	}
}

func TestParseHelpCommands_NoCommandsSection(t *testing.T) {
	helpOutput := `Usage: simple-tool [options]

Options:
  --port     Port to listen on
  --config   Config file path
`

	commands := parseHelpCommands(helpOutput)
	if len(commands) != 0 {
		t.Errorf("expected 0 commands (no commands section), got %d", len(commands))
	}
}

func TestParseHelpFlags(t *testing.T) {
	helpOutput := `Usage:
  myapp serve [flags]

Flags:
  -h, --help            help for serve
      --port int        port to listen on (default 8080)
      --host string     host to bind to (default "localhost")
      --tls-cert file   TLS certificate file
      --tls-key file    TLS key file
      --timeout duration request timeout (default 30s)
`

	flags := parseHelpFlags(helpOutput)

	expected := map[string]bool{
		"help":     false,
		"port":     false,
		"host":     false,
		"tls-cert": false,
		"tls-key":  false,
		"timeout":  false,
	}

	for _, flag := range flags {
		if _, ok := expected[flag]; ok {
			expected[flag] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected flag %q not found in parsed output", name)
		}
	}
}

func TestParseHelpFlags_Empty(t *testing.T) {
	flags := parseHelpFlags("")
	if len(flags) != 0 {
		t.Errorf("expected 0 flags from empty output, got %d", len(flags))
	}
}

func TestExtractDocCommands(t *testing.T) {
	content := `# MyApp

## Usage

Run the server:

` + "```bash" + `
myapp serve --port 8080
myapp migrate --up
` + "```" + `

Use ` + "`myapp version`" + ` to check the version.

The ` + "`myapp completion`" + ` command generates shell completions.
`

	commands := make(map[string]bool)
	extractDocCommands(content, "myapp", commands)

	expected := []string{"serve", "migrate", "version", "completion"}
	for _, cmd := range expected {
		if !commands[cmd] {
			t.Errorf("expected command %q to be found in doc, but it was not", cmd)
		}
	}
}

func TestExtractDocFlags(t *testing.T) {
	content := `## Flags

- ` + "`--port`" + ` - Port to listen on
- ` + "`--host`" + ` - Host to bind to
- ` + "`--tls-cert`" + ` - TLS certificate file
- ` + "`--timeout`" + ` - Request timeout
`

	flags := make(map[string]bool)
	extractDocFlags(content, flags)

	expected := []string{"port", "host", "tls-cert", "timeout"}
	for _, flag := range expected {
		if !flags[flag] {
			t.Errorf("expected flag %q to be found in doc, but it was not", flag)
		}
	}
}

func TestExtractCodeExamples(t *testing.T) {
	content := `# Usage

` + "```bash" + `
myapp serve --port 8080
# This is a comment
myapp version
echo "not a myapp command"
` + "```" + `

` + "```" + `
myapp migrate --up
` + "```" + `
`

	examples := extractCodeExamples(content, "myapp")

	if len(examples) != 3 {
		t.Fatalf("expected 3 examples, got %d: %v", len(examples), examples)
	}

	if examples[0] != "myapp serve --port 8080" {
		t.Errorf("example[0] = %q, want %q", examples[0], "myapp serve --port 8080")
	}
	if examples[1] != "myapp version" {
		t.Errorf("example[1] = %q, want %q", examples[1], "myapp version")
	}
	if examples[2] != "myapp migrate --up" {
		t.Errorf("example[2] = %q, want %q", examples[2], "myapp migrate --up")
	}
}

func TestExtractCodeExamples_NoMatches(t *testing.T) {
	content := `# Usage

` + "```bash" + `
echo "hello world"
curl http://example.com
` + "```" + `
`

	examples := extractCodeExamples(content, "myapp")
	if len(examples) != 0 {
		t.Errorf("expected 0 examples, got %d: %v", len(examples), examples)
	}
}

func TestIsValidCommandName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"serve", true},
		{"my-command", true},
		{"my_command", true},
		{"cmd123", true},
		{"", false},
		{"-flag", false},
		{"123abc", false},
		{"has space", false},
	}

	for _, tt := range tests {
		if got := isValidCommandName(tt.name); got != tt.valid {
			t.Errorf("isValidCommandName(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

func TestCheckDocIntegrity_BinaryNotFound(t *testing.T) {
	check := &schema.DocIntegrityCheck{
		Binary: "/nonexistent/binary/path",
		Docs:   []string{"README.md"},
	}

	results := CheckDocIntegrity(check, t.TempDir())

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Error("expected failure for non-existent binary")
	}
	if !strings.Contains(results[0].Actual, "binary not found") {
		t.Errorf("expected 'binary not found' in actual, got %q", results[0].Actual)
	}
}

func TestCheckDocIntegrity_DocNotFound(t *testing.T) {
	dir := t.TempDir()

	// Create a fake binary that outputs help
	binaryPath := filepath.Join(dir, "testcli")
	script := "#!/bin/sh\necho 'Available Commands:'\necho '  serve   Start server'\n"
	os.WriteFile(binaryPath, []byte(script), 0755)

	check := &schema.DocIntegrityCheck{
		Binary: binaryPath,
		Docs:   []string{"nonexistent.md"},
	}

	results := CheckDocIntegrity(check, dir)

	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	foundDocErr := false
	for _, r := range results {
		if strings.Contains(r.Actual, "doc file not found") {
			foundDocErr = true
			break
		}
	}
	if !foundDocErr {
		t.Error("expected 'doc file not found' error in results")
	}
}

func TestCheckDocIntegrity_UndocumentedCommand(t *testing.T) {
	dir := t.TempDir()

	// Create a fake binary that lists commands
	binaryPath := filepath.Join(dir, "testcli")
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  echo "Available Commands:"
  echo "  serve       Start server"
  echo "  migrate     Run migrations"
  echo "  secret-cmd  Hidden command"
  echo ""
  echo "Flags:"
  echo "  --help   show help"
elif [ "$1" = "serve" ] && [ "$2" = "--help" ]; then
  echo "Usage: testcli serve"
  echo "Flags:"
  echo "  --port   port number"
elif [ "$1" = "migrate" ] && [ "$2" = "--help" ]; then
  echo "Usage: testcli migrate"
  echo "Flags:"
  echo "  --up   run up migrations"
elif [ "$1" = "secret-cmd" ] && [ "$2" = "--help" ]; then
  echo "Usage: testcli secret-cmd"
fi
`
	os.WriteFile(binaryPath, []byte(script), 0755)

	// Create a doc that only mentions serve and migrate
	docPath := filepath.Join(dir, "README.md")
	docContent := "# TestCLI\n\nUsage:\n\n```bash\ntestcli serve --port 8080\ntestcli migrate --up\n```\n"
	os.WriteFile(docPath, []byte(docContent), 0644)

	check := &schema.DocIntegrityCheck{
		Binary: binaryPath,
		Docs:   []string{"README.md"},
	}

	results := CheckDocIntegrity(check, dir)

	foundUndocumented := false
	for _, r := range results {
		if strings.Contains(r.Actual, "undocumented commands") && strings.Contains(r.Actual, "secret-cmd") {
			foundUndocumented = true
			break
		}
	}
	if !foundUndocumented {
		t.Error("expected 'undocumented commands' containing 'secret-cmd'")
		for _, r := range results {
			t.Logf("  result: passed=%v expected=%q actual=%q", r.Passed, r.Expected, r.Actual)
		}
	}
}

func TestCheckDocIntegrity_StaleReference(t *testing.T) {
	dir := t.TempDir()

	// Binary only has "serve"
	binaryPath := filepath.Join(dir, "testcli")
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  echo "Available Commands:"
  echo "  serve   Start server"
  echo ""
  echo "Flags:"
  echo "  --help   show help"
elif [ "$1" = "serve" ] && [ "$2" = "--help" ]; then
  echo "Usage: testcli serve"
fi
`
	os.WriteFile(binaryPath, []byte(script), 0755)

	// Doc mentions "serve" and "deploy" (which no longer exists)
	docPath := filepath.Join(dir, "README.md")
	docContent := "# TestCLI\n\n`testcli serve` starts the server.\n\n`testcli deploy` deploys the app.\n"
	os.WriteFile(docPath, []byte(docContent), 0644)

	check := &schema.DocIntegrityCheck{
		Binary: binaryPath,
		Docs:   []string{"README.md"},
	}

	results := CheckDocIntegrity(check, dir)

	foundStale := false
	for _, r := range results {
		if strings.Contains(r.Actual, "stale references") && strings.Contains(r.Actual, "deploy") {
			foundStale = true
			break
		}
	}
	if !foundStale {
		t.Error("expected 'stale references' containing 'deploy'")
		for _, r := range results {
			t.Logf("  result: passed=%v expected=%q actual=%q", r.Passed, r.Expected, r.Actual)
		}
	}
}

func TestCheckDocIntegrity_AllInSync(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "testcli")
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  echo "Available Commands:"
  echo "  serve   Start server"
  echo "  version Show version"
  echo ""
  echo "Flags:"
  echo "  --help   show help"
elif [ "$1" = "serve" ] && [ "$2" = "--help" ]; then
  echo "Usage: testcli serve"
  echo "Flags:"
  echo "  --port   port number"
elif [ "$1" = "version" ] && [ "$2" = "--help" ]; then
  echo "Usage: testcli version"
fi
`
	os.WriteFile(binaryPath, []byte(script), 0755)

	docPath := filepath.Join(dir, "README.md")
	docContent := "# TestCLI\n\n`testcli serve --port 8080` starts the server.\n\n`testcli version` shows version.\n"
	os.WriteFile(docPath, []byte(docContent), 0644)

	check := &schema.DocIntegrityCheck{
		Binary: binaryPath,
		Docs:   []string{"README.md"},
	}

	results := CheckDocIntegrity(check, dir)

	if len(results) != 1 {
		t.Fatalf("expected 1 result for in-sync docs, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected pass for in-sync docs, got fail: %q", results[0].Actual)
	}
}

func TestCheckDocIntegrity_IgnoreCommands(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "testcli")
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  echo "Available Commands:"
  echo "  serve      Start server"
  echo "  help       Help about commands"
  echo "  completion Generate completions"
  echo ""
fi
`
	os.WriteFile(binaryPath, []byte(script), 0755)

	docPath := filepath.Join(dir, "README.md")
	docContent := "# TestCLI\n\n`testcli serve` starts the server.\n"
	os.WriteFile(docPath, []byte(docContent), 0644)

	check := &schema.DocIntegrityCheck{
		Binary:         binaryPath,
		Docs:           []string{"README.md"},
		IgnoreCommands: []string{"help", "completion"},
	}

	results := CheckDocIntegrity(check, dir)

	// Should pass since help and completion are ignored
	if len(results) != 1 || !results[0].Passed {
		t.Error("expected pass when undocumented commands are in ignore list")
		for _, r := range results {
			t.Logf("  result: passed=%v expected=%q actual=%q", r.Passed, r.Expected, r.Actual)
		}
	}
}

func TestCheckDocIntegrity_CheckExamples(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "testcli")
	script := `#!/bin/sh
if [ "$1" = "version" ] && [ "$2" = "--help" ]; then
  echo "Usage: testcli version"
elif [ "$1" = "--help" ]; then
  echo "Available Commands:"
  echo "  version   Show version"
  echo ""
elif [ "$1" = "version" ]; then
  echo "v1.0.0"
  exit 0
fi
`
	os.WriteFile(binaryPath, []byte(script), 0755)

	// Put binary dir on PATH so sh -c can find testcli by name
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	docPath := filepath.Join(dir, "README.md")
	docContent := "# TestCLI\n\n```bash\ntestcli version\n```\n"
	os.WriteFile(docPath, []byte(docContent), 0644)

	check := &schema.DocIntegrityCheck{
		Binary:        binaryPath,
		Docs:          []string{"README.md"},
		CheckExamples: true,
	}

	results := CheckDocIntegrity(check, dir)

	// All should pass - version is documented and the example runs fine
	allPassed := true
	for _, r := range results {
		if !r.Passed {
			allPassed = false
			t.Logf("  failed: expected=%q actual=%q", r.Expected, r.Actual)
		}
	}
	if !allPassed {
		t.Error("expected all results to pass for valid examples")
	}
}

func TestParseHelpCommands_AlternateFormat(t *testing.T) {
	// Test with "Commands:" instead of "Available Commands:"
	helpOutput := `Usage: tool <command>

Commands:
  init       Initialize project
  build      Build the project
  test       Run tests

Global Flags:
  --verbose  Enable verbose mode
`

	commands := parseHelpCommands(helpOutput)

	names := make(map[string]bool)
	for _, cmd := range commands {
		names[cmd.Name] = true
	}

	for _, expected := range []string{"init", "build", "test"} {
		if !names[expected] {
			t.Errorf("expected command %q not found", expected)
		}
	}
}

func TestParseCommandName_SkipsFlags(t *testing.T) {
	name := parseCommandName("--verbose  Enable verbose output")
	if name != "" {
		t.Errorf("expected empty string for flag line, got %q", name)
	}
}
