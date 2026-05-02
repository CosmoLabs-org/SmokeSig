---
project: cosmo-smoke
version: 0.3.0
date: 2026-04-16
previous: 0.2.0
slug: features-and-fixes
title: "features-and-fixes Release"
---

# cosmo-smoke v0.3.0 Release Notes

**Release Date**: April 16, 2026

**Previous**: v0.2.0

## Overview

This release brings 9 new features, and 2 bug fixes.

## Highlights

v0.3.0 — The Assertion Pack. Expanded from 10 to 15 assertion types with v0.3 Assertion Pack: process_running, response_time_ms, ssl_cert, redis_ping, memcached_version, and grpc_health. Added Prometheus text-format reporter for observability integration. Added allow_failure flag for flaky tests. Fixed Makefile ldflags for proper version injection. Implemented via parallel Sonnet agent dispatch (5 agents in isolated worktrees) with mandatory Opus quality-gate review. Test suite grew from 144 to 176 tests. Positions cosmo-smoke as the modern Goss successor with pattern-first extensibility.

## What's New

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

## Bug Fixes

- harden process_running after Opus review (commit:2023700e)
- inject version via ldflags in Makefile (commit:d5d6c5f0)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 54 |
| Files changed | 97 |
| New features | 9 |
| Bug fixes | 2 |

---
_Full changelog: CHANGELOG.md_
