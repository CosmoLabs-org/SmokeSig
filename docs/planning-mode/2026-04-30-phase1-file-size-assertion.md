---
date: 2026-04-30
status: plan
issue: FEAT-037
brainstorm: docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
deliverables:
  - id: P-01
    title: "file_size assertion struct and schema registration"
  - id: P-02
    title: "file_size assertion evaluation logic"
  - id: P-03
    title: "file_size assertion tests (happy path, edge cases, errors)"
  - id: P-04
    title: "schema validation for file_size"
---

# Phase 1: File Size Assertion (BR-11 / FEAT-037)

**Goal**: Add `file_size` assertion type that checks file existence AND size thresholds.

## Current State

`file_exists` checks presence only. `port_listening` shows the pattern for structured assertions with multiple fields. This extends `file_exists` naturally.

## Implementation Steps

### P-01: Schema Definition (~15 lines)

**File**: `internal/schema/schema.go`

Add `FileSize *FileSizeCheck` to `Assertion` struct:
```go
type FileSizeCheck struct {
    Path     string `yaml:"path"`
    MinBytes *int64 `yaml:"min_bytes,omitempty"`
    MaxBytes *int64 `yaml:"max_bytes,omitempty"`
}
```

Register in assertion validation map.

### P-02: Assertion Logic (~25 lines)

**File**: `internal/runner/assertion_file.go`

Add `checkFileSize` function following the `checkFileExists` pattern:
- `os.Stat(path)` for file info
- Compare `info.Size()` against min/max thresholds
- Return meaningful error messages: "file 52.3MB exceeds max 50MB"

### P-03: Tests (~60 lines)

**File**: `internal/runner/assertion_test.go`

Test cases:
- File within range → pass
- File exceeds max_bytes → fail with size info
- File below min_bytes → fail with size info
- File doesn't exist → fail with file-not-found
- No min/max specified → just checks existence (like file_exists)
- min_bytes only → no upper bound
- max_bytes only → no lower bound

### P-04: Schema Validation (~10 lines)

**File**: `internal/schema/schema.go` (validation function)

- `path` is required
- At least one of `min_bytes` or `max_bytes` must be set (or both optional — just check existence if neither set)
- `min_bytes` < `max_bytes` if both set

## YAML Usage

```yaml
tests:
  - name: android-apk-size
    assertions:
      - file_size:
          path: android/app/build/outputs/apk/release/app-release.apk
          max_bytes: 50000000   # 50MB
  - name: tauri-binary
    assertions:
      - file_size:
          path: target/release/bundle/macos/app.app
          min_bytes: 5000000
          max_bytes: 100000000
```

## Scope

- **Files changed**: 3 (schema.go, assertion_file.go, assertion_test.go)
- **Estimated lines**: ~110 total
- **New dependencies**: None
- **Breaking changes**: None
