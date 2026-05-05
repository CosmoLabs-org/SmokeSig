package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// jaegerJSON builds a Jaeger API response body with the given span count.
func jaegerJSON(traceID string, spanCount int) string {
	type span struct {
		TraceID string `json:"traceID"`
		SpanID  string `json:"spanID"`
	}
	spans := make([]span, spanCount)
	for i := range spans {
		spans[i] = span{TraceID: traceID, SpanID: fmt.Sprintf("span%d", i)}
	}
	type trace struct {
		TraceID string `json:"traceID"`
		Spans   []span `json:"spans"`
	}
	data := trace{TraceID: traceID, Spans: spans}
	b, _ := json.Marshal(map[string]interface{}{"data": []trace{data}})
	return string(b)
}

func TestCheckOTelTrace_TraceFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jaegerJSON("abc123", 1))
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "myservice",
		MinSpans:    1,
		Timeout:     schema.Duration{Duration: 2 * time.Second},
	}
	result := CheckOTelTrace(check, "abc123", ts.Client())
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Actual)
	}
}

func TestCheckOTelTrace_MinSpansNotMet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jaegerJSON("abc123", 1))
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "myservice",
		MinSpans:    5,
		Timeout:     schema.Duration{Duration: 100 * time.Millisecond},
	}
	result := CheckOTelTrace(check, "abc123", ts.Client())
	if result.Passed {
		t.Error("expected fail, got pass")
	}
}

func TestCheckOTelTrace_TimeoutNoSpans(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "myservice",
		MinSpans:    1,
		Timeout:     schema.Duration{Duration: 100 * time.Millisecond},
	}
	result := CheckOTelTrace(check, "abc123", ts.Client())
	if result.Passed {
		t.Error("expected fail, got pass")
	}
}

func TestCheckOTelTrace_CollectorUnreachable(t *testing.T) {
	// Port 1 is not listening — forces connection refused.
	check := &schema.OTelTraceCheck{
		JaegerURL:   "http://127.0.0.1:1",
		ServiceName: "myservice",
		MinSpans:    1,
		Timeout:     schema.Duration{Duration: 100 * time.Millisecond},
	}
	result := CheckOTelTrace(check, "abc123", http.DefaultClient)
	if result.Passed {
		t.Error("expected fail, got pass")
	}
}

func TestCheckOTelTrace_PollingRetries(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount < 3 {
			fmt.Fprint(w, `{"data":[]}`)
			return
		}
		fmt.Fprint(w, jaegerJSON("retry-trace", 2))
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		JaegerURL:   ts.URL,
		ServiceName: "myservice",
		MinSpans:    1,
		Timeout:     schema.Duration{Duration: 5 * time.Second},
	}
	result := CheckOTelTrace(check, "retry-trace", ts.Client())
	if !result.Passed {
		t.Errorf("expected pass after retries, got fail: %s", result.Actual)
	}
	if callCount < 3 {
		t.Errorf("expected >= 3 calls, got %d", callCount)
	}
}

func TestCheckOTelTrace_TempoBackend(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/traces/abc123" {
			t.Errorf("path = %q, want /api/traces/abc123", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jaegerJSON("abc123", 3))
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		Backend:     "tempo",
		JaegerURL:   ts.URL,
		ServiceName: "myservice",
		MinSpans:    2,
		Timeout:     schema.Duration{Duration: 2 * time.Second},
	}
	result := CheckOTelTrace(check, "abc123", ts.Client())
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Actual)
	}
}

func TestCheckOTelTrace_HoneycombBackend(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Honeycomb-Team"); got != "hc-api-key-123" {
			t.Errorf("X-Honeycomb-Team = %q, want hc-api-key-123", got)
		}
		if got := r.URL.Path; got != "/v1/traces/trace-hc" {
			t.Errorf("path = %q, want /v1/traces/trace-hc", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"spans":[{},{},{}]}}`)
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		Backend:   "honeycomb",
		JaegerURL: ts.URL,
		APIKey:    "hc-api-key-123",
		MinSpans:  2,
		Timeout:   schema.Duration{Duration: 2 * time.Second},
	}
	result := CheckOTelTrace(check, "trace-hc", ts.Client())
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Actual)
	}
}

func TestCheckOTelTrace_DatadogBackend(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("DD-API-KEY"); got != "dd-api-key-456" {
			t.Errorf("DD-API-KEY = %q, want dd-api-key-456", got)
		}
		if got := r.Header.Get("DD-APPLICATION-KEY"); got != "dd-app-key-789" {
			t.Errorf("DD-APPLICATION-KEY = %q, want dd-app-key-789", got)
		}
		if got := r.URL.Path; got != "/api/v1/traces/trace-dd" {
			t.Errorf("path = %q, want /api/v1/traces/trace-dd", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"traces":[{"spans":[{},{}]},{"spans":[{}]}]}`)
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		Backend:   "datadog",
		JaegerURL: ts.URL,
		APIKey:    "dd-api-key-456",
		DDAppKey:  "dd-app-key-789",
		MinSpans:  3,
		Timeout:   schema.Duration{Duration: 2 * time.Second},
	}
	result := CheckOTelTrace(check, "trace-dd", ts.Client())
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Actual)
	}
}

func TestCheckOTelTrace_HoneycombBackend_NoSpans(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"spans":[]}}`)
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		Backend:   "honeycomb",
		JaegerURL: ts.URL,
		APIKey:    "test-key",
		MinSpans:  1,
		Timeout:   schema.Duration{Duration: 100 * time.Millisecond},
	}
	result := CheckOTelTrace(check, "missing", ts.Client())
	if result.Passed {
		t.Error("expected fail for empty spans")
	}
}

func TestCheckOTelTrace_DefaultBackendIsJaeger(t *testing.T) {
	// Empty backend string should behave like jaeger.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/traces/default-trace" {
			t.Errorf("path = %q, want /api/traces/default-trace (jaeger format)", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jaegerJSON("default-trace", 1))
	}))
	defer ts.Close()

	check := &schema.OTelTraceCheck{
		Backend:   "",
		JaegerURL: ts.URL,
		MinSpans:  1,
		Timeout:   schema.Duration{Duration: 2 * time.Second},
	}
	result := CheckOTelTrace(check, "default-trace", ts.Client())
	if !result.Passed {
		t.Errorf("expected pass with default (jaeger) backend, got fail: %s", result.Actual)
	}
}
