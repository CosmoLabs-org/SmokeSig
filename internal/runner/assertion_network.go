package runner

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/tidwall/gjson"
)

// CheckPortListening verifies that a port is open and accepting connections.
func CheckPortListening(port int, protocol, host string) AssertionResult {
	if protocol == "" {
		protocol = "tcp"
	}
	if host == "" {
		host = "localhost"
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout(protocol, addr, 5*time.Second)
	if err != nil {
		return AssertionResult{Type: "port_listening", Expected: addr, Actual: err.Error(), Passed: false}
	}
	conn.Close()
	return AssertionResult{Type: "port_listening", Expected: addr, Actual: "open", Passed: true}
}

// CheckProcessRunning verifies that a named process is currently running on the host.
// Uses exact process-name matching (pgrep -x on Unix, CSV-parsed tasklist on Windows).
// Bounded by a 2s timeout to prevent hangs.
func CheckProcessRunning(name string) AssertionResult {
	if name == "" {
		return AssertionResult{Type: "process_running", Expected: name, Actual: "empty name", Passed: false}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if runtime.GOOS == "windows" {
		filter := fmt.Sprintf("IMAGENAME eq %s", name)
		out, err := exec.CommandContext(ctx, "tasklist", "/FI", filter, "/FO", "CSV", "/NH").Output()
		if err != nil {
			return AssertionResult{Type: "process_running", Expected: name, Actual: "lookup error", Passed: false}
		}
		if !strings.Contains(string(out), "\""+name) {
			return AssertionResult{Type: "process_running", Expected: name, Actual: "not found", Passed: false}
		}
		return AssertionResult{Type: "process_running", Expected: name, Actual: "running", Passed: true}
	}
	out, err := exec.CommandContext(ctx, "pgrep", "-x", name).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return AssertionResult{Type: "process_running", Expected: name, Actual: "not found", Passed: false}
		}
		return AssertionResult{Type: "process_running", Expected: name, Actual: "lookup error: " + err.Error(), Passed: false}
	}
	if len(out) == 0 {
		return AssertionResult{Type: "process_running", Expected: name, Actual: "not found", Passed: false}
	}
	return AssertionResult{Type: "process_running", Expected: name, Actual: strings.TrimSpace(string(out)), Passed: true}
}

// CheckHTTP performs an HTTP request and returns assertion results for status, body, and headers.
func CheckHTTP(check *schema.HTTPCheck) []AssertionResult {
	var results []AssertionResult

	// Default method to GET
	method := check.Method
	if method == "" {
		method = "GET"
	}

	// Set timeout (default 10s)
	timeout := 10 * time.Second
	if check.Timeout.Duration > 0 {
		timeout = check.Timeout.Duration
	}

	client := &http.Client{Timeout: timeout}

	// Build request
	var bodyReader io.Reader
	if check.Body != "" {
		bodyReader = strings.NewReader(check.Body)
	}

	req, err := http.NewRequest(method, check.URL, bodyReader)
	if err != nil {
		return []AssertionResult{{
			Type:     "http_request",
			Expected: check.URL,
			Actual:   fmt.Sprintf("invalid request: %v", err),
			Passed:   false,
		}}
	}

	// Add headers
	for k, v := range check.Headers {
		req.Header.Set(k, v)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return []AssertionResult{{
			Type:     "http_request",
			Expected: check.URL,
			Actual:   fmt.Sprintf("request failed: %v", err),
			Passed:   false,
		}}
	}
	defer resp.Body.Close()

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []AssertionResult{{
			Type:     "http_body",
			Expected: "readable body",
			Actual:   fmt.Sprintf("failed to read body: %v", err),
			Passed:   false,
		}}
	}
	bodyStr := string(body)

	// Check status code
	if check.StatusCode != nil {
		results = append(results, AssertionResult{
			Type:     "http_status",
			Expected: fmt.Sprintf("%d", *check.StatusCode),
			Actual:   fmt.Sprintf("%d", resp.StatusCode),
			Passed:   resp.StatusCode == *check.StatusCode,
		})
	}

	// Check body contains
	if check.BodyContains != "" {
		results = append(results, AssertionResult{
			Type:     "http_body_contains",
			Expected: check.BodyContains,
			Actual:   bodyStr,
			Passed:   strings.Contains(bodyStr, check.BodyContains),
		})
	}

	// Check body matches regex
	if check.BodyMatches != "" {
		matched, err := regexp.MatchString(check.BodyMatches, bodyStr)
		if err != nil {
			results = append(results, AssertionResult{
				Type:     "http_body_matches",
				Expected: check.BodyMatches,
				Actual:   fmt.Sprintf("invalid regex: %v", err),
				Passed:   false,
			})
		} else {
			results = append(results, AssertionResult{
				Type:     "http_body_matches",
				Expected: check.BodyMatches,
				Actual:   bodyStr,
				Passed:   matched,
			})
		}
	}

	// Check header contains
	for k, v := range check.HeaderContains {
		actual := resp.Header.Get(k)
		results = append(results, AssertionResult{
			Type:     "http_header_contains",
			Expected: fmt.Sprintf("%s: %s", k, v),
			Actual:   fmt.Sprintf("%s: %s", k, actual),
			Passed:   strings.Contains(actual, v),
		})
	}

	return results
}

