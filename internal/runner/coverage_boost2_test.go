package runner

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// ============================================================
// Fake docker binary helper — injects a script on PATH so
// CheckDockerComposeHealthy can be tested without a real daemon.
// ============================================================

func cb2FakeDockerOnPATH(t *testing.T, output string, exitCode int) func() {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "docker")
	// Single-quoted printf to avoid shell interpretation of JSON braces
	content := fmt.Sprintf("#!/bin/sh\ncat <<'EOFJSON'\n%s\nEOFJSON\nexit %d\n", output, exitCode)
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	old := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+old)
	return func() { os.Setenv("PATH", old) }
}

func TestCoverageBoost2_CheckDockerComposeHealthy_DockerFails(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "", 1)
	defer restore()

	check := &schema.DockerComposeCheck{}
	result := CheckDockerComposeHealthy(check)
	if result.Passed {
		t.Fatal("expected failure when docker exits non-zero")
	}
	if result.Type != "docker_compose_healthy" {
		t.Fatalf("expected type docker_compose_healthy, got %q", result.Type)
	}
}

func TestCoverageBoost2_CheckDockerComposeHealthy_InvalidJSON(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "not valid json", 0)
	defer restore()

	check := &schema.DockerComposeCheck{}
	result := CheckDockerComposeHealthy(check)
	if result.Passed {
		t.Fatal("expected failure for invalid JSON")
	}
}

func TestCoverageBoost2_CheckDockerComposeHealthy_EmptyArray(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "[]", 0)
	defer restore()

	check := &schema.DockerComposeCheck{}
	result := CheckDockerComposeHealthy(check)
	if result.Passed {
		t.Fatal("expected failure for empty services array")
	}
	if result.Actual != "no services found" {
		t.Fatalf("expected 'no services found', got %q", result.Actual)
	}
}

func TestCoverageBoost2_CheckDockerComposeHealthy_AllRunning(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "db", State: "running", Health: ""},
	}
	jsonBytes, _ := json.Marshal(services)
	restore := cb2FakeDockerOnPATH(t, string(jsonBytes), 0)
	defer restore()

	check := &schema.DockerComposeCheck{}
	result := CheckDockerComposeHealthy(check)
	if !result.Passed {
		t.Fatalf("expected pass for healthy services, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckDockerComposeHealthy_OneExited(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "db", State: "exited", Health: ""},
	}
	jsonBytes, _ := json.Marshal(services)
	restore := cb2FakeDockerOnPATH(t, string(jsonBytes), 0)
	defer restore()

	check := &schema.DockerComposeCheck{}
	result := CheckDockerComposeHealthy(check)
	if result.Passed {
		t.Fatal("expected failure when service exited")
	}
}

func TestCoverageBoost2_CheckDockerComposeHealthy_UnhealthyStatus(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "unhealthy"},
	}
	jsonBytes, _ := json.Marshal(services)
	restore := cb2FakeDockerOnPATH(t, string(jsonBytes), 0)
	defer restore()

	check := &schema.DockerComposeCheck{}
	result := CheckDockerComposeHealthy(check)
	if result.Passed {
		t.Fatal("expected failure for unhealthy service")
	}
}

func TestCoverageBoost2_CheckDockerComposeHealthy_WithComposeFile(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "[]", 0)
	defer restore()

	check := &schema.DockerComposeCheck{
		ComposeFile: "docker-compose.yml",
	}
	result := CheckDockerComposeHealthy(check)
	if result.Passed {
		t.Fatal("expected failure for empty services")
	}
}

func TestCoverageBoost2_CheckDockerComposeHealthy_ServiceFilter(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "db", State: "exited", Health: ""},
	}
	jsonBytes, _ := json.Marshal(services)
	restore := cb2FakeDockerOnPATH(t, string(jsonBytes), 0)
	defer restore()

	check := &schema.DockerComposeCheck{
		Services: []string{"web"},
	}
	result := CheckDockerComposeHealthy(check)
	if !result.Passed {
		t.Fatalf("expected pass when filtering to only healthy services, got: %v", result.Actual)
	}
}

// ============================================================
// CheckDockerContainerRunning via fake docker
// ============================================================

