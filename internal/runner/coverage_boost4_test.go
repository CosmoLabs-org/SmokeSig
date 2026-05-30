package runner

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// ============================================================
// CheckK8sResource — via fake kubectl on PATH
// ============================================================

func cb4FakeKubectlOnPATH(t *testing.T, output string, exitCode int) func() {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "kubectl")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\nexit %d\n", output, exitCode)
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("write fake kubectl: %v", err)
	}
	old := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+old)
	return func() { os.Setenv("PATH", old) }
}

func TestCoverageBoost4_CheckK8sResource_KubectlFails(t *testing.T) {
	restore := cb4FakeKubectlOnPATH(t, "Error from server: not found", 1)
	defer restore()

	check := &schema.K8sResourceCheck{
		Kind:      "deployment",
		Name:      "myapp",
		Namespace: "default",
	}
	result := CheckK8sResource(check)
	if result.Passed {
		t.Fatal("expected failure when kubectl fails")
	}
	if result.Type != "k8s_resource" {
		t.Fatalf("expected type k8s_resource, got %q", result.Type)
	}
}

func TestCoverageBoost4_CheckK8sResource_KubectlSucceeds(t *testing.T) {
	restore := cb4FakeKubectlOnPATH(t, "deployment.apps/myapp", 0)
	defer restore()

	check := &schema.K8sResourceCheck{
		Kind:      "deployment",
		Name:      "myapp",
		Namespace: "default",
	}
	result := CheckK8sResource(check)
	if !result.Passed {
		t.Fatalf("expected pass when kubectl succeeds, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckK8sResource_WithCondition_True(t *testing.T) {
	restore := cb4FakeKubectlOnPATH(t, "True", 0)
	defer restore()

	check := &schema.K8sResourceCheck{
		Kind:      "deployment",
		Name:      "myapp",
		Namespace: "default",
		Condition: "Available",
	}
	result := CheckK8sResource(check)
	if !result.Passed {
		t.Fatalf("expected pass for condition=True, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckK8sResource_WithCondition_False(t *testing.T) {
	restore := cb4FakeKubectlOnPATH(t, "False", 0)
	defer restore()

	check := &schema.K8sResourceCheck{
		Kind:      "deployment",
		Name:      "myapp",
		Namespace: "default",
		Condition: "Available",
	}
	result := CheckK8sResource(check)
	if result.Passed {
		t.Fatal("expected failure when condition=False")
	}
}

func TestCoverageBoost4_CheckK8sResource_WithContext(t *testing.T) {
	restore := cb4FakeKubectlOnPATH(t, "pod/mypod", 0)
	defer restore()

	check := &schema.K8sResourceCheck{
		Kind:      "pod",
		Name:      "mypod",
		Namespace: "kube-system",
		Context:   "my-cluster",
	}
	result := CheckK8sResource(check)
	if !result.Passed {
		t.Fatalf("expected pass with context flag, got: %v", result.Actual)
	}
}

// ============================================================
// CheckLDAPBind — via mock TCP server
// ============================================================

// buildLDAPSuccessResponse builds a minimal LDAP BindResponse with resultCode=0 (success).
func buildLDAPSuccessResponse() []byte {
	// LDAPMessage: SEQUENCE { messageID=1, bindResponse [APPLICATION 1] { resultCode=0, matchedDN="", diagnosticMessage="" } }
	resultCode := []byte{0x0a, 0x01, 0x00} // ENUMERATED, len=1, value=0 (success)
	matchedDN := []byte{0x04, 0x00}         // OCTET STRING, len=0
	diagMsg := []byte{0x04, 0x00}           // OCTET STRING, len=0
	bindRespBody := append(append(resultCode, matchedDN...), diagMsg...)
	bindResp := append([]byte{0x61, byte(len(bindRespBody))}, bindRespBody...) // APPLICATION 1
	msgID := []byte{0x02, 0x01, 0x01}                                          // INTEGER, len=1, value=1
	msgBody := append(msgID, bindResp...)
	return append([]byte{0x30, byte(len(msgBody))}, msgBody...) // SEQUENCE
}

// buildLDAPFailResponse builds a BindResponse with resultCode=49 (invalidCredentials).
func buildLDAPFailResponse() []byte {
	resultCode := []byte{0x0a, 0x01, 0x31} // resultCode=49
	matchedDN := []byte{0x04, 0x00}
	diagMsg := []byte{0x04, 0x00}
	bindRespBody := append(append(resultCode, matchedDN...), diagMsg...)
	bindResp := append([]byte{0x61, byte(len(bindRespBody))}, bindRespBody...)
	msgID := []byte{0x02, 0x01, 0x01}
	msgBody := append(msgID, bindResp...)
	return append([]byte{0x30, byte(len(msgBody))}, msgBody...)
}

func cb4StartLDAPServer(t *testing.T, response []byte) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetDeadline(time.Now().Add(2 * time.Second))
				buf := make([]byte, 4096)
				c.Read(buf) // read the BindRequest
				if len(response) > 0 {
					c.Write(response)
				}
			}(conn)
		}
	}()
	return ln, ln.Addr().(*net.TCPAddr).Port
}

