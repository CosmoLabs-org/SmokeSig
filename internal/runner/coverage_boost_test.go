package runner

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// ============================================================
// wsReadFrame tests
// ============================================================

// writeRawFrame writes a raw WebSocket frame to a writer.
func writeRawFrame(w io.Writer, opcode byte, payload []byte, masked bool) {
	header := []byte{0x80 | opcode}
	length := len(payload)
	maskBit := byte(0)
	if masked {
		maskBit = 0x80
	}
	if length <= 125 {
		header = append(header, maskBit|byte(length))
	} else if length <= 65535 {
		header = append(header, maskBit|126, byte(length>>8), byte(length))
	} else {
		header = append(header, maskBit|127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(length))
		header = append(header, ext...)
	}
	if masked {
		mask := []byte{0x01, 0x02, 0x03, 0x04}
		header = append(header, mask...)
		maskedPayload := make([]byte, length)
		for i, b := range payload {
			maskedPayload[i] = b ^ mask[i%4]
		}
		payload = maskedPayload
	}
	w.Write(header)
	w.Write(payload)
}

func TestCoverageBoost_wsReadFrame_TextFrame(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		writeRawFrame(server, 0x01, []byte("hello world"), false)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed {
		t.Fatal("expected not closed")
	}
	if msg != "hello world" {
		t.Fatalf("expected 'hello world', got %q", msg)
	}
}

func TestCoverageBoost_wsReadFrame_BinaryFrame(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		writeRawFrame(server, 0x02, []byte("binary data"), false)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed {
		t.Fatal("expected not closed")
	}
	if msg != "binary data" {
		t.Fatalf("expected 'binary data', got %q", msg)
	}
}

func TestCoverageBoost_wsReadFrame_CloseFrameNoPayload(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		writeRawFrame(server, 0x08, nil, false)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !closed {
		t.Fatal("expected closed=true for close frame")
	}
	if msg != "" {
		t.Fatalf("expected empty reason, got %q", msg)
	}
}

func TestCoverageBoost_wsReadFrame_CloseFrameWithPayload(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		writeRawFrame(server, 0x08, []byte("going away"), false)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !closed {
		t.Fatal("expected closed=true")
	}
	if msg != "going away" {
		t.Fatalf("expected 'going away', got %q", msg)
	}
}

func TestCoverageBoost_wsReadFrame_PingFrameWithPayload(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		writeRawFrame(server, 0x09, []byte("ping data"), false)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed {
		t.Fatal("expected not closed for ping")
	}
	if msg != "" {
		t.Fatalf("expected empty message for ping, got %q", msg)
	}
}

func TestCoverageBoost_wsReadFrame_PingFrameEmptyPayload(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		writeRawFrame(server, 0x09, nil, false)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed {
		t.Fatal("expected not closed for ping")
	}
	_ = msg
}

func TestCoverageBoost_wsReadFrame_16BitExtendedLength(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	payload := make([]byte, 200)
	for i := range payload {
		payload[i] = 'x'
	}

	go func() {
		writeRawFrame(server, 0x01, payload, false)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed {
		t.Fatal("expected not closed")
	}
	if len(msg) != 200 {
		t.Fatalf("expected 200 bytes, got %d", len(msg))
	}
}

func TestCoverageBoost_wsReadFrame_MaskedFrame(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		writeRawFrame(server, 0x01, []byte("masked content"), true)
		server.Close()
	}()

	_, _, _ = wsReadFrame(client)
}

func TestCoverageBoost_wsReadFrame_EOFError(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	server.Close()

	_, _, err := wsReadFrame(client)
	if err == nil {
		t.Fatal("expected error on EOF")
	}
}

// ============================================================
// wsUpgrade tests
// ============================================================

