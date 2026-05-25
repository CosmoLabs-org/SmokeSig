---
project: cosmo-smoke
version: 0.8.0
date: 2026-04-19
previous: 0.7.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.8.0 Release Notes

**Release Date**: April 19, 2026

**Previous**: v0.7.0

## Overview

This release brings 4 new features.

## Highlights

Smoke tests now propagate W3C trace context into downstream services via HTTP, gRPC, and WebSocket assertions. A new otel_trace assertion verifies traces arrive at Jaeger-compatible collectors. CLI flags enable runtime otel control without config changes.

## What's New

- OpenTelemetry trace correlation with W3C traceparent propagation
- otel_trace assertion querying Jaeger API for trace verification
- add --otel-collector and --no-otel CLI flags (commit:5a1d491e)
- add OpenTelemetry trace correlation (FEAT-012) (commit:14f504d6)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 10 |
| Files changed | 44 |
| New features | 4 |

---
_Full changelog: CHANGELOG.md_
