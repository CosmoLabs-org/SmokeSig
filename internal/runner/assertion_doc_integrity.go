package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CLICommand represents a discovered CLI subcommand with its flags.
type CLICommand struct {
	Name  string
	Flags []string
}

// DocIntegrityResult holds the mismatch details from a doc integrity check.
type DocIntegrityResult struct {
	UndocumentedCommands []string
	StaleReferences      []string
	UndocumentedFlags    map[string][]string // command -> flags not in docs
	ExampleFailures      []string
}

// CheckDocIntegrity verifies CLI documentation stays in sync with actual commands and flags.
func CheckDocIntegrity(check *schema.DocIntegrityCheck, configDir string) []AssertionResult {
	var results []AssertionResult

	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Resolve binary path
	binary := check.Binary
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(configDir, binary)
	}

	// Check binary exists
	if _, err := os.Stat(binary); err != nil {
		results = append(results, AssertionResult{
			Type:     "doc_integrity",
			Expected: "binary exists",
			Actual:   fmt.Sprintf("binary not found: %s", check.Binary),
			Passed:   false,
		})
		return results
	}

	// Build ignore set
	ignoreSet := make(map[string]bool)
	for _, cmd := range check.IgnoreCommands {
		ignoreSet[cmd] = true
	}

	// Discover commands from --help
	commands, err := discoverCommands(binary, timeout)
	if err != nil {
		results = append(results, AssertionResult{
			Type:     "doc_integrity",
			Expected: "binary --help succeeds",
			Actual:   fmt.Sprintf("error running --help: %v", err),
			Passed:   false,
		})
		return results
	}

	// Filter ignored commands
	var filtered []CLICommand
	for _, cmd := range commands {
		if !ignoreSet[cmd.Name] {
			filtered = append(filtered, cmd)
		}
	}
	commands = filtered

	// Discover flags per subcommand
	for i, cmd := range commands {
		flags, err := discoverFlags(binary, cmd.Name, timeout)
		if err == nil {
			commands[i].Flags = flags
		}
	}

	// Parse documentation files
	docCommands, docFlags, err := parseDocFiles(check.Docs, check.Binary, configDir)
	if err != nil {
		results = append(results, AssertionResult{
			Type:     "doc_integrity",
			Expected: "doc files readable",
			Actual:   err.Error(),
			Passed:   false,
		})
		return results
	}

	// Build set of actual command names
	actualCommands := make(map[string]bool)
	for _, cmd := range commands {
		actualCommands[cmd.Name] = true
	}

	// Check for undocumented commands
	var undocumented []string
	for _, cmd := range commands {
		if !docCommands[cmd.Name] {
			undocumented = append(undocumented, cmd.Name)
		}
	}
	sort.Strings(undocumented)

	if len(undocumented) > 0 {
		results = append(results, AssertionResult{
			Type:     "doc_integrity",
			Expected: "all commands documented",
			Actual:   fmt.Sprintf("undocumented commands: %s", strings.Join(undocumented, ", ")),
			Passed:   false,
		})
	}

	// Check for stale command references in docs
	var stale []string
	for ref := range docCommands {
		if !actualCommands[ref] && !ignoreSet[ref] {
			stale = append(stale, ref)
		}
	}
	sort.Strings(stale)

	if len(stale) > 0 {
		results = append(results, AssertionResult{
			Type:     "doc_integrity",
			Expected: "no stale command references",
			Actual:   fmt.Sprintf("stale references: %s", strings.Join(stale, ", ")),
			Passed:   false,
		})
	}

	// Check for undocumented flags per command
	var undocFlagMsgs []string
	for _, cmd := range commands {
		if len(cmd.Flags) == 0 {
			continue
		}
		for _, flag := range cmd.Flags {
			if !docFlags[flag] {
				undocFlagMsgs = append(undocFlagMsgs, fmt.Sprintf("%s: --%s", cmd.Name, flag))
			}
		}
	}
	sort.Strings(undocFlagMsgs)

	if len(undocFlagMsgs) > 0 {
		// Limit output to avoid overwhelming reports
		display := undocFlagMsgs
		if len(display) > 20 {
			display = display[:20]
			display = append(display, fmt.Sprintf("... and %d more", len(undocFlagMsgs)-20))
		}
		results = append(results, AssertionResult{
			Type:     "doc_integrity",
			Expected: "all flags documented",
			Actual:   fmt.Sprintf("undocumented flags: %s", strings.Join(display, "; ")),
			Passed:   false,
		})
	}

	// Check examples if requested
	if check.CheckExamples {
		failures := checkDocExamples(check.Docs, check.Binary, configDir, timeout)
		if len(failures) > 0 {
			results = append(results, AssertionResult{
				Type:     "doc_integrity",
				Expected: "all doc examples exit 0",
				Actual:   fmt.Sprintf("example failures: %s", strings.Join(failures, "; ")),
				Passed:   false,
			})
		}
	}

	// If no issues found, report pass
	if len(results) == 0 {
		results = append(results, AssertionResult{
			Type:     "doc_integrity",
			Expected: "docs in sync with CLI",
			Actual:   fmt.Sprintf("docs in sync (%d commands, %d doc files)", len(commands), len(check.Docs)),
			Passed:   true,
		})
	}

	return results
}

