---
brainstorm: docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
completed: "2026-05-12"
created: "2026-05-02"
deliverables:
    - id: P-01
      title: Backstage reporter struct implementing Reporter interface
    - id: P-02
      title: Backstage entity annotation JSON format
    - id: P-03
      title: '`--format backstage` integration in chain.go'
    - id: P-04
      title: Tests for Backstage output format
goals_completed: 0
goals_total: 0
issue: FEAT-050
related_prompts: []
requires_reading: []
schema_version: 1
status: COMPLETED
tags:
    - feat-050
    - reporter
    - backstage
title: 'FEAT-050: Backstage.io Schema Output Reporter (BR-05)'
---

# FEAT-050: Backstage.io Schema Output Reporter (BR-05)

**Goal**: Add `--format backstage` that emits JSON conforming to Backstage's entity annotation format, allowing smoke test health to surface in developer portals.

## Problem

Organizations running Backstage want smoke test status visible alongside service ownership, API docs, and runbooks. cosmo-smoke already has 6 output formats (terminal, json, junit, tap, prometheus, gha). A Backstage reporter is a natural extension — low effort (~150 LOC), zero new dependencies, reuses the existing Reporter interface pattern.

## Architecture

### Backstage Entity Annotation Format

Backstage consumes entity annotations as part of its catalog model. The standard pattern for health checks uses the `backstage.io/` annotation namespace. The reporter outputs a JSON object that can be:

1. Written to a file for the Backstage catalog importer to pick up
2. POSTed to a Backstage endpoint (via existing `--report-url`)
3. Consumed by custom Backstage plugins

**Output schema**:

```json
{
  "apiVersion": "backstage.io/v1alpha1",
  "kind": "Component",
  "metadata": {
    "name": "my-service",
    "annotations": {
      "backstage.io/smoke-status": "healthy",
      "backstage.io/smoke-passed": "5",
      "backstage.io/smoke-failed": "0",
      "backstage.io/smoke-total": "5",
      "backstage.io/smoke-timestamp": "2026-05-02T10:30:00Z"
    }
  },
  "status": {
    "healthcheck": {
      "status": "healthy",
      "checks": [
        {
          "name": "api-health",
          "status": "healthy",
          "duration_ms": 120,
          "message": ""
        },
        {
          "name": "db-connection",
          "status": "unhealthy",
          "duration_ms": 302,
          "message": "exit_code: expected 0, got 1"
        }
      ]
    }
  }
}
```

### Reporter Pattern

Follows the same pattern as `github.go`:

- Struct with `io.Writer` + collected `[]TestResultData`
- No-op on `PrereqStart`, `PrereqResult`, `TestStart`
- Accumulate tests in `TestResult`
- Build and emit JSON in `Summary`

## Implementation Steps

### P-01: Backstage Reporter Struct (~60 lines)

**File**: `internal/reporter/backstage.go`

```go
type Backstage struct {
    w      io.Writer
    tests  []TestResultData
}

func NewBackstage(w io.Writer) *Backstage {
    return &Backstage{w: w}
}

func (b *Backstage) PrereqStart(_ string)            {}
func (b *Backstage) PrereqResult(_ PrereqResultData) {}
func (b *Backstage) TestStart(_ string)              {}

func (b *Backstage) TestResult(r TestResultData) {
    b.tests = append(b.tests, r)
}

func (b *Backstage) Summary(s SuiteResultData) {
    // Build Backstage entity JSON, write to b.w
}
```

### P-02: Backstage JSON Format (~50 lines)

**File**: `internal/reporter/backstage.go` (same file)

Types for Backstage entity output:

```go
type backstageEntity struct {
    APIVersion string              `json:"apiVersion"`
    Kind       string              `json:"kind"`
    Metadata   backstageMetadata   `json:"metadata"`
    Status     backstageStatus     `json:"status,omitempty"`
}

type backstageMetadata struct {
    Name        string            `json:"name"`
    Annotations map[string]string `json:"annotations"`
}

type backstageStatus struct {
    HealthCheck backstageHealthCheck `json:"healthcheck,omitempty"`
}

type backstageHealthCheck struct {
    Status string             `json:"status"`
    Checks []backstageCheck   `json:"checks"`
}

type backstageCheck struct {
    Name       string `json:"name"`
    Status     string `json:"status"`
    DurationMs int64  `json:"duration_ms"`
    Message    string `json:"message,omitempty"`
}
```

Status mapping:
- All tests passed → `"healthy"`
- Any test failed (non-allowed) → `"unhealthy"`
- All tests failed but allowed → `"degraded"`
- No tests → `"unknown"`

### P-03: Format Registration (~5 lines)

**File**: `internal/reporter/chain.go`

Add to the `formats` map:

```go
"backstage": {"smoke-backstage.json", func(w io.Writer) Reporter { return NewBackstage(w) }},
```

Update the error message in `Chain()` to list `backstage` in valid formats.

### P-04: Tests (~80 lines)

**File**: `internal/reporter/backstage_test.go`

Test cases:

1. **All passing** — status is `"healthy"`, all checks have `"healthy"` status
2. **One failure** — overall status is `"unhealthy"`, failed check has `"unhealthy"` with error message
3. **Allowed failure** — overall status is `"degraded"` (or `"healthy"` if all non-allowed pass), allowed failure check has appropriate message
4. **Empty suite** — no tests, status is `"unknown"`, empty checks array
5. **JSON validity** — output parses as valid JSON, required fields present
6. **Annotations** — metadata.annotations contains expected keys (`backstage.io/smoke-status`, `backstage.io/smoke-passed`, etc.)
7. **Timestamp format** — `backstage.io/smoke-timestamp` is RFC 3339

## Scope

- **Files changed**: 2 (`chain.go`, new `backstage.go`)
- **New files**: `internal/reporter/backstage.go`, `internal/reporter/backstage_test.go`
- **Estimated lines**: ~150 total (60 struct + 50 JSON types/build + 5 registration + 80 tests... minus test boilerplate ≈ 150 non-test, 80 test)
- **New dependencies**: None
- **Breaking changes**: None (additive only)

## Key Decisions

1. **File output vs stdout**: Like other non-terminal formats, Backstage writes to a file (`smoke-backstage.json`) when used as a secondary format, or stdout when used as primary. This follows the existing `chain.go` convention.

2. **Entity kind**: Always `Component`. This is the most common Backstage entity type for services. Users can transform in their Backstage pipeline if needed.

3. **No new CLI flags**: The format is purely `--format backstage`. Project name comes from `project:` in `.smoke.yaml` (already available via `SuiteResultData.Project`). No additional config needed.

4. **Status granularity**: Three-tier status (healthy/unhealthy/degraded) maps cleanly from existing pass/fail/allowed-failure semantics. No new concepts needed.

## Usage

```bash
# Write Backstage JSON to stdout
smoke run --format backstage

# Write to file alongside terminal output
smoke run --format terminal,backstage

# In CI: terminal for logs, Backstage JSON for catalog import
smoke run --format terminal,backstage
# Then: curl -X POST backstage-api/entities -d @smoke-backstage.json
```
