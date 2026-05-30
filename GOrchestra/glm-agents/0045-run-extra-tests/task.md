# Task

Add tests for cmd/run.go: withConfigNotifications and handleBaseline.

withConfigNotifications (13.3%): This function wraps a reporter with
webhook notifications from config. Read the function to understand what
it does, then test:
- Config with no notifications → returns original reporter
- Config with one notification (format: json, on: always) → returns wrapped reporter
- Config with invalid URL → returns original reporter (handles gracefully)

handleBaseline (0%): This function compares test results against stored
baselines. Read the function signature and test:
- No baseline flag set → returns nil (no-op)
- Baseline flag set with no prior baseline → creates new baseline file
- Baseline flag set with existing baseline → compares and reports

Use t.TempDir() for baseline storage. Set the package-level vars
(baselineEnabled, baselineThreshold) before calling.

Add tests to cmd/run_extra_test.go (append to it).

Verify:
  go test ./cmd/ -v -run "TestWithConfigNotifications|TestHandleBaseline"
  go test -cover ./cmd/

Commit via: ccs commit-batch --message "test(cmd): add tests for withConfigNotifications and handleBaseline"

