# Task

Add AuthConfig and AuthProviderConfig types to internal/schema/schema.go.

Add these types after the LifecycleConfig struct:

type AuthConfig struct {
    Providers []AuthProviderConfig `yaml:"providers"`
}

type AuthProviderConfig struct {
    Type           string `yaml:"type"`
    RoleARN        string `yaml:"role_arn,omitempty"`
    Region         string `yaml:"region,omitempty"`
    SessionName    string `yaml:"session_name,omitempty"`
    ProjectNumber  string `yaml:"project_number,omitempty"`
    PoolID         string `yaml:"pool_id,omitempty"`
    ProviderID     string `yaml:"provider_id,omitempty"`
    ServiceAccount string `yaml:"service_account,omitempty"`
    TenantID       string `yaml:"tenant_id,omitempty"`
    ClientID       string `yaml:"client_id,omitempty"`
    SubscriptionID string `yaml:"subscription_id,omitempty"`
}

Add field to SmokeConfig struct:
    Auth *AuthConfig `yaml:"auth,omitempty"`

Verify:
  go test ./internal/schema/ -v
  go build ./...

Commit: ccs commit -m "feat(schema): add AuthConfig types for OIDC providers (FEAT-049)"

