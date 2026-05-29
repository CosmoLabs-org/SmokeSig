---
feature: FEAT-048
documents_reviewed:
  - docs/brainstorming/2026-05-29-wasm-plugin-system.md
  - docs/planning-mode/2026-05-29-wasm-plugin-system.md
date: "2026-05-29"
---

# FEAT-048: Wasm Plugin System — Review Summary

## Brainstorm Review: 8/10

| Dimension | Score |
|-----------|-------|
| Completeness | 8/10 |
| Feasibility | 7/10 |
| Consistency | 9/10 |
| Risk identification | 7/10 |
| Actionability | 8/10 |

### Strengths
1. Correct problem identification — `Plugin map[string]interface{}` avoids Expect struct bloat
2. Dual ABI with clear tradeoff articulation — shared memory vs WASI performance analysis grounded
3. Reporter compatibility analysis — zero reporter changes needed, verified against `toReporterResult`

### Issues Found & Fixed
1. **Map iteration order breaks output determinism** → Added Deterministic Iteration subsection with sorted-keys pattern
2. **`context.env` has security implications** → Changed to always `{}` empty, env access via `host_env_get` with capability
3. **SDK scope contradiction** → Resolved: WASI stdin/stdout is PRIMARY ABI for v1 (no SDK needed), shared-memory deferred to v2

## Plan Review: 7/10

| Dimension | Score |
|-----------|-------|
| Completeness | 9/10 |
| Accuracy | 7/10 |
| Implementability | 8/10 |
| Review feedback incorporation | 6/10 |
| Test coverage | 8/10 |

### Strengths
1. Exhaustive task decomposition with verification gates — every task ends with `go test` and commit message
2. Security model well-designed — deny-by-default capabilities, double-gate on exec, always-empty context.env
3. Zero-overhead for non-plugin users — PluginManager nil when no plugins, wazero never initialized

### Issues Found & Fixed
1. **ABI version numbering contradicts review feedback** → Swapped: WASI=v1 (primary), Memory=v2 (performance). DetectABI checks `_start` first
2. **`input.Env` compile error** → Fixed: set `Env: map[string]string{}` on PluginContext construction, removed from Evaluate
3. **Host module registration unresolved** → Chose context-based dispatch: register once in NewPluginManager, sandbox via `ctx.Value(sandboxKey{})`
4. Minor: added `import "sort"` to sandbox.go, removed unused `hostModuleName` param, `strings.Contains` over `bytes.Contains`
