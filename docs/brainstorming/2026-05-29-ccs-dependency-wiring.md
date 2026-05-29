---
date: "2026-05-29T10:00:00-03:00"
status: brainstorm
issue: FEAT-052
tags:
  - integration
  - ccs
  - external-dependency
  - migration
title: "Wire SmokeSig as External Dependency in CCS"
deliverables:
  - id: BR-01
    title: "SmokeSig binary contract (name, version output, exit codes, config discovery)"
  - id: BR-02
    title: "CCS smoke.go passthrough rewrite (smokesig binary, arg forwarding, exit code mapping)"
  - id: BR-03
    title: "CCS rebuild.go SmokeSig entry (clone, build, install)"
  - id: BR-04
    title: "CCS .binary-registry.json SmokeSig entry"
  - id: BR-05
    title: "CCS sync external dependency check for smokesig"
  - id: BR-06
    title: "Migration plan from cosmo-smoke references in CCS"
  - id: BR-07
    title: "Design decisions: CLI-only vs Go API, version pinning, config fallback, CCS-specific flags"
---

# Wire SmokeSig as External Dependency in CCS

## Problem

SmokeSig was renamed from `cosmo-smoke` (see `2026-05-05-rename-to-smokesig.md`). The SmokeSig side of the rename is complete: the binary is `smokesig`, the config file is `.smokesig.yaml`, the Go module is `github.com/CosmoLabs-org/SmokeSig`. However, CCS (ClaudeCodeSetup) still has stale references:

1. `ccs smoke` passthrough (`smoke.go`) references the old `cosmo-smoke` / `smoke` binary name
2. `ccs rebuild` (`rebuild.go`) does not include SmokeSig as a buildable external dependency
3. `.binary-registry.json` has no SmokeSig entry
4. `ccs sync` does not verify SmokeSig is installed
5. Documentation across CCS references old names (`cosmo-smoke`, `.smoke.yaml`, `smoke run`)

FB-010 documents 4 CCS files that were partially updated but need review and commit. This brainstorm covers the full integration surface from SmokeSig's perspective -- what SmokeSig exposes, what CCS needs to consume, and how to migrate cleanly.

## Current State

### SmokeSig Binary Contract (as of v0.23.0)

| Aspect | Current Value | Stable? |
|--------|--------------|---------|
| Binary name | `smokesig` | Yes -- renamed from `smoke`, clean break |
| Module path | `github.com/CosmoLabs-org/SmokeSig` | Yes |
| Config file (primary) | `.smokesig.yaml` | Yes |
| Config file (fallback) | `.smoke.yaml` with deprecation warning | Yes, intentional backward compat |
| Version output | `smokesig 0.23.0` (format: `smokesig X.Y.Z`) | Yes -- ldflags-injected |
| Version var | `cmd.Version` (set via `-ldflags "-X github.com/CosmoLabs-org/SmokeSig/cmd.Version=X.Y.Z"`) | Yes |
| Root command `Use` | `smokesig` | Yes |

### SmokeSig Exit Codes (observed from `cmd/run.go`, `cmd/root.go`)

| Exit Code | Meaning | Source |
|-----------|---------|--------|
| `0` | All tests passed (or `--dry-run` completed) | Normal return from `runSmoke()` |
| `1` | One or more tests failed | `os.Exit(1)` when `result.Failed > 0` |
| `1` | Config load/validation error | Cobra `RunE` returns error, root `Execute()` calls `os.Exit(1)` |
| `1` | Prerequisite failure | Runner returns error, Cobra propagates |
| `1` | Watch mode interrupted (SIGINT/SIGTERM) | Clean shutdown, returns nil (exit 0 actually) |

**Note:** SmokeSig currently uses only exit codes 0 and 1. There is no distinction between "tests failed" (exit 1) and "config error" (exit 1). CCS can differentiate by parsing stderr for specific error prefixes (`error: loading config:`, `error:`) vs the JSON output structure, but the exit code alone is binary pass/fail.

### SmokeSig JSON Output Contract (`--format json`)

The JSON reporter emits a `jsonOutput` struct to stdout:

```json
{
  "project": "MyProject",
  "total": 6,
  "passed": 5,
  "failed": 1,
  "skipped": 0,
  "allowed_failures": 0,
  "duration_ms": 2340,
  "prerequisites": [
    {"name": "Go installed", "passed": true, "output": "go1.26.2"}
  ],
  "tests": [
    {
      "name": "Compiles",
      "passed": true,
      "duration_ms": 890,
      "assertions": [{"type": "exit_code", "expected": "0", "actual": "0", "passed": true}]
    }
  ]
}
```

This is the machine-consumption format CCS should use when it needs structured results (e.g., for `ccs health-check`, `/audit`, or session-start hooks).

### CCS Integration Points (from `2026-04-15-ccs-integration-vision.md`)

The original vision document defines 6 integration points. All reference old names. The architecture is sound -- SmokeSig remains 100% standalone, CCS calls it via Bash:

```
SmokeSig (standalone binary)
    ^
    | exec via os/exec or shell
    |
CCS Integration Layer
    - ccs smoke        -> smokesig run (passthrough)
    - ccs rebuild smokesig -> clone + go build + install
    - ccs sync         -> verify smokesig on PATH
    - /health-check    -> smokesig validate + smokesig run
    - /audit           -> check .smokesig.yaml exists
    - /project-init    -> offer smokesig init
```

## Design Decisions

### Q1: Should SmokeSig expose a Go API (importable package) or remain CLI-only?

**Decision: CLI-only. No Go API.**

Rationale:
- SmokeSig's `internal/` packages are intentionally internal. Exposing them creates a coupling surface that constrains SmokeSig's evolution.
- CCS already has the pattern for CLI-only external tools: `go`, `git`, `gh`, `bun` are all called via `exec.Command()`. SmokeSig fits this pattern exactly.
- The JSON output format (`--format json`) provides all the structured data CCS needs. Parsing JSON is trivial compared to maintaining a Go API contract.
- SmokeSig can be open-sourced independently without worrying about breaking CCS imports.
- Performance is not a concern: smoke tests run in 2-5 seconds, and exec overhead is negligible.

**What this means for CCS:** `ccs smoke` shells out to `smokesig` via `exec.Command("smokesig", args...)`. It never imports SmokeSig Go packages.

### Q2: Version pinning -- how does CCS know which SmokeSig version to build?

**Decision: Git tag pinning in `.binary-registry.json`, not go.mod replace.**

Options considered:
1. **go.mod replace directive** -- Would require SmokeSig as a Go module dependency of CCS. Rejected: CCS should not `import` SmokeSig, and `replace` for a CLI binary is an abuse of the mechanism.
2. **Hardcoded version in `rebuild.go`** -- Fragile, requires code changes to bump. Rejected.
3. **Git tag in `.binary-registry.json`** -- The registry already tracks binary metadata. Add a `version` or `tag` field that `ccs rebuild smokesig` uses for `git checkout`. Accepted.
4. **Always build from HEAD of master** -- Simple but non-reproducible. Could be a default with tag override.

Recommended `.binary-registry.json` entry:

```json
{
  "smokesig": {
    "repo": "github.com/CosmoLabs-org/SmokeSig",
    "binary": "smokesig",
    "build_cmd": "go build -ldflags \"-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version={{.Version}}\" -o smokesig .",
    "install_path": "~/bin/smokesig",
    "version": "v0.23.0",
    "pin": "tag"
  }
}
```

`ccs rebuild smokesig` workflow:
1. Clone `github.com/CosmoLabs-org/SmokeSig` to temp dir (or use cached clone)
2. `git checkout v0.23.0` (from registry `version` field)
3. `go build` with ldflags injecting the version
4. Copy binary to `~/bin/smokesig`
5. Verify: `smokesig version` output matches expected version

**Version bump workflow:** When SmokeSig releases a new version, update `.binary-registry.json` in CCS. `ccs rebuild smokesig` picks up the new tag automatically.

### Q3: Config migration -- should SmokeSig auto-detect `.smoke.yaml` as fallback?

**Decision: Already implemented in code, but NOT wired into production paths.**