func TestCoverageBoost2_CheckDockerContainerRunning_Running(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "true", 0)
	defer restore()

	check := &schema.DockerContainerCheck{Name: "mycontainer"}
	result := CheckDockerContainerRunning(check)
	if !result.Passed {
		t.Fatalf("expected pass when docker inspect returns 'true', got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckDockerContainerRunning_NotRunning(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "false", 0)
	defer restore()

	check := &schema.DockerContainerCheck{Name: "mycontainer"}
	result := CheckDockerContainerRunning(check)
	if result.Passed {
		t.Fatal("expected failure when docker inspect returns 'false'")
	}
}

func TestCoverageBoost2_CheckDockerContainerRunning_DockerFails(t *testing.T) {
	restore := cb2FakeDockerOnPATH(t, "", 1)
	defer restore()

	check := &schema.DockerContainerCheck{Name: "mycontainer"}
	result := CheckDockerContainerRunning(check)
	if result.Passed {
		t.Fatal("expected failure when docker inspect fails")
	}
}

// ============================================================
// Mock TCP servers for DB checks
// ============================================================

// cb2StartTCPResponder starts a server that writes response immediately on connect (no read first).
// This matches protocols like MySQL where the server speaks first.
func cb2StartTCPResponder(t *testing.T, response []byte) (net.Listener, int) {
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
				if len(response) > 0 {
					c.Write(response)
				}
				// Drain any client data
				buf := make([]byte, 4096)
				c.Read(buf)
			}(conn)
		}
	}()
	return ln, ln.Addr().(*net.TCPAddr).Port
}

// cb2StartTCPReadFirstResponder starts a server that reads first, then writes.
// Use for Redis/Memcached/Postgres where the client sends a request first.
func cb2StartTCPReadFirstResponder(t *testing.T, response []byte) (net.Listener, int) {
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
				buf := make([]byte, 4096)
				c.SetDeadline(time.Now().Add(2 * time.Second))
				c.Read(buf)
				if len(response) > 0 {
					c.Write(response)
				}
			}(conn)
		}
	}()
	return ln, ln.Addr().(*net.TCPAddr).Port
}

func cb2StartTCPResponderMultiRead(t *testing.T, responses [][]byte) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 4096)
		for _, resp := range responses {
			conn.Read(buf)
			if len(resp) > 0 {
				conn.Write(resp)
			}
		}
	}()
	return ln, ln.Addr().(*net.TCPAddr).Port
}

// ============================================================
// CheckRedisPing tests
// ============================================================

func TestCoverageBoost2_CheckRedisPing_ConnectionRefused(t *testing.T) {
	check := &schema.RedisCheck{Host: "127.0.0.1", Port: 1}
	result := CheckRedisPing(check)
	if result.Passed {
		t.Fatal("expected failure for connection refused")
	}
	if result.Type != "redis_ping" {
		t.Fatalf("expected type redis_ping, got %q", result.Type)
	}
}

func TestCoverageBoost2_CheckRedisPing_ValidPong(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("+PONG\r\n"))
	defer ln.Close()

	check := &schema.RedisCheck{Host: "127.0.0.1", Port: port}
	result := CheckRedisPing(check)
	if !result.Passed {
		t.Fatalf("expected pass for +PONG response, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckRedisPing_InvalidResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("-ERR unknown\r\n"))
	defer ln.Close()

	check := &schema.RedisCheck{Host: "127.0.0.1", Port: port}
	result := CheckRedisPing(check)
	if result.Passed {
		t.Fatal("expected failure for non-PONG response")
	}
}

func TestCoverageBoost2_CheckRedisPing_EmptyResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, nil)
	defer ln.Close()

	check := &schema.RedisCheck{Host: "127.0.0.1", Port: port}
	result := CheckRedisPing(check)
	if result.Passed {
		t.Fatal("expected failure for empty/closed response")
	}
}

func TestCoverageBoost2_CheckRedisPing_DefaultHostPort(t *testing.T) {
	check := &schema.RedisCheck{} // uses defaults localhost:6379
	result := CheckRedisPing(check)
	_ = result // just check no panic
}

