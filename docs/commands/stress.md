# smoke stress

Run a single test repeatedly to detect flakiness. Reports pass rate and failure patterns.

## Usage

```bash
smoke stress <test-name> [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--runs` | `50` | Total number of executions |
| `--workers` | `1` | Concurrency (1 = sequential) |
| `--fail-fast` | `false` | Stop on first failure |
| `-f, --file` | `.smoke.yaml` | Config file path |
| `--format` | `terminal` | Output format (`terminal`, `json`) |

## Examples

```bash
smoke stress check-api-health              # Run 50 times, sequential
smoke stress check-api-health --runs 100   # 100 executions
smoke stress check-api-health --workers 5  # 5 concurrent workers
smoke stress check-api-health --runs 200 --workers 10  # Heavy load
smoke stress check-api-health --fail-fast  # Stop on first failure
```

## Reliability Thresholds

| Pass Rate | Status |
|-----------|--------|
| 100% | Stable |
| 95-99% | Flaky |
| <95% | Unreliable |

## Output

Shows progress during execution, then a summary with:
- Total runs, concurrency, duration
- Reliability percentage and status
- Deduplicated failure messages with occurrence counts
