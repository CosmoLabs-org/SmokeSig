# SmokeSig Schema Specification

Reference for the `.smokesig.yaml` config format. All fields are documented with types, defaults, and behavior.

---

## Format

`.smoke.yaml` is a YAML file placed at the project root. It is parsed once at startup and validated before any tests run. All errors are reported together — validation does not stop at the first error.

---

## Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | integer | yes | Schema version. Must be `1`. |
| `project` | string | yes | Project name. Used in output headers. |
| `description` | string | no | Human-readable description of what these tests cover. |
| `settings` | Settings | no | Global defaults for test behavior. |
| `prerequisites` | []Prerequisite | no | Commands that must pass before any tests run. |
| `tests` | []Test | yes | List of smoke tests. At least one is required. |

**Minimal valid config:**
```yaml
version: 1
project: my-app
tests:
  - name: "Starts"
    run: "./my-app --help"
    expect:
      exit_code: 0
```

---

## Settings Block

Controls global test execution behavior. All fields are optional.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `30s` | Default timeout applied to each test. Overridable per test. |
| `fail_fast` | bool | `false` | If `true`, stop after the first test failure. |
| `parallel` | bool | `false` | If `true`, run tests concurrently. |

```yaml
settings:
  timeout: 30s
  fail_fast: true
  parallel: false
```

**Duration format:** Go duration strings — `30s`, `2m`, `1m30s`, `500ms`.

---

## Prerequisite Schema

Prerequisites are checked before any tests run. If any check fails, the entire run aborts and no tests execute.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Human-readable name shown in output. |
| `check` | string | yes | Shell command to run. Must exit `0` to pass. |
| `hint` | string | no | Message shown if the check fails. Use for install instructions. |

```yaml
prerequisites:
  - name: "Docker running"
    check: "docker info"
    hint: "Start Docker Desktop before running these tests"

  - name: "Go 1.21+"
    check: "go version"
    hint: "Install Go from https://go.dev"
```

---

## Test Schema

Each entry in `tests` defines one smoke test.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Human-readable test name. Shown in output. |
| `run` | string | yes | Shell command to execute. |
| `expect` | Expect | yes | One or more assertions to evaluate. |
| `tags` | []string | no | Labels for filtering with `--tag` / `--exclude-tag`. |
| `timeout` | duration | no | Per-test timeout. Overrides `settings.timeout`. |
| `cleanup` | string | no | Shell command run after the test, regardless of pass/fail. |

```yaml
tests:
  - name: "Builds binary"
    run: "go build -o ./bin/app ./..."
    expect:
      exit_code: 0
    tags: [build]
    timeout: 120s
    cleanup: "rm -f ./bin/app"
```

---

## Expect Block (Assertions)

All fields are optional but at least one must be present. Multiple assertions in a single `expect` block must all pass for the test to pass.

