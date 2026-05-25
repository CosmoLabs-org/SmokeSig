---
project: cosmo-smoke
version: 0.11.0
date: 2026-04-19
previous: 0.10.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.11.0 Release Notes

**Release Date**: April 19, 2026

**Previous**: v0.10.0

## Overview

This release brings 2 new features.

## Highlights

Add multi-format reporter chaining via comma-separated --format flag. First format writes to stdout, subsequent formats to auto-named files. Enables CI/CD pipelines and dashboard ingestion from a single run.

## What's New

- Multi-format reporter chaining (--format terminal,json,prometheus)
- add multi-format reporter chaining (commit:779a88f1)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 4 |
| Files changed | 18 |
| New features | 2 |

---
_Full changelog: CHANGELOG.md_
