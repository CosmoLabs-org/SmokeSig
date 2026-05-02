---
project: cosmo-smoke
version: 0.12.0
date: 2026-04-21
previous: 0.11.1
slug: features-and-fixes
title: "features-and-fixes Release"
---

# cosmo-smoke v0.12.0 Release Notes

**Release Date**: April 21, 2026

**Previous**: v0.11.1

## Overview

This release brings 7 new features, and 1 bug fix.

## Highlights

Add two-tier progressive deep link assertion for Android, iOS, React Native, and Flutter projects. Add mobile project type detection with smoke init templates. Fix watch mode config reload and TraceHealth persistence across runs.

## What's New

- Add mobile deep link assertion (FEAT-013)
- Add React Native, Flutter, iOS, Android project detection
- add mobile project smoke init templates (commit:68e269f4)
- wire deep_link assertion into test execution pipeline (commit:0402f91a)
- add deep link assertion with tier 1 HTTP checks and tier 2 resolution (commit:b7fd8c2c)
- add React Native, Flutter, iOS, Android project types (commit:8045997a)
- add DeepLinkCheck struct for mobile deep link assertions (commit:27b1631a)

## Bug Fixes

- Fix watch mode config reload and TraceHealth persistence

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 107 |
| Files changed | 292 |
| New features | 7 |
| Bug fixes | 1 |

---
_Full changelog: CHANGELOG.md_
