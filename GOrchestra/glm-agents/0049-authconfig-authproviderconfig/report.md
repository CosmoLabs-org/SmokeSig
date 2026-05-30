---
id: "0049"
task: "Add AuthConfig and AuthProviderConfig types to internal/schema/schema.go.  Add these types after the LifecycleConfig struct:  type AuthConfig struct {     Providers []AuthProviderConfig `yaml:\"providers\"` }  type AuthProviderConfig struct {     Type           string `yaml:\"type\"`     RoleARN        string `yaml:\"role_arn,omitempty\"`     Region         string `yaml:\"region,omitempty\"`     SessionName    string `yaml:\"session_name,omitempty\"`     ProjectNumber  string `yaml:\"project_number,omitempty\"`     PoolID         string `yaml:\"pool_id,omitempty\"`     ProviderID     string `yaml:\"provider_id,omitempty\"`     ServiceAccount string `yaml:\"service_account,omitempty\"`     TenantID       string `yaml:\"tenant_id,omitempty\"`     ClientID       string `yaml:\"client_id,omitempty\"`     SubscriptionID string `yaml:\"subscription_id,omitempty\"` }  Add field to SmokeConfig struct:     Auth *AuthConfig `yaml:\"auth,omitempty\"`  Verify:   go test ./internal/schema/ -v   go build ./...  Commit: ccs commit -m \"feat(schema): add AuthConfig types for OIDC providers (FEAT-049)\" "
status: merged
date: 2026-05-30
duration: 8m36s
commit: d299fb9c7cf40c9e7621fa749c00a8053098f996
files_changed: 1
---

# Agent Report

## Task
Add AuthConfig and AuthProviderConfig types to internal/schema/schema.go for OIDC-based cloud role assumption (FEAT-049).

## What I Did
1. Read `internal/schema/schema.go` to locate insertion points: SmokeConfig struct (line 14-26) and LifecycleConfig struct (line 62-68).
2. Added `Auth *AuthConfig` field to SmokeConfig struct, between Lifecycle and Tests fields.
3. Added `AuthConfig` struct (with Providers slice) and `AuthProviderConfig` struct (with Type, RoleARN, Region, SessionName, ProjectNumber, PoolID, ProviderID, ServiceAccount, TenantID, ClientID, SubscriptionID fields) after LifecycleConfig.
4. Ran `go test ./internal/schema/ -v` — all tests passed.
5. Ran `go build ./...` — build succeeded.
6. Committed with `ccs commit -m "feat(schema): add AuthConfig types for OIDC providers (FEAT-049)"`.

## Decisions Made
- Placed AuthConfig/AuthProviderConfig after LifecycleConfig to keep related config types grouped logically.
- Used pointer `*AuthConfig` on SmokeConfig (consistent with optional config patterns like `*RetryPolicy`).
- All provider-specific fields are `omitempty` since only a subset apply per provider type (AWS uses role_arn/region/session_name, GCP uses project_number/pool_id/provider_id/service_account, Azure uses tenant_id/client_id/subscription_id).

## Verification
- Build: pass
- Vet/Lint: pass (implicit via build)
- Tests: pass (all existing schema tests green)

## Files Changed
- `internal/schema/schema.go` — added AuthConfig struct, AuthProviderConfig struct, Auth field on SmokeConfig

## Issues or Concerns
- None. The types are pure data structs with no behavior, so existing tests are unaffected. No new tests were needed since the types have no validation logic yet (validation will be added when the runner consumes Auth config).
