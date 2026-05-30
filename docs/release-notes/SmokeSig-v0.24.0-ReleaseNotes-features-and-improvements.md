---
project: SmokeSig
version: 0.24.0
date: 2026-05-29
previous: 0.23.0
slug: features-and-improvements
title: "features-and-improvements Release"
---

# SmokeSig v0.24.0 Release Notes

**Release Date**: May 29, 2026

**Previous**: v0.23.0

## Overview

This release brings 7 new features, and 1 improvement.

## Highlights

Interactive TUI mode with Bubbletea (FEAT-051), test coverage 75%→88.5% across 12 packages, monorepo at 100%

## What's New

- # FEAT-045: Auto-Add Generator — smokesig observe command (BR-13)

**Type**: feature
**Status**: closed
**Created**: 2026-04-30

## Description

New command smokesig observe "cmd" [--dir] [--quiet]. io.MultiWriter stdout capture. Pre/post filesystem SHA-256 diffing. gopsutil port detection. Intelligent sanitization (ANSI, timestamps, UUIDs). Interactive mode via huh. ~1,080 lines.
- Stack-aware observation via detector integration — portless-first port detection and stack-specific HTTP probe paths (FEAT-046)
- Interactive TUI mode with Bubbletea (--tui flag, -tags tui build tag)
- Runner.RunSingle method for single-test re-execution
- add lipgloss styles, key bindings, bubbletea deps (FEAT-051) (commit:d3f38629)
- add RunSingle for single-test re-execution (FEAT-051) (commit:5de65fa3)
- detector-observer integration with portless-first hints (FEAT-046) (commit:7aaad3e9)

## Improvements

- Test coverage improved from 75% to 88.5% across all packages

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 49 |
| Files changed | 150 |
| New features | 7 |

---
_Full changelog: CHANGELOG.md_
