---
plan: docs/planning-mode/2026-05-29-ccs-dependency-wiring.md
brainstorm: docs/brainstorming/2026-05-29-ccs-dependency-wiring.md
reviewed: "2026-05-29T14:30:00-03:00"
reviewer: opus-independent
overall_score: 8
---

# Review: CCS Dependency Wiring Implementation Plan

## Per-Dimension Scores

| Dimension | Score | Notes |
|-----------|-------|-------|
| **Completeness** | 8/10 | Covers all six deliverables end-to-end. Cross-project scope (CCS repo vs SmokeSig repo) is clearly delineated. Missing: validate's `-f` flag registration bug (fixed). |
| **Accuracy** | 7/10 | `LoadDefault()` dead code analysis is correct. Three issues found: (1) validate.go `-f` flag never registered, (2) `configDir` derivation breaks when `configFile` stays empty, (3) P-02 `Execute()` needs `SilenceErrors` to avoid double-print. All fixed. |
| **Implementability** | 8/10 | Code snippets are close to copy-paste ready. Execution order and dependency graph are sound. P-01 validate.go design was unresolved ("refactor to pass loaded config or resolved path") -- now committed to `LoadDefaultPath()` approach. |
| **Review feedback incorporation** | 9/10 | Plan correctly addresses all brainstorm findings: LoadDefault dead code (Q3/SC-4), stale version references (v0.23.0 used throughout), exit code contract (SC-1 promoted from optional to P-02), subcommand passthrough (Q4 decision honored). |
| **Test coverage** | 7/10 | Test scenarios listed for each deliverable. Missing: validate `-f` flag regression test (added), no test for `configDir` correctness after LoadDefault resolution, no test for Cobra `SilenceErrors` behavior. |

**Overall: 8/10**

## Top 3 Strengths

1. **Excellent scope separation.** The plan clearly marks which deliverables belong to SmokeSig (P-01, P-02) vs CCS (P-03, P-04, P-05) vs cross-project (P-06), with execution order dependencies mapped. This prevents confusion during implementation.

2. **LoadDefault() dead code analysis is spot-on.** The plan correctly identifies that `LoadDefault()` at schema.go:496 is never called from any command path, with specific line references. The brainstorm's Q3 decision ("already implemented but NOT wired") is accurately reflected.

3. **Exit code contract is well-designed.** The 0/1/2/3 mapping (pass/fail/config/prereq) is clean, covers all known error categories, and the plan correctly identifies that consumers checking `!= 0` are unaffected by the change.

## Top 3 Issues Found and Fixed

### Issue 1: validate.go `-f` flag never registered (CRITICAL)

**Problem:** The plan's P-01 code for `validate.go` copies the existing pattern of `cmd.Flags().GetString("file")`, but the `-f`/`--file` flag is only registered on `runCmd` (in `run.go:147`), not on `validateCmd`. The `validateCmd.init()` function only calls `rootCmd.AddCommand(validateCmd)` with zero flag registrations. This means:
- `cmd.Flags().GetString("file")` always returns `""` (silently, no error)
- `smokesig validate -f custom.yaml` fails with "unknown flag: -f"
- The fallback to `.smokesig.yaml` at line 18 always activates regardless of user intent

**Fix applied:** Added `-f` flag registration on `validateCmd` in its `init()` function. Updated the validate.go code snippet to show the complete corrected file including flag registration.

### Issue 2: configDir derivation breaks after P-01 change (HIGH)

**Problem:** After P-01 changes `configFile`'s default to `""`, the `loadConfig()` function calls `schema.LoadDefault()` which returns a config but does NOT update the `configFile` package variable. At `run.go:211`, `configDir := filepath.Dir(configFile)` would compute `filepath.Dir("")` which returns `"."`. While `"."` happens to be correct for the cwd case, the `configFile` variable is also used at line 238 (`runWatch(configDir, configFile, ...)`), line 333, and in the watch mode reload path at line 239 (`loadConfig()` again). The variable being empty while the config was loaded from a real file is a latent bug.

**Fix applied:** Introduced `LoadDefaultPath() (string, error)` that resolves the filename without loading. `loadConfig()` now calls `LoadDefaultPath()` first and assigns the result back to `configFile` before calling `schema.Load(configFile)`. This preserves correct `configDir` derivation and watch-mode behavior.

### Issue 3: P-02 Execute() double-prints errors without SilenceErrors (MEDIUM)

**Problem:** Cobra's `rootCmd.Execute()` prints errors to stderr by default before returning them. The plan's `Execute()` function also prints via `fmt.Fprintln(os.Stderr, err)`. Without `rootCmd.SilenceErrors = true`, every error would appear twice on stderr.

**Fix applied:** Added `rootCmd.SilenceErrors = true` to the P-02 code snippet and a note explaining why it is required.

## Additional Observations

- **version.go default stale:** `cmd/version.go` has `var Version = "0.21.1"` but latest tag is `v0.23.0`. This only affects `go run .` during development (ldflags override at build time), but could confuse contributors. Added to risk assessment table.

- **P-03 minor:** Removed explicit `cmd.Dir = ""` from the exec sketch -- it is the zero value and the default behavior, so setting it explicitly is noise.

- **Test gap:** No test verifies that after `LoadDefault()` resolves to `.smoke.yaml`, the `configDir` is correctly derived. Consider adding an integration test that places `.smoke.yaml` in a subdirectory and verifies `configDir` points to that subdirectory.

- **P-02 validate.go interaction:** The plan should ensure `validate.go` errors are also wrapped in `ConfigError` so `Execute()` maps them to exit code 2. Added to P-02 steps.
