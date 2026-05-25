package reporter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Slack payload tests ---

func TestBuildSlackPayload_AllPass(t *testing.T) {
	s := SuiteResultData{
		Project:  "my-api",
		Total:    3,
		Passed:   3,
		Duration: 250 * time.Millisecond,
	}
	tests := []TestResultData{
		{Name: "health", Passed: true},
		{Name: "login", Passed: true},
		{Name: "homepage", Passed: true},
	}

	body, err := buildSlackPayload(s, tests)
	if err != nil {
		t.Fatalf("buildSlackPayload: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(payload.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(payload.Attachments))
	}
	att := payload.Attachments[0]
	if att.Color != "#36a64f" {
		t.Errorf("color = %q, want green (#36a64f)", att.Color)
	}
	// Header block should contain project name and pass status
	if len(att.Blocks) < 2 {
		t.Fatalf("blocks = %d, want >= 2", len(att.Blocks))
	}
	headerText := att.Blocks[0].Text.Text
	if !contains(headerText, "my-api") {
		t.Errorf("header missing project name, got %q", headerText)
	}
	if !contains(headerText, "All tests passed") {
		t.Errorf("header missing pass message, got %q", headerText)
	}
	// Stats block
	statsText := att.Blocks[1].Text.Text
	if !contains(statsText, "Passed:* 3") {
		t.Errorf("stats missing pass count, got %q", statsText)
	}
	if !contains(statsText, "Failed:* 0") {
		t.Errorf("stats missing fail count, got %q", statsText)
	}
}

func TestBuildSlackPayload_SomeFail(t *testing.T) {
	s := SuiteResultData{
		Project:  "my-api",
		Total:    3,
		Passed:   1,
		Failed:   2,
		Duration: 500 * time.Millisecond,
	}
	tests := []TestResultData{
		{Name: "health", Passed: true},
		{Name: "login", Passed: false, Error: &testError{"timeout after 5s"}},
		{Name: "payment", Passed: false, Error: &testError{"status 503"}},
	}

	body, err := buildSlackPayload(s, tests)
	if err != nil {
		t.Fatalf("buildSlackPayload: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	att := payload.Attachments[0]
	if att.Color != "#E01E5A" {
		t.Errorf("color = %q, want red (#E01E5A)", att.Color)
	}

	// Should have header, stats, and failed tests blocks
	if len(att.Blocks) < 3 {
		t.Fatalf("blocks = %d, want >= 3", len(att.Blocks))
	}

	headerText := att.Blocks[0].Text.Text
	if !contains(headerText, "2 of 3 tests failed") {
		t.Errorf("header missing failure count, got %q", headerText)
	}

	// Find the failed tests block
	failedBlock := att.Blocks[2].Text.Text
	if !contains(failedBlock, "login") {
		t.Errorf("failed block missing 'login', got %q", failedBlock)
	}
	if !contains(failedBlock, "timeout after 5s") {
		t.Errorf("failed block missing error detail, got %q", failedBlock)
	}
	if !contains(failedBlock, "payment") {
		t.Errorf("failed block missing 'payment', got %q", failedBlock)
	}
}

func TestBuildSlackPayload_AllFail(t *testing.T) {
	s := SuiteResultData{
		Project: "broken-svc",
		Total:   2,
		Failed:  2,
	}
	tests := []TestResultData{
		{Name: "a", Passed: false, Error: &testError{"err a"}},
		{Name: "b", Passed: false, Error: &testError{"err b"}},
	}

	body, err := buildSlackPayload(s, tests)
	if err != nil {
		t.Fatalf("buildSlackPayload: %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	att := payload.Attachments[0]
	if att.Color != "#E01E5A" {
		t.Errorf("color = %q, want red", att.Color)
	}
	headerText := att.Blocks[0].Text.Text
	if !contains(headerText, "2 of 2 tests failed") {
		t.Errorf("header = %q, want '2 of 2 tests failed'", headerText)
	}
}

func TestBuildSlackPayload_AllowedFailureNotListed(t *testing.T) {
	s := SuiteResultData{
		Project:         "svc",
		Total:           2,
		Passed:          1,
		Failed:          1,
		AllowedFailures: 1,
	}
	tests := []TestResultData{
		{Name: "stable", Passed: true},
		{Name: "flaky", Passed: false, AllowedFailure: true, Error: &testError{"flaky"}},
	}

	body, err := buildSlackPayload(s, tests)
	if err != nil {
		t.Fatalf("buildSlackPayload: %v", err)
	}

	var payload slackPayload
	json.Unmarshal(body, &payload)

	// allowed_failure tests should NOT appear in the failed tests list
	for _, block := range payload.Attachments[0].Blocks {
		if block.Text != nil && contains(block.Text.Text, "flaky") && contains(block.Text.Text, "Failed tests") {
			t.Errorf("allowed_failure test 'flaky' should not appear in failed tests block")
		}
	}
}

// --- PagerDuty payload tests ---

func TestBuildPagerDutyPayload_Trigger(t *testing.T) {
	s := SuiteResultData{
		Project:  "prod-api",
		Total:    10,
		Passed:   7,
		Failed:   3,
		Duration: 2 * time.Second,
	}

	body, err := buildPagerDutyPayload(s, "routing-key-123", true, false)
	if err != nil {
		t.Fatalf("buildPagerDutyPayload: %v", err)
	}

	var payload pagerDutyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if payload.RoutingKey != "routing-key-123" {
		t.Errorf("routing_key = %q, want routing-key-123", payload.RoutingKey)
	}
	if payload.EventAction != "trigger" {
		t.Errorf("event_action = %q, want trigger", payload.EventAction)
	}
	if payload.DedupKey != "smokesig-prod-api" {
		t.Errorf("dedup_key = %q, want smokesig-prod-api", payload.DedupKey)
	}
	if payload.Payload == nil {
		t.Fatal("payload.payload is nil")
	}
	if payload.Payload.Severity != "error" {
		t.Errorf("severity = %q, want error (30%% failure)", payload.Payload.Severity)
	}
	if !contains(payload.Payload.Summary, "3/10") {
		t.Errorf("summary = %q, want to contain '3/10'", payload.Payload.Summary)
	}
	if !contains(payload.Payload.Summary, "prod-api") {
		t.Errorf("summary = %q, want to contain 'prod-api'", payload.Payload.Summary)
	}
}

func TestBuildPagerDutyPayload_Resolve(t *testing.T) {
	s := SuiteResultData{
		Project: "prod-api",
		Total:   10,
		Passed:  10,
	}

	body, err := buildPagerDutyPayload(s, "routing-key-123", false, true)
	if err != nil {
		t.Fatalf("buildPagerDutyPayload: %v", err)
	}

	var payload pagerDutyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if payload.EventAction != "resolve" {
		t.Errorf("event_action = %q, want resolve", payload.EventAction)
	}
	if payload.DedupKey != "smokesig-prod-api" {
		t.Errorf("dedup_key = %q, want smokesig-prod-api", payload.DedupKey)
	}
	if payload.Payload != nil {
		t.Errorf("resolve event should have nil payload, got %+v", payload.Payload)
	}
}

func TestPagerDutySeverity_Critical(t *testing.T) {
	// >50% failed = critical
	sev := pagerDutySeverity(6, 10)
	if sev != "critical" {
		t.Errorf("severity = %q, want critical (60%% failure)", sev)
	}
}

func TestPagerDutySeverity_Error(t *testing.T) {
	// <=50% failed = error
	sev := pagerDutySeverity(3, 10)
	if sev != "error" {
		t.Errorf("severity = %q, want error (30%% failure)", sev)
	}
}

func TestPagerDutySeverity_Exactly50(t *testing.T) {
	// Exactly 50% = error (not critical, since > is strict)
	sev := pagerDutySeverity(5, 10)
	if sev != "error" {
		t.Errorf("severity = %q, want error (exactly 50%%)", sev)
	}
}

func TestPagerDutySeverity_ZeroTotal(t *testing.T) {
	sev := pagerDutySeverity(0, 0)
	if sev != "error" {
		t.Errorf("severity = %q, want error (zero total)", sev)
	}
}

// --- Webhook "on" condition tests ---

func TestWebhookReporter_OnFailure_NoSendOnPass(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnFailure)
	wh.Summary(SuiteResultData{Project: "test", Total: 3, Passed: 3})

	if called {
		t.Error("webhook should not fire on all-pass with on=failure")
	}
}

func TestWebhookReporter_OnFailure_SendsOnFail(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnFailure)
	wh.TestResult(TestResultData{Name: "fail", Passed: false, Error: &testError{"oops"}})
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Failed: 1})

	if !called {
		t.Error("webhook should fire on failure with on=failure")
	}
}

func TestWebhookReporter_OnAlways_SendsOnPass(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnAlways)
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	if !called {
		t.Error("webhook should fire on all-pass with on=always")
	}
}

