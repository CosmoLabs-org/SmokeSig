package reporter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOTelReporter_TestResult_SendsSpan(t *testing.T) {
	var received sync.Mutex
	var bodies []json.RawMessage

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/traces" {
			t.Errorf("path = %q, want /v1/traces", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body, _ := io.ReadAll(r.Body)
		received.Lock()
		bodies = append(bodies, body)
		received.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	r := NewOTelReporter(ts.URL+"/v1/traces", "smoke-test", nil)
	r.TestResult(TestResultData{
		Name:     "api-health",
		Passed:   true,
		Duration: 150 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Passed: true},
		},
	})

	// Wait for async send
	time.Sleep(100 * time.Millisecond)

	received.Lock()
	if len(bodies) != 1 {
		t.Fatalf("expected 1 request, got %d", len(bodies))
	}
	received.Unlock()

	// Parse OTLP JSON
	var otlp struct {
		ResourceSpans []struct {
			ScopeSpans []struct {
				Spans []struct {
					Name       string `json:"name"`
					TraceID    string `json:"traceId"`
					SpanID     string `json:"spanId"`
					Attributes []struct {
						Key   string `json:"key"`
						Value struct {
							StringValue string `json:"stringValue"`
						} `json:"value"`
					} `json:"attributes"`
				} `json:"spans"`
			} `json:"scopeSpans"`
		} `json:"resourceSpans"`
	}
	if err := json.Unmarshal(bodies[0], &otlp); err != nil {
		t.Fatalf("parse OTLP JSON: %v", err)
	}
	if len(otlp.ResourceSpans) == 0 || len(otlp.ResourceSpans[0].ScopeSpans) == 0 {
		t.Fatal("no resource spans in OTLP payload")
	}
	spans := otlp.ResourceSpans[0].ScopeSpans[0].Spans
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]
	if s.Name != "smoke-test/api-health" {
		t.Errorf("span name = %q, want smoke-test/api-health", s.Name)
	}
	if s.TraceID == "" {
		t.Error("span traceId is empty")
	}
	if s.SpanID == "" {
		t.Error("span spanId is empty")
	}
	// Check attributes for test result
	foundStatus := false
	for _, a := range s.Attributes {
		if a.Key == "smoke.passed" {
			foundStatus = true
			if a.Value.StringValue != "true" {
				t.Errorf("smoke.passed = %q, want true", a.Value.StringValue)
			}
		}
	}
	if !foundStatus {
		t.Error("missing smoke.passed attribute")
	}
}

func TestOTelReporter_Summary_SendsSpan(t *testing.T) {
	var received sync.Mutex
	var bodyCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Lock()
		bodyCount++
		received.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	r := NewOTelReporter(ts.URL+"/v1/traces", "smoke-test", nil)
	r.Summary(SuiteResultData{
		Project: "myapp",
		Total:   5,
		Passed:  3,
		Failed:  2,
	})

	time.Sleep(100 * time.Millisecond)

	received.Lock()
	if bodyCount != 1 {
		t.Errorf("expected 1 request, got %d", bodyCount)
	}
	received.Unlock()
}

func TestOTelReporter_CustomHeaders(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	headers := map[string]string{"Authorization": "Bearer test-token"}
	r := NewOTelReporter(ts.URL+"/v1/traces", "smoke-test", headers)
	r.TestResult(TestResultData{Name: "test1", Passed: true})

	time.Sleep(100 * time.Millisecond)

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestOTelReporter_CollectorUnreachable_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	r := NewOTelReporter("http://127.0.0.1:1/v1/traces", "smoke-test", nil)
	r.warnOut = &buf // capture warnings so they don't pollute test output
	// Should not panic when collector is unreachable
	r.TestResult(TestResultData{Name: "test1", Passed: true})
	r.Summary(SuiteResultData{Project: "myapp", Total: 1, Passed: 1})
}

