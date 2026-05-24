---
id: FB-011
title: 'Doc-integrity check type: detect stale documentation vs actual CLI features'
type: feature
status: pending
priority: medium
complexity: complex
from_project: WikipediaDB
from_path: /Users/gabstudio/PROJECTS/WikipediaDB
to_project: SmokeSig
to_target: project
created: "2026-05-24T05:45:51.06921-03:00"
updated: "2026-05-24T05:45:51.06921-03:00"
suggested_conversion: feature
converted_to: null
related_issues: []
brainstorm_ref: null
session: 2027
suggested_workflow:
  - brainstorming
  - plan-mode
  - implementation
response:
  acknowledged: null
  acknowledged_by: null
  started: null
  implemented: null
  rejected: null
  rejection_reason: null
  notes: ""
---

# FB-011: Doc-integrity check type: detect stale documentation vs actual CLI features

## What happened
When CosmoWiki added new commands (setup, tui, export) and flags (--plan, --format, --lang=all), the README, CLAUDE.md, skill files, and USAGE.md all became stale. There's no automated way to detect this — it only surfaces when an agent tries a documented command that doesn't exist, or misses a feature that was never documented.

## Why it matters
AI agents (GLM 5.1, Claude Sonnet) rely on skill files and documentation to know what tools are available. Stale docs cause agents to: (1) try commands that were removed, (2) miss new features that could solve their task, (3) use wrong flags. This is a systematic problem across all CosmoLabs projects.

## Proposed solution
New SmokeSig check type: doc-integrity. Algorithm:

1. Run `<binary> --help` → parse subcommand list
2. For each subcommand: `<binary> <cmd> --help` → parse flags
3. Parse README.md, CLAUDE.md, USAGE.md, skill files for command references
4. Set diff both directions:
   - 'setup command exists in --help but not documented in README' (undocumented feature)
   - 'README documents --format=dot but export --help doesn't list it' (stale doc)
   - 'Skill mentions cosmowiki query but command doesn't exist' (broken reference)
5. Optionally: extract code block examples from docs → run each → check exit 0

Config in .smokesig.yaml:
```yaml
doc_integrity:
  binary: bin/cosmowiki
  docs:
    - README.md
    - CLAUDE.md
    - docs/skills/cosmowiki.md
  check_examples: true
```

This is ~300-400 lines of Go. The core is: run commands, parse text, parse markdown, compare sets. All deterministic, no LLM needed. SmokeSig already runs commands and checks output — this is a natural extension.

## Priority justification
High — affects every CosmoLabs project. Every time we add features, docs drift. This catches it automatically in CI or during ccs smoke.

## Suggested Workflow

1. brainstorming
2. plan-mode
3. implementation

