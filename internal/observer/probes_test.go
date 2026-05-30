package observer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func extractPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return p
}

func TestProbeEndpointsFindsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)

	results := ProbeEndpoints([]PortBinding{{Port: port}}, 2*time.Second)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URL != fmt.Sprintf("http://localhost:%d/healthz", port) {
		t.Errorf("expected URL with /healthz, got %s", results[0].URL)
	}
	if results[0].StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", results[0].StatusCode)
	}
	if !results[0].Reachable {
		t.Error("expected Reachable=true")
	}
}

func TestProbeEndpointsNoPorts(t *testing.T) {
	results := ProbeEndpoints(nil, 2*time.Second)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for nil ports, got %d", len(results))
	}
}

func TestProbeEndpointsUnreachablePort(t *testing.T) {
	results := ProbeEndpoints([]PortBinding{{Port: 1}}, 500*time.Millisecond)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for unreachable port, got %d", len(results))
	}
}

func TestProbeEndpointsStopsAfterFirstReachable(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)

	results := ProbeEndpoints([]PortBinding{{Port: port}}, 2*time.Second)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should stop after /health (first commonPath) since it returns 200
	if len(paths) != 1 || paths[0] != "/health" {
		t.Errorf("expected only /health probed, got %v", paths)
	}
}

// TestProbeEndpoints_DefaultTimeout exercises the timeout==0 branch that defaults to 2s.
func TestProbeEndpoints_DefaultTimeout(t *testing.T) {
	// Pass timeout=0 so the default branch is taken, with nil ports so it returns immediately.
	results := ProbeEndpoints(nil, 0)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestProbeEndpointsSkipsDuplicatePorts(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)

	results := ProbeEndpoints([]PortBinding{
		{Port: port},
		{Port: port},
	}, 2*time.Second)

	if len(results) != 1 {
		t.Fatalf("expected 1 result for duplicate ports, got %d", len(results))
	}
	if count != 1 {
		t.Errorf("expected 1 HTTP request, got %d", count)
	}
}
