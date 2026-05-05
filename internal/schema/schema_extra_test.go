package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDuration_MarshalYAML(t *testing.T) {
	d := Duration{Duration: 5 * time.Second}
	out, err := yaml.Marshal(&d)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got != "5s" {
		t.Errorf("MarshalYAML = %q, want %q", got, "5s")
	}
}

func TestDuration_UnmarshalYAML_InvalidDuration(t *testing.T) {
	input := `
version: 1
project: test
settings:
  timeout: not_a_duration
tests:
  - name: t
    run: echo
    expect:
      exit_code: 0
`
	_, err := Parse([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid duration string")
	}
	if !strings.Contains(err.Error(), "invalid duration") {
		t.Errorf("error = %q, want mention of invalid duration", err.Error())
	}
}

func TestLoadDefault_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	_, err := LoadDefault()
	if err == nil {
		t.Fatal("expected error when no .smokesig.yaml exists")
	}
}

func TestLoadDefault_FileExists(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
version: 1
project: default-test
tests:
  - name: t
    run: echo
    expect:
      exit_code: 0
`
	if err := os.WriteFile(filepath.Join(dir, ".smokesig.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if cfg.Project != "default-test" {
		t.Errorf("project = %q, want default-test", cfg.Project)
	}
}

func TestProcessTemplate_BadTemplate(t *testing.T) {
	// This exercises the template parse error path
	dir := t.TempDir()
	// Write a config with an invalid Go template syntax
	yamlContent := `
version: 1
project: bad-template
tests:
  - name: t
    run: "echo {{ .Bad.Field"
    expect:
      exit_code: 0
`
	configPath := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for bad template")
	}
	if !strings.Contains(err.Error(), "processing template") {
		t.Errorf("error = %q, want mention of template error", err.Error())
	}
}

func TestLoadWithDepth_InvalidIncludePath(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
version: 1
project: include-test
includes:
  - nonexistent.yaml
tests:
  - name: t
    run: echo
    expect:
      exit_code: 0
`
	configPath := filepath.Join(dir, ".smokesig.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing include file")
	}
	if !strings.Contains(err.Error(), "loading include") {
		t.Errorf("error = %q, want mention of include", err.Error())
	}
}

func TestParse_InvalidJSONInExpect(t *testing.T) {
	_, err := Parse([]byte("not: valid\n[yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_AllCheckTypes(t *testing.T) {
	input := `
version: 1
project: checks
tests:
  - name: port
    expect:
      port_listening:
        port: 8080
        protocol: tcp
        host: localhost
  - name: ssl
    expect:
      ssl_cert:
        host: example.com
        port: 443
        min_days_remaining: 30
        allow_self_signed: false
  - name: http
    expect:
      http:
        url: http://localhost:8080/health
        method: GET
        headers:
          Authorization: "Bearer token"
        body: "test"
        timeout: 5s
        status_code: 200
        body_contains: "ok"
        body_matches: "ok.*"
        header_contains:
          Content-Type: "application/json"
  - name: json
    expect:
      json_field:
        path: ".status"
        equals: "healthy"
        contains: "health"
        matches: "health.*"
  - name: redis
    expect:
      redis_ping:
        host: localhost
        port: 6379
        password: secret
  - name: memcached
    expect:
      memcached_version:
        host: localhost
        port: 11211
  - name: postgres
    expect:
      postgres_ping:
        host: localhost
        port: 5432
  - name: mysql
    expect:
      mysql_ping:
        host: localhost
        port: 3306
  - name: docker
    expect:
      docker_container_running:
        name: my-container
  - name: docker-image
    expect:
      docker_image_exists:
        image: nginx:latest
  - name: url-reachable
    expect:
      url_reachable:
        url: https://example.com
        timeout: 10s
        status_code: 200
  - name: service-reachable
    expect:
      service_reachable:
        url: https://api.example.com
  - name: s3
    expect:
      s3_bucket:
        bucket: my-bucket
        region: us-east-1
        endpoint: https://s3.amazonaws.com
  - name: version
    expect:
      version_check:
        command: "go version"
        pattern: "go1\\.2[0-9]"
  - name: websocket
    expect:
      websocket:
        url: ws://localhost:8080/ws
        send: '{"type":"ping"}'
        expect_contains: "pong"
        expect_matches: "pong.*"
        timeout: 5s
        headers:
          Authorization: "Bearer token"
  - name: otel
    expect:
      otel_trace:
        backend: jaeger
        jaeger_url: http://localhost:16686
        service_name: my-service
        min_spans: 2
        timeout: 10s
  - name: credential
    expect:
      credential_check:
        source: env
        name: API_KEY
        contains: "sk-"
  - name: graphql
    expect:
      graphql:
        url: https://api.example.com/graphql
        query: "{ __schema { types { name } } }"
        status_code: 200
        expect_types:
          - Query
          - Mutation
        expect_contains: "Query"
        timeout: 5s
`
	cfg, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Tests) != 18 {
		t.Fatalf("expected 18 tests, got %d", len(cfg.Tests))
	}
}

// --- Validate extended tests ---

func TestValidate_StandaloneAssertion_NoRunRequired(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "port-check", Expect: Expect{PortListening: &PortCheck{Port: 8080}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("standalone assertion should not require run: %v", err)
	}
}

func TestValidate_StandaloneAssertion_ProcessRunning(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "proc", Expect: Expect{ProcessRunning: "nginx"}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("process_running should be standalone assertion: %v", err)
	}
}

