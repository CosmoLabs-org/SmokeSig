package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// --- generateExpectBlock tests ---

func TestGenerateExpectBlock_AllTypes(t *testing.T) {
	tests := []struct {
		assertionType string
		params        map[string]interface{}
		wantContains  string
	}{
		{"exit_code", map[string]interface{}{"code": float64(0)}, "exit_code: 0"},
		{"exit_code", nil, "exit_code: 0"},
		{"stdout_contains", map[string]interface{}{"text": "hello"}, "stdout_contains"},
		{"stdout_contains", nil, "stdout_contains"},
		{"stdout_matches", map[string]interface{}{"pattern": "^v"}, "stdout_matches"},
		{"stderr_contains", map[string]interface{}{"text": "err"}, "stderr_contains"},
		{"stderr_matches", map[string]interface{}{"pattern": "err.*"}, "stderr_matches"},
		{"file_exists", map[string]interface{}{"path": "out.txt"}, "file_exists"},
		{"env_exists", map[string]interface{}{"var": "HOME"}, "env_exists"},
		{"port_listening", map[string]interface{}{"port": float64(9090)}, "port_listening"},
		{"port_listening", nil, "port_listening"},
		{"process_running", map[string]interface{}{"name": "nginx"}, "process_running"},
		{"http", map[string]interface{}{"url": "http://localhost/health"}, "http"},
		{"http", nil, "http"},
		{"json_field", map[string]interface{}{"path": "status", "equals": "ok"}, "equals"},
		{"json_field", map[string]interface{}{"path": "status"}, "json_field"},
		{"response_time_ms", map[string]interface{}{"ms": float64(500)}, "response_time_ms"},
		{"ssl_cert", map[string]interface{}{"host": "example.com"}, "ssl_cert"},
		{"ssl_cert", nil, "ssl_cert"},
		{"redis_ping", map[string]interface{}{"host": "redis", "port": float64(6379)}, "redis_ping"},
		{"memcached_version", map[string]interface{}{"host": "memcached"}, "memcached_version"},
		{"postgres_ping", map[string]interface{}{"host": "db"}, "postgres_ping"},
		{"mysql_ping", map[string]interface{}{"host": "db"}, "mysql_ping"},
		{"grpc_health", map[string]interface{}{"address": "localhost:9090"}, "grpc_health"},
		{"docker_container_running", map[string]interface{}{"name": "my-container"}, "docker_container_running"},
		{"docker_image_exists", map[string]interface{}{"image": "nginx:alpine"}, "docker_image_exists"},
		{"url_reachable", map[string]interface{}{"url": "https://example.com"}, "url_reachable"},
		{"service_reachable", map[string]interface{}{"url": "https://api.example.com"}, "service_reachable"},
		{"s3_bucket", map[string]interface{}{"bucket": "my-bucket"}, "s3_bucket"},
		{"version_check", map[string]interface{}{"command": "go version"}, "version_check"},
		{"websocket", map[string]interface{}{"url": "ws://localhost/ws"}, "websocket"},
		{"otel_trace", map[string]interface{}{"service_name": "svc"}, "otel_trace"},
		{"credential_check", map[string]interface{}{"source": "env"}, "credential_check"},
		{"graphql", map[string]interface{}{"url": "http://localhost/graphql"}, "graphql"},
		{"unknown_type", nil, "unknown_type"},
	}

	for _, tt := range tests {
		t.Run(tt.assertionType, func(t *testing.T) {
			got := generateExpectBlock(tt.assertionType, tt.params)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("generateExpectBlock(%q) = %q, want to contain %q", tt.assertionType, got, tt.wantContains)
			}
		})
	}
}

// --- generateTestYAML tests ---

func TestGenerateTestYAML_WithTags(t *testing.T) {
	got := generateTestYAML("my test", "http", nil, []string{"integration", "api"})
	if !strings.Contains(got, "my test") {
		t.Error("should contain test name")
	}
	if !strings.Contains(got, "tags:") {
		t.Error("should contain tags block")
	}
	if !strings.Contains(got, "integration") {
		t.Error("should contain tag 'integration'")
	}
}

