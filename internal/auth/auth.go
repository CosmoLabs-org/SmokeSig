// Package auth implements OIDC-based cloud authentication for SmokeSig.
//
// v1 Scope:
//   - AWS STS AssumeRoleWithWebIdentity (raw HTTP, no SDK)
//   - GCP Workload Identity Federation (raw HTTP, no SDK)
//   - CI auto-detection: GitHub Actions, GitLab CI, CircleCI
//
// v1 Limitations:
//   - Credential injection is env-var-only. run: commands and k8s_resource (kubectl)
//     consume them. Standalone assertions (s3_bucket, http, url_reachable) use raw
//     net/http without AWS SigV4 signing — they remain anonymous.
//   - SigV4 request signing for standalone assertions is a v2 item.
//   - Azure Managed Identity is a v2 item (different architecture: VM-attached).
//   - No per-assertion auth header injection in v1.
//
// GCP credential format:
//   - Default: CLOUDSDK_AUTH_ACCESS_TOKEN env var (no temp file)
//   - Optional: gcp_credential_format=keyfile writes a temp JSON file and sets
//     GOOGLE_APPLICATION_CREDENTIALS. SIGTERM handler registered for cleanup.
package auth

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Provider identifies a cloud provider.
type Provider string

const (
	ProviderAWS   Provider = "aws"
	ProviderGCP   Provider = "gcp"
	ProviderAzure Provider = "azure"
)

// Credentials holds temporary cloud credentials obtained via OIDC exchange.
// Byte slices (not strings) enable zeroing on cleanup.
type Credentials struct {
	Provider       Provider
	ProfileName    string
	AccessKeyID    []byte // AWS
	SecretAccessKey []byte // AWS
	SessionToken   []byte // AWS
	AccessToken    []byte // GCP, Azure
	Expiration     time.Time
	// GCP keyfile path (only when gcp_credential_format=keyfile)
	KeyfilePath string
	// Azure-specific
	TenantID       string
	ClientID       string
	SubscriptionID string
}

// Expired returns true if credentials have expired or are within the skew window.
func (c *Credentials) Expired(skew time.Duration) bool {
	if c.Expiration.IsZero() {
		return false
	}
	return time.Now().Add(skew).After(c.Expiration)
}

// Zero overwrites all credential byte slices with zeros.
// Best-effort: Go's GC may have copied the data, but this closes the obvious window.
func (c *Credentials) Zero() {
	for i := range c.AccessKeyID {
		c.AccessKeyID[i] = 0
	}
	for i := range c.SecretAccessKey {
		c.SecretAccessKey[i] = 0
	}
	for i := range c.SessionToken {
		c.SessionToken[i] = 0
	}
	for i := range c.AccessToken {
		c.AccessToken[i] = 0
	}
}

// String returns a redacted representation (first 4 chars of key ID only).
func (c *Credentials) String() string {
	switch c.Provider {
	case ProviderAWS:
		id := string(c.AccessKeyID)
		if len(id) > 4 {
			id = id[:4] + "***"
		}
		return fmt.Sprintf("aws:%s", id)
	case ProviderGCP:
		tok := string(c.AccessToken)
		if len(tok) > 4 {
			tok = tok[:4] + "***"
		}
		return fmt.Sprintf("gcp:%s", tok)
	case ProviderAzure:
		tok := string(c.AccessToken)
		if len(tok) > 4 {
			tok = tok[:4] + "***"
		}
		return fmt.Sprintf("azure:%s", tok)
	default:
		return "unknown"
	}
}

// EnvVars returns the environment variables to set for these credentials.
func (c *Credentials) EnvVars() map[string]string {
	vars := make(map[string]string)
	switch c.Provider {
	case ProviderAWS:
		vars["AWS_ACCESS_KEY_ID"] = string(c.AccessKeyID)
		vars["AWS_SECRET_ACCESS_KEY"] = string(c.SecretAccessKey)
		vars["AWS_SESSION_TOKEN"] = string(c.SessionToken)
	case ProviderGCP:
		vars["CLOUDSDK_AUTH_ACCESS_TOKEN"] = string(c.AccessToken)
		if c.KeyfilePath != "" {
			vars["GOOGLE_APPLICATION_CREDENTIALS"] = c.KeyfilePath
		}
	case ProviderAzure:
		vars["AZURE_CLIENT_ID"] = c.ClientID
		vars["AZURE_TENANT_ID"] = c.TenantID
		vars["AZURE_FEDERATED_TOKEN"] = string(c.AccessToken)
		if c.SubscriptionID != "" {
			vars["AZURE_SUBSCRIPTION_ID"] = c.SubscriptionID
		}
	}
	return vars
}

// AuthContext carries resolved credentials for the current test execution.
// Passed internally within the runner package — not a public API.
type AuthContext struct {
	mu       sync.RWMutex
	profiles map[string]*Credentials
	active   string // currently active profile name
}

// NewAuthContext creates an empty auth context.
func NewAuthContext() *AuthContext {
	return &AuthContext{
		profiles: make(map[string]*Credentials),
	}
}

// Set stores credentials for a named profile.
func (ac *AuthContext) Set(name string, creds *Credentials) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.profiles[name] = creds
}

// Get returns credentials for a named profile, or nil if not found.
func (ac *AuthContext) Get(name string) *Credentials {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.profiles[name]
}

// Active returns the currently active credentials (default profile or overridden).
func (ac *AuthContext) Active() *Credentials {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	if ac.active != "" {
		return ac.profiles[ac.active]
	}
	return ac.profiles["default"]
}

// SetActive sets the active profile name.
func (ac *AuthContext) SetActive(name string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.active = name
}

// CleanupKeyfiles removes any GCP temp keyfiles from all profiles.
func (ac *AuthContext) CleanupKeyfiles() {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	for _, c := range ac.profiles {
		if c.KeyfilePath != "" {
			os.Remove(c.KeyfilePath)
		}
	}
}

// ZeroAll zeroes all stored credentials.
func (ac *AuthContext) ZeroAll() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	for _, c := range ac.profiles {
		c.Zero()
	}
}
