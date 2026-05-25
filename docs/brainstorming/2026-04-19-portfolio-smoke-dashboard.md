# Portfolio Smoke Dashboard (ROAD-039)

**Date**: 2026-04-19
**Status**: Brainstorm
**Source**: IDEA-025, promoted to ROAD-039

## Goal

Central dashboard that aggregates `smoke run` results across CosmoLabs' ~95 projects for real-time portfolio health visibility. Single binary, zero external deps, runs on internal infra.

---

## 1. Backend Architecture: Extend `smoke serve`

**Recommendation: Extend `smoke serve` in-place. No separate binary.**

Why not a separate binary:
- The `smoke serve` command already has HTTP scaffolding (Cobra registration, `http.Server`, graceful shutdown, config loading). It is 158 lines and does one thing: run tests on `/healthz` and return JSON. Extending it is straightforward.
- A separate `smoke-dashboard` binary means a second build target, a second release pipeline, and two binaries to deploy. For internal tooling across 95 projects, one binary is simpler.
- The dashboard is fundamentally "serve HTTP with smoke data" -- it is the same domain as `smoke serve`.

How `smoke serve` grows:

```
smoke serve --port 8080 --dashboard
```

The `--dashboard` flag activates the aggregation mode. Without it, `smoke serve` behaves exactly as today (health-only). With it, the server gains these routes:

| Route | Purpose |
|-------|---------|
| `GET /healthz` | Existing behavior, unchanged |
| `POST /api/results` | Push endpoint for project reporters |
| `GET /api/projects` | List all projects with latest status |
| `GET /api/projects/{name}/history` | Run history for a project |
| `GET /dashboard` | HTML dashboard (embedded) |

The health-only mode keeps working for projects that just need a kube probe. The dashboard mode is opt-in.

---

## 2. Data Collection Model: Push

**Recommendation: Push model via `--report-url` flag on `smoke run`.**

Why push over pull for 95 projects:

- **Pull requires service discovery.** The dashboard would need to know the address of every project's `smoke serve` endpoint. That means a config file listing 95 URLs, or DNS-based discovery, or a service registry. All operational overhead.
- **Projects already run `smoke run` in CI.** Every CosmoLabs project runs smoke tests in CI pipelines. Adding `--report-url https://dashboard.internal:8080/api/results` to those CI jobs is a one-line change. The data flows exactly where it needs to go.
- **No agent required.** Pull means each project must run `smoke serve` as a long-lived process. Most projects don't -- they run `smoke run` and exit. Push matches the existing execution model.
- **CI is the natural source of truth.** Smoke results come from CI. CI pushes them to the dashboard. No additional infrastructure per project.

Implementation:

```bash
# In CI (each project's pipeline)
smoke run --format json --report-url https://smoke-dashboard.internal:8080/api/results
```

The `--report-url` flag causes `smoke run` to POST its JSON output to the URL after the run completes. This is a fire-and-forget HTTP POST in the existing `Summary()` path, identical to how `OTelReporter` already exports telemetry. The payload is exactly the same JSON that `--format json` produces today (`jsonOutput` struct).

Add a `PushReporter` alongside `OTelReporter` in `internal/reporter/`:

```go
type PushReporter struct {
    endpoint  string
    headers   map[string]string
    client    *http.Client
    // accumulates results, POSTs on Summary()
}
```

The `--report-url` flag wires it in via `withPushReport()` in `cmd/run.go`, same pattern as `withOTelExport()`.

---

## 3. Storage: SQLite

**Recommendation: SQLite via `modernc.org/sqlite` (pure Go, no CGO).**

Why not in-memory ring buffer:
- A ring buffer loses all data on restart. For a dashboard tracking portfolio health, historical trends matter. "Last 24 hours of CI" is the minimum useful window.
- Ring buffers also require manual compaction logic. SQLite handles this natively.

Why SQLite over Postgres/MySQL:
- Single binary deployment. No database server to run, connect, or configure.
- The write pattern is append-only (one POST per CI run, ~95 projects, maybe 10-20 runs/day each = ~2000 writes/day). SQLite handles this trivially.
- Read pattern is simple range queries (last N runs for project X). SQLite is excellent at this.
- `modernc.org/sqlite` is pure Go -- no CGO, cross-compile works, matches cosmo-smoke's "minimal deps" philosophy.

Schema:

