---
project: cosmo-smoke
version: 0.20.0
date: 2026-05-02
previous: 0.19.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.20.0 Release Notes

**Release Date**: May 2, 2026

**Previous**: v0.19.0

## Overview

This release brings 6 new features.

## Highlights

Test chaining with data extraction (FEAT-038). Distribution tooling including goreleaser, Docker, and Homebrew (FEAT-036). Stress command for flakiness detection (FEAT-044). File size threshold assertion (FEAT-037).

## What's New

- FEAT-036: Distribution tooling — goreleaser, Docker, Homebrew
- FEAT-044: Flakiness detector — smoke stress command with worker pool
- FEAT-038: Test chaining with data extraction (extract, VarStore, chain detection)
- # FEAT-038: Test chaining with data extraction (BR-08)

**Type**: feature
**Status**: closed
**Plan**: docs/planning-mode/2026-04-30-phase1-test-chaining.md
**Created**: 2026-04-30

## Description

Enable extracting values from one test (extract: field on json_field, stdout_matches) and injecting into subsequent tests via {{ .Vars.name }} templating. Sensitive vars masked. Chained tests run sequentially. ~500 lines. Biggest functional gap for API testing workflows.
- wire chain detection to force sequential execution for chained tests
- wire chain detection to force sequential execution (commit:b76cf89b)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 5 |
| Files changed | 11 |
| New features | 6 |

---
_Full changelog: CHANGELOG.md_
