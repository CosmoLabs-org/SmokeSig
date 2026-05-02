---
date: 2026-05-02
status: plan
issue: FEAT-044
brainstorm: docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
deliverables:
  - id: P-01
    title: "stress result types, error dedup, reliability scoring"
  - id: P-02
    title: "worker pool stress execution with atomic counters"
  - id: P-03
    title: "stress command Cobra wiring and flags"
  - id: P-04
    title: "terminal summary output formatting"
  - id: P-05
    title: "edge cases — fail-fast, config dir, final verification"
---

# Flakiness Detector — smoke stress (FEAT-044 / BR-02)

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `smoke stress <test-name>` command that runs a single test N times with configurable parallelism, reports pass rate and timing distribution.

**Architecture:** New `cmd/stress.go` wires a Cobra command. New `internal/runner/stress.go` implements a bounded worker pool with `sync/atomic` counters, error deduplication, and reliability scoring. Reuses existing `Runner.runTestOnce()` for each iteration. Terminal output via fmt (no new deps needed beyond existing Lipgloss).

**Tech Stack:** Go stdlib (`sync`, `sync/atomic`, `time`), existing Cobra/Lipgloss deps. No new dependencies.

---

## File Structure

| File | Responsibility |
|------|---------------|
| `cmd/stress.go` | Cobra command, flags, config loading, orchestration |
| `cmd/stress_test.go` | Tests for CLI arg parsing and flag validation |
| `internal/runner/stress.go` | Worker pool, result collection, error dedup, reliability scoring |
| `internal/runner/stress_test.go` | Tests for stress engine logic |

---

## Chunk 1: Stress Engine Core

### Task 1: Stress result types and error dedup

**Files:**
- Create: `internal/runner/stress.go`
- Test: `internal/runner/stress_test.go`

- [ ] **Step 1: Write failing tests for StressResult and error dedup**

```go
package runner

import (
	"testing"
)

func TestDedupErrors_GroupsIdentical(t *testing.T) {
	errors := []string{
		"exit_code expected 0, got 1",
		"exit_code expected 0, got 1",
		"exit_code expected 0, got 1",
		"stdout: missing \"database connected\"",
	}
	grouped := DedupErrors(errors)
	if len(grouped) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(grouped))
	}
	if grouped[0].Count != 3 {
		t.Errorf("expected first group count 3, got %d", grouped[0].Count)
	}
	if grouped[0].Message != "exit_code expected 0, got 1" {
		t.Errorf("unexpected first group message: %s", grouped[0].Message)
	}
}

func TestDedupErrors_Empty(t *testing.T) {
	grouped := DedupErrors(nil)
	if len(grouped) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(grouped))
	}
}

func TestReliabilityStatus(t *testing.T) {
	tests := []struct {
		rate     float64
		expected string
	}{
		{100.0, "Stable"},
		{99.0, "Flaky"},
		{95.0, "Flaky"},
		{94.9, "Unreliable"},
		{50.0, "Unreliable"},
		{0.0, "Unreliable"},
	}
	for _, tt := range tests {
		got := ReliabilityStatus(tt.rate)
		if got != tt.expected {
			t.Errorf("ReliabilityStatus(%.1f) = %q, want %q", tt.rate, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/ -run "TestDedupErrors|TestReliabilityStatus" -v`
Expected: FAIL (functions not defined)

- [ ] **Step 3: Implement types and functions**

```go
package runner

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

// StressResult holds aggregate results from a stress test run.
type StressResult struct {
	TestName      string
	TotalRuns     int
	Passes        int
	Failures      int
	PassRate      float64
	Duration      time.Duration
	Concurrency   int
	Reliability   string // Stable, Flaky, Unreliable
	ErrorGroups   []ErrorGroup
	RunDurations  []time.Duration
}

// ErrorGroup holds a deduplicated error with its occurrence count.
type ErrorGroup struct {
	Message string
	Count   int
}

// DedupErrors groups identical error messages and sorts by frequency (descending).
func DedupErrors(errors []string) []ErrorGroup {
	counts := make(map[string]int)
	for _, e := range errors {
		counts[e]++
	}
	groups := make([]ErrorGroup, 0, len(counts))
	for msg, cnt := range counts {
		groups = append(groups, ErrorGroup{Message: msg, Count: cnt})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Count > groups[j].Count
	})
	return groups
}

// ReliabilityStatus returns a human-readable reliability label.
func ReliabilityStatus(passRate float64) string {
	switch {
	case passRate >= 100.0:
		return "Stable"
	case passRate >= 95.0:
		return "Flaky"
	default:
		return "Unreliable"
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/ -run "TestDedupErrors|TestReliabilityStatus" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runner/stress.go internal/runner/stress_test.go
git commit -m "feat(FEAT-044): stress result types, error dedup, reliability scoring"
```