func TestCoverageBoost4_CheckLDAPBind_ConnectionRefused(t *testing.T) {
	check := &schema.LDAPCheck{Host: "127.0.0.1", Port: 1}
	result := CheckLDAPBind(check)
	if result.Passed {
		t.Fatal("expected failure for connection refused")
	}
	if result.Type != "ldap_bind" {
		t.Fatalf("expected type ldap_bind, got %q", result.Type)
	}
}

func TestCoverageBoost4_CheckLDAPBind_SuccessResponse(t *testing.T) {
	ln, port := cb4StartLDAPServer(t, buildLDAPSuccessResponse())
	defer ln.Close()

	check := &schema.LDAPCheck{Host: "127.0.0.1", Port: port}
	result := CheckLDAPBind(check)
	if !result.Passed {
		t.Fatalf("expected pass for LDAP success response, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckLDAPBind_FailResponse(t *testing.T) {
	ln, port := cb4StartLDAPServer(t, buildLDAPFailResponse())
	defer ln.Close()

	check := &schema.LDAPCheck{Host: "127.0.0.1", Port: port, BindDN: "cn=admin,dc=example,dc=com"}
	result := CheckLDAPBind(check)
	if result.Passed {
		t.Fatal("expected failure for invalid credentials response")
	}
}

func TestCoverageBoost4_CheckLDAPBind_EmptyResponse(t *testing.T) {
	ln, port := cb4StartLDAPServer(t, nil)
	defer ln.Close()

	check := &schema.LDAPCheck{Host: "127.0.0.1", Port: port}
	result := CheckLDAPBind(check)
	if result.Passed {
		t.Fatal("expected failure for empty response")
	}
}

func TestCoverageBoost4_CheckLDAPBind_PasswordEnvMissing(t *testing.T) {
	os.Unsetenv("CB4_LDAP_PASS_MISSING")
	check := &schema.LDAPCheck{
		Host:        "127.0.0.1",
		Port:        389,
		PasswordEnv: "CB4_LDAP_PASS_MISSING",
	}
	result := CheckLDAPBind(check)
	if result.Passed {
		t.Fatal("expected failure when password_env not set")
	}
	if !strings.Contains(result.Actual, "not set") {
		t.Fatalf("expected 'not set' in actual, got %q", result.Actual)
	}
}

func TestCoverageBoost4_CheckLDAPBind_PasswordEnvSet(t *testing.T) {
	ln, port := cb4StartLDAPServer(t, buildLDAPSuccessResponse())
	defer ln.Close()

	os.Setenv("CB4_LDAP_PASS", "secret123")
	defer os.Unsetenv("CB4_LDAP_PASS")

	check := &schema.LDAPCheck{
		Host:        "127.0.0.1",
		Port:        port,
		PasswordEnv: "CB4_LDAP_PASS",
	}
	result := CheckLDAPBind(check)
	if !result.Passed {
		t.Fatalf("expected pass when password env is set and server responds OK, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckLDAPBind_DefaultPort_TLS(t *testing.T) {
	// Just check the default port is 636 for TLS — connection will be refused
	check := &schema.LDAPCheck{Host: "127.0.0.1", UseTLS: true}
	result := CheckLDAPBind(check)
	if result.Passed {
		t.Fatal("expected failure (nothing listening on 636)")
	}
}

func TestCoverageBoost4_CheckLDAPBind_DefaultPort_Plain(t *testing.T) {
	// Default port 389 — connection will be refused
	check := &schema.LDAPCheck{Host: "127.0.0.1"}
	result := CheckLDAPBind(check)
	if result.Passed {
		t.Fatal("expected failure (nothing listening on 389)")
	}
}

// ============================================================
// CheckKafkaBroker — via mock TCP server
// ============================================================

// buildKafkaMetadataResponse builds a minimal valid Kafka MetadataResponse.
func buildKafkaMetadataResponse() []byte {
	// Response: int32(size) + int32(correlationID=1) + minimal body
	corrID := make([]byte, 4)
	binary.BigEndian.PutUint32(corrID, 1)
	// Minimal MetadataResponse body: throttle_time_ms(4) + brokers array(4) + topics array(4)
	body := append([]byte{0x00, 0x00, 0x00, 0x00}, // throttle_time_ms=0
		[]byte{0x00, 0x00, 0x00, 0x00}...) // brokers array length=0
	body = append(body, []byte{0x00, 0x00, 0x00, 0x00}...) // topics array length=0
	payload := append(corrID, body...)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(payload)))
	return append(size, payload...)
}

func cb4StartKafkaServer(t *testing.T, response []byte) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetDeadline(time.Now().Add(2 * time.Second))
				buf := make([]byte, 4096)
				c.Read(buf)
				if len(response) > 0 {
					c.Write(response)
				}
			}(conn)
		}
	}()
	return ln, ln.Addr().String()
}

