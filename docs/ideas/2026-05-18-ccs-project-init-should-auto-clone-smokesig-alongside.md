---
id: IDEA-020
title: CCS project-init should auto-clone SmokeSig alongside GoRalph
created: "2026-05-18T02:58:21.882665-03:00"
status: seed
source: human
origin:
    session: 2027
---

# CCS project-init should auto-clone SmokeSig alongside GoRalph

When setting up a new machine via ccs project-init, SmokeSig should be cloned to ~/PROJECTS/SmokeSig automatically. It's a dependency at the same tier as GoRalph. This ensures ccs rebuild smokesig works on day one.
