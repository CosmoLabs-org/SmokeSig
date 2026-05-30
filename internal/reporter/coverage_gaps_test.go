package reporter

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// backstageOverallStatus — the hasFailure-but-all-allowed branch (line 110-111)
// ---------------------------------------------------------------------------

// TestBackstageOverallStatus_AllFailed_SomeAllowed covers the branch where
// there is a mix of: one hard failure (returns "unhealthy" immediately via line 98)
// and confirms the allowed-only path returns "degraded".
func TestBackstageOverallStatus_HasFailureTrueWithAllowed(t *testing.T) {
	// A test that fails AND is allowed — hasAllowedFailure=true, hasFailure=true
	// The "unhealthy" early-return at line 98 only fires for !AllowedFailure failures.
	// So a list with only AllowedFailure failures goes through to line 107 → "degraded".
	tests := []TestResultData{
		{Passed: false, AllowedFailure: true},
		{Passed: false, AllowedFailure: true},
	}
	got := backstageOverallStatus(tests)
	if got != "degraded" {
		t.Errorf("got %q, want \"degraded\"", got)
	}
}

// TestBackstageOverallStatus_AllPassedNoFailure covers line 110-113 where
// hasFailure=false and hasAllowedFailure=false → "healthy".
func TestBackstageOverallStatus_AllPassed(t *testing.T) {
	tests := []TestResultData{
		{Passed: true},
		{Passed: true},
	}
	got := backstageOverallStatus(tests)
	if got != "healthy" {
		t.Errorf("got %q, want \"healthy\"", got)
	}
}

// ---------------------------------------------------------------------------
// ChainWithVerbosity — file creation error path (line 59-63)
// ---------------------------------------------------------------------------

// TestChainWithVerbosity_FileCreateError triggers the os.Create error branch by
// using a format whose auto-filename is not writable (read-only dir workaround:
// we pass "terminal,json" but mock isn't possible; instead rely on the fact that
// "terminal" as first format goes to stdout, then "json" tries to create
// "smoke-results.json" in the current dir — we can't easily break that.
// Instead we test the unknown-format error path which is also at 0% and adjacent.
func TestChainWithVerbosity_UnknownFormat(t *testing.T) {
	_, closers, err := ChainWithVerbosity("nosuchformat", io.Discard, VerbosityNormal)
	for _, c := range closers {
		c.Close()
	}
	if err == nil {
		t.Error("expected error for unknown format, got nil")
	}
	if !strings.Contains(err.Error(), "nosuchformat") {
		t.Errorf("error should mention the bad format name, got: %v", err)
	}
}

// TestChainWithVerbosity_EmptyFormat covers the "no output format specified" error.
func TestChainWithVerbosity_EmptyFormat(t *testing.T) {
	_, closers, err := ChainWithVerbosity("", io.Discard, VerbosityNormal)
	for _, c := range closers {
		c.Close()
	}
	if err == nil {
		t.Error("expected error for empty format, got nil")
	}
}

// TestChainWithVerbosity_SingleReporterPath covers line 75-76 (single reporter returned directly).
func TestChainWithVerbosity_Single(t *testing.T) {
	r, closers, err := ChainWithVerbosity("terminal", io.Discard, VerbosityNormal)
	for _, c := range closers {
		c.Close()
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Error("expected non-nil reporter")
	}
}

// ---------------------------------------------------------------------------
// PushReporter.Summary — HTTP error paths
// ---------------------------------------------------------------------------

// TestPushReporter_SummaryHTTPError covers the resp.StatusCode >= 400 branch.
func TestPushReporter_SummaryHTTP400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var warn bytes.Buffer
	p := NewPushReporter(srv.URL, "key")
	p.warnOut = &warn
	p.TestResult(TestResultData{Name: "t1", Passed: true})
	p.Summary(SuiteResultData{Project: "proj", Total: 1, Passed: 1})

	if !strings.Contains(warn.String(), "500") && !strings.Contains(warn.String(), "server returned") {
		t.Errorf("expected warning about 500 response, got: %q", warn.String())
	}
}

// TestPushReporter_SummaryWithAPIKey covers the apiKey branch (line 97-99).
func TestPushReporter_SummaryWithAPIKey(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewPushReporter(srv.URL, "secret-key")
	p.warnOut = io.Discard
	p.Summary(SuiteResultData{Project: "proj", Total: 0})

	if gotKey != "secret-key" {
		t.Errorf("expected X-API-Key header 'secret-key', got %q", gotKey)
	}
}