func TestCoverageBoost4_CheckKafkaBroker_ConnectionRefused(t *testing.T) {
	check := &schema.KafkaCheck{Brokers: []string{"127.0.0.1:1"}}
	result := CheckKafkaBroker(check)
	if result.Passed {
		t.Fatal("expected failure for connection refused")
	}
	if result.Type != "kafka_broker" {
		t.Fatalf("expected type kafka_broker, got %q", result.Type)
	}
}

func TestCoverageBoost4_CheckKafkaBroker_ValidResponse(t *testing.T) {
	ln, addr := cb4StartKafkaServer(t, buildKafkaMetadataResponse())
	defer ln.Close()

	check := &schema.KafkaCheck{Brokers: []string{addr}}
	result := CheckKafkaBroker(check)
	if !result.Passed {
		t.Fatalf("expected pass for valid Kafka response, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckKafkaBroker_EmptyResponse(t *testing.T) {
	ln, addr := cb4StartKafkaServer(t, nil)
	defer ln.Close()

	check := &schema.KafkaCheck{Brokers: []string{addr}}
	result := CheckKafkaBroker(check)
	if result.Passed {
		t.Fatal("expected failure for empty response")
	}
}

func TestCoverageBoost4_CheckKafkaBroker_WrongCorrelationID(t *testing.T) {
	// Build response with correlation ID = 99 (not 1)
	corrID := make([]byte, 4)
	binary.BigEndian.PutUint32(corrID, 99)
	body := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	payload := append(corrID, body...)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(payload)))
	response := append(size, payload...)

	ln, addr := cb4StartKafkaServer(t, response)
	defer ln.Close()

	check := &schema.KafkaCheck{Brokers: []string{addr}}
	result := CheckKafkaBroker(check)
	if result.Passed {
		t.Fatal("expected failure for wrong correlation ID")
	}
}