// CheckSSLCert dials host:port over TLS and validates the certificate chain,
// expiry, and days-remaining threshold.
func CheckSSLCert(check *schema.SSLCertCheck) AssertionResult {
	port := check.Port
	if port == 0 {
		port = 443
	}
	addr := fmt.Sprintf("%s:%d", check.Host, port)
	conf := &tls.Config{
		ServerName:         check.Host,
		InsecureSkipVerify: check.AllowSelfSigned, //nolint:gosec -- opt-in per AllowSelfSigned flag
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, conf)
	if err != nil {
		return AssertionResult{Type: "ssl_cert", Expected: addr, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return AssertionResult{Type: "ssl_cert", Expected: addr, Actual: "no peer certificate", Passed: false}
	}
	leaf := certs[0]
	now := time.Now()
	if now.After(leaf.NotAfter) {
		return AssertionResult{
			Type:     "ssl_cert",
			Expected: "valid cert",
			Actual:   "expired on " + leaf.NotAfter.Format("2006-01-02"),
			Passed:   false,
		}
	}
	if check.MinDaysRemaining > 0 {
		daysLeft := int(leaf.NotAfter.Sub(now).Hours() / 24)
		if daysLeft < check.MinDaysRemaining {
			return AssertionResult{
				Type:     "ssl_cert",
				Expected: fmt.Sprintf(">= %d days", check.MinDaysRemaining),
				Actual:   fmt.Sprintf("%d days", daysLeft),
				Passed:   false,
			}
		}
	}
	return AssertionResult{
		Type:     "ssl_cert",
		Expected: addr,
		Actual:   fmt.Sprintf("valid, expires %s", leaf.NotAfter.Format("2006-01-02")),
		Passed:   true,
	}
}

// CheckJSONField extracts a field from JSON and validates it against equals/contains/matches.
func CheckJSONField(jsonStr string, check *schema.JSONFieldCheck) []AssertionResult {
	var results []AssertionResult

	// Check if JSON is valid
	if !gjson.Valid(jsonStr) {
		return []AssertionResult{{
			Type:     "json_field",
			Expected: check.Path,
			Actual:   "invalid JSON",
			Passed:   false,
		}}
	}

	// Extract the field value
	result := gjson.Get(jsonStr, check.Path)
	if !result.Exists() {
		return []AssertionResult{{
			Type:     "json_field",
			Expected: check.Path,
			Actual:   "field not found",
			Passed:   false,
		}}
	}

	actual := result.String()

	// Check equals
	if check.Equals != "" {
		results = append(results, AssertionResult{
			Type:     "json_field_equals",
			Expected: check.Equals,
			Actual:   actual,
			Passed:   actual == check.Equals,
		})
	}

	// Check contains
	if check.Contains != "" {
		results = append(results, AssertionResult{
			Type:     "json_field_contains",
			Expected: check.Contains,
			Actual:   actual,
			Passed:   strings.Contains(actual, check.Contains),
		})
	}

	// Check matches
	if check.Matches != "" {
		matched, err := regexp.MatchString(check.Matches, actual)
		if err != nil {
			results = append(results, AssertionResult{
				Type:     "json_field_matches",
				Expected: check.Matches,
				Actual:   fmt.Sprintf("invalid regex: %v", err),
				Passed:   false,
			})
		} else {
			results = append(results, AssertionResult{
				Type:     "json_field_matches",
				Expected: check.Matches,
				Actual:   actual,
				Passed:   matched,
			})
		}
	}

	return results
}

// CheckHTTPWithTrace is like CheckHTTP but injects a traceparent header.
func CheckHTTPWithTrace(check *schema.HTTPCheck, span *SpanContext) []AssertionResult {
	clone := *check
	clone.Headers = make(map[string]string, len(check.Headers)+1)
	for k, v := range check.Headers {
		clone.Headers[k] = v
	}
	clone.Headers["traceparent"] = span.Traceparent()
	return CheckHTTP(&clone)
}
