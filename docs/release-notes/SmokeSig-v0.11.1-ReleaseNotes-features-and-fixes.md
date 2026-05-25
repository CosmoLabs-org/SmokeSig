---
project: cosmo-smoke
version: 0.11.1
date: 2026-04-20
previous: 0.11.0
slug: features-and-fixes
title: "features-and-fixes Release"
---

# cosmo-smoke v0.11.1 Release Notes

**Release Date**: April 20, 2026

**Previous**: v0.11.0

## Overview

This release brings 8 new features, and 3 bug fixes.

## Highlights

Adds smoke validate for standalone config validation. Adds performance baseline tracking with --baseline flag. Adds smoke schema for JSON export of assertion types. Enhances JUnit XML with timestamp, hostname, and properties metadata.

## What's New

- smoke validate command — standalone config validation
- performance baseline tracking (--baseline, --baseline-threshold)
- smoke schema command — export assertion types as JSON
- JUnit XML CI metadata (timestamp, hostname, properties)
- add timestamp, hostname, and properties to JUnit XML (commit:50118869)
- add smoke schema command for JSON export (commit:591251de)
- add smoke validate command (commit:f4533c8a)
- add performance baseline tracking (commit:7557d744)

## Bug Fixes

- # BUG-001: Watch mode reporter state reset

**Type**: bug
**Status**: closed
**Severity**: medium
**Created**: 2026-04-19

## Description

Watch mode accumulates reporter state across re-runs. File-based reporter (and potentially others) may grow unbounded as results accumulate on each watch cycle. Needs investigation to determine if accumulation is in reporter layer, runner layer, or watch orchestration.
- watch mode reporter state reset
- recreate reporters per watch cycle to prevent state accumulation (commit:b224f524)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 13 |
| Files changed | 44 |
| New features | 8 |
| Bug fixes | 3 |

---
_Full changelog: CHANGELOG.md_
