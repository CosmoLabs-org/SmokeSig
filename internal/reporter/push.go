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

// PushReporter POSTs smoke test results to a remote URL on Summary().
// It reuses the same jsonOutput format as the JSON reporter.
type PushReporter struct {
	endpoint string
	apiKey   string
	client   *http.Client
	warnOut  io.Writer // warning output; defaults to os.Stderr
	prereqs  []PrereqResultData
	tests    []TestResultData
}

// NewPushReporter creates a reporter that POSTs results to endpoint.
func NewPushReporter(endpoint, apiKey string) *PushReporter {
	return &PushReporter{
		endpoint: endpoint,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 10 * time.Second},
		warnOut:  os.Stderr,
	}
}

func (p *PushReporter) PrereqStart(_ string) {}

func (p *PushReporter) PrereqResult(r PrereqResultData) {
	p.prereqs = append(p.prereqs, r)
}

func (p *PushReporter) TestStart(_ string) {}

func (p *PushReporter) TestResult(r TestResultData) {
	p.tests = append(p.tests, r)
}

func (p *PushReporter) Summary(s SuiteResultData) {
	out := jsonOutput{
		Project:         s.Project,
		Total:           s.Total,
		Passed:          s.Passed,
		Failed:          s.Failed,
		Skipped:         s.Skipped,
		AllowedFailures: s.AllowedFailures,
		DurationMs:      s.Duration.Milliseconds(),
	}

	for _, pr := range p.prereqs {
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

	for _, t := range p.tests {
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

	body, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(p.warnOut, "⚠️  Warning: failed to push results to %s: %v\n", p.endpoint, err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(p.warnOut, "⚠️  Warning: failed to push results to %s: %v\n", p.endpoint, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("X-API-Key", p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		fmt.Fprintf(p.warnOut, "⚠️  Warning: failed to push results to %s: %v\n", p.endpoint, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Fprintf(p.warnOut, "⚠️  Warning: failed to push results to %s: server returned %s\n", p.endpoint, resp.Status)
	}
}
