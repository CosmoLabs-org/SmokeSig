# SmokeSig — How to Use

Runs lightweight smoke tests defined in `.smokesig.yaml` to verify a project is functional.

## Commands

| Command | What it does |
|---------|--------------|
| `smokesig run` | Run all smoke tests in `.smokesig.yaml` |
| `smokesig run --tag <tag>` | Run only tests matching the given tag |
| `smokesig run --exclude-tag <tag>` | Run all tests except those with the given tag |
| `smokesig run --format json` | Output results as JSON (for CI pipelines) |
| `smokesig run --format junit` | Output results as JUnit XML |
| `smokesig run --format tap` | Output results in TAP format |
| `smokesig run --format prometheus` | Output Prometheus metrics |
| `smokesig run --format gha` | Output GitHub Actions annotations + step summary |
| `smokesig run --format backstage` | Output Backstage entity annotation JSON |
| `smokesig run --fail-fast` | Stop immediately on the first failure |
| `smokesig run --timeout <dur>` | Override per-test timeout (e.g. `60s`, `2m`) |
| `smokesig run --dry-run` | List matching tests without executing them |
| `smokesig run --watch` | Re-run tests on file changes (500ms debounce) |
| `smokesig run --monorepo` | Auto-discover `.smokesig.yaml` in subdirectories |
| `smokesig run --baseline` | Compare results against performance baselines |
| `smokesig run -f <path>` | Use a config file at a non-default path |
| `smokesig validate` | Validate config without running tests |
| `smokesig init` | Auto-detect project type and generate `.smokesig.yaml` |
| `smokesig init --force` | Overwrite an existing `.smokesig.yaml` |
| `smokesig init --from-running <container>` | Generate config from a running Docker container |
| `smokesig schema` | Export assertion types as JSON schema |
| `smokesig serve` | Start dashboard server (SQLite + API + embedded UI) |
| `smokesig version` | Print the binary version |

## CCS Integration

SmokeSig is a CCS dependency. Install and rebuild via:

```bash
ccs rebuild smokesig     # Build from ~/PROJECTS/SmokeSig, deploy to ~/bin/smokesig
ccs smoke                # Thin passthrough to smokesig (runs smokesig run)
```

## Workflow

1. Run `smokesig init` to generate a `.smokesig.yaml` in your project root.
2. Edit the generated config — add real commands, adjust timeouts, tag tests.
3. Run `smokesig run` to execute all tests.
4. Use `--tag` to run focused subsets (e.g. `--tag build` in CI, `--tag runtime` locally).
5. Use `--format json` or `--format junit` to integrate results into CI pipelines.
6. Use `--watch` during development to re-run on file changes.
7. Add `.pre-commit-hooks.yaml` integration for automatic smoke testing on commit.

## Flags Reference

### `smokesig run`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --file` | string | `.smokesig.yaml` | Config file path |
| `--tag` | string (repeatable) | — | Include only tests with this tag |
| `--exclude-tag` | string (repeatable) | — | Exclude tests with this tag |
| `--format` | string | `terminal` | Output format: `terminal`, `json`, `junit`, `tap`, `prometheus`, `gha`, `backstage` |
| `--fail-fast` | bool | `false` | Stop on first failure |
| `--timeout` | duration | _(from config)_ | Per-test timeout override |
| `--dry-run` | bool | `false` | List tests without running |
| `--watch` | bool | `false` | Re-run tests on file changes |
| `--monorepo` | bool | `false` | Auto-discover configs in subdirectories |
| `--baseline` | bool | `false` | Compare against performance baselines |
| `--baseline-threshold` | int | `50` | Baseline deviation threshold (ms) |

### `smokesig init`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --force` | bool | `false` | Overwrite existing `.smokesig.yaml` |
| `--from-running` | string | — | Generate config from a running Docker container |

### `smokesig serve`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--port` | int | `8080` | Server port |
| `--dashboard` | bool | `false` | Enable embedded dashboard UI |
| `--api-key` | string | — | API key for authentication |
| `--db-path` | string | — | SQLite database path |

## Conventions

- Config file is `.smokesig.yaml` at the project root. Use `-f` to override.
- Commands in `run` and `prerequisites.check` execute via the system shell (`sh -c`).
- All commands run from the directory containing the config file, not the caller's cwd.
- `file_exists` paths are relative to the config file's directory.
- Timeouts use Go duration strings: `30s`, `2m`, `1m30s`.
- Tags are free-form strings. A test with no tags is always included unless `--tag` is set.
- `cleanup` runs after the test regardless of pass/fail, but its exit code is ignored.
- Prerequisites run before all tests. If any prerequisite fails, the run aborts.

## Test Modifiers

| Modifier | Field | Description |
|----------|-------|-------------|
| **Retry** | `retry: {count: N, backoff: <dur>}` | Retry flaky tests with exponential backoff. `backoff` doubles each retry. |
| **Allow failure** | `allow_failure: true` | Test passes even if assertions fail. Useful for flaky external dependencies. |
| **Skip conditions** | `skip_if: {env_unset, env_equals, file_missing}` | Conditionally skip a test without failing. |
| **Tags** | `tags: [build, runtime]` | Free-form labels for `--tag`/`--exclude-tag` filtering. |
| **Timeout** | `timeout: 60s` | Per-test timeout override. |
| **Cleanup** | `cleanup: "rm -f /tmp/test"` | Runs after the test regardless of pass/fail. |

## Conditional Execution

Skip tests based on environment conditions:

```yaml
tests:
  - name: "Docker build"
    run: "docker build ."
    expect:
      exit_code: 0
    skip_if:
      env_unset: "DOCKER_HOST"      # skip if DOCKER_HOST not set
    # skip_if:
    #   env_equals: { var: "CI", value: "true" }
    #   file_missing: "Dockerfile"
```

## Config Inheritance

Use `includes:` to share tests across multiple configs:

```yaml
# .smokesig.yaml
version: 1
project: my-api
includes:
  - .smokesig.common.yaml
tests:
  - name: "Project-specific test"
    run: "..."
```

## Multi-Environment Configs

Load environment-specific overrides:

```bash
# Base config + staging overrides
smokesig run -f .smokesig.yaml -f .smokesig.staging.yaml
```

Environment configs append tests and override settings.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All tests passed |
| `1` | One or more tests failed |
| `2` | Config error or invalid arguments |

## Related

- [README.md](./README.md) — Overview and quick start
- [SPEC.md](./SPEC.md) — Full schema reference
- [.smokesig.yaml](./.smokesig.yaml) — This project's own smoke tests
