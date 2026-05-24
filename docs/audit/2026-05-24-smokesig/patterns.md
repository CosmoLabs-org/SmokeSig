# Codebase Patterns

## Error Handling

**Pattern**: Consistent `fmt.Errorf("context: %w", err)` wrapping throughout. Validation uses collect-all-errors pattern via `ValidationError.Errors` slice. (see agent-1-code-quality.md)

**Inconsistencies**:
- Push reporter silently swallows all HTTP errors (push.go:83-98) — no log, no warning (see agent-3)
- OTel reporter fires spans in goroutines with `//nolint:errcheck` (otel.go:125) (see agent-3)
- Redis AUTH response error silently discarded (assertion_db.go:37-38) (see agent-2)
- `runSmoke` calls `os.Exit(1)` inside Cobra's RunE, bypassing defer chains (run.go:239) (see agent-2)

## State Management

**Thread-safe**: VarStore uses `sync.RWMutex` with secret masking — well-designed pattern (see agent-1, agent-8)

**Unsafe**: `Runner.lifecycleEnv` map has unsynchronized read/write in parallel mode (runner.go:310). `backgroundProcesses` global slice has no mutex (lifecycle.go:24). (see agent-1, agent-2, agent-8)

**Config mutation**: `CheckHTTPWithTrace`, `CheckWebSocketWithTrace`, `CheckGRPCHealthWithTrace` mutate shared schema config pointers in-place by modifying headers maps. (see agent-2, agent-8)

## Naming Conventions

**Consistent**: Go naming conventions followed throughout — exported types have doc comments, clean PascalCase/camelCase.

**Inconsistency**: `AssertionResult` (missing 's' — should be `AssertionResult` but the pattern is consistent with itself). Validation error prefix switches between `test[%d]` and `tests[%d]` (validate.go:36 vs 44). (see agent-3)

## File Organization

**Pattern**: One assertion type per file (`assertion_db.go`, `assertion_dns.go`, etc.) with matching test files. Reporter implementations one per file.

**Hotspots**: `runner.go` at 857 lines is the largest source file — the 440-line `runTestOnce` method should be decomposed via an assertion registry pattern. `schema.go` at ~500 lines with a 42-field flat `Expect` struct scales poorly. (see agent-1)

## Secret Handling

**Inconsistency**: MongoDB, Kafka, LDAP, MQTT use `PasswordEnv` pattern (env var name). Redis uses `Password` (plaintext in YAML). OTel uses `APIKey` and `DDAppKey` (plaintext). Credential assertion properly redacts values with `***redacted***`. (see agent-2, agent-8)

## Testing Patterns

**Coverage**: 1,045 tests, strong across packages (baseline 94%, reporter 92%, schema 85%, runner 75%, cmd 25%). Wire protocol tests in `assertion_wire_test.go` are thorough. No fuzz tests for protocol parsers despite handling untrusted network input. (see agent-1)
