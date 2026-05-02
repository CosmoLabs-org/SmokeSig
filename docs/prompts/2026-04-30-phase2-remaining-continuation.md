---
brainstorm_ref: docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
branch: master
covers_brainstorm_deliverables:
    - BR-14
    - BR-06
created: "2026-04-30"
id: P-2026-04-30-phase2-remaining-continuation
plan_ref: docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md
priority: high
requires_reading:
    - docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md
    - docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md
schema_version: 1
status: PENDING
title: "Phase 2 Remaining — FEAT-041 Background Commands + FEAT-043 GitHub Action"
---

# Phase 2 Remaining — FEAT-041 Background Commands + FEAT-043 GitHub Action

## BEFORE Starting — Required Reading

Read in order:

1. **`docs/brainstorming/2026-04-29-gemini-ecosystem-feedback.md`** — Focus on BR-14 (Background commands with wait_for_port) starting around line 800. This is the design rationale: why port polling beats arbitrary sleep, which dev servers need it, and the exponential backoff strategy.
2. **`docs/planning-mode/2026-04-30-phase1-github-actions-reporter.md`** — Context for FEAT-043. The GHA reporter (FEAT-039) shipped this session. The GitHub Action wrapper (FEAT-043) depends on that `--format gha` output.

## Project State

- **Version**: v0.16.0
- **Tests**: 1008 passing across 11 packages
- **Phase 1**: COMPLETE (FEAT-037 file_size, FEAT-038 test chaining, FEAT-039 GHA reporter)
- **Phase 2**: 2/4 done (FEAT-040 lifecycle hooks, FEAT-042 remote config both shipped)
- **Remaining**: FEAT-041 (background commands), FEAT-043 (GitHub Action wrapper)

## What Already Exists

**Lifecycle hooks** (`internal/runner/lifecycle.go`, 84 lines):
- `RunLifecycleHooks()` executes hooks sequentially via `sh -c <command>`
- Supports `AlwaysRun`, `EnvPass`, `Timeout` fields
- Context-aware with per-hook timeout
- All hooks use `cmd.Run()` (blocking) — no background execution yet

**LifecycleHook schema** (`internal/schema/schema.go`, line 61):
```go
type LifecycleHook struct {
    Command   string   `yaml:"command"`
    Timeout   Duration `yaml:"timeout,omitempty"`
    AlwaysRun bool     `yaml:"always_run,omitempty"`
    EnvPass   bool     `yaml:"env_pass,omitempty"`
}
```

**LifecycleConfig** (line 54):
```go
type LifecycleConfig struct {
    BeforeAll  []LifecycleHook `yaml:"before_all,omitempty"`
    AfterAll   []LifecycleHook `yaml:"after_all,omitempty"`
    BeforeEach []LifecycleHook `yaml:"before_each,omitempty"`
    AfterEach  []LifecycleHook `yaml:"after_each,omitempty"`
}
```

**Port listening assertion** exists in the runner — reuse its TCP dial logic for port polling.

**GHA reporter** (`internal/reporter/github.go`) is live with `--format gha` that writes `$GITHUB_STEP_SUMMARY` and emits `::error`/`::warning` workflow commands.

## GLM Dispatch Rules

1. **ALWAYS** use `ccs glm-agent exec` for GLM agents (routes through queue with retry)
2. **NEVER** use Agent tool with `model:sonnet`/`model:haiku` for GLM work (bypasses queue)
3. Agent tool with `model:opus` is fine for Opus subagents
4. For parallel work: use `/glm-sprint` or `ccs glm-agent exec-batch`

## Goals

### [ ] G-01 Build FEAT-041: Background commands with wait_for_port
**Model:** `glm-turbo` | **Priority:** 1 | **Depends on:** FEAT-040 (done)

Extend lifecycle hooks to support background process execution with port-readiness polling:

```yaml
lifecycle:
  before_all:
    - command: "npm run tauri dev"
      background: true
      wait_for_port: 1420
      timeout: 30s
```

**Implementation:**

1. **Extend schema** (`internal/schema/schema.go`):
   - Add to `LifecycleHook`: `Background bool \`yaml:"background,omitempty"\``, `WaitForPort int \`yaml:"wait_for_port,omitempty"\``, `StartupTimeout Duration \`yaml:"startup_timeout,omitempty"\``
   - Add validation: `background: true` requires either `wait_for_port` or `timeout` > 0

2. **Extend runner** (`internal/runner/lifecycle.go`):
   - When `hook.Background` is true, use `exec.CommandContext(hookCtx, "sh", "-c", hook.Command).Start()` instead of `.Run()`
   - Store the `*exec.Cmd` (or its `Process.Pid`) in a slice on the Runner for cleanup
   - When `hook.WaitForPort > 0`, poll TCP `net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200ms)` with exponential backoff (50ms, 100ms, 200ms, 400ms...) up to `timeout`
   - If port becomes ready within timeout, proceed. If timeout elapses, return error (kill the background process first)
   - After port is ready, continue to next hook (background process keeps running)

