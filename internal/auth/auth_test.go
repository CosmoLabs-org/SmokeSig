package auth

import (
	"testing"
	"time"
)

func TestCredentials_Zero(t *testing.T) {
	creds := &Credentials{
		AccessKeyID:    []byte("ASIAMOCKKEY"),
		SecretAccessKey: []byte("secretvalue"),
		SessionToken:   []byte("sessiontoken"),
		AccessToken:    []byte("ya29.accesstoken"),
	}
	creds.Zero()

	for i, b := range creds.AccessKeyID {
		if b != 0 {
			t.Errorf("AccessKeyID[%d] = %d, want 0", i, b)
		}
	}
	for i, b := range creds.SecretAccessKey {
		if b != 0 {
			t.Errorf("SecretAccessKey[%d] = %d, want 0", i, b)
		}
	}
	for i, b := range creds.SessionToken {
		if b != 0 {
			t.Errorf("SessionToken[%d] = %d, want 0", i, b)
		}
	}
	for i, b := range creds.AccessToken {
		if b != 0 {
			t.Errorf("AccessToken[%d] = %d, want 0", i, b)
		}
	}
}

func TestCredentials_Zero_Empty(t *testing.T) {
	// Zero on empty credentials should not panic
	creds := &Credentials{}
	creds.Zero()
}