func TestWebhookReporter_OnAlways_SendsOnFail(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnAlways)
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Failed: 1})

	if !called {
		t.Error("webhook should fire on failure with on=always")
	}
}

func TestWebhookReporter_OnChange_FirstRunAlwaysSends(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnChange)
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	if !called {
		t.Error("webhook should fire on first run with on=change")
	}
}

func TestWebhookReporter_OnChange_NoSendOnSameState(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnChange)

	// First run: passes (sends because first run)
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
	if callCount != 1 {
		t.Fatalf("first run: callCount = %d, want 1", callCount)
	}

	// Reset test data for second run
	wh.tests = nil
	wh.prereqs = nil

	// Second run: still passes (should NOT send)
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
	if callCount != 1 {
		t.Errorf("second run same state: callCount = %d, want 1", callCount)
	}
}

func TestWebhookReporter_OnChange_SendsOnStateChange(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnChange)

	// First run: passes
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})
	if callCount != 1 {
		t.Fatalf("first run: callCount = %d, want 1", callCount)
	}

	// Reset test data
	wh.tests = nil
	wh.prereqs = nil

	// Second run: fails (state changed)
	wh.TestResult(TestResultData{Name: "broken", Passed: false})
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Failed: 1})
	if callCount != 2 {
		t.Errorf("state change: callCount = %d, want 2", callCount)
	}
}

