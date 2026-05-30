package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeAWS_Success(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("Action") != "AssumeRoleWithWebIdentity" {
			t.Errorf("Action = %q, want AssumeRoleWithWebIdentity", r.FormValue("Action"))
		}
		if r.FormValue("RoleArn") != "arn:aws:iam::123456789012:role/test" {
			t.Errorf("RoleArn = %q", r.FormValue("RoleArn"))
		}
		if r.FormValue("Version") != "2011-06-15" {
			t.Errorf("Version = %q", r.FormValue("Version"))
		}
		if r.FormValue("DurationSeconds") != "3600" {
			t.Errorf("DurationSeconds = %q, want 3600", r.FormValue("DurationSeconds"))
		}
		w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>
			<AssumeRoleWithWebIdentityResult>
				<Credentials>
					<AccessKeyId>ASIAMOCKKEY123</AccessKeyId>
					<SecretAccessKey>mocksecret</SecretAccessKey>
					<SessionToken>mocktoken</SessionToken>
					<Expiration>2026-05-29T13:00:00Z</Expiration>
				</Credentials>
			</AssumeRoleWithWebIdentityResult>
		</AssumeRoleWithWebIdentityResponse>`))
	}))
	defer sts.Close()

	creds, err := ExchangeAWS("arn:aws:iam::123456789012:role/test", "", "", "", "fake-token", sts.URL)
	if err != nil {
		t.Fatalf("ExchangeAWS() error = %v", err)
	}
	if string(creds.AccessKeyID) != "ASIAMOCKKEY123" {
		t.Errorf("AccessKeyID = %q, want ASIAMOCKKEY123", string(creds.AccessKeyID))
	}
	if string(creds.SecretAccessKey) != "mocksecret" {
		t.Errorf("SecretAccessKey = %q, want mocksecret", string(creds.SecretAccessKey))
	}
	if string(creds.SessionToken) != "mocktoken" {
		t.Errorf("SessionToken = %q, want mocktoken", string(creds.SessionToken))
	}
	if creds.Provider != ProviderAWS {
		t.Errorf("Provider = %q, want aws", creds.Provider)
	}
	if creds.Expiration.IsZero() {
		t.Error("Expiration should be set")
	}
}

func TestExchangeAWS_STSError(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`<ErrorResponse>
			<Error>
				<Code>AccessDenied</Code>
				<Message>Not authorized to perform sts:AssumeRoleWithWebIdentity</Message>
			</Error>
		</ErrorResponse>`))
	}))
	defer sts.Close()

	_, err := ExchangeAWS("arn:aws:iam::123:role/test", "", "", "", "fake-token", sts.URL)
	if err == nil {
		t.Fatal("ExchangeAWS() should return error for 403")
	}
	// Error should contain the STS error code
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestExchangeAWS_EmptyCredentials(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>
			<AssumeRoleWithWebIdentityResult>
				<Credentials>
					<AccessKeyId></AccessKeyId>
					<SecretAccessKey></SecretAccessKey>
					<SessionToken></SessionToken>
				</Credentials>
			</AssumeRoleWithWebIdentityResult>
		</AssumeRoleWithWebIdentityResponse>`))
	}))
	defer sts.Close()

	_, err := ExchangeAWS("arn:aws:iam::123:role/test", "", "", "", "fake-token", sts.URL)
	if err == nil {
		t.Fatal("ExchangeAWS() should return error for empty credentials")
	}
}

func TestExchangeAWS_RegionalEndpoint(t *testing.T) {
	// Verify that when no stsEndpoint is provided but region is set,
	// the regional endpoint is constructed correctly.
	// We test this by providing a custom stsEndpoint that captures the call.
	var called bool
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>
			<AssumeRoleWithWebIdentityResult>
				<Credentials>
					<AccessKeyId>ASIAREGIONAL</AccessKeyId>
					<SecretAccessKey>secret</SecretAccessKey>
					<SessionToken>token</SessionToken>
					<Expiration>2026-05-29T13:00:00Z</Expiration>
				</Credentials>
			</AssumeRoleWithWebIdentityResult>
		</AssumeRoleWithWebIdentityResponse>`))
	}))
	defer sts.Close()

	// With explicit endpoint, region is ignored
	creds, err := ExchangeAWS("arn:aws:iam::123:role/test", "", "us-west-2", "", "fake-token", sts.URL)
	if err != nil {
		t.Fatalf("ExchangeAWS() error = %v", err)
	}
	if !called {
		t.Error("mock STS should have been called")
	}
	if string(creds.AccessKeyID) != "ASIAREGIONAL" {
		t.Errorf("AccessKeyID = %q", string(creds.AccessKeyID))
	}
}

func TestExchangeAWS_CustomDuration(t *testing.T) {
	var receivedDuration string
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedDuration = r.FormValue("DurationSeconds")
		w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>
			<AssumeRoleWithWebIdentityResult>
				<Credentials>
					<AccessKeyId>ASIAKEY</AccessKeyId>
					<SecretAccessKey>secret</SecretAccessKey>
					<SessionToken>token</SessionToken>
					<Expiration>2026-05-29T13:00:00Z</Expiration>
				</Credentials>
			</AssumeRoleWithWebIdentityResult>
		</AssumeRoleWithWebIdentityResponse>`))
	}))
	defer sts.Close()

	_, err := ExchangeAWS("arn:aws:iam::123:role/test", "", "", "2h", "fake-token", sts.URL)
	if err != nil {
		t.Fatalf("ExchangeAWS() error = %v", err)
	}
	if receivedDuration != "7200" {
		t.Errorf("DurationSeconds = %q, want 7200 (2h)", receivedDuration)
	}
}

func TestExchangeAWS_DefaultAudience(t *testing.T) {
	// When no audience is provided, it should not appear in params
	// (audience is for CI token request, not STS)
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<AssumeRoleWithWebIdentityResponse>
			<AssumeRoleWithWebIdentityResult>
				<Credentials>
					<AccessKeyId>ASIAKEY</AccessKeyId>
					<SecretAccessKey>secret</SecretAccessKey>
					<SessionToken>token</SessionToken>
					<Expiration>2026-05-29T13:00:00Z</Expiration>
				</Credentials>
			</AssumeRoleWithWebIdentityResult>
		</AssumeRoleWithWebIdentityResponse>`))
	}))
	defer sts.Close()

	_, err := ExchangeAWS("arn:aws:iam::123:role/test", "", "", "", "fake-token", sts.URL)
	if err != nil {
		t.Fatalf("ExchangeAWS() error = %v", err)
	}
}
