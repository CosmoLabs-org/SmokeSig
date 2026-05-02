---
project: cosmo-smoke
version: 0.17.0
date: 2026-04-30
previous: 0.16.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.17.0 Release Notes

**Release Date**: April 30, 2026

**Previous**: v0.16.0

## Overview

This release brings 6 new features.

## Highlights

Add GitHub Actions native output reporter with --format gha. Add setup/teardown lifecycle hooks (before_all, after_all, before_each, after_each, env_pass). Add remote config inheritance via extends: URL with HTTP caching.

## What's New

- # FEAT-040: Setup/teardown lifecycle hooks (BR-10)

**Type**: feature
**Status**: closed
**Created**: 2026-04-30

## Description

Extend prerequisites: with after_all (guaranteed execution on failure/SIGINT), before_each/after_each per-test hooks, always_run flag, environment passing (capture KEY=VALUE from stdout). Context-based timeouts. Signal interception. ~500 lines extending prereq.go.
- GitHub Actions native output reporter (--format gha)
- Setup/teardown lifecycle hooks (before_all, after_all, before_each, after_each, env_pass)
- Remote config inheritance via extends URL with HTTP caching
- add setup/teardown lifecycle hooks (commit:8a601564)
- add GitHub Actions native output reporter (commit:d60e4eba)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 16 |
| Files changed | 37 |
| New features | 6 |

---
_Full changelog: CHANGELOG.md_
