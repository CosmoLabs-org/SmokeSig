package auth

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

const tokenExpirySkew = 30 * time.Second
const watchRefreshWindow = 5 * time.Minute

// cachedCtx holds credentials from the last successful exchange.
// Used by watch mode to avoid redundant STS calls.
var (
	cachedCtx *AuthContext
	cachedMu  sync.Mutex
)

// ExchangeAll performs OIDC token exchange for all configured auth profiles.
// Returns an AuthContext with all credentials, or error on first failure (unless fallback=env).
func ExchangeAll(authCfg schema.AuthConfig) (*AuthContext, error) {
	if len(authCfg.Profiles) == 0 {
		return nil, nil // No auth configured — not an error
	}

	ctx := NewAuthContext()
	fallback := authCfg.Fallback
	if fallback == "" {
		fallback = "fail"
	}

	for _, profile := range authCfg.Profiles {
		name := profile.Name
		if name == "" {
			name = "default"
		}

		creds, err := exchangeProfile(profile)
		if err != nil {
			if fallback == "env" {
				// Log warning, continue with whatever env provides
				fmt.Fprintf(os.Stderr, "⚠ auth profile %q: OIDC exchange failed, falling back to environment: %v\n", name, err)
				continue
			}
			return nil, fmt.Errorf("auth profile %q: %w", name, err)
		}

		creds.ProfileName = name
		ctx.Set(name, creds)
	}

	// Set the first profile as active by default
	if len(authCfg.Profiles) > 0 {
		name := authCfg.Profiles[0].Name
		if name == "" {
			name = "default"
		}
		ctx.SetActive(name)
	}

	return ctx, nil
}

// ExchangeAllCached returns cached credentials if still valid, otherwise performs fresh exchange.
// Used by watch mode to avoid redundant STS calls between cycles.
func ExchangeAllCached(authCfg schema.AuthConfig) (*AuthContext, error) {
	cachedMu.Lock()
	defer cachedMu.Unlock()

	if cachedCtx != nil && !NeedsRefresh(cachedCtx) {
		return cachedCtx, nil
	}

	ctx, err := ExchangeAll(authCfg)
	if err != nil {
		return nil, err
	}

	// Zero old cached credentials
	if cachedCtx != nil {
		cachedCtx.ZeroAll()
	}
	cachedCtx = ctx
	return ctx, nil
}

// exchangeProfile detects CI, validates token, and exchanges for a single profile.
func exchangeProfile(profile schema.AuthProfile) (*Credentials, error) {
	// Detect CI and acquire OIDC token
	result, err := DetectCI(profile.TokenEnv, profile.Audience)
	if err != nil {
		return nil, err
	}

	// Validate token expiry locally before network call
	if err := ValidateTokenExp(result.Token, tokenExpirySkew); err != nil {
		return nil, err
	}

	// Exchange token with cloud provider
	switch Provider(profile.Provider) {
	case ProviderAWS:
		return ExchangeAWS(
			profile.RoleARN,
			profile.Audience,
			profile.Region,
			profile.SessionDuration,
			result.Token,
			"", // production endpoint
		)
	case ProviderGCP:
		format := profile.GCPCredentialFormat
		if format == "" {
			format = "env"
		}
		return ExchangeGCP(
			profile.WorkloadIdentityProvider,
			profile.ServiceAccountEmail,
			result.Token,
			format,
			"", "", // production endpoints
		)
	case ProviderAzure:
		return ExchangeAzure(
			profile.TenantID,
			profile.AzureClientID,
			profile.SubscriptionID,
			result.Token,
			"", // production endpoint
		)
	default:
		return nil, fmt.Errorf("unsupported provider %q", profile.Provider)
	}
}

// NeedsRefresh returns true if any credential is within the refresh window of expiry.
// Used by watch mode to trigger re-exchange between cycles.
func NeedsRefresh(ctx *AuthContext) bool {
	if ctx == nil {
		return false
	}
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	for _, creds := range ctx.profiles {
		if creds.Expired(watchRefreshWindow) {
			return true
		}
	}
	return false
}

// RefreshAll re-exchanges all profiles. Used by watch mode.
func RefreshAll(authCfg schema.AuthConfig, ctx *AuthContext) error {
	if ctx == nil {
		return nil
	}
	newCtx, err := ExchangeAll(authCfg)
	if err != nil {
		return err
	}
	// Swap credentials
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	// Zero old credentials
	for _, c := range ctx.profiles {
		c.Zero()
	}
	ctx.profiles = newCtx.profiles
	ctx.active = newCtx.active
	return nil
}