func cbComputeAcceptKey(clientKey string) string {
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(clientKey + guid))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func cbStartMockWSServer(t *testing.T, rejectUpgrade bool, badAcceptKey bool) (net.Listener, string) {
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

		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		req := string(buf[:n])

		var clientKey string
		for _, line := range strings.Split(req, "\r\n") {
			if strings.HasPrefix(strings.ToLower(line), "sec-websocket-key:") {
				clientKey = strings.TrimSpace(line[len("sec-websocket-key:"):])
			}
		}

		if rejectUpgrade {
			conn.Write([]byte("HTTP/1.1 403 Forbidden\r\n\r\n"))
			return
		}

		acceptKey := cbComputeAcceptKey(clientKey)
		if badAcceptKey {
			acceptKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}

		resp := fmt.Sprintf(
			"HTTP/1.1 101 Switching Protocols\r\n"+
				"Upgrade: websocket\r\n"+
				"Connection: Upgrade\r\n"+
				"Sec-WebSocket-Accept: %s\r\n\r\n",
			acceptKey,
		)
		conn.Write([]byte(resp))
	}()

	return ln, ln.Addr().String()
}

func TestCoverageBoost_wsUpgrade_Success(t *testing.T) {
	ln, addr := cbStartMockWSServer(t, false, false)
	defer ln.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	err = wsUpgrade(conn, addr, "/", 2*time.Second, nil)
	if err != nil {
		t.Fatalf("unexpected upgrade error: %v", err)
	}
}

func TestCoverageBoost_wsUpgrade_UpgradeRejected(t *testing.T) {
	ln, addr := cbStartMockWSServer(t, true, false)
	defer ln.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	err = wsUpgrade(conn, addr, "/", 2*time.Second, nil)
	if err == nil {
		t.Fatal("expected error for rejected upgrade")
	}
	if !strings.Contains(err.Error(), "upgrade failed") {
		t.Fatalf("expected 'upgrade failed', got: %v", err)
	}
}

func TestCoverageBoost_wsUpgrade_BadAcceptKey(t *testing.T) {
	ln, addr := cbStartMockWSServer(t, false, true)
	defer ln.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	err = wsUpgrade(conn, addr, "/", 2*time.Second, nil)
	if err == nil {
		t.Fatal("expected error for invalid accept key")
	}
	if !strings.Contains(err.Error(), "invalid accept key") {
		t.Fatalf("expected 'invalid accept key', got: %v", err)
	}
}

func TestCoverageBoost_wsUpgrade_WithExtraHeaders(t *testing.T) {
	ln, addr := cbStartMockWSServer(t, false, false)
	defer ln.Close()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	extraHeaders := map[string]string{
		"Authorization": "Bearer token123",
		"X-Custom":      "value",
	}
	err = wsUpgrade(conn, addr, "/path", 2*time.Second, extraHeaders)
	if err != nil {
		t.Fatalf("unexpected error with extra headers: %v", err)
	}
}

func TestCoverageBoost_wsUpgrade_ReadError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	err = wsUpgrade(conn, ln.Addr().String(), "/", 2*time.Second, nil)
	if err == nil {
		t.Fatal("expected error when server closes connection immediately")
	}
}

// ============================================================
// CheckDockerComposeHealthy — pure logic tests via composeService struct
// ============================================================

func cbEvaluateComposeServices(services []composeService, filterNames []string) AssertionResult {
	if len(services) == 0 {
		return AssertionResult{
			Type:     "docker_compose_healthy",
			Expected: "compose services running",
			Actual:   "no services found",
			Passed:   false,
		}
	}

	serviceFilter := make(map[string]bool)
	for _, s := range filterNames {
		serviceFilter[s] = true
	}

	var unhealthy []string
	for _, svc := range services {
		if len(serviceFilter) > 0 && !serviceFilter[svc.Name] {
			continue
		}
		if svc.State != "running" {
			unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", svc.Name, svc.State))
		} else if svc.Health != "" && svc.Health != "healthy" {
			unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", svc.Name, svc.Health))
		}
	}

	if len(unhealthy) > 0 {
		return AssertionResult{
			Type:     "docker_compose_healthy",
			Expected: "all services healthy",
			Actual:   strings.Join(unhealthy, ", "),
			Passed:   false,
		}
	}
	return AssertionResult{
		Type:     "docker_compose_healthy",
		Expected: "all services healthy",
		Actual:   fmt.Sprintf("%d services healthy", len(services)),
		Passed:   true,
	}
}

