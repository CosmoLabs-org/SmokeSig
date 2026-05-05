package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// traceBackend queries a trace backend for trace data.
type traceBackend interface {
	queryTrace(client *http.Client, traceID string) (int, error)
}

// jaegerBackend queries Jaeger or Tempo (Jaeger-compatible API).
type jaegerBackend struct {
	baseURL     string
	serviceName string
}

func (b *jaegerBackend) queryTrace(client *http.Client, traceID string) (int, error) {
	url := fmt.Sprintf("%s/api/traces/%s", b.baseURL, traceID)
	if b.serviceName != "" {
		url += "?service=" + b.serviceName
	}
	resp, err := queryJSON(client, url, nil)
	if err != nil {
		return 0, err
	}
	var jr jaegerResponse
	if err := json.Unmarshal(resp, &jr); err != nil {
		return 0, err
	}
	if len(jr.Data) == 0 {
		return 0, nil
	}
	return len(jr.Data[0].Spans), nil
}

// honeycombBackend queries the Honeycomb traces API.
type honeycombBackend struct {
	baseURL string
	apiKey  string
}

func (b *honeycombBackend) queryTrace(client *http.Client, traceID string) (int, error) {
	url := fmt.Sprintf("%s/v1/traces/%s", b.baseURL, traceID)
	headers := map[string]string{"X-Honeycomb-Team": b.apiKey}
	resp, err := queryJSON(client, url, headers)
	if err != nil {
		return 0, err
	}
	// Honeycomb returns {"data": {"spans": [...]}}
	var hr struct {
		Data struct {
			Spans []struct{} `json:"spans"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &hr); err != nil {
		return 0, err
	}
	return len(hr.Data.Spans), nil
}

// datadogBackend queries the Datadog APM traces API.
type datadogBackend struct {
	baseURL string
	apiKey  string
	appKey  string
}

func (b *datadogBackend) queryTrace(client *http.Client, traceID string) (int, error) {
	url := fmt.Sprintf("%s/api/v1/traces/%s", b.baseURL, traceID)
	headers := map[string]string{
		"DD-API-KEY": b.apiKey,
	}
	if b.appKey != "" {
		headers["DD-APPLICATION-KEY"] = b.appKey
	}
	resp, err := queryJSON(client, url, headers)
	if err != nil {
		return 0, err
	}
	// Datadog returns {"traces": [{"spans": [...]}]}
	var dr struct {
		Traces []struct {
			Spans []struct{} `json:"spans"`
		} `json:"traces"`
	}
	if err := json.Unmarshal(resp, &dr); err != nil {
		return 0, err
	}
	if len(dr.Traces) == 0 {
		return 0, nil
	}
	total := 0
	for _, tr := range dr.Traces {
		total += len(tr.Spans)
	}
	return total, nil
}

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

// newTraceBackend creates the appropriate backend from an OTelTraceCheck config.
func newTraceBackend(check *schema.OTelTraceCheck) traceBackend {
	switch check.Backend {
	case "tempo":
		return &jaegerBackend{baseURL: check.JaegerURL, serviceName: check.ServiceName}
	case "honeycomb":
		return &honeycombBackend{baseURL: check.JaegerURL, apiKey: check.APIKey}
	case "datadog":
		return &datadogBackend{baseURL: check.JaegerURL, apiKey: check.APIKey, appKey: check.DDAppKey}
	default: // "jaeger" or empty
		return &jaegerBackend{baseURL: check.JaegerURL, serviceName: check.ServiceName}
	}
}

// CheckOTelTrace queries a trace backend to verify that a trace arrived
// with at least MinSpans spans within Timeout.
func CheckOTelTrace(check *schema.OTelTraceCheck, traceID string, client *http.Client) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	minSpans := check.MinSpans
	if minSpans == 0 {
		minSpans = 1
	}

	backend := newTraceBackend(check)

	deadline := time.Now().Add(timeout)
	var lastSpanCount int
	for time.Now().Before(deadline) {
		count, err := backend.queryTrace(client, traceID)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		lastSpanCount = count
		if count >= minSpans {
			return AssertionResult{
				Type:     "otel_trace",
				Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
				Actual:   fmt.Sprintf("%d spans found", count),
				Passed:   true,
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Final attempt after deadline expired.
	count, err := backend.queryTrace(client, traceID)
	if err != nil {
		return AssertionResult{
			Type:     "otel_trace",
			Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
			Actual:   fmt.Sprintf("collector error: %v", err),
			Passed:   false,
		}
	}
	lastSpanCount = count
	if count > 0 {
		return AssertionResult{
			Type:     "otel_trace",
			Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
			Actual:   fmt.Sprintf("%d spans found within %s", count, timeout),
			Passed:   count >= minSpans,
		}
	}
	_ = lastSpanCount
	return AssertionResult{
		Type:     "otel_trace",
		Expected: fmt.Sprintf(">=%d spans for trace %s", minSpans, traceID),
		Actual:   fmt.Sprintf("no spans found for trace %s within %s", traceID, timeout),
		Passed:   false,
	}
}

// queryJSON performs a GET request with optional headers and returns the body.
func queryJSON(client *http.Client, url string, headers map[string]string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// queryJaeger is kept for backwards compatibility — delegates to queryJSON.
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
