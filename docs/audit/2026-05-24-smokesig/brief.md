# SmokeSig Project Brief

**Version**: 0.21.1 | **Language**: Go | **Framework**: Cobra CLI | **Audit Score**: 66.1/100

## What It Is
Universal smoke test runner. Standalone Go binary that reads `.smokesig.yaml` configs and runs 39+ assertion types covering HTTP, databases, message queues, Docker, K8s, OTel traces, mobile deep links, and more. Designed for CosmoLabs' ~95-project portfolio.

## Tech Stack
- Go 1.25 with Cobra CLI, Lipgloss terminal styling, yaml.v3, gjson
- No Viper, no Bubbletea, minimal external dependencies
- Pure wire-protocol implementations for database/broker checks (no SDK deps)
- SQLite for portfolio dashboard storage

## Architecture
```
main.go → cmd/root.go → cmd/{run,validate,init,serve,schema,stress,migrate,mcp,watch}.go
                         ↓
internal/schema/     — YAML parsing, validation, config inheritance
internal/runner/     — Assertion engine (39 types), prereq runner, lifecycle hooks
internal/reporter/   — 7 output formats via pluggable Reporter interface
internal/baseline/   — Performance regression detection
internal/monorepo/   — Sub-config discovery
internal/dashboard/  — SQLite storage, REST API, embedded UI
internal/detector/   — 31 project type detection + template generation
internal/mcp/        — MCP server (7 tools for Claude Desktop)
internal/migrate/    — Goss migration translator
```

## Key Strengths
1. 39 native assertion types with zero external dependencies
2. MCP server for AI integration (unique in space)
3. 7 output formats (terminal, JSON, JUnit, TAP, Prometheus, GHA, Backstage)
4. 31 project type auto-detection
5. 1,045 tests passing, strong coverage

## Known Critical Issues
1. **Incomplete rename**: 60+ stale `cosmo-smoke` references in release pipeline, docs, examples
2. **Distribution broken**: No release workflow, 18/20 versions have zero binary artifacts
3. **Docker Compose bug**: `--compose-file` flag silently ignored (append no-op)
4. **Race conditions**: `r.lifecycleEnv` unsynchronized in parallel mode
5. **SMTP bug**: double-handshake causes protocol desync
6. **Security**: HTTP server has no timeouts, container runs as root, API key timing-vulnerable

## Entry Points
- CLI: `main.go` → `cmd/root.go`
- Core engine: `internal/runner/runner.go` → `runTestOnce()` (440-line method)
- Config: `internal/schema/schema.go` → `Expect` struct (40 fields)
- Dashboard: `internal/dashboard/handler.go` (3 REST endpoints)
- MCP: `internal/mcp/server.go` (7 tools)
