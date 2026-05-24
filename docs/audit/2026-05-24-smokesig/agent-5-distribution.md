# Agent 5: Distribution Readiness Audit

## SmokeSig v0.21.1 — Distribution Readiness Audit

### 1. Binary Distribution (Score: 35/100)

**What exists:**
- GoReleaser config at `.goreleaser.yml` (line 1-79) with cross-compilation for linux/darwin/windows on amd64/arm64, CGO_ENABLED=0
- Makefile `build` target (line 47-49) with ldflags version injection
- Dockerfile (multi-stage alpine build, 14 lines)

**Critical bugs:**

1. **GoReleaser ldflags point to dead module path.** `.goreleaser.yml:18` uses `-X github.com/CosmoLabs-org/cosmo-smoke/cmd.Version={{.Version}}` but `go.mod:1` declares `module github.com/CosmoLabs-org/SmokeSig`. GoReleaser builds will produce binaries where the `Version` variable is never injected — every release binary will report `smokesig 0.13.0` (the hardcoded fallback in `cmd/version.go:10`).

2. **GoReleaser binary name is `smoke`, not `smokesig`.** `.goreleaser.yml:19` has `binary: smoke`. The project was renamed to SmokeSig, the Makefile builds `smokesig`, the README says `smokesig`, but GoReleaser would produce a binary named `smoke`.

3. **Dockerfile ldflags point to dead module path.** `Dockerfile:8` uses `-X github.com/CosmoLabs-org/cosmo-smoke/cmd.Version=...`, same problem as GoReleaser.

4. **No GitHub release workflow.** There are only 2 workflow files (`ci.yml`, `smoke.yml`) — neither triggers GoReleaser. The `.goreleaser.yml` exists but nothing invokes `goreleaser release`. Only v0.2.0 has a GitHub release (from April 2026), despite 20 git tags existing. **18 versions have zero binary artifacts on GitHub.**

5. **Version fallback drift.** `cmd/version.go:10` hardcodes `Version = "0.13.0"` but the actual version is `0.21.1`. When ldflags injection fails (which it does, per bug #1), users see a version 8 releases behind.

6. **GoReleaser release target points to dead repo.** `.goreleaser.yml:76-77` targets `owner: CosmoLabs-org, name: cosmo-smoke` — the old repo name. Releases would attempt to publish to the wrong repository.

### 2. Package Managers (Score: 15/100)

**What exists:**
- GoReleaser Homebrew tap config in `.goreleaser.yml:46-56` targeting `CosmoLabs-org/homebrew-tap`
- Docker image templates in `.goreleaser.yml:58-72`

**Critical bugs:**

7. **Homebrew tap references dead binary.** `.goreleaser.yml:54-56` installs `bin.install "smoke"` and tests `system "#{bin}/smoke version"`. Should be `smokesig`.

8. **Homebrew homepage points to dead URL.** `.goreleaser.yml:51` uses `https://github.com/CosmoLabs-org/cosmo-smoke`.

9. **Docker images use old name.** `.goreleaser.yml:63-70` publishes as `cosmolabs/cosmo-smoke:*` instead of `cosmolabs/smokesig:*`.

10. **No presence on any public package manager.** No Homebrew formula exists (the GoReleaser config would create one, but GoReleaser never runs). No apt/snap/Scoop/Chocolatey/Nix/winget packages. No Docker Hub images. The only install path is `go install`.

11. **No install script.** No `curl | sh` installer for users without Go.

### 3. CI/CD Integration (Score: 55/100)

**What exists:**
- `ci.yml` (lines 1-37): Build, test, self-smoke on push/PR to master
- `smoke.yml` (lines 1-88): Reusable workflow for other repos with version pinning, tag filtering, artifact upload
- README documents GitHub Actions, GitLab CI, Docker-based CI, JUnit integration (lines 292-360)
- 7 output formats including `gha` for GitHub Actions annotations and `junit` for CI ingestion

**Issues:**

12. **Reusable workflow references dead module.** `smoke.yml:55-56` runs `go install github.com/CosmoLabs-org/cosmo-smoke@latest`. This will fail since the module path changed to `SmokeSig`.

13. **CI workflow comment uses old name.** `ci.yml:1` says `CI for cosmo-smoke itself`.

14. **No release pipeline.** No workflow triggers GoReleaser on tag push. The standard pattern (`on: push: tags: ['v*']` + `goreleaser-action`) is completely absent.

15. **No matrix testing.** `ci.yml` only tests on `ubuntu-latest`. No macOS or Windows coverage, despite cross-compiling for both.

16. **README reusable workflow example uses stale ref.** `README.md:299` references `CosmoLabs-org/SmokeSig/.github/workflows/smoke.yml@v1` which is correct for the new name, but the workflow itself (`smoke.yml:55`) installs from the old path.

### 4. Installation UX (Score: 40/100)

**What exists:**
- `go install` documented in README (line 9-12)
- Build from source documented (lines 14-19)
- Pre-commit hook documented (lines 21-29)
- `smokesig init` for auto-generating config (31 project types)
- Comprehensive USAGE.md (159 lines)
- SPEC.md for schema reference
- Migration guide from cosmo-smoke (README lines 367-376)

**Issues:**

17. **Only one real install method works: `go install`.** Requires Go toolchain. No pre-built binaries, no Homebrew, no Docker image, no install script. This eliminates non-Go users entirely.

18. **Pre-commit hook version pinned to v0.18.0** in README (line 28), which is 3 versions behind v0.21.1.

19. **Self-smoke config `.smokesig.yaml:46` references `.smoke.yaml`** in init command test: `stdout_contains: ".smoke.yaml"` — should be `.smokesig.yaml`.

20. **STABILITY.md still uses old names throughout.** Line 1: `cosmo-smoke follows...`, Line 8: `smoke run`, `smoke init`, etc. (not `smokesig`).

### 5. Versioning & Releases (Score: 45/100)

**What exists:**
- 20 semantic version tags (v0.2.0 through v0.21.1)
- CHANGELOG.md following Keep a Changelog format (320 lines)
- `.version-registry.json` tracking version 0.21.1, build 449
- STABILITY.md documenting semver guarantees and deprecation policy
- Conventional commits with 100% adherence

**Issues:**

21. **Only 1 of 20 tags has a GitHub release with notes.** v0.2.0 is the only release with description/assets. All others are bare tags with no release notes, no binary artifacts, no checksums.

22. **CHANGELOG has formatting issues.** Several entries contain raw YAML issue metadata inline (e.g., lines 39-48 dump the full FEAT-038 issue body into the changelog entry for v0.20.0). This is not human-readable release documentation.

23. **Version hardcode in `cmd/version.go:10` is `0.13.0`** while actual version is `0.21.1`. This is 8 releases stale. The ldflags injection was meant to override this at build time, but since GoReleaser uses the wrong module path, it would fail.

---

### Summary of stale `cosmo-smoke` references across distribution files:

| File | Count | Severity |
|------|-------|----------|
| `.goreleaser.yml` | 7 | Critical (build/release broken) |
| `.github/workflows/smoke.yml` | 6 | Critical (reusable workflow broken) |
| `Dockerfile` | 1 | Critical (version injection broken) |
| `.github/workflows/ci.yml` | 1 | Low (comment only) |
| `STABILITY.md` | 1 | Medium (documentation) |
| `SPEC.md` | 1 | Medium (documentation) |
| `.smoke.yaml` | 1 | Low (legacy compat file) |
