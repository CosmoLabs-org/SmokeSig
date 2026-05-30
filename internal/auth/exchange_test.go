package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestExchangeAll_NoProfiles(t *testing.T) {
	ctx, err := ExchangeAll(schema.AuthConfig{})
	if err != nil {
		t.Fatalf("ExchangeAll() error = %v", err)
	}
	if ctx != nil {
		t.Error("ExchangeAll() should return nil for empty config")
	}
}

func TestExchangeAll_FallbackEnv(t *testing.T) {
	// No CI environment set, fallback=env should succeed (no error)
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	cfg := schema.AuthConfig{
		Profiles: []schema.AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123:role/test"},
		},
		Fallback: "env",
	}

	ctx, err := ExchangeAll(cfg)
	if err != nil {
		t.Fatalf("ExchangeAll() with fallback=env should not error: %v", err)
	}
	// Context should be created but with no profiles (exchange failed, fell back)
	if ctx == nil {
		t.Fatal("ExchangeAll() should return non-nil context")
	}
}

func TestExchangeAll_FallbackFail(t *testing.T) {
	// No CI environment set, fallback=fail should return error
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	cfg := schema.AuthConfig{
		Profiles: []schema.AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123:role/test"},
		},
		Fallback: "fail",
	}

	_, err := ExchangeAll(cfg)
	if err == nil {
		t.Fatal("ExchangeAll() with fallback=fail should error when no CI detected")
	}
}

func TestExchangeAll_DefaultFallbackIsFail(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	cfg := schema.AuthConfig{
		Profiles: []schema.AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123:role/test"},
		},
		// No Fallback set — default is "fail"
	}

	_, err := ExchangeAll(cfg)
	if err == nil {
		t.Fatal("ExchangeAll() should error by default when no CI detected")
	}
}

func TestFullExchangeFlow_AWS(t *testing.T) {
	// Mock STS server
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>
			<AssumeRoleWithWebIdentityResult>
				<Credentials>
					<AccessKeyId>ASIAFULLFLOW</AccessKeyId>
					<SecretAccessKey>fullsecret</SecretAccessKey>
					<SessionToken>fulltoken</SessionToken>
					<Expiration>2026-05-29T15:00:00Z</Expiration>
				</Credentials>
			</AssumeRoleWithWebIdentityResult>
		</AssumeRoleWithWebIdentityResponse>`))
	}))
	defer sts.Close()

	// Mock GitHub Actions token endpoint
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"value": "github-oidc-jwt"})
	}))
	defer tokenSrv.Close()

	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", tokenSrv.URL)
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "bearer-test")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	// We can't use ExchangeAll directly because it uses production STS endpoints.
	// Instead, test the components individually and verify integration via DetectCI + ExchangeAWS.

	// Step 1: Detect CI
	result, err := DetectCI("", "")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}
	if result.Provider != CIGitHubActions {
		t.Errorf("Provider = %q, want github-actions", result.Provider)
	}

	// Step 2: Exchange with mock STS
	creds, err := ExchangeAWS("arn:aws:iam::123:role/test", "", "", "", result.Token, sts.URL)
	if err != nil {
		t.Fatalf("ExchangeAWS() error = %v", err)
	}

	// Step 3: Build AuthContext
	ctx := NewAuthContext()
	creds.ProfileName = "default"
	ctx.Set("default", creds)
	ctx.SetActive("default")

	// Step 4: Verify EnvVars
	vars := creds.EnvVars()
	if vars["AWS_ACCESS_KEY_ID"] != "ASIAFULLFLOW" {
		t.Errorf("AWS_ACCESS_KEY_ID = %q", vars["AWS_ACCESS_KEY_ID"])
	}
	if vars["AWS_SECRET_ACCESS_KEY"] != "fullsecret" {
		t.Errorf("AWS_SECRET_ACCESS_KEY = %q", vars["AWS_SECRET_ACCESS_KEY"])
	}
	if vars["AWS_SESSION_TOKEN"] != "fulltoken" {
		t.Errorf("AWS_SESSION_TOKEN = %q", vars["AWS_SESSION_TOKEN"])
	}

	// Step 5: Verify ZeroAll
	ctx.ZeroAll()
	for i, b := range creds.AccessKeyID {
		if b != 0 {
			t.Errorf("After ZeroAll, AccessKeyID[%d] = %d", i, b)
		}
	}
}

func TestFullExchangeFlow_GCP(t *testing.T) {
	// Mock STS endpoint
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gcpSTSResponse{
			AccessToken: "fed-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer sts.Close()

	// Mock IAM endpoint
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gcpIAMResponse{
			AccessToken: "ya29.gcpfullflow",
			ExpireTime:  "2026-05-29T15:00:00Z",
		})
	}))
	defer iam.Close()

	// Use GitLab CI token
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "gitlab-jwt-for-gcp")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	result, err := DetectCI("", "")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}

	creds, err := ExchangeGCP(
		"projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
		"sa@project.iam.gserviceaccount.com",
		result.Token,
		"env",
		sts.URL,
		iam.URL,
	)
	if err != nil {
		t.Fatalf("ExchangeGCP() error = %v", err)
	}

	vars := creds.EnvVars()
	if vars["CLOUDSDK_AUTH_ACCESS_TOKEN"] != "ya29.gcpfullflow" {
		t.Errorf("CLOUDSDK_AUTH_ACCESS_TOKEN = %q", vars["CLOUDSDK_AUTH_ACCESS_TOKEN"])
	}
}

func TestNeedsRefresh(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		if NeedsRefresh(nil) {
			t.Error("NeedsRefresh(nil) should return false")
		}
	})

	t.Run("fresh credentials", func(t *testing.T) {
		ctx := NewAuthContext()
		ctx.Set("default", &Credentials{
			Provider:   ProviderAWS,
			Expiration: time.Now().Add(1 * time.Hour),
		})
		if NeedsRefresh(ctx) {
			t.Error("NeedsRefresh should return false for fresh credentials")
		}
	})

	t.Run("expiring soon", func(t *testing.T) {
		ctx := NewAuthContext()
		ctx.Set("default", &Credentials{
			Provider:   ProviderAWS,
			Expiration: time.Now().Add(2 * time.Minute), // within 5min window
		})
		if !NeedsRefresh(ctx) {
			t.Error("NeedsRefresh should return true for nearly-expired credentials")
		}
	})
}

func TestExchangeAllCached(t *testing.T) {
	// Reset package-level cache
	cachedMu.Lock()
	cachedCtx = nil
	cachedMu.Unlock()

	// No CI env = will fail
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	cfg := schema.AuthConfig{
		Profiles: []schema.AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123:role/test"},
		},
		Fallback: "fail",
	}

	_, err := ExchangeAllCached(cfg)
	if err == nil {
		t.Fatal("ExchangeAllCached() should error when no CI detected")
	}
}
