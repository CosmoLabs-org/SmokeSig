# Risk Assessment

## Tech Debt Hotspots

| File | Lines | Issue | Source |
|------|-------|-------|--------|
| runner.go | 857 | `runTestOnce` is 440 lines with 30+ identical blocks — no assertion registry | agent-1 |
| schema.go | ~500 | `Expect` struct has 42 flat fields — scales poorly | agent-1 |
| validate.go | 266 | Mirrors runTestOnce structure — grows in lockstep | agent-1 |
| export.go | ~100 | Missing 12 of 40 assertion types from schema export | agent-8 |

## Fragile Areas

### 1. Parallel Execution (HIGH)
`runner.go:233-258` spawns unbounded goroutines. `r.lifecycleEnv` (line 310) has unsynchronized read/write across goroutines. `backgroundProcesses` (lifecycle.go:24) is a global slice with no mutex. Combined: parallel mode has multiple race conditions. (see agent-1, agent-2, agent-8)

### 2. Trace Propagation Config Mutation (HIGH)
`CheckHTTPWithTrace`, `CheckWebSocketWithTrace`, `CheckGRPCHealthWithTrace` all mutate shared config pointers in-place by adding traceparent headers. In retry or stress scenarios, the original test config is permanently altered. (see agent-2, agent-8)

### 3. Release Pipeline (CRITICAL)
The entire release pipeline (GoReleaser, Dockerfile, CI workflows, Homebrew) references the old `cosmo-smoke` module path. No release workflow exists. Version fallback is 8 releases stale. Only 1 of 20 tags has a GitHub release. (see agent-5, agent-9)

## Security Surface

| Risk | Severity | File:Line | Source |
|------|----------|-----------|--------|
| Container runs as root | HIGH | Dockerfile:13 | agent-9 |
| API key timing-vulnerable (== comparison) | HIGH | handler.go:30 | agent-9 |
| HTTP server no timeouts (slowloris) | HIGH | serve.go:155 | agent-9 |
| Internal errors leaked to HTTP clients | HIGH | handler.go:55,72,123 | agent-9 |
| Redis password in plaintext YAML | MEDIUM | schema.go:176 | agent-2, agent-8 |
| OTel API keys in plaintext YAML | MEDIUM | schema.go:282 | agent-2 |
| Credential exec has no timeout | MEDIUM | assertion_credential.go:94 | agent-2 |
| JSON injection in error handler | MEDIUM | handler.go:61 | agent-4 |

## Chained Risks

**IPv6 + Protocol Parsing**: 8 assertions use `fmt.Sprintf("%s:%d")` which breaks IPv6. These same assertions use `conn.Read` instead of `io.ReadFull` for protocol headers. Combined: IPv6 users hit cryptic errors, and partial TCP reads on any platform can silently corrupt protocol parsing. (see agent-1, agent-2)

**Silent Reporters + No Monitoring**: Push reporter and OTel reporter both silently swallow all errors. There's no structured logging and no metrics endpoint on the serve command. Combined: smoke test results can be lost without any indication, and there's no way to monitor the monitoring tool. (see agent-3, agent-9)

**Stale Docs + Incomplete Schema Export**: SPEC.md documents 5 of 40 types. ExportSchema() returns 28 of 40 types. MCP server says "29 types". Combined: every programmatic or documentation-based consumer of SmokeSig's schema gets incomplete data. (see agent-8, agent-10)

## Single Points of Failure

1. **`runTestOnce`** (runner.go:326-769): Every assertion type flows through this 440-line method. A bug here affects all tests.
2. **`Expect` struct** (schema.go:106-148): Every new assertion type requires touching 4+ files.
3. **Issue index** (docs/issues/index.yaml): Only contains 12 of 52 issues — tooling that reads this gets wrong data.