LoadDefault() exists at schema.go:496 but is DEAD CODE — it is never called from the run or validate commands. The config file flag defaults to `.smokesig.yaml` with no fallback to `.smoke.yaml`. This means CCS Phase 3 config migration is REQUIRED before removing `.smoke.yaml` files from downstream projects. The plan must wire LoadDefault() into the run/validate command paths.

The deprecation warning goes to stderr, so it does not pollute JSON output on stdout. CCS can choose to surface or suppress it.

**Monorepo discovery** (`internal/monorepo/`) also checks both filenames.

### Q4: Should `ccs smoke` add CCS-specific flags?

**Decision: No CCS-specific flags in v1. Pure passthrough.**

Options considered:
1. **`--project-type` from `ccs intel`** -- Would let CCS tell SmokeSig what kind of project it is. But SmokeSig already auto-detects project type in `smokesig init`. Adding a CCS-injected flag creates a dependency direction violation (SmokeSig should not need CCS context to run).
2. **`--ccs-context`** -- Inject CCS project metadata. Same problem.
3. **`--format json` auto-add for programmatic callers** -- CCS could always append `--format json` when it needs structured output (e.g., in `/health-check`). But `ccs smoke` is a user-facing passthrough -- it should forward exactly what the user typed. Internal CCS consumers (health-check, audit) can add `--format json` themselves.

**The passthrough principle:** `ccs smoke [args]` is equivalent to `smokesig [args]`. No magic, no injection, no surprises. CCS adds value through orchestration (rebuild, sync, health-check integration), not by modifying SmokeSig's behavior.

**Future consideration:** If CCS needs to pass project context to SmokeSig (e.g., for dashboard registration), use environment variables (`SMOKESIG_PROJECT`, `SMOKESIG_DASHBOARD_URL`) rather than flags. SmokeSig can read these without creating a CLI dependency on CCS.

## SmokeSig Changes Required

SmokeSig is already in good shape after the rename. The changes needed are minimal and mostly about hardening the contract for external consumers.

### SC-1: Formalize Exit Code Contract

Currently SmokeSig uses only 0 and 1. Consider distinguishing:

| Exit Code | Meaning |
|-----------|---------|
| `0` | All tests passed |
| `1` | One or more tests failed |
| `2` | Configuration error (file not found, parse error, validation error) |
| `3` | Prerequisite failure (required tool missing) |

**Impact:** CCS could react differently to "tests failed" vs "config broken" vs "missing prereqs". For example, `ccs health-check` might report "SmokeSig config invalid" (fix config) vs "smoke tests failing" (fix code) vs "missing tools" (install deps).

**Recommendation:** Implement in SmokeSig as a separate FEAT. Not blocking for FEAT-052 -- CCS can parse stderr or JSON output to distinguish error types in the meantime. But richer exit codes are a better long-term contract.

### SC-2: Version Output Stability

Current format: `smokesig 0.23.0` (space-separated, no `v` prefix in output).

CCS should parse this with: `smokesig version | awk '{print $2}'` or regex `^smokesig\s+(\S+)`.

This format is stable. No changes needed unless CCS wants additional metadata (e.g., `smokesig 0.23.0 (go1.26.2, darwin/arm64)`). That would be a nice-to-have but not required for FEAT-052.

### SC-3: JSON Output Completeness

The JSON reporter (`internal/reporter/json.go`) outputs everything CCS needs. The `jsonOutput` struct includes project name, counts (total/passed/failed/skipped/allowed_failures), duration, prerequisites, and per-test details with assertion breakdowns.

No changes needed. The JSON contract is already sufficient for machine consumption.

### SC-4: Config Discovery Documentation

SmokeSig should document its config discovery order explicitly (for CCS developers and other consumers):

1. `--file / -f` flag -- explicit path, highest priority
2. `.smokesig.yaml` in current directory (or config dir)
3. `.smoke.yaml` fallback with deprecation warning
4. `--env NAME` loads `NAME.smokesig.yaml` as overlay

This is already implemented but not formally documented as a contract. Add to README or SPEC.

## CCS Changes Required

These are documented here for CCS developers implementing FEAT-052. All changes happen in the CCS repo (`CosmoLabs-org/ClaudeCodeSetup`).

### CC-1: `smoke.go` Passthrough Rewrite

