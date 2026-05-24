# Agent 10: Documentation Audit

## SmokeSig v0.21.1 — Documentation Audit

### 1. Completeness (Score: 72/100)
- 40 YAML assertion fields in Expect struct — `file_size` missing from README
- `backstage` format missing from README Output Formats and `--format` help string
- `smokesig stress`, `smokesig migrate goss` undocumented in README
- Lifecycle hooks (before_all/after_all/before_each/after_each) completely undocumented
- `extends:` remote config feature undocumented
- 8 flags missing from README CLI Reference (otel-collector, no-otel, report-url, etc.)
- HTTP assertion docs missing headers, body, timeout fields

### 2. Accuracy (Score: 58/100)
- **SPEC.md catastrophically outdated**: documents 5 of 40 assertion types, titled "cosmo-smoke"
- **FEATURES.md frozen at v0.12.0**: says 31 assertion types (actual 40), uses old names
- STABILITY.md references old `smoke` binary and nonexistent `smoke observe` command
- examples/README.md and all 7 examples use old `.smoke.yaml` and `smoke` binary name
- Version.go hardcoded to 0.13.0; MCP says 0.9.0; CLAUDE.md says 0.13.0
- MCP server says "29 assertion types" (actual: 40)
- CHANGELOG has raw issue metadata leaked into 8 entries
- README pre-commit hook pinned to v0.18.0 (current v0.21.1)

### 3. Onboarding (Score: 82/100)
- README Quick Start is excellent — 5 clear steps
- 31 project type auto-detection well-documented
- USAGE.md provides thorough workflow guide
- 7 example directories well-organized
- No getting-started tutorial or troubleshooting section
- SPEC.md (the "full reference") is 87% incomplete

### 4. API Documentation (Score: 35/100)
- MCP Server: 7 tools registered — zero user-facing documentation
- Dashboard API: 3 endpoints — zero documentation
- Reporter interface: not documented for custom reporters
- Push reporter payload format undocumented

### 5. Example Quality (Score: 74/100)
- 7 realistic examples covering Go, Node, Python, Docker, Rust, K8s, Monorepo
- All use old `.smoke.yaml` filename
- No examples for 30+ newer assertion types
- No CI example config