```sql
CREATE TABLE runs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    project     TEXT NOT NULL,
    timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP,
    total       INTEGER,
    passed      INTEGER,
    failed      INTEGER,
    skipped     INTEGER,
    allowed_failures INTEGER DEFAULT 0,
    duration_ms INTEGER,
    payload     TEXT  -- full JSON blob for drill-down
);

CREATE INDEX idx_runs_project ON runs(project);
CREATE INDEX idx_runs_timestamp ON runs(timestamp);

-- Optional: auto-vacuum old runs
-- Keep last 1000 runs per project, prune on insert
```

Configuration:

```yaml
# In .smoke.yaml on the dashboard server
dashboard:
  enabled: true
  db_path: "./smoke-dashboard.db"
  max_runs_per_project: 1000
  retention_days: 30
```

If `db_path` is empty or `:memory:`, falls back to in-memory (useful for testing).

---

## 4. Frontend: Embedded Static HTML + Vanilla JS

**Recommendation: Single HTML file embedded via `embed.FS`, vanilla JS with fetch API.**

Why no build step:
- cosmo-smoke has zero npm/node dependency. Adding a JS build pipeline would violate the project's "minimal deps" design principle.
- The dashboard is a status board, not a web app. It needs: a table of projects with pass/fail status, click-through to test details, and a timestamp. Vanilla JS handles this in ~300 lines.
- `embed.FS` (Go 1.16+) compiles static files into the binary. Single binary deployment preserved.

Structure:

```
internal/dashboard/
├── handler.go          # HTTP handlers for API routes + dashboard
├── store.go            # SQLite storage layer
├── templates/
│   └── index.html      # Embedded HTML+CSS+JS (~400 lines)
```

The HTML file contains inline CSS and JS. No external assets. Uses `fetch('/api/projects')` to populate the table, auto-refreshes every 30 seconds.

Dashboard UI layout:

```
┌─────────────────────────────────────────────────────────┐
│  CosmoLabs Smoke Dashboard          Last updated: 12:34 │
│  87/95 healthy  |  5 degraded  |  3 failing             │
├─────────────────────────────────────────────────────────┤
│  Project        | Tests | Pass | Fail | Last Run | Age  │
│  cosmo-api      |   12  |  12  |   0  | healthy  | 2m  │
│  cosmo-web      |    8  |   6  |   2  | failing  | 15m │
│  cosmo-auth     |    5  |   5  |   0  | healthy  | 5m  │
│  ...                                                    │
│  [click row for test details]                           │
└─────────────────────────────────────────────────────────┘
```

