---
date: "2026-05-26T00:15:00-03:00"
source: /brainplan session
status: brainstorm
issue: FEAT-046
related:
  - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
deliverables:
  - id: BR-01
    title: "StackHints struct and HintsFromDir function"
  - id: BR-02
    title: "Portless.json reader with stack-type fallback"
  - id: BR-03
    title: "Stack hint table for v1 project types"
  - id: BR-04
    title: "Observer integration — hints applied to port detection and HTTP probing"
  - id: BR-05
    title: "cmd/observe.go auto-detection wiring"
---

# FEAT-046: Detector-Observer Integration

## Problem

`smokesig observe` captures raw behavior — stdout, ports, files, HTTP endpoints — but has no awareness of what kind of project it's observing. A Go server on port 8080 gets the same treatment as a React Native Metro bundler on 8081. Stack-aware observation would produce better-targeted assertions by knowing what to look for.

## Design Decision: Observer-Level Hints

The detector integration lives in the observer layer, not the generator. The observer becomes stack-aware and captures more intelligently; the generator stays simple (takes an Observation, emits YAML).

Rationale: It's better to capture the right data than to post-process generic data. If the observer knows to probe `/metrics` on a Go server, the generated assertions are more useful than if it only probed the generic `/health` path.

## Architecture

### New File: `internal/observer/hints.go`

```go
type StackHints struct {
    ExpectedPorts   []int
    ExtraProbePaths []string
}

func HintsFromDir(dir string) StackHints
```

`HintsFromDir` resolves hints in priority order:

1. **Portless-first**: Read `portless.json` in `dir`. If present and has a `port` field, use it as the primary expected port. This aligns with the CCS portless SOP where every project declares its dev port.

2. **Stack fallback**: Call `detector.Detect(dir)` to identify project types. Map each type to default ports and probe paths. Merge with any portless port (portless port goes first).

### Portless Schema

Two known formats in the wild:

```json
{"name": "project", "port": 4650, "domain": "project.cosmo"}
```

```json
{"apps": {"web": {"name": "project.cosmo"}}}
```

The reader handles the `port` field from the flat format. The `apps` format doesn't declare ports — fall through to stack detection.

### Stack Hint Table (v1)

| Stack | Expected Ports | Extra Probe Paths |
|-------|---------------|-------------------|
| Go | 8080 | /metrics, /debug/pprof |
| Node | 3000 | /api, /graphql |
| Python | 8000 | /admin, /api |
| React Native | 8081 | /status |
| Rust | 8080 | /api |
| Java / Java Gradle | 8080 | /actuator/health |
| DotNet | 5000 | /swagger |
| Ruby | 3000 | /api |
| PHP | 8000 | /api |
| Elixir | 4000 | /api |
| Deno | 8000 | /api |
| Docker | (none) | (none — ports come from container) |
| Flutter / iOS / Android | (none) | (none — not server stacks) |
| Terraform / Helm / Kustomize | (none) | (none — infra, not servers) |
| Hugo / Astro / Jekyll | 1313, 4321, 4000 | / |

Stacks not listed get empty hints (observer falls back to generic behavior).

### Integration Points

**ObserveOptions** gains a `StackHints *StackHints` field:
- `nil` = auto-detect disabled (caller didn't set dir, or explicitly opted out)
- Non-nil = observer uses hints to tune behavior

**Observe()** changes:
- After snapshot setup, if `opts.Dir != ""` and `opts.StackHints == nil`, call `HintsFromDir(opts.Dir)` and store result
- Pass `hints.ExtraProbePaths` to `ProbeEndpoints()` so it probes stack-specific paths in addition to the default common paths

**ProbeEndpoints()** changes:
- Accept an optional `extraPaths []string` parameter (or append to the existing `commonPaths` slice)
- Probe extra paths after common paths, same logic (first reachable per port wins)

**cmd/observe.go** changes:
- When `--dir` is set, auto-populate `opts.StackHints` via `HintsFromDir(dir)`
- When `--dir` is not set but cwd contains project markers, still auto-detect from cwd

## What This Does NOT Do (v1 boundaries)

- Does NOT tune snapshot directories (SnapshotDirs/SkipDirs). Generic snapshot behavior is sufficient.
- Does NOT add stack-specific stdout keywords. ExtractKeyPhrases stays generic.
- Does NOT change the generator. Generator still takes a plain Observation.
- Does NOT add new CLI flags. Detection is automatic.

## Estimated Scope

- `internal/observer/hints.go`: ~60 LOC (StackHints struct, HintsFromDir, portless reader, stack table)
- `internal/observer/hints_test.go`: ~80 LOC (portless parsing, stack detection, fallback behavior)
- `internal/observer/probes.go`: ~10 LOC change (accept extra paths)
- `internal/observer/observer.go`: ~5 LOC change (call HintsFromDir, pass to probes)
- `internal/observer/types.go`: ~3 LOC change (add StackHints field)
- `cmd/observe.go`: ~5 LOC change (wire auto-detection)
- **Total: ~160 LOC** (80 new + 80 test)
