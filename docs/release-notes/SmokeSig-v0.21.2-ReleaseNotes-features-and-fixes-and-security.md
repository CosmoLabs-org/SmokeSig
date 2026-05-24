---
project: SmokeSig
version: 0.21.2
date: 2026-05-24
previous: 0.21.1
slug: features-and-fixes-and-security
title: "features-and-fixes-and-security Release"
---

# SmokeSig v0.21.2 Release Notes

**Release Date**: May 24, 2026

**Previous**: v0.21.1

## Overview

This release brings 2 new features, and 8 bug fixes.

## Highlights

Complete cosmo-smoke rename across release pipeline and fix 10 audit bugs. Harden HTTP server security, fix race conditions in parallel mode, and add IPv6 support. Add 12 missing assertion types to ExportSchema and ~35 new tests.

## What's New

- comprehensive 10-agent codebase audit (66.1/100) with 21 artifact files
- ~35 new tests increasing coverage for cmd (31.3%), detector (71.6%), runner (76.5%)

## Bug Fixes

- complete cosmo-smoke to SmokeSig rename across release pipeline
- Docker Compose --compose-file flag silently ignored (append no-op)
- DeepLink missing from hasStandaloneAssertions validation
- IPv6 address formatting across 8 assertion types via net.JoinHostPort
- race conditions in parallel mode (lifecycleEnv mutex, backgroundProcesses, WithTrace config cloning)
- 12 missing assertion types added to ExportSchema
- add 12 missing assertion types to ExportSchema (commit:52d12115)
- complete cosmo-smoke rename and fix 6 critical bugs (commit:ea3effb9)

## Security

- HTTP server timeouts, constant-time API key comparison, error sanitization
- Dockerfile hardened with non-root USER, HEALTHCHECK, EXPOSE, STOPSIGNAL
- harden assertions, HTTP server, and parallel execution (commit:b04bc805)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 8 |
| Files changed | 85 |
| New features | 2 |
| Bug fixes | 8 |

---
_Full changelog: CHANGELOG.md_
