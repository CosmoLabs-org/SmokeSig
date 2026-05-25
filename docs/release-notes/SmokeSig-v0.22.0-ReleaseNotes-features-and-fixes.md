---
project: SmokeSig
version: 0.22.0
date: 2026-05-25
previous: 0.21.2
slug: features-and-fixes
title: "features-and-fixes Release"
---

# SmokeSig v0.22.0 Release Notes

**Release Date**: May 25, 2026

**Previous**: v0.21.2

## Overview

This release brings 6 new features, and 3 bug fixes.

## Highlights

5 new assertion types (ios_simulator, android_emulator, doc_integrity, plus Slack/PagerDuty webhooks), smokesig audit command, init --with-doc-integrity, --verbose/--quiet flags, SMTP fix, reporter failure warnings, golangci-lint CI, complete SPEC.md rewrite

## What's New

- add Slack and PagerDuty webhook notifications (commit:7bcb9c0b)
- add smokesig audit command for config gap analysis (commit:b8f2590a)
- auto-include doc_integrity for CLI projects (commit:8d205f5e)
- add ios_simulator and android_emulator health checks (commit:256bb5ce)
- add doc_integrity check for stale documentation detection (commit:0a79a7ca)
- add --verbose and --quiet flags for output verbosity (commit:71dcb87b)

## Bug Fixes

- increase test suite timeout to 120s in self-smoke config (commit:11354dbc)
- surface warnings on push and OTel export failures (commit:36ac0bb8)
- remove double-handshake in SMTP assertion (commit:aae967c3)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 86 |
| Files changed | 196 |
| New features | 6 |
| Bug fixes | 3 |

---
_Full changelog: CHANGELOG.md_
