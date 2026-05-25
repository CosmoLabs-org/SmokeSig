package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// WebhookFormat identifies the payload format for a webhook notification.
type WebhookFormat string

const (
	WebhookFormatSlack     WebhookFormat = "slack"
	WebhookFormatPagerDuty WebhookFormat = "pagerduty"
	WebhookFormatJSON      WebhookFormat = "json"
)

// WebhookCondition controls when a webhook fires.
type WebhookCondition string

const (
	WebhookOnFailure WebhookCondition = "failure"
	WebhookOnAlways  WebhookCondition = "always"
	WebhookOnChange  WebhookCondition = "change"
)

// WebhookReporter sends formatted notifications to webhook endpoints.
// It wraps the results into platform-specific payloads (Slack, PagerDuty, or raw JSON).
type WebhookReporter struct {
	endpoint string
	apiKey   string
	format   WebhookFormat
	on       WebhookCondition
	client   *http.Client
	warnOut  io.Writer
	prereqs  []PrereqResultData
	tests    []TestResultData

	// lastFailed tracks whether the previous run had failures,
	// used for "on: change" condition and PagerDuty resolve events.
	lastFailed *bool
}

// NewWebhookReporter creates a reporter that sends webhook notifications.
func NewWebhookReporter(endpoint, apiKey string, format WebhookFormat, on WebhookCondition) *WebhookReporter {
	if on == "" {
		on = WebhookOnFailure
	}
	if format == "" {
		format = WebhookFormatJSON
	}
	return &WebhookReporter{
		endpoint: endpoint,
		apiKey:   apiKey,
		format:   format,
		on:       on,
		client:   &http.Client{Timeout: 10 * time.Second},
		warnOut:  os.Stderr,
	}
}

func (w *WebhookReporter) PrereqStart(_ string) {}

func (w *WebhookReporter) PrereqResult(r PrereqResultData) {
	w.prereqs = append(w.prereqs, r)
}

func (w *WebhookReporter) TestStart(_ string) {}

func (w *WebhookReporter) TestResult(r TestResultData) {
	w.tests = append(w.tests, r)
}

func (w *WebhookReporter) Summary(s SuiteResultData) {
	hasFailed := s.Failed > 0

	// Check "on" condition
	if !w.shouldSend(hasFailed) {
		// Update tracking for next run
		w.lastFailed = &hasFailed
		return
	}

	var body []byte
	var contentType string
	var err error

	switch w.format {
	case WebhookFormatSlack:
		body, err = buildSlackPayload(s, w.tests)
		contentType = "application/json"
	case WebhookFormatPagerDuty:
		wasFailedBefore := w.lastFailed != nil && *w.lastFailed
		body, err = buildPagerDutyPayload(s, w.apiKey, hasFailed, wasFailedBefore)
		contentType = "application/json"
	default:
		body, err = buildWebhookJSONPayload(s, w.tests, w.prereqs)
		contentType = "application/json"
	}

	if err != nil {
		fmt.Fprintf(w.warnOut, "Warning: failed to build webhook payload for %s: %v\n", w.endpoint, err)
		w.lastFailed = &hasFailed
		return
	}

	w.sendPayload(body, contentType)
	w.lastFailed = &hasFailed
}

func (w *WebhookReporter) shouldSend(hasFailed bool) bool {
	switch w.on {
	case WebhookOnAlways:
		return true
	case WebhookOnChange:
		if w.lastFailed == nil {
			// First run — always send
			return true
		}
		return *w.lastFailed != hasFailed
	default: // WebhookOnFailure
		return hasFailed
	}
}

func (w *WebhookReporter) sendPayload(body []byte, contentType string) {
	req, err := http.NewRequest(http.MethodPost, w.endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(w.warnOut, "Warning: failed to send webhook to %s: %v\n", w.endpoint, err)
		return
	}
	req.Header.Set("Content-Type", contentType)
	if w.apiKey != "" && w.format != WebhookFormatPagerDuty {
		// PagerDuty routing key is embedded in the payload, not a header
		req.Header.Set("X-API-Key", w.apiKey)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		fmt.Fprintf(w.warnOut, "Warning: failed to send webhook to %s: %v\n", w.endpoint, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Fprintf(w.warnOut, "Warning: webhook to %s returned %s\n", w.endpoint, resp.Status)
	}
}

// --- Slack Block Kit payload ---

type slackPayload struct {
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func buildSlackPayload(s SuiteResultData, tests []TestResultData) ([]byte, error) {
	color := "#36a64f" // green
	statusEmoji := ":white_check_mark:"
	statusText := "All tests passed"
	if s.Failed > 0 {
		color = "#E01E5A" // red
		statusEmoji = ":x:"
		statusText = fmt.Sprintf("%d of %d tests failed", s.Failed, s.Total)
	}

	blocks := []slackBlock{
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("%s *SmokeSig — %s*\n%s", statusEmoji, s.Project, statusText),
			},
		},
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*Passed:* %d | *Failed:* %d | *Skipped:* %d | *Duration:* %dms",
					s.Passed, s.Failed, s.Skipped, s.Duration.Milliseconds()),
			},
		},
	}

	// Add failed test details
	if s.Failed > 0 {
		var failedDetails string
		for _, t := range tests {
			if !t.Passed && !t.Skipped && !t.AllowedFailure {
				errMsg := ""
				if t.Error != nil {
					errMsg = fmt.Sprintf(": %s", t.Error.Error())
				}
				failedDetails += fmt.Sprintf("• `%s`%s\n", t.Name, errMsg)
			}
		}
		if failedDetails != "" {
			blocks = append(blocks, slackBlock{
				Type: "section",
				Text: &slackText{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Failed tests:*\n%s", failedDetails),
				},
			})
		}
	}

	// Add CI link if available
	if ciURL := detectCIURL(); ciURL != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("<%s|View CI Run>", ciURL),
			},
		})
	}

	payload := slackPayload{
		Attachments: []slackAttachment{
			{
				Color:  color,
				Blocks: blocks,
			},
		},
	}

	return json.Marshal(payload)
}