// discoverCommands runs `binary --help` and parses the output for subcommand names.
func discoverCommands(binary string, timeout time.Duration) ([]CLICommand, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// --help may exit non-zero in some tools; we still want the output
	cmd.Run()

	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}
	if output == "" {
		return nil, fmt.Errorf("no output from %s --help", binary)
	}

	return parseHelpCommands(output), nil
}

// parseHelpCommands extracts subcommand names from --help output.
// Handles Cobra-style "Available Commands:" and similar patterns.
func parseHelpCommands(helpOutput string) []CLICommand {
	var commands []CLICommand
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(helpOutput))
	inCommandSection := false

	for scanner.Scan() {
		line := scanner.Text()

		// Detect command sections (Cobra uses "Available Commands:", others may vary)
		lower := strings.ToLower(strings.TrimSpace(line))
		if lower == "available commands:" || lower == "commands:" ||
			strings.HasSuffix(lower, "commands:") {
			inCommandSection = true
			continue
		}

		// End of command section: empty line or a new section header (e.g., "Flags:")
		if inCommandSection {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				inCommandSection = false
				continue
			}
			// Another section header
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.HasSuffix(trimmed, ":") {
				inCommandSection = false
				continue
			}

			// Parse "  command_name   Description text"
			name := parseCommandName(trimmed)
			if name != "" && !seen[name] {
				seen[name] = true
				commands = append(commands, CLICommand{Name: name})
			}
		}
	}

	return commands
}

// parseCommandName extracts the command name from an indented help line.
func parseCommandName(line string) string {
	// Skip lines that look like flags
	if strings.HasPrefix(line, "-") {
		return ""
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}

	name := fields[0]
	// Command names should be alphanumeric with hyphens/underscores
	if isValidCommandName(name) {
		return name
	}
	return ""
}

