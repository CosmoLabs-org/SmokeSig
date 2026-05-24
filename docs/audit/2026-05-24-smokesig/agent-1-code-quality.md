# Agent 1: Code Quality Audit

## SmokeSig v0.21.1 — Code Quality Audit

### 1. Architecture (Score: 82)

Clean Go package boundaries with clear separation of concerns:

- **`cmd/`** — CLI layer (Cobra commands), delegates to `internal/`
- **`internal/runner/`** — core assertion engine, test execution, lifecycle hooks
- **`internal/schema/`** — config parsing, validation, YAML handling
- **`internal/reporter/`** — pluggable output formats via `Reporter` interface
- **`internal/baseline/`**, **`internal/monorepo/`**, **`internal/dashboard/`**, **`internal/mcp/`** — well-isolated feature modules

**Strengths:**
- The `Reporter` interface (`reporter.go:6-12`) is clean with 5 methods, and `Chain()` (`chain.go:30`) elegantly handles multi-format output.
- Config parsing cleanly separates loading, templating, and validation.
- VarStore (`varstore.go`) is thread-safe with `sync.RWMutex` and secret-masking.

**Weaknesses:**
- **`runTestOnce` is a 440-line method** (`runner.go:326-769`) — 30+ identical `if t.Expect.X != nil` blocks. No assertion registry pattern.
- **`Expect` struct is a 42-field flat struct** (`schema.go:106-148`) — scales poorly.
- **Global mutable state** in `lifecycle.go:24`: `var backgroundProcesses []BackgroundProcess`.

### 2. Code Patterns (Score: 78)

Idiomatic Go overall with consistent error wrapping (`%w`), proper `context.WithTimeout`, clean Cobra registration.

**Issues:**
- **BUG: `append` no-op in `assertion_docker.go:65`** — `--compose-file` silently ignored
- **Race condition in parallel mode** (`runner.go:289-323`) — `r.lifecycleEnv` written without synchronization
- **IPv6 format bug** (9 instances) — `fmt.Sprintf("%s:%d")` doesn't work with IPv6
- Inconsistent indentation in validate.go:175-182
- Inconsistent prefix naming (`test` vs `tests`) in validate.go:44-46

### 3. Tech Debt (Score: 79)

- 11 TODOs (mostly in Goss migration — reasonable)
- **Complexity hotspots:** runner.go (857 lines), assertion_test.go (1,399 lines), schema.go (~500 lines)
- Raw `cmd.Start()` at lifecycle.go:73 — acceptable for managed background processes
- No FIXMEs — good signal

### 4. Test Coverage (Score: 81)

| Package | Coverage |
|---------|----------|
| baseline | 93.9% |
| monorepo | 92.9% |
| reporter | 92.3% |
| mcp | 89.0% |
| migrate/goss | 88.2% |
| schema | 85.2% |
| dashboard | 83.2% |
| runner | 75.4% |
| detector | 70.4% |
| cmd | 24.7% |

- 1,045 tests all passing
- `assertion_test.go` has 92 test functions covering edge cases
- Wire protocol tests in `assertion_wire_test.go` (15.8K lines)
- **Gaps:** cmd/ at 24.7%, no fuzz tests for protocol parsers

### 5. Maintainability (Score: 83)

**Strengths:**
- Every exported type/function has doc comment
- Clean, descriptive naming
- CLAUDE.md is comprehensive and accurate
- Pure assertion functions make testing straightforward

**Weaknesses:**
- **No assertion registry pattern** — adding a new type requires 4+ file changes
- **Large file sizes** approaching navigation difficulty
- **Redis password inconsistency** — plaintext vs `password_env` pattern

---

### Summary of Critical and High Findings

| # | Severity | File:Line | Issue |
|---|----------|-----------|-------|
| 1 | High | assertion_docker.go:65 | `append` no-op — `--compose-file` silently ignored |
| 2 | High | runner.go:310 + runner.go:233 | Data race on `r.lifecycleEnv` in parallel mode |
| 3 | Medium | assertion_db.go:22,68,101,138 + 5 more | IPv6 address format broken (9 instances) |
| 4 | Medium | assertion_ldap.go:113-129 | Response parsing trusts fixed byte offsets |
| 5 | Medium | schema.go:177 | RedisCheck.Password stores plaintext |
