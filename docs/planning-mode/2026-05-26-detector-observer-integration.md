---
brainstorm: docs/brainstorming/2026-05-26-detector-observer-integration.md
completed: "2026-05-27"
created: "2026-05-26T00:20:00-03:00"
deliverables:
    - id: P-01
      title: StackHints type and HintsFromDir with portless reader + stack table
    - id: P-02
      title: ProbeEndpoints accepts extra paths
    - id: P-03
      title: Observer wires hints into observation pipeline
    - id: P-04
      title: cmd/observe auto-detects and passes hints
goals_completed: 24
goals_total: 24
issue: FEAT-046
related_prompts: []
requires_reading: []
schema_version: 1
status: COMPLETED
tags: []
title: 'FEAT-046: Detector-Observer Integration — Implementation Plan'
---

# FEAT-046: Detector-Observer Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `smokesig observe` stack-aware by detecting the project type and using hints to tune port detection and HTTP probing.

**Architecture:** New `hints.go` file in `internal/observer/` reads `portless.json` (CCS SOP) then falls back to `detector.Detect()` for stack-type heuristics. Hints flow into the existing `ProbeEndpoints()` as extra probe paths. Observer calls `HintsFromDir()` automatically when a directory is set.

**Tech Stack:** Go, existing `internal/detector` package, `encoding/json` for portless parsing.

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/observer/hints.go` | Create | StackHints struct, HintsFromDir(), portless reader, stack hint table |
| `internal/observer/hints_test.go` | Create | Tests for portless parsing, stack detection, fallback, merge |
| `internal/observer/probes.go` | Modify | Accept `extraPaths` parameter |
| `internal/observer/probes_test.go` | Modify | Test extra paths behavior |
| `internal/observer/observer.go` | Modify | Call HintsFromDir, pass extra paths to ProbeEndpoints |
| `cmd/observe.go` | Modify | Wire auto-detection when dir is set |

---

### Task 1: StackHints type and portless reader (P-01)

**Files:**
- Create: `internal/observer/hints.go`
- Create: `internal/observer/hints_test.go`

- [x] **Step 1: Write failing tests for portless parsing**

```go
// hints_test.go
package observer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPortlessJSON_FlatFormat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"name":"test","port":4650,"domain":"test.cosmo"}`), 0644)
	port := readPortlessPort(dir)
	if port != 4650 {
		t.Errorf("got %d, want 4650", port)
	}
}

func TestReadPortlessJSON_AppsFormat_NoPort(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"apps":{"web":{"name":"test.cosmo"}}}`), 0644)
	port := readPortlessPort(dir)
	if port != 0 {
		t.Errorf("got %d, want 0 (apps format has no port)", port)
	}
}