---

### Task 2: Worker pool and stress execution

**Files:**
- Modify: `internal/runner/stress.go`
- Modify: `internal/runner/stress_test.go`

- [ ] **Step 1: Write failing tests for StressTest function**

```go
func TestStressTest_AllPass(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "always-passes", Run: "true"},
			},
		},
		ConfigDir: "",
	}
	result := r.StressTest("always-passes", 10, 1, false)
	if result.TotalRuns != 10 {
		t.Errorf("expected 10 runs, got %d", result.TotalRuns)
	}
	if result.Passes != 10 {
		t.Errorf("expected 10 passes, got %d", result.Passes)
	}
	if result.Failures != 0 {
		t.Errorf("expected 0 failures, got %d", result.Failures)
	}
	if result.PassRate != 100.0 {
		t.Errorf("expected 100%% pass rate, got %.1f%%", result.PassRate)
	}
	if result.Reliability != "Stable" {
		t.Errorf("expected Stable, got %s", result.Reliability)
	}
}

func TestStressTest_WithFailures(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "sometimes-fails", Run: "sh -c 'exit $((RANDOM % 3))'"},
			},
		},
		ConfigDir: "",
	}
	result := r.StressTest("sometimes-fails", 20, 1, false)
	if result.TotalRuns != 20 {
		t.Errorf("expected 20 runs, got %d", result.TotalRuns)
	}
	if result.Passes+result.Failures != 20 {
		t.Errorf("passes(%d) + failures(%d) != 20", result.Passes, result.Failures)
	}
}

func TestStressTest_TestNotFound(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{Tests: nil},
	}
	result := r.StressTest("nonexistent", 5, 1, false)
	if result.TotalRuns != 0 {
		t.Errorf("expected 0 runs for missing test, got %d", result.TotalRuns)
	}
}

func TestStressTest_Concurrent(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "pass", Run: "true"},
			},
		},
	}
	result := r.StressTest("pass", 20, 5, false)
	if result.Concurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", result.Concurrency)
	}
	if result.Passes != 20 {
		t.Errorf("expected 20 passes, got %d", result.Passes)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/ -run "TestStressTest_" -v`
Expected: FAIL (StressTest method not defined)

- [ ] **Step 3: Implement StressTest method**

Add to `internal/runner/stress.go`:

```go
// StressTest runs a single test N times with configurable concurrency.
// Returns aggregate results including pass rate, timing, and deduplicated errors.
func (r *Runner) StressTest(testName string, runs, workers int, failFast bool) StressResult {
	var target *schema.Test
	for i := range r.Config.Tests {
		if r.Config.Tests[i].Name == testName {
			target = &r.Config.Tests[i]
			break
		}
	}
	if target == nil {
		return StressResult{TestName: testName}
	}

	var (
		passes   atomic.Int64
		failures atomic.Int64
		mu       sync.Mutex
		errors   []string
		durations []time.Duration
	)

	if len(r.Config.Lifecycle.BeforeAll) > 0 {
		RunLifecycleHooks(context.Background(), r.Config.Lifecycle.BeforeAll, nil, r.ConfigDir)
	}
	defer func() {
		if len(r.Config.Lifecycle.AfterAll) > 0 {
			RunLifecycleHooks(context.Background(), r.Config.Lifecycle.AfterAll, nil, r.ConfigDir)
		}
		CleanupBackgroundProcesses()
	}()

	start := time.Now()
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := 0; i < runs; i++ {
		if failFast && failures.Load() > 0 {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			opts := RunOptions{}
			tr := r.runTestOnce(*target, opts)
			if tr.Passed {
				passes.Add(1)
			} else {
				failures.Add(1)
				errMsg := testErrorMessage(tr)
				if errMsg != "" {
					mu.Lock()
					errors = append(errors, errMsg)
					mu.Unlock()
				}
			}
			mu.Lock()
			durations = append(durations, tr.Duration)
			mu.Unlock()
		}()
	}
	wg.Wait()

	totalRuns := int(passes.Load() + failures.Load())
	var passRate float64
	if totalRuns > 0 {
		passRate = float64(passes.Load()) / float64(totalRuns) * 100.0
	}

	return StressResult{
		TestName:     testName,
		TotalRuns:    totalRuns,
		Passes:       int(passes.Load()),
		Failures:     int(failures.Load()),
		PassRate:     passRate,
		Duration:     time.Since(start),
		Concurrency:  workers,
		Reliability:  ReliabilityStatus(passRate),
		ErrorGroups:  DedupErrors(errors),
		RunDurations: durations,
	}
}

// testErrorMessage extracts a human-readable error from a TestResult.
func testErrorMessage(tr TestResult) string {
	for _, a := range tr.Assertions {
		if !a.Passed {
			return fmt.Sprintf("%s: expected %s, got %s", a.Type, a.Expected, a.Actual)
		}
	}
	if tr.Error != nil {
		return tr.Error.Error()
	}
	return ""
}
```

Add `"context"` to imports in `internal/runner/stress.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/ -run "TestStressTest_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runner/stress.go internal/runner/stress_test.go
git commit -m "feat(FEAT-044): worker pool stress execution with atomic counters"
```

---

## Chunk 2: CLI Command and Output

### Task 3: Cobra command wiring

**Files:**
- Create: `cmd/stress.go`
- Create: `cmd/stress_test.go`

- [ ] **Step 1: Write failing tests for stress command flags**

```go
package cmd

import (
	"testing"
)

func TestStressCmd_Flags(t *testing.T) {
	if !stressCmd.HasFlags() {
		t.Fatal("stress command should have flags")
	}
	runs, err := stressCmd.Flags().GetInt("runs")
	if err != nil || runs != 50 {
		t.Errorf("expected --runs default 50, got %d, err %v", runs, err)
	}
	workers, err := stressCmd.Flags().GetInt("workers")
	if err != nil || workers != 1 {
		t.Errorf("expected --workers default 1, got %d, err %v", workers, err)
	}
}

func TestStressCmd_RequiresTestName(t *testing.T) {
	err := stressCmd.Args(stressCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no test name provided")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run "TestStressCmd_" -v`
Expected: FAIL (stressCmd not defined)

- [ ] **Step 3: Implement stress command**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/reporter"
	"github.com/CosmoLabs-org/cosmo-smoke/internal/runner"
	"github.com/spf13/cobra"
)

var (
	stressRuns     int
	stressWorkers  int
	stressFailFast bool
)

var stressCmd = &cobra.Command{
	Use:   "stress <test-name>",
	Short: "Run a single test repeatedly to detect flakiness",
	Long:  "Run a single smoke test N times with configurable parallelism.\nReports pass rate, timing distribution, and deduplicated errors.",
	Args:  cobra.ExactArgs(1),
	RunE:  runStress,
}

func init() {
	rootCmd.AddCommand(stressCmd)
	stressCmd.Flags().IntVar(&stressRuns, "runs", 50, "Total number of executions")
	stressCmd.Flags().IntVar(&stressWorkers, "workers", 1, "Concurrency (1 = sequential)")
	stressCmd.Flags().BoolVar(&stressFailFast, "fail-fast", false, "Stop on first failure")
	stressCmd.Flags().StringVarP(&configFile, "file", "f", ".smoke.yaml", "Config file path")
	stressCmd.Flags().StringVar(&format, "format", "terminal", "Output format (terminal, json)")
}