func TestValidate_URLReachable_EmptyURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "url", Run: "true", Expect: Expect{URLReachable: &URLReachableCheck{URL: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty url_reachable.url")
	}
	if !strings.Contains(err.Error(), "url_reachable.url is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestValidate_URLReachable_BadScheme(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "url", Run: "true", Expect: Expect{URLReachable: &URLReachableCheck{URL: "ftp://example.com"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for bad url scheme")
	}
	if !strings.Contains(err.Error(), "must start with http:// or https://") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestValidate_URLReachable_Valid(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "url", Expect: Expect{URLReachable: &URLReachableCheck{URL: "https://example.com"}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("valid url_reachable should pass: %v", err)
	}
}

func TestValidate_ServiceReachable_EmptyURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "svc", Run: "true", Expect: Expect{ServiceReachable: &ServiceReachableCheck{URL: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty service_reachable.url")
	}
}

func TestValidate_ServiceReachable_BadScheme(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "svc", Run: "true", Expect: Expect{ServiceReachable: &ServiceReachableCheck{URL: "grpc://svc:9090"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for bad service_reachable scheme")
	}
}

func TestValidate_S3Bucket_EmptyBucket(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "s3", Run: "true", Expect: Expect{S3Bucket: &S3BucketCheck{Bucket: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty s3_bucket.bucket")
	}
}

func TestValidate_S3Bucket_Valid(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "s3", Expect: Expect{S3Bucket: &S3BucketCheck{Bucket: "my-bucket"}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("valid s3_bucket should pass: %v", err)
	}
}

func TestValidate_VersionCheck_EmptyCommand(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ver", Run: "true", Expect: Expect{VersionCheck: &VersionCheck{Command: "", Pattern: "1.0"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty version_check.command")
	}
}

func TestValidate_VersionCheck_EmptyPattern(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ver", Run: "true", Expect: Expect{VersionCheck: &VersionCheck{Command: "go version", Pattern: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty version_check.pattern")
	}
}

func TestValidate_VersionCheck_InvalidRegex(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ver", Run: "true", Expect: Expect{VersionCheck: &VersionCheck{Command: "go version", Pattern: "[invalid"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid regex in version_check.pattern")
	}
}

func TestValidate_VersionCheck_Valid(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ver", Expect: Expect{VersionCheck: &VersionCheck{Command: "go version", Pattern: "go1\\.\\d+"}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("valid version_check should pass: %v", err)
	}
}

func TestValidate_WebSocket_EmptyURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ws", Run: "true", Expect: Expect{WebSocket: &WebSocketCheck{URL: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty websocket.url")
	}
}

func TestValidate_WebSocket_BadScheme(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ws", Run: "true", Expect: Expect{WebSocket: &WebSocketCheck{URL: "http://example.com"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for bad websocket scheme")
	}
}

func TestValidate_WebSocket_InvalidExpectMatches(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ws", Run: "true", Expect: Expect{WebSocket: &WebSocketCheck{
				URL:          "ws://localhost:8080",
				ExpectMatches: "[invalid",
			}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid websocket.expect_matches regex")
	}
}

