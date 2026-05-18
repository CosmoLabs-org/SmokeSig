# Session 2026-05-18 - CCS Integration and Docs

## Date
2026-05-15/18

## Branch
master

## Summary

This session bridged SmokeSig into its parent ecosystem by wiring it as an external dependency of ClaudeCodeSetup (CCS). The rename from cosmo-smoke to SmokeSig (completed in the prior session) left CCS referencing the old binary name across four files: smoke.go, smoke_test.go, rebuild.go, and merge.go. All four were edited to point at the `smokesig` binary and `.smokesig.yaml` config instead of the legacy `cosmo-smoke` / `.smoke.yaml` names. These edits live in the CCS repository and were sent as detailed feedback (FB-959 to CCS inbox, FB-010 tracked locally in SmokeSig) so the CCS side can review and commit them. FEAT-052 was created to track the full dependency wiring as an open feature.

With the cross-project integration documented, the session turned to SmokeSig's own documentation. The USAGE.md was fully rewritten: every reference to `smoke` and `.smoke.yaml` was updated to `smokesig` and `.smokesig.yaml`, all eight output formats (terminal, json, junit, tap, prometheus, gha, backstage, plus comma-separated multi-format) were documented, all subcommands (run, validate, schema, serve, init, version) got usage examples, and a new CCS integration section was added explaining how SmokeSig plugs into the CCS rebuild and merge pipelines. This was the last major doc cleanup after the rename.

Finally, the roadmap was extended with five items reflecting SmokeSig's forward trajectory: ROAD-082 (CCS integration as a p95 priority), ROAD-083 (portfolio-wide adoption across CosmoLabs' ~95 projects), ROAD-084 (push reporter for CI/CD notifications), ROAD-085 (TUI dashboard), and ROAD-086 (Wasm plugins for custom assertions). A changelog entry for the CCS dependency integration was staged. The session was docs-only from SmokeSig's perspective -- no source code changes to the Go binary itself.

## Key Decisions

| Decision | Options Considered | Why This Choice |
|----------|-------------------|-----------------|
| Send CCS edits as feedback rather than committing cross-project | Commit directly to CCS repo vs. feedback pipeline | CCS has its own quality gate and review cycle; edits to 4 CCS files need CCS-side verification before merge. Feedback pipeline (FB-959) preserves the work without bypassing CCS review. |
| Extend roadmap with 5 items | Keep roadmap minimal vs. capture forward vision | SmokeSig is post-rename and entering adoption phase. Capturing integration, adoption, push reporter, TUI, and Wasm directions now prevents losing the momentum from the rename session. |

## Task Log

| # | Task | Status | Notes |
|---|------|--------|-------|
| 1 | Wire SmokeSig into CCS as external dependency | completed | Edited 4 CCS files (smoke.go, smoke_test.go, rebuild.go, merge.go); sent as FB-959 |
| 2 | Rewrite USAGE.md for SmokeSig branding | completed | Full rename, all formats, all subcommands, CCS integration section |
| 3 | Send feedback to CCS (FB-959) | completed | Detailed description of integration work for CCS review |
| 4 | Create FEAT-052 and FB-010 | completed | Tracking CCS dependency integration |
| 5 | Update roadmap with 5 new items | completed | ROAD-082 through ROAD-086 |
| 6 | Stage changelog entry | completed | CCS dependency integration |

## Reference

- **Commits**: `2e1e488` docs(usage): rewrite for SmokeSig branding and full command set, `0e7d068` feat: add CCS dependency integration tracking, `ed54957` chore: update session metadata and intel
- **Files modified**: USAGE.md, docs/issues/FEAT-052.yaml, docs/issues.yaml, docs/feedback/index.yaml, docs/roadmap/index.yaml, docs/roadmap/items/ROAD-082.yaml, docs/roadmap/items/ROAD-083.yaml, docs/roadmap/items/ROAD-084.yaml, docs/roadmap/items/ROAD-085.yaml, docs/roadmap/items/ROAD-086.yaml, docs/prompts/2026-05-05-smokesig-docs.md, .version-registry.json, GOrchestra/intel/architecture.json, GOrchestra/intel/status.json
- **Issues touched**: FEAT-052 (created), FB-010 (created), FEAT-050 (updated)
- **Roadmap items**: ROAD-082 (p95 CCS integration), ROAD-083 (portfolio adoption), ROAD-084 (push reporter), ROAD-085 (TUI dashboard), ROAD-086 (Wasm plugins)
- **Cross-project**: FB-959 sent to CCS inbox for CCS-side review of smoke.go, smoke_test.go, rebuild.go, merge.go edits

## Related

- [SmokeSig Rename Session](Session-2026-05-05-SmokeSig-Rename.md) - Previous session that completed the cosmo-smoke to SmokeSig rename
- [CCS Integration Vision](../brainstorming/2026-04-15-ccs-integration-vision.md) - Original brainstorm for CCS integration
