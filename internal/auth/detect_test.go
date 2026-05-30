package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDetectCI_GitHubActions(t *testing.T) {
	// Set up mock HTTP server for token endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "bearer test-bearer-token" {
			http.Error(w, "unauthorized", 403)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"value": "gh-oidc-token"})
	}))
	defer srv.Close()

	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", srv.URL+"?param=1")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-bearer-token")
	// Clear other CI env vars
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	result, err := DetectCI("", "")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}
	if result.Provider != CIGitHubActions {
		t.Errorf("Provider = %q, want %q", result.Provider, CIGitHubActions)
	}
	if result.Token != "gh-oidc-token" {
		t.Errorf("Token = %q, want %q", result.Token, "gh-oidc-token")
	}
}

func TestDetectCI_GitHubActions_WithAudience(t *testing.T) {
	var receivedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		json.NewEncoder(w).Encode(map[string]string{"value": "gh-token"})
	}))
	defer srv.Close()

	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", srv.URL+"?param=1")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "bearer-tok")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	_, err := DetectCI("", "sts.amazonaws.com")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}
	if receivedURL != "/?param=1&audience=sts.amazonaws.com" {
		t.Errorf("received URL = %q, want audience appended", receivedURL)
	}
}

func TestDetectCI_GitHubActions_MissingBearerToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"value": "token"})
	}))
	defer srv.Close()

	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", srv.URL)
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "") // missing
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	_, err := DetectCI("", "")
	if err == nil {
		t.Fatal("expected error for missing bearer token")
	}
}

func TestDetectCI_GitLabCI(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "gitlab-jwt-token")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	result, err := DetectCI("", "")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}
	if result.Provider != CIGitLabCI {
		t.Errorf("Provider = %q, want %q", result.Provider, CIGitLabCI)
	}
	if result.Token != "gitlab-jwt-token" {
		t.Errorf("Token = %q, want %q", result.Token, "gitlab-jwt-token")
	}
}

func TestDetectCI_CircleCI(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "circleci-oidc-token")

	result, err := DetectCI("", "")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}
	if result.Provider != CICircleCI {
		t.Errorf("Provider = %q, want %q", result.Provider, CICircleCI)
	}
	if result.Token != "circleci-oidc-token" {
		t.Errorf("Token = %q, want %q", result.Token, "circleci-oidc-token")
	}
}

func TestDetectCI_Custom(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")
	t.Setenv("MY_CUSTOM_TOKEN", "custom-jwt-value")

	result, err := DetectCI("MY_CUSTOM_TOKEN", "")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}
	if result.Provider != CICustom {
		t.Errorf("Provider = %q, want %q", result.Provider, CICustom)
	}
	if result.Token != "custom-jwt-value" {
		t.Errorf("Token = %q, want %q", result.Token, "custom-jwt-value")
	}
}

func TestDetectCI_Custom_Empty(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")
	t.Setenv("MY_CUSTOM_TOKEN", "")

	_, err := DetectCI("MY_CUSTOM_TOKEN", "")
	if err == nil {
		t.Fatal("expected error for empty custom token")
	}
}

func TestDetectCI_NoneDetected(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("CI_JOB_JWT_V2", "")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "")

	_, err := DetectCI("", "")
	if err == nil {
		t.Fatal("expected error when no CI detected")
	}
}

func TestDetectCI_Priority(t *testing.T) {
	// GitHub Actions takes priority over GitLab CI
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"value": "gh-token"})
	}))
	defer srv.Close()

	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", srv.URL)
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "bearer")
	t.Setenv("CI_JOB_JWT_V2", "gitlab-token")
	t.Setenv("CIRCLE_OIDC_TOKEN_V2", "circle-token")

	result, err := DetectCI("", "")
	if err != nil {
		t.Fatalf("DetectCI() error = %v", err)
	}
	if result.Provider != CIGitHubActions {
		t.Errorf("Provider = %q, want %q (GitHub should win)", result.Provider, CIGitHubActions)
	}
}

// buildTestJWT creates a minimal JWT with the given exp claim for testing.
func buildTestJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"exp":%d}`, exp),
	))
	return header + "." + payload + ".sig"
}

func TestValidateTokenExp_Valid(t *testing.T) {
	token := buildTestJWT(time.Now().Add(1 * time.Hour).Unix())
	err := ValidateTokenExp(token, 30*time.Second)
	if err != nil {
		t.Errorf("ValidateTokenExp() unexpected error: %v", err)
	}
}

func TestValidateTokenExp_Expired(t *testing.T) {
	token := buildTestJWT(time.Now().Add(-1 * time.Hour).Unix())
	err := ValidateTokenExp(token, 30*time.Second)
	if err == nil {
		t.Error("ValidateTokenExp() should return error for expired token")
	}
}

func TestValidateTokenExp_WithinSkew(t *testing.T) {
	token := buildTestJWT(time.Now().Add(10 * time.Second).Unix())
	err := ValidateTokenExp(token, 30*time.Second)
	if err == nil {
		t.Error("ValidateTokenExp() should return error for token within skew window")
	}
}

func TestValidateTokenExp_NotJWT(t *testing.T) {
	err := ValidateTokenExp("not-a-jwt", 30*time.Second)
	if err != nil {
		t.Errorf("ValidateTokenExp() should pass through non-JWT: %v", err)
	}
}

func TestValidateTokenExp_NoExpClaim(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test"}`))
	token := header + "." + payload + ".sig"

	err := ValidateTokenExp(token, 30*time.Second)
	if err != nil {
		t.Errorf("ValidateTokenExp() should pass when no exp claim: %v", err)
	}
}