func TestCoverageBoost2_CheckRedisPing_WithPassword_Success(t *testing.T) {
	ln, port := cb2StartTCPResponderMultiRead(t, [][]byte{
		[]byte("+OK\r\n"),
		[]byte("+PONG\r\n"),
	})
	defer ln.Close()

	check := &schema.RedisCheck{Host: "127.0.0.1", Port: port, Password: "secret"}
	result := CheckRedisPing(check)
	if !result.Passed {
		t.Fatalf("expected pass with password auth, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckRedisPing_WithPassword_AuthFails(t *testing.T) {
	ln, port := cb2StartTCPResponderMultiRead(t, [][]byte{
		[]byte("-ERR invalid password\r\n"),
		nil,
	})
	defer ln.Close()

	check := &schema.RedisCheck{Host: "127.0.0.1", Port: port, Password: "wrong"}
	result := CheckRedisPing(check)
	if result.Passed {
		t.Fatal("expected failure for failed auth")
	}
}

// ============================================================
// CheckMemcachedVersion tests
// ============================================================

func TestCoverageBoost2_CheckMemcachedVersion_ConnectionRefused(t *testing.T) {
	check := &schema.MemcachedCheck{Host: "127.0.0.1", Port: 1}
	result := CheckMemcachedVersion(check)
	if result.Passed {
		t.Fatal("expected failure")
	}
	if result.Type != "memcached_version" {
		t.Fatalf("expected type memcached_version, got %q", result.Type)
	}
}

func TestCoverageBoost2_CheckMemcachedVersion_ValidResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("VERSION 1.6.12\r\n"))
	defer ln.Close()

	check := &schema.MemcachedCheck{Host: "127.0.0.1", Port: port}
	result := CheckMemcachedVersion(check)
	if !result.Passed {
		t.Fatalf("expected pass for VERSION response, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckMemcachedVersion_InvalidResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("ERROR\r\n"))
	defer ln.Close()

	check := &schema.MemcachedCheck{Host: "127.0.0.1", Port: port}
	result := CheckMemcachedVersion(check)
	if result.Passed {
		t.Fatal("expected failure for non-VERSION response")
	}
}

func TestCoverageBoost2_CheckMemcachedVersion_EmptyResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, nil)
	defer ln.Close()

	check := &schema.MemcachedCheck{Host: "127.0.0.1", Port: port}
	result := CheckMemcachedVersion(check)
	if result.Passed {
		t.Fatal("expected failure for empty response")
	}
}

func TestCoverageBoost2_CheckMemcachedVersion_DefaultHostPort(t *testing.T) {
	check := &schema.MemcachedCheck{}
	result := CheckMemcachedVersion(check)
	_ = result
}

// ============================================================
// CheckPostgresPing tests
// ============================================================

func TestCoverageBoost2_CheckPostgresPing_ConnectionRefused(t *testing.T) {
	check := &schema.PostgresCheck{Host: "127.0.0.1", Port: 1}
	result := CheckPostgresPing(check)
	if result.Passed {
		t.Fatal("expected failure")
	}
	if result.Type != "postgres_ping" {
		t.Fatalf("expected type postgres_ping, got %q", result.Type)
	}
}

