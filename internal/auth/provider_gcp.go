package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// gcpSTSResponse is returned by sts.googleapis.com/v1/token.
type gcpSTSResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// gcpIAMResponse is returned by iamcredentials generateAccessToken.
type gcpIAMResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  string `json:"expireTime"`
}

// ExchangeGCP performs the two-step GCP Workload Identity Federation exchange.
// stsEndpoint and iamEndpoint are overridable for testing (empty = production).
// credentialFormat: "env" (default, sets CLOUDSDK_AUTH_ACCESS_TOKEN) or "keyfile" (writes temp JSON).
func ExchangeGCP(
	workloadIdentityProvider, serviceAccountEmail, oidcToken string,
	credentialFormat string,
	stsEndpoint, iamEndpoint string,
) (*Credentials, error) {
	if stsEndpoint == "" {
		stsEndpoint = "https://sts.googleapis.com/v1/token"
	}

	// Step 1: Exchange OIDC token for federated access token
	stsBody := map[string]string{
		"grantType":          "urn:ietf:params:oauth:grant-type:token-exchange",
		"audience":           "//iam.googleapis.com/" + workloadIdentityProvider,
		"scope":              "https://www.googleapis.com/auth/cloud-platform",
		"requestedTokenType": "urn:ietf:params:oauth:token-type:access_token",
		"subjectTokenType":   "urn:ietf:params:oauth:token-type:jwt",
		"subjectToken":       oidcToken,
	}
	stsJSON, _ := json.Marshal(stsBody)

	stsResp, err := http.Post(stsEndpoint, "application/json", bytes.NewReader(stsJSON))
	if err != nil {
		return nil, fmt.Errorf("GCP STS request failed: %w", err)
	}
	defer stsResp.Body.Close()

	stsRespBody, _ := io.ReadAll(stsResp.Body)
	if stsResp.StatusCode != 200 {
		return nil, fmt.Errorf("GCP STS error (HTTP %d): %s (token: ***redacted***)",
			stsResp.StatusCode, string(stsRespBody))
	}

	var stsResult gcpSTSResponse
	if err := json.Unmarshal(stsRespBody, &stsResult); err != nil {
		return nil, fmt.Errorf("parsing GCP STS response: %w", err)
	}

	// Step 2: Exchange federated token for service account access token
	if iamEndpoint == "" {
		iamEndpoint = fmt.Sprintf(
			"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
			serviceAccountEmail,
		)
	}

	iamBody := map[string]interface{}{
		"scope":    []string{"https://www.googleapis.com/auth/cloud-platform"},
		"lifetime": "3600s",
	}
	iamJSON, _ := json.Marshal(iamBody)

	iamReq, _ := http.NewRequest("POST", iamEndpoint, bytes.NewReader(iamJSON))
	iamReq.Header.Set("Authorization", "Bearer "+stsResult.AccessToken)
	iamReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	iamResp, err := client.Do(iamReq)
	if err != nil {
		return nil, fmt.Errorf("GCP IAM request failed: %w", err)
	}
	defer iamResp.Body.Close()

	iamRespBody, _ := io.ReadAll(iamResp.Body)
	if iamResp.StatusCode != 200 {
		return nil, fmt.Errorf("GCP IAM error (HTTP %d): %s (token: ***redacted***)",
			iamResp.StatusCode, string(iamRespBody))
	}

	var iamResult gcpIAMResponse
	if err := json.Unmarshal(iamRespBody, &iamResult); err != nil {
		return nil, fmt.Errorf("parsing GCP IAM response: %w", err)
	}

	expiration, _ := time.Parse(time.RFC3339, iamResult.ExpireTime)

	creds := &Credentials{
		Provider:    ProviderGCP,
		AccessToken: []byte(iamResult.AccessToken),
		Expiration:  expiration,
	}

	// If keyfile format requested, write temp credentials file
	if credentialFormat == "keyfile" {
		keyfilePath, cleanup, err := writeGCPKeyfile(iamResult.AccessToken, expiration)
		if err != nil {
			return nil, fmt.Errorf("writing GCP temp keyfile: %w", err)
		}
		creds.KeyfilePath = keyfilePath
		// Register SIGTERM handler for cleanup.
		// NOTE: The goroutine blocks until a signal is received. This is acceptable
		// because there is at most one keyfile per GCP profile per suite run, and
		// the goroutine exits on process termination. The AuthContext.CleanupKeyfiles()
		// method handles normal teardown; this handler covers abnormal termination.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			<-sigCh
			cleanup()
			signal.Stop(sigCh) // Unregister to avoid stacking handlers on refresh
		}()
	}

	return creds, nil
}

// writeGCPKeyfile writes a temporary GCP credentials JSON file.
// Returns the file path and a cleanup function.
func writeGCPKeyfile(accessToken string, expiration time.Time) (string, func(), error) {
	tmpDir := os.TempDir()
	keyfilePath := filepath.Join(tmpDir, fmt.Sprintf("smokesig-gcp-%d.json", time.Now().UnixNano()))

	keyfileContent := map[string]interface{}{
		"type":         "external_account",
		"access_token": accessToken,
		"expiry":       expiration.Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(keyfileContent, "", "  ")

	if err := os.WriteFile(keyfilePath, data, 0600); err != nil {
		return "", nil, err
	}

	cleanup := func() {
		os.Remove(keyfilePath)
	}

	return keyfilePath, cleanup, nil
}
