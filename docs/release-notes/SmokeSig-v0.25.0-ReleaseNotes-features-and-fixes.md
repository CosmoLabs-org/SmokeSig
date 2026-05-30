---
project: SmokeSig
version: 0.25.0
date: 2026-05-30
previous: 0.24.0
slug: features-and-fixes
title: "features-and-fixes Release"
---

# SmokeSig v0.25.0 Release Notes

**Release Date**: May 30, 2026

**Previous**: v0.24.0

## Overview

This release brings 9 new features, and 2 bug fixes.

## Highlights

OIDC cloud auth for AWS, GCP, and Azure CI role assumption. Wasm plugin system for custom assertions via wazero. Interactive TUI test runner with Bubbletea. Formalized exit code contract.

## What's New

- OIDC cloud authentication for AWS, GCP, Azure CI role assumption (FEAT-049)
- WebAssembly plugin system for custom assertions via wazero (FEAT-048)
- Interactive TUI test runner with Bubbletea (FEAT-051)
- Formalized exit code contract: 0=pass, 1=fail, 2=config, 3=prereq
- add AuthConfig types for OIDC providers (FEAT-049) (commit:84b0ff55)
- add WebAssembly plugin system for custom assertions (FEAT-048) (commit:02c2484a)
- add OIDC cloud authentication for CI role assumption (FEAT-049) (commit:94d1d06f)
- add interactive TUI test runner (FEAT-051) (commit:1609ca47)
- formalize exit code contract (0=pass, 1=fail, 2=config, 3=prereq) (commit:8145295a)

## Bug Fixes

- LoadDefault() fallback wired into run and validate commands
- wire LoadDefault() fallback into run and validate commands (commit:5636e98b)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 23 |
| Files changed | 105 |
| New features | 9 |
| Bug fixes | 2 |

---
_Full changelog: CHANGELOG.md_
