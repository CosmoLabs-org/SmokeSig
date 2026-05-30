package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestExchangeGCP_Success_EnvVar(t *testing.T) {
	// Mock STS endpoint (step 1)
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["grantType"] != "urn:ietf:params:oauth:grant-type:token-exchange" {
			t.Errorf("grantType = %q", body["grantType"])
		}
		if body["subjectToken"] != "fake-oidc-token" {
			t.Errorf("subjectToken = %q", body["subjectToken"])
		}
		json.NewEncoder(w).Encode(gcpSTSResponse{
			AccessToken: "federated-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer sts.Close()

	// Mock IAM endpoint (step 2)
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer federated-token" {
			t.Errorf("Authorization = %q, want Bearer federated-token", authHeader)
		}
		json.NewEncoder(w).Encode(gcpIAMResponse{
			AccessToken: "ya29.final-access-token",
			ExpireTime:  "2026-05-29T14:00:00Z",
		})
	}))
	defer iam.Close()

	creds, err := ExchangeGCP(
		"projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
		"sa@project.iam.gserviceaccount.com",
		"fake-oidc-token",
		"env", // default: env var mode
		sts.URL,
		iam.URL,
	)
	if err != nil {
		t.Fatalf("ExchangeGCP() error = %v", err)
	}
	if string(creds.AccessToken) != "ya29.final-access-token" {
		t.Errorf("AccessToken = %q, want ya29.final-access-token", string(creds.AccessToken))
	}
	if creds.Provider != ProviderGCP {
		t.Errorf("Provider = %q, want gcp", creds.Provider)
	}
	if creds.KeyfilePath != "" {
		t.Errorf("KeyfilePath should be empty in env mode, got %q", creds.KeyfilePath)
	}
	if creds.Expiration.IsZero() {
		t.Error("Expiration should be set")
	}
}

func TestExchangeGCP_Success_Keyfile(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gcpSTSResponse{
			AccessToken: "federated-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer sts.Close()

	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gcpIAMResponse{
			AccessToken: "ya29.keyfile-token",
			ExpireTime:  "2026-05-29T14:00:00Z",
		})
	}))
	defer iam.Close()

	creds, err := ExchangeGCP(
		"projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
		"sa@project.iam.gserviceaccount.com",
		"fake-oidc-token",
		"keyfile",
		sts.URL,
		iam.URL,
	)
	if err != nil {
		t.Fatalf("ExchangeGCP() error = %v", err)
	}
	if creds.KeyfilePath == "" {
		t.Fatal("KeyfilePath should be set in keyfile mode")
	}

	// Verify the keyfile exists and has correct content
	data, err := os.ReadFile(creds.KeyfilePath)
	if err != nil {
		t.Fatalf("reading keyfile: %v", err)
	}
	var keyfile map[string]interface{}
	if err := json.Unmarshal(data, &keyfile); err != nil {
		t.Fatalf("parsing keyfile JSON: %v", err)
	}
	if keyfile["type"] != "external_account" {
		t.Errorf("keyfile type = %q, want external_account", keyfile["type"])
	}
	if keyfile["access_token"] != "ya29.keyfile-token" {
		t.Errorf("keyfile access_token = %q", keyfile["access_token"])
	}

	// Clean up the temp file
	os.Remove(creds.KeyfilePath)
}

func TestExchangeGCP_STSError(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error": "invalid_grant", "error_description": "token expired"}`))
	}))
	defer sts.Close()

	_, err := ExchangeGCP(
		"projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
		"sa@project.iam.gserviceaccount.com",
		"fake-oidc-token",
		"env",
		sts.URL,
		"http://unused",
	)
	if err == nil {
		t.Fatal("ExchangeGCP() should return error for STS failure")
	}
}

func TestExchangeGCP_IAMError(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gcpSTSResponse{
			AccessToken: "federated-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer sts.Close()

	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error": {"code": 403, "message": "Permission denied"}}`))
	}))
	defer iam.Close()

	_, err := ExchangeGCP(
		"projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
		"sa@project.iam.gserviceaccount.com",
		"fake-oidc-token",
		"env",
		sts.URL,
		iam.URL,
	)
	if err == nil {
		t.Fatal("ExchangeGCP() should return error for IAM failure")
	}
}
