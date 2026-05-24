package reporter

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// OTelReporter emits smoke test results as OTLP JSON to an OTel collector.
type OTelReporter struct {
	endpoint   string
	service    string
	headers    map[string]string
	traceID    string
	client     *http.Client
	warnOut    io.Writer // warning output; defaults to os.Stderr
	mu         sync.Mutex
	spanIndex  uint64
	wg         sync.WaitGroup
	errsMu     sync.Mutex
	errs       []error
}

// NewOTelReporter creates a reporter that exports spans to the given OTLP HTTP endpoint.
func NewOTelReporter(endpoint, service string, headers map[string]string) *OTelReporter {
	var tid [16]byte
	rand.Read(tid[:])
	return &OTelReporter{
		endpoint: endpoint,
		service:  service,
		headers:  headers,
		traceID:  hex.EncodeToString(tid[:]),
		client:   &http.Client{Timeout: 5 * time.Second},
		warnOut:  os.Stderr,
	}
}

func (o *OTelReporter) PrereqStart(_ string)             {}
func (o *OTelReporter) PrereqResult(_ PrereqResultData) {}

func (o *OTelReporter) TestStart(_ string) {}

func (o *OTelReporter) TestResult(r TestResultData) {
	spanID := o.nextSpanID()
	status := "PASS"
	if r.Skipped {
		status = "SKIP"
	} else if r.AllowedFailure {
		status = "ALLOWED_FAILURE"
	} else if !r.Passed {
		status = "FAIL"
	}

	attrs := []otlpAttribute{
		{Key: "smoke.passed", Value: otlpValue{StringValue: fmt.Sprintf("%v", r.Passed)}},
		{Key: "smoke.status", Value: otlpValue{StringValue: status}},
		{Key: "smoke.duration_ms", Value: otlpValue{StringValue: fmt.Sprintf("%.0f", float64(r.Duration.Milliseconds()))}},
	}
	for _, a := range r.Assertions {
		attrs = append(attrs, otlpAttribute{
			Key:   fmt.Sprintf("smoke.assertion.%s.passed", a.Type),
			Value: otlpValue{StringValue: fmt.Sprintf("%v", a.Passed)},
		})
	}

	span := otlpSpan{
		Name:          o.service + "/" + r.Name,
		TraceID:       o.traceID,
		SpanID:        spanID,
		StartTime:     time.Now().Add(-r.Duration).Format(time.RFC3339Nano),
		EndTime:       time.Now().Format(time.RFC3339Nano),
		Status:        otlpSpanStatus{Code: boolToIntStr(r.Passed)},
		Attributes:    attrs,
	}

	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		if err := o.export(otlpPayload(o.service, []otlpSpan{span})); err != nil {
			o.errsMu.Lock()
			o.errs = append(o.errs, err)
			o.errsMu.Unlock()
		}
	}()
}

func (o *OTelReporter) Summary(s SuiteResultData) {
	spanID := o.nextSpanID()
	status := "PASS"
	if s.Failed > 0 {
		status = "FAIL"
	}

	attrs := []otlpAttribute{
		{Key: "smoke.suite.project", Value: otlpValue{StringValue: s.Project}},
		{Key: "smoke.suite.total", Value: otlpValue{StringValue: fmt.Sprintf("%d", s.Total)}},
		{Key: "smoke.suite.passed", Value: otlpValue{StringValue: fmt.Sprintf("%d", s.Passed)}},
		{Key: "smoke.suite.failed", Value: otlpValue{StringValue: fmt.Sprintf("%d", s.Failed)}},
		{Key: "smoke.suite.status", Value: otlpValue{StringValue: status}},
	}

	span := otlpSpan{
		Name:          o.service + "/suite",
		TraceID:       o.traceID,
		SpanID:        spanID,
		StartTime:     time.Now().Add(-s.Duration).Format(time.RFC3339Nano),
		EndTime:       time.Now().Format(time.RFC3339Nano),
		Status:        otlpSpanStatus{Code: boolToIntStr(s.Failed == 0)},
		Attributes:    attrs,
	}

	// Wait for any in-flight TestResult exports to finish.
	o.wg.Wait()

	// Export summary span synchronously so warnings print after all test output.
	if err := o.export(otlpPayload(o.service, []otlpSpan{span})); err != nil {
		o.errsMu.Lock()
		o.errs = append(o.errs, err)
		o.errsMu.Unlock()
	}

	// Print all collected warnings to stderr.
	o.errsMu.Lock()
	defer o.errsMu.Unlock()
	for _, e := range o.errs {
		fmt.Fprintf(o.warnOut, "⚠️  Warning: failed to export telemetry: %v\n", e)
	}
}

func (o *OTelReporter) nextSpanID() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.spanIndex++
	var sid [8]byte
	rand.Read(sid[:])
	return hex.EncodeToString(sid[:])
}

func (o *OTelReporter) export(payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, o.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("collector returned %s", resp.Status)
	}
	return nil
}

// OTLP JSON types

type otlpAttribute struct {
	Key   string     `json:"key"`
	Value otlpValue  `json:"value"`
}

type otlpValue struct {
	StringValue string `json:"stringValue,omitempty"`
}

type otlpSpanStatus struct {
	Code string `json:"code,omitempty"` // "0" = unset, "1" = ok, "2" = error
}

type otlpSpan struct {
	Name       string          `json:"name"`
	TraceID    string          `json:"traceId"`
	SpanID     string          `json:"spanId"`
	StartTime  string          `json:"startTime"`
	EndTime    string          `json:"endTime"`
	Status     otlpSpanStatus  `json:"status"`
	Attributes []otlpAttribute `json:"attributes,omitempty"`
}

func otlpPayload(service string, spans []otlpSpan) []byte {
	payload := map[string]any{
		"resourceSpans": []map[string]any{
			{
				"resource": map[string]any{
					"attributes": []otlpAttribute{
						{Key: "service.name", Value: otlpValue{StringValue: service}},
					},
				},
				"scopeSpans": []map[string]any{
					{"spans": spans},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func boolToIntStr(ok bool) string {
	if ok {
		return "1"
	}
	return "2"
}
