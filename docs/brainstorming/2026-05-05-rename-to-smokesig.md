---
date: 2026-05-05
status: brainstorm
deliverables:
  - id: BR-01
    title: "Go module + import path rename (cosmo-smoke → SmokeSig)"
  - id: BR-02
    title: "Binary + CLI command rename (smoke → smokesig)"
  - id: BR-03
    title: "Config file rename (.smoke.yaml → .smokesig.yaml) with backward compat"
  - id: BR-04
    title: "Documentation rewrite (README, CLAUDE.md, all docs)"
  - id: BR-05
    title: "GitHub repo rename + Go import redirect"
  - id: BR-06
    title: "Test suite update + verification"
---

# Rename cosmo-smoke → SmokeSig

## Why

"cosmo-smoke" ties the tool to the CosmoLabs brand in its name. "SmokeSig" is shorter, punchier, and has double meaning: smoke signal (communication) and signature (assertion). The binary `smokesig` is fast to type. The repo name is clean: `CosmoLabs-org/SmokeSig`.

## Assumptions

1. The GitHub repo will be renamed from `cosmo-smoke` to `SmokeSignal` (GitHub convention is case-insensitive, display name can be SmokeSig). Actually — let's use `SmokeSig` as the repo name to match.
2. Go module path becomes `github.com/CosmoLabs-org/SmokeSig`.
3. Binary becomes `smokesig` (lowercase, one word).
4. Config file becomes `.smokesig.yaml` with `.smoke.yaml` still accepted as fallback.
5. All CLI subcommands remain the same: `smokesig run`, `smokesig validate`, `smokesig serve`, etc.

## Scope

### Files affected

| Category | Count | What changes |
|----------|-------|-------------|
| Go files (import paths) | 83 | `github.com/CosmoLabs-org/cosmo-smoke` → `github.com/CosmoLabs-org/SmokeSig` |
| Go files (config refs) | 28 | `.smoke.yaml` → `.smokesig.yaml` (primary), `.smoke.yaml` as fallback |
| Go files (binary refs) | ~15 | `smoke` binary name in ldflags, help text, comments |
| Markdown docs | 185 | All mentions of cosmo-smoke, binary commands, config file names |
| Markdown (commands) | 89 | `smoke run` → `smokesig run`, etc. |
| go.mod | 1 | Module path |
| go.sum | 1 | No change (deps unchanged) |
| CLAUDE.md | 1 | Project identity, version, all refs |
| README.md | 1 | Full rewrite — new branding, fresh structure |
| Makefile / build scripts | ~3 | Binary name, ldflags |
| Version registry | 1 | `.version-registry.json` project name |
| GOrchestra agent files | ~5 | Agent prompt files referencing cosmo-smoke |

### What stays the same

- All assertion types and their YAML field names (no breaking config schema)
- Subcommand names: `run`, `validate`, `serve`, `init`, `schema`, `version`, `stress`, `migrate`
- Config structure: `tests:`, `prerequisites:`, `lifecycle:`, `otel:`, etc.
- Reporter format names: `terminal`, `json`, `junit`, `tap`, `prometheus`, `gha`
- Internal package structure: `internal/runner/`, `internal/schema/`, etc.

## Decisions

### Config file backward compat

`.smokesig.yaml` is the new primary. But `.smoke.yaml` should still be recognized as a fallback for existing projects. This means:

- `LoadDefault()` checks `.smokesig.yaml` first, then falls back to `.smoke.yaml`
- `smokesig init` creates `.smokesig.yaml`
- `--file` flag still works for either name
- Deprecation warning when loading `.smoke.yaml`: "Config file .smoke.yaml is deprecated, rename to .smokesig.yaml"

### Monorepo discovery

`monorepo.Discover()` looks for `.smoke.yaml` in subdirectories. Should look for both `.smokesig.yaml` and `.smoke.yaml`.

### Binary name transition

No backward compat shim needed — users update their CI/aliases. Clean break.

### GitHub repo rename

GitHub handles redirects automatically for renames. Old URL `github.com/CosmoLabs-org/cosmo-smoke` redirects to `CosmoLabs-org/SmokeSig` forever. But Go module proxy may cache the old path — we need to tag the new module path on the renamed repo.

**Critical**: After renaming the repo, the Go import path changes. Any downstream `go get github.com/CosmoLabs-org/cosmo-smoke` will break. We should add a `go.mod` in a branch on the old repo path that points to the new one, OR just accept the break since this is the only consumer.

**Verdict**: Since CosmoLabs is the primary consumer, just rename and update all references in one shot. No compat shim needed for the module path.

## Execution Model — Parallel Agent Dispatch

This is a perfect candidate for parallel GLM agents since every change is mechanical find-replace:

### Agent batches

**Batch 1: Go source — module path** (all 83 files)
- Find: `github.com/CosmoLabs-org/cosmo-smoke`
- Replace: `github.com/CosmoLabs-org/SmokeSig`
- Includes go.mod

**Batch 2: Go source — config file refs** (28 files)
- Add `.smokesig.yaml` as primary, `.smoke.yaml` as fallback in `LoadDefault()`
- Update monorepo discovery to check both
- Add deprecation warning for `.smoke.yaml`

**Batch 3: Go source — binary name** (~15 files)
- `smoke` → `smokesig` in ldflags, help text, root command
- Update `Use` fields in Cobra commands

**Batch 4: Documentation** (185+ md files)
- Full README rewrite with new branding
- CLAUDE.md update
- All other docs: find-replace cosmo-smoke → SmokeSig, smoke → smokesig

**Batch 5: Build & config** (Makefile, version registry, etc.)
- Update binary name in build targets
- Update project name in version registry
- Any CI/config files

**Opus review**: After all agents complete, review every diff, run `go build ./...` and `go test ./...`.

## Risks

1. **Go module cache**: `go get` with old path may fail after rename. Accept this — we're the only consumer.
2. **Test fixtures**: Some tests reference `.smoke.yaml` in string literals. All must be updated.
3. **GOrchestra agent files**: Prompt files in `GOrchestra/glm-agents/` reference cosmo-smoke. These need updates too.
4. **Incomplete replacement**: A grep after all changes must find zero remaining references.

## Verification

After all changes:
```bash
# Zero remaining references
grep -r "cosmo-smoke\|cosmo_smoke\|CosmoSmoke" --include="*.go" --include="*.md" --include="*.yaml" --include="*.json" . | grep -v "GOrchestra/glm-agents" | grep -v ".git/"

# Build passes
go build ./...

# Tests pass
go test ./... -count=1

# Binary works with new name
go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=dev" -o smokesig .
./smokesig version
./smokesig run --help
```
