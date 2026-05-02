---
id: FB-818
title: Missing nopReporter for Runner unit tests
type: idea
status: pending
priority: medium
complexity: ""
from_project: Cosmo-Smoke
from_path: /Users/gab/PROJECTS/Cosmo-Smoke
to_project: ClaudeCodeSetup
to_target: project
created: "2026-05-02T14:39:30.259533-03:00"
updated: "2026-05-02T14:39:30.259533-03:00"
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

# FB-818: Missing nopReporter for Runner unit tests

## Problem
Every test that creates a runner.Runner needs a reporter.Reporter because runTestOnce() calls r.Reporter.TestStart() and r.Reporter.TestResult(). There is no nop/discard reporter in internal/reporter/, so every test file that tests the Runner has to invent its own nopReporter struct.

## Current vs Expected
Currently, internal/runner/stress_test.go defines its own:
type nopReporter struct{}
func (nopReporter) PrereqStart(string) {}
func (nopReporter) PrereqResult(reporter.PrereqResultData) {}
func (nopReporter) TestStart(string) {}
func (nopReporter) TestResult(reporter.TestResultData) {}
func (nopReporter) Summary(reporter.SuiteResultData) {}

Expected: internal/reporter/nop.go or similar with a NopReporter{} zero-value that satisfies the interface, importable by any test package.

## Why It Matters
Every new test file that exercises Runner.runTestOnce() (or any method that calls Reporter methods) will need this same boilerplate. With 39 assertion types and growing, the number of test files needing a Runner will increase. The stress test feature alone has 9 tests all using the same inline nopReporter.

## Priority
Medium — not blocking, but the boilerplate will compound as more features need Runner-level tests.

## Reproduction
1. Create a test file in internal/runner/ that constructs &Runner{Config: cfg} without setting Reporter
2. Call any method that invokes runTestOnce()
3. Nil pointer dereference panic at runner.go:324 (r.Reporter.TestStart())

## Affected Files
- internal/reporter/reporter.go:6-12 (Reporter interface definition)
- internal/runner/stress_test.go:10-17 (inline nopReporter)
- internal/runner/runner.go:324 (panic site)

## Suggested Implementation
Add internal/reporter/nop.go:
type NopReporter struct{}
func (NopReporter) PrereqStart(string) {}
func (NopReporter) PrereqResult(PrereqResultData) {}
func (NopReporter) TestStart(string) {}
func (NopReporter) TestResult(TestResultData) {}
func (NopReporter) Summary(SuiteResultData) {}

Then replace inline nopReporter in stress_test.go with reporter.NopReporter{}.

