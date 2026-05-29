---
brainstorm: docs/brainstorming/2026-05-29-ccs-dependency-wiring.md
created: "2026-05-29T12:00:00-03:00"
issue: FEAT-052
status: PENDING
deliverables:
  - id: P-01
    title: "Wire LoadDefault() fallback into run/validate commands"
  - id: P-02
    title: "Formalize exit code contract"
  - id: P-03
    title: "CCS smoke.go passthrough command"
  - id: P-04
    title: "CCS rebuild.go SmokeSig entry"
  - id: P-05
    title: "CCS .binary-registry.json entry + sync check"
  - id: P-06
    title: "Documentation sweep and migration guide"
---

# Implementation Plan: Wire SmokeSig as External Dependency in CCS

**Goal:** Complete the SmokeSig/CCS integration so `ccs smoke` delegates to the `smokesig` binary, `ccs rebuild smokesig` builds it from source, and `ccs sync` verifies it is installed. SmokeSig-side changes (P-01, P-02) harden the binary contract. CCS-side changes (P-03-P-05) are documented here but implemented in the CCS repo. P-06 is a cross-project documentation sweep.

**Current version:** v0.23.0 (latest tag); `cmd/version.go` default is `0.21.1` (overridden by ldflags at build time)

---

## P-01: Wire LoadDefault() Fallback into run/validate Commands

**Problem:** `schema.LoadDefault()` (schema.go:496-505) implements `.smokesig.yaml` -> `.smoke.yaml` fallback with a deprecation warning. But it is **dead code** -- never called from any command. Both `cmd/run.go` and `cmd/validate.go` hardcode `.smokesig.yaml`:

- `run.go:147` -- flag default: `StringVarP(&configFile, "file", "f", ".smokesig.yaml", ...)`
- `run.go:172` -- `loadConfig()` calls `schema.Load(configFile)` directly (no fallback)
- `validate.go:16-18` -- uses `cmd.Flags().GetString("file")` but the `-f` flag is **never registered on `validateCmd`** (only on `runCmd`), so it always returns `""`, triggering the hardcoded `.smokesig.yaml` fallback at line 18. This also means `smokesig validate -f custom.yaml` silently fails (unknown flag error).

This means a project with only `.smoke.yaml` (pre-rename) gets a hard error instead of the intended deprecation warning + graceful fallback.

### Files

| File | Change |
|------|--------|
| `cmd/run.go` | Change flag default to `""`, branch `loadConfig()` on empty; track resolved path for `configDir` |
| `cmd/validate.go` | Register `-f` flag on `validateCmd`; use `LoadDefault()` when flag is empty |
| `internal/schema/schema.go` | Add `LoadDefaultPath() (string, error)` to return resolved filename without loading |
| `cmd/run_test.go` | Add test for fallback behavior |

### Steps

- [ ] Add `LoadDefaultPath() (string, error)` to `internal/schema/schema.go` -- returns the resolved config filename (`.smokesig.yaml` or `.smoke.yaml`) without loading, prints deprecation warning for `.smoke.yaml`
- [ ] Refactor `LoadDefault()` to call `LoadDefaultPath()` then `Load(path)`
- [ ] Modify `cmd/run.go` flag default from `.smokesig.yaml` to empty string
- [ ] Modify `loadConfig()` to branch: if `configFile == ""`, call `schema.LoadDefaultPath()` to resolve the path, assign to `configFile`, then proceed with `schema.Load(configFile)`. This preserves correct `configDir` derivation downstream at line 211 (`filepath.Dir(configFile)`)
- [ ] **Register `-f` flag on `validateCmd`** in its `init()` -- currently missing entirely, making `smokesig validate -f custom.yaml` an unknown flag error
- [ ] Modify `cmd/validate.go` to call `LoadDefaultPath()` when `configFile == ""`, then pass resolved path to `runValidate()`
- [ ] Add test: project with only `.smoke.yaml` triggers deprecation warning on stderr and loads successfully
- [ ] Add test: project with `.smokesig.yaml` loads without warning
- [ ] Add test: project with neither file returns clear error
- [ ] Add test: `smokesig validate -f custom.yaml` works (regression for missing flag)
- [ ] Verify existing tests still pass

### Code

**cmd/run.go** -- flag registration (line 147):

```go
// Before:
runCmd.Flags().StringVarP(&configFile, "file", "f", ".smokesig.yaml", "Config file path")

// After:
runCmd.Flags().StringVarP(&configFile, "file", "f", "", "Config file path (default: .smokesig.yaml, falls back to .smoke.yaml)")
```

