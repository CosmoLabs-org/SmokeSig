package reporter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPushReporter_SummaryPOSTs(t *testing.T) {
	var received jsonOutput
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json content-type, got %s", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]bool{"stored": true})
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "")
	p.TestResult(TestResultData{
		Name:     "health check",
		Passed:   true,
		Duration: 100 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "http", Expected: "200", Actual: "200", Passed: true},
		},
	})
	p.Summary(SuiteResultData{
		Project:  "cosmo-api",
		Total:    1,
		Passed:   1,
		Duration: 100 * time.Millisecond,
	})

	if received.Project != "cosmo-api" {
		t.Errorf("project = %q, want cosmo-api", received.Project)
	}
	if received.Total != 1 {
		t.Errorf("total = %d, want 1", received.Total)
	}
	if received.Passed != 1 {
		t.Errorf("passed = %d, want 1", received.Passed)
	}
	if len(received.Tests) != 1 {
		t.Fatalf("tests len = %d, want 1", len(received.Tests))
	}
	if received.Tests[0].Name != "health check" {
		t.Errorf("test name = %q, want health check", received.Tests[0].Name)
	}
	if received.DurationMs != 100 {
		t.Errorf("duration_ms = %d, want 100", received.DurationMs)
	}
}

func TestPushReporter_WithAPIKey(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "secret-key-123")
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	if gotKey != "secret-key-123" {
		t.Errorf("api key = %q, want secret-key-123", gotKey)
	}
}

func TestPushReporter_UnreachableEndpoint(t *testing.T) {
	p := NewPushReporter("http://127.0.0.1:1/nonexistent", "")
	p.TestResult(TestResultData{Name: "test", Passed: true})
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
	// Should not panic
}

func TestPushReporter_Prerequisites(t *testing.T) {
	var received jsonOutput
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "")
	p.PrereqResult(PrereqResultData{
		Name:   "docker",
		Passed: true,
		Output: "Docker version 24.0",
	})
	p.Summary(SuiteResultData{Project: "test", Total: 0})

	if len(received.Prerequisites) != 1 {
		t.Fatalf("prereqs len = %d, want 1", len(received.Prerequisites))
	}
	if received.Prerequisites[0].Name != "docker" {
		t.Errorf("prereq name = %q, want docker", received.Prerequisites[0].Name)
	}
}

func TestPushReporter_FailedTestWithError(t *testing.T) {
	var received jsonOutput
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "")
	p.TestResult(TestResultData{
		Name:   "failing test",
		Passed: false,
		Error:  errTest,
	})
	p.Summary(SuiteResultData{Project: "test", Total: 1, Failed: 1})

	if received.Tests[0].Error != "test error" {
		t.Errorf("error = %q, want 'test error'", received.Tests[0].Error)
	}
	if received.Failed != 1 {
		t.Errorf("failed = %d, want 1", received.Failed)
	}
}

func TestPushReporter_Non200Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "key")
	p.TestResult(TestResultData{Name: "test", Passed: true})
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
}

func TestPushReporter_WarnsOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	p := NewPushReporter(srv.URL, "")
	p.warnOut = &buf
	p.TestResult(TestResultData{Name: "test", Passed: true})
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	got := buf.String()
	if !strings.Contains(got, "Warning: failed to push results to") {
		t.Errorf("expected push warning, got %q", got)
	}
	if !strings.Contains(got, "500 Internal Server Error") {
		t.Errorf("expected status in warning, got %q", got)
	}
}

func TestPushReporter_WarnsOnNetworkError(t *testing.T) {
	var buf bytes.Buffer
	p := NewPushReporter("http://127.0.0.1:1/nonexistent", "")
	p.warnOut = &buf
	p.TestResult(TestResultData{Name: "test", Passed: true})
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	got := buf.String()
	if !strings.Contains(got, "Warning: failed to push results to") {
		t.Errorf("expected push warning on network error, got %q", got)
	}
}

func TestPushReporter_WarnsOnMalformedURL(t *testing.T) {
	var buf bytes.Buffer
	p := NewPushReporter("://not-valid", "")
	p.warnOut = &buf
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	got := buf.String()
	if !strings.Contains(got, "Warning: failed to push results to") {
		t.Errorf("expected push warning on malformed URL, got %q", got)
	}
}

func TestPushReporter_NoWarningOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	p := NewPushReporter(srv.URL, "")
	p.warnOut = &buf
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	if buf.Len() > 0 {
		t.Errorf("expected no warnings on success, got %q", buf.String())
	}
}

func TestPushReporter_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "")
	p.client.Timeout = 10 * time.Millisecond
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
}

func TestPushReporter_EmptyURL(t *testing.T) {
	p := NewPushReporter("", "")
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
}

func TestPushReporter_MalformedURL(t *testing.T) {
	p := NewPushReporter("://not-valid", "")
	p.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
}

func TestPushReporter_PrereqWithError_Serialized(t *testing.T) {
	var body jsonOutput
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "")
	p.PrereqResult(PrereqResultData{
		Name:   "docker",
		Passed: false,
		Error:  &testError{"daemon not running"},
	})
	p.Summary(SuiteResultData{Project: "push-prereq-err", Total: 0})

	if len(body.Prerequisites) != 1 {
		t.Fatalf("prereqs = %d, want 1", len(body.Prerequisites))
	}
	if body.Prerequisites[0].Error != "daemon not running" {
		t.Errorf("prereq error = %q, want 'daemon not running'", body.Prerequisites[0].Error)
	}
}

var errTest = &testError{"test error"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
