---
id: IDEA-034
legacy_id: IDEA-MO1X1N40
title: Parallel-agent merge-conflict SOP
created: "2026-04-16T17:12:36.288701-03:00"
status: harvested
source: human
origin:
    session: 2026
promoted_to: ROAD-033
---

# Parallel-agent merge-conflict SOP

# Parallel-agent merge-conflict SOP

Document the proven pattern from v0.3.0 session: Opus briefs → Sonnet agents in worktrees → ccs verify-worktree review → sequential merge with union-resolve for schema.go/assertion.go/runner.go adjacent-line conflicts + checkout --ours for metadata files (.ccsession.json, .review.json, conversation-transcripts). Pattern scales to 7+ parallel agents, proven at 5-way parallel dispatch for v0.3 assertion pack. Would be useful as a reusable command or skill across CosmoLabs projects.