func TestGenerateTestYAML_NeedsRunCommand(t *testing.T) {
	got := generateTestYAML("test", "exit_code", nil, nil)
	if !strings.Contains(got, "run: <command>") {
		t.Errorf("exit_code needs a run command, got: %s", got)
	}
}

func TestGenerateTestYAML_StandaloneNoRunCommand(t *testing.T) {
	got := generateTestYAML("test", "http", nil, nil)
	if strings.Contains(got, "run: <command>") {
		t.Errorf("http is standalone, should not have run command, got: %s", got)
	}
}

func TestNeedsRunCommand(t *testing.T) {
	tests := []struct {
		assertionType string
		needsRun      bool
	}{
		{"exit_code", true},
		{"stdout_contains", true},
		{"stderr_contains", true},
		{"file_exists", true},
		{"response_time_ms", true},
		{"http", false},
		{"port_listening", false},
		{"redis_ping", false},
		{"docker_container_running", false},
		{"url_reachable", false},
		{"websocket", false},
		{"otel_trace", false},
		{"graphql", false},
	}
	for _, tt := range tests {
		t.Run(tt.assertionType, func(t *testing.T) {
			got := needsRunCommand(tt.assertionType)
			if got != tt.needsRun {
				t.Errorf("needsRunCommand(%q) = %v, want %v", tt.assertionType, got, tt.needsRun)
			}
		})
	}
}

// --- boolArg tests ---

func TestBoolArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		def  bool
		want bool
	}{
		{"missing key", map[string]interface{}{}, "flag", false, false},
		{"missing key with true default", map[string]interface{}{}, "flag", true, true},
		{"bool true", map[string]interface{}{"flag": true}, "flag", false, true},
		{"bool false", map[string]interface{}{"flag": false}, "flag", true, false},
		{"non-bool value", map[string]interface{}{"flag": "yes"}, "flag", false, false},
		{"non-bool int", map[string]interface{}{"flag": 1}, "flag", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boolArg(tt.args, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("boolArg() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- strArg tests ---

func TestStrArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		def  string
		want string
	}{
		{"missing key", map[string]interface{}{}, "key", "default", "default"},
		{"string value", map[string]interface{}{"key": "value"}, "key", "default", "value"},
		{"non-string value", map[string]interface{}{"key": 42}, "key", "default", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strArg(tt.args, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("strArg() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- strSliceArg tests ---

func TestStrSliceArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		want []string
	}{
		{"missing key", map[string]interface{}{}, "tags", nil},
		{"[]string value", map[string]interface{}{"tags": []string{"a", "b"}}, "tags", []string{"a", "b"}},
		{"[]interface{} value", map[string]interface{}{"tags": []interface{}{"a", "b"}}, "tags", []string{"a", "b"}},
		{"[]interface{} with non-strings", map[string]interface{}{"tags": []interface{}{"a", 42, "b"}}, "tags", []string{"a", "b"}},
		{"other type", map[string]interface{}{"tags": "a,b"}, "tags", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strSliceArg(tt.args, tt.key)
			if len(got) != len(tt.want) {
				t.Fatalf("strSliceArg() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("strSliceArg()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- hasTag tests ---

func TestHasTag(t *testing.T) {
	tests := []struct {
		name   string
		tags   []string
		filter []string
		want   bool
	}{
		{"match", []string{"api", "integration"}, []string{"api"}, true},
		{"no match", []string{"api"}, []string{"unit"}, false},
		{"empty tags", nil, []string{"api"}, false},
		{"empty filter", []string{"api"}, nil, false},
		{"both empty", nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasTag(tt.tags, tt.filter)
			if got != tt.want {
				t.Errorf("hasTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- intParam / strParam tests ---

func TestIntParam(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		key    string
		def    int
		want   int
	}{
		{"missing key", nil, "port", 8080, 8080},
		{"float64 value", map[string]interface{}{"port": float64(9090)}, "port", 8080, 9090},
		{"non-float value", map[string]interface{}{"port": "8080"}, "port", 8080, 8080},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intParam(tt.params, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("intParam() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStrParam(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		key    string
		def    string
		want   string
	}{
		{"missing key", nil, "host", "localhost", "localhost"},
		{"string value", map[string]interface{}{"host": "redis"}, "host", "localhost", "redis"},
		{"non-string value", map[string]interface{}{"host": 42}, "host", "localhost", "localhost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strParam(tt.params, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("strParam() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- skipIfString tests ---

func TestSkipIfString(t *testing.T) {
	tests := []struct {
		name string
		si   *schema.SkipIf
		want string
	}{
		{"nil", nil, ""},
		{"env_unset only", &schema.SkipIf{EnvUnset: "CI"}, "env_unset:CI"},
		{"env_equals only", &schema.SkipIf{EnvEquals: &schema.EnvEqualsCond{Var: "ENV", Value: "prod"}}, "env_equals:ENV=prod"},
		{"file_missing only", &schema.SkipIf{FileMissing: "docker-compose.yml"}, "file_missing:docker-compose.yml"},
		{"all fields", &schema.SkipIf{
			EnvUnset:    "CI",
			EnvEquals:   &schema.EnvEqualsCond{Var: "ENV", Value: "staging"},
			FileMissing: "docker-compose.yml",
		}, "env_unset:CI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skipIfString(tt.si)
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("skipIfString() = %q, want to contain %q", got, tt.want)
			}
			if tt.want == "" && got != "" {
				t.Errorf("skipIfString() = %q, want empty", got)
			}
		})
	}
}

// --- getAssertionTypes comprehensive test ---

func TestGetAssertionTypes_AllTypes(t *testing.T) {
	exitCode := 0
	rtms := 500
	e := schema.Expect{
		ExitCode:       &exitCode,
		StdoutContains: "hello",
		StdoutMatches:  "hel*",
		StderrContains: "err",
		StderrMatches:  "err*",
		FileExists:     "out.txt",
		EnvExists:      "HOME",
		PortListening:  &schema.PortCheck{Port: 8080},
		ProcessRunning: "nginx",
		HTTP:           &schema.HTTPCheck{URL: "http://localhost/health"},
		JSONField:      &schema.JSONFieldCheck{Path: "status"},
		ResponseTimeMs: &rtms,
		SSLCert:        &schema.SSLCertCheck{Host: "example.com"},
		Redis:          &schema.RedisCheck{},
		Memcached:      &schema.MemcachedCheck{},
		Postgres:       &schema.PostgresCheck{},
		MySQL:          &schema.MySQLCheck{},
		GRPCHealth:     &schema.GRPCHealthCheck{Address: "localhost:9090"},
		DockerContainer: &schema.DockerContainerCheck{Name: "web"},
		DockerImage:    &schema.DockerImageCheck{Image: "nginx"},
		URLReachable:   &schema.URLReachableCheck{URL: "https://example.com"},
		ServiceReachable: &schema.ServiceReachableCheck{URL: "https://api.example.com"},
		S3Bucket:       &schema.S3BucketCheck{Bucket: "bucket"},
		VersionCheck:   &schema.VersionCheck{Command: "go version", Pattern: "go\\d+"},
		OTelTrace:      &schema.OTelTraceCheck{},
		WebSocket:      &schema.WebSocketCheck{URL: "ws://localhost/ws"},
		Credential:     &schema.CredentialCheck{Source: "env", Name: "KEY"},
		GraphQL:        &schema.GraphQLCheck{URL: "http://localhost/graphql"},
	}

	types := getAssertionTypes(e)
	expected := []string{
		"exit_code", "stdout_contains", "stdout_matches", "stderr_contains", "stderr_matches",
		"file_exists", "env_exists", "port_listening", "process_running", "http",
		"json_field", "response_time_ms", "ssl_cert", "redis_ping", "memcached_version",
		"postgres_ping", "mysql_ping", "grpc_health", "docker_container_running",
		"docker_image_exists", "url_reachable", "service_reachable", "s3_bucket",
		"version_check", "otel_trace", "websocket", "credential_check", "graphql",
	}

	if len(types) != len(expected) {
		t.Fatalf("expected %d assertion types, got %d: %v", len(expected), len(types), types)
	}

	typeSet := make(map[string]bool)
	for _, at := range types {
		typeSet[at] = true
	}
	for _, exp := range expected {
		if !typeSet[exp] {
			t.Errorf("missing assertion type: %s", exp)
		}
	}
}

func TestGetAssertionTypes_Empty(t *testing.T) {
	types := getAssertionTypes(schema.Expect{})
	if len(types) != 0 {
		t.Errorf("empty Expect should have 0 types, got %d", len(types))
	}
}

// --- suiteResultToMCP tests ---

func TestSuiteResultToMCP_WithErrors(t *testing.T) {
	err := errors.New("command failed")
	sr := &runner.SuiteResult{
		Project:  "test-project",
		Total:    2,
		Passed:   1,
		Failed:   1,
		Skipped:  0,
		Duration: 0,
		Tests: []runner.TestResult{
			{
				Name:   "passing-test",
				Passed: true,
				Assertions: []runner.AssertionResult{
					{Type: "exit_code", Expected: "0", Actual: "0", Passed: true},
				},
			},
			{
				Name:   "failing-test",
				Passed: false,
				Error:  err,
				Assertions: []runner.AssertionResult{
					{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
				},
			},
		},
	}

	result := suiteResultToMCP(sr, "/path/to/.smokesig.yaml")

	if result.Project != "test-project" {
		t.Errorf("project = %q, want test-project", result.Project)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	if len(result.Tests) != 2 {
		t.Fatalf("tests count = %d, want 2", len(result.Tests))
	}

	// Check passing test
	if result.Tests[0].Error != "" {
		t.Errorf("passing test should have no error, got %q", result.Tests[0].Error)
	}

	// Check failing test has error message and fix suggestions
	if result.Tests[1].Error == "" {
		t.Error("failing test should have error message")
	}
	if len(result.Tests[1].FixSuggestions) == 0 {
		t.Error("failing test should have fix suggestions")
	}
}

func TestSuiteResultToMCP_AllowedFailure(t *testing.T) {
	sr := &runner.SuiteResult{
		Project: "test",
		Total:   1,
		Passed:  0,
		Failed:  1,
		Tests: []runner.TestResult{
			{
				Name:           "flaky",
				Passed:         false,
				AllowedFailure: true,
				Assertions: []runner.AssertionResult{
					{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
				},
			},
		},
	}

	result := suiteResultToMCP(sr, ".smokesig.yaml")
	if len(result.Tests) != 1 {
		t.Fatalf("tests = %d, want 1", len(result.Tests))
	}
	if !result.Tests[0].AllowedFailure {
		t.Error("AllowedFailure should be true")
	}
	// Allowed failures should NOT have fix suggestions
	if len(result.Tests[0].FixSuggestions) != 0 {
		t.Errorf("allowed failure should not have fix suggestions, got %v", result.Tests[0].FixSuggestions)
	}
}

func TestSuiteResultToMCP_SkippedTest(t *testing.T) {
	sr := &runner.SuiteResult{
		Project: "test",
		Total:   1,
		Skipped: 1,
		Tests: []runner.TestResult{
			{
				Name:    "skipped-test",
				Passed:  false,
				Skipped: true,
			},
		},
	}

	result := suiteResultToMCP(sr, ".smokesig.yaml")
	if len(result.Tests) != 1 {
		t.Fatalf("tests = %d, want 1", len(result.Tests))
	}
	if !result.Tests[0].Skipped {
		t.Error("Skipped should be true")
	}
}

// --- resolveConfigPath tests ---

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"absolute path", "/abs/path/.smokesig.yaml", "/abs/path/.smokesig.yaml"},
		{"relative path", ".smokesig.yaml", ".smokesig.yaml"}, // just check it doesn't panic
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveConfigPath(tt.path)
			if tt.path == "/abs/path/.smokesig.yaml" && got != tt.want {
				t.Errorf("resolveConfigPath() = %q, want %q", got, tt.want)
			}
			// For relative paths, just ensure we got a non-empty result
			if got == "" {
				t.Error("resolveConfigPath() returned empty string")
			}
		})
	}
}