func TestCoverageBoost2_CheckPostgresPing_SSLSupported(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("S"))
	defer ln.Close()

	check := &schema.PostgresCheck{Host: "127.0.0.1", Port: port}
	result := CheckPostgresPing(check)
	if !result.Passed {
		t.Fatalf("expected pass for 'S' response, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckPostgresPing_SSLNotSupported(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("N"))
	defer ln.Close()

	check := &schema.PostgresCheck{Host: "127.0.0.1", Port: port}
	result := CheckPostgresPing(check)
	if !result.Passed {
		t.Fatalf("expected pass for 'N' response, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckPostgresPing_ErrorResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("E"))
	defer ln.Close()

	check := &schema.PostgresCheck{Host: "127.0.0.1", Port: port}
	result := CheckPostgresPing(check)
	if !result.Passed {
		t.Fatalf("expected pass for 'E' (error) response, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckPostgresPing_InvalidResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("X"))
	defer ln.Close()

	check := &schema.PostgresCheck{Host: "127.0.0.1", Port: port}
	result := CheckPostgresPing(check)
	if result.Passed {
		t.Fatal("expected failure for invalid byte")
	}
}

func TestCoverageBoost2_CheckPostgresPing_EmptyResponse(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, nil)
	defer ln.Close()

	check := &schema.PostgresCheck{Host: "127.0.0.1", Port: port}
	result := CheckPostgresPing(check)
	if result.Passed {
		t.Fatal("expected failure for empty response")
	}
}

func TestCoverageBoost2_CheckPostgresPing_DefaultHostPort(t *testing.T) {
	check := &schema.PostgresCheck{}
	result := CheckPostgresPing(check)
	_ = result
}

// ============================================================
// CheckMySQLPing tests
// ============================================================

func TestCoverageBoost2_CheckMySQLPing_ConnectionRefused(t *testing.T) {
	check := &schema.MySQLCheck{Host: "127.0.0.1", Port: 1}
	result := CheckMySQLPing(check)
	if result.Passed {
		t.Fatal("expected failure")
	}
	if result.Type != "mysql_ping" {
		t.Fatalf("expected type mysql_ping, got %q", result.Type)
	}
}

func TestCoverageBoost2_CheckMySQLPing_ValidHandshake(t *testing.T) {
	handshake := []byte{0x4a, 0x00, 0x00, 0x00, 0x0a}
	ln, port := cb2StartTCPResponder(t, handshake)
	defer ln.Close()

	check := &schema.MySQLCheck{Host: "127.0.0.1", Port: port}
	result := CheckMySQLPing(check)
	if !result.Passed {
		t.Fatalf("expected pass for v10 handshake, got: %v", result.Actual)
	}
}

func TestCoverageBoost2_CheckMySQLPing_WrongProtocol(t *testing.T) {
	handshake := []byte{0x4a, 0x00, 0x00, 0x00, 0x09}
	ln, port := cb2StartTCPResponder(t, handshake)
	defer ln.Close()

	check := &schema.MySQLCheck{Host: "127.0.0.1", Port: port}
	result := CheckMySQLPing(check)
	if result.Passed {
		t.Fatal("expected failure for non-v10 protocol")
	}
}

func TestCoverageBoost2_CheckMySQLPing_ShortResponse(t *testing.T) {
	ln, port := cb2StartTCPResponder(t, []byte{0x01, 0x02})
	defer ln.Close()

	check := &schema.MySQLCheck{Host: "127.0.0.1", Port: port}
	result := CheckMySQLPing(check)
	if result.Passed {
		t.Fatal("expected failure for short response")
	}
}

func TestCoverageBoost2_CheckMySQLPing_EmptyResponse(t *testing.T) {
	ln, port := cb2StartTCPResponder(t, nil)
	defer ln.Close()

	check := &schema.MySQLCheck{Host: "127.0.0.1", Port: port}
	result := CheckMySQLPing(check)
	if result.Passed {
		t.Fatal("expected failure for empty response")
	}
}

func TestCoverageBoost2_CheckMySQLPing_DefaultHostPort(t *testing.T) {
	check := &schema.MySQLCheck{}
	result := CheckMySQLPing(check)
	_ = result
}

// ============================================================
// runTestOnce — more assertion branches via Runner.Run
// ============================================================

func cb2NewRunner(t *testing.T, tests []schema.Test) *Runner {
	return &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb2",
			Tests: tests,
		},
		Reporter:  &noopReporter{},
		ConfigDir: t.TempDir(),
	}
}

func TestCoverageBoost2_runTestOnce_FileExists_Pass(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "exists.txt")
	os.WriteFile(tmpFile, []byte("x"), 0644)

	r := cb2NewRunner(t, []schema.Test{
		{Name: "file", Run: "true", Expect: schema.Expect{FileExists: tmpFile}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost2_runTestOnce_FileExists_Fail(t *testing.T) {
	r := cb2NewRunner(t, []schema.Test{
		{Name: "file-miss", Run: "true", Expect: schema.Expect{FileExists: "/nonexistent/xyz/abc"}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Failed)
	}
}

func TestCoverageBoost2_runTestOnce_PortListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	r := cb2NewRunner(t, []schema.Test{
		{Name: "port", Run: "true", Expect: schema.Expect{
			PortListening: &schema.PortCheck{Port: port, Host: "127.0.0.1"},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost2_runTestOnce_Redis_via_Run(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("+PONG\r\n"))
	defer ln.Close()

	r := cb2NewRunner(t, []schema.Test{
		{Name: "redis", Run: "true", Expect: schema.Expect{
			Redis: &schema.RedisCheck{Host: "127.0.0.1", Port: port},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost2_runTestOnce_Postgres_via_Run(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("N"))
	defer ln.Close()

	r := cb2NewRunner(t, []schema.Test{
		{Name: "pg", Run: "true", Expect: schema.Expect{
			Postgres: &schema.PostgresCheck{Host: "127.0.0.1", Port: port},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost2_runTestOnce_MySQL_via_Run(t *testing.T) {
	handshake := []byte{0x4a, 0x00, 0x00, 0x00, 0x0a}
	ln, port := cb2StartTCPResponder(t, handshake)
	defer ln.Close()

	r := cb2NewRunner(t, []schema.Test{
		{Name: "mysql", Run: "true", Expect: schema.Expect{
			MySQL: &schema.MySQLCheck{Host: "127.0.0.1", Port: port},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost2_runTestOnce_Memcached_via_Run(t *testing.T) {
	ln, port := cb2StartTCPReadFirstResponder(t, []byte("VERSION 1.6.0\r\n"))
	defer ln.Close()

	r := cb2NewRunner(t, []schema.Test{
		{Name: "memcached", Run: "true", Expect: schema.Expect{
			Memcached: &schema.MemcachedCheck{Host: "127.0.0.1", Port: port},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost2_runTestOnce_HTTP_via_Run(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "ok body")
	}))
	defer ts.Close()

	statusCode := 200
	r := cb2NewRunner(t, []schema.Test{
		{Name: "http", Run: "true", Expect: schema.Expect{
			HTTP: &schema.HTTPCheck{
				URL:        ts.URL,
				StatusCode: &statusCode,
			},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost2_runTestOnce_DeepLink_via_Run(t *testing.T) {
	falseVal := false
	r := cb2NewRunner(t, []schema.Test{
		{Name: "deeplink", Run: "true", Expect: schema.Expect{
			DeepLink: &schema.DeepLinkCheck{
				URL:             "myapp://open",
				CheckAssetlinks: &falseVal,
				CheckAASA:       &falseVal,
				Tier:            "config-only",
			},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got passed=%d failed=%d", result.Passed, result.Failed)
	}
}

func TestCoverageBoost2_runTestOnce_WebSocket_via_Run(t *testing.T) {
	ln, addr := cbStartConnectOnlyWSServer(t)
	defer ln.Close()

	r := cb2NewRunner(t, []schema.Test{
		{Name: "ws", Run: "true", Expect: schema.Expect{
			WebSocket: &schema.WebSocketCheck{
				URL:     "ws://" + addr,
				Timeout: schema.Duration{Duration: 3 * time.Second},
			},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got passed=%d failed=%d", result.Passed, result.Failed)
	}
}

func TestCoverageBoost2_runTestOnce_DockerCompose_via_Run(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
	}
	jsonBytes, _ := json.Marshal(services)
	restore := cb2FakeDockerOnPATH(t, string(jsonBytes), 0)
	defer restore()

	r := cb2NewRunner(t, []schema.Test{
		{Name: "compose", Run: "true", Expect: schema.Expect{
			DockerCompose: &schema.DockerComposeCheck{},
		}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

// ============================================================
// CheckProcessRunning — cover the pgrep-missing branch
// ============================================================

func TestCoverageBoost2_CheckProcessRunning_PgrepMissing(t *testing.T) {
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", t.TempDir()) // empty dir — pgrep not found
	defer os.Setenv("PATH", oldPath)

	result := CheckProcessRunning("anything")
	if result.Passed {
		t.Fatal("expected failure when pgrep is missing")
	}
	if result.Type != "process_running" {
		t.Fatalf("expected type process_running, got %q", result.Type)
	}
}

// isDockerBinaryAvailable checks if docker exists on PATH.
func isDockerBinaryAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}
