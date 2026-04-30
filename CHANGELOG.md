# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.16.0] - 2026-04-30

### Added
- file_size assertion with byte thresholds (min/max, human-readable formatting)
- test chaining with variable extraction (VarStore, {{ .Vars.X }} templates, extract from json_field/stdout_matches, secret masking)
- add test chaining with variable extraction (commit:b08e78f3)
- add file_size assertion with byte thresholds (commit:909d8bc1)

## [0.15.0] - 2026-04-22

### Added
- LDAP authenticated bind with password_env support
- implement LDAP authenticated bind with password_env (commit:2948921a)

## [0.14.0] - 2026-04-22

### Added
- ICMP, MongoDB, Kafka, LDAP, MQTT, NTP, and K8s assertion types (39 total)
- add ICMP, MongoDB, Kafka, LDAP, MQTT, NTP, and K8s checks (commit:bba9d052)
- add DNS, SMTP, and Docker Compose health checks (commit:be61e35b)
- add 22 project types for universal auto-detection (commit:d4e67c9c)

### Changed
- Replaced 29 manual loops with slices.Contains in detector tests
- replace manual loops with slices.Contains (commit:4f4f94b0)

## [v0.13.0] - 2026-04-21

### Added
- # FEAT-013: Mobile deep link assertion

**Type**: feature
**Status**: closed
**Created**: 2026-04-21

## Description

Two-tier progressive deep link assertion for Android, iOS, React Native, Flutter. Tier 1: zero-dep HTTP/config checks. Tier 2: adb/xcrun resolution when available. Design: docs/brainstorming/2026-04-21-mobile-deep-link-assertion.md
- Add 22 project types for universal auto-detection (Java, .NET, Ruby, PHP, Deno, Terraform, Helm, Kustomize, Serverless, Zig, Elixir, Scala, Swift, Dart, Hugo, Astro, Jekyll, Make, CMake, Haskell, Lua)
- Add DNS resolution assertion (dns_resolve) supporting A, AAAA, TXT, MX, CNAME records
- Add SMTP ping assertion (smtp_ping) with EHLO handshake verification
- Add Docker Compose health assertion (docker_compose_healthy) for service status checks

## [0.12.0] - 2026-04-21

### Added
- Add mobile deep link assertion (FEAT-013)
- Add React Native, Flutter, iOS, Android project detection
- add mobile project smoke init templates (commit:68e269f4)
- wire deep_link assertion into test execution pipeline (commit:0402f91a)
- add deep link assertion with tier 1 HTTP checks and tier 2 resolution (commit:b7fd8c2c)
- add React Native, Flutter, iOS, Android project types (commit:8045997a)
- add DeepLinkCheck struct for mobile deep link assertions (commit:27b1631a)

### Fixed
- Fix watch mode config reload and TraceHealth persistence

## [0.11.1] - 2026-04-20

### Added
- smoke validate command — standalone config validation
- performance baseline tracking (--baseline, --baseline-threshold)
- smoke schema command — export assertion types as JSON
- JUnit XML CI metadata (timestamp, hostname, properties)
- add timestamp, hostname, and properties to JUnit XML (commit:50118869)
- add smoke schema command for JSON export (commit:591251de)
- add smoke validate command (commit:f4533c8a)
- add performance baseline tracking (commit:7557d744)

### Fixed
- # BUG-001: Watch mode reporter state reset

**Type**: bug
**Status**: closed
**Severity**: medium
**Created**: 2026-04-19

## Description

Watch mode accumulates reporter state across re-runs. File-based reporter (and potentially others) may grow unbounded as results accumulate on each watch cycle. Needs investigation to determine if accumulation is in reporter layer, runner layer, or watch orchestration.
- watch mode reporter state reset
- recreate reporters per watch cycle to prevent state accumulation (commit:b224f524)

## [0.11.0] - 2026-04-19

### Added
- Multi-format reporter chaining (--format terminal,json,prometheus)
- add multi-format reporter chaining (commit:779a88f1)

## [0.10.0] - 2026-04-19

### Added
- portfolio smoke dashboard with push reporting
- MCP server for Claude Desktop integration — 7 tools over stdio (ROAD-032)
- add MCP server for Claude Desktop integration (ROAD-032) (commit:72321bcc)
- add portfolio smoke dashboard with push reporting (commit:dfcc8a9c)

## [0.9.0] - 2026-04-19

### Added
- # FEAT-010: Make run field optional for network-only tests

**Type**: feature
**Status**: closed
**Created**: 2026-04-18

## Description

Currently tests require a run field even when only using network assertions (url_reachable, service_reachable, s3_bucket, redis_ping, etc.). Users must add run: 'true' as a dummy. Relax validation: if expect contains at least one network/storage assertion, run can be omitted. The test would skip command execution and only evaluate assertions.

Origin: session-end: validation review
- Trace-aware retry: only retry when otel_trace assertion fails (ROAD-037)
- Multi-backend trace verification: Jaeger, Tempo, Honeycomb, Datadog (ROAD-036)
- Export smoke results as OTLP telemetry to OTel collector (ROAD-035)
- Watch mode trace health monitoring with sliding window (ROAD-038)
- add multi-backend trace reporter with health checks (commit:22c9eb88)
- add GraphQL introspection assertion (commit:acbd0bdf)
- add credential_check assertion type (commit:7c286afd)