func TestValidate_WebSocket_Valid(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "ws", Expect: Expect{WebSocket: &WebSocketCheck{
				URL:            "ws://localhost:8080",
				ExpectContains: "pong",
			}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("valid websocket should pass: %v", err)
	}
}

func TestValidate_Credential_EmptySource(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "cred", Run: "true", Expect: Expect{Credential: &CredentialCheck{Source: "", Name: "API_KEY"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty credential_check.source")
	}
}

func TestValidate_Credential_InvalidSource(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "cred", Run: "true", Expect: Expect{Credential: &CredentialCheck{Source: "vault", Name: "API_KEY"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid credential_check.source")
	}
}

func TestValidate_Credential_EmptyName(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "cred", Run: "true", Expect: Expect{Credential: &CredentialCheck{Source: "env", Name: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty credential_check.name")
	}
}

func TestValidate_Credential_Valid(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "cred", Expect: Expect{Credential: &CredentialCheck{Source: "env", Name: "API_KEY"}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("valid credential should pass: %v", err)
	}
}

func TestValidate_GraphQL_EmptyURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "gql", Run: "true", Expect: Expect{GraphQL: &GraphQLCheck{URL: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty graphql.url")
	}
}

func TestValidate_GraphQL_ValidIntrospection(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "gql", Expect: Expect{GraphQL: &GraphQLCheck{URL: "https://api.example.com/graphql"}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("graphql with just URL should be valid: %v", err)
	}
}

func TestValidate_OTelTrace_InvalidJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "otel", Run: "true", Expect: Expect{OTelTrace: &OTelTraceCheck{JaegerURL: "ftp://bad"}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid jaeger_url scheme")
	}
}

func TestValidate_OTelTrace_GlobalJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		OTel:    OTelConfig{JaegerURL: "http://jaeger:16686"},
		Tests: []Test{
			{Name: "otel", Run: "true", Expect: Expect{OTelTrace: &OTelTraceCheck{MinSpans: 1}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("global jaeger_url should satisfy otel_trace: %v", err)
	}
}

func TestValidate_OTelTrace_NegativeMinSpans(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "otel", Run: "true", Expect: Expect{OTelTrace: &OTelTraceCheck{
				JaegerURL: "http://jaeger:16686",
				MinSpans:  -1,
			}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for negative min_spans")
	}
}

func TestValidate_OTelEnabled_InvalidJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		OTel:    OTelConfig{Enabled: true, JaegerURL: "ftp://bad"},
		Tests:   []Test{{Name: "t", Run: "true"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid otel.jaeger_url scheme")
	}
}

func TestValidate_OTelTrace_Honeycomb_WithAPIKey(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "otel", Run: "true", Expect: Expect{OTelTrace: &OTelTraceCheck{
				JaegerURL: "https://api.honeycomb.io",
				Backend:   "honeycomb",
				APIKey:    "hc-key-123",
			}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("valid honeycomb config should pass: %v", err)
	}
}

func TestValidate_OTelTrace_Datadog_WithAPIKey(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "otel", Run: "true", Expect: Expect{OTelTrace: &OTelTraceCheck{
				JaegerURL: "https://api.datadoghq.com",
				Backend:   "datadog",
				APIKey:    "dd-key-123",
			}}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("valid datadog config should pass: %v", err)
	}
}

func TestValidationError_ErrorFormat(t *testing.T) {
	ve := &ValidationError{Errors: []string{"err1", "err2"}}
	msg := ve.Error()
	if !strings.Contains(msg, "err1") || !strings.Contains(msg, "err2") {
		t.Errorf("Error() = %q, should contain all errors", msg)
	}
}

func TestMergeEnv_ParallelOverride(t *testing.T) {
	dir := t.TempDir()

	baseYAML := `
project: myapp
tests:
  - name: base
    run: "true"
    expect:
      exit_code: 0
`
	basePath := filepath.Join(dir, ".smokesig.yaml")
	os.WriteFile(basePath, []byte(baseYAML), 0644)

	envYAML := `
settings:
  parallel: true
tests: []
`
	envPath := filepath.Join(dir, "parallel.smokesig.yaml")
	os.WriteFile(envPath, []byte(envYAML), 0644)

	base, _ := Load(basePath)
	merged, err := MergeEnv(base, envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !merged.Settings.Parallel {
		t.Error("parallel should be true after env merge")
	}
}
