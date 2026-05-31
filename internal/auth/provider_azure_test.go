package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeAzure_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("client_id"); got != "test-client" {
			t.Errorf("client_id = %q, want test-client", got)
		}
		if got := r.FormValue("grant_type"); got != "client_credentials" {
			t.Errorf("grant_type = %q, want client_credentials", got)
		}
		if got := r.FormValue("client_assertion"); got != "oidc-jwt-token" {
			t.Errorf("client_assertion = %q, want oidc-jwt-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(azureTokenResponse{
			AccessToken: "azure-access-token-123",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	}))
	defer srv.Close()

	creds, err := ExchangeAzure("test-tenant", "test-client", "test-sub", "oidc-jwt-token", srv.URL)
	if err != nil {
		t.Fatalf("ExchangeAzure: %v", err)
	}
	if creds.Provider != ProviderAzure {
		t.Errorf("provider = %q, want azure", creds.Provider)
	}
	if string(creds.AccessToken) != "azure-access-token-123" {
		t.Errorf("access_token = %q, want azure-access-token-123", string(creds.AccessToken))
	}
	if creds.TenantID != "test-tenant" {
		t.Errorf("tenant_id = %q, want test-tenant", creds.TenantID)
	}
	if creds.ClientID != "test-client" {
		t.Errorf("client_id = %q, want test-client", creds.ClientID)
	}
	if creds.SubscriptionID != "test-sub" {
		t.Errorf("subscription_id = %q, want test-sub", creds.SubscriptionID)
	}
	if creds.Expiration.IsZero() {
		t.Error("expiration should be set")
	}
}

func TestExchangeAzure_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(azureErrorResponse{
			Error:       "invalid_client",
			Description: "Client assertion validation failed",
		})
	}))
	defer srv.Close()

	_, err := ExchangeAzure("test-tenant", "test-client", "", "bad-token", srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !contains(got, "invalid_client") {
		t.Errorf("error = %q, want to contain invalid_client", got)
	}
}

func TestExchangeAzure_NoSubscription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(azureTokenResponse{
			AccessToken: "token-no-sub",
			ExpiresIn:   3600,
		})
	}))
	defer srv.Close()

	creds, err := ExchangeAzure("tenant", "client", "", "jwt", srv.URL)
	if err != nil {
		t.Fatalf("ExchangeAzure: %v", err)
	}
	envVars := creds.EnvVars()
	if _, ok := envVars["AZURE_SUBSCRIPTION_ID"]; ok {
		t.Error("AZURE_SUBSCRIPTION_ID should not be set when empty")
	}
	if envVars["AZURE_CLIENT_ID"] != "client" {
		t.Errorf("AZURE_CLIENT_ID = %q, want client", envVars["AZURE_CLIENT_ID"])
	}
	if envVars["AZURE_TENANT_ID"] != "tenant" {
		t.Errorf("AZURE_TENANT_ID = %q, want tenant", envVars["AZURE_TENANT_ID"])
	}
}

func TestExchangeAzure_EmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(azureTokenResponse{ExpiresIn: 3600})
	}))
	defer srv.Close()

	_, err := ExchangeAzure("tenant", "client", "", "jwt", srv.URL)
	if err == nil {
		t.Fatal("expected error for empty access_token")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && containsStr(s, substr)))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
