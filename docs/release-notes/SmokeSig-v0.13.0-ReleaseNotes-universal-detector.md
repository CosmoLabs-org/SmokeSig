---
project: cosmo-smoke
version: 0.13.0
date: 2026-04-21
slug: universal-detector
title: "universal-detector Release"
---

# cosmo-smoke v0.13.0 Release Notes

**Release Date**: April 21, 2026

## Overview

This release brings 5 new features.

## Highlights

_No highlights provided._

## What's New

- # FEAT-013: Mobile deep link assertion

**Type**: feature
**Status**: closed
**Created**: 2026-04-21

## Description

Two-tier progressive deep link assertion for Android, iOS, React Native, Flutter. Tier 1: zero-dep HTTP/config checks. Tier 2: adb/xcrun resolution when available. Design: docs/brainstorming/2026-04-21-mobile-deep-link-assertion.md
- Add 22 project types for universal auto-detection (Java, .NET, Ruby, PHP, Deno, Terraform, Helm, Kustomize, Serverless, Zig, Elixir, Scala, Swift, Dart, Hugo, Astro, Jekyll, Make, CMake, Haskell, Lua)
- Add DNS resolution assertion (dns_resolve) supporting A, AAAA, TXT, MX, CNAME records
- Add SMTP ping assertion (smtp_ping) with EHLO handshake verification
- Add Docker Compose health assertion (docker_compose_healthy) for service status checks

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 16 |
| Files changed | 99 |
| New features | 5 |

---
_Full changelog: CHANGELOG.md_
