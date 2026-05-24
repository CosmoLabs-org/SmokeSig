# Lifecycle Hooks

SmokeSig provides a lifecycle hook system for running setup and teardown commands around your smoke tests. This covers starting services before tests run, cleaning up after tests finish, and passing environment variables between hooks and tests.

## Overview

The lifecycle system has three layers:

1. **Prerequisites** -- Checks that must pass before any tests run (e.g. "Is Go installed?")
2. **Lifecycle hooks** -- Commands that run at four lifecycle points: before all tests, after all tests, before each test, after each test
3. **Per-test cleanup** -- A cleanup command on individual tests that runs after the test completes

These layers execute in order: prerequisites first, then `before_all` hooks, then for each test: `before_each` hooks, the test itself, `after_each` hooks. After all tests complete, `after_all` hooks run. Per-test cleanup runs immediately after each individual test, before `after_each`.

## Configuration

### Prerequisites

Prerequisites are simple check commands that must succeed (exit 0) before any tests run. If any prerequisite fails, the entire test suite aborts.

```yaml
prerequisites:
  - name: Go installed
    check: command -v go
    hint: "Install Go from https://go.dev/dl/"

  - name: Docker running
    check: docker info
    hint: "Start Docker Desktop or run: sudo systemctl start docker"

  - name: Redis available
    check: redis-cli ping
    hint: "Start Redis: docker run -d -p 6379:6379 redis:alpine"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Human-readable name displayed in output |
| `check` | string | Yes | Shell command to run (via `sh -c`). Exit 0 = pass. |
| `hint` | string | No | Help text shown when the check fails |

Prerequisites run sequentially. The first line of stdout from each check is captured and displayed (e.g. showing the Go version). All prerequisites are evaluated even if one fails -- all failures are reported together.

### Lifecycle Hooks

Lifecycle hooks run at four points in the test execution:

```yaml
lifecycle:
  before_all:
    - command: docker compose up -d
      timeout: 60s
      background: true
      wait_for_port: 5432
      startup_timeout: 30s

    - command: ./scripts/seed-database.sh
      timeout: 30s
      env_pass: true

  after_all:
    - command: docker compose down
      timeout: 30s
      always_run: true

  before_each:
    - command: ./scripts/reset-test-state.sh
      timeout: 10s

  after_each:
    - command: ./scripts/collect-logs.sh
      timeout: 10s
      always_run: true
```

#### Hook Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `command` | string | (required) | Shell command to run via `sh -c` |
| `timeout` | duration | `30s` | Maximum time for the hook to complete |
| `always_run` | boolean | `false` | Run even if a previous hook failed |
| `env_pass` | boolean | `false` | Capture `KEY=VALUE` lines from stdout and pass as environment variables |
| `background` | boolean | `false` | Start the command without waiting for it to exit |
| `wait_for_port` | int | 0 | When `background: true`, poll this TCP port until it accepts connections |
| `startup_timeout` | duration | same as `timeout` | How long to wait for the port to become ready |

#### Hook Execution Points

| Point | When it runs | Failure behavior |
|-------|-------------|------------------|
| `before_all` | Once, before any tests | Aborts the entire suite |
| `after_all` | Once, after all tests complete | Runs regardless of test outcomes (via `defer`) |
| `before_each` | Before every individual test | Fails that specific test |
| `after_each` | After every individual test | Errors logged to stderr, does not fail the test |

### Per-Test Cleanup

Individual tests can define a `cleanup` command that runs after the test completes, regardless of pass or fail:

```yaml
tests:
  - name: temp file creation
    run: ./create-temp-files.sh
    cleanup: rm -rf /tmp/smoke-test-*
    expect:
      exit_code: 0
      file_exists: /tmp/smoke-test-output.txt
```

Cleanup commands run with a fixed 10-second timeout and execute in the config file's directory. Errors from cleanup commands are silently ignored -- they are best-effort.

## Background Process Management

Background hooks start a process and immediately continue to the next hook without waiting for it to exit. This is essential for starting services that your tests depend on.

### Starting a Background Service

```yaml
lifecycle:
  before_all:
    - command: node server.js
      background: true
      wait_for_port: 3000
      startup_timeout: 15s

  after_all:
    - command: "true"  # Background processes are auto-cleaned
      always_run: true
```

### How It Works

1. The command starts via `sh -c` with `cmd.Start()` (non-blocking)
2. The process PID is tracked in an internal process registry
3. If `wait_for_port` is set, SmokeSig polls `127.0.0.1:<port>` with TCP connections
4. Polling uses exponential backoff: starts at 50ms, doubles each attempt, caps at 2 seconds
5. If the port is not ready within `startup_timeout`, the process is killed (SIGKILL) and the hook fails

### Automatic Cleanup

All tracked background processes are automatically cleaned up when the test suite finishes via `CleanupBackgroundProcesses()`:

1. Send `SIGTERM` to all tracked processes
2. Wait 100ms for graceful shutdown
3. Send `SIGKILL` to any processes still alive
4. Clear the process registry

This cleanup runs automatically at the end of the test suite. You do not need to manually stop background processes.

### Port Wait Algorithm

The port readiness check uses TCP dial with exponential backoff:

```
Initial backoff: 50ms
Backoff multiplier: 2x
Maximum backoff: 2s
Dial timeout per attempt: 200ms
```

The check connects to `127.0.0.1:<port>`. Once a TCP connection succeeds, the port is considered ready and the hook continues.

## Environment Variable Passing

Hooks with `env_pass: true` capture `KEY=VALUE` lines from their stdout and merge them into the environment available to subsequent hooks and tests.

### How It Works

```yaml
lifecycle:
  before_all:
    - command: |
        echo "DATABASE_URL=postgres://localhost:5432/testdb"
        echo "API_TOKEN=generated-token-123"
      env_pass: true

    - command: echo "Using database $DATABASE_URL"
      # This hook can access DATABASE_URL from the previous hook