func TestReadPortlessJSON_Missing(t *testing.T) {
	dir := t.TempDir()
	port := readPortlessPort(dir)
	if port != 0 {
		t.Errorf("got %d, want 0", port)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/observer/ -run TestReadPortless -v`
Expected: FAIL — `readPortlessPort` undefined

- [x] **Step 3: Implement portless reader**

```go
// hints.go
package observer

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/CosmoLabs-org/SmokeSig/internal/detector"
)

type StackHints struct {
	ExpectedPorts   []int
	ExtraProbePaths []string
}

func readPortlessPort(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, "portless.json"))
	if err != nil {
		return 0
	}
	var cfg struct {
		Port int `json:"port"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0
	}
	return cfg.Port
}
```

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/observer/ -run TestReadPortless -v`
Expected: PASS (3 tests)

- [x] **Step 5: Write failing tests for HintsFromDir**

```go
func TestHintsFromDir_PortlessOverridesStack(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"port":4650}`), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	hints := HintsFromDir(dir)
	if len(hints.ExpectedPorts) == 0 || hints.ExpectedPorts[0] != 4650 {
		t.Errorf("portless port should be first, got %v", hints.ExpectedPorts)
	}
}

func TestHintsFromDir_GoProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	hints := HintsFromDir(dir)
	if len(hints.ExpectedPorts) == 0 {
		t.Fatal("expected Go default ports")
	}
	found8080 := false
	for _, p := range hints.ExpectedPorts {
		if p == 8080 {
			found8080 = true
		}
	}
	if !found8080 {
		t.Errorf("Go project should hint port 8080, got %v", hints.ExpectedPorts)
	}
}

func TestHintsFromDir_NodeProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	hints := HintsFromDir(dir)
	found3000 := false
	for _, p := range hints.ExpectedPorts {
		if p == 3000 {
			found3000 = true
		}
	}
	if !found3000 {
		t.Errorf("Node project should hint port 3000, got %v", hints.ExpectedPorts)
	}
}

func TestHintsFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	hints := HintsFromDir(dir)
	if len(hints.ExpectedPorts) != 0 {
		t.Errorf("empty dir should have no port hints, got %v", hints.ExpectedPorts)
	}
}

func TestHintsFromDir_PortlessMergesWithStack(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "portless.json"), []byte(`{"port":4650}`), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	hints := HintsFromDir(dir)
	if len(hints.ExpectedPorts) < 2 {
		t.Errorf("should have portless + Go defaults, got %v", hints.ExpectedPorts)
	}
	if len(hints.ExtraProbePaths) == 0 {
		t.Error("Go project should have extra probe paths")
	}
}
```

- [x] **Step 6: Implement HintsFromDir with stack table**

```go
var stackHints = map[detector.ProjectType]StackHints{
	detector.Go:          {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/metrics", "/debug/pprof"}},
	detector.Node:        {ExpectedPorts: []int{3000}, ExtraProbePaths: []string{"/api", "/graphql"}},
	detector.Python:      {ExpectedPorts: []int{8000}, ExtraProbePaths: []string{"/admin", "/api"}},
	detector.ReactNative: {ExpectedPorts: []int{8081}, ExtraProbePaths: []string{"/status"}},
	detector.Rust:        {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/api"}},
	detector.Java:        {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/actuator/health"}},
	detector.JavaGradle:  {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/actuator/health"}},
	detector.DotNet:      {ExpectedPorts: []int{5000}, ExtraProbePaths: []string{"/swagger"}},
	detector.Ruby:        {ExpectedPorts: []int{3000}, ExtraProbePaths: []string{"/api"}},
	detector.PHP:         {ExpectedPorts: []int{8000}, ExtraProbePaths: []string{"/api"}},
	detector.Elixir:      {ExpectedPorts: []int{4000}, ExtraProbePaths: []string{"/api"}},
	detector.Deno:        {ExpectedPorts: []int{8000}, ExtraProbePaths: []string{"/api"}},
	detector.Hugo:        {ExpectedPorts: []int{1313}, ExtraProbePaths: []string{"/"}},
	detector.Astro:       {ExpectedPorts: []int{4321}, ExtraProbePaths: []string{"/"}},
	detector.Jekyll:      {ExpectedPorts: []int{4000}, ExtraProbePaths: []string{"/"}},
}

func HintsFromDir(dir string) StackHints {
	var result StackHints

	if port := readPortlessPort(dir); port > 0 {
		result.ExpectedPorts = append(result.ExpectedPorts, port)
	}

	types := detector.Detect(dir)
	for _, pt := range types {
		if h, ok := stackHints[pt]; ok {
			for _, p := range h.ExpectedPorts {
				if !containsInt(result.ExpectedPorts, p) {
					result.ExpectedPorts = append(result.ExpectedPorts, p)
				}
			}
			result.ExtraProbePaths = append(result.ExtraProbePaths, h.ExtraProbePaths...)
		}
	}

	return result
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
```

- [x] **Step 7: Run all hints tests**

Run: `go test ./internal/observer/ -run "TestReadPortless|TestHintsFromDir" -v`
Expected: PASS (8 tests)

- [x] **Step 8: Commit**

```
feat(observer): add stack hints with portless-first detection (FEAT-046)
```

---

### Task 2: ProbeEndpoints accepts extra paths (P-02)

**Files:**
- Modify: `internal/observer/probes.go`
- Modify: `internal/observer/probes_test.go`

- [x] **Step 1: Write failing test for extra paths**

Add to `probes_test.go`:

```go
func TestProbeEndpointsExtraPaths(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/custom-health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	port := srv.Listener.Addr().(*net.TCPAddr).Port
	ports := []PortBinding{{Port: port, Protocol: "tcp"}}
	results := ProbeEndpoints(ports, 2*time.Second, "/custom-health")
	if len(results) == 0 {
		t.Fatal("expected to find /custom-health endpoint")
	}
	if !strings.Contains(results[0].URL, "/custom-health") {
		t.Errorf("expected /custom-health in URL, got %s", results[0].URL)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/observer/ -run TestProbeEndpointsExtraPaths -v`
Expected: FAIL — too many arguments to ProbeEndpoints

- [x] **Step 3: Update ProbeEndpoints signature**

Change `probes.go`:

```go
func ProbeEndpoints(ports []PortBinding, timeout time.Duration, extraPaths ...string) []HTTPProbeResult {
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	paths := append(commonPaths, extraPaths...)

	seen := make(map[int]bool)
	client := &http.Client{Timeout: timeout}
	var results []HTTPProbeResult

	for _, pb := range ports {
		if seen[pb.Port] {
			continue
		}
		seen[pb.Port] = true

		for _, path := range paths {
			url := fmt.Sprintf("http://localhost:%d%s", pb.Port, path)
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
				results = append(results, HTTPProbeResult{
					URL:        url,
					StatusCode: resp.StatusCode,
					Reachable:  true,
				})
				break
			}
		}
	}

	return results
}
```

Using variadic `...string` means all existing callers (observer.go passes zero extra paths) compile without changes.

- [x] **Step 4: Run all probe tests**

Run: `go test ./internal/observer/ -run TestProbe -v`
Expected: PASS (6 tests — 5 existing + 1 new)

- [x] **Step 5: Commit**

```
feat(observer): ProbeEndpoints accepts extra paths via variadic (FEAT-046)
```

---

### Task 3: Observer wires hints into pipeline (P-03)

**Files:**
- Modify: `internal/observer/observer.go`

- [x] **Step 1: Add hint resolution after snapshot setup**

In `Observe()`, after the pre-snapshot block (line ~31) and before command setup, add:

```go
	// Resolve stack hints for smarter observation.
	var hints StackHints
	if opts.Dir != "" {
		hints = HintsFromDir(opts.Dir)
	}
```

- [x] **Step 2: Pass extra probe paths to ProbeEndpoints**

Change the ProbeEndpoints call (around line ~145) from:

```go
	httpProbes = ProbeEndpoints(detectedPorts, 2*time.Second)
```

to:

```go
	httpProbes = ProbeEndpoints(detectedPorts, 2*time.Second, hints.ExtraProbePaths...)
```

- [x] **Step 3: Build and run all observer tests**

Run: `go build ./... && go test ./internal/observer/ -v -count=1`
Expected: BUILD OK, all tests PASS

- [x] **Step 4: Commit**

```
feat(observer): wire stack hints into observation pipeline (FEAT-046)
```

---

### Task 4: cmd/observe wiring and integration test (P-04)

**Files:**
- Modify: `cmd/observe.go` (no changes needed — observer auto-detects from dir)

- [x] **Step 1: Verify no cmd changes needed**

The observer now calls `HintsFromDir(opts.Dir)` internally when dir is set. The command already passes `--dir` through to `ObserveOptions.Dir`. No wiring changes needed in `cmd/observe.go`.

- [x] **Step 2: Run full build + test suite**

Run: `go build ./... && go test ./internal/observer/ ./cmd/ -count=1`
Expected: BUILD OK, all tests PASS

- [x] **Step 3: Manual smoke test**

```bash
go build -o /tmp/smokesig-test . && /tmp/smokesig-test observe --quiet --dir . --output /tmp/test-hints.yaml "echo 'server listening on port 8080'"
cat /tmp/test-hints.yaml
```

Expected: generated YAML should include stack-aware probe paths (Go project detected, `/metrics` and `/debug/pprof` probed in addition to defaults).

- [x] **Step 4: Cleanup and commit**

```bash
rm /tmp/smokesig-test /tmp/test-hints.yaml
```

```
feat(observer): detector-observer integration complete (FEAT-046)
```

---

### Task 5: Close issue and update docs

- [x] **Step 1: Close FEAT-046**

```bash
ccs issues update FEAT-046 --status done
```

- [x] **Step 2: Stage changelog**

```bash
ccs changelog add "Stack-aware observation via detector integration — portless-first port detection, stack-specific HTTP probe paths (FEAT-046)" --type added
```

- [x] **Step 3: Update CLAUDE.md observe command description if needed**
