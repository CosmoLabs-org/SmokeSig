package observer

import (
	"fmt"
	"net/http"
	"time"
)

var commonPaths = []string{"/health", "/healthz", "/ready", "/readyz", "/api", "/"}

func ProbeEndpoints(ports []PortBinding, timeout time.Duration, extraPaths ...string) []HTTPProbeResult {
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	seen := make(map[int]bool)
	client := &http.Client{Timeout: timeout}
	var results []HTTPProbeResult

	for _, pb := range ports {
		if seen[pb.Port] {
			continue
		}
		seen[pb.Port] = true

		paths := append(commonPaths, extraPaths...)
		for _, path := range paths {
			url := fmt.Sprintf("http://localhost:%d%s", pb.Port, path)
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
				results = append(results, HTTPProbeResult{
					URL:        url,
					StatusCode: resp.StatusCode,
					Reachable:  true,
				})
				break
			}
		}
	}

	return results
}
