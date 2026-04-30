---
date: 2026-04-30
status: plan
issue: FEAT-039
brainstorm: docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
deliverables:
  - id: P-01
    title: "GitHub Actions reporter implementation"
  - id: P-02
    title: "Problem matcher JSON definition"
  - id: P-03
    title: "Reporter registration and format flag integration"
  - id: P-04
    title: "Tests for GHA output format"
---

# Phase 1: GitHub Actions Native Output (BR-01 / FEAT-039)

**Goal**: Add `--format gha` that integrates natively with GitHub Actions UI.

## What It Does

1. Writes markdown summary to `$GITHUB_STEP_SUMMARY` file
2. Emits `::error` and `::warning` workflow commands for failed/allowed-failure tests
3. Ships a problem matcher JSON for regex-based annotation

## Implementation Steps

### P-01: GitHub Actions Reporter (~120 lines)

**File**: `internal/reporter/github.go`

```go
type GitHubActions struct {
    summaryFile string // $GITHUB_STEP_SUMMARY path
    w           io.Writer
    tests       []ghaTestResult
}

func (g *GitHubActions) TestComplete(r TestResultData) { ... }
func (g *GitHubActions) Summary(suite SuiteResultData) error { ... }
```

**Summary markdown format**:
```markdown
## Smoke Test Results
**Status**: ✅ 5/5 passed | Duration: 2.3s

| Test | Status | Duration |
|------|--------|----------|
| api-health | ✅ | 120ms |
| db-ping | ✅ | 45ms |

### Failed Tests
| Test | Assertion | Error |
|------|-----------|-------|
| auth-endpoint | exit_code | expected 0, got 1 |
```

**Workflow commands**:
- `::error title=Smoke Test Failed,file=.smoke.yaml::auth-endpoint: exit_code expected 0, got 1`
- `::warning title=Flaky Test,file=.smoke.yaml::cache-ping: allowed failure`

### P-02: Problem Matcher (~30 lines)

**File**: `docs/github/problem-matcher.json`

Shipped as a companion file. Users add to their workflow:
```yaml
- uses: actions/setup-python@v5 # example
- run: echo "::add-matcher::./problem-matcher.json"
- run: smoke run --format gha
```

### P-03: Format Registration (~10 lines)

**File**: `internal/reporter/multi.go`

Register `gha` as a valid format string. When `$GITHUB_ACTIONS` env is set and `gha` format is selected, enable GHA-specific output.

### P-04: Tests (~60 lines)

**File**: `internal/reporter/github_test.go`

- Summary generation with all passing tests
- Summary generation with failures
- Workflow command format verification
- `$GITHUB_STEP_SUMMARY` not set → graceful fallback (stdout only)
- Problem matcher regex validation

## Scope

- **Files changed**: ~4
- **New files**: `internal/reporter/github.go`, `internal/reporter/github_test.go`, `docs/github/problem-matcher.json`
- **Estimated lines**: ~220
- **New dependencies**: None
- **Breaking changes**: None
