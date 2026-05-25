---
id: IDEA-038
legacy_id: IDEA-MO6B5E6M
title: Watch mode reporter state reset
created: "2026-04-19T18:58:30.670295-03:00"
status: withered
source: human
origin:
    session: 2027
promoted_to: BUG-001
resolution:
    reason: implemented
    date: "2026-04-19"
    ref: BUG-001
    note: Resolved via BUG-001 (closed)
---

# Watch mode reporter state reset

# Watch mode reporter state reset

# Watch mode reporter state reset

File-based reporters (JSON, JUnit, Prometheus, TAP) accumulate internal state across watch mode re-runs because they're created once and never reset. Additionally, open files aren't truncated between cycles. This produces corrupt/accumulated output after the first watch cycle when using multi-format chaining like --format terminal,json. Fix: either recreate reporters per watch cycle (move buildReporter inside runOnce), or add a Reset() method to the Reporter interface that clears internal slices and truncates files.
