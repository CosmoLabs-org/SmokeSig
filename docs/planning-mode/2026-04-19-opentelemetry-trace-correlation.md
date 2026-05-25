---
completed: "2026-04-19"
created: "2026-04-19"
goals_completed: 52
goals_total: 52
status: COMPLETED
title: OpenTelemetry Trace Correlation — Implementation Plan
---

# OpenTelemetry Trace Correlation — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add W3C trace context propagation to cosmo-smoke's network assertions and a Jaeger-based trace verification assertion.

**Architecture:** Global `otel` config block enables trace context generation. Runner creates a suite-level traceID, per-test spanIDs. Traceparent injected into HTTP/gRPC/WebSocket requests at construction time. Dedicated `otel_trace` assertion polls Jaeger query API.

**Tech Stack:** Go stdlib only (crypto/rand, net/http, encoding/json, encoding/hex). No OTel SDK.

**Spec:** `docs/brainstorming/2026-04-18-opentelemetry-trace-correlation.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `internal/runner/trace.go` | TraceContext struct, W3C traceparent generation |
| Create | `internal/runner/trace_test.go` | Tests for trace context |
| Create | `internal/runner/assertion_otel.go` | CheckOTelTrace + Jaeger API client |
| Create | `internal/runner/assertion_otel_test.go` | Tests with httptest mock Jaeger |
| Modify | `internal/schema/schema.go` | OTelConfig + OTelTraceCheck structs |
| Modify | `internal/schema/validate.go` | Validation for otel config fields |
| Modify | `internal/runner/runner.go` | Init trace context, pass to assertions |
| Modify | `internal/runner/assertion_network.go` | Inject traceparent into HTTP requests |
| Modify | `internal/runner/assertion_grpc.go` | Inject traceparent into gRPC metadata |
| Modify | `internal/runner/assertion_ws.go` | Inject traceparent into WS handshake |
| Modify | `internal/runner/assertion_grpc_stub.go` | No-op trace injection for stub build |
| Modify | `cmd/run.go` | --otel-collector and --no-otel flags |

**Task dependencies:** Chunk 4 (Tasks 5-7) MUST complete before Chunk 5 (Task 8). Chunk 5 requires `CheckHTTPWithTrace`, `CheckWebSocketWithTrace`, and `CheckGRPCHealthWithTrace` to be defined.

---

## Chunk 1: Schema & Validation

### Task 1: Add OTelConfig and OTelTraceCheck to schema

**Files:**
- Modify: `internal/schema/schema.go:15-23` (SmokeConfig struct, add OTel field)
- Modify: `internal/schema/schema.go:99` (Expect struct, add OTelTrace field)
- Modify: `internal/schema/schema.go` (WebSocketCheck: add Headers field)
- Modify: `internal/schema/schema.go` (GRPCHealthCheck: add Metadata field, `yaml:"-"` tag)
- Test: `internal/schema/schema_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/schema/schema_test.go`:

```go
func TestSmokeConfig_OTel(t *testing.T) {
	yaml := `
version: 1
project: test
otel:
  enabled: true
  jaeger_url: "http://jaeger:16686"
  service_name: "my-service"
  trace_propagation: true
tests:
  - name: otel check
    expect:
      otel_trace:
        jaeger_url: "http://jaeger:16686"
        service_name: "my-service"
        min_spans: 1
        timeout: 5s
`
	cfg, err := LoadFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !cfg.OTel.Enabled {
		t.Error("expected otel.enabled = true")
	}
	if cfg.OTel.JaegerURL != "http://jaeger:16686" {
		t.Errorf("jaeger_url = %q, want http://jaeger:16686", cfg.OTel.JaegerURL)
	}
	if cfg.OTel.ServiceName != "my-service" {
		t.Errorf("service_name = %q, want my-service", cfg.OTel.ServiceName)
	}
	if !cfg.OTel.TracePropagation {
		t.Error("expected trace_propagation = true")
	}
	if cfg.Tests[0].Expect.OTelTrace == nil {
		t.Fatal("expected otel_trace assertion")
	}
	if cfg.Tests[0].Expect.OTelTrace.MinSpans != 1 {
		t.Errorf("min_spans = %d, want 1", cfg.Tests[0].Expect.OTelTrace.MinSpans)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/schema/ -run TestSmokeConfig_OTel -v`
Expected: FAIL — `cfg.OTel` undefined

- [ ] **Step 3: Add OTelConfig and OTelTraceCheck structs**

In `internal/schema/schema.go`, add after the `Settings` struct (around line 33):

```go
// OTelConfig configures OpenTelemetry trace context propagation.
type OTelConfig struct {
	Enabled           bool   `yaml:"enabled,omitempty"`
	JaegerURL         string `yaml:"jaeger_url,omitempty"`
	ServiceName       string `yaml:"service_name,omitempty"`
	TracePropagation  bool   `yaml:"trace_propagation,omitempty"`
}
```

Add to `SmokeConfig` struct (after `Settings`):

```go
OTel OTelConfig `yaml:"otel,omitempty"`
```

Add `OTelTraceCheck` struct (after `WebSocketCheck`):

```go
// OTelTraceCheck verifies that a trace arrived at a Jaeger-compatible collector.
type OTelTraceCheck struct {
	JaegerURL   string   `yaml:"jaeger_url,omitempty"`
	ServiceName string   `yaml:"service_name,omitempty"`
	MinSpans    int      `yaml:"min_spans,omitempty"`
	Timeout     Duration `yaml:"timeout,omitempty"`
}
```

Add to `Expect` struct (after `WebSocket` field):

```go
OTelTrace *OTelTraceCheck `yaml:"otel_trace,omitempty"`
```

Add `Headers` field to `WebSocketCheck` struct:

```go
Headers map[string]string `yaml:"headers,omitempty"`
```

Add `Metadata` field to `GRPCHealthCheck` struct (runtime-only, not from YAML):

```go
Metadata map[string]string `yaml:"-"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/schema/ -run TestSmokeConfig_OTel -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(schema): add OTelConfig and OTelTraceCheck structs

- Add top-level OTelConfig for trace propagation settings
- Add OTelTraceCheck assertion for Jaeger trace verification
- Wire into SmokeConfig and Expect structs

Refs: IDEA-023
```

---

### Task 2: Add validation for otel config

**Files:**
- Modify: `internal/schema/validate.go:86-98` (add otel_trace validation block)
- Modify: `internal/schema/validate.go:108-125` (add to hasStandaloneAssertions)
- Test: `internal/schema/validate_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestValidate_OTelTraceRequiresJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "otel",
			Expect: Expect{
				OTelTrace: &OTelTraceCheck{MinSpans: 1},
			},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for otel_trace without jaeger_url")
	}
	if !strings.Contains(err.Error(), "otel_trace.jaeger_url") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_OTelEnabledRequiresJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		OTel:    OTelConfig{Enabled: true},
		Tests:   []Test{{Name: "t", Run: "true"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for otel enabled without jaeger_url")
	}
	if !strings.Contains(err.Error(), "otel.jaeger_url") {
		t.Errorf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/schema/ -run "TestValidate_OTel" -v`
Expected: FAIL

- [ ] **Step 3: Add validation logic**

In `validate.go`, after the WebSocket validation block (around line 97), add:

```go
if e := t.Expect.OTelTrace; e != nil {
	if e.JaegerURL == "" && cfg.OTel.JaegerURL == "" {
		errs = append(errs, fmt.Sprintf("%s: otel_trace.jaeger_url is required (or set otel.jaeger_url globally)", prefix))
	} else if e.JaegerURL != "" && !strings.HasPrefix(e.JaegerURL, "http://") && !strings.HasPrefix(e.JaegerURL, "https://") {
		errs = append(errs, fmt.Sprintf("%s: otel_trace.jaeger_url must start with http:// or https://", prefix))
	}
	if e.MinSpans < 0 {
		errs = append(errs, fmt.Sprintf("%s: otel_trace.min_spans must be >= 0", prefix))
	}
}
```

After the test loop, add global otel validation:

```go
if cfg.OTel.Enabled && cfg.OTel.JaegerURL == "" {
	errs = append(errs, "otel.jaeger_url is required when otel is enabled")
}
if cfg.OTel.Enabled && !strings.HasPrefix(cfg.OTel.JaegerURL, "http://") && !strings.HasPrefix(cfg.OTel.JaegerURL, "https://") {
	errs = append(errs, "otel.jaeger_url must start with http:// or https://")
}
```

Add to `hasStandaloneAssertions`:

```go
e.OTelTrace != nil ||
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/schema/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```
feat(schema): add otel config and assertion validation

- Validate jaeger_url required when otel enabled or otel_trace used
- Validate URL format for both global and per-test jaeger_url
- Add otel_trace to standalone assertions list

Refs: IDEA-023
```

---

## Chunk 2: Trace Context Generation

### Task 3: Implement TraceContext

**Files:**
- Create: `internal/runner/trace.go`
- Create: `internal/runner/trace_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/runner/trace_test.go`:

```go
package runner

import (
	"regexp"
	"testing"
)

func TestNewTraceContext(t *testing.T) {
	tc := NewTraceContext()
	if tc == nil {
		t.Fatal("expected non-nil TraceContext")
	}
	if !tc.Enabled {
		t.Error("expected Enabled = true")
	}
	if len(tc.TraceID()) != 32 {
		t.Errorf("traceID length = %d, want 32", len(tc.TraceID()))
	}
	matched, _ := regexp.MatchString("^[0-9a-f]{32}$", tc.TraceID())
	if !matched {
		t.Errorf("traceID = %q, want 32 lowercase hex chars", tc.TraceID())
	}
}

func TestTraceContext_ChildSpan(t *testing.T) {
	tc := NewTraceContext()
	span := tc.NewSpan()
	if len(span.SpanID()) != 16 {
		t.Errorf("spanID length = %d, want 16", len(span.SpanID()))
	}
}

func TestSpanContext_Traceparent(t *testing.T) {
	tc := NewTraceContext()
	span := tc.NewSpan()
	tp := span.Traceparent()
	// Format: 00-{traceID32}-{spanID16}-{flags2}
	pattern := `^00-[0-9a-f]{32}-[0-9a-f]{16}-01$`
	matched, err := regexp.MatchString(pattern, tp)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("traceparent = %q, want format 00-{32hex}-{16hex}-01", tp)
	}
}

func TestTraceContext_DifferentTraceIDs(t *testing.T) {
	tc1 := NewTraceContext()
	tc2 := NewTraceContext()
	if tc1.TraceID() == tc2.TraceID() {
		t.Error("two trace contexts should have different trace IDs")
	}
}

func TestTraceContext_DifferentSpanIDs(t *testing.T) {
	tc := NewTraceContext()
	s1 := tc.NewSpan()
	s2 := tc.NewSpan()
	if s1.SpanID() == s2.SpanID() {
		t.Error("two spans should have different span IDs")
	}
}

func TestDisabledTraceContext(t *testing.T) {
	tc := &TraceContext{}
	if tc.Enabled {
		t.Error("zero-value TraceContext should not be enabled")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/ -run "TestTraceContext|TestSpanContext|TestDisabledTraceContext|TestNewTraceContext" -v`
Expected: FAIL — `NewTraceContext` undefined

- [ ] **Step 3: Implement TraceContext**

`internal/runner/trace.go`:

```go
package runner

import (
	"crypto/rand"
	"encoding/hex"
)

// TraceContext holds the suite-level W3C trace context.
type TraceContext struct {
	traceID [16]byte
	Enabled bool
}

// SpanContext holds per-test trace context.
type SpanContext struct {
	traceID [16]byte
	spanID  [8]byte
}

// NewTraceContext creates a new trace context with a random trace ID.
func NewTraceContext() *TraceContext {
	var tc TraceContext
	rand.Read(tc.traceID[:])
	tc.Enabled = true
	return &tc
}

// TraceID returns the hex-encoded trace ID.
func (tc *TraceContext) TraceID() string {
	return hex.EncodeToString(tc.traceID[:])
}

// NewSpan creates a child span with a random span ID under this trace.
func (tc *TraceContext) NewSpan() *SpanContext {
	var sc SpanContext
	sc.traceID = tc.traceID
	rand.Read(sc.spanID[:])
	return &sc
}

// SpanID returns the hex-encoded span ID.
func (sc *SpanContext) SpanID() string {
	return hex.EncodeToString(sc.spanID[:])
}

// Traceparent returns the W3C traceparent header value.
// Format: 00-{traceID}-{spanID}-01
func (sc *SpanContext) Traceparent() string {
	return "00-" + hex.EncodeToString(sc.traceID[:]) + "-" + hex.EncodeToString(sc.spanID[:]) + "-01"
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/ -run "TestTraceContext|TestSpanContext|TestDisabledTraceContext|TestNewTraceContext" -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```
feat(runner): add W3C trace context generation

- TraceContext generates suite-level trace ID via crypto/rand
- SpanContext generates per-test span ID with traceparent formatting
- 6 tests covering: format, uniqueness, zero-value disabled state

Refs: IDEA-023
```

---

## Chunk 3: otel_trace Assertion

### Task 4: Implement CheckOTelTrace

**Files:**
- Create: `internal/runner/assertion_otel.go`
- Create: `internal/runner/assertion_otel_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/runner/assertion_otel_test.go`:

```go
package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

func jaegerResponse(traceID string, spanCount int) string {
	type span struct {
		TraceID       string `json:"traceID"`
		SpanID        string `json:"spanID"`
		OperationName string `json:"operationName"`
	}
	type traceData struct {
		TraceID string `json:"traceID"`
		Spans   []span `json:"spans"`
	}
	spans := make([]span, spanCount)
	for i := range spans {
		spans[i] = span{TraceID: traceID, SpanID: fmt.Sprintf("span%d", i), OperationName: fmt.Sprintf("op%d", i)}
	}
	data := []traceData{{TraceID: traceID, Spans: spans}}
	b, _ := json.Marshal(map[string]interface{}{"data": data})
	return string(b)
}

func TestCheckOTelTrace_TraceFound(t *testing.T) {
	traceID := "abc123def456abc123def456abc123de"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/traces/"+traceID {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(jaegerResponse(traceID, 2)))
	}))
	defer ts.Close()

	result := CheckOTelTrace(&schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "my-service",
		MinSpans:    1,
	}, traceID, ts.Client())

	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
	if result.Type != "otel_trace" {
		t.Errorf("type = %q, want otel_trace", result.Type)
	}
}

func TestCheckOTelTrace_MinSpansNotMet(t *testing.T) {
	traceID := "abc123def456abc123def456abc123de"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(jaegerResponse(traceID, 1)))
	}))
	defer ts.Close()

	result := CheckOTelTrace(&schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "my-service",
		MinSpans:    5,
	}, traceID, ts.Client())

	if result.Passed {
		t.Error("expected failure for min_spans not met")
	}
}

func TestCheckOTelTrace_TimeoutNoSpans(t *testing.T) {
	traceID := "abc123def456abc123def456abc123de"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	result := CheckOTelTrace(&schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "my-service",
		MinSpans:    1,
		Timeout:     schema.Duration{Duration: 100 * time.Millisecond},
	}, traceID, ts.Client())

	if result.Passed {
		t.Error("expected failure for empty trace data")
	}
}

func TestCheckOTelTrace_CollectorUnreachable(t *testing.T) {
	result := CheckOTelTrace(&schema.OTelTraceCheck{
		JaegerURL:   "http://127.0.0.1:1",
		ServiceName: "my-service",
		MinSpans:    1,
		Timeout:     schema.Duration{Duration: 100 * time.Millisecond},
	}, "abc123def456abc123def456abc123de", http.DefaultClient)

	if result.Passed {
		t.Error("expected failure for unreachable collector")
	}
}

func TestCheckOTelTrace_PollingRetries(t *testing.T) {
	traceID := "abc123def456abc123def456abc123de"
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.Write([]byte(jaegerResponse(traceID, 1)))
	}))
	defer ts.Close()

	result := CheckOTelTrace(&schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "my-service",
		MinSpans:    1,
		Timeout:     schema.Duration{Duration: 2 * time.Second},
	}, traceID, ts.Client())

	if !result.Passed {
		t.Errorf("expected pass after retry, got: %s", result.Actual)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 calls, got %d", callCount)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/ -run TestCheckOTelTrace -v`
Expected: FAIL — `CheckOTelTrace` undefined

- [ ] **Step 3: Implement CheckOTelTrace**

`internal/runner/assertion_otel.go`:

```go
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

// jaegerResponse represents the Jaeger API trace response.
type jaegerResponse struct {
	Data []jaegerTrace `json:"data"`
}

type jaegerTrace struct {
	TraceID string       `json:"traceID"`
	Spans   []jaegerSpan `json:"spans"`
}

type jaegerSpan struct {
	TraceID       string `json:"traceID"`
	SpanID        string `json:"spanID"`
	OperationName string `json:"operationName"`
}

// CheckOTelTrace queries a Jaeger-compatible API to verify that a trace was received.
// Polls at 500ms intervals until timeout expires or min_spans is satisfied.
func CheckOTelTrace(check *schema.OTelTraceCheck, traceID string, client *http.Client) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	minSpans := check.MinSpans
	if minSpans == 0 {
		minSpans = 1
	}
	url := fmt.Sprintf("%s/api/traces/%s?service=%s", check.JaegerURL, traceID, check.ServiceName)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := queryJaeger(client, url)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if len(resp.Data) > 0 && len(resp.Data[0].Spans) >= minSpans {
			return AssertionResult{
				Type:     "otel_trace",
				Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
				Actual:   fmt.Sprintf("%d spans found", len(resp.Data[0].Spans)),
				Passed:   true,
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Final attempt
	resp, err := queryJaeger(client, url)
	if err != nil {
		return AssertionResult{
			Type:     "otel_trace",
			Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
			Actual:   fmt.Sprintf("collector error: %v", err),
			Passed:   false,
		}
	}
	if len(resp.Data) > 0 {
		return AssertionResult{
			Type:     "otel_trace",
			Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
			Actual:   fmt.Sprintf("%d spans found within %s", len(resp.Data[0].Spans), timeout),
			Passed:   len(resp.Data[0].Spans) >= minSpans,
		}
	}
	return AssertionResult{
		Type:     "otel_trace",
		Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
		Actual:   fmt.Sprintf("no spans found for trace %s within %s", traceID, timeout),
		Passed:   false,
	}
}

func queryJaeger(client *http.Client, url string) (*jaegerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jr jaegerResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, err
	}
	return &jr, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/ -run TestCheckOTelTrace -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```
feat(runner): add otel_trace assertion with Jaeger API polling

- CheckOTelTrace queries Jaeger /api/traces/{id}?service=X
- Polls at 500ms intervals until timeout or min_spans met
- Accepts *http.Client for testability (httptest mocks)
- 5 tests covering: found, min_spans not met, timeout, unreachable, polling retry

Refs: IDEA-023
```

---

## Chunk 4: Trace Injection into Network Assertions

### Task 5: Inject traceparent into HTTP requests

**Files:**
- Modify: `internal/runner/assertion_network.go:71-107` (CheckHTTP function)
- Test: `internal/runner/assertion_test.go` or new test

- [ ] **Step 1: Write the failing test**

```go
func TestCheckHTTP_TraceparentInjected(t *testing.T) {
	var receivedTraceparent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceparent = r.Header.Get("traceparent")
		w.WriteHeader(200)
	}))
	defer ts.Close()

	check := &schema.HTTPCheck{
		URL:        ts.URL,
		StatusCode: intPtr(200),
	}
	span := NewTraceContext().NewSpan()
	results := CheckHTTPWithTrace(check, span)
	for _, r := range results {
		if !r.Passed {
			t.Errorf("unexpected failure: %v", r)
		}
	}
	if receivedTraceparent == "" {
		t.Error("expected traceparent header to be injected")
	}
	expected := span.Traceparent()
	if receivedTraceparent != expected {
		t.Errorf("traceparent = %q, want %q", receivedTraceparent, expected)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestCheckHTTP_TraceparentInjected -v`
Expected: FAIL — `CheckHTTPWithTrace` undefined

- [ ] **Step 3: Implement CheckHTTPWithTrace**

In `assertion_network.go`, after `CheckHTTP`, add:

```go
// CheckHTTPWithTrace is like CheckHTTP but injects a traceparent header.
func CheckHTTPWithTrace(check *schema.HTTPCheck, span *SpanContext) []AssertionResult {
	if check.Headers == nil {
		check.Headers = make(map[string]string)
	}
	check.Headers["traceparent"] = span.Traceparent()
	return CheckHTTP(check)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/ -run TestCheckHTTP_TraceparentInjected -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(runner): add HTTP traceparent injection via CheckHTTPWithTrace

- Inject W3C traceparent into HTTP request headers before sending
- Delegates to existing CheckHTTP after header injection
- 1 test verifying header is present in downstream request

Refs: IDEA-023
```

---

### Task 6: Inject traceparent into WebSocket handshake

**Files:**
- Modify: `internal/runner/assertion_ws.go:160-163` (CheckWebSocket function)
- Test: `internal/runner/assertion_ws_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestCheckWebSocket_TraceparentInjected(t *testing.T) {
	var receivedTP string
	ts := wsTestServerWithHeader(func(conn net.Conn, msg string, headers http.Header) string {
		receivedTP = headers.Get("traceparent")
		return "ok"
	})
	defer ts.Close()

	check := &schema.WebSocketCheck{
		URL:            "ws://" + strings.TrimPrefix(ts.URL, "http://") + "/ws",
		ExpectContains: "ok",
	}
	span := NewTraceContext().NewSpan()
	result := CheckWebSocketWithTrace(check, span)
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
	if receivedTP != span.Traceparent() {
		t.Errorf("traceparent = %q, want %q", receivedTP, span.Traceparent())
	}
}
```

Note: This requires the wsTestServer helper to expose received headers. If the existing helper doesn't support it, create `wsTestServerWithHeader` that captures the HTTP upgrade request headers.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestCheckWebSocket_TraceparentInjected -v`
Expected: FAIL

- [ ] **Step 3: Implement CheckWebSocketWithTrace**

In `assertion_ws.go`, add:

```go
// CheckWebSocketWithTrace is like CheckWebSocket but injects a traceparent header.
func CheckWebSocketWithTrace(check *schema.WebSocketCheck, span *SpanContext) AssertionResult {
	if check.Headers == nil {
		check.Headers = make(map[string]string)
	}
	check.Headers["traceparent"] = span.Traceparent()
	return CheckWebSocket(check)
}
```

Also add `Headers` field to `WebSocketCheck` in schema if not already present (check `internal/schema/schema.go` for the WebSocketCheck struct).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/ -run TestCheckWebSocket_TraceparentInjected -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(runner): add WebSocket traceparent injection via CheckWebSocketWithTrace

- Inject W3C traceparent into WebSocket handshake headers
- Add Headers field to WebSocketCheck schema if missing
- 1 test verifying header in upgrade request

Refs: IDEA-023
```

---

### Task 7: Inject traceparent into gRPC metadata

**Files:**
- Modify: `internal/runner/assertion_grpc.go:17-20` (CheckGRPCHealth function)
- Modify: `internal/runner/assertion_grpc_stub.go:10-13` (stub version)
- Test: `internal/runner/assertion_grpc_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestCheckGRPCHealth_WithTraceparent(t *testing.T) {
	addr, _, stop := startTestGRPCServer(t)
	defer stop()
	// This test verifies the WithTrace variant doesn't break existing behavior
	// Actual traceparent verification requires interceptor on server side
	check := &schema.GRPCHealthCheck{Address: addr}
	span := NewTraceContext().NewSpan()
	result := CheckGRPCHealthWithTrace(check, span)
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags grpc ./internal/runner/ -run TestCheckGRPCHealth_WithTraceparent -v`
Expected: FAIL — `CheckGRPCHealthWithTrace` undefined

- [ ] **Step 3: Implement CheckGRPCHealthWithTrace**

In `assertion_grpc.go`, add:

```go
// CheckGRPCHealthWithTrace is like CheckGRPCHealth but injects traceparent into metadata.
func CheckGRPCHealthWithTrace(check *schema.GRPCHealthCheck, span *SpanContext) AssertionResult {
	if check.Metadata == nil {
		check.Metadata = make(map[string]string)
	}
	check.Metadata["traceparent"] = span.Traceparent()
	return CheckGRPCHealth(check)
}
```

Add `Metadata` field to `GRPCHealthCheck` in schema:

```go
Metadata map[string]string `yaml:"-"` // injected at runtime, not from YAML
```

In `assertion_grpc.go`, modify the dial options to attach metadata if present:

In the existing `CheckGRPCHealth`, before the dial, add metadata to context if `check.Metadata` is set:

```go
ctx := context.Background()
if len(check.Metadata) > 0 {
	md := metadata.New(check.Metadata)
	ctx = metadata.NewOutgoingContext(ctx, md)
}
```

Also add the stub version in `assertion_grpc_stub.go`:

```go
// CheckGRPCHealthWithTrace is the stub version (no-op).
func CheckGRPCHealthWithTrace(check *schema.GRPCHealthCheck, span *SpanContext) AssertionResult {
	return CheckGRPCHealth(check)
}
```

- [ ] **Step 4: Run tests**

Run: `go test -tags grpc ./internal/runner/ -run TestCheckGRPCHealth -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```
feat(runner): add gRPC traceparent injection via CheckGRPCHealthWithTrace

- Inject W3C traceparent into gRPC metadata before dial
- Add runtime Metadata field to GRPCHealthCheck (not from YAML)
- Stub version for non-grpc builds

Refs: IDEA-023
```

---

## Chunk 5: Runner Integration

### Task 8: Wire trace context into runner

**Files:**
- Modify: `internal/runner/runner.go:52-56` (Runner struct)
- Modify: `internal/runner/runner.go:59-117` (Run method)
- Modify: `internal/runner/runner.go:243-508` (runTestOnce — assertion dispatch)

- [ ] **Step 1: Write integration test**

```go
func TestRunner_OTelTracePropagation(t *testing.T) {
	// Start a fake Jaeger that records trace lookups
	var traceLookups []string
	jaeger := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceLookups = append(traceLookups, r.URL.Path)
		w.Write([]byte(`{"data":[{"traceID":"test","spans":[{"spanID":"s1","operationName":"test"}]}]}`))
	}))
	defer jaeger.Close()

	// Start an HTTP server that checks for traceparent
	var gotTP string
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTP = r.Header.Get("traceparent")
		w.WriteHeader(200)
	}))
	defer httpSrv.Close()

	yaml := fmt.Sprintf(`
version: 1
project: otel-test
otel:
  enabled: true
  jaeger_url: %s
  service_name: test-svc
  trace_propagation: true
tests:
  - name: http with trace
    expect:
      http:
        url: %s
        status_code: 200
  - name: verify trace
    expect:
      otel_trace:
        jaeger_url: %s
        service_name: test-svc
        timeout: 2s
`, jaeger.URL, httpSrv.URL, jaeger.URL)

	cfg, err := schema.LoadFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	buf := &bytes.Buffer{}
	r := &Runner{
		Config:   cfg,
		Reporter: reporter.NewTerminal(buf),
	}
	result, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Failed > 0 {
		t.Errorf("expected all pass, got %d failed", result.Failed)
	}
	if gotTP == "" {
		t.Error("expected traceparent header in HTTP request")
	}
	if len(traceLookups) == 0 {
		t.Error("expected Jaeger API to be queried for trace verification")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestRunner_OTelTracePropagation -v`
Expected: FAIL — HTTP test doesn't receive traceparent, otel_trace not dispatched

- [ ] **Step 3: Modify Runner struct**

Add `trace` field to `Runner`:

```go
type Runner struct {
	Config    *schema.SmokeConfig
	Reporter  reporter.Reporter
	ConfigDir string
	trace     *TraceContext
}
```

- [ ] **Step 4: Initialize trace context in Run()**

In `Run()`, after filtering tests (around line 88), add:

```go
if r.Config.OTel.Enabled {
	r.trace = NewTraceContext()
}
```

- [ ] **Step 5: Create span per test in runTestOnce()**

At the start of `runTestOnce`, after skip/dry-run checks (around line 259):

```go
var span *SpanContext
if r.trace != nil && r.trace.Enabled {
	span = r.trace.NewSpan()
}
```

- [ ] **Step 6: Dispatch traceparent into network assertions**

Replace the HTTP assertion dispatch (around line 380):

```go
if t.Expect.HTTP != nil {
	var httpResults []AssertionResult
	if span != nil && r.Config.OTel.TracePropagation {
		httpResults = CheckHTTPWithTrace(t.Expect.HTTP, span)
	} else {
		httpResults = CheckHTTP(t.Expect.HTTP)
	}
	for _, a := range httpResults {
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
}
```

Similarly for WebSocket (around line 482):

```go
if t.Expect.WebSocket != nil {
	var wsResult AssertionResult
	if span != nil && r.Config.OTel.TracePropagation {
		wsResult = CheckWebSocketWithTrace(t.Expect.WebSocket, span)
	} else {
		wsResult = CheckWebSocket(t.Expect.WebSocket)
	}
	assertions = append(assertions, wsResult)
	if !wsResult.Passed {
		allPassed = false
	}
}
```

And for gRPC (around line 433):

```go
if t.Expect.GRPCHealth != nil {
	var grpcResult AssertionResult
	if span != nil && r.Config.OTel.TracePropagation {
		grpcResult = CheckGRPCHealthWithTrace(t.Expect.GRPCHealth, span)
	} else {
		grpcResult = CheckGRPCHealth(t.Expect.GRPCHealth)
	}
	assertions = append(assertions, grpcResult)
	if !grpcResult.Passed {
		allPassed = false
	}
}
```

- [ ] **Step 7: Dispatch otel_trace assertion**

After the WebSocket block (around line 488), add:

```go
if t.Expect.OTelTrace != nil {
	check := t.Expect.OTelTrace
	if check.JaegerURL == "" {
		check.JaegerURL = r.Config.OTel.JaegerURL
	}
	if check.ServiceName == "" {
		check.ServiceName = r.Config.OTel.ServiceName
	}
	if check.ServiceName == "" {
		check.ServiceName = "smoke"
	}
	traceID := ""
	if r.trace != nil && r.trace.Enabled {
		traceID = r.trace.TraceID()
	}
	client := &http.Client{Timeout: check.Timeout.Duration + 5*time.Second}
	if client.Timeout == 0 {
		client.Timeout = 10 * time.Second
	}
	a := CheckOTelTrace(check, traceID, client)
	assertions = append(assertions, a)
	if !a.Passed {
		allPassed = false
	}
}
```

- [ ] **Step 8: Run integration test**

Run: `go test ./internal/runner/ -run TestRunner_OTelTracePropagation -v`
Expected: PASS

- [ ] **Step 9: Run full test suite**

Run: `go test ./...`
Expected: ALL PASS (no regressions)

- [ ] **Step 10: Commit**

```
feat(runner): wire trace context into test execution pipeline

- Initialize TraceContext in Runner when otel config enabled
- Create per-test SpanContext, inject traceparent into HTTP/gRPC/WS
- Dispatch otel_trace assertion with Jaeger API verification
- Global otel config provides defaults for per-test jaeger_url/service_name

Refs: IDEA-023
```

---

## Chunk 6: CLI Flags & Self-Smoke

### Task 9: Add --otel-collector and --no-otel CLI flags

**Files:**
- Modify: `cmd/run.go`
- Test: `cmd/run_test.go` (or relevant test file)

- [ ] **Step 1: Write the failing test**

```go
func TestRunFlags_OTelCollector(t *testing.T) {
	// Test that --otel-collector overrides config
	// Verify cfg.OTel.JaegerURL is set and cfg.OTel.Enabled is true
}

func TestRunFlags_NoOtel(t *testing.T) {
	// Test that --no-otel disables otel regardless of config
	// Verify cfg.OTel.Enabled is false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run "TestRunFlags_OTel|TestRunFlags_NoOtel" -v`
Expected: FAIL

- [ ] **Step 3: Add flags to run command**

In `cmd/run.go`, add flags:

```go
runCmd.Flags().String("otel-collector", "", "Override otel.jaeger_url at runtime")
runCmd.Flags().Bool("no-otel", false, "Disable otel trace propagation for this run")
```

Wire in the `run` function:

```go
if v, _ := cmd.Flags().GetBool("no-otel"); v {
	cfg.OTel.Enabled = false
}
if v, _ := cmd.Flags().GetString("otel-collector"); v != "" {
	cfg.OTel.JaegerURL = v
	cfg.OTel.Enabled = true
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/ -run "TestRunFlags_OTel|TestRunFlags_NoOtel" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(cmd): add --otel-collector and --no-otel CLI flags

- --otel-collector overrides jaeger_url and enables otel
- --no-otel disables trace propagation for this run
- 2 tests covering flag override behavior

Refs: IDEA-023
```

---

### Task 10: Update self-smoke config and CLAUDE.md

**Files:**
- Modify: `.smoke.yaml` (add otel assertion example)
- Modify: `CLAUDE.md` (add otel to assertion table, update build section)

- [ ] **Step 1: Update CLAUDE.md assertion table**

Update assertion count from 26 to 27. Add row to the assertion types table:

```
| otel_trace | `{jaeger_url, service_name?, min_spans?, timeout?}` | Jaeger API trace verification (W3C traceparent propagation) |
```

Add to the otel config section in schema:

```yaml
otel:
  enabled: true
  jaeger_url: "http://jaeger:16686"
  service_name: "cosmo-smoke"
  trace_propagation: true
```

- [ ] **Step 2: Commit**

```
docs: update CLAUDE.md with otel trace correlation assertion

Refs: IDEA-023
```
