---
project: cosmo-smoke
version: 0.16.0
date: 2026-04-30
previous: 0.15.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.16.0 Release Notes

**Release Date**: April 30, 2026

**Previous**: v0.15.0

## Overview

This release brings 4 new features.

## Highlights

New file_size assertion with byte thresholds. Test chaining with variable extraction and secret masking. Gemini ecosystem feedback brainstorm yielding 15 planned features.

## What's New

- file_size assertion with byte thresholds (min/max, human-readable formatting)
- test chaining with variable extraction (VarStore, {{ .Vars.X }} templates, extract from json_field/stdout_matches, secret masking)
- add test chaining with variable extraction (commit:b08e78f3)
- add file_size assertion with byte thresholds (commit:909d8bc1)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 7 |
| Files changed | 49 |
| New features | 4 |

---
_Full changelog: CHANGELOG.md_
