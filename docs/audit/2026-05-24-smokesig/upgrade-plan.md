# Upgrade Plan

## Phase 0: Critical Fixes (1-2 days)

### P0-1: Complete cosmo-smoke → SmokeSig Rename
- `.goreleaser.yml`: ldflags path (line 18), binary name (19), Docker images (63-70), Homebrew (46-56), release target (76-77)
- `Dockerfile`: ldflags path (line 7), binary name (12-13)
- `.github/workflows/smoke.yml`: install path (line 55), description, comments
- `.github/workflows/ci.yml`: comment (line 1)
- `SPEC.md`: title + all references
- `STABILITY.md`: binary name + `smoke observe` (nonexistent command)
- `docs/FEATURES.md`: version, assertion count, all references
- `examples/README.md`: title + all references
- `examples/*/.smoke.yaml`: rename to `.smokesig.yaml`

### P0-2: Fix Critical Code Bugs
- `assertion_docker.go:65`: Change `args = append([]string{...})` to `args = []string{...}`
- `assertion_smtp.go:38-62`: Remove manual greeting read, let smtp.NewClient handle it
- `validate.go:226`: Add `t.Expect.DeepLink != nil` to `hasStandaloneAssertions`
- `cmd/version.go:10`: Update fallback from `0.13.0` to `0.21.1`
- `internal/mcp/server.go:196`: Change "29 assertion types" to "40"
- `internal/mcp/server.go:20`: Update version from `0.9.0` to `0.21.1`

## Phase 1: Foundation (3-5 days)

### P1-1: Release Pipeline
- Create `.github/workflows/release.yml` (GoReleaser on tag push)
- Add golangci-lint step to `ci.yml`
- Add matrix testing (macOS, Windows)

### P1-2: Security Hardening
- `Dockerfile`: Add `RUN adduser -D smoke` + `USER smoke`, HEALTHCHECK, EXPOSE, LABEL
- `serve.go`: Add ReadTimeout (10s), WriteTimeout (30s), ReadHeaderTimeout (5s)
- `handler.go:30`: Use `crypto/subtle.ConstantTimeCompare` for API key
- `handler.go:55,72,123`: Sanitize error messages, don't expose internals

### P1-3: Concurrency Fixes
- Add `sync.RWMutex` to `Runner.lifecycleEnv` or copy map immutably before parallel dispatch
- Add mutex to `backgroundProcesses` slice or move to Runner instance
- Clone `HTTPCheck`/`WebSocketCheck`/`GRPCCheck` before mutating headers in WithTrace functions
- Add semaphore to `runParallel` (match stress.go pattern)

### P1-4: IPv6 Fix
- Replace `fmt.Sprintf("%s:%d")` with `net.JoinHostPort()` in:
  - assertion_db.go (4 instances)
  - assertion_ldap.go, assertion_mongo.go, assertion_network.go, assertion_smtp.go

### P1-5: Data Integrity
- Rebuild issue index (add FEAT-013 through FEAT-052)
- Clean CHANGELOG corruption (8 entries with raw YAML)
- Update roadmap items ROAD-077, ROAD-078, ROAD-079 to completed

## Phase 2: Quality (1-2 weeks)

### P2-1: Documentation
- Rewrite SPEC.md from scratch (all 40 assertion types)
- Add 12 missing types to ExportSchema()
- Update FEATURES.md to v0.21.1
- Document MCP server, Dashboard API, lifecycle hooks, extends
- Add `backstage` to README Output Formats and --format help
- Update examples to use `.smokesig.yaml` filename

### P2-2: Code Quality
- Add validation for 15 unvalidated assertion types
- Replace `conn.Read` with `io.ReadFull` in protocol parsers (MongoDB, MySQL, Kafka, LDAP)
- Fix `CheckEnvExists` to use `os.LookupEnv`
- Add warning on push/OTel reporter failures
- Add `--verbose`/`--quiet` flags to run command
- Fix validation prefix inconsistency (`test[%d]` → `tests[%d]`)

### P2-3: Schema Improvements
- Migrate `RedisCheck.Password` to `PasswordEnv` pattern
- Consider `yaml.v3 KnownFields(true)` for strict parsing
- Fix Goss migration to emit `version:1` and `project:`

## Phase 3: Growth (2-4 weeks)

### P3-1: Distribution
- Add terminal screenshot/GIF to README
- Create install.sh script for non-Go users
- Publish Docker images to GHCR
- Create Homebrew formula (after release workflow works)

### P3-2: Content
- Write "Why SmokeSig?" README section
- Create comparison pages (vs Goss, Bats, Terratest)
- Add GitHub topics for discoverability
- Document getting-started tutorial

### P3-3: Features
- Add negation assertions (stdout_not_contains, file_not_exists)
- Add scheduled execution mode (agent/cron)
- Add webhook notifications (Slack, PagerDuty)
- Add structured logging (slog)