Current state (broken): References `cosmo-smoke` or `smoke` binary name.

Required changes:
- Binary lookup: `exec.LookPath("smokesig")` -- find on PATH
- If not found: print actionable error: `smokesig not found. Install: ccs rebuild smokesig`
- Arg forwarding: `ccs smoke run --tag build` -> `smokesig run --tag build`
- Subcommand mapping: `ccs smoke` with no args -> `smokesig run` (convenience default)
- Exit code propagation: Forward SmokeSig's exit code as CCS's exit code
- Stderr/stdout: Pipe through directly (no capture unless CCS needs to parse)

```go
// Pseudocode for smoke.go
func runSmoke(args []string) error {
    bin, err := exec.LookPath("smokesig")
    if err != nil {
        return fmt.Errorf("smokesig not found on PATH. Install with: ccs rebuild smokesig")
    }
    if len(args) == 0 {
        args = []string{"run"}
    }
    cmd := exec.Command(bin, args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            os.Exit(exitErr.ExitCode())
        }
        return err
    }
    return nil
}
```

### CC-2: `rebuild.go` SmokeSig Entry

Add SmokeSig to the list of external binaries `ccs rebuild` can build:

- Repository: `github.com/CosmoLabs-org/SmokeSig`
- Build command: `go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version={{version}}" -o smokesig .`
- Install destination: `~/bin/smokesig` (or wherever CCS installs external binaries)
- Post-build verification: `smokesig version` output contains expected version string

`ccs rebuild smokesig` should:
1. Clone (or pull) the SmokeSig repo
2. Checkout the pinned version tag from `.binary-registry.json`
3. Run `go build` with ldflags
4. Install to PATH
5. Verify with `smokesig version`

### CC-3: `.binary-registry.json` Entry

Add SmokeSig entry with:
- `repo`: GitHub URL
- `binary`: `smokesig`
- `version`: pinned git tag (e.g., `v0.23.0`)
- `build_cmd`: go build with ldflags template
- `install_path`: target location

### CC-4: `ccs sync` Dependency Check

Add SmokeSig to the external dependency verification in `ccs sync`:

```
Checking external dependencies...
  go:       go1.26.2     OK
  git:      2.47.0       OK
  gh:       2.74.0       OK
  smokesig: 0.23.0       OK       (or: NOT FOUND - run 'ccs rebuild smokesig')
```

This should be a soft check (warning, not error) since SmokeSig is optional -- not all CCS users run smoke tests.

### CC-5: Documentation Updates in CCS

All CCS docs referencing the old names need updating:

| Old Reference | New Reference |
|---------------|---------------|
| `cosmo-smoke` | `SmokeSig` (product name) or `smokesig` (binary) |
| `smoke` (binary) | `smokesig` |
| `.smoke.yaml` | `.smokesig.yaml` |
| `smoke run` | `smokesig run` |
| `ccs rebuild cosmo-smoke` | `ccs rebuild smokesig` |
| `CosmoLabs-org/cosmo-smoke` | `CosmoLabs-org/SmokeSig` |

Files to check in CCS (non-exhaustive):
- `CLAUDE.md` -- already references `ccs smoke` and `ccs rebuild smokesig` (may be current)
- `smoke.go` -- command implementation
- `rebuild.go` -- build targets
- Skills referencing smoke tests (e.g., `health-check`, `audit`, `session-end`)
- Rules files referencing `.smoke.yaml`
- Any `docs/` files mentioning cosmo-smoke

## Migration Plan

### Phase 1: CCS Code Changes (FEAT-052 scope)

1. Update `smoke.go` to look for `smokesig` binary (CC-1)
2. Add SmokeSig to `rebuild.go` (CC-2)
3. Add `.binary-registry.json` entry (CC-3)
4. Update `ccs sync` dependency check (CC-4)
5. Review and commit the 4 files from FB-010

### Phase 2: CCS Documentation Sweep

1. Grep CCS repo for all `cosmo-smoke`, `cosmo_smoke`, `.smoke.yaml` references
2. Update to new names
3. Verify `ccs smoke --help` output is correct

### Phase 3: Cross-Project Config Migration