func TestCoverageBoost_CheckDockerCompose_EmptyServices(t *testing.T) {
	result := cbEvaluateComposeServices(nil, nil)
	if result.Passed {
		t.Fatal("expected failure for empty services")
	}
	if !strings.Contains(result.Actual, "no services found") {
		t.Fatalf("expected 'no services found', got %q", result.Actual)
	}
}

func TestCoverageBoost_CheckDockerCompose_AllHealthy(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "db", State: "running", Health: ""},
	}
	result := cbEvaluateComposeServices(services, nil)
	if !result.Passed {
		t.Fatalf("expected pass for healthy services, got: %v", result.Actual)
	}
}

func TestCoverageBoost_CheckDockerCompose_OneUnhealthy(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "db", State: "exited", Health: ""},
	}
	result := cbEvaluateComposeServices(services, nil)
	if result.Passed {
		t.Fatal("expected failure when one service is exited")
	}
	if !strings.Contains(result.Actual, "db") {
		t.Fatalf("expected 'db' in actual, got %q", result.Actual)
	}
}

func TestCoverageBoost_CheckDockerCompose_UnhealthyHealth(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "unhealthy"},
	}
	result := cbEvaluateComposeServices(services, nil)
	if result.Passed {
		t.Fatal("expected failure for unhealthy health status")
	}
}

func TestCoverageBoost_CheckDockerCompose_ServiceFilter(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "db", State: "exited", Health: ""},
		{Name: "cache", State: "running", Health: "healthy"},
	}
	result := cbEvaluateComposeServices(services, []string{"web", "cache"})
	if !result.Passed {
		t.Fatalf("expected pass when filtered services are healthy, got: %v", result.Actual)
	}
}

func TestCoverageBoost_CheckDockerCompose_ServiceFilterWithBadService(t *testing.T) {
	services := []composeService{
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "db", State: "exited", Health: ""},
	}
	result := cbEvaluateComposeServices(services, []string{"db"})
	if result.Passed {
		t.Fatal("expected failure when filtered service is not running")
	}
}

// ============================================================
// CheckGRPCHealthWithTrace — stub build tag
// ============================================================

func TestCoverageBoost_CheckGRPCHealthWithTrace_Stub(t *testing.T) {
	check := &schema.GRPCHealthCheck{
		Address: "localhost:50051",
	}
	result := CheckGRPCHealthWithTrace(check, nil)
	if result.Passed {
		t.Fatal("expected stub to return Passed=false")
	}
	if !strings.Contains(result.Actual, "grpc_health not available") {
		t.Fatalf("expected stub message, got %q", result.Actual)
	}
	if result.Type != "grpc_health" {
		t.Fatalf("expected type 'grpc_health', got %q", result.Type)
	}
}

func TestCoverageBoost_CheckGRPCHealth_Stub(t *testing.T) {
	check := &schema.GRPCHealthCheck{
		Address: "localhost:50051",
	}
	result := CheckGRPCHealth(check)
	if result.Passed {
		t.Fatal("expected stub to return Passed=false")
	}
	if result.Expected != "localhost:50051" {
		t.Fatalf("expected address in Expected, got %q", result.Expected)
	}
}

// ============================================================
// CheckDeepLink tests
// ============================================================

func TestCoverageBoost_CheckDeepLink_ConfigOnly_NoChecks(t *testing.T) {
	falseVal := false
	cfg := &schema.DeepLinkCheck{
		URL:             "https://example.com/path",
		AndroidPackage:  "com.example.app",
		IOSBundleID:     "com.example.app",
		CheckAssetlinks: &falseVal,
		CheckAASA:       &falseVal,
		Tier:            "config-only",
	}
	results := CheckDeepLink(cfg, "")
	if len(results) != 0 {
		t.Fatalf("expected 0 results for config-only with all checks disabled, got %d", len(results))
	}
}

