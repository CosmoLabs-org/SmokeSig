---
project: cosmo-smoke
version: 0.5.0
date: 2026-04-18
previous: 0.4.0
slug: features-and-improvements
title: "features-and-improvements Release"
---

# cosmo-smoke v0.5.0 Release Notes

**Release Date**: April 18, 2026

**Previous**: v0.4.0

## Overview

This release brings 14 new features, and 1 improvement.

## Highlights

v0.7 brings WebSocket support with a zero-dependency client for connect-send-expect patterns, monorepo sub-config discovery to run smoke tests across all subdirectories, and an opt-in gRPC build tag that cuts the default binary by 25%. Network assertions no longer require a dummy run command, and the assertion engine has been reorganized into domain-focused files.

## What's New

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

## Improvements

- Split assertion.go into per-domain files

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 23 |
| Files changed | 113 |
| New features | 14 |

---
_Full changelog: CHANGELOG.md_
