---
brainstorm: docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
completed: "2026-05-02"
created: "2026-04-30"
date: 2026-04-30T00:00:00Z
deliverables:
    - id: P-01
      title: Variable store and extraction interface
    - id: P-02
      title: 'extract: field on json_field, stdout_matches, http assertions'
    - id: P-03
      title: Variable resolution in Go templates (.Vars namespace)
    - id: P-04
      title: Sequential execution for chained test groups
    - id: P-05
      title: Sensitive variable masking in reporter output
    - id: P-06
      title: Tests for full chain lifecycle (extract → resolve → assert)
goals_completed: 0
goals_total: 0
issue: FEAT-038
related_prompts: []
requires_reading: []
schema_version: 1
status: COMPLETED
tags: []
title: 'Phase 1: Test Chaining with Data Extraction (BR-08 / FEAT-038)'
---

# Phase 1: Test Chaining with Data Extraction (BR-08 / FEAT-038)

**Goal**: Enable extracting values from one test and injecting into subsequent tests.

## Problem

Real-world smoke tests need state: login → get token → use token. Currently all tests are stateless. This is the biggest functional gap for API testing workflows.

## Architecture

### Variable Store

```go
type VarStore struct {
    mu     sync.RWMutex
    vars   map[string]string
    secret map[string]bool // marks sensitive vars for masking
}
```

- Created fresh per `smoke run` invocation
- Populated by `extract:` fields on assertions
- Read by Go template engine when resolving `{{ .Vars.X }}`
- Thread-safe because parallel test groups may share the store

### Extraction Sources

| Assertion Type | Extraction Mechanism |
|---|---|
| `json_field` with `extract: var_name` | Extracts the matched value |
| `stdout_matches` with `extract: var_name` | Uses first capture group |
| `http` with `extract_headers: [name]` | Extracts header values |

### Variable Resolution

Extend existing Go template engine (`{{ .Env.FOO }}`) with `.Vars` namespace:
- `{{ .Vars.jwt_token }}` resolves from VarStore
- `{{ .Env.API_HOST }}` resolves from environment (existing)
- Both work in: command args, assertion values, URLs

### Execution Model

- Tests with `extract:` fields form implicit dependency chains
- Chained tests must run sequentially (disable parallel within chain)
- Independent tests still parallelize normally
- Simplest approach: add `chain_group` field to Test; tests in same group run sequentially in order

### Security

- Vars matching `*token*`, `*key*`, `*secret*`, `*password*`, `*auth*` auto-masked in output
- Masked as `***REDACTED***` in terminal and JSON reporters
- Full values available in `--verbose` mode for debugging

## Implementation Steps

### P-01: Variable Store (~60 lines)

**File**: `internal/runner/varstore.go`

```go
type VarStore struct { ... }
func (v *VarStore) Set(key, value string) { ... }
func (v *VarStore) Get(key string) (string, bool) { ... }
func (v *VarStore) ResolveTemplate(tmpl string) (string, error) { ... }
func (v *VarStore) IsSecret(key string) bool { ... }
```

### P-02: Extract Fields in Schema (~40 lines)

**File**: `internal/schema/schema.go`

Add `Extract string` to relevant assertion structs:
- `JSONFieldCheck.Extract`
- `StdoutMatchesCheck.Extract`
- `HTTPCheck.ExtractHeaders []string`

### P-03: Template Resolution with Vars (~50 lines)

**File**: Extend existing template resolution in runner

Add `.Vars` to template data struct alongside `.Env`:
```go
type templateData struct {
    Env  map[string]string
    Vars *VarStore
}
```

### P-04: Sequential Chain Execution (~80 lines)

**File**: `internal/runner/runner.go`

- Detect tests with `extract:` references
- Group chained tests together
- Run groups sequentially, independent tests in parallel
- Simple heuristic: if test B's command or assertions contain `{{ .Vars.X }}` and test A has `extract: X`, they chain

### P-05: Masking in Reporters (~40 lines)

**File**: `internal/reporter/terminal.go`, `internal/reporter/json.go`

- Before displaying command or assertion values, redact sensitive vars
- Check VarStore.IsSecret() for each variable name

### P-06: Tests (~230 lines)

**File**: `internal/runner/runner_test.go`, `internal/runner/varstore_test.go`

- Extract from json_field → resolve in next test
- Extract from stdout_matches capture group → resolve
- Sensitive var masking
- Chain ordering (A must complete before B)
- Missing variable → clear error
- Circular dependency detection

## Scope

- **Files changed**: ~6
- **New files**: `internal/runner/varstore.go`, `internal/runner/varstore_test.go`
- **Estimated lines**: ~500
- **New dependencies**: None
- **Breaking changes**: None (additive only)
