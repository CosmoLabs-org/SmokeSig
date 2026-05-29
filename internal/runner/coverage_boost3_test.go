package runner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// ============================================================
// queryJaeger — 0% coverage
// ============================================================

func TestCoverageBoost3_queryJaeger_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"traceID":"abc","spans":[{"spanID":"1"},{"spanID":"2"}]}],"total":1,"limit":1,"offset":0,"errors":null}`)
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	result, err := queryJaeger(client, ts.URL+"/api/traces/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(result.Data))
	}
	if len(result.Data[0].Spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(result.Data[0].Spans))
	}
}

func TestCoverageBoost3_queryJaeger_HTTPError(t *testing.T) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err := queryJaeger(client, "http://127.0.0.1:1/api/traces/abc")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestCoverageBoost3_queryJaeger_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	_, err := queryJaeger(client, ts.URL+"/api/traces/abc")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCoverageBoost3_queryJaeger_InvalidURL(t *testing.T) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err := queryJaeger(client, "://bad url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// ============================================================
// wsSendMessage — large message paths (16-bit and 64-bit length)
// ============================================================

func TestCoverageBoost3_wsSendMessage_SmallMessage(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- wsSendMessage(client, "hello")
	}()

	buf := make([]byte, 256)
	n, err := server.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n < 2 {
		t.Fatalf("expected at least 2 bytes, got %d", n)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("wsSendMessage error: %v", err)
	}
}

func TestCoverageBoost3_wsSendMessage_MediumMessage_16bit(t *testing.T) {
	msg := strings.Repeat("x", 200)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- wsSendMessage(client, msg)
	}()

	buf := make([]byte, 4096)
	server.Read(buf)
	if err := <-errCh; err != nil {
		t.Fatalf("wsSendMessage error: %v", err)
	}
}

func TestCoverageBoost3_wsSendMessage_LargeMessage_64bit(t *testing.T) {
	msg := strings.Repeat("x", 70000)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- wsSendMessage(client, msg)
	}()

	// Drain server to prevent deadlock
	buf := make([]byte, 8192)
	for total := 0; total < 70010; {
		n, err := server.Read(buf)
		total += n
		if err != nil {
			break
		}
	}

	if err := <-errCh; err != nil {
		t.Fatalf("wsSendMessage error: %v", err)
	}
}

func TestCoverageBoost3_wsSendMessage_WriteError(t *testing.T) {
	server, client := net.Pipe()
	server.Close()
	defer client.Close()

	err := wsSendMessage(client, "hello")
	if err == nil {
		t.Fatal("expected write error when connection is closed")
	}
}

// ============================================================
// wsReadFrame — 64-bit extended length path
// ============================================================

func TestCoverageBoost3_wsReadFrame_64BitExtendedLength(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	payloadLen := 70000
	payload := make([]byte, payloadLen)
	for i := range payload {
		payload[i] = 'A'
	}

	go func() {
		header := []byte{0x81, 127}
		lenBytes := make([]byte, 8)
		l := uint64(payloadLen)
		for i := 7; i >= 0; i-- {
			lenBytes[i] = byte(l)
			l >>= 8
		}
		header = append(header, lenBytes...)
		server.Write(header)
		server.Write(payload)
		server.Close()
	}()

	msg, closed, err := wsReadFrame(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed {
		t.Fatal("expected not closed")
	}
	if len(msg) != payloadLen {
		t.Fatalf("expected %d bytes, got %d", payloadLen, len(msg))
	}
}

// ============================================================
// CheckWebSocket — ExpectMatches path
// ============================================================

func TestCoverageBoost3_CheckWebSocket_WithExpectMatches_Match(t *testing.T) {
	ln, addr := cbStartRawWSServer(t, "hello world 123")
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:           "ws://" + addr,
		Send:          "ping",
		ExpectMatches: `hello.*\d+`,
		Timeout:       schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	if !result.Passed {
		t.Fatalf("expected pass for matching regex, got: %v", result.Actual)
	}
}

func TestCoverageBoost3_CheckWebSocket_WithExpectMatches_NoMatch(t *testing.T) {
	ln, addr := cbStartRawWSServer(t, "hello world 123")
	defer ln.Close()

	check := &schema.WebSocketCheck{
		URL:           "ws://" + addr,
		Send:          "ping",
		ExpectMatches: `^goodbye`,
		Timeout:       schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocket(check)
	if result.Passed {
		t.Fatal("expected failure when regex does not match")
	}
}

// ============================================================
// CheckWebSocketWithTrace
// ============================================================

func TestCoverageBoost3_CheckWebSocketWithTrace_Success(t *testing.T) {
	ln, addr := cbStartConnectOnlyWSServer(t)
	defer ln.Close()

	// Use NewTraceContext to get a valid SpanContext
	tc := NewTraceContext()
	span := tc.NewSpan()

	check := &schema.WebSocketCheck{
		URL:     "ws://" + addr,
		Timeout: schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocketWithTrace(check, span)
	if !result.Passed {
		t.Fatalf("expected pass for CheckWebSocketWithTrace, got: %v", result.Actual)
	}
}

func TestCoverageBoost3_CheckWebSocketWithTrace_WithExistingHeaders(t *testing.T) {
	ln, addr := cbStartConnectOnlyWSServer(t)
	defer ln.Close()

	tc := NewTraceContext()
	span := tc.NewSpan()

	check := &schema.WebSocketCheck{
		URL:     "ws://" + addr,
		Headers: map[string]string{"X-Custom": "value"},
		Timeout: schema.Duration{Duration: 3 * time.Second},
	}
	result := CheckWebSocketWithTrace(check, span)
	if !result.Passed {
		t.Fatalf("expected pass, got: %v", result.Actual)
	}
}

// ============================================================
// runTestWithHooks — before_each and after_each lifecycle paths
// ============================================================

func TestCoverageBoost3_runTestWithHooks_BeforeEach(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1, Project: "cb3",
		Lifecycle: schema.LifecycleConfig{
			BeforeEach: []schema.LifecycleHook{
				{Command: "echo before_each_ran"},
			},
		},
		Tests: []schema.Test{
			{Name: "with-before", Run: "true", Expect: schema.Expect{}},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost3_runTestWithHooks_AfterEach(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1, Project: "cb3",
		Lifecycle: schema.LifecycleConfig{
			AfterEach: []schema.LifecycleHook{
				{Command: "echo after_each_ran"},
			},
		},
		Tests: []schema.Test{
			{Name: "with-after", Run: "true", Expect: schema.Expect{}},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

func TestCoverageBoost3_runTestWithHooks_BeforeEach_Fails(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1, Project: "cb3",
		Lifecycle: schema.LifecycleConfig{
			BeforeEach: []schema.LifecycleHook{
				{Command: "exit 1"},
			},
		},
		Tests: []schema.Test{
			{Name: "before-fail", Run: "true", Expect: schema.Expect{}},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error at suite level: %v", err)
	}
	// Test should fail due to before_each failure
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got passed=%d failed=%d", result.Passed, result.Failed)
	}
}

// ============================================================
// RunLifecycleHooks — background + EnvPass path
// ============================================================

func TestCoverageBoost3_RunLifecycleHooks_Background_EnvPass(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{
			Command:    "printf 'BG_KEY=bg_value\\n'",
			Background: true,
			EnvPass:    true,
		},
	}
	env, err := RunLifecycleHooks(context.Background(), hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = env
}

// ============================================================
// runTestOnce — remaining uncovered assertion branches
// ============================================================

func TestCoverageBoost3_runTestOnce_URLReachable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb3",
			Tests: []schema.Test{
				{Name: "url-reachable", Run: "true", Expect: schema.Expect{
					URLReachable: &schema.URLReachableCheck{URL: ts.URL},
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

func TestCoverageBoost3_runTestOnce_VersionCheck(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb3",
			Tests: []schema.Test{
				{Name: "version", Run: "echo 1.2.3", Expect: schema.Expect{
					VersionCheck: &schema.VersionCheck{
						Command: "echo 1.2.3",
						Pattern: `\d+\.\d+\.\d+`,
					},
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
	_ = result
}

func TestCoverageBoost3_runTestOnce_ResponseTime_Pass(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb3",
			Tests: []schema.Test{
				{Name: "response-time", Run: "true", Expect: schema.Expect{
					ResponseTimeMs: intPtrCb3(5000),
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
		t.Fatalf("expected 1 passed (fast cmd under 5000ms), got %d", result.Passed)
	}
}

func TestCoverageBoost3_runTestOnce_ResponseTime_Fail(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb3",
			Tests: []schema.Test{
				{Name: "response-time-fail", Run: "sleep 0.1", Expect: schema.Expect{
					ResponseTimeMs: intPtrCb3(1),
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
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed for slow cmd, got passed=%d failed=%d", result.Passed, result.Failed)
	}
}

func TestCoverageBoost3_runTestOnce_JSONField(t *testing.T) {
	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb3",
			Tests: []schema.Test{
				{Name: "json-field", Run: `printf '{"name":"alice","age":30}\n'`, Expect: schema.Expect{
					JSONField: &schema.JSONFieldCheck{
						Path:   "name",
						Equals: "alice",
					},
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

func TestCoverageBoost3_runTestOnce_ExitCode_NonZero(t *testing.T) {
	exitCode := 2
	r := &Runner{
		Config: &schema.SmokeConfig{
			Version: 1, Project: "cb3",
			Tests: []schema.Test{
				{Name: "exit-code", Run: "exit 2", Expect: schema.Expect{
					ExitCode: &exitCode,
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
		t.Fatalf("expected 1 passed for expected exit code 2, got %d", result.Passed)
	}
}

// ============================================================
// Run() — before_all and after_all lifecycle hook paths
// ============================================================

func TestCoverageBoost3_Run_BeforeAll_Fails(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1, Project: "cb3",
		Lifecycle: schema.LifecycleConfig{
			BeforeAll: []schema.LifecycleHook{
				{Command: "exit 1"},
			},
		},
		Tests: []schema.Test{
			{Name: "test", Run: "true"},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	_, err := r.Run(RunOptions{})
	if err == nil {
		t.Fatal("expected error when before_all fails")
	}
	if !strings.Contains(err.Error(), "before_all") {
		t.Fatalf("expected 'before_all' in error, got: %v", err)
	}
}

func TestCoverageBoost3_Run_AfterAll_Runs(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1, Project: "cb3",
		Lifecycle: schema.LifecycleConfig{
			AfterAll: []schema.LifecycleHook{
				{Command: "echo after_all_ran"},
			},
		},
		Tests: []schema.Test{
			{Name: "test", Run: "true"},
		},
	}
	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", result.Passed)
	}
}

// ============================================================
// helpers
// ============================================================

func intPtrCb3(n int) *int { return &n }