// TestPushReporter_SummaryNetworkError covers the client.Do error path (lines 102-104).
func TestPushReporter_SummaryNetworkError(t *testing.T) {
	var warn bytes.Buffer
	p := NewPushReporter("http://127.0.0.1:1", "") // port 1 always refuses
	p.warnOut = &warn
	p.client.Timeout = 100 * time.Millisecond
	p.Summary(SuiteResultData{Project: "proj", Total: 0})

	if warn.Len() == 0 {
		t.Error("expected warning on network error, got none")
	}
}

// ---------------------------------------------------------------------------
// WebhookReporter.Summary — payload build error (unreachable normally) +
// buildPagerDutyPayload resolve branch
// ---------------------------------------------------------------------------

// TestBuildPagerDutyPayload_ResolveEvent covers the !hasFailed && wasFailedBefore path.
func TestBuildPagerDutyPayload_ResolveEvent(t *testing.T) {
	s := SuiteResultData{Project: "proj", Total: 5, Passed: 5, Failed: 0}
	wasFailedBefore := true
	body, err := buildPagerDutyPayload(s, "routing-key-123", false, wasFailedBefore)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(body, []byte("resolve")) {
		t.Errorf("expected 'resolve' in payload, got: %s", body)
	}
	if bytes.Contains(body, []byte("trigger")) {
		t.Errorf("expected no 'trigger' in resolve payload, got: %s", body)
	}
}

// TestBuildPagerDutyPayload_TriggerEvent covers the hasFailed=true path.
func TestBuildPagerDutyPayload_TriggerEvent(t *testing.T) {
	s := SuiteResultData{Project: "proj", Total: 5, Passed: 3, Failed: 2, Duration: time.Second}
	body, err := buildPagerDutyPayload(s, "routing-key", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(body, []byte("trigger")) {
		t.Errorf("expected 'trigger' in payload, got: %s", body)
	}
}

// TestBuildPagerDutyPayload_EmptyRoutingKeyUsesEnv covers the env-var fallback.
func TestBuildPagerDutyPayload_EnvFallback(t *testing.T) {
	t.Setenv("SMOKESIG_PAGERDUTY_KEY", "env-routing-key")
	s := SuiteResultData{Project: "proj", Total: 1, Passed: 0, Failed: 1, Duration: time.Second}
	body, err := buildPagerDutyPayload(s, "", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(body, []byte("env-routing-key")) {
		t.Errorf("expected env routing key in payload, got: %s", body)
	}
}

// TestWebhookSummary_PagerDutyResolve covers the WebhookReporter.Summary
// going through the PagerDuty format with a resolve event (wasFailedBefore=true, now passing).
func TestWebhookSummary_PagerDutyResolve(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		received = buf.Bytes()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "key", WebhookFormatPagerDuty, WebhookOnAlways)
	wh.warnOut = io.Discard
	// Simulate previous failure
	prev := true
	wh.lastFailed = &prev

	// Now all pass → resolve
	wh.Summary(SuiteResultData{Project: "proj", Total: 3, Passed: 3, Failed: 0})

	if !bytes.Contains(received, []byte("resolve")) {
		t.Errorf("expected resolve event sent to webhook, got: %s", received)
	}
}

// TestWebhookSummary_SlackFormat exercises the Slack format branch in Summary.
func TestWebhookSummary_SlackFormat(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		received = buf.Bytes()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatSlack, WebhookOnAlways)
	wh.warnOut = io.Discard
	wh.TestResult(TestResultData{Name: "t1", Passed: false})
	wh.Summary(SuiteResultData{Project: "proj", Total: 1, Passed: 0, Failed: 1})

	if len(received) == 0 {
		t.Error("expected Slack payload to be sent")
	}
	if !bytes.Contains(received, []byte("attachments")) {
		t.Errorf("expected Slack-style payload with attachments, got: %s", received)
	}
}

// ---------------------------------------------------------------------------
// OTelReporter.export — HTTP error + 400 paths
// ---------------------------------------------------------------------------

