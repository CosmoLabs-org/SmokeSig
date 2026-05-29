package reporter

import (
	"io"
	"testing"
)

// TestBackstage_NoOpMethods directly calls PrereqStart, PrereqResult, TestStart on Backstage.
func TestBackstage_NoOpMethods(t *testing.T) {
	b := NewBackstage(io.Discard)
	b.PrereqStart("prereq-name")
	b.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	b.TestStart("test-name")
}

// TestGitHubActions_NoOpMethodsDirect directly calls PrereqStart, PrereqResult, TestStart on GitHubActions.
func TestGitHubActions_NoOpMethodsDirect(t *testing.T) {
	g := NewGitHubActions(io.Discard)
	g.PrereqStart("prereq-name")
	g.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	g.TestStart("test-name")
}

// TestJSON_NoOpMethods directly calls PrereqStart and TestStart on JSON reporter.
func TestJSON_NoOpMethods(t *testing.T) {
	j := NewJSON(io.Discard)
	j.PrereqStart("prereq-name")
	j.TestStart("test-name")
}

// TestJUnit_NoOpMethods directly calls PrereqStart, PrereqResult, TestStart on JUnit reporter.
func TestJUnit_NoOpMethods(t *testing.T) {
	j := NewJUnit(io.Discard)
	j.PrereqStart("prereq-name")
	j.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	j.TestStart("test-name")
}

// TestOTel_NoOpMethods directly calls PrereqStart, PrereqResult, TestStart on OTelReporter.
func TestOTel_NoOpMethods(t *testing.T) {
	o := NewOTelReporter("http://127.0.0.1:1/noop", "test", nil)
	o.PrereqStart("prereq-name")
	o.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	o.TestStart("test-name")
}

// TestPrometheus_NoOpMethods directly calls PrereqStart, PrereqResult, TestStart on Prometheus reporter.
func TestPrometheus_NoOpMethods(t *testing.T) {
	p := NewPrometheus(io.Discard)
	p.PrereqStart("prereq-name")
	p.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
	p.TestStart("test-name")
}

// TestTAP_NoOpMethods directly calls PrereqStart and PrereqResult on TAP reporter.
func TestTAP_NoOpMethods(t *testing.T) {
	tap := NewTAP(io.Discard)
	tap.PrereqStart("prereq-name")
	tap.PrereqResult(PrereqResultData{Name: "prereq", Passed: true})
}

// TestPushReporter_NoOpMethods directly calls PrereqStart and TestStart on PushReporter.
func TestPushReporter_NoOpMethods(t *testing.T) {
	p := NewPushReporter("http://127.0.0.1:1/noop", "")
	p.PrereqStart("prereq-name")
	p.TestStart("test-name")
}

// TestWebhookReporter_NoOpMethods directly calls PrereqStart and TestStart on WebhookReporter.
func TestWebhookReporter_NoOpMethods(t *testing.T) {
	wh := NewWebhookReporter("http://127.0.0.1:1/noop", "", WebhookFormatJSON, WebhookOnFailure)
	wh.PrereqStart("prereq-name")
	wh.TestStart("test-name")
}
