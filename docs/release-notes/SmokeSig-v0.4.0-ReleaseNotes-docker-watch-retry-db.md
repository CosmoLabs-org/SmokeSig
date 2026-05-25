---
project: cosmo-smoke
version: 0.4.0
date: 2026-04-17
slug: docker-watch-retry-db
title: "docker-watch-retry-db Release"
---

# cosmo-smoke v0.4.0 Release Notes

**Release Date**: April 17, 2026

## Overview

This release brings 8 new features.

## Highlights

_No highlights provided._

## What's New

- Add --watch mode for continuous testing with fsnotify and 500ms debounce
- Add retry with exponential backoff for flaky tests (retry: {count, backoff})
- Add postgres_ping and mysql_ping assertions (stdlib net, no new deps)
- retry with exponential backoff (retry: {count, backoff} on test level)
- postgres_ping assertion via SSLRequest handshake
- mysql_ping assertion via v10 handshake packet
- docker_container_running and docker_image_exists assertions
- watch flag for continuous re-runs on file change via fsnotify with 500ms debounce

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 42 |
| Files changed | 90 |
| New features | 8 |

---
_Full changelog: CHANGELOG.md_
