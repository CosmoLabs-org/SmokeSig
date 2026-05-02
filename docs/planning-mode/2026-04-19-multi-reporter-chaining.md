---
branch: multi-reporter-chaining
completed: "2026-05-02"
created: "2026-04-19"
goals_completed: 15
goals_total: 15
origin: migrated by ccs prompts migrate
priority: medium
related_prompts: []
requires_reading: []
schema_version: 1
status: COMPLETED
tags: []
title: Multi-Reporter Chaining Implementation Plan
---

# Multi-Reporter Chaining Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow `--format` to accept comma-separated values (e.g. `terminal,json,prometheus`) so tests run once and output to multiple reporters simultaneously.

**Architecture:** A `reporter.Chain()` factory parses the format string, creates reporters (first to stdout, rest to auto-named files), wraps them in the existing `MultiReporter`. The duplicated `switch format` blocks in `cmd/run.go` are replaced with a single `Chain()` call.

**Tech Stack:** Go, existing `internal/reporter` package, no new dependencies.

**Spec:** `docs/brainstorming/2026-04-19-multi-reporter-chaining.md`

---

## Chunk 1: Chain Factory

### Task 1: Create chain.go with Chain() function

**Files:**
- Create: `internal/reporter/chain.go`
- Test: `internal/reporter/chain_test.go`

- [x] **Step 1: Write failing tests for Chain()**

Create `internal/reporter/chain_test.go`:

```go
package reporter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChain_SingleFormat_NoFilesCreated(t *testing.T) {
	rep, closers, err := Chain("terminal", os.Stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers, got %d", len(closers))
	}
}

func TestChain_MultipleFormats_CreatesFiles(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	rep, closers, err := Chain("terminal,json", os.Stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep == nil {
		t.Fatal("expected non-nil reporter")
	}
	if len(closers) != 1 {
		t.Fatalf("expected 1 closer, got %d", len(closers))
	}
	if _, err := os.Stat(filepath.Join(tmp, "smoke-results.json")); err != nil {
		t.Fatalf("expected smoke-results.json to exist: %v", err)
	}
	for _, c := range closers {
		c.Close()
	}
}

func TestChain_DeduplicatesFormats(t *testing.T) {
	var buf bytes.Buffer
	rep, closers, err := Chain("json,json,json", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers (single format after dedup), got %d", len(closers))
	}
	_ = rep
}

func TestChain_UnknownFormat_ReturnsError(t *testing.T) {
	_, _, err := Chain("xml", os.Stdout)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Fatalf("error should mention unknown format: %v", err)
	}
}

func TestChain_EmptyFormat_ReturnsError(t *testing.T) {
	_, _, err := Chain("", os.Stdout)
	if err == nil {
		t.Fatal("expected error for empty format")
	}
}

func TestChain_CommasOnly_ReturnsError(t *testing.T) {
	_, _, err := Chain(",,,", os.Stdout)
	if err == nil {
		t.Fatal("expected error for commas-only format")
	}
}

func TestChain_CaseInsensitive(t *testing.T) {
	rep, closers, err := Chain("JSON", os.Stdout)
	if err != nil {
		t.Fatalf("case-insensitive match should work: %v", err)
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers for single format, got %d", len(closers))
	}
	_ = rep
}

func TestChain_WhitespaceTrimmed(t *testing.T) {
	var buf bytes.Buffer
	rep, closers, err := Chain(" json , terminal ", &buf)
	if err != nil {
		t.Fatalf("whitespace trimming should work: %v", err)
	}
	if len(closers) != 1 {
		t.Fatalf("expected 1 closer (terminal to file), got %d", len(closers))
	}
	_ = rep
	for _, c := range closers {
		c.Close()
	}
}

func TestChain_TrailingComma(t *testing.T) {
	rep, closers, err := Chain("json,", os.Stdout)
	if err != nil {
		t.Fatalf("trailing comma should be handled: %v", err)
	}
	if len(closers) != 0 {
		t.Fatalf("expected 0 closers, got %d", len(closers))
	}
	_ = rep
}

func TestChain_FileNaming(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	tests := []struct {
		format   string
		filename string
	}{
		{"json", "smoke-results.json"},
		{"junit", "smoke-junit.xml"},
		{"prometheus", "smoke-metrics.prom"},
		{"tap", "smoke-tap.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			_, closers, err := Chain("terminal,"+tc.format, os.Stdout)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			path := filepath.Join(tmp, tc.filename)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected %s to exist: %v", tc.filename, err)
			}
			for _, c := range closers {
				c.Close()
			}
		})
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/reporter/ -run TestChain_ -v`
Expected: compilation error — `Chain` undefined

- [x] **Step 3: Implement Chain() in chain.go**

Create `internal/reporter/chain.go`:

```go
package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var formatFile = map[string]string{
	"json":       "smoke-results.json",
	"junit":      "smoke-junit.xml",
	"prometheus": "smoke-metrics.prom",
	"tap":        "smoke-tap.txt",
	"terminal":   "smoke-output.txt",
}

var validFormats = map[string]bool{
	"terminal":   true,
	"json":       true,
	"junit":      true,
	"tap":        true,
	"prometheus": true,
}

// Chain parses a comma-separated format string, creates reporters for each
// format, and wraps them in a MultiReporter. The first format writes to
// stdout; subsequent formats write to auto-named files in the working
// directory. Returns the reporter, any opened files (creation order, close
// in reverse), or an error.
func Chain(format string, stdout io.Writer) (Reporter, []io.Closer, error) {
	names := parseFormats(format)
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("no output format specified")
	}

	for _, n := range names {
		if !validFormats[n] {
			return nil, nil, fmt.Errorf("unknown format %q (valid: terminal, json, junit, tap, prometheus)", n)
		}
	}

	var reporters []Reporter
	var closers []io.Closer

	for i, name := range names {
		var w io.Writer
		if i == 0 {
			w = stdout
		} else {
			f, err := os.Create(formatFile[name])
			if err != nil {
				for j := len(closers) - 1; j >= 0; j-- {
					closers[j].Close()
				}
				return nil, nil, fmt.Errorf("creating %s: %w", formatFile[name], err)
			}
			closers = append(closers, f)
			w = f
		}
		reporters = append(reporters, newReporter(name, w))
	}

	if len(reporters) == 1 {
		return reporters[0], closers, nil
	}
	return NewMultiReporter(reporters...), closers, nil
}

func parseFormats(format string) []string {
	parts := strings.Split(format, ",")
	var names []string
	seen := make(map[string]bool)
	for _, p := range parts {
		name := strings.ToLower(strings.TrimSpace(p))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func newReporter(name string, w io.Writer) Reporter {
	switch name {
	case "terminal":
		return NewTerminal(w)
	case "json":
		return NewJSON(w)
	case "junit":
		return NewJUnit(w)
	case "tap":
		return NewTAP(w)
	case "prometheus":
		return NewPrometheus(w)
	default:
		return NewTerminal(w)
	}
}
```

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/reporter/ -run TestChain_ -v`
Expected: all 9 tests PASS

- [x] **Step 5: Run full reporter test suite**

Run: `go test ./internal/reporter/ -v`
Expected: all existing + new tests PASS

---

## Chunk 2: Integrate into cmd/run.go

### Task 2: Refactor cmd/run.go to use Chain()

**Files:**
- Modify: `cmd/run.go:136-151` (monorepo reporter block)
- Modify: `cmd/run.go:203-217` (single-config reporter block)

- [x] **Step 1: Replace monorepo reporter block (lines 136-151)**

Remove the monorepo `// Create reporter early for monorepo mode` block through `rep = withPushReport(rep)` (lines 136-151). Replace with:

```go
	// Create reporter
	rep, closers, err := reporter.Chain(format, os.Stdout)
	if err != nil {
		return err
	}
	defer func() {
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i].Close(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: closing reporter: %v\n", err)
			}
		}
	}()
	rep = withOTelExport(rep, cfg)
	rep = withPushReport(rep)
```

- [x] **Step 2: Replace single-config reporter block (lines 203-217)**

Remove the `// Create reporter` block through `rep = withOTelExport(rep, cfg)` (lines 203-217). Replace with:

```go
	// Create reporter
	rep, closers, err := reporter.Chain(format, os.Stdout)
	if err != nil {
		return err
	}
	defer func() {
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i].Close(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: closing reporter: %v\n", err)
			}
		}
	}()
	rep = withOTelExport(rep, cfg)
```

- [x] **Step 3: Build and verify compilation**

Run: `go build ./...`
Expected: success

- [x] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all tests PASS

- [x] **Step 5: Manual smoke test — single format (backward compat)**

Run: `./smoke run --format terminal`
Expected: identical to previous behavior, no files created

- [x] **Step 6: Manual smoke test — multi-format**

Run: `./smoke run --format terminal,json && cat smoke-results.json | head -5`
Expected: terminal output on screen + valid JSON in `smoke-results.json`

- [x] **Step 7: Clean up test artifacts**

Run: `rm -f smoke-results.json`

---

## Chunk 3: Polish

### Task 3: Update help text and docs

**Files:**
- Modify: `cmd/run.go:86` (flag description)
- Modify: `CLAUDE.md` (Commands section)

- [x] **Step 1: Update --format flag description**

Change line 86:
```go
// Before:
runCmd.Flags().StringVar(&format, "format", "terminal", "Output format (terminal|json|junit|tap|prometheus)")

// After:
runCmd.Flags().StringVar(&format, "format", "terminal", "Output format(s), comma-separated (terminal,json,junit,tap,prometheus)")
```

- [x] **Step 2: Update CLAUDE.md Commands section**

In the `smoke run` command line, change `[--format terminal|json|junit|tap|prometheus]` to `[--format terminal,json,junit,tap,prometheus]`

- [x] **Step 3: Final test suite run**

Run: `go test ./...`
Expected: all tests PASS