func runStress(cmd *cobra.Command, args []string) error {
	testName := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	rep, closeAll, err := buildReporter(format, cfg)
	if err != nil {
		return err
	}
	defer closeAll()

	configDir := filepath.Dir(configFile)
	if configDir == "." {
		configDir = ""
	}

	r := &runner.Runner{
		Config:    cfg,
		Reporter:  rep,
		ConfigDir: configDir,
	}

	result := r.StressTest(testName, stressRuns, stressWorkers, stressFailFast)
	if result.TotalRuns == 0 {
		return fmt.Errorf("test %q not found in %s", testName, configFile)
	}

	fmt.Fprintln(os.Stdout, formatStressSummary(result, cfg.Project))

	reportStressResult(rep, result, cfg.Project)

	if result.PassRate < 100.0 {
		os.Exit(1)
	}
	return nil
}

func reportStressResult(rep reporter.Reporter, result runner.StressResult, project string) {
	for i := 0; i < result.Passes; i++ {
		rep.TestResult(reporter.TestResultData{
			Name:   fmt.Sprintf("%s (run %d)", result.TestName, i+1),
			Passed: true,
		})
	}
	rep.Summary(reporter.SuiteResultData{
		Project:  project,
		Total:    result.TotalRuns,
		Passed:   result.Passes,
		Failed:   result.Failures,
		Duration: result.Duration,
	})
}

func formatStressSummary(r runner.StressResult, project string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n  Stress Test Complete: %s\n", r.TestName)
	fmt.Fprintf(&b, "  %s\n", strings.Repeat("-", 50))
	fmt.Fprintf(&b, "  Total Runs:    %d\n", r.TotalRuns)
	fmt.Fprintf(&b, "  Concurrency:   %d workers\n", r.Concurrency)
	fmt.Fprintf(&b, "  Duration:      %s\n", r.Duration.Round(time.Millisecond))
	fmt.Fprintf(&b, "  Reliability:   %.0f%% (%s)\n", r.PassRate, r.Reliability)
	fmt.Fprintf(&b, "  Passed:        %d/%d\n", r.Passes, r.TotalRuns)

	if len(r.ErrorGroups) > 0 {
		fmt.Fprintf(&b, "\n  Failures:\n")
		for _, eg := range r.ErrorGroups {
			fmt.Fprintf(&b, "    - [%d times]: %s\n", eg.Count, eg.Message)
		}
	}

	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run "TestStressCmd_" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/stress.go cmd/stress_test.go
git commit -m "feat(FEAT-044): stress command with Cobra flags, summary output, and reporter"
```

---

## Chunk 3: Edge Cases and Final Verification

### Task 4: Edge case tests

**Files:**
- Modify: `internal/runner/stress_test.go`

- [ ] **Step 1: Write edge case tests**

```go
func TestStressTest_FailFast(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "always-fails", Run: "false"},
			},
		},
	}
	result := r.StressTest("always-fails", 100, 1, true)
	if result.TotalRuns != 1 {
		t.Errorf("expected 1 run with fail-fast, got %d", result.TotalRuns)
	}
	if result.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", result.Failures)
	}
}

func TestStressTest_AllowsFailure(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Tests: []schema.Test{
				{Name: "pass", Run: "true", AllowFailure: true},
			},
		},
	}
	result := r.StressTest("pass", 5, 1, false)
	if result.Passes != 5 {
		t.Errorf("expected 5 passes, got %d", result.Passes)
	}
}
```

- [ ] **Step 2: Run all stress tests**

Run: `go test ./internal/runner/ -run "TestStressTest_|TestDedupErrors|TestReliabilityStatus" -v`
Expected: All pass

- [ ] **Step 3: Run full suite to check for regressions**

Run: `go test ./... -count=1`
Expected: All 1023+ tests pass

- [ ] **Step 4: Build and verify binary**

Run: `go build -o smoke . && ./smoke stress --help`
Expected: Help output showing flags (--runs, --workers, --fail-fast, --file, --format)

- [ ] **Step 5: Commit**

```bash
git add internal/runner/stress_test.go
git commit -m "test(FEAT-044): edge cases — fail-fast, allow-failure"
```

- [ ] **Step 6: Update issues and changelog**

```bash
ccs issues update FEAT-044 --status done
ccs changelog add "FEAT-044: Flakiness detector — smoke stress command" --type added
```
