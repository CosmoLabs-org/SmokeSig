---
brainstorm_ref: docs/brainstorming/2026-05-29-wasm-plugin-system.md
branch: master
covers_brainstorm_deliverables:
    - BR-01
    - BR-02
    - BR-03
    - BR-04
    - BR-05
    - BR-06
    - BR-07
    - BR-08
    - BR-09
    - BR-10
    - BR-11
    - BR-12
covers_plan_deliverables:
    - P-01
    - P-02
    - P-03
    - P-04
    - P-05
    - P-06
    - P-07
    - P-08
    - P-09
    - P-10
    - P-11
    - P-12
created: "2026-05-29"
id: P-2026-05-29-wasm-plugin-system
plan_ref: docs/planning-mode/2026-05-29-wasm-plugin-system.md
priority: medium
requires_reading:
    - docs/brainstorming/2026-05-29-wasm-plugin-system.md
    - docs/planning-mode/2026-05-29-wasm-plugin-system.md
schema_version: 1
status: PENDING
title: 'FEAT-048: Wasm Plugin System'
---
# FEAT-048: Wasm Plugin System

## BEFORE Starting — Required Reading

**You MUST read these files in full before writing any code. They are the gospel truth of what must be implemented.**

Read in order:

1. **`docs/brainstorming/2026-05-29-wasm-plugin-system.md`** — design rationale and decision history.
2. **`docs/planning-mode/2026-05-29-wasm-plugin-system.md`** — implementation plan with deliverables and file scope.

Loading is enforced by `ccs prompts load-context` — the command will error if any file is missing.

## Context

Add a Wasm plugin system using `tetratelabs/wazero` (pure Go, zero deps). Users write custom assertions compiled to `.wasm`, register them in `plugins:` config, and reference them as assertion types. Dual ABI: WASI stdin/stdout (primary, no SDK needed) and shared-memory (v2, performance). Capability-based sandboxing (deny-by-default). ~1,390 LOC total.

## Execution Strategy

Two parallel tracks merge at runner integration:

- **Track A** (plugin package): G-01 → G-02 → G-03 → G-04 → G-05 → G-10 → G-11
- **Track B** (schema/config): G-07 → G-08 → G-09
- **Merge**: G-06 (runner integration, depends on both tracks)
- **Final**: G-12 (docs)

## Goals

### [ ] G-01 Plugin types and schema integration (PluginEntry, Expect.Plugin, Settings additions)
Covers P-01.

### [ ] G-02 PluginManager core: compile, cache, instantiate, evaluate, close
Covers P-02.

### [ ] G-03 ABI layer: memory protocol (v1) and WASI stdin/stdout fallback (v2)
Covers P-03.

### [ ] G-04 Host functions: HTTP, env, time with capability gating
Covers P-04.

### [ ] G-05 Sandbox: capability enforcement, memory limits, timeout
Covers P-05.

### [ ] G-06 Runner integration: plugin evaluation in assertion block with sorted iteration
Covers P-06.

### [ ] G-07 Validation integration: plugin references, file existence, export probing
Covers P-07.

### [ ] G-08 Config merge semantics: includes last-wins for plugins, monorepo path resolution
Covers P-08.

### [ ] G-09 Schema export: plugin metadata in smokesig schema output
Covers P-09.

### [ ] G-10 Test fixtures: .wasm binaries in testdata/ with build script
Covers P-10.

### [ ] G-11 Debug mode: SMOKESIG_PLUGIN_DEBUG=1 logging
Covers P-11.

### [ ] G-12 Documentation: plugin authoring guide, ABI reference
Covers P-12.

## Related

- Brainstorm: `docs/brainstorming/2026-05-29-wasm-plugin-system.md`
- Plan: `docs/planning-mode/2026-05-29-wasm-plugin-system.md`