**internal/schema/schema.go** -- new `LoadDefaultPath()`:

```go
// LoadDefaultPath resolves the config filename without loading.
// Returns ".smokesig.yaml" if it exists, falls back to ".smoke.yaml"
// with a deprecation warning, or returns an error if neither exists.
func LoadDefaultPath() (string, error) {
	if _, err := os.Stat(".smokesig.yaml"); err == nil {
		return ".smokesig.yaml", nil
	}
	if _, err := os.Stat(".smoke.yaml"); err == nil {
		fmt.Fprintln(os.Stderr, "⚠ Config file .smoke.yaml is deprecated, rename to .smokesig.yaml")
		return ".smoke.yaml", nil
	}
	return "", fmt.Errorf("no config file found: .smokesig.yaml or .smoke.yaml")
}

// LoadDefault finds and loads the config from the current directory.
func LoadDefault() (*SmokeConfig, error) {
	path, err := LoadDefaultPath()
	if err != nil {
		return nil, err
	}
	return Load(path)
}
```

**cmd/run.go** -- `loadConfig()` (line 170-175):

```go
func loadConfig() (*schema.SmokeConfig, error) {
	// Resolve config path: explicit -f flag takes priority, otherwise auto-detect
	if configFile == "" {
		resolved, err := schema.LoadDefaultPath()
		if err != nil {
			return nil, err
		}
		configFile = resolved  // Assign back so configDir derivation at line 211 works correctly
	}

	cfg, err := schema.Load(configFile)
	if err != nil {
		return nil, err
	}
	// ... rest of loadConfig unchanged (env merge, CLI flag overrides)
```

> **Critical: `configFile` must be assigned the resolved path** so that `configDir := filepath.Dir(configFile)` at line 211 still works. If `configFile` stays `""`, `filepath.Dir("")` returns `"."` which is incidentally correct for the cwd case, but breaks if `LoadDefaultPath()` ever supports parent-directory search.

**cmd/validate.go** -- full file (register flag + use fallback):

```go
var validateCmd = &cobra.Command{
	Use:   "validate [-f path]",
	Short: "Validate smoke test config without running tests",
	Long:  "Load and validate .smokesig.yaml configuration. Reports all errors at once.",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			resolved, err := schema.LoadDefaultPath()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return err
			}
			file = resolved
		}
		out, err := runValidate(file)
		if err != nil {
			fmt.Fprint(os.Stderr, out)
			return err
		}
		fmt.Fprint(os.Stdout, out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "", "Config file path (default: .smokesig.yaml, falls back to .smoke.yaml)")
}
```

> **Bug fix: validate's `-f` flag was never registered.** The old code used `cmd.Flags().GetString("file")` which silently returned `""` since only `runCmd` registered the flag. `smokesig validate -f custom.yaml` was an unknown flag error. Now `validateCmd` registers its own `-f` flag.

### Test Commands

```bash
go test ./cmd/ -run TestLoadDefault -v
go test ./internal/schema/ -run TestLoadDefault -v
go test ./... -count=1
```

### Commit

```
fix(config): wire LoadDefault() fallback into run and validate commands

LoadDefault() implements .smokesig.yaml -> .smoke.yaml fallback with
deprecation warning but was never called from any command path.
Both run.go and validate.go hardcoded .smokesig.yaml, breaking
backward compatibility for pre-rename projects.
```

---

## P-02: Formalize Exit Code Contract

**Problem:** SmokeSig uses only exit codes 0 and 1. Config errors, test failures, and prerequisite failures all return exit 1. CCS (and other consumers) cannot distinguish "tests failed" from "config broken" without parsing stderr.

### Exit Code Contract

| Code | Meaning | When |
|------|---------|------|
| `0` | All tests passed (or `--dry-run` completed) | Normal return |
| `1` | One or more tests failed | `result.Failed > 0` |
| `2` | Configuration error | File not found, YAML parse error, validation error |
| `3` | Prerequisite failure | Required tool missing, prereq command non-zero |

### Files

| File | Change |
|------|--------|
| `cmd/exitcodes.go` (new) | Constants for exit codes |
| `cmd/run.go` | Use specific exit codes instead of blanket `os.Exit(1)` |
| `cmd/root.go` | Map Cobra errors to exit code 2 |
| `cmd/validate.go` | Return exit 2 on validation errors |
| `internal/runner/runner.go` | Return typed errors for prereq failures |
| `cmd/run_test.go` | Test exit code scenarios |

### Steps

