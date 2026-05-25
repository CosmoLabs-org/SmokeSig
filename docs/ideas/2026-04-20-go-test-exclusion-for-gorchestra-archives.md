---
id: IDEA-039
legacy_id: IDEA-MO7RL70S
title: go test exclusion for GOrchestra archives
created: "2026-04-20T19:26:27.916945-03:00"
status: withered
source: human
origin:
    session: 2027
promoted_to: TASK-001
resolution:
    reason: implemented
    date: "2026-04-20"
    ref: TASK-001
    note: Resolved via TASK-001 (closed)
---

# go test exclusion for GOrchestra archives

# go test exclusion for GOrchestra archives

# go test exclusion for GOrchestra archives

GOrchestra/glm-agents/*/files/ contains .go copies that break go test ./... with compilation errors. Options: (1) add GOrchestra/ to go.work exclude, (2) strip .go files from archives, (3) use go test ./cmd/... ./internal/... as the standard command. Currently using option 3 as workaround.