func TestCredentials_String_AWS(t *testing.T) {
	creds := &Credentials{Provider: ProviderAWS, AccessKeyID: []byte("ASIAMOCKKEY123")}
	got := creds.String()
	want := "aws:ASIA***"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCredentials_String_AWS_Short(t *testing.T) {
	creds := &Credentials{Provider: ProviderAWS, AccessKeyID: []byte("AK")}
	got := creds.String()
	want := "aws:AK"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCredentials_String_GCP(t *testing.T) {
	creds := &Credentials{Provider: ProviderGCP, AccessToken: []byte("ya29.longaccesstoken")}
	got := creds.String()
	want := "gcp:ya29***"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCredentials_String_Azure(t *testing.T) {
	creds := &Credentials{Provider: ProviderAzure, AccessToken: []byte("eyJhbGciOiJSUzI1NiJ9")}
	got := creds.String()
	if got != "azure:eyJh***" {
		t.Errorf("String() = %q, want azure:eyJh***", got)
	}
}

func TestCredentials_String_Unknown(t *testing.T) {
	creds := &Credentials{Provider: "digitalocean"}
	got := creds.String()
	want := "unknown"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCredentials_EnvVars_AWS(t *testing.T) {
	creds := &Credentials{
		Provider:       ProviderAWS,
		AccessKeyID:    []byte("AKID"),
		SecretAccessKey: []byte("SECRET"),
		SessionToken:   []byte("TOKEN"),
	}
	vars := creds.EnvVars()
	if vars["AWS_ACCESS_KEY_ID"] != "AKID" {
		t.Errorf("AWS_ACCESS_KEY_ID = %q, want AKID", vars["AWS_ACCESS_KEY_ID"])
	}
	if vars["AWS_SECRET_ACCESS_KEY"] != "SECRET" {
		t.Errorf("AWS_SECRET_ACCESS_KEY = %q, want SECRET", vars["AWS_SECRET_ACCESS_KEY"])
	}
	if vars["AWS_SESSION_TOKEN"] != "TOKEN" {
		t.Errorf("AWS_SESSION_TOKEN = %q, want TOKEN", vars["AWS_SESSION_TOKEN"])
	}
}

func TestCredentials_EnvVars_GCP_Default(t *testing.T) {
	creds := &Credentials{
		Provider:    ProviderGCP,
		AccessToken: []byte("ya29.token"),
	}
	vars := creds.EnvVars()
	if vars["CLOUDSDK_AUTH_ACCESS_TOKEN"] != "ya29.token" {
		t.Errorf("CLOUDSDK_AUTH_ACCESS_TOKEN = %q, want ya29.token", vars["CLOUDSDK_AUTH_ACCESS_TOKEN"])
	}
	if _, ok := vars["GOOGLE_APPLICATION_CREDENTIALS"]; ok {
		t.Error("GOOGLE_APPLICATION_CREDENTIALS should not be set without keyfile")
	}
}

func TestCredentials_EnvVars_GCP_Keyfile(t *testing.T) {
	creds := &Credentials{
		Provider:    ProviderGCP,
		AccessToken: []byte("ya29.token"),
		KeyfilePath: "/tmp/keyfile.json",
	}
	vars := creds.EnvVars()
	if vars["CLOUDSDK_AUTH_ACCESS_TOKEN"] != "ya29.token" {
		t.Errorf("CLOUDSDK_AUTH_ACCESS_TOKEN = %q, want ya29.token", vars["CLOUDSDK_AUTH_ACCESS_TOKEN"])
	}
	if vars["GOOGLE_APPLICATION_CREDENTIALS"] != "/tmp/keyfile.json" {
		t.Errorf("GOOGLE_APPLICATION_CREDENTIALS = %q, want /tmp/keyfile.json", vars["GOOGLE_APPLICATION_CREDENTIALS"])
	}
}

func TestCredentials_Expired(t *testing.T) {
	tests := []struct {
		name       string
		expiration time.Time
		skew       time.Duration
		want       bool
	}{
		{
			name:       "zero expiration is not expired",
			expiration: time.Time{},
			skew:       time.Minute,
			want:       false,
		},
		{
			name:       "future expiration is not expired",
			expiration: time.Now().Add(1 * time.Hour),
			skew:       30 * time.Second,
			want:       false,
		},
		{
			name:       "past expiration is expired",
			expiration: time.Now().Add(-1 * time.Hour),
			skew:       30 * time.Second,
			want:       true,
		},
		{
			name:       "within skew window is expired",
			expiration: time.Now().Add(10 * time.Second),
			skew:       30 * time.Second,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &Credentials{Expiration: tt.expiration}
			got := creds.Expired(tt.skew)
			if got != tt.want {
				t.Errorf("Expired(%v) = %v, want %v", tt.skew, got, tt.want)
			}
		})
	}
}

func TestAuthContext_ProfileManagement(t *testing.T) {
	ctx := NewAuthContext()

	// Initially empty
	if got := ctx.Get("default"); got != nil {
		t.Error("Get(default) should return nil for empty context")
	}
	if got := ctx.Active(); got != nil {
		t.Error("Active() should return nil for empty context")
	}

	// Set a profile
	awsCreds := &Credentials{Provider: ProviderAWS, AccessKeyID: []byte("AKID")}
	ctx.Set("default", awsCreds)

	if got := ctx.Get("default"); got != awsCreds {
		t.Error("Get(default) should return the stored credentials")
	}
	if got := ctx.Active(); got != awsCreds {
		t.Error("Active() should return default profile")
	}

	// Set another profile and make it active
	gcpCreds := &Credentials{Provider: ProviderGCP, AccessToken: []byte("ya29.token")}
	ctx.Set("staging", gcpCreds)
	ctx.SetActive("staging")

	if got := ctx.Active(); got != gcpCreds {
		t.Error("Active() should return staging profile after SetActive")
	}
	if got := ctx.Get("default"); got != awsCreds {
		t.Error("Get(default) should still return aws creds")
	}
}

func TestAuthContext_ZeroAll(t *testing.T) {
	ctx := NewAuthContext()
	creds := &Credentials{
		Provider:    ProviderAWS,
		AccessKeyID: []byte("AKID1234"),
	}
	ctx.Set("default", creds)
	ctx.ZeroAll()

	for i, b := range creds.AccessKeyID {
		if b != 0 {
			t.Errorf("After ZeroAll, AccessKeyID[%d] = %d, want 0", i, b)
		}
	}
}

func TestMaskCredentials(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "AWS temporary key",
			input: `error: AWS returned ASIAMOCKKEY123456 for role`,
			want:  `error: AWS returned ***redacted*** for role`,
		},
		{
			name:  "AWS long-lived key",
			input: `key=AKIAIOSFODNN7EXAMPLE secret=abc`,
			want:  `key=***redacted*** secret=abc`,
		},
		{
			name:  "GCP access token",
			input: `token: ya29.a0AfH6SMBx_long_token_here end`,
			want:  `token: ***redacted*** end`,
		},
		{
			name:  "JWT token",
			input: `bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig in header`,
			want:  `bearer ***redacted*** in header`,
		},
		{
			name:  "no credentials",
			input: `normal error message with no secrets`,
			want:  `normal error message with no secrets`,
		},
		{
			name:  "multiple credentials",
			input: `AKIAKEY1 and ASIAKEY2 found`,
			want:  `***redacted*** and ***redacted*** found`,
		},
		{
			name:  "credential at end of string",
			input: `key=ASIAMOCKKEY`,
			want:  `key=***redacted***`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskCredentials(tt.input)
			if got != tt.want {
				t.Errorf("MaskCredentials(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
