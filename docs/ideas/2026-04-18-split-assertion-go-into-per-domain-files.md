---
id: IDEA-036
legacy_id: IDEA-MO53CPEM
title: Split assertion.go into per-domain files
created: "2026-04-18T22:32:28.70252-03:00"
status: withered
source: agent
origin:
    session: 2027
    trigger: 'brainplan: v0.7 prep — assertion.go is 800 lines and growing'
tags:
    - architecture
    - v0.7
promoted_to: FEAT-011
resolution:
    reason: implemented
    date: "2026-04-19"
    ref: FEAT-011
    note: Resolved via FEAT-011 (closed)
---

# Split assertion.go into per-domain files

# Split assertion.go into per-domain files

# Split assertion.go into per-domain files

Split internal/runner/assertion.go into domain-focused files: assertion_process.go (exit_code, stdout, stderr), assertion_file.go (file_exists, env_exists), assertion_network.go (port, process, http, ssl), assertion_db.go (redis, memcached, postgres, mysql), assertion_reachable.go (url_reachable, service_reachable, s3_bucket), assertion_version.go (version_check), assertion_docker.go (container, image). No code changes — just file moves. Reduces per-file complexity and makes build-tag splits easier.
