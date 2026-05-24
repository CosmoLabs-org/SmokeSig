# Agent 6: SEO/Content Audit

## SmokeSig v0.21.1 — SEO/Content Audit

### 1. README Quality (Score: 72/100)

**Strengths:**
- Well-structured: Install > Quick Start > Example > Assertion Types > CLI Reference > Auto-Detection > CI/CD Integration
- Three badge links (Go Reference, Go Report Card, MIT License) at README.md:3
- Excellent assertion type documentation organized by category spanning lines 119-204
- Comprehensive CI/CD section with GitHub Actions, GitLab CI, Docker, and centralized reporting examples (lines 292-360)
- Strong tagline: "Define lightweight 'does it turn on?' checks" (line 5)

**Weaknesses:**
- **No project logo or visual identity.** The banner is plain text.
- **No terminal screenshot or GIF.** Lipgloss output is a selling point but invisible.
- **Pre-commit hook version stale.** README.md:28 references v0.18.0 but current is v0.21.1.
- **No "Why SmokeSig?" section.** Jumps to install without explaining competitive advantage.
- **Missing feature highlights.** 39 assertion types, 31 project detection, OTel integration buried in tables.
- **No contributor or community section.**

### 2. Documentation Depth (Score: 68/100)

**Strengths:**
- USAGE.md: thorough 163-line command reference
- SPEC.md: 328-line formal schema documentation
- STABILITY.md: clear stability tiers and deprecation policy
- docs/FEATURES.md: comprehensive 237-line feature matrix
- Inline --help text is clear

**Weaknesses:**
- **SPEC.md severely outdated:** documents only 5 of 39 assertion types, still titled "cosmo-smoke"
- **FEATURES.md version stale:** says "Version: 0.12.0" when current is 0.21.1, uses old naming
- **CHANGELOG.md formatting issues:** raw issue YAML content pasted inline (lines 40-48, 73-81, etc.)
- **No getting-started tutorial**
- **No man page or --help per assertion type**
- **docs/competitive-analysis/README.md says "No analyses yet"** but research docs exist

### 3. Content Strategy (Score: 35/100)

**Strengths:**
- Competitive analysis exists internally (docs/research/2026-04-15-grok-competitive-analysis.md)
- 7 curated example configs covering diverse stacks
- 18 release notes document evolution

**Weaknesses:**
- **Zero blog posts, tutorials, or guides**
- **No comparison pages** ("SmokeSig vs Goss", etc.)
- **No cookbook or recipes**
- **No website or landing page**
- **Inconsistent release note naming** (mix of Cosmo-Smoke-v*, cosmo-smoke-v*, SmokeSig-v*)
- **No social proof** — no testimonials, no "used by" section

### 4. Discoverability (Score: 40/100)

**Strengths:**
- Clean GitHub URL
- Importable Go module path
- Pre-commit hook and GitHub Actions workflow provide organic discovery

**Weaknesses:**
- No GitHub topics configured, no social preview image, no FUNDING.yml
- No searchable keywords in README intro
- Binary name `smokesig` not self-explanatory
- GoReleaser config still uses old names throughout
- Homebrew formula references old URL

### 5. Example Quality (Score: 82/100)

**Strengths:**
- 7 production-quality example configs (Go, Node, Python, Rust, Docker Compose, K8s, Monorepo)
- Consistent tag conventions documented
- Comments explain WHY, not just WHAT
- Self-smoke config demonstrates dogfooding

**Weaknesses:**
- **Examples use old `.smoke.yaml` filename** — all 7 files, not `.smokesig.yaml`
- **examples/README.md uses old name** — "cosmo-smoke Examples"
- **Self-smoke tests for stale output** (.smokesig.yaml:45 checks for ".smoke.yaml")
- **No examples for 30+ assertion types** — only exit_code and stdout
- **No CI example config**

---

### Critical Rename Inconsistency (Cross-Cutting)

60+ stale `cosmo-smoke` and `.smoke.yaml` references across:

| File | Count | Severity |
|------|-------|----------|
| `.goreleaser.yml` | 8 | Critical |
| `Dockerfile:7` | 1 | Critical |
| `.github/workflows/smoke.yml` | 7 | High |
| `SPEC.md` | 4 | High |
| `STABILITY.md` | 2 | High |
| `docs/FEATURES.md` | 8 | High |
| `examples/README.md` | 15+ | High |
| `examples/*/.smoke.yaml` | 7 files | Medium |
| `.smokesig.yaml:45` | 1 | Low |
