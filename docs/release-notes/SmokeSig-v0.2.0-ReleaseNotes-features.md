---
project: cosmo-smoke
version: 0.2.0
date: 2026-04-16
previous: 0.1.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.2.0 Release Notes

**Release Date**: April 16, 2026

**Previous**: v0.1.0

## Overview

This release brings 8 new features.

## Highlights

v0.2.0 transforms cosmo-smoke from a basic smoke tester into a comprehensive testing toolkit. This release adds HTTP endpoint assertions with status/body/header checks, JSON field assertions using gjson for JSONPath queries, and config inheritance via includes directive and Go templates. Four output formats now supported (terminal, json, junit, tap). Container inspection via --from-running flag enables automatic test generation from running Docker containers. Total assertion types expanded from 5 to 10.

## What's New

- add --from-running container inspection (commit:47c889a2)
- add config inheritance with includes and templates (commit:9ea0581c)
- add HTTP endpoint and JSON field assertions (commit:ab31b785)
- add TAP v14 output format (commit:8eb6794a)
- add port_listening assertion type (commit:db28d927)
- add JUnit XML output format (commit:5a2a2cbb)
- add stderr_matches and env_exists assertion types (commit:df0c7e3a)
- add HTTP health endpoint for container probes (commit:bddc019b)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 33 |
| Files changed | 113 |
| New features | 8 |

---
_Full changelog: CHANGELOG.md_