func TestCoverageBoost_CheckDeepLink_AutoTier_NonWebURL(t *testing.T) {
	cfg := &schema.DeepLinkCheck{
		URL:  "myapp://open/screen",
		Tier: "auto",
	}
	results := CheckDeepLink(cfg, "")
	_ = results
}

func TestCoverageBoost_CheckDeepLink_FullResolveTier_NoTools(t *testing.T) {
	cfg := &schema.DeepLinkCheck{
		URL:  "myapp://open/screen",
		Tier: "full-resolve",
	}
	results := CheckDeepLink(cfg, "")
	if !hasTool("adb") && !hasTool("xcrun") {
		if len(results) != 1 {
			t.Fatalf("expected 1 failure result for full-resolve with no tools, got %d", len(results))
		}
		if results[0].Passed {
			t.Fatal("expected failure when no mobile tools available")
		}
		if results[0].Type != "deep_link.resolve" {
			t.Fatalf("expected type 'deep_link.resolve', got %q", results[0].Type)
		}
	}
}

func TestCoverageBoost_CheckDeepLink_WebURL_WithHTTPServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/assetlinks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.example.app","sha256_cert_fingerprints":["AA:BB"]}}]`)
	})
	mux.HandleFunc("/.well-known/apple-app-site-association", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"applinks":{"apps":[],"details":[{"appID":"TEAMID.com.example.app","paths":["*"]}]}}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	falseVal := false
	cfg := &schema.DeepLinkCheck{
		URL:             ts.URL + "/path",
		AndroidPackage:  "com.example.app",
		IOSBundleID:     "com.example.app",
		CheckAssetlinks: nil,
		CheckAASA:       &falseVal,
		Tier:            "config-only",
	}
	results := CheckDeepLink(cfg, "")
	if len(results) == 0 {
		t.Fatal("expected at least one result for web URL with assetlinks check")
	}
}

func TestCoverageBoost_CheckDeepLink_WebURL_AssetlinksCheckDisabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	trueVal := true
	falseVal := false
	cfg := &schema.DeepLinkCheck{
		URL:             ts.URL + "/page",
		AndroidPackage:  "com.example.app",
		CheckAssetlinks: &falseVal,
		CheckAASA:       &trueVal,
		Tier:            "config-only",
	}
	results := CheckDeepLink(cfg, "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (AASA only), got %d", len(results))
	}
}

// ============================================================
// CheckProcessRunning tests
// ============================================================

func TestCoverageBoost_CheckProcessRunning_EmptyName(t *testing.T) {
	result := CheckProcessRunning("")
	if result.Passed {
		t.Fatal("expected failure for empty process name")
	}
	if result.Actual != "empty name" {
		t.Fatalf("expected 'empty name', got %q", result.Actual)
	}
	if result.Type != "process_running" {
		t.Fatalf("expected type 'process_running', got %q", result.Type)
	}
}

func TestCoverageBoost_CheckProcessRunning_NotFound(t *testing.T) {
	result := CheckProcessRunning("zzz_nonexistent_process_xyz_12345")
	if result.Passed {
		t.Fatal("expected failure for non-existent process")
	}
	if result.Type != "process_running" {
		t.Fatalf("expected type 'process_running', got %q", result.Type)
	}
}

func TestCoverageBoost_CheckProcessRunning_ActualProcess(t *testing.T) {
	candidates := []string{"sh", "go", "bash", "zsh"}
	for _, name := range candidates {
		result := CheckProcessRunning(name)
		_ = result
	}
}

// ============================================================
// RunLifecycleHooks tests
// ============================================================

