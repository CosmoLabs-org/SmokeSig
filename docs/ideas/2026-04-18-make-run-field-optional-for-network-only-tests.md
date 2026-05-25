---
id: IDEA-037
legacy_id: IDEA-MO53CPHE
title: Make run field optional for network-only tests
created: "2026-04-18T22:32:28.802598-03:00"
status: withered
source: agent
origin:
    session: 2027
    trigger: 'session-end: validation review'
tags:
    - ux
    - v0.8
promoted_to: FEAT-010
resolution:
    reason: implemented
    date: "2026-04-19"
    ref: FEAT-010
    note: Resolved via FEAT-010 (closed)
---

# Make run field optional for network-only tests

# Make run field optional for network-only tests

# Make run field optional for network-only tests

Currently tests require a run field even when only using network assertions (url_reachable, service_reachable, s3_bucket, redis_ping, etc.). Users must add run: 'true' as a dummy. Relax validation: if expect contains at least one network/storage assertion, run can be omitted. The test would skip command execution and only evaluate assertions.