3. **Cleanup in after_all** (`internal/runner/lifecycle.go`):
   - Kill all stored background PIDs during `after_all` execution
   - Send SIGTERM, wait briefly, then SIGKILL if still alive
   - Cleanup runs even if suite panics (defer in the Runner)

4. **Tests** (`internal/runner/lifecycle_test.go`, currently 299 lines):
   - `TestRunLifecycleHooks_BackgroundStarts` — background command starts without blocking
   - `TestRunLifecycleHooks_WaitForPortReady` — background + wait_for_port succeeds when port opens
   - `TestRunLifecycleHooks_WaitForPortTimeout` — fails when port never opens within timeout
   - `TestRunLifecycleHooks_BackgroundCleanup` — PIDs killed after after_all
   - `TestRunLifecycleHooks_BackgroundWithoutWaitForPort` — background: true with no port, just starts and moves on
   - `TestRunLifecycleHooks_PortPollingBackoff` — verify exponential backoff intervals
   - `TestRunLifecycleHooks_EnvPassFromBackground` — env_pass works with background processes
   - `TestRunLifecycleHooks_BackgroundWithAlwaysRun` — always_run doesn't apply to background (it's about error continuation)

**Files:** `internal/schema/schema.go`, `internal/schema/validate.go`, `internal/schema/validate_test.go`, `internal/runner/lifecycle.go`, `internal/runner/lifecycle_test.go`

**Estimated:** ~200 lines new code, ~8 tests

**TDD:** Invoke `superpowers:test-driven-development` first. Write tests, watch them fail, then implement.

### [ ] G-02 Create FEAT-043: Official GitHub Action wrapper
**Model:** `sonnet` | **Priority:** 2

This is a **separate repository**: `cosmolabs-org/cosmo-smoke-action`. Not modifications to cosmo-smoke itself.

**action.yaml:**
```yaml
name: "Cosmo Smoke"
description: "Run cosmo-smoke tests in GitHub Actions"
inputs:
  config-path:
    description: "Path to .smoke.yaml"
    required: false
    default: ".smoke.yaml"
  format:
    description: "Output format"
    required: false
    default: "gha"
  tags:
    description: "Comma-separated tags to filter"
    required: false
  fail-fast:
    description: "Stop on first failure"
    required: false
    default: "true"
runs:
  using: "composite"
  steps:
    - shell: bash
      run: |
        # Download latest smoke binary
        VERSION=$(curl -sL https://api.github.com/repos/CosmoLabs-org/cosmo-smoke/releases/latest | jq -r '.tag_name')
        curl -sL "https://github.com/CosmoLabs-org/cosmo-smoke/releases/download/${VERSION}/smoke-${VERSION}-$(uname -s)-$(uname -m)" -o /usr/local/bin/smoke
        chmod +x /usr/local/bin/smoke
        # Run smoke with GHA format
        ARGS="-f ${{ inputs.config-path }} --format ${{ inputs.format }}"
        [[ "${{ inputs.fail-fast }}" == "true" ]] && ARGS="$ARGS --fail-fast"
        [[ -n "${{ inputs.tags }}" ]] && ARGS="$ARGS --tag ${{ inputs.tags }}"
        smoke run $ARGS
```

**Steps:**
1. Create `cosmolabs-org/cosmo-smoke-action` repo on GitHub
2. Add `action.yaml` (inputs, composite steps)
3. Add `README.md` with usage example
4. Tag as `v1`

**Usage:**
```yaml
- uses: cosmolabs-org/cosmo-smoke-action@v1
  with:
    config-path: ".smoke.yaml"
    format: "gha"
```

This automatically gets `$GITHUB_STEP_SUMMARY` integration because FEAT-039's `--format gha` writes to it.

### [ ] G-03 Update issues and roadmap
**Model:** `opus`

After both features are complete:
```bash
ccs issues update FEAT-041 --status done
ccs issues update FEAT-043 --status done
```

Also stage changelog entries for v0.17.0:
```bash
ccs changelog add "FEAT-041: Background commands with wait_for_port on lifecycle hooks"
ccs changelog add "FEAT-043: Official GitHub Action wrapper (cosmolabs-org/cosmo-smoke-action)"
```

## Where We're Headed

After Phase 2 completes, Phase 3 is **FEAT-044** (flakiness detector / `smoke stress`). The brainstorm has a detailed blueprint: bounded worker pool, `sync/atomic` counters, error deduplication, reliability thresholds (stable/flaky/unreliable), ~730 lines.

Phase 3 is the last single-feature phase. After that, Phase 4 brings `smoke observe` (BR-13, ~1080 lines) and Phase 5 is the ecosystem play (Wasm plugins, OIDC).

## Priority Order
1. G-01 (FEAT-041 background commands) — completes Phase 2 lifecycle story, ships with v0.17.0
2. G-02 (FEAT-043 GitHub Action) — distribution, separate repo
3. G-03 (housekeeping) — update issues, stage changelog
