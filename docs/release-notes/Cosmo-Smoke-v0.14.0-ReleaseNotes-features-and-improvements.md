---
project: cosmo-smoke
version: 0.14.0
date: 2026-04-22
previous: 0.13.0
slug: features-and-improvements
title: "features-and-improvements Release"
---

# cosmo-smoke v0.14.0 Release Notes

**Release Date**: April 22, 2026

**Previous**: v0.13.0

## Overview

This release brings 4 new features, and 2 improvements.

## Highlights

Seven new assertion types bring the total to 39. Added ICMP ping, MongoDB, Kafka, LDAP, MQTT, NTP, and Kubernetes checks. Refactored detector tests to use slices.Contains. Test count grew from 850 to 910.

## What's New

- ICMP, MongoDB, Kafka, LDAP, MQTT, NTP, and K8s assertion types (39 total)
- add ICMP, MongoDB, Kafka, LDAP, MQTT, NTP, and K8s checks (commit:bba9d052)
- add DNS, SMTP, and Docker Compose health checks (commit:be61e35b)
- add 22 project types for universal auto-detection (commit:d4e67c9c)

## Improvements

- Replaced 29 manual loops with slices.Contains in detector tests
- replace manual loops with slices.Contains (commit:4f4f94b0)

## Breaking Changes

> _None in this release_

## Upgrade Instructions

No breaking changes in this release. Standard upgrade applies.

## Stats

| Metric | Value |
|--------|-------|
| Commits | 21 |
| Files changed | 117 |
| New features | 4 |

---
_Full changelog: CHANGELOG.md_