// --- Integration: mock HTTP server receives formatted webhook ---

func TestWebhookReporter_SlackFormat_MockServer(t *testing.T) {
	var received slackPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q, want application/json", ct)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "", WebhookFormatSlack, WebhookOnAlways)
	wh.TestResult(TestResultData{Name: "pass-test", Passed: true, Duration: 50 * time.Millisecond})
	wh.TestResult(TestResultData{Name: "fail-test", Passed: false, Error: &testError{"connection refused"}, Duration: 100 * time.Millisecond})
	wh.Summary(SuiteResultData{
		Project:  "webhook-test",
		Total:    2,
		Passed:   1,
		Failed:   1,
		Duration: 150 * time.Millisecond,
	})

	if len(received.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(received.Attachments))
	}
	if received.Attachments[0].Color != "#E01E5A" {
		t.Errorf("color = %q, want red", received.Attachments[0].Color)
	}
}

func TestWebhookReporter_PagerDutyFormat_MockServer(t *testing.T) {
	var received pagerDutyPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "pd-routing-key", WebhookFormatPagerDuty, WebhookOnAlways)
	wh.TestResult(TestResultData{Name: "critical", Passed: false, Error: &testError{"down"}})
	wh.Summary(SuiteResultData{
		Project: "prod-svc",
		Total:   1,
		Failed:  1,
	})

	if received.RoutingKey != "pd-routing-key" {
		t.Errorf("routing_key = %q, want pd-routing-key", received.RoutingKey)
	}
	if received.EventAction != "trigger" {
		t.Errorf("event_action = %q, want trigger", received.EventAction)
	}
	if received.Payload == nil {
		t.Fatal("payload is nil")
	}
	if received.Payload.Severity != "critical" {
		t.Errorf("severity = %q, want critical (100%% failure)", received.Payload.Severity)
	}
}

