---
project: cosmo-smoke
version: 0.19.0
date: 2026-05-02
previous: 0.18.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.19.0 Release Notes

**Release Date**: May 2, 2026

**Previous**: v0.18.0

## Overview

This release brings 5 new features.

## Highlights

smoke stress command for flakiness detection, v1.0 release readiness (README polish, semver guarantees, distribution tooling), upgrade audit cleanup

## What's New

- FEAT-044: Flakiness detector — smoke stress command
- FEAT-036: v1.0 release readiness — README polish, API audit, semver guarantees, distribution tooling
- add distribution tooling — goreleaser, Docker, Homebrew (commit:c1775662)
- add smoke stress command with Cobra wiring (commit:49fded3d)
- add stress test engine with worker pool and error dedup (commit:75e12363)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 15 |
| Files changed | 34 |
| New features | 5 |

---
_Full changelog: CHANGELOG.md_