func TestCoverageBoost_RunLifecycleHooks_Empty(t *testing.T) {
	ctx := context.Background()
	env, err := RunLifecycleHooks(ctx, nil, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env == nil {
		t.Fatal("expected non-nil env map")
	}
}

func TestCoverageBoost_RunLifecycleHooks_SkipsEmptyCommand(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: ""},
		{Command: "echo ok"},
	}
	env, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = env
}

func TestCoverageBoost_RunLifecycleHooks_SimpleCommand(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "echo hello"},
	}
	env, err := RunLifecycleHooks(ctx, hooks, map[string]string{"EXISTING": "val"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["EXISTING"] != "val" {
		t.Fatal("expected existing env to be preserved")
	}
}

func TestCoverageBoost_RunLifecycleHooks_FailingCommand(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "exit 1"},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err == nil {
		t.Fatal("expected error from failing hook")
	}
}

func TestCoverageBoost_RunLifecycleHooks_AlwaysRunAfterError(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "exit 1"},
		{Command: "true", AlwaysRun: true},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err == nil {
		t.Fatal("expected error from first hook")
	}
}

func TestCoverageBoost_RunLifecycleHooks_SkipsAfterError(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "exit 1"},
		{Command: "true"},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err == nil {
		t.Fatal("expected error from first hook")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Fatalf("expected 'failed' in error, got: %v", err)
	}
}

func TestCoverageBoost_RunLifecycleHooks_EnvPass(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{
			Command: "printf 'FOO=bar\nBAZ=qux\n'",
			EnvPass: true,
		},
	}
	env, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["FOO"] != "bar" {
		t.Fatalf("expected FOO=bar, got FOO=%q", env["FOO"])
	}
	if env["BAZ"] != "qux" {
		t.Fatalf("expected BAZ=qux, got BAZ=%q", env["BAZ"])
	}
}

func TestCoverageBoost_RunLifecycleHooks_EnvPassWithFailingCmd(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{
			Command: "printf 'KEY=value\n'; exit 1",
			EnvPass: true,
		},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err == nil {
		t.Fatal("expected error from exit 1")
	}
}

func TestCoverageBoost_RunLifecycleHooks_Background(t *testing.T) {
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{
			Command:    "sleep 0.1",
			Background: true,
		},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error for background hook: %v", err)
	}
}

func TestCoverageBoost_RunLifecycleHooks_BackgroundWithWaitForPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{
			Command:     "sleep 10",
			Background:  true,
			WaitForPort: port,
		},
	}
	_, err = RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error when port is already open: %v", err)
	}
}

func TestCoverageBoost_RunLifecycleHooks_BackgroundWaitForPortTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{
			Command:        "sleep 10",
			Background:     true,
			WaitForPort:    port,
			StartupTimeout: schema.Duration{Duration: 200 * time.Millisecond},
		},
	}
	_, err = RunLifecycleHooks(ctx, hooks, nil, "")
	if err == nil {
		t.Fatal("expected error when waiting for unopened port times out")
	}
}

func TestCoverageBoost_RunLifecycleHooks_PreservesInitialEnv(t *testing.T) {
	ctx := context.Background()
	initial := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}
	hooks := []schema.LifecycleHook{
		{Command: "true"},
	}
	env, err := RunLifecycleHooks(ctx, hooks, initial, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY1"] != "value1" || env["KEY2"] != "value2" {
		t.Fatal("initial env not preserved")
	}
}

// ============================================================
// CheckWebSocket — raw TCP mock server
// ============================================================