func TestCoverageBoost4_CheckKafkaBroker_WithTopic(t *testing.T) {
	ln, addr := cb4StartKafkaServer(t, buildKafkaMetadataResponse())
	defer ln.Close()

	check := &schema.KafkaCheck{Brokers: []string{addr}, Topic: "my-topic"}
	result := CheckKafkaBroker(check)
	if !result.Passed {
		t.Fatalf("expected pass with topic specified, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckKafkaBroker_DefaultPort(t *testing.T) {
	// Broker without port — default :9092 appended
	check := &schema.KafkaCheck{Brokers: []string{"127.0.0.1"}}
	result := CheckKafkaBroker(check)
	if result.Passed {
		t.Fatal("expected failure (nothing on 9092)")
	}
}

func TestCoverageBoost4_CheckKafkaBroker_MultipleBrokers_FirstFails(t *testing.T) {
	ln, addr := cb4StartKafkaServer(t, buildKafkaMetadataResponse())
	defer ln.Close()

	check := &schema.KafkaCheck{
		Brokers: []string{"127.0.0.1:1", addr}, // first fails, second succeeds
	}
	result := CheckKafkaBroker(check)
	if !result.Passed {
		t.Fatalf("expected pass when second broker succeeds, got: %v", result.Actual)
	}
}

// ============================================================
// CheckS3Bucket — via httptest server
// ============================================================

func TestCoverageBoost4_CheckS3Bucket_200Response(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "<LocationConstraint>us-east-1</LocationConstraint>")
	}))
	defer ts.Close()

	check := &schema.S3BucketCheck{
		Bucket:   "my-bucket",
		Endpoint: ts.URL,
	}
	result := CheckS3Bucket(check)
	if !result.Passed {
		t.Fatalf("expected pass for 200 response, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckS3Bucket_403Response(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer ts.Close()

	check := &schema.S3BucketCheck{
		Bucket:   "my-bucket",
		Endpoint: ts.URL,
	}
	result := CheckS3Bucket(check)
	if result.Passed {
		t.Fatal("expected failure for 403 response")
	}
	if !strings.Contains(result.Actual, "403") {
		t.Fatalf("expected '403' in actual, got %q", result.Actual)
	}
}

func TestCoverageBoost4_CheckS3Bucket_ConnectionError(t *testing.T) {
	check := &schema.S3BucketCheck{
		Bucket:   "my-bucket",
		Endpoint: "http://127.0.0.1:1",
	}
	result := CheckS3Bucket(check)
	if result.Passed {
		t.Fatal("expected failure for connection error")
	}
}

func TestCoverageBoost4_CheckS3Bucket_DefaultEndpoint(t *testing.T) {
	// No endpoint set — uses s3.amazonaws.com, will fail (no credentials)
	check := &schema.S3BucketCheck{Bucket: "nonexistent-bucket-xyz-123"}
	result := CheckS3Bucket(check)
	// Just check no panic and correct type
	if result.Type != "s3_bucket" {
		t.Fatalf("expected type s3_bucket, got %q", result.Type)
	}
}

func TestCoverageBoost4_CheckS3Bucket_404Response(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	check := &schema.S3BucketCheck{
		Bucket:   "nonexistent",
		Endpoint: ts.URL,
	}
	result := CheckS3Bucket(check)
	if result.Passed {
		t.Fatal("expected failure for 404")
	}
}

// ============================================================
// runTestOnce — more branches: S3, Kafka, LDAP, K8s via Runner.Run
// ============================================================

func TestCoverageBoost4_runTestOnce_Kafka_via_Run(t *testing.T) {
	ln, addr := cb4StartKafkaServer(t, buildKafkaMetadataResponse())
	defer ln.Close()

	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb4",
			Tests: []schema.Test{
				{Name: "kafka", Run: "true", Expect: schema.Expect{
					Kafka: &schema.KafkaCheck{Brokers: []string{addr}},
				}},
			},
		},
		Reporter:  &noopReporter{},
		ConfigDir: t.TempDir(),
	}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost4_runTestOnce_LDAP_via_Run(t *testing.T) {
	ln, port := cb4StartLDAPServer(t, buildLDAPSuccessResponse())
	defer ln.Close()

	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb4",
			Tests: []schema.Test{
				{Name: "ldap", Run: "true", Expect: schema.Expect{
					LDAP: &schema.LDAPCheck{Host: "127.0.0.1", Port: port},
				}},
			},
		},
		Reporter:  &noopReporter{},
		ConfigDir: t.TempDir(),
	}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost4_runTestOnce_K8s_via_Run(t *testing.T) {
	restore := cb4FakeKubectlOnPATH(t, "deployment.apps/myapp", 0)
	defer restore()

	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb4",
			Tests: []schema.Test{
				{Name: "k8s", Run: "true", Expect: schema.Expect{
					K8sResource: &schema.K8sResourceCheck{Kind: "deployment", Name: "myapp", Namespace: "default"},
				}},
			},
		},
		Reporter:  &noopReporter{},
		ConfigDir: t.TempDir(),
	}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost4_runTestOnce_S3_via_Run(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb4",
			Tests: []schema.Test{
				{Name: "s3", Run: "true", Expect: schema.Expect{
					S3Bucket: &schema.S3BucketCheck{Bucket: "my-bucket", Endpoint: ts.URL},
				}},
			},
		},
		Reporter:  &noopReporter{},
		ConfigDir: t.TempDir(),
	}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