For all CosmoLabs projects that have `.smoke.yaml`:
1. Rename to `.smokesig.yaml`
2. Update any CI workflows referencing `smoke run` to `smokesig run`
3. SmokeSig's fallback handles the transition gracefully -- no rush

### Phase 4: SmokeSig Contract Hardening (optional, separate FEATs)

1. Formalize exit codes (SC-1) -- separate FEAT
2. Add build metadata to version output (SC-2) -- nice-to-have
3. Document config discovery contract in README (SC-4)

## CI Integration Pattern

For CosmoLabs projects using GitHub Actions:

```yaml
# .github/workflows/smoke.yml
name: Smoke Tests
on: [push, pull_request]
jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Install SmokeSig
        run: |
          go install github.com/CosmoLabs-org/SmokeSig@v0.23.0
      - name: Run smoke tests
        run: smokesig run --format terminal,gha
```

The `gha` format writes to `$GITHUB_STEP_SUMMARY` and emits `::error`/`::warning` annotations. This is SmokeSig's native CI integration -- CCS does not need to wrap it.

For projects using `ccs` in CI:

```yaml
      - name: Run smoke tests
        run: ccs smoke
```

## Scope Boundaries

### In scope for FEAT-052
- CCS `smoke.go` rewrite to find `smokesig` binary
- CCS `rebuild.go` SmokeSig build target
- CCS `.binary-registry.json` entry
- CCS `sync` dependency check
- Commit the FB-010 partial changes
- Documentation sweep for old names in CCS

### Out of scope for FEAT-052
- SmokeSig exit code formalization (separate FEAT, SC-1)
- SmokeSig Go API exposure (rejected, Q1)
- CCS-specific SmokeSig flags (rejected, Q4)
- `go.mod` replace directive (rejected, Q2)
- SmokeSig version output changes (SC-2, nice-to-have)
- GitHub Actions reusable workflow for SmokeSig (separate work)
- Dashboard integration between CCS and SmokeSig serve (future)
- Auto-smoke on session start (future CCS hook, documented in vision doc)

### Risks and Mitigations

- **Version mismatch**: `ccs sync` should compare installed `smokesig version` output against `.binary-registry.json` pinned version and warn on drift. If the installed binary reports `0.21.1` but the registry pins `v0.23.0`, `ccs sync` should emit a warning with `ccs rebuild smokesig` as the remediation command.

- **Subcommand passthrough**: ALL subcommands pass through verbatim to `smokesig` (not just `run`). `ccs smoke validate`, `ccs smoke init`, `ccs smoke serve`, `ccs smoke schema`, `ccs smoke version` all forward unchanged. The passthrough layer must not hardcode a subcommand whitelist -- it forwards whatever the user typed after `ccs smoke`.

- **Multi-machine sync**: If machine A has v0.23.0 and machine B has v0.21.1, smoke test behavior may differ (new assertion types, changed defaults, fixed bugs). The `ccs sync` dependency check should flag version mismatches across machines. Since `.binary-registry.json` is git-tracked, both machines share the same pinned version -- the drift happens when one machine has not run `ccs rebuild smokesig` after a registry bump.

- **Binary exists but wrong version**: `smokesig version` may return an unexpected format (e.g., `dev`, empty string, multi-line output with build metadata). CCS should parse gracefully: extract the first token matching `\d+\.\d+\.\d+` from the output, and if no match is found, warn "unable to determine smokesig version" rather than crashing or reporting a false mismatch.

## Cross-References

| Document | Location | Relevance |
|----------|----------|-----------|
| CCS Integration Vision | `docs/brainstorming/2026-04-15-ccs-integration-vision.md` | Original architecture (uses old names) |
| Rename Brainstorm | `docs/brainstorming/2026-05-05-rename-to-smokesig.md` | SmokeSig-side rename details |
| FB-010 | CCS feedback inbox | 4 CCS files partially updated |
| FEAT-052 | CCS issue tracker | Parent issue for this work |
| SmokeSig CLAUDE.md | `CLAUDE.md` | Current binary contract, commands, assertion types |
| SmokeSig self-smoke | `.smokesig.yaml` | SmokeSig's own smoke test config (reference for format) |
