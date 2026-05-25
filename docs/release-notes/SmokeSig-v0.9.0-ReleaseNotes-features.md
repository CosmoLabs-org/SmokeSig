---
project: cosmo-smoke
version: 0.9.0
date: 2026-04-19
previous: 0.8.0
slug: features
title: "features Release"
---

# cosmo-smoke v0.9.0 Release Notes

**Release Date**: April 19, 2026

**Previous**: v0.8.0

## Overview

This release brings 8 new features.

## Highlights

Adds trace-aware retry that skips retries when otel_trace confirms delivery. Supports multi-backend trace verification across Jaeger, Tempo, Honeycomb, and Datadog. Exports smoke results as OTLP telemetry. Adds watch mode trace health monitoring with sliding window.

## What's New

- # FEAT-010: Make run field optional for network-only tests

**Type**: feature
**Status**: closed
**Created**: 2026-04-18

## Description

Currently tests require a run field even when only using network assertions (url_reachable, service_reachable, s3_bucket, redis_ping, etc.). Users must add run: 'true' as a dummy. Relax validation: if expect contains at least one network/storage assertion, run can be omitted. The test would skip command execution and only evaluate assertions.

Origin: session-end: validation review
- Trace-aware retry: only retry when otel_trace assertion fails (ROAD-037)
- Multi-backend trace verification: Jaeger, Tempo, Honeycomb, Datadog (ROAD-036)
- Export smoke results as OTLP telemetry to OTel collector (ROAD-035)
- Watch mode trace health monitoring with sliding window (ROAD-038)
- add multi-backend trace reporter with health checks (commit:22c9eb88)
- add GraphQL introspection assertion (commit:acbd0bdf)
- add credential_check assertion type (commit:7c286afd)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 13 |
| Files changed | 92 |
| New features | 8 |

---
_Full changelog: CHANGELOG.md_