// ============================================================
// CheckDockerImageExists — via fake docker
// ============================================================

func TestCoverageBoost4_CheckDockerImageExists_Success(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "{}", 0)
	defer restore()

	check := &schema.DockerImageCheck{Image: "myimage:latest"}
	result := CheckDockerImageExists(check)
	if !result.Passed {
		t.Fatalf("expected pass when docker image inspect succeeds, got: %v", result.Actual)
	}
}

func TestCoverageBoost4_CheckDockerImageExists_NotFound(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "", 1)
	defer restore()

	check := &schema.DockerImageCheck{Image: "nonexistent:latest"}
	result := CheckDockerImageExists(check)
	if result.Passed {
		t.Fatal("expected failure when image not found")
	}
}

// ============================================================
// CheckHTTPWithTrace — exercise the trace header injection path
// ============================================================

func TestCoverageBoost4_CheckHTTPWithTrace_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	tc := NewTraceContext()
	span := tc.NewSpan()

	statusCode := 200
	check := &schema.HTTPCheck{
		URL:        ts.URL,
		StatusCode: &statusCode,
	}
	results := CheckHTTPWithTrace(check, span)
	passed := true
	for _, r := range results {
		if !r.Passed {
			passed = false
		}
	}
	if !passed {
		t.Fatalf("expected all assertions to pass for CheckHTTPWithTrace")
	}
}

// ============================================================
// CheckWebSocket — server-closed path (ExpectContains set, server sends close)
// ============================================================

func TestCoverageBoost4_CheckWebSocket_ServerClosedBeforeMessage(t *testing.T) {
	// Server closes immediately after upgrade without sending a message frame.
	// When ExpectContains is set, this should result in failure (server closed).
	ln, addr := cbStartConnectOnlyWSServer(t)
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:            "ws://" + addr,
		Send:           "ping",
		ExpectContains: "expected response",
		Timeout:        schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	// Server sends close frame immediately after upgrade without a message,
	// so the result should be failure (server closed before sending expected content)
	if result.Passed {
		t.Fatal("expected failure when server closes before sending expected message")
	}
}
