# Agent 8: Data/Schema Design Audit

## SmokeSig v0.21.1 — Data/Schema Audit

### 1. Config Schema (Score: 78/100)
- 40 assertion types with proper YAML tags and pointer types for optionals
- Custom Duration type for human-readable YAML timeouts
- Config inheritance via includes: with circular-include guard (max depth 10)
- extends: directive with HTTP caching (ETag/Last-Modified)
- **12 assertion types missing from ExportSchema()** (export.go) — smokesig schema returns incomplete data
- MergeEnv mutates base parameter in-place
- YAML parsing silently ignores unknown keys — typos dropped without error

### 2. Type Safety (Score: 82/100)
- Proper pointer types for optional fields (ExitCode *int, StatusCode *int)
- Thread-safe VarStore with sync.RWMutex
- CheckHTTPWithTrace mutates input *schema.HTTPCheck headers in-place — corrupts across retries
- backgroundProcesses global slice has no mutex
- CheckEnvExists uses Getenv != "" instead of LookupEnv
- Regex compiled at runtime without caching (5 instances)

### 3. Data Integrity (Score: 75/100)
- Validation returns ALL errors at once
- Regex patterns validated at config-load time
- Credential check properly redacts values
- **DeepLink missing from hasStandaloneAssertions()** — tests with only deep_link: incorrectly require run:
- **15 assertion types have zero validation** (empty URLs, zero ports accepted)
- RedisCheck.Password in plaintext while others use PasswordEnv pattern
- OTelTraceCheck.APIKey also plaintext

### 4. Migration Support (Score: 72/100)
- Goss migration covers all 14 resource types with 3-tier warning system
- Distro-aware package checking (deb/rpm/apk)
- Emitted config lacks version:1 and project: — fails validation
- GossAttrs is map[string]interface{} with silent zero-value type assertions

### 5. Storage Design (Score: 80/100)
- SQLite with proper indexes and auto-pruning
- Baseline comparison with percentage-based regression thresholds
- Dashboard API validates required fields
- No database schema versioning for future migrations
- GetProjects uses O(n^2) correlated subquery
