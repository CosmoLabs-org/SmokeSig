# Agent 9: Infrastructure Audit

## SmokeSig v0.21.1 — Infrastructure Audit

### 1. Container Quality (Score: 52/100)
- Multi-stage Docker build with CGO_ENABLED=0 and minimal alpine
- **No non-root USER** — container runs as root
- No HEALTHCHECK, EXPOSE, LABEL, or STOPSIGNAL directives
- **Stale ldflags path** — Dockerfile uses cosmo-smoke, version injection broken
- Binary named `smoke` instead of `smokesig`

### 2. CI/CD Pipeline (Score: 60/100)
- CI runs build, test, self-smoke on push/PR to master
- Uses go-version-file: go.mod for Go version sync
- Reusable workflow with configurable inputs
- **No release workflow** — GoReleaser config exists but nothing triggers it
- **No lint step** — golangci-lint absent from CI
- **No security scanning** — no govulncheck, CodeQL, or Dependabot
- **No matrix testing** — ubuntu-only
- Reusable workflow installs from stale cosmo-smoke module path

### 3. Build System (Score: 72/100)
- Clean Makefile with self-documenting help target
- Version injection via git describe + ldflags (Makefile properly updated)
- GoReleaser covers 6 platforms, Homebrew tap, Docker images
- **GoReleaser ldflags use stale path** — all releases have broken version
- GoReleaser binary name `smoke`, Docker images `cosmo-smoke`
- Makefile `check` target depends on undefined `type-check`

### 4. Deployment (Score: 55/100)
- go install works, pre-commit hook support
- GoReleaser references all use old name
- No install script, no self-update mechanism
- No systemd unit file or Helm chart
- Dashboard deployment undocumented

### 5. Observability (Score: 68/100)
- Excellent: 7 output formats, OTel/OTLP, Prometheus, Backstage, push reporting
- SQLite dashboard with auto-pruning
- Trace health tracking with sliding window
- OTel export fire-and-forget — spans may be lost
- No structured logging anywhere
- **HTTP server has no timeouts** — vulnerable to slowloris
- **API key compared with ==** — timing-vulnerable (handler.go:30)
- Internal error messages leaked to HTTP clients (handler.go:55,72,123)
