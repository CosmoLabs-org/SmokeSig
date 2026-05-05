package runner

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestCheckURLReachable_Pass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	result := CheckURLReachable(&schema.URLReachableCheck{URL: ts.URL})
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}

func TestCheckURLReachable_Fail5xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer ts.Close()

	result := CheckURLReachable(&schema.URLReachableCheck{URL: ts.URL})
	if result.Passed {
		t.Error("expected fail for 503")
	}
}

func TestCheckURLReachable_SpecificStatusCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer ts.Close()

	code := 201
	result := CheckURLReachable(&schema.URLReachableCheck{URL: ts.URL, StatusCode: &code})
	if !result.Passed {
		t.Errorf("expected pass for 201, got: %s", result.Actual)
	}
}

func TestCheckURLReachable_InvalidURL(t *testing.T) {
	result := CheckURLReachable(&schema.URLReachableCheck{URL: "http://invalid.invalid.invalid", Timeout: schema.Duration{Duration: 1 * time.Second}})
	if result.Passed {
		t.Error("expected fail for invalid URL")
	}
}

func TestCheckServiceReachable_Pass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	result := CheckServiceReachable(&schema.ServiceReachableCheck{URL: ts.URL})
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}

func TestCheckServiceReachable_Fail(t *testing.T) {
	result := CheckServiceReachable(&schema.ServiceReachableCheck{URL: "http://invalid.invalid.invalid", Timeout: schema.Duration{Duration: 1 * time.Second}})
	if result.Passed {
		t.Error("expected fail for unreachable service")
	}
}

func TestCheckS3Bucket_Pass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	result := CheckS3Bucket(&schema.S3BucketCheck{
		Bucket:   "test-bucket",
		Endpoint: ts.URL,
	})
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}

func TestCheckS3Bucket_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	result := CheckS3Bucket(&schema.S3BucketCheck{
		Bucket:   "missing-bucket",
		Endpoint: ts.URL,
	})
	if result.Passed {
		t.Error("expected fail for 404")
	}
}

func TestCheckS3Bucket_AuthRequired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer ts.Close()

	result := CheckS3Bucket(&schema.S3BucketCheck{
		Bucket:   "private-bucket",
		Endpoint: ts.URL,
	})
	if result.Passed {
		t.Error("expected fail for 403")
	}
	if !strings.Contains(result.Actual, "authentication") {
		t.Error("expected hint about authentication in output")
	}
}