```

1. The hook runs and its stdout is captured
2. Each line is parsed: if it contains `=` with a non-empty key, it is captured
3. Leading/trailing whitespace is trimmed from both key and value
4. Captured variables are merged into the lifecycle environment
5. Subsequent hooks and all tests can access these variables

### Environment Flow

```
before_all hooks (env_pass captured)
  |
  v
before_each hooks (can read + add env)
  |
  v
Test execution (env available via Vars/templates)
  |
  v
after_each hooks (can read env)
  |
  v
after_all hooks (can read env)
```

Environment variables from lifecycle hooks are propagated into the runner's `VarStore`, making them available for Go template resolution in test `run:` commands:

```yaml
lifecycle:
  before_all:
    - command: echo "PORT=8080"
      env_pass: true

tests:
  - name: server responds
    expect:
      http:
        url: "http://localhost:{{ .Vars.PORT }}/health"
        status_code: 200
```

## Execution Details

### Command Execution

All commands (prerequisites, lifecycle hooks, cleanup) execute via `sh -c <command>`. This means:

- Shell features like pipes, redirects, and environment variable expansion work
- Commands run in the config file's directory (not the current working directory)
- Each command gets its own shell process

### Error Handling

Hooks run sequentially within each lifecycle point. Error handling follows these rules:

| Rule | Behavior |
|------|----------|
| First error captured | Only the first non-nil error is returned |
| `always_run: false` (default) | Hook is skipped if a previous hook in the same phase failed |
| `always_run: true` | Hook runs even if a previous hook failed |
| `after_all` | Always runs (deferred), errors logged to stderr |
| `after_each` | Always runs, errors logged to stderr |
| Background process fails to start | Error captured, subsequent hooks may be skipped |
| Port wait timeout | Background process is killed, error captured |

### Thread Safety

The lifecycle environment is protected by a read-write mutex (`sync.RWMutex`). This is relevant during parallel test execution: `before_each` and `after_each` hooks acquire locks to safely read and update the shared environment. In parallel mode, each test gets a snapshot of the environment at the time its `before_each` hooks run.

## Full Example

```yaml
project: my-api

prerequisites:
  - name: Go installed
    check: go version
    hint: "Install Go: https://go.dev/dl/"
  - name: Docker running
    check: docker info
    hint: "Start Docker Desktop"

lifecycle:
  before_all:
    # Start infrastructure
    - command: docker compose up -d postgres redis
      background: true
      wait_for_port: 5432
      startup_timeout: 30s
      timeout: 60s

    # Run migrations
    - command: go run ./cmd/migrate up
      timeout: 30s

    # Start the API server
    - command: go run ./cmd/server
      background: true
      wait_for_port: 8080
      startup_timeout: 20s

    # Generate test token
    - command: ./scripts/create-test-token.sh
      env_pass: true  # Captures TOKEN=xxx from stdout

  after_all:
    - command: docker compose down -v
      timeout: 30s
      always_run: true

  before_each:
    - command: ./scripts/reset-db.sh
      timeout: 10s

  after_each:
    - command: ./scripts/save-test-logs.sh
      timeout: 5s
      always_run: true

tests:
  - name: API health check
    expect:
      http:
        url: "http://localhost:8080/health"
        status_code: 200

  - name: Create user
    run: |
      curl -s -X POST http://localhost:8080/users \
        -H "Authorization: Bearer {{ .Vars.TOKEN }}" \
        -H "Content-Type: application/json" \
        -d '{"name": "test"}'
    cleanup: "curl -s -X DELETE http://localhost:8080/users/test"
    expect:
      exit_code: 0
      stdout_contains: '"id"'

  - name: Redis connected
    expect:
      redis_ping:
        host: localhost
        port: 6379

  - name: Postgres connected
    expect:
      postgres_ping:
        host: localhost
        port: 5432
```

## Architecture

```
internal/runner/
  lifecycle.go   # RunLifecycleHooks, waitForPort, BackgroundProcess, CleanupBackgroundProcesses
  prereq.go      # CheckPrerequisites, runPrereq
  runner.go      # Runner.Run (orchestrates lifecycle), runTestWithHooks (before_each/after_each)

internal/schema/
  schema.go      # LifecycleConfig, LifecycleHook, Prerequisite struct definitions
  validate.go    # Lifecycle hook validation (command required, background hook rules)
```