CSS: dark theme (terminal aesthetic matching cosmo-smoke's Lipgloss output), responsive, no framework.

---

## 5. API Design

### `POST /api/results`

Push endpoint. Receives the same JSON that `smoke run --format json` outputs.

Request:
```json
{
  "project": "cosmo-api",
  "total": 12,
  "passed": 12,
  "failed": 0,
  "skipped": 0,
  "allowed_failures": 0,
  "duration_ms": 3400,
  "tests": [
    {"name": "health check", "passed": true, "duration_ms": 120, "assertions": [...]}
  ]
}
```

Response: `202 Accepted` with `{"stored": true}`. Fire-and-forget from the client perspective.

Validation: `project` field required, must be non-empty. If the project name is not in the allow-list (see Auth section), reject with `403`.

### `GET /api/projects`

```json
{
  "projects": [
    {
      "name": "cosmo-api",
      "latest_status": "healthy",
      "total_tests": 12,
      "passed": 12,
      "failed": 0,
      "last_run": "2026-04-19T12:34:56Z",
      "last_run_age_seconds": 120
    }
  ],
  "summary": {
    "total_projects": 95,
    "healthy": 87,
    "degraded": 5,
    "failing": 3
  }
}
```

Query params: `?status=healthy|failing|degraded` to filter.

### `GET /api/projects/{name}/history`

```json
{
  "project": "cosmo-api",
  "runs": [
    {
      "timestamp": "2026-04-19T12:34:56Z",
      "total": 12,
      "passed": 12,
      "failed": 0,
      "duration_ms": 3400,
      "tests": [
        {"name": "health check", "passed": true, "duration_ms": 120}
      ]
    }
  ]
}
```

Query params: `?limit=50` (default 50), `?since=2026-04-19T00:00:00Z`.

### `GET /dashboard`

Serves the embedded `index.html`. The JS fetches `/api/projects` and renders.

---

## 6. Auth: Portfolio-wide API Key

**Recommendation: Single shared API key, header-based. No per-project tokens.**

Why: Internal network. 95 projects pushing to one endpoint. Per-project token management is operational overhead for zero security gain on an internal network.

Implementation:

```bash
# Dashboard server side
smoke serve --dashboard --api-key "cosmo-internal-2026"

# CI side
smoke run --report-url https://dashboard:8080/api/results --report-api-key "cosmo-internal-2026"
```

The API key is validated on `POST /api/results` via `X-API-Key` header. `GET` endpoints (dashboard, API) are unauthenticated on the internal network.

If `--api-key` is not set, all endpoints are open (default for dev/internal). This keeps the zero-config experience for local development.

Key rotation: change the key in CI secrets and restart the dashboard. For 95 projects using a shared CI secret, this is a one-place change.

---

## 7. Implementation Plan

### File Structure

```
internal/dashboard/
├── dashboard.go        # Config, types, registration
├── handler.go          # HTTP handlers (POST /api/results, GET /api/projects, etc.)
├── store.go            # SQLite storage (insert, query, prune)
├── store_test.go       # Storage layer tests
├── templates/
│   └── index.html      # Embedded dashboard UI

internal/reporter/
├── push.go             # PushReporter (new)
├── push_test.go        # PushReporter tests

cmd/
├── serve.go            # Extended with --dashboard flag
├── run.go              # Extended with --report-url flag
```

### Implementation Order

**Phase 1: Push infrastructure (reporter side)**

1. Create `internal/reporter/push.go` -- `PushReporter` that POSTs JSON to a URL on `Summary()`
2. Add `--report-url` and `--report-api-key` flags to `cmd/run.go`
3. Wire `PushReporter` in `runSmoke()` alongside existing reporters
4. Test: `smoke run --report-url http://localhost:8080/api/results` with a netcat listener

**Phase 2: Storage layer**

5. Add `modernc.org/sqlite` dependency
6. Create `internal/dashboard/store.go` -- SQLite schema, `InsertRun()`, `GetLatestByProject()`, `GetProjectHistory()`, `PruneOld()`
7. Create `internal/dashboard/store_test.go` -- full coverage with in-memory SQLite

**Phase 3: API handlers**

8. Create `internal/dashboard/handler.go` -- `POST /api/results`, `GET /api/projects`, `GET /api/projects/{name}/history`
9. Create `internal/dashboard/dashboard.go` -- config types, `RegisterRoutes(mux, store)`
10. Extend `cmd/serve.go` -- `--dashboard` flag, `--api-key`, `--db-path`, route registration

**Phase 4: Frontend**

11. Create `internal/dashboard/templates/index.html` -- dashboard UI
12. Add `GET /dashboard` handler serving embedded HTML
13. Auto-refresh via `setInterval(fetch, 30000)`

**Phase 5: Polish**

14. Graceful degradation: if SQLite fails, fall back to in-memory
15. Auto-prune on insert (keep last N per project)
16. README update for `smoke serve --dashboard`

### Dependency Addition

Only one new dependency: `modernc.org/sqline` (pure Go SQLite). Everything else is stdlib (`net/http`, `embed`, `database/sql`, `encoding/json`).

### Estimated Size

| File | Lines |
|------|-------|
| `internal/reporter/push.go` | ~80 |
| `internal/dashboard/store.go` | ~150 |
| `internal/dashboard/handler.go` | ~120 |
| `internal/dashboard/dashboard.go` | ~40 |
| `internal/dashboard/templates/index.html` | ~350 |
| Changes to `cmd/run.go` | ~20 |
| Changes to `cmd/serve.go` | ~30 |
| Tests | ~250 |
| **Total** | **~1040** |

---

## Open Questions

1. **Project name source**: Should the `project` field in push payloads come from `.smoke.yaml`'s `project:` field (already exists in schema), or from a CLI flag? Recommendation: use the YAML field, with `--report-project` override for edge cases.

2. **Stale project detection**: After how long should a project be marked "stale" (no report received)? Default proposal: 24 hours. Configurable via `dashboard.stale_after` in the server's `.smoke.yaml`.

3. **CI pipeline template**: Should cosmo-smoke ship a GitHub Actions reusable workflow that includes the `--report-url` flag? That would make adoption across 95 projects a one-line include. Out of scope for v1 but worth noting.

4. **WebSocket live updates**: The dashboard could use WebSocket to push updates instead of polling. Adds complexity to both sides. Proposal: v1 uses polling (30s interval). v2 can add WebSocket if the polling UX is insufficient.
