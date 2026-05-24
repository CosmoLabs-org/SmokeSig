# Agent 3: Frontend/UX/Growth Audit

## SmokeSig v0.21.1 — Frontend/UX/Growth Audit

### 1. CLI UX (Score: 72/100)
- Clean command hierarchy but banner is plain text with no visual identity (root.go:10-11)
- No usage examples on most commands — only `migrate goss` has Examples
- `--format` flag help text omits `gha` and `backstage` (run.go:109)
- No `--verbose`/`--quiet` flags for debugging or scripting
- Version fallback hardcoded at 0.13.0 (version.go:10)
- `run` is not the default command

### 2. Terminal Output (Score: 81/100)
- Clean Lipgloss styling with ANSI 256 colors that respect terminal themes (terminal.go:13-18)
- Smart 3-tier duration formatting (terminal.go:125-133)
- Inline spinner via carriage return — dependency-free progress (terminal.go:50-51)
- Allowed-failure tests get distinct yellow `~` treatment (terminal.go:64-76)
- Watch mode has proper UX with change detection messages
- Missing: project name header, [N/M] progress counter, prerequisite visual grouping

### 3. Error Messages (Score: 78/100)
- Validation returns ALL errors at once (validate.go:20-219) — best practice
- Prerequisite failures include actionable hints with install URLs
- Legacy config fallback warns on .smoke.yaml
- Push reporter silently swallows all HTTP errors (push.go:83-98) — data loss
- OTel reporter fires spans with no WaitGroup (otel.go:76) — spans lost on exit
- Validation prefix inconsistency: `test[%d]` vs `tests[%d]` (validate.go:44 vs 36)

### 4. Onboarding (Score: 74/100)
- 31 project type auto-detection with prerequisite install hints
- `--from-running` Docker container inspection
- `migrate goss` for competitive migration
- No interactive mode for `init`, no next-step guidance, no comments in generated YAML
- MCP server says "29 assertion types" (server.go:196) — actual is 39+

### 5. Output Formats (Score: 85/100)
- 7 formats with pluggable Reporter interface (chain.go:15-23)
- Multi-format via comma separation — first to stdout, rest to auto-named files
- JUnit, TAP v14, Prometheus, GitHub Actions, Backstage all standards-compliant
- OTel reporter fire-and-forget goroutines — spans may be lost
- JSON output missing timestamp field