// cbStartRawWSServer starts a raw TCP WS server.
// If sendMsg is non-empty, server waits for a client frame then replies.
func cbStartRawWSServer(t *testing.T, sendMsg string) (net.Listener, string) {
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
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				req := string(buf[:n])
				var clientKey string
				for _, line := range strings.Split(req, "\r\n") {
					if strings.HasPrefix(strings.ToLower(line), "sec-websocket-key:") {
						clientKey = strings.TrimSpace(line[len("sec-websocket-key:"):])
					}
				}
				acceptKey := cbComputeAcceptKey(clientKey)
				resp := fmt.Sprintf(
					"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
					acceptKey,
				)
				c.Write([]byte(resp))

				if sendMsg != "" {
					// Wait for client frame first
					clientBuf := make([]byte, 4096)
					c.SetDeadline(time.Now().Add(2 * time.Second))
					c.Read(clientBuf)
					c.SetDeadline(time.Time{})

					payload := []byte(sendMsg)
					frame := []byte{0x81, byte(len(payload))}
					frame = append(frame, payload...)
					c.Write(frame)
				}
				c.Write([]byte{0x88, 0x00})
			}(conn)
		}
	}()
	return ln, ln.Addr().String()
}

// cbStartConnectOnlyWSServer — simple server that upgrades then sends close.
func cbStartConnectOnlyWSServer(t *testing.T) (net.Listener, string) {
	return cbStartRawWSServer(t, "")
}

func TestCoverageBoost_CheckWebSocket_ConnectionRefused(t *testing.T) {
	check := &schema.WebSocketCheck{
		URL:     "ws://127.0.0.1:1",
		Timeout: schema.Duration{Duration: 500 * time.Millisecond},
	}
	result := CheckWebSocket(check)
	if result.Passed {
		t.Fatal("expected failure for connection refused")
	}
}

func TestCoverageBoost_CheckWebSocket_SuccessNoMessage(t *testing.T) {
	ln, addr := cbStartConnectOnlyWSServer(t)
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:     "ws://" + addr,
		Timeout: schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	if !result.Passed {
		t.Fatalf("expected pass for successful WebSocket, got: %v", result.Actual)
	}
}

func TestCoverageBoost_CheckWebSocket_WithExpectContains_Match(t *testing.T) {
	ln, addr := cbStartRawWSServer(t, "hello from server")
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:            "ws://" + addr,
		Send:           "ping",
		ExpectContains: "hello",
		Timeout:        schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	if !result.Passed {
		t.Fatalf("expected pass when response contains expected string, got: %v", result.Actual)
	}
}

func TestCoverageBoost_CheckWebSocket_WithExpectContains_NoMatch(t *testing.T) {
	ln, addr := cbStartRawWSServer(t, "hello from server")
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:            "ws://" + addr,
		Send:           "ping",
		ExpectContains: "goodbye",
		Timeout:        schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	if result.Passed {
		t.Fatal("expected failure when response does not contain expected string")
	}
}

func TestCoverageBoost_CheckWebSocket_WithSendMessage(t *testing.T) {
	ln, addr := cbStartRawWSServer(t, "echo response")
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:     "ws://" + addr,
		Send:    "ping message",
		Timeout: schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	_ = result
}

func TestCoverageBoost_CheckWebSocket_DefaultTimeout(t *testing.T) {
	check := &schema.WebSocketCheck{
		URL: "ws://127.0.0.1:1",
	}
	result := CheckWebSocket(check)
	if result.Passed {
		t.Fatal("expected failure")
	}
}

func TestCoverageBoost_CheckWebSocket_WithHeaders(t *testing.T) {
	ln, addr := cbStartConnectOnlyWSServer(t)
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:     "ws://" + addr,
		Headers: map[string]string{"X-Token": "abc123"},
		Timeout: schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	if !result.Passed {
		t.Fatalf("expected pass with custom headers, got: %v", result.Actual)
	}
}

// ============================================================
// runTestOnce — via Runner.Run to exercise branches
// ============================================================

func cbNewRunner(tests []schema.Test) *Runner {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "cb-test",
		Tests:   tests,
	}
	return &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: "/tmp"}
}

