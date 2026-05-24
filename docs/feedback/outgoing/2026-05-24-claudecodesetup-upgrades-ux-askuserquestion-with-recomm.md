---
id: FB-1039
title: 'Upgrades UX: AskUserQuestion with recommended fix bundles'
type: feature
status: pending
priority: high
complexity: ""
from_project: SmokeSig
from_path: /Users/gabstudio/PROJECTS/SmokeSig
to_project: ClaudeCodeSetup
to_target: project
created: "2026-05-24T19:29:10.916597-03:00"
updated: "2026-05-24T19:29:10.916597-03:00"
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

# FB-1039: Upgrades UX: AskUserQuestion with recommended fix bundles

After the audit phase, present findings via AskUserQuestion with structured options: All safe fixes (Recommended), Critical only, Pick individually, Audit only. Bundle recommendations by risk level. Include SmokeSig integration: if project lacks .smokesig.yaml, include 'Add smoke tests' as a safe fix (calls smokesig init). If .smokesig.yaml exists, run smokesig audit --json and surface recommendations as upgrade items. Key improvements: (1) one-click 'Apply all safe fixes' default, (2) multiSelect for individual picks, (3) each recommendation shows files affected and reversibility, (4) SmokeSig audit results feed directly into upgrade recommendations.

