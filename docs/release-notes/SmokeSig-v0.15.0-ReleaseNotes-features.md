---
project: cosmo-smoke
version: 0.15.0
date: 2026-04-22
previous: 0.14.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.15.0 Release Notes

**Release Date**: April 22, 2026

**Previous**: v0.14.0

## Overview

This release brings 2 new features.

## Highlights

LDAP authenticated bind now reads passwords from environment variables with proper ASN.1 BER encoding. Fails fast when password_env references an unset variable, preventing silent anonymous fallback in CI.

## What's New

- LDAP authenticated bind with password_env support
- implement LDAP authenticated bind with password_env (commit:2948921a)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 4 |
| Files changed | 17 |
| New features | 2 |

---
_Full changelog: CHANGELOG.md_
