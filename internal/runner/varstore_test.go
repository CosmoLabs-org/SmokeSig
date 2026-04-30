package runner

import (
	"strings"
	"testing"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

// --- VarStore tests ---

func TestVarStore_SetAndGet(t *testing.T) {
	v := NewVarStore()
	v.Set("token", "abc123")
	val, ok := v.Get("token")
	if !ok {
		t.Fatal("expected to find token")
	}
	if val != "abc123" {
		t.Errorf("got %q, want %q", val, "abc123")
	}
}

func TestVarStore_GetMissing(t *testing.T) {
	v := NewVarStore()
	_, ok := v.Get("missing")
	if ok {
		t.Error("expected not found")
	}
}

func TestVarStore_Overwrite(t *testing.T) {
	v := NewVarStore()
	v.Set("key", "old")
	v.Set("key", "new")
	val, _ := v.Get("key")
	if val != "new" {
		t.Errorf("got %q, want %q", val, "new")
	}
}

func TestVarStore_ResolveTemplate(t *testing.T) {
	v := NewVarStore()
	v.Set("host", "localhost")
	v.Set("port", "8080")
	resolved, err := v.ResolveTemplate("http://{{ .Vars.host }}:{{ .Vars.port }}/health")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "http://localhost:8080/health" {
		t.Errorf("got %q", resolved)
	}
}

func TestVarStore_ResolveTemplateMixed(t *testing.T) {
	v := NewVarStore()
	v.Set("token", "secret123")
	resolved, err := v.ResolveTemplate("Bearer {{ .Vars.token }}")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "Bearer secret123" {
		t.Errorf("got %q", resolved)
	}
}

func TestVarStore_ResolveTemplateMissing(t *testing.T) {
	v := NewVarStore()
	_, err := v.ResolveTemplate("{{ .Vars.missing }}")
	if err == nil {
		t.Error("expected error for missing variable")
	}
}

func TestVarStore_ResolveTemplateNoVars(t *testing.T) {
	v := NewVarStore()
	resolved, err := v.ResolveTemplate("static text")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "static text" {
		t.Errorf("got %q", resolved)
	}
}

func TestVarStore_IsSecret(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"token", "api_token", true},
		{"key", "ssh_key", true},
		{"secret", "client_secret", true},
		{"password", "db_password", true},
		{"auth", "auth_header", true},
		{"normal", "host", false},
		{"normal", "port", false},
		{"normal", "base_url", false},
	}
	v := NewVarStore()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.IsSecret(tt.key); got != tt.want {
				t.Errorf("IsSecret(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestVarStore_Mask(t *testing.T) {
	v := NewVarStore()
	v.Set("api_token", "super-secret-jwt")
	v.Set("host", "localhost")

	input := "token=super-secret-jwt host=localhost"
	masked := v.Mask(input)
	if strings.Contains(masked, "super-secret-jwt") {
		t.Error("secret should be masked")
	}
	if !strings.Contains(masked, "localhost") {
		t.Error("non-secret should not be masked")
	}
}

// --- Extract tests ---

func TestExtractFromJSONField(t *testing.T) {
	stdout := `{"token": "jwt-abc-123", "user": "gab"}`
	ev := extractFromJSON(stdout, "token", "jwt_token")
	if ev.key != "jwt_token" {
		t.Errorf("key = %q, want %q", ev.key, "jwt_token")
	}
	if ev.value != "jwt-abc-123" {
		t.Errorf("value = %q, want %q", ev.value, "jwt-abc-123")
	}
}

func TestExtractFromJSONFieldNested(t *testing.T) {
	stdout := `{"data": {"id": 42}}`
	ev := extractFromJSON(stdout, "data.id", "user_id")
	if ev.key != "user_id" {
		t.Errorf("key = %q, want %q", ev.key, "user_id")
	}
	if ev.value != "42" {
		t.Errorf("value = %q, want %q", ev.value, "42")
	}
}

func TestExtractFromRegex(t *testing.T) {
	stdout := "port=8080"
	ev := extractFromRegex(stdout, "port=(\\d+)", "port")
	if ev.key != "port" {
		t.Errorf("key = %q, want %q", ev.key, "port")
	}
	if ev.value != "8080" {
		t.Errorf("value = %q, want %q", ev.value, "8080")
	}
}

func TestExtractFromRegexNoCaptureGroup(t *testing.T) {
	stdout := "Listening on 8080"
	ev := extractFromRegex(stdout, "8080", "port")
	if ev.key != "port" {
		t.Errorf("key = %q, want %q", ev.key, "port")
	}
	if ev.value != "8080" {
		t.Errorf("value = %q, want %q", ev.value, "8080")
	}
}

func TestExtractFromRegexNoMatch(t *testing.T) {
	stdout := "nothing here"
	ev := extractFromRegex(stdout, "port=(\\d+)", "port")
	if ev.value != "" {
		t.Errorf("expected empty value for no match, got %q", ev.value)
	}
}

// --- processExtracts integration test ---

func TestProcessExtracts(t *testing.T) {
	store := NewVarStore()
	expect := &schema.Expect{
		JSONField: &schema.JSONFieldCheck{
			Path:    "token",
			Extract: "jwt_token",
		},
	}
	stdout := `{"token": "abc-123-def", "user": "gab"}`
	processExtracts(expect, stdout, store)

	val, ok := store.Get("jwt_token")
	if !ok {
		t.Fatal("expected jwt_token to be extracted")
	}
	if val != "abc-123-def" {
		t.Errorf("got %q, want %q", val, "abc-123-def")
	}
}

func TestProcessExtractsStdoutMatches(t *testing.T) {
	store := NewVarStore()
	expect := &schema.Expect{
		StdoutMatches: "port=(\\d+)",
		Extract:       "server_port",
	}
	stdout := "listening port=3000 ok"
	processExtracts(expect, stdout, store)

	val, ok := store.Get("server_port")
	if !ok {
		t.Fatal("expected server_port to be extracted")
	}
	if val != "3000" {
		t.Errorf("got %q, want %q", val, "3000")
	}
}

// --- Chain detection tests ---

func TestDetectChains(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		wantGroups int
	}{
		{
			name: "no chains",
			configYAML: `
version: 1
project: test
tests:
  - name: a
    run: echo hi
  - name: b
    run: echo bye
`,
			wantGroups: 0,
		},
		{
			name: "simple chain via json_field extract",
			configYAML: `
version: 1
project: test
tests:
  - name: login
    run: curl -s /auth/login
    expect:
      json_field:
        path: token
        extract: jwt_token
  - name: get-profile
    run: "curl -s -H 'Authorization: Bearer {{ .Vars.jwt_token }}' /profile"
`,
			wantGroups: 1,
		},
		{
			name: "chain via stdout_matches extract",
			configYAML: `
version: 1
project: test
tests:
  - name: start-server
    run: ./start.sh
    expect:
      stdout_matches: "port=(\\d+)"
      extract: server_port
  - name: health-check
    run: "curl http://localhost:{{ .Vars.server_port }}/health"
`,
			wantGroups: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := schema.Parse([]byte(tt.configYAML))
			if err != nil {
				t.Fatal(err)
			}
			groups := detectChains(cfg.Tests)
			if len(groups) != tt.wantGroups {
				t.Errorf("got %d chain groups, want %d", len(groups), tt.wantGroups)
			}
		})
	}
}