func TestWebhookReporter_PagerDuty_ResolveAfterPreviousFailure(t *testing.T) {
	var lastPayload pagerDutyPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastPayload)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "pd-key", WebhookFormatPagerDuty, WebhookOnAlways)

	// First run: failure
	wh.TestResult(TestResultData{Name: "broken", Passed: false})
	wh.Summary(SuiteResultData{Project: "svc", Total: 1, Failed: 1})

	if lastPayload.EventAction != "trigger" {
		t.Fatalf("first run: event_action = %q, want trigger", lastPayload.EventAction)
	}

	// Reset test data
	wh.tests = nil
	wh.prereqs = nil

	// Second run: all pass (should resolve)
	wh.TestResult(TestResultData{Name: "fixed", Passed: true})
	wh.Summary(SuiteResultData{Project: "svc", Total: 1, Passed: 1})

	if lastPayload.EventAction != "resolve" {
		t.Errorf("second run: event_action = %q, want resolve", lastPayload.EventAction)
	}
	if lastPayload.DedupKey != "smokesig-svc" {
		t.Errorf("dedup_key = %q, want smokesig-svc", lastPayload.DedupKey)
	}
}

func TestWebhookReporter_JSONFormat_MockServer(t *testing.T) {
	var received jsonOutput
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "api-key", WebhookFormatJSON, WebhookOnAlways)
	wh.PrereqResult(PrereqResultData{Name: "docker", Passed: true})
	wh.TestResult(TestResultData{Name: "test1", Passed: true, Duration: 100 * time.Millisecond})
	wh.Summary(SuiteResultData{
		Project:  "json-test",
		Total:    1,
		Passed:   1,
		Duration: 100 * time.Millisecond,
	})

	if received.Project != "json-test" {
		t.Errorf("project = %q, want json-test", received.Project)
	}
	if len(received.Prerequisites) != 1 {
		t.Fatalf("prereqs = %d, want 1", len(received.Prerequisites))
	}
	if received.Prerequisites[0].Name != "docker" {
		t.Errorf("prereq name = %q, want docker", received.Prerequisites[0].Name)
	}
}

// --- Error handling ---

func TestWebhookReporter_WarnsOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	wh := NewWebhookReporter(srv.URL, "", WebhookFormatJSON, WebhookOnAlways)
	wh.warnOut = &buf
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	if !contains(buf.String(), "Warning: webhook to") {
		t.Errorf("expected warning, got %q", buf.String())
	}
}

func TestWebhookReporter_WarnsOnNetworkError(t *testing.T) {
	var buf bytes.Buffer
	wh := NewWebhookReporter("http://127.0.0.1:1/nonexistent", "", WebhookFormatJSON, WebhookOnAlways)
	wh.warnOut = &buf
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	if !contains(buf.String(), "Warning: failed to send webhook") {
		t.Errorf("expected warning, got %q", buf.String())
	}
}

func TestWebhookReporter_APIKeyHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "my-secret", WebhookFormatJSON, WebhookOnAlways)
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Passed: 1})

	if gotKey != "my-secret" {
		t.Errorf("X-API-Key = %q, want my-secret", gotKey)
	}
}

func TestWebhookReporter_PagerDuty_NoAPIKeyHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	wh := NewWebhookReporter(srv.URL, "pd-routing-key", WebhookFormatPagerDuty, WebhookOnAlways)
	wh.Summary(SuiteResultData{Project: "test", Total: 1, Failed: 1})

	if gotKey != "" {
		t.Errorf("PagerDuty should not send X-API-Key header, got %q", gotKey)
	}
}

func TestWebhookReporter_DefaultOnIsFailure(t *testing.T) {
	wh := NewWebhookReporter("http://example.com", "", WebhookFormatJSON, "")
	if wh.on != WebhookOnFailure {
		t.Errorf("default on = %q, want failure", wh.on)
	}
}

func TestWebhookReporter_DefaultFormatIsJSON(t *testing.T) {
	wh := NewWebhookReporter("http://example.com", "", "", WebhookOnAlways)
	if wh.format != WebhookFormatJSON {
		t.Errorf("default format = %q, want json", wh.format)
	}
}

// --- Reporter interface compliance ---

func TestWebhookReporter_ImplementsReporter(t *testing.T) {
	var _ Reporter = (*WebhookReporter)(nil)
}

// helpers

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
