package auth

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// awsSTSResponse is the XML structure returned by AssumeRoleWithWebIdentity.
type awsSTSResponse struct {
	XMLName xml.Name `xml:"AssumeRoleWithWebIdentityResponse"`
	Result  struct {
		Credentials struct {
			AccessKeyID     string `xml:"AccessKeyId"`
			SecretAccessKey string `xml:"SecretAccessKey"`
			SessionToken    string `xml:"SessionToken"`
			Expiration      string `xml:"Expiration"`
		} `xml:"Credentials"`
	} `xml:"AssumeRoleWithWebIdentityResult"`
}

// awsSTSErrorResponse parses STS error responses.
type awsSTSErrorResponse struct {
	XMLName xml.Name `xml:"ErrorResponse"`
	Error   struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	} `xml:"Error"`
}

// ExchangeAWS performs AssumeRoleWithWebIdentity against AWS STS.
// stsEndpoint is overridable for testing (empty = production endpoint).
func ExchangeAWS(roleARN, audience, region, sessionDuration, oidcToken, stsEndpoint string) (*Credentials, error) {
	if stsEndpoint == "" {
		if region != "" {
			stsEndpoint = fmt.Sprintf("https://sts.%s.amazonaws.com/", region)
		} else {
			stsEndpoint = "https://sts.amazonaws.com/"
		}
	}

	if audience == "" {
		audience = "sts.amazonaws.com"
	}

	duration := "3600" // 1 hour default
	if sessionDuration != "" {
		d, err := time.ParseDuration(sessionDuration)
		if err == nil {
			duration = fmt.Sprintf("%d", int(d.Seconds()))
		}
	}

	sessionName := fmt.Sprintf("smokesig-%d", time.Now().Unix())

	params := url.Values{
		"Action":           {"AssumeRoleWithWebIdentity"},
		"Version":          {"2011-06-15"},
		"RoleArn":          {roleARN},
		"RoleSessionName":  {sessionName},
		"WebIdentityToken": {oidcToken},
		"DurationSeconds":  {duration},
	}

	resp, err := http.PostForm(stsEndpoint, params)
	if err != nil {
		return nil, fmt.Errorf("AWS STS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading STS response: %w", err)
	}

	if resp.StatusCode != 200 {
		var stsErr awsSTSErrorResponse
		if xml.Unmarshal(body, &stsErr) == nil && stsErr.Error.Message != "" {
			return nil, fmt.Errorf("AWS STS error (HTTP %d): %s — %s (token: ***redacted***)",
				resp.StatusCode, stsErr.Error.Code, stsErr.Error.Message)
		}
		return nil, fmt.Errorf("AWS STS error (HTTP %d): %s (token: ***redacted***)",
			resp.StatusCode, string(body))
	}

	var stsResp awsSTSResponse
	if err := xml.Unmarshal(body, &stsResp); err != nil {
		return nil, fmt.Errorf("parsing STS XML response: %w", err)
	}

	creds := stsResp.Result.Credentials
	if creds.AccessKeyID == "" {
		return nil, fmt.Errorf("AWS STS returned empty credentials")
	}

	expiration, _ := time.Parse(time.RFC3339, creds.Expiration)

	return &Credentials{
		Provider:       ProviderAWS,
		AccessKeyID:    []byte(creds.AccessKeyID),
		SecretAccessKey: []byte(creds.SecretAccessKey),
		SessionToken:   []byte(creds.SessionToken),
		Expiration:     expiration,
	}, nil
}