- [ ] Create `cmd/exitcodes.go` with constants: `ExitPass = 0`, `ExitFail = 1`, `ExitConfigError = 2`, `ExitPrereqFailure = 3`
- [ ] Set `rootCmd.SilenceErrors = true` to prevent Cobra double-printing errors
- [ ] Modify `cmd/root.go:Execute()` to detect config/validation errors and exit with code 2
- [ ] Modify `cmd/run.go` -- `runSmoke()` error handling to distinguish error types
- [ ] Create sentinel error types: `ConfigError` (wraps load/parse/validate), `PrereqError` (wraps prereq failures)
- [ ] Wrap errors from `loadConfig()` in `ConfigError` -- covers file not found, YAML parse, and validation errors
- [ ] Wrap errors from runner prerequisite checks in `PrereqError`
- [ ] Modify `cmd/run.go:292` and `cmd/run.go:355` -- replace `os.Exit(1)` with `os.Exit(ExitFail)` (value unchanged but uses constant for clarity)
- [ ] `validate.go` errors should also return `ConfigError` so `Execute()` maps them to exit 2
- [ ] Add `--dry-run` exit 0 (already correct, verify)
- [ ] Watch mode SIGINT/SIGTERM -- exit 0 (already correct, verify)
- [ ] Add tests: config error -> exit 2, prereq failure -> exit 3, test failure -> exit 1, all pass -> exit 0
- [ ] Document contract in README and `smokesig schema` output

### Code

**cmd/exitcodes.go** (new file):

```go
package cmd

const (
	ExitPass           = 0
	ExitFail           = 1
	ExitConfigError    = 2
	ExitPrereqFailure  = 3
)
```

**Error type wrapping** (in `cmd/run.go` or a shared errors file):

```go
type ConfigError struct{ Err error }
func (e *ConfigError) Error() string { return e.Err.Error() }
func (e *ConfigError) Unwrap() error { return e.Err }

type PrereqError struct{ Err error }
func (e *PrereqError) Error() string { return e.Err.Error() }
func (e *PrereqError) Unwrap() error { return e.Err }
```

**cmd/root.go** -- `Execute()` exit code mapping:

```go
func init() {
	// Prevent Cobra from printing errors itself -- we handle formatting and exit codes
	rootCmd.SilenceErrors = true
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var cfgErr *ConfigError
		var preErr *PrereqError
		switch {
		case errors.As(err, &cfgErr):
			os.Exit(ExitConfigError)
		case errors.As(err, &preErr):
			os.Exit(ExitPrereqFailure)
		default:
			os.Exit(ExitFail)
		}
	}
}
```

