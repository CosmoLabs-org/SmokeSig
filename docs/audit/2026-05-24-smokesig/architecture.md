# Architecture Map

## System Layers

```
┌─────────────────────────────────────────────────────────────┐
│ CLI Layer (cmd/)                                            │
│  root.go → run.go, validate.go, init_cmd.go, serve.go,     │
│            schema.go, stress.go, migrate.go, mcp.go, watch.go │
├─────────────────────────────────────────────────────────────┤
│ Config Layer (internal/schema/)                             │
│  schema.go (SmokeConfig, Expect 40-field struct)            │
│  validate.go (collect-all-errors validation)                │
│  remote.go (includes, extends, HTTP caching)                │
│  export.go (JSON schema export — 28 of 40 types)           │
├─────────────────────────────────────────────────────────────┤
│ Execution Layer (internal/runner/)                          │
│  runner.go (test orchestration, parallel/sequential)        │
│  assertion.go + 25 assertion_*.go (39 pure-function types)  │
│  lifecycle.go (hooks: before/after all/each, background)    │
│  prereq.go (prerequisite checks with install hints)         │
│  varstore.go (thread-safe variable store for chaining)      │
│  trace.go (W3C traceparent), trace_health.go (sliding win)  │
│  stress.go (flakiness detection with semaphore)             │
├─────────────────────────────────────────────────────────────┤
│ Output Layer (internal/reporter/)                           │
│  reporter.go (5-method Reporter interface)                  │
│  chain.go (format routing, multi-format support)            │
│  terminal.go, json.go, junit.go, tap.go, prometheus.go,    │
│  github.go, backstage.go, otel.go, push.go                 │
├─────────────────────────────────────────────────────────────┤
│ Feature Modules                                             │
│  internal/baseline/    — Performance regression detection   │
│  internal/monorepo/    — Sub-config discovery               │
│  internal/dashboard/   — SQLite storage + REST API          │
│  internal/detector/    — 31 project type detection          │
│  internal/mcp/         — MCP server (7 tools)               │
│  internal/migrate/     — Goss translation                   │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

1. **Config Load**: YAML → `schema.Load()` → Go template expansion → `yaml.Unmarshal` → `SmokeConfig`
2. **Validation**: `schema.Validate()` → collect all errors → return `ValidationError.Errors[]`
3. **Execution**: `Runner.Run()` → detect chains → sequential or parallel dispatch → `runTestOnce()`
4. **Assertions**: `runTestOnce()` → 30+ `if t.Expect.X != nil` blocks → pure assertion functions → `AssertionResult[]`
5. **Reporting**: `Reporter.TestResult()` per test → `Reporter.Summary()` at end → file/stdout output

## Dependency Graph

- `cmd/` depends on: `internal/schema`, `internal/runner`, `internal/reporter`, `internal/baseline`, `internal/monorepo`, `internal/dashboard`, `internal/detector`, `internal/mcp`, `internal/migrate`
- `internal/runner/` depends on: `internal/schema` (config types)
- `internal/reporter/` depends on: `internal/runner` (result types), `internal/schema` (test types)
- `internal/baseline/` is standalone
- `internal/monorepo/` depends on: `internal/schema`
- `internal/dashboard/` is standalone (SQLite)
- `internal/detector/` depends on: `internal/schema`
- `internal/mcp/` depends on: `internal/schema`, `internal/runner`, `internal/reporter`

No circular dependencies. Clean DAG with `internal/schema` as the shared foundation.

## Key Design Patterns

- **Pure assertion functions**: Each assertion type is a standalone function with no side effects (see agent-1, agent-2)
- **Reporter interface**: 5 methods, 9 implementations, composable via `Chain()` (see agent-3)
- **Config inheritance**: `includes:` (merge) and `extends:` (override) with circular guard (see agent-8)
- **VarStore chaining**: Tests extract values via regex/jsonpath, subsequent tests use `{{ .Vars.X }}` (see agent-4)
