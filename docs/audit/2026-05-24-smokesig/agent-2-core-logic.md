# Agent 2: Core Logic / Domain Audit

## SmokeSig v0.21.1 — Core Logic Audit Report

### Files Read (21 source files)

- `internal/runner/runner.go` (core engine)
- `internal/runner/assertion.go` (base assertions)
- `internal/runner/assertion_db.go` (Redis, Memcached, Postgres, MySQL)
- `internal/runner/assertion_network.go` (port, process, HTTP, SSL, JSON)
- `internal/runner/assertion_ws.go` (WebSocket)
- `internal/runner/assertion_otel.go` (OTel trace verification)
- `internal/runner/assertion_smtp.go` (SMTP)
- `internal/runner/assertion_ldap.go` (LDAP)
- `internal/runner/assertion_mongo.go` (MongoDB)
- `internal/runner/assertion_docker.go` (Docker)
- `internal/runner/assertion_kafka.go` (Kafka)
- `internal/runner/assertion_mqtt.go` (MQTT)
- `internal/runner/assertion_ntp.go` (NTP)
- `internal/runner/assertion_dns.go` (DNS)
- `internal/runner/assertion_credential.go` (credentials)
- `internal/runner/assertion_reachable.go` (URL/S3/service reachable)
- `internal/runner/assertion_file.go` (file exists/size)
- `internal/runner/assertion_ping.go` (ICMP ping)
- `internal/runner/assertion_k8s.go` (Kubernetes)
- `internal/runner/assertion_deeplink.go` (deep links)
- `internal/runner/assertion_graphql.go` (GraphQL)
- `internal/runner/assertion_version.go` (version check)
- `internal/runner/assertion_grpc.go` + `assertion_grpc_stub.go` (gRPC)
- `internal/runner/lifecycle.go` (lifecycle hooks, background processes)
- `internal/runner/prereq.go` (prerequisites)
- `internal/runner/trace.go` (W3C traceparent)
- `internal/runner/trace_health.go` (trace health sliding window)
- `internal/runner/varstore.go` (variable store, chained tests)
- `internal/runner/stress.go` (stress testing)
- `internal/schema/schema.go` (config structs, YAML parsing)
- `internal/schema/validate.go` (config validation)
- `internal/schema/remote.go` (remote config, includes, extends)
- `cmd/run.go` (CLI entry point)

---

### 1. CORRECTNESS (Score: 68)

**BUG: Docker Compose args silently drops `--format json`** (assertion_docker.go:65, CRITICAL)

```go
if check.ComposeFile != "" {
    args = append([]string{"compose", "-f", check.ComposeFile, "ps", "--format", "json"})
}
```

This `append` has no values to append — it creates a new slice literal and discards it. The variable `args` remains `["compose", "ps", "--format", "json"]` — the custom compose file is silently ignored. This is confirmed by `go vet`: "append with no values". The fix should be:

```go
args = []string{"compose", "-f", check.ComposeFile, "ps", "--format", "json"}
```

**BUG: IPv6 address formatting breaks 8 assertions** (assertion_db.go:22,68,101,138; assertion_ldap.go:23; assertion_mongo.go:22; assertion_network.go:28; assertion_smtp.go:26)

All database pings, port checks, LDAP, MongoDB, and SMTP use `fmt.Sprintf("%s:%d", host, port)` which produces `::1:5432` for IPv6, but `net.Dial` requires `[::1]:5432`. Users with IPv6-only hosts will get cryptic connection errors.

**BUG: `CheckHTTPWithTrace` and `CheckWebSocketWithTrace` mutate the caller's config** (assertion_network.go:310-314, assertion_ws.go:308-313)

```go
func CheckHTTPWithTrace(check *schema.HTTPCheck, span *SpanContext) []AssertionResult {
    if check.Headers == nil {
        check.Headers = make(map[string]string)
    }
    check.Headers["traceparent"] = span.Traceparent()
    return CheckHTTP(check)
}
```

These functions mutate the `check` pointer's `Headers` map in-place. Since the `HTTPCheck` is passed by pointer from the schema config, this permanently adds `traceparent` to the test's headers. On retry, the header accumulates or overwrites with a new traceparent — unintended mutation of shared config.

**BUG: `CheckGRPCHealthWithTrace` also mutates shared config** (assertion_grpc.go:71-74) — same pattern.

**BUG: Race condition in parallel mode — `lifecycleEnv` and `VarStore` shared across goroutines** (runner.go:233-258, runner.go:289-311)

In `runParallel`, multiple goroutines call `runTestWithHooks` concurrently. The `r.lifecycleEnv = env` assignment on line 310 is a data race — multiple goroutines write to the same field without synchronization.

**BUG: `backgroundProcesses` is a package-level slice with no synchronization** (lifecycle.go:24) — races if lifecycle hooks run from parallel tests.

**BUG: Lifecycle hook context leak — `defer cancel()` inside loop** (lifecycle.go:59-60) — defers accumulate until function returns, not per iteration.

**BUG: WebSocket `wsReadFrame` does not unmask server-to-client frames when masked flag is set** (assertion_ws.go:90-96) — mask key is read and discarded but payload is never XOR-unmasked.

**BUG: SMTP double-handshake** (assertion_smtp.go:38-68) — manually reads 220 greeting, then `smtp.NewClient` reads it again, causing protocol desync.

### 2. EDGE CASES (Score: 72)

- Redis AUTH response error silently discarded (assertion_db.go:37-38)
- `conn.Read` for MongoDB may return partial response (assertion_mongo.go:66-68) — should use `io.ReadFull`
- MySQL handshake `conn.Read` may return < 5 bytes (assertion_db.go:150-151) — same issue
- LDAP BER encoding assumes BindDN fits in single-byte length (assertion_ldap.go:74) — truncates > 127 bytes
- `EnvExists` considers empty string as "not set" (assertion_file.go:13-14) — should use `os.LookupEnv`
- NTP epoch truncation after Feb 7, 2036 (assertion_ntp.go:93-98)

### 3. PERFORMANCE (Score: 78)

- No connection pooling for OTel trace polling (assertion_otel.go:149-167)
- **Parallel execution spawns unbounded goroutines** (runner.go:237-244) — no semaphore unlike stress.go
- `processTemplate` creates env map on every Load call (schema.go:433-437)

### 4. ERROR HANDLING (Score: 75)

- `processTemplate` hides template errors with silent `<no value>` substitution (schema.go:425-448)
- `version_check` uses `regexp.MustCompile` which panics on invalid patterns (assertion_version.go:33)
- Lifecycle hook errors don't identify the command (lifecycle.go:75, 128)
- `runSmoke` calls `os.Exit(1)` inside Cobra's `RunE` (cmd/run.go:239, 302)

### 5. SECURITY (Score: 76)

- Redis password in YAML config in cleartext (schema.go:177) — unlike LDAP/MongoDB/MQTT which use `password_env`
- OTel API keys in YAML config in cleartext (schema.go:282-283)
- `exec` credential source has no timeout (assertion_credential.go:94)
- Template injection surface via `text/template` (schema.go:425-448)
- HTTP configs don't validate URL schemes

### Summary of Strengths

1. Comprehensive 39 assertion types covering databases, message queues, cloud services, mobile deep links, DNS, NTP
2. Good separation of concerns — schema/validate/runner/reporter cleanly separated
3. VarStore is thread-safe with credential masking
4. Config validation is thorough
5. Retry with exponential backoff well implemented
6. Chain detection correctly forces sequential execution
7. Remote config with HTTP caching (ETag/Last-Modified) and circular include protection
8. W3C traceparent implementation is correct per spec
