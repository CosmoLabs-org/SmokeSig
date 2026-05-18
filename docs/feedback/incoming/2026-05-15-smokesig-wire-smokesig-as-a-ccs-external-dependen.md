---
id: FB-010
title: Wire SmokeSig as a CCS external dependency
type: feature
status: implemented
priority: high
complexity: medium
from_project: SmokeSig
from_path: /Users/gab/PROJECTS/SmokeSig
to_project: SmokeSig
to_target: self
created: "2026-05-15T13:06:31.277302-03:00"
updated: "2026-05-15T13:48:19.609048-03:00"
suggested_conversion: feature
converted_to: null
related_issues: []
brainstorm_ref: null
session: 2027
suggested_workflow:
  - brainstorming
  - implementation
response:
  acknowledged: null
  acknowledged_by: null
  started: null
  implemented: null
  rejected: null
  rejection_reason: null
  notes: Converted to FEAT-052. CCS files already edited in SmokeSig session — CCS needs to review, commit, and rebuild.
---

# FB-010: Wire SmokeSig as a CCS external dependency

Cross-project feedback FROM SmokeSig session TO ClaudeCodeSetup.

SmokeSig (formerly cosmo-smoke) has been renamed and significantly expanded (39 assertion types, 8 output formats, watch mode, monorepo, dashboard) but CCS still references the old binary name and config. Needs to be wired as a proper external dependency alongside GoRalph.

FILES CHANGED IN CCS (from SmokeSig session — CCS must review and commit):

1. tools/ccsession/cmd/smoke.go — Full rewrite: binary name smoke->smokesig, lookPath smoke->smokesig, knownSubcmds expanded (run/validate/init/schema/serve/version), install hint updated to ccs rebuild smokesig, error messages updated.

2. tools/ccsession/cmd/smoke_test.go — Added test cases for validate, schema, serve passthrough. Updated log messages from smoke to smokesig.

3. tools/ccsession/cmd/rebuild.go — Added smokesig as external tool (SourceDir PROJECTS/SmokeSig, BinaryName smokesig, External true). Updated ValidArgs, build-all default, error message, Long description example.

4. tools/ccsession/cmd/merge.go — Line 1142 comment: cosmo-smoke->SmokeSig.

5. SmokeSig/USAGE.md — Full rewrite with smokesig commands, .smokesig.yaml config, CCS integration section.

WHAT CCS STILL NEEDS TO DO:
- Review and commit the 4 CCS files above
- ccs rebuild ccs to deploy updated binary
- ccs rebuild smokesig to build+deploy SmokeSig to ~/bin/smokesig
- Verify ccs smoke delegates to smokesig run
- Update CLAUDE.md references if needed
- Consider adding SmokeSig to ccs sync or ccs project-init for new machines

PROJECT DETAILS: github.com/CosmoLabs-org/SmokeSig, binary smokesig, config .smokesig.yaml, path ~/bin/smokesig, source ~/PROJECTS/SmokeSig/, version 0.20.1

## Suggested Workflow

1. brainstorming
2. implementation

