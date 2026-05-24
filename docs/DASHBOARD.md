# Dashboard API

SmokeSig includes a portfolio dashboard that aggregates smoke test results from multiple projects into a single SQLite-backed view. Projects push their results via a REST API, and the embedded web UI provides a real-time overview of health across your fleet.

## Overview

The dashboard is designed for organizations running smoke tests across many projects. Each project pushes its results after a test run (via `--report-url`), and the dashboard stores them in SQLite. The embedded web UI polls the API every 30 seconds and shows a table of all projects with their latest health status.

The dashboard runs as part of `smokesig serve` when the `--dashboard` flag is enabled.

## Starting the Dashboard

```bash
# Basic dashboard on port 8080
smokesig serve --dashboard

# Custom port, API key, and database path
smokesig serve --dashboard \
  --port 9090 \
  --api-key "my-secret-key" \
  --db-path /var/lib/smokesig/dashboard.db

# Full serve with health endpoint + dashboard
smokesig serve --dashboard --api-key "secret" -f .smokesig.yaml
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port`, `-p` | `8080` | Port to listen on |
| `--dashboard` | `false` | Enable dashboard aggregation mode |
| `--api-key` | (none) | API key for `POST /api/results` (sent as `X-API-Key` header) |
| `--db-path` | `smoke-dashboard.db` | SQLite database file path |
| `--path` | `/healthz` | Health endpoint path (independent of dashboard) |
| `--file`, `-f` | `.smokesig.yaml` | Config file for the health endpoint |

When `--dashboard` is enabled, these routes are registered in addition to the health endpoint:

- `POST /api/results` -- Push test results
- `GET /api/projects` -- List all projects with latest status
- `GET /api/projects/{name}/history` -- Get run history for a project
- `GET /dashboard` -- Embedded web UI

## Configuration

### Pushing Results from smokesig run

Configure your smoke test runs to report results to the dashboard:

```yaml
# .smokesig.yaml
project: my-api
tests:
  - name: health check
    expect:
      http:
        url: "http://localhost:8080/health"
        status_code: 200
```

```bash
# Push results after running
smokesig run --report-url http://dashboard:8080/api/results --report-api-key "my-secret-key"
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Run smoke tests
  run: |
    smokesig run \
      --report-url ${{ secrets.SMOKE_DASHBOARD_URL }}/api/results \
      --report-api-key ${{ secrets.SMOKE_API_KEY }} \
      --format terminal,json
```

## REST API Reference

### POST /api/results

Push smoke test results for a project. The dashboard extracts summary fields from the JSON payload and stores the full payload for history.

**Headers:**

| Header | Required | Description |
|--------|----------|-------------|
| `X-API-Key` | When `--api-key` is set | API key for authentication |
| `Content-Type` | Yes | `application/json` |

**Request body:**

```json
{
  "project": "my-api",
  "total": 5,
  "passed": 4,
  "failed": 1,
  "skipped": 0,
  "allowed_failures": 0,
  "duration_ms": 2340
}
```

The `project` field is required. Additional fields in the JSON are stored in the `payload` column for later retrieval. The request body is stored verbatim, so you can include test-level details.

**Response (202 Accepted):**

```json
{
  "stored": true
}
```

**Error responses:**

| Status | Body | Cause |
|--------|------|-------|
| 400 | `{"error":"invalid json"}` | Malformed JSON body |
| 400 | `{"error":"project field required"}` | Missing `project` in body |
| 403 | `{"error":"unauthorized"}` | Missing or incorrect `X-API-Key` |
| 405 | `{"error":"method not allowed"}` | Non-POST request |
| 500 | `{"error":"internal server error"}` | Database write failure |

### GET /api/projects

List all projects with their latest test status. Returns a summary with counts of healthy and failing projects.

**Response (200 OK):**

