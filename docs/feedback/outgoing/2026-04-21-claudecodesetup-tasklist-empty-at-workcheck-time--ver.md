---
id: FB-627
title: TaskList empty at workcheck time — verification pipeline blind spot
type: idea
status: pending
priority: medium
complexity: ""
from_project: cosmo-smoke
from_path: /Users/gab/PROJECTS/cosmo-smoke
to_project: ClaudeCodeSetup
to_target: project
created: "2026-04-21T19:23:58.120669-03:00"
updated: "2026-04-22T13:57:03.771472-03:00"
suggested_conversion: feature
converted_to: null
related_issues: []
brainstorm_ref: null
session: 2027
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

# FB-627: TaskList empty at workcheck time — verification pipeline blind spot

## Problem
TaskCreate creates in-memory tasks that don't survive context compaction or session boundaries. By the time /workcheck runs (often late in session), TaskList returns empty even though 14 tasks were created and completed. The workcheck skill relies on TaskList to cross-reference task completion against commits — if tasks are gone, verification is incomplete.

## Current vs Expected

**Current:**
- Created 14 tasks via TaskCreate during session
- All marked completed as work finished
- Ran /workcheck late in session
- TaskList returned: \"No tasks found\"
- CCS workcheck JSON showed 13 tasks from a DIFFERENT session (CCS's own tracking), not the current session's TaskCreate tasks
- Workcheck Step 2 says to persist to .claude/task-log.jsonl, but by then the data is already gone

**Expected:**
- TaskList should still show the current session's tasks, OR
- Tasks should be auto-persisted as they're created so workcheck can recover them

## Why It Matters
The /workcheck skill's Step 4 (\"Verify Tasks\") and Step 5 (\"cross-reference TaskList against commits\") are fundamentally broken if TaskList is empty. This session had 14 completed tasks with zero evidence at verification time. The gap means workcheck can never reliably answer \"did we finish what we started?\" for task-tracked work.

This will bite every long session that uses TaskCreate for tracking.

## Priority Justification
Medium — the /workcheck skill is the quality gate at session end. If it can't see completed tasks, it can't catch dropped work. The workaround (CCS workcheck JSON) catches some things but misses TaskCreate-only items.

## Reproduction Steps
1. Start a session, do significant work
2. Create 10+ tasks via TaskCreate, mark them completed as you go
3. Work for 30+ minutes (enough for context to shift/compact)
4. Run /workcheck
5. Call TaskList — observe \"No tasks found\"
6. CCS workcheck JSON may show old tasks from its own system, but not the TaskCreate tasks

## Affected Files
- The /workcheck skill definition (wherever TaskList is called in Step 1 and Step 4)
- Potentially: a new persistence layer for TaskCreate tasks

## Suggested Implementation
Two options:

**Option A: Auto-persist on create.** Every TaskCreate call appends to `.claude/task-log.jsonl`. Workcheck reads from this file instead of (or in addition to) in-memory TaskList. Survives compaction.

**Option B: Workcheck reads CCS task data.** The CCS workcheck JSON already has a task tracking system. Make the /workcheck skill prefer this over in-memory TaskList, since it's more durable.

Option A is simpler and doesn't depend on CCS. The workcheck skill could add a Step 0: \"read .claude/task-log.jsonl if it exists, merge with TaskList.\"


---

**Update (2026-04-22):**



## Session 301 Recurrence (2026-04-22)

Same symptom observed in a fresh session with the current binary (v1.200.1, build 4981 — rebuilt mid-session).

**Fresh evidence:**
- Created 11 tasks (`TaskCreate`) during session: BUG-213/214/215, FEAT-385, FEAT-231, FEAT-254, FEAT-300, FEAT-230, plus P-01/02/03 for FEAT-341.
- Marked tasks in_progress → completed as work finished (e.g. TaskUpdate taskId=7 status=completed after FEAT-385 shipped).
- At /workcheck time late in session: `TaskList` returned `No tasks found`.
- `ccs workcheck --json` .tasks[] showed only historical tasks from sessions 281 and 295/296, not the current session 301 tasks.

**Why this matters (new data point):**
The `/workcheck` Step 5 ("Verify Tasks: Cross-reference TaskList against commits. Flag tasks marked completed without evidence") is structurally blind when TaskList is empty. The report I produced this session skipped Step 5 entirely and just listed stale pending tasks from older sessions as if they were mine.

Status on FB-627 was marked `implemented` but the underlying issue persists — either the implementation didn't land in the binary version I was running, or the .claude/task-log.jsonl write path isn't being triggered by TaskCreate/TaskUpdate. Needs re-investigation.

**Suggested next step:** write a trivial test: `TaskCreate` → read `.claude/task-log.jsonl` → assert line present. If the file isn't being written, the append logic was never actually wired into TaskCreate. This is a 30-minute debug session.