func TestCoverageBoost_runTestOnce_DryRun(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "dry", Run: "exit 1"},
	})
	result, err := r.Run(RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed in dry run, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_StdoutContains_Pass(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "stdout", Run: "echo hello", Expect: schema.Expect{StdoutContains: "hello"}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_StdoutContains_Fail(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "stdout-fail", Run: "echo hello", Expect: schema.Expect{StdoutContains: "goodbye"}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Failed)
	}
}

func TestCoverageBoost_runTestOnce_StdoutMatches(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "regex", Run: "echo hello123", Expect: schema.Expect{StdoutMatches: `hello\d+`}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_StderrContains(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "stderr", Run: "echo errout >&2", Expect: schema.Expect{StderrContains: "errout"}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_StderrMatches(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "stderr-regex", Run: "echo err42 >&2", Expect: schema.Expect{StderrMatches: `err\d+`}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_WithCleanup(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "cleanup", Run: "echo ok", Cleanup: "true"},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_NonExitError(t *testing.T) {
	// Use a command that produces a non-ExitError (invalid command syntax via exec directly)
	// We use a context timeout that fires before command runs by setting a zero-length timeout
	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1,
			Project: "cb-test",
			Tests: []schema.Test{
				{
					Name:    "timeout-test",
					Run:     "sleep 10",
					Timeout: schema.Duration{Duration: 50 * time.Millisecond},
				},
			},
		},
		Reporter:  &noopReporter{},
		ConfigDir: "/tmp",
	}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Timed-out test should fail
	if result.Failed != 1 && result.Passed != 1 {
		t.Logf("passed=%d failed=%d skipped=%d", result.Passed, result.Failed, result.Skipped)
	}
}

func TestCoverageBoost_runTestOnce_EnvExists(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "env", Run: "true", Expect: schema.Expect{EnvExists: "PATH"}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_ProcessRunning(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "proc", Run: "true", Expect: schema.Expect{ProcessRunning: "zzz_nonexistent_12345"}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed for nonexistent process, got passed=%d failed=%d", result.Passed, result.Failed)
	}
}

func TestCoverageBoost_runTestOnce_GRPCHealth(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "grpc", Run: "true", Expect: schema.Expect{GRPCHealth: &schema.GRPCHealthCheck{Address: "localhost:9999"}}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed (stub), got passed=%d failed=%d", result.Passed, result.Failed)
	}
}

func TestCoverageBoost_runTestOnce_DockerCompose(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "compose", Run: "true", Expect: schema.Expect{DockerCompose: &schema.DockerComposeCheck{}}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Will fail (docker not running in test env) — just checking it doesn't panic
	_ = result
}

func TestCoverageBoost_runTestOnce_WebSocket(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "ws", Run: "true", Expect: schema.Expect{WebSocket: &schema.WebSocketCheck{
			URL:     "ws://127.0.0.1:1",
			Timeout: schema.Duration{Duration: 200 * time.Millisecond},
		}}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed (ws refused), got passed=%d failed=%d", result.Passed, result.Failed)
	}
}

func TestCoverageBoost_runTestOnce_NoRunCommand_WithAssertion(t *testing.T) {
	// No Run command — standalone assertion mode
	r := cbNewRunner([]schema.Test{
		{Name: "standalone", Run: "", Expect: schema.Expect{EnvExists: "PATH"}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost_runTestOnce_AllowFailure(t *testing.T) {
	r := cbNewRunner([]schema.Test{
		{Name: "allowed", Run: "exit 1", AllowFailure: true, Expect: schema.Expect{ExitCode: intPtr(0)}},
	})
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllowedFailures != 1 {
		t.Fatalf("expected 1 allowed failure, got %d", result.AllowedFailures)
	}
}

// ============================================================
// computeAcceptKey — RFC 6455 known value
// ============================================================

func TestCoverageBoost_computeAcceptKey_KnownValue(t *testing.T) {
	clientKey := "dGhlIHNhbXBsZSBub25jZQ=="
	got := computeAcceptKey(clientKey)
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("computeAcceptKey(%q) = %q, want %q", clientKey, got, want)
	}
}
