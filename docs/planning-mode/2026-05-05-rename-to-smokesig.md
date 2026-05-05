---
brainstorm: docs/brainstorming/2026-05-05-rename-to-smokesig.md
created: "2026-05-05"
deliverables:
  - id: P-01
    title: "Go module path rename (go.mod + all 83 import paths)"
  - id: P-02
    title: "Config file rename (.smokesig.yaml primary + .smoke.yaml fallback)"
  - id: P-03
    title: "Binary and CLI rename (smoke → smokesig in all Go files)"
  - id: P-04
    title: "Documentation rewrite (README, CLAUDE.md, all .md files)"
  - id: P-05
    title: "Build config update (Makefile, version registry, GOrchestra)"
  - id: P-06
    title: "Full verification (build, tests, grep audit)"
issue: FEAT-000
requires_reading:
  - docs/brainstorming/2026-05-05-rename-to-smokesig.md
schema_version: 1
status: PENDING
tags:
  - rename
  - smokesig
title: "Rename cosmo-smoke → SmokeSig"
---

# Rename cosmo-smoke → SmokeSig

**Goal**: Rename the entire project from cosmo-smoke to SmokeSig. Parallel agent execution with Opus review.

## Pre-flight

```bash
go build ./...
go test ./... -count=1 -timeout 120s 2>&1 | tail -5
```

## Execution — Parallel GLM Agents

All agents run in parallel. Each has exact file lists and exact find-replace patterns.

### P-01: Module Path Rename

**Scope**: go.mod + all 83 Go files with import paths

**Exact changes**:
1. `go.mod`: `module github.com/CosmoLabs-org/cosmo-smoke` → `module github.com/CosmoLabs-org/SmokeSig`
2. Every `*.go` file: `github.com/CosmoLabs-org/cosmo-smoke` → `github.com/CosmoLabs-org/SmokeSig`

This is a single sed across all Go files:
```bash
find . -name "*.go" -not -path "./.git/*" -exec sed -i '' 's|github.com/CosmoLabs-org/cosmo-smoke|github.com/CosmoLabs-org/SmokeSig|g' {} +
sed -i '' 's|github.com/CosmoLabs-org/cosmo-smoke|github.com/CosmoLabs-org/SmokeSig|g' go.mod
```

### P-02: Config File Rename

**Scope**: ~28 Go files + deprecation logic

**Changes**:
1. `internal/schema/schema.go`: `LoadDefault()` checks `.smokesig.yaml` first, falls back to `.smoke.yaml` with deprecation warning
2. `cmd/root.go`: Update default config flag from `.smoke.yaml` to `.smokesig.yaml`
3. `cmd/run.go`: Update `--file` default to `.smokesig.yaml`
4. `cmd/init_cmd.go`: Generate `.smokesig.yaml` instead of `.smoke.yaml`
5. `internal/monorepo/discover.go`: Look for both `.smokesig.yaml` and `.smoke.yaml`
6. All test files: Update string literals referencing `.smoke.yaml` → `.smokesig.yaml`
7. `internal/mcp/` files: Update config file references

**Deprecation warning** (in schema.go LoadDefault):
```go
func LoadDefault() (*SmokeConfig, error) {
    if _, err := os.Stat(".smokesig.yaml"); err == nil {
        return Load(".smokesig.yaml")
    }
    if _, err := os.Stat(".smoke.yaml"); err == nil {
        fmt.Fprintln(os.Stderr, "⚠ Config file .smoke.yaml is deprecated, rename to .smokesig.yaml")
        return Load(".smoke.yaml")
    }
    return nil, fmt.Errorf("no config file found: .smokesig.yaml or .smoke.yaml")
}
```

### P-03: Binary + CLI Rename

**Scope**: All Go files referencing the binary name

**Changes**:
1. `cmd/root.go`: Banner and `Use` field: `smoke` → `smokesig`
2. `cmd/version.go`: Update ldflags variable reference path
3. `cmd/run.go`: Help text mentions `smoke run` → `smokesig run`
4. All `cmd/*.go` files: Update help text, examples, short/long descriptions
5. Test files: Update command invocation strings

**Key files to update**:
- `cmd/root.go` — banner says "SmokeSig" not "cosmo-smoke"
- `cmd/init_cmd.go` — generated config comments reference smokesig
- `cmd/serve.go` — help text
- `cmd/stress.go` — help text
- `cmd/migrate.go` — help text
- `cmd/schema.go` — help text

### P-04: Documentation Rewrite

**Scope**: README.md (full rewrite), CLAUDE.md, all other .md files

