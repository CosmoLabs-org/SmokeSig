package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type azureTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type azureErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// ExchangeAzure performs Azure AD federated token exchange.
// loginEndpoint is overridable for testing (empty = production).
func ExchangeAzure(tenantID, clientID, subscriptionID, oidcToken, loginEndpoint string) (*Credentials, error) {
	if loginEndpoint == "" {
		loginEndpoint = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	}

	form := url.Values{
		"client_id":             {clientID},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {oidcToken},
		"grant_type":            {"client_credentials"},
		"scope":                 {"https://management.azure.com/.default"},
	}

	resp, err := http.PostForm(loginEndpoint, form)
	if err != nil {
		return nil, fmt.Errorf("azure token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp azureErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("azure token exchange failed (HTTP %d): %s: %s", resp.StatusCode, errResp.Error, errResp.Description)
		}
		return nil, fmt.Errorf("azure token exchange failed (HTTP %d)", resp.StatusCode)
	}

	var tokenResp azureTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("azure parse response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("azure token exchange returned empty access_token")
	}

	return &Credentials{
		Provider:       ProviderAzure,
		AccessToken:    []byte(tokenResp.AccessToken),
		TenantID:       tenantID,
		ClientID:       clientID,
		SubscriptionID: subscriptionID,
		Expiration:     time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}
