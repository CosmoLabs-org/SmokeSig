package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// CIProvider identifies a CI system.
type CIProvider string

const (
	CIGitHubActions CIProvider = "github-actions"
	CIGitLabCI      CIProvider = "gitlab-ci"
	CICircleCI      CIProvider = "circleci"
	CICustom        CIProvider = "custom"
)

// DetectResult holds the detected CI environment and token acquisition method.
type DetectResult struct {
	Provider CIProvider
	Token    string
}

// DetectCI identifies the CI provider and acquires an OIDC token.
// Detection order: GitHub Actions > GitLab CI > CircleCI > custom token_env.
// Returns error if no CI environment is detected and no token_env fallback is configured.
//
// CI provider API versions (maintenance surface):
//   - GitHub Actions: OIDC token endpoint v1 (ACTIONS_ID_TOKEN_REQUEST_URL)
//   - GitLab CI: CI_JOB_JWT_V2 (JWT v2 format, introduced GitLab 15.7)
//   - CircleCI: CIRCLE_OIDC_TOKEN_V2 (OIDC v2 format)
func DetectCI(tokenEnv string, audience string) (*DetectResult, error) {
	// GitHub Actions: HTTP request to token endpoint
	if reqURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"); reqURL != "" {
		token, err := fetchGitHubToken(reqURL, audience)
		if err != nil {
			return nil, fmt.Errorf("github actions OIDC token fetch failed: %w", err)
		}
		return &DetectResult{Provider: CIGitHubActions, Token: token}, nil
	}

	// GitLab CI: token is directly in env var
	if token := os.Getenv("CI_JOB_JWT_V2"); token != "" {
		return &DetectResult{Provider: CIGitLabCI, Token: token}, nil
	}

	// CircleCI: token is directly in env var
	if token := os.Getenv("CIRCLE_OIDC_TOKEN_V2"); token != "" {
		return &DetectResult{Provider: CICircleCI, Token: token}, nil
	}

	// Custom: user-specified env var
	if tokenEnv != "" {
		if token := os.Getenv(tokenEnv); token != "" {
			return &DetectResult{Provider: CICustom, Token: token}, nil
		}
		return nil, fmt.Errorf("token_env %q is set in config but the environment variable is empty or unset", tokenEnv)
	}

	return nil, fmt.Errorf("no CI OIDC environment detected (checked: ACTIONS_ID_TOKEN_REQUEST_URL, CI_JOB_JWT_V2, CIRCLE_OIDC_TOKEN_V2); set auth.profiles[].token_env to use a custom token source")
}

// fetchGitHubToken requests an OIDC token from GitHub Actions.
func fetchGitHubToken(reqURL, audience string) (string, error) {
	bearerToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if bearerToken == "" {
		return "", fmt.Errorf("ACTIONS_ID_TOKEN_REQUEST_TOKEN not set (missing id-token: write permission?)")
	}

	if audience != "" {
		reqURL = reqURL + "&audience=" + audience
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "bearer "+bearerToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response JSON: %w", err)
	}

	if result.Value == "" {
		return "", fmt.Errorf("empty token in response")
	}
	return result.Value, nil
}

// ValidateTokenExp checks the OIDC JWT's exp claim locally.
// Returns error if token is expired or within skew of expiry.
// This is a fast-fail check before making the STS network call.
func ValidateTokenExp(token string, skew time.Duration) error {
	// JWT is three base64url-encoded segments separated by dots
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		// Not a valid JWT structure; let the STS endpoint reject it
		return nil
	}

	// Decode the payload (second segment)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Can't decode; let STS handle validation
		return nil
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}

	if claims.Exp == 0 {
		return nil // No exp claim; proceed
	}

	expTime := time.Unix(claims.Exp, 0)
	if time.Now().Add(skew).After(expTime) {
		return fmt.Errorf("OIDC token expired or expiring within %v (exp: %s, now: %s)",
			skew, expTime.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	}

	return nil
}