// isValidCommandName checks if a string looks like a CLI command name.
func isValidCommandName(s string) bool {
	if s == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z][a-zA-Z0-9_-]*$`, s)
	return matched
}

// discoverFlags runs `binary cmd --help` and parses the output for flag names.
func discoverFlags(binary, command string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, command, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}
	if output == "" {
		return nil, fmt.Errorf("no output from %s %s --help", binary, command)
	}

	return parseHelpFlags(output), nil
}

// parseHelpFlags extracts flag names from --help output.
func parseHelpFlags(helpOutput string) []string {
	var flags []string
	seen := make(map[string]bool)

	// Match --flag-name patterns in the help output
	flagRe := regexp.MustCompile(`--([a-zA-Z][a-zA-Z0-9_-]*)`)
	matches := flagRe.FindAllStringSubmatch(helpOutput, -1)

	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			flags = append(flags, name)
		}
	}

	return flags
}

// parseDocFiles reads documentation files and extracts command and flag references.
func parseDocFiles(docs []string, binaryName string, configDir string) (commands map[string]bool, flags map[string]bool, err error) {
	commands = make(map[string]bool)
	flags = make(map[string]bool)

	// Extract the base binary name for matching
	baseName := filepath.Base(binaryName)

	for _, doc := range docs {
		path := doc
		if !filepath.IsAbs(path) {
			path = filepath.Join(configDir, path)
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, nil, fmt.Errorf("doc file not found: %s", doc)
		}

		content := string(data)

		// Extract command references: `binary cmd`, binary cmd (in code blocks), etc.
		extractDocCommands(content, baseName, commands)

		// Extract flag references: --flag-name
		extractDocFlags(content, flags)
	}

	return commands, flags, nil
}

// extractDocCommands finds command references in documentation text.
func extractDocCommands(content, binaryName string, commands map[string]bool) {
	// Pattern 1: `binary cmd` or `binary cmd ...` in backticks
	backtickRe := regexp.MustCompile("`" + regexp.QuoteMeta(binaryName) + `\s+([a-zA-Z][a-zA-Z0-9_-]*)`)
	for _, m := range backtickRe.FindAllStringSubmatch(content, -1) {
		commands[m[1]] = true
	}

	// Pattern 2: bare "binary cmd" patterns in code blocks or plain text
	bareRe := regexp.MustCompile(`(?:^|\s)` + regexp.QuoteMeta(binaryName) + `\s+([a-zA-Z][a-zA-Z0-9_-]*)`)
	for _, m := range bareRe.FindAllStringSubmatch(content, -1) {
		commands[m[1]] = true
	}

	// Pattern 3: commands in markdown headings that explicitly reference the binary
	// e.g., "### binary cmd" but NOT general headings like "# ProjectName"
	headingRe := regexp.MustCompile(`(?m)^#+\s+` + regexp.QuoteMeta(binaryName) + `\s+([a-zA-Z][a-zA-Z0-9_-]*)`)
	for _, m := range headingRe.FindAllStringSubmatch(content, -1) {
		commands[m[1]] = true
	}
}

// extractDocFlags finds flag references (--flag-name) in documentation text.
func extractDocFlags(content string, flags map[string]bool) {
	flagRe := regexp.MustCompile(`--([a-zA-Z][a-zA-Z0-9_-]*)`)
	for _, m := range flagRe.FindAllStringSubmatch(content, -1) {
		flags[m[1]] = true
	}
}

// checkDocExamples extracts fenced code blocks starting with the binary name and runs them.
func checkDocExamples(docs []string, binaryName string, configDir string, timeout time.Duration) []string {
	var failures []string
	baseName := filepath.Base(binaryName)

	for _, doc := range docs {
		path := doc
		if !filepath.IsAbs(path) {
			path = filepath.Join(configDir, path)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		examples := extractCodeExamples(string(data), baseName)
		for _, example := range examples {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			cmd := exec.CommandContext(ctx, "sh", "-c", example)
			cmd.Dir = configDir

			if err := cmd.Run(); err != nil {
				short := example
				if len(short) > 60 {
					short = short[:60] + "..."
				}
				failures = append(failures, fmt.Sprintf("%s: %q exit non-zero", filepath.Base(doc), short))
			}
			cancel()
		}
	}

	return failures
}

// extractCodeExamples extracts executable code blocks from markdown that start with the binary name.
func extractCodeExamples(content, binaryName string) []string {
	var examples []string

	// Match fenced code blocks: ```bash or ``` followed by content
	codeBlockRe := regexp.MustCompile("(?s)```(?:bash|sh|shell)?\\s*\\n(.*?)```")
	blocks := codeBlockRe.FindAllStringSubmatch(content, -1)

	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block[1]), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Skip empty lines, comments
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Only include lines that start with the binary name
			if strings.HasPrefix(trimmed, binaryName+" ") || trimmed == binaryName {
				examples = append(examples, trimmed)
			}
		}
	}

	return examples
}
