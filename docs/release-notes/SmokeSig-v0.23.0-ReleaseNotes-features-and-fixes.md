---
project: SmokeSig
version: 0.23.0
date: 2026-05-26
previous: 0.22.0
slug: features-and-fixes
title: "features-and-fixes Release"
---

# SmokeSig v0.23.0 Release Notes

**Release Date**: May 26, 2026

**Previous**: v0.22.0

## Overview

This release brings 9 new features, and 3 bug fixes.

## Highlights

Auto-Add Generator: smokesig observe wraps commands, detects ports, snapshots files, and generates .smokesig.yaml automatically. Fork bomb prevention: runner-level recursion guard protects all projects from infinite go test loops (BUG-012). Comprehensive docs refresh for landing page readiness.

## What's New

- Auto-Add Generator — smokesig observe command wraps processes, detects ports, snapshots files, probes HTTP, generates .smokesig.yaml (FEAT-045)
- add observe command with smoke test generation (commit:b80d4180)
- add Generate function for observation-to-YAML config conversion (commit:113539c1)
- add Observe command wrapper with tests (commit:5ff5ec7f)
- add ProbeEndpoints for HTTP health probing (commit:cc982283)
- add DetectPorts and parseLsofOutput for port detection (commit:f879d0f0)
- add string sanitization and key phrase extraction (commit:8c3ef1d1)
- add TakeSnapshot and DiffSnapshots for file change detection (commit:ef4c8b3b)
- add foundation types for smokesig observe command (commit:e90ea932)

## Bug Fixes

- Runner-level recursion guard prevents fork bombs when .smokesig.yaml contains test runner commands (BUG-012)
- MCP server TestSmokeRunAgainstSelf now accounts for AllowedFailures in result count assertion
- add recursion guard to prevent fork bombs (BUG-012) (commit:250a7f80)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 40 |
| Files changed | 50 |
| New features | 9 |
| Bug fixes | 3 |

---
_Full changelog: CHANGELOG.md_