**README.md** — Complete rewrite with:
- New branding: "SmokeSig — Universal smoke test runner by CosmoLabs"
- Updated installation: `go install github.com/CosmoLabs-org/SmokeSig@latest`
- Updated commands: `smokesig run`, `smokesig validate`, etc.
- Updated config examples: `.smokesig.yaml`
- Fresh structure with better organization
- Keep all assertion type documentation

**CLAUDE.md** — Update:
- Title and version references
- Module path references
- Binary name references
- Command examples

**All other .md files** — Find-replace:
- `cosmo-smoke` → `SmokeSig` or `smokesig` (context-dependent)
- `cosmo_smoke` → `SmokeSig`
- `.smoke.yaml` → `.smokesig.yaml`
- `smoke run` → `smokesig run` (and all other commands)
- `smoke` (as binary) → `smokesig`

### P-05: Build Config

**Scope**: Makefile, .version-registry.json, GOrchestra files

**Changes**:
1. `Makefile`: Binary name `smoke` → `smokesig`, ldflags path update
2. `.version-registry.json`: Project name update
3. `GOrchestra/glm-agents/*/files/*.go`: Module path updates in agent test files
4. `GOrchestra/glm-agents/*.md` or prompt files: Update cosmo-smoke references
5. Any CI config files (if present)

### P-06: Verification (Opus — main session)

After all agents complete and merge:

```bash
# 1. Zero remaining references
grep -ri "cosmo-smoke\|cosmo_smoke\|CosmoSmoke" --include="*.go" --include="*.md" --include="*.yaml" --include="*.json" . | grep -v ".git/" | grep -v "deprecated" | grep -v "fallback"

# 2. Build passes
go build ./...

# 3. All tests pass
go test ./... -count=1

# 4. Binary works
go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=dev" -o smokesig .
./smokesig version
./smokesig run --help
./smokesig validate --help
```

## Agent Dispatch Commands

```bash
# P-01: Module path (biggest mechanical change)
ccs glm-agent exec --files "go.mod,*.go" "Replace all occurrences of 'github.com/CosmoLabs-org/cosmo-smoke' with 'github.com/CosmoLabs-org/SmokeSig' in go.mod and all Go files. Use sed: find . -name '*.go' -exec sed -i '' 's|github.com/CosmoLabs-org/cosmo-smoke|github.com/CosmoLabs-org/SmokeSig|g' {} + and also fix go.mod."

# P-03: Binary rename (help text)
ccs glm-agent exec --files "cmd/*.go" "Update all binary name references in cmd/*.go files. Change 'smoke' to 'smokesig' in: Use fields, help text, Short/Long descriptions, examples, banner text. The root command banner should say 'SmokeSig' not 'cosmo-smoke'. Do NOT change subcommand names (run, validate, serve, etc stay the same)."

# P-02: Config file (needs judgment)
# Keep this for main session or sonnet — backward compat logic is not pure find-replace

# P-04: Documentation
ccs glm-agent exec --files "README.md,CLAUDE.md" "Rewrite README.md completely for SmokeSig project. New title: 'SmokeSig — Universal Smoke Test Runner'. Update all binary references from 'smoke' to 'smokesig', all 'cosmo-smoke' to 'SmokeSig', all '.smoke.yaml' to '.smokesig.yaml'. Keep all assertion type tables and architecture docs. Update CLAUDE.md similarly."

# P-05: Build config
ccs glm-agent exec --files "Makefile,.version-registry.json" "Update Makefile: change binary name from 'smoke' to 'smokesig', update ldflags path from 'github.com/CosmoLabs-org/cosmo-smoke/cmd' to 'github.com/CosmoLabs-org/SmokeSig/cmd'. Update .version-registry.json project name."
```

## Post-Merge: GitHub Rename

After all code changes are committed and verified:

1. Rename GitHub repo: `cosmo-smoke` → `SmokeSignal` (or `SmokeSig`)
2. Update local remote: `git remote set-url origin git@github.com:CosmoLabs-org/SmokeSig.git`
3. Tag the renamed version

## Scope Summary

| Deliverable | Files | Agent | Model |
|-------------|-------|-------|-------|
| P-01 Module path | 83+ | GLM-turbo | Mechanical sed |
| P-02 Config file | 28 | Sonnet | Needs backward compat logic |
| P-03 Binary rename | ~15 | GLM-turbo | Find-replace in help text |
| P-04 Documentation | 185+ | GLM-turbo | Find-replace + README rewrite |
| P-05 Build config | ~5 | GLM-turbo | Small targeted changes |
| P-06 Verification | — | Opus (main) | Build, test, grep audit |