func TestOTelReporter_WarnsOnCollectorError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	r := NewOTelReporter(ts.URL+"/v1/traces", "smoke-test", nil)
	r.warnOut = &buf
	r.TestResult(TestResultData{Name: "test1", Passed: true, Duration: 50 * time.Millisecond})
	r.Summary(SuiteResultData{Project: "myapp", Total: 1, Passed: 1})

	got := buf.String()
	if !strings.Contains(got, "Warning: failed to export telemetry") {
		t.Errorf("expected telemetry warning, got %q", got)
	}
	if !strings.Contains(got, "500 Internal Server Error") {
		t.Errorf("expected status in warning, got %q", got)
	}
}

func TestOTelReporter_WarnsOnUnreachableCollector(t *testing.T) {
	var buf bytes.Buffer
	r := NewOTelReporter("http://127.0.0.1:1/v1/traces", "smoke-test", nil)
	r.warnOut = &buf
	r.TestResult(TestResultData{Name: "test1", Passed: true, Duration: 50 * time.Millisecond})
	r.Summary(SuiteResultData{Project: "myapp", Total: 1, Passed: 1})

	got := buf.String()
	if !strings.Contains(got, "Warning: failed to export telemetry") {
		t.Errorf("expected telemetry warning on unreachable collector, got %q", got)
	}
}

func TestOTelReporter_NoWarningOnSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	r := NewOTelReporter(ts.URL+"/v1/traces", "smoke-test", nil)
	r.warnOut = &buf
	r.TestResult(TestResultData{Name: "test1", Passed: true, Duration: 50 * time.Millisecond})
	r.Summary(SuiteResultData{Project: "myapp", Total: 1, Passed: 1})

	if buf.Len() > 0 {
		t.Errorf("expected no warnings on success, got %q", buf.String())
	}
}

func TestOTelReporter_MultipleTestFailuresCollected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	r := NewOTelReporter(ts.URL+"/v1/traces", "smoke-test", nil)
	r.warnOut = &buf
	r.TestResult(TestResultData{Name: "test1", Passed: true, Duration: 10 * time.Millisecond})
	r.TestResult(TestResultData{Name: "test2", Passed: false, Duration: 10 * time.Millisecond})
	r.Summary(SuiteResultData{Project: "myapp", Total: 2, Passed: 1, Failed: 1})

	got := buf.String()
	// Should have warnings for test1 export + test2 export + summary export = 3
	count := strings.Count(got, "Warning: failed to export telemetry")
	if count < 3 {
		t.Errorf("expected at least 3 telemetry warnings (2 tests + 1 summary), got %d in: %q", count, got)
	}
}

func TestOTelReporter_TestResult_SkippedStatus(t *testing.T) {
	var received []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	r := NewOTelReporter(ts.URL+"/v1/traces", "svc", nil)
	r.TestResult(TestResultData{
		Name:    "skipped-test",
		Skipped: true,
		Passed:  false,
	})
	r.wg.Wait()

	body := string(received)
	if !strings.Contains(body, "SKIP") {
		t.Errorf("skipped test should emit SKIP status, got: %s", body)
	}
}

func TestOTelReporter_TestResult_AllowedFailureStatus(t *testing.T) {
	var received []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	r := NewOTelReporter(ts.URL+"/v1/traces", "svc", nil)
	r.TestResult(TestResultData{
		Name:           "flaky-test",
		AllowedFailure: true,
		Passed:         false,
	})
	r.wg.Wait()

	body := string(received)
	if !strings.Contains(body, "ALLOWED_FAILURE") {
		t.Errorf("allowed failure test should emit ALLOWED_FAILURE status, got: %s", body)
	}
}

func TestOTelReporter_Export_Returns400Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	r := NewOTelReporter(ts.URL+"/v1/traces", "svc", nil)
	r.warnOut = &buf
	r.TestResult(TestResultData{Name: "test", Passed: true, Duration: 10 * time.Millisecond})
	// Warnings are printed during Summary after wg.Wait()
	r.Summary(SuiteResultData{Project: "svc", Total: 1, Passed: 1})

	if !strings.Contains(buf.String(), "Warning: failed to export telemetry") {
		t.Errorf("expected export warning on 400, got: %q", buf.String())
	}
}