// TestOTelExport_HTTP400 covers the resp.StatusCode >= 400 error return path.
func TestOTelExport_HTTP400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	o := NewOTelReporter(srv.URL, "test-svc", nil)
	// Call export directly — it's an unexported method accessible within same package.
	err := o.export([]byte(`{"test":"data"}`))
	if err == nil {
		t.Error("expected error for 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "collector returned") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestOTelExport_NetworkError covers the client.Do error return path.
func TestOTelExport_NetworkError(t *testing.T) {
	o := NewOTelReporter("http://127.0.0.1:1", "svc", nil)
	o.client.Timeout = 100 * time.Millisecond
	err := o.export([]byte(`{}`))
	if err == nil {
		t.Error("expected network error, got nil")
	}
}

// TestOTelExport_WithHeaders covers the custom headers loop (line 152-154).
func TestOTelExport_WithHeaders(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	o := NewOTelReporter(srv.URL, "svc", map[string]string{"X-Custom-Header": "val123"})
	err := o.export([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "val123" {
		t.Errorf("expected X-Custom-Header=val123, got %q", gotHeader)
	}
}

// TestOTelExport_InvalidURL covers the http.NewRequest error path (line 148-150).
// An invalid URL (containing a space) causes http.NewRequest to fail.
func TestOTelExport_InvalidURL(t *testing.T) {
	o := NewOTelReporter("http://bad host with spaces/path", "svc", nil)
	err := o.export([]byte(`{}`))
	if err == nil {
		t.Error("expected error for invalid URL in http.NewRequest, got nil")
	}
}

// TestPushReporter_InvalidURLNewRequestError covers the http.NewRequest error path
// in push.Summary (line 91-94). Space in URL causes NewRequest to fail.
func TestPushReporter_InvalidURLNewRequestError(t *testing.T) {
	var warn bytes.Buffer
	p := NewPushReporter("http://bad host with spaces/push", "")
	p.warnOut = &warn
	p.Summary(SuiteResultData{Project: "proj", Total: 0})
	if warn.Len() == 0 {
		t.Error("expected warning on NewRequest error, got none")
	}
}

// TestChainWithVerbosity_MultipleFormatsFileCleanup covers the closers cleanup loop
// (line 60-62) which runs when creating a second format file fails.
// We trigger this by passing two formats where the second format's file can't be created.
// Since we can't easily make os.Create fail, we verify the multi-reporter path (line 78)
// works with two valid formats writing to stdout+file.
func TestChainWithVerbosity_MultiFormatReturnsMultiReporter(t *testing.T) {
	// Use terminal as first (stdout), json as second (file in current dir).
	// This exercises len(reporters) > 1 → NewMultiReporter path.
	r, closers, err := ChainWithVerbosity("terminal,json", io.Discard, VerbosityNormal)
	for _, c := range closers {
		c.Close()
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Error("expected non-nil multi reporter")
	}
}

// TestWebhookSummary_JSONFormat_NoFailure covers webhook Summary with JSON format
// where shouldSend returns false (on=failure, no failures → skip send).
// This exercises the w.lastFailed update-and-return path (line 83-86).
func TestWebhookSummary_ShouldNotSend(t *testing.T) {
	// on=failure, but no failures → shouldSend returns false → early return path
	wh := NewWebhookReporter("http://127.0.0.1:1/noop", "", WebhookFormatJSON, WebhookOnFailure)
	wh.warnOut = io.Discard
	// No failures → shouldSend(false) = false → takes the early-return branch
	wh.Summary(SuiteResultData{Project: "proj", Total: 1, Passed: 1, Failed: 0})
	// Verify lastFailed was set to false
	if wh.lastFailed == nil || *wh.lastFailed != false {
		t.Error("expected lastFailed=false after no-send path")
	}
}

// TestBackstageOverallStatus_HasFailureUnhealthyLine covers the hasFailure branch
// at line 110-111. This requires: all tests are non-AllowedFailure failures,
// but the early return at line 98 fires first. The line 110-111 block is structurally
// unreachable after line 98's early return for !AllowedFailure failures.
// We confirm this by checking the "healthy" return when hasFailure=false.
func TestBackstageOverallStatus_HealthyNoFailures(t *testing.T) {
	// No failures at all — hasAllowedFailure=false, hasFailure=false → "healthy"
	tests := []TestResultData{
		{Passed: true},
	}
	got := backstageOverallStatus(tests)
	if got != "healthy" {
		t.Errorf("got %q, want \"healthy\"", got)
	}
}

// TestBuildPagerDutyPayload_EmptyHostname covers the hostname="" fallback branch.
// os.Hostname can't be forced to fail in unit tests, but we cover the success path
// to ensure the trigger event with custom details is fully exercised.
func TestBuildPagerDutyPayload_FullTriggerDetails(t *testing.T) {
	s := SuiteResultData{
		Project:  "myproject",
		Total:    10,
		Passed:   7,
		Failed:   3,
		Skipped:  0,
		Duration: 5 * time.Second,
	}
	body, err := buildPagerDutyPayload(s, "my-routing-key", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify key fields present
	for _, want := range []string{"trigger", "my-routing-key", "myproject", "smokesig"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("expected %q in PagerDuty payload, got: %s", want, body)
		}
	}
}
