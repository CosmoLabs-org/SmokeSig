---
id: FB-1040
title: 'Session-end SOP: integrate smokesig run as verification step'
type: feature
status: pending
priority: medium
complexity: ""
from_project: SmokeSig
from_path: /Users/gabstudio/PROJECTS/SmokeSig
to_project: ClaudeCodeSetup
to_target: project
created: "2026-05-24T19:29:14.054385-03:00"
updated: "2026-05-24T19:29:14.054385-03:00"
suggested_conversion: feature
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

# FB-1040: Session-end SOP: integrate smokesig run as verification step

Add a phase to session-end SOP between 'run tests' and 'commit': (1) check if .smokesig.yaml exists, (2) if yes run smokesig run --fail-fast, (3) surface failures as warnings not blockers, (4) if no config skip silently. Optional: if session modified cmd/*.go, auto-run only doc_integrity tagged tests via smokesig run --tag docs. Smoke tests catch integration-level regressions unit tests miss: HTTP endpoints, Docker builds, docs matching CLI reality via doc_integrity, ports, certs. Lightweight addition — ~5 lines in the SOP.