| Field | Type | Description |
|-------|------|-------------|
| `exit_code` | integer | Exact process exit code. `0` = success, any other value = specific failure code. |
| `stdout_contains` | string | The literal string must appear anywhere in stdout. |
| `stdout_matches` | string | A Go regular expression matched against the full stdout output. |
| `stderr_contains` | string | The literal string must appear anywhere in stderr. |
| `file_exists` | string | Path (relative to the config file's directory) that must exist as a file or directory. |

### exit_code

```yaml
expect:
  exit_code: 0        # must exit cleanly
```

```yaml
expect:
  exit_code: 1        # must exit with error
```

Exit code is a pointer field — omitting it means the exit code is not checked.

### stdout_contains

```yaml
expect:
  stdout_contains: "Server started"
```

Case-sensitive substring match. The output only needs to contain the string; it does not need to equal it.

### stdout_matches

```yaml
expect:
  stdout_matches: "^v[0-9]+\\.[0-9]+\\.[0-9]+"
```

Full Go regex syntax. Matched against the complete stdout string. Use `(?i)` for case-insensitive matching.

### stderr_contains

```yaml
expect:
  stderr_contains: "flag provided but not defined"
```

Same semantics as `stdout_contains`, applied to stderr.

### file_exists

```yaml
expect:
  file_exists: "dist/index.html"
```

Checked relative to the directory containing `.smoke.yaml`. Works for files and directories. Does not check file contents.

### Combining assertions

```yaml
expect:
  exit_code: 0
  stdout_contains: "OK"
  stdout_matches: "^OK [0-9]+ tests"
```

All specified assertions must pass. If any fail, the test is marked failed and all failures are reported.

---

## Execution Semantics

### Shell

All commands (`run`, `prerequisites.check`, `cleanup`) are executed via `sh -c "<command>"`. Shell features (pipes, redirects, `&&`, `||`) are supported.

```yaml
run: "go build ./... && echo 'build ok'"
run: "./my-app --bad-flag 2>&1"
```

### Working directory

All commands execute from the directory containing the config file, regardless of where `smoke` was invoked. This makes configs portable and self-contained.

### Cleanup

The `cleanup` command runs after the test completes — whether the test passed or failed. Its exit code is ignored. Use it to remove build artifacts or temporary files created by `run`.

```yaml
tests:
  - name: "Produces binary"
    run: "go build -o /tmp/testbin ."
    expect:
      exit_code: 0
    cleanup: "rm -f /tmp/testbin"
```

### Timeout

Tests that exceed their timeout are killed and marked as failed. The effective timeout per test is resolved in this order:
1. `test.timeout` (if set)
2. `settings.timeout` (if set)
3. Binary default (`30s`)

The `--timeout` CLI flag overrides all of the above for every test in the run.

### Tag filtering

Tags are free-form strings. A test with no tags matches all `--tag` filters. When `--tag` is specified, only tests that have at least one matching tag are run. When `--exclude-tag` is specified, tests with any matching tag are skipped. Both flags can be used together.

### Prerequisites and fail-fast

Prerequisites always run sequentially before tests. If any prerequisite exits non-zero, the run halts immediately. `fail_fast` only controls test execution — prerequisites always behave as fail-fast.

---

## Output Formats

Controlled via `--format`. All formats write to stdout.

| Format | Flag | Description |
|--------|------|-------------|
| `terminal` | `--format terminal` | Human-readable colored output (default) |
| `json` | `--format json` | Machine-readable JSON — includes prerequisites, tests, assertions, and durations |
| `junit` | `--format junit` | JUnit XML — compatible with GitHub Actions, Jenkins, GitLab CI, and any tool that consumes the standard JUnit XML schema |

### JUnit XML Schema

The JUnit output follows the standard schema:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="smoke" tests="6" failures="1" time="2.300">
  <testsuite name="project-name" tests="6" failures="1" skipped="0" time="2.300">
    <testcase name="Compiles" time="0.800"/>
    <testcase name="CLI works" time="0.500">
      <failure message="stdout_contains: expected &quot;Usage&quot; not found in &quot;error: unknown command&quot;">
        stdout_contains:
          Expected: Usage
          Actual:   error: unknown command
      </failure>
    </testcase>
  </testsuite>
</testsuites>
```

Skipped tests include a `<skipped/>` element. Prerequisite results are not included in JUnit output (only test cases).

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All tests passed (or `--dry-run` completed) |
| `1` | One or more tests failed |
| `2` | Config error, validation error, or invalid CLI arguments |

---

## Full Example

```yaml
version: 1
project: my-api
description: "Smoke tests — verifies the API compiles, starts, and handles basic requests"

settings:
  timeout: 30s
  fail_fast: false
  parallel: false

prerequisites:
  - name: "Go toolchain available"
    check: "go version"
    hint: "Install Go 1.21+ from https://go.dev"

  - name: "No port conflict on 8080"
    check: "! lsof -i :8080 -t"
    hint: "Stop any process using port 8080"

tests:
  - name: "Compiles without errors"
    run: "go build -o /tmp/my-api ./..."
    expect:
      exit_code: 0
    tags: [build]
    timeout: 60s
    cleanup: "rm -f /tmp/my-api"

  - name: "Passes vet"
    run: "go vet ./..."
    expect:
      exit_code: 0
    tags: [build]

  - name: "Help flag works"
    run: "/tmp/my-api --help"
    expect:
      exit_code: 0
      stdout_contains: "Usage"
    tags: [runtime]

  - name: "Rejects unknown flags"
    run: "/tmp/my-api --definitely-not-a-flag"
    expect:
      exit_code: 1
      stderr_contains: "flag provided but not defined"
    tags: [runtime]

  - name: "Config template exists"
    run: "echo ok"
    expect:
      file_exists: "config.example.yaml"
    tags: [structure]
```
