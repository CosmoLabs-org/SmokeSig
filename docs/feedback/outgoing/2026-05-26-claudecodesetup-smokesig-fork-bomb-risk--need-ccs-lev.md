---
id: FB-1043
title: SmokeSig fork bomb risk — need CCS-level process monitoring for ccs smoke and GLM agents
type: bug
status: pending
priority: high
complexity: ""
from_project: SmokeSig
from_path: /Users/gabstudio/PROJECTS/SmokeSig
to_project: ClaudeCodeSetup
to_target: project
created: "2026-05-26T04:25:31.830138-03:00"
updated: "2026-05-26T04:25:31.830138-03:00"
suggested_conversion: bug
converted_to: null
related_issues: []
brainstorm_ref: null
session: 2026
suggested_workflow: []
response:
  acknowledged: null
  acknowledged_by: null
  started: null
  implemented: null
  rejected: null
  rejection_reason: null
  notes: ""
---

# FB-1043: SmokeSig fork bomb risk — need CCS-level process monitoring for ccs smoke and GLM agents

SmokeSig BUG-012: .smokesig.yaml containing 'go test ./...' + MCP handler tests that execute the config = infinite fork bomb. Fixed in SmokeSig with SMOKESIG_RUNNING env var guard. But CCS needs protection too: (1) ccs smoke should monitor child process count and kill trees exceeding 10 children from a single test, (2) GLM agent worktrees need a process ceiling (50 procs hard-kill), (3) smokesig audit should flag .smokesig.yaml configs where a test command matches the project's own test runner. The pattern is natural and WILL recur in other projects.