## [0.8.0] - 2026-04-19

### Added
- OpenTelemetry trace correlation with W3C traceparent propagation
- otel_trace assertion querying Jaeger API for trace verification
- add --otel-collector and --no-otel CLI flags (commit:5a1d491e)
- add OpenTelemetry trace correlation (FEAT-012) (commit:14f504d6)

## [0.7.0] - 2026-04-18

### Added
- add WebSocket assertion, monorepo discovery, gRPC build tag, optional run field (commit:089eac65)
- add pre-commit hook integration (commit:7c7f2483)
- implement v0.6 connect-and-verify assertions (commit:4a515fa5)
- add url_reachable, service_reachable, s3_bucket, version_check types (commit:6edb4502)
- add skip_if conditional execution and env config merge (commit:4b426ba8)
- add Goss-to-cosmo-smoke migration tool (ROAD-024) (commit:c4226da8)

## [0.6.0] - 2026-04-18

### Added
- add WebSocket assertion, monorepo discovery, gRPC build tag, optional run field (commit:089eac65)
- add pre-commit hook integration (commit:7c7f2483)
- implement v0.6 connect-and-verify assertions (commit:4a515fa5)
- add url_reachable, service_reachable, s3_bucket, version_check types (commit:6edb4502)
- add skip_if conditional execution and env config merge (commit:4b426ba8)
- add Goss-to-cosmo-smoke migration tool (ROAD-024) (commit:c4226da8)

## [0.5.0] - 2026-04-18

### Added
- smoke migrate goss: one-command Goss to cosmo-smoke migration with core 7 key mapping, --distro/--strict/--stats flags
- skip_if: conditional test execution via env_unset, env_equals, file_missing conditions
- Multi-environment configs via --env flag with deep-merge onto base config
- # FEAT-009: Pre-commit hook integration

**Type**: feature
**Status**: closed
**Created**: 2026-04-18

## Description

Pre-commit framework hook for smoke run integration
- WebSocket connect-send-expect assertion (stdlib-only)
- Monorepo sub-config auto-discovery with --monorepo flag
- Optional gRPC module via build tag (-tags grpc)
- Run field optional for network-only tests
- add WebSocket assertion, monorepo discovery, gRPC build tag, optional run field (commit:089eac65)
- add pre-commit hook integration (commit:7c7f2483)
- implement v0.6 connect-and-verify assertions (commit:4a515fa5)
- add url_reachable, service_reachable, s3_bucket, version_check types (commit:6edb4502)
- add skip_if conditional execution and env config merge (commit:4b426ba8)
- add Goss-to-cosmo-smoke migration tool (ROAD-024) (commit:c4226da8)

### Changed
- Split assertion.go into per-domain files

## [v0.4.0] - 2026-04-17

### Added
- Add --watch mode for continuous testing with fsnotify and 500ms debounce
- Add retry with exponential backoff for flaky tests (retry: {count, backoff})
- Add postgres_ping and mysql_ping assertions (stdlib net, no new deps)
- retry with exponential backoff (retry: {count, backoff} on test level)
- postgres_ping assertion via SSLRequest handshake
- mysql_ping assertion via v10 handshake packet
- docker_container_running and docker_image_exists assertions
- watch flag for continuous re-runs on file change via fsnotify with 500ms debounce

## [0.3.0] - 2026-04-16

### Added
- # FEAT-006: TAP output format

**Type**: feature
**Status**: closed
**Created**: 2026-04-16

## Description

Test Anything Protocol output for broader CI compatibility. Simpler than JUnit, widely supported.
- # FEAT-005: Process running assertion

**Type**: feature
**Status**: closed
**Created**: 2026-04-16

## Description

New assertion: process_running. Check if process exists by name or pattern. Syntax: process_running: 'nginx'. For daemon/service smoke tests.
- add grpc_health assertion via standard health protocol (commit:67532938)
- add redis_ping and memcached_version assertions (commit:ec481107)
- add ssl_cert assertion for TLS certificate validation (commit:1ddd7880)
- add prometheus text-format output (commit:e1912cd8)
- add response_time_ms threshold assertion (commit:d3981133)
- add allow_failure flag for flaky tests (commit:46999819)
- add process_running assertion type (commit:a778725b)

### Fixed
- harden process_running after Opus review (commit:2023700e)
- inject version via ldflags in Makefile (commit:d5d6c5f0)

## [0.2.0] - 2026-04-16

### Added
- add --from-running container inspection (commit:47c889a2)
- add config inheritance with includes and templates (commit:9ea0581c)
- add HTTP endpoint and JSON field assertions (commit:ab31b785)
- add TAP v14 output format (commit:8eb6794a)
- add port_listening assertion type (commit:db28d927)
- add JUnit XML output format (commit:5a2a2cbb)
- add stderr_matches and env_exists assertion types (commit:df0c7e3a)
- add HTTP health endpoint for container probes (commit:bddc019b)