```json
{
  "projects": [
    {
      "name": "api-gateway",
      "latest_status": "healthy",
      "total_tests": 8,
      "passed": 8,
      "failed": 0,
      "last_run": "2026-05-24T10:30:00Z",
      "last_run_age_seconds": 1800
    },
    {
      "name": "auth-service",
      "latest_status": "failing",
      "total_tests": 5,
      "passed": 3,
      "failed": 2,
      "last_run": "2026-05-24T10:25:00Z",
      "last_run_age_seconds": 2100
    }
  ],
  "summary": {
    "total_projects": 2,
    "healthy": 1,
    "failing": 1
  }
}
```

The `latest_status` is derived from the most recent run: `"healthy"` when `failed == 0`, `"failing"` otherwise.

### GET /api/projects/{name}/history

Get the run history for a specific project, ordered newest first.

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Maximum number of runs to return |

**Response (200 OK):**

```json
{
  "project": "my-api",
  "runs": [
    {
      "ID": 42,
      "Project": "my-api",
      "Timestamp": "2026-05-24T10:30:00Z",
      "Total": 5,
      "Passed": 5,
      "Failed": 0,
      "Skipped": 0,
      "AllowedFailures": 0,
      "DurationMs": 1234,
      "Payload": "{\"project\":\"my-api\",\"total\":5,...}"
    }
  ]
}
```

The `Payload` field contains the full JSON body that was submitted via `POST /api/results`, preserved verbatim.

## SQLite Storage Schema

The dashboard uses a single `runs` table with automatic migration on startup:

```sql
CREATE TABLE IF NOT EXISTS runs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    project          TEXT NOT NULL,
    timestamp        DATETIME DEFAULT CURRENT_TIMESTAMP,
    total            INTEGER,
    passed           INTEGER,
    failed           INTEGER,
    skipped          INTEGER,
    allowed_failures INTEGER DEFAULT 0,
    duration_ms      INTEGER,
    payload          TEXT
);

CREATE INDEX IF NOT EXISTS idx_runs_project ON runs(project);
CREATE INDEX IF NOT EXISTS idx_runs_timestamp ON runs(timestamp);
```

### Automatic Pruning

Old runs are automatically pruned per project after each insert. The default retention is 1000 runs per project. Pruning deletes the oldest runs beyond the limit, keeping the most recent ones.

### Storage Backend

The database uses `modernc.org/sqlite` (a pure-Go SQLite implementation), so no CGo or system SQLite library is required. Use `":memory:"` as the path for an in-memory database (useful for testing).

## Embedded Web UI

The dashboard includes a minimal embedded web UI served at `/dashboard`. It is a single HTML file with inline CSS and JavaScript (no external dependencies).

### Features

- Auto-refreshes every 30 seconds
- Shows project name, test counts, pass/fail, status badge, and last run age
- Color-coded status badges: green for healthy, red for failing
- Human-readable age display (seconds, minutes, hours, days)
- Monospace terminal-aesthetic theme (Tokyo Night color palette)
- Shows "No projects reporting yet" when the database is empty

### Accessing the UI

```
http://localhost:8080/dashboard
```

The UI fetches data from `GET /api/projects` and renders it client-side. No server-side templating is involved.

## Architecture

```
internal/dashboard/
  dashboard.go    # Config struct
  store.go        # SQLite storage layer (NewStore, InsertRun, GetProjects, GetProjectHistory, pruning)
  handler.go      # HTTP API handlers (RegisterRoutes, results/projects/history endpoints)
  static.go       # Embedded UI via go:embed
  templates/
    index.html    # Single-page dashboard UI
```

The dashboard is integrated into `cmd/serve.go`. When `--dashboard` is passed, the serve command opens the SQLite store, registers API routes on the HTTP mux, and mounts the embedded UI handler at `/dashboard`.

### Authentication

API key authentication is optional. When `--api-key` is set:

- `POST /api/results` requires the `X-API-Key` header to match (constant-time comparison)
- `GET` endpoints are unauthenticated (read-only)

### Graceful Shutdown

The serve command handles `SIGINT` and `SIGTERM` for graceful shutdown with a 5-second deadline. The SQLite database is closed via `defer store.Close()`.