> **Note:** `rootCmd.SilenceErrors = true` is required. Without it, Cobra prints the error via its own handler AND our `Execute()` prints it again, producing duplicate output. The existing code already sets `fmt.Fprintln(os.Stderr, err)` + `os.Exit(1)`, so we are just replacing the exit code logic. Also add `rootCmd.SilenceUsage = true` in RunE error paths to avoid printing usage on config errors (users don't need flag help when their YAML is broken).

### Test Commands

```bash
go test ./cmd/ -run TestExitCode -v
go test ./... -count=1
```

### Commit

```
feat(cli): formalize exit code contract (0=pass, 1=fail, 2=config, 3=prereq)

Distinguishes test failures from config errors and prerequisite
failures via distinct exit codes. Enables CCS and CI consumers to
react differently without parsing stderr.

Exit codes: 0=pass, 1=test failure, 2=config error, 3=prereq failure.
```

---

## P-03: CCS smoke.go Passthrough Command

> **Scope note:** This task is implemented in the CCS repo (`CosmoLabs-org/ClaudeCodeSetup`), not SmokeSig. Documented here so the full integration surface is visible in one plan.

**Problem:** CCS `smoke.go` references the old `cosmo-smoke` / `smoke` binary name. It needs to find `smokesig` on PATH and forward all arguments verbatim.

### Design

- `ccs smoke [args...]` -> `smokesig [args...]` (all subcommands pass through: `run`, `validate`, `schema`, `init`, `version`, `serve`)
- `ccs smoke` with no args -> `smokesig run` (convenience default, matches current behavior)
- Binary lookup via `exec.LookPath("smokesig")`
- If not found: actionable error with install instructions
- Exit code: propagate SmokeSig's exit code as CCS's exit code
- Stdin/stdout/stderr: pipe through directly, no capture

### CCS Files to Modify

| File | Change |
|------|--------|
| `cmd/smoke.go` | Rewrite binary lookup to `smokesig`, verbatim arg forwarding |

### Implementation Sketch

```go
func runSmoke(args []string) error {
    bin, err := exec.LookPath("smokesig")
    if err != nil {
        return fmt.Errorf("smokesig not found on PATH. Install with: ccs rebuild smokesig")
    }
    // No arg transformation -- all subcommands pass through verbatim
    if len(args) == 0 {
        args = []string{"run"}
    }
    cmd := exec.Command(bin, args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    // cmd.Dir defaults to "" which inherits cwd -- no need to set explicitly
    if err := cmd.Run(); err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            os.Exit(exitErr.ExitCode())
        }
        return err
    }
    return nil
}
```

### Key Details

- [ ] Binary name: `smokesig` (not `smoke`, not `cosmo-smoke`)
- [ ] All subcommands forwarded: `run`, `validate`, `schema`, `init`, `version`, `serve`
- [ ] No CCS-specific flag injection (design decision Q4 from brainstorm)
- [ ] SIGTERM: if CCS receives SIGTERM, forward to child process for graceful shutdown
- [ ] Exit code mapping: propagate verbatim. With P-02 implemented, CCS gets 0/1/2/3

### Commit (CCS repo)

```
fix(smoke): rewrite passthrough to use smokesig binary

Replaces stale cosmo-smoke/smoke binary references with smokesig.
All subcommands forwarded verbatim. Exit codes propagated.
```

---

## P-04: CCS rebuild.go SmokeSig Entry

> **Scope note:** CCS repo change. Documented here for completeness.

**Problem:** `ccs rebuild` does not include SmokeSig as a buildable external dependency. Users must manually clone and build.

### Design

`ccs rebuild smokesig` workflow:

1. Clone `github.com/CosmoLabs-org/SmokeSig` to temp dir (or use cached clone in `~/.cache/ccs/repos/`)
2. Checkout pinned version tag from `.binary-registry.json` (`version` field)
3. `go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version={{version}}" -o smokesig .`
4. Copy binary to `~/bin/smokesig` (or CCS install path)
5. Verify: `smokesig version` output contains expected version string
6. Print success with installed path and version

### CCS Files to Modify

| File | Change |
|------|--------|
| `cmd/rebuild.go` | Add `smokesig` to build targets with repo, ldflags template, install path |

### Key Details

- [ ] Repo URL: `github.com/CosmoLabs-org/SmokeSig`
- [ ] Build flags: `-s -w` (strip debug, reduce binary size) + `-X` version injection
- [ ] Version source: `.binary-registry.json` `smokesig.version` field
- [ ] Post-build verify: run `smokesig version`, parse output, confirm match
- [ ] Support `ccs rebuild --only smokesig` for targeted rebuild
- [ ] Support `ccs rebuild --all` includes SmokeSig

### Commit (CCS repo)

```
feat(rebuild): add SmokeSig as external buildable dependency

ccs rebuild smokesig clones the repo, checks out the pinned version
tag, builds with ldflags, and installs to ~/bin. Post-build
verification confirms version match.
```

---

## P-05: CCS .binary-registry.json Entry + Sync Check

> **Scope note:** CCS repo change. Documented here for completeness.

### Part A: Registry Entry

Add SmokeSig to `.binary-registry.json`:

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

### Part B: Version Mismatch Detection in `ccs sync`

Add SmokeSig to the external dependency check in `ccs sync`:

```
Checking external dependencies...
  go:       go1.26.2     OK
  git:      2.47.0       OK
  gh:       2.74.0       OK
  smokesig: 0.23.0       OK

  # or if version mismatch:
  smokesig: 0.21.1       ⚠ pinned v0.23.0, run 'ccs rebuild smokesig'

  # or if missing:
  smokesig: NOT FOUND    ⚠ optional, run 'ccs rebuild smokesig' to install
```

### Key Details

- [ ] Registry version: `v0.23.0` (current, NOT stale v0.21.1 from brainstorm)
- [ ] Use `{{latest}}` placeholder or template var in build_cmd for version injection
- [ ] Sync check: soft warning (SmokeSig is optional, not all users need it)
- [ ] Version parse: `smokesig version` -> regex `^smokesig\s+(\S+)` -> compare to pinned
- [ ] Mismatch detection: compare installed version vs `.binary-registry.json` `version` field
- [ ] `ccs sync` output includes SmokeSig in the dependency table

### CCS Files to Modify

| File | Change |
|------|--------|
| `.binary-registry.json` | Add `smokesig` entry |
| `cmd/sync.go` | Add SmokeSig to dependency check loop, version comparison |

### Commit (CCS repo)

```
feat(sync): add SmokeSig to binary registry and sync dependency check

Pins SmokeSig v0.23.0 in .binary-registry.json. ccs sync now
verifies smokesig is installed and version matches the pin.
Warns on mismatch with actionable rebuild command.
```

---

## P-06: Documentation Sweep and Migration Guide

Cross-project documentation update. Two scopes: SmokeSig docs and CCS docs.

### SmokeSig-Side Documentation

- [ ] Document config discovery order in README (formalize the contract from SC-4):
  1. `--file / -f` flag (explicit path, highest priority)
  2. `.smokesig.yaml` in working directory
  3. `.smoke.yaml` fallback with deprecation warning
  4. `--env NAME` loads `NAME.smokesig.yaml` as overlay
- [ ] Document exit code contract in README (after P-02 is implemented)
- [ ] Document JSON output contract (already in CLAUDE.md, add to README)
- [ ] Document version output format: `smokesig X.Y.Z` (space-separated, no `v` prefix)

### CCS-Side Documentation (CCS repo)

- [ ] Grep CCS repo for stale references: `cosmo-smoke`, `cosmo_smoke`, `.smoke.yaml`, `smoke run` (not `ccs smoke`)
- [ ] Update all matches to new names per mapping:

| Old | New |
|-----|-----|
| `cosmo-smoke` | `SmokeSig` (product) or `smokesig` (binary) |
| `smoke` (binary name) | `smokesig` |
| `.smoke.yaml` | `.smokesig.yaml` |
| `smoke run` (direct invocation) | `smokesig run` |
| `ccs rebuild cosmo-smoke` | `ccs rebuild smokesig` |
| `CosmoLabs-org/cosmo-smoke` | `CosmoLabs-org/SmokeSig` |

- [ ] Verify `ccs smoke --help` output references `smokesig` correctly
- [ ] Check skills that reference smoke: `health-check`, `audit`, `session-end`

### Cross-Project Migration Guide

For CosmoLabs projects still using `.smoke.yaml`:

1. Rename `.smoke.yaml` -> `.smokesig.yaml` (SmokeSig handles both, but new name is canonical)
2. Update CI workflows: `smoke run` -> `smokesig run`
3. Update CI install: `go install github.com/CosmoLabs-org/SmokeSig@v0.23.0`
4. SmokeSig's `.smoke.yaml` fallback handles the transition gracefully -- no hard deadline

### Commit (SmokeSig repo)

```
docs: document config discovery, exit codes, and version output contract

Formalizes the external consumer contract: config file fallback
order, exit code meanings, JSON output structure, and version
output format.
```

---

## Execution Order

```
Phase 1 — SmokeSig-side (this repo)
  P-01: Wire LoadDefault() fallback ←── prerequisite for migration
  P-02: Exit code contract          ←── prerequisite for CCS exit code mapping

Phase 2 — CCS-side (ClaudeCodeSetup repo)
  P-03: smoke.go passthrough        ←── depends on P-01 (binary contract stable)
  P-04: rebuild.go entry            ←── independent
  P-05: registry + sync check       ←── depends on P-04 (needs registry entry)

Phase 3 — Cross-project
  P-06: Documentation sweep         ←── after all code changes landed
```

P-01 and P-02 can be implemented in parallel. P-03, P-04, and P-05 can begin after P-01 lands (P-02 is nice-to-have for CCS but not blocking). P-06 is last.

## Testing Strategy

| Deliverable | Test Approach |
|-------------|---------------|
| P-01 | Unit tests for `LoadDefault()` call path in run/validate; integration: project with `.smoke.yaml` only |
| P-02 | Unit tests for each exit code scenario; integration: bad config -> exit 2, missing prereq -> exit 3 |
| P-03 | CCS test: mock `smokesig` binary, verify arg forwarding and exit code propagation |
| P-04 | CCS test: `ccs rebuild smokesig` in temp dir with mocked git clone |
| P-05 | CCS test: version parse, mismatch detection, registry entry validity |
| P-06 | Grep-based: zero matches for old names after sweep |

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| P-01 changes default flag behavior | Empty string default + `LoadDefault()` preserves all explicit `-f` usage; only changes the "no flag" path |
| P-02 breaks consumers expecting exit 1 for config errors | Exit 1 was never documented as a contract; consumers checking `!= 0` are unaffected; only `== 1` checks need updating |
| CCS repo changes (P-03-P-05) depend on SmokeSig version | Pin to released tag, not HEAD; verify after build |
| Stale version in registry | `ccs sync` version mismatch detection (P-05) catches this proactively |
| `cmd/version.go` default is `0.21.1` | Overridden by ldflags at build time; only affects `go run .` during development. Consider bumping the default to match latest tag to avoid confusion. |