// --- PagerDuty Events API v2 payload ---

type pagerDutyPayload struct {
	RoutingKey  string          `json:"routing_key"`
	EventAction string          `json:"event_action"` // trigger, resolve
	DedupKey    string          `json:"dedup_key"`
	Payload     *pagerDutyEvent `json:"payload,omitempty"`
}

type pagerDutyEvent struct {
	Summary   string            `json:"summary"`
	Source    string            `json:"source"`
	Severity  string            `json:"severity"` // critical, error, warning, info
	Timestamp string            `json:"timestamp"`
	CustomDetails map[string]any `json:"custom_details,omitempty"`
}

func buildPagerDutyPayload(s SuiteResultData, routingKey string, hasFailed bool, wasFailedBefore bool) ([]byte, error) {
	// Resolve routing key: explicit param > env var
	if routingKey == "" {
		routingKey = os.Getenv("SMOKESIG_PAGERDUTY_KEY")
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	dedupKey := fmt.Sprintf("smokesig-%s", s.Project)

	// If all pass and was previously failed, send resolve
	if !hasFailed && wasFailedBefore {
		payload := pagerDutyPayload{
			RoutingKey:  routingKey,
			EventAction: "resolve",
			DedupKey:    dedupKey,
		}
		return json.Marshal(payload)
	}

	// Trigger event for failures
	severity := pagerDutySeverity(s.Failed, s.Total)

	payload := pagerDutyPayload{
		RoutingKey:  routingKey,
		EventAction: "trigger",
		DedupKey:    dedupKey,
		Payload: &pagerDutyEvent{
			Summary:   fmt.Sprintf("SmokeSig: %d/%d tests failed for %s", s.Failed, s.Total, s.Project),
			Source:    hostname,
			Severity:  severity,
			Timestamp: now,
			CustomDetails: map[string]any{
				"project":  s.Project,
				"total":    s.Total,
				"passed":   s.Passed,
				"failed":   s.Failed,
				"skipped":  s.Skipped,
				"duration": s.Duration.String(),
			},
		},
	}

	return json.Marshal(payload)
}

// pagerDutySeverity returns "critical" if >50% of tests failed, otherwise "error".
func pagerDutySeverity(failed, total int) string {
	if total == 0 {
		return "error"
	}
	if float64(failed)/float64(total) > 0.5 {
		return "critical"
	}
	return "error"
}

// --- JSON webhook payload (reuses jsonOutput) ---

func buildWebhookJSONPayload(s SuiteResultData, tests []TestResultData, prereqs []PrereqResultData) ([]byte, error) {
	out := jsonOutput{
		Project:         s.Project,
		Total:           s.Total,
		Passed:          s.Passed,
		Failed:          s.Failed,
		Skipped:         s.Skipped,
		AllowedFailures: s.AllowedFailures,
		DurationMs:      s.Duration.Milliseconds(),
	}
	for _, pr := range prereqs {
		jp := jsonPrereq{
			Name:   pr.Name,
			Passed: pr.Passed,
			Output: pr.Output,
			Hint:   pr.Hint,
		}
		if pr.Error != nil {
			jp.Error = pr.Error.Error()
		}
		out.Prerequisites = append(out.Prerequisites, jp)
	}
	for _, t := range tests {
		jt := jsonTest{
			Name:           t.Name,
			Passed:         t.Passed,
			Skipped:        t.Skipped,
			AllowedFailure: t.AllowedFailure,
			DurationMs:     t.Duration.Milliseconds(),
			Assertions:     t.Assertions,
		}
		if t.Error != nil {
			jt.Error = t.Error.Error()
		}
		out.Tests = append(out.Tests, jt)
	}
	return json.Marshal(out)
}

// detectCIURL tries to find a CI job URL from common environment variables.
func detectCIURL() string {
	// GitHub Actions
	if server := os.Getenv("GITHUB_SERVER_URL"); server != "" {
		repo := os.Getenv("GITHUB_REPOSITORY")
		runID := os.Getenv("GITHUB_RUN_ID")
		if repo != "" && runID != "" {
			return fmt.Sprintf("%s/%s/actions/runs/%s", server, repo, runID)
		}
	}
	// GitLab CI
	if url := os.Getenv("CI_JOB_URL"); url != "" {
		return url
	}
	// CircleCI
	if url := os.Getenv("CIRCLE_BUILD_URL"); url != "" {
		return url
	}
	// Jenkins
	if url := os.Getenv("BUILD_URL"); url != "" {
		return url
	}
	return ""
}
