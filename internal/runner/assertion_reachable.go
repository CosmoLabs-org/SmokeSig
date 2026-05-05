package runner

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// httpReachable makes an HTTP GET request and returns the status code,
// response time, and any error. expectedStatus=0 means any 2xx passes.
func httpReachable(url string, timeout time.Duration, expectedStatus int) (statusCode int, elapsed time.Duration, err error) {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	start := time.Now()
	resp, err := client.Get(url)
	elapsed = time.Since(start)
	if err != nil {
		return 0, elapsed, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, elapsed, nil
}

// is2xx returns true if the status code is in the 200-299 range.
func is2xx(code int) bool {
	return code >= 200 && code <= 299
}

// CheckURLReachable verifies an HTTP/HTTPS endpoint is accessible.
func CheckURLReachable(check *schema.URLReachableCheck) AssertionResult {
	statusCode, elapsed, err := httpReachable(check.URL, check.Timeout.Duration, 0)
	if err != nil {
		return AssertionResult{
			Type:     "url_reachable",
			Expected: fmt.Sprintf("%s reachable", check.URL),
			Actual:   fmt.Sprintf("connection failed: %v", err),
			Passed:   false,
		}
	}
	expected := "any 2xx"
	passed := is2xx(statusCode)
	if check.StatusCode != nil {
		expected = fmt.Sprintf("HTTP %d", *check.StatusCode)
		passed = statusCode == *check.StatusCode
	}
	return AssertionResult{
		Type:     "url_reachable",
		Expected: expected,
		Actual:   fmt.Sprintf("HTTP %d (%s)", statusCode, elapsed.Round(time.Millisecond)),
		Passed:   passed,
	}
}

// CheckServiceReachable verifies an external service dependency is accessible.
func CheckServiceReachable(check *schema.ServiceReachableCheck) AssertionResult {
	statusCode, elapsed, err := httpReachable(check.URL, check.Timeout.Duration, 0)
	if err != nil {
		return AssertionResult{
			Type:     "service_reachable",
			Expected: fmt.Sprintf("%s reachable", check.URL),
			Actual:   fmt.Sprintf("connection failed: %v", err),
			Passed:   false,
		}
	}
	return AssertionResult{
		Type:     "service_reachable",
		Expected: "any 2xx",
		Actual:   fmt.Sprintf("HTTP %d (%s)", statusCode, elapsed.Round(time.Millisecond)),
		Passed:   is2xx(statusCode),
	}
}

// CheckS3Bucket verifies an S3-compatible bucket is accessible via anonymous HEAD.
func CheckS3Bucket(check *schema.S3BucketCheck) AssertionResult {
	endpoint := check.Endpoint
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}
	var url string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		url = fmt.Sprintf("%s/%s?location", endpoint, check.Bucket)
	} else {
		url = fmt.Sprintf("https://%s/%s?location", endpoint, check.Bucket)
	}

	statusCode, elapsed, err := httpReachable(url, 5*time.Second, 0)
	if err != nil {
		return AssertionResult{
			Type:     "s3_bucket",
			Expected: fmt.Sprintf("bucket %s accessible", check.Bucket),
			Actual:   fmt.Sprintf("connection failed: %v", err),
			Passed:   false,
		}
	}
	if statusCode == 403 {
		return AssertionResult{
			Type:     "s3_bucket",
			Expected: fmt.Sprintf("bucket %s accessible", check.Bucket),
			Actual:   "HTTP 403 Forbidden — bucket requires authentication; use http assertion with Go templates for authenticated access",
			Passed:   false,
		}
	}
	return AssertionResult{
		Type:     "s3_bucket",
		Expected: fmt.Sprintf("bucket %s accessible", check.Bucket),
		Actual:   fmt.Sprintf("HTTP %d (%s)", statusCode, elapsed.Round(time.Millisecond)),
		Passed:   is2xx(statusCode),
	}
}
