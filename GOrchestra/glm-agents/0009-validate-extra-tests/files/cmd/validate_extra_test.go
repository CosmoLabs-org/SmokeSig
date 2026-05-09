//go:build ignore
package cmd

import (
	"os"
	"testing"
)

// TestValidateCmd_AllAssertionTypes validates a config containing every assertion type.
func TestValidateCmd_AllAssertionTypes(t *testing.T) {
	cfg := `
version: 1
project: all-assertions
tests:
  - name: exit-code-test
    run: echo hello
    expect:
      exit_code: 0
  - name: stdout-contains-test
    run: echo hello
    expect:
      stdout_contains: "hello"
  - name: stdout-matches-test
    run: echo hello
    expect:
      stdout_matches: "hel+o"
  - name: stderr-contains-test
    run: 'echo error >&2'
    expect:
      stderr_contains: "error"
  - name: stderr-matches-test
    run: 'echo "fail something" >&2'
    expect:
      stderr_matches: "fail.*"
  - name: file-exists-test
    run: touch testfile.txt
    expect:
      file_exists: testfile.txt
  - name: env-exists-test
    run: echo ok
    expect:
      env_exists: PATH
  - name: port-listening-test
    expect:
      port_listening:
        port: 80
        protocol: tcp
        host: localhost
  - name: process-running-test
    expect:
      process_running: init
  - name: http-test
    expect:
      http:
        url: http://localhost:80
        method: GET
        status_code: 200
  - name: json-field-test
    run: 'echo ''{"key": "value"}'''
    expect:
      json_field:
        path: key
        equals: "value"
  - name: response-time-test
    run: echo fast
    expect:
      response_time_ms: 5000
  - name: ssl-cert-test
    expect:
      ssl_cert:
        host: example.com
        port: 443
        min_days_remaining: 1
  - name: redis-ping-test
    expect:
      redis_ping:
        host: localhost
        port: 6379
  - name: memcached-version-test
    expect:
      memcached_version:
        host: localhost
        port: 11211
  - name: postgres-ping-test
    expect:
      postgres_ping:
        host: localhost
        port: 5432
  - name: mysql-ping-test
    expect:
      mysql_ping:
        host: localhost
        port: 3306
  - name: grpc-health-test
    expect:
      grpc_health:
        address: localhost:9090
        service: ""
        use_tls: false
        timeout: 5s
  - name: docker-container-test
    expect:
      docker_container_running:
        name: test-container
  - name: docker-image-test
    expect:
      docker_image_exists:
        image: alpine:latest
  - name: url-reachable-test
    expect:
      url_reachable:
        url: http://localhost:80
        timeout: 5s
  - name: service-reachable-test
    expect:
      service_reachable:
        url: http://localhost:80
        timeout: 5s
  - name: s3-bucket-test
    expect:
      s3_bucket:
        bucket: test-bucket
        region: us-east-1
  - name: version-check-test
    expect:
      version_check:
        command: go version
        pattern: "go[0-9]+\\.[0-9]+"
  - name: websocket-test
    expect:
      websocket:
        url: ws://localhost:8080/ws
        send: '{"type":"ping"}'
        expect_contains: "pong"
  - name: otel-trace-test
    expect:
      otel_trace:
        backend: jaeger
        jaeger_url: http://localhost:16686
        service_name: test-svc
        min_spans: 1
        timeout: 10s
  - name: credential-check-test
    expect:
      credential_check:
        source: env
        name: HOME
  - name: graphql-test
    expect:
      graphql:
        url: http://localhost:4000/graphql
        query: "{ __schema { types { name } } }"
        status_code: 200
`
	dir := t.TempDir()
	path := dir + "/.smoke.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v\n%s", err, out)
	}
	if out == "" {
		t.Error("expected some output")
	}
}

// TestValidateCmd_OTelEnabled validates a config with OpenTelemetry enabled.
func TestValidateCmd_OTelEnabled(t *testing.T) {
	cfg := `
version: 1
project: otel-test
otel:
  enabled: true
  jaeger_url: "http://jaeger:16686"
  service_name: "SmokeSig"
  trace_propagation: true
  export_url: "http://jaeger:16686/v1/traces"
tests:
  - name: trace-check
    run: curl http://localhost:8080
    expect:
      exit_code: 0
      otel_trace:
        jaeger_url: "http://jaeger:16686"
        service_name: "SmokeSig"
        min_spans: 1
        timeout: 10s
`
	dir := t.TempDir()
	path := dir + "/.smoke.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v\n%s", err, out)
	}
	if out == "" {
		t.Error("expected some output")
	}
}

// TestValidateCmd_RetryPolicy validates a config with retry policies.
func TestValidateCmd_RetryPolicy(t *testing.T) {
	cfg := `
version: 1
project: retry-test
tests:
  - name: flaky-endpoint
    run: curl http://localhost:8080/health
    expect:
      exit_code: 0
    retry:
      count: 3
      backoff: 2s
  - name: trace-only-retry
    run: curl http://localhost:8080/api
    expect:
      exit_code: 0
      otel_trace:
        jaeger_url: "http://localhost:16686"
        service_name: "test-svc"
        timeout: 5s
    retry:
      count: 2
      backoff: 1s
      retry_on_trace_only: true
`
	dir := t.TempDir()
	path := dir + "/.smoke.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v\n%s", err, out)
	}
	if out == "" {
		t.Error("expected some output")
	}
}

// TestValidateCmd_SkipIf validates a config with skip_if conditions.
func TestValidateCmd_SkipIf(t *testing.T) {
	cfg := `
version: 1
project: skipif-test
tests:
  - name: skip-if-env-unset
    run: echo running
    expect:
      exit_code: 0
    skip_if:
      env_unset: SKIP_TESTS
  - name: skip-if-env-equals
    run: echo running
    expect:
      exit_code: 0
    skip_if:
      env_equals:
        var: CI
        value: "true"
  - name: skip-if-file-missing
    run: echo running
    expect:
      exit_code: 0
    skip_if:
      file_missing: /tmp/skip-marker
`
	dir := t.TempDir()
	path := dir + "/.smoke.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v\n%s", err, out)
	}
	if out == "" {
		t.Error("expected some output")
	}
}

// TestValidateCmd_EnvOverrides validates a config using Go template env vars.
func TestValidateCmd_EnvOverrides(t *testing.T) {
	t.Setenv("SMOKESIG_PROJECT", "env-override-test")
	t.Setenv("SMOKESIG_URL", "http://localhost:8080")

	cfg := `
version: 1
project: "{{ .Env.SMOKESIG_PROJECT }}"
tests:
  - name: templated-url
    expect:
      http:
        url: "{{ .Env.SMOKESIG_URL }}/health"
        method: GET
        status_code: 200
  - name: fallback-run
    run: echo ok
    expect:
      exit_code: 0
`
	dir := t.TempDir()
	path := dir + "/.smoke.yaml"
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runValidate(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v\n%s", err, out)
	}
	if out == "" {
		t.Error("expected some output")
	}
}
