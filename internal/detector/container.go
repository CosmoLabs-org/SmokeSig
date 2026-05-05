package detector

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// SmokeConfig is an alias for schema.SmokeConfig for external use.
type SmokeConfig = schema.SmokeConfig

// InspectContainer inspects a running Docker container and generates a smoke config.
// It detects listening ports and generates appropriate tests.
func InspectContainer(container string) (*SmokeConfig, error) {
	// Verify container exists and is running
	if err := checkContainer(container); err != nil {
		return nil, err
	}

	cfg := &SmokeConfig{
		Version: 1,
		Project: container,
		Settings: schema.Settings{
			Timeout: schema.Duration{},
		},
	}

	// Get listening ports
	ports, err := getListeningPorts(container)
	if err != nil {
		// Non-fatal: continue without port tests
		fmt.Printf("Warning: could not detect ports: %v\n", err)
	}

	// Generate port listening tests
	for _, port := range ports {
		cfg.Tests = append(cfg.Tests, schema.Test{
			Name: fmt.Sprintf("Port %d is listening", port.Port),
			Run:  "true", // No-op command, we're just checking the port
			Expect: schema.Expect{
				PortListening: &schema.PortCheck{
					Port:     port.Port,
					Protocol: port.Protocol,
					Host:     "localhost",
				},
			},
		})
	}

	// Try to detect HTTP endpoints on common ports
	for _, port := range ports {
		if isHTTPPort(port.Port) {
			status := 200
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: fmt.Sprintf("HTTP endpoint on port %d", port.Port),
				Run:  "true",
				Expect: schema.Expect{
					HTTP: &schema.HTTPCheck{
						URL:        fmt.Sprintf("http://localhost:%d/", port.Port),
						StatusCode: &status,
					},
				},
			})
		}
	}

	// Add a basic container running test
	exitCode := 0
	cfg.Tests = append([]schema.Test{{
		Name: "Container is running",
		Run:  fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s", container),
		Expect: schema.Expect{
			ExitCode:       &exitCode,
			StdoutContains: "true",
		},
	}}, cfg.Tests...)

	return cfg, nil
}

// PortInfo holds information about a listening port.
type PortInfo struct {
	Port     int
	Protocol string // "tcp" or "udp"
}

func checkContainer(container string) error {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", container)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("container %q not found or not accessible", container)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("container %q is not running", container)
	}
	return nil
}

func getListeningPorts(container string) ([]PortInfo, error) {
	// Try ss first (modern), fall back to netstat
	ports, err := getPortsWithSS(container)
	if err != nil {
		ports, err = getPortsWithNetstat(container)
	}
	if err != nil {
		// Try docker port command as fallback
		ports, err = getPortsFromDocker(container)
	}
	return ports, err
}

func getPortsWithSS(container string) ([]PortInfo, error) {
	cmd := exec.Command("docker", "exec", container, "ss", "-tlnp")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseSSOutput(string(out))
}

func getPortsWithNetstat(container string) ([]PortInfo, error) {
	cmd := exec.Command("docker", "exec", container, "netstat", "-tlnp")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseNetstatOutput(string(out))
}

func getPortsFromDocker(container string) ([]PortInfo, error) {
	cmd := exec.Command("docker", "port", container)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseDockerPortOutput(string(out))
}

// parseSSOutput parses `ss -tlnp` output.
// Example: LISTEN 0 128 *:8080 *:*
func parseSSOutput(output string) ([]PortInfo, error) {
	var ports []PortInfo
	portRe := regexp.MustCompile(`\*:(\d+)|\]:(\d+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		matches := portRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			portStr := matches[1]
			if portStr == "" {
				portStr = matches[2]
			}
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
				ports = append(ports, PortInfo{Port: port, Protocol: "tcp"})
			}
		}
	}
	return ports, nil
}

// parseNetstatOutput parses `netstat -tlnp` output.
func parseNetstatOutput(output string) ([]PortInfo, error) {
	var ports []PortInfo
	portRe := regexp.MustCompile(`:(\d+)\s`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		matches := portRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			if port, err := strconv.Atoi(matches[1]); err == nil && port > 0 {
				ports = append(ports, PortInfo{Port: port, Protocol: "tcp"})
			}
		}
	}
	return ports, nil
}

// parseDockerPortOutput parses `docker port CONTAINER` output.
// Example: 8080/tcp -> 0.0.0.0:8080
func parseDockerPortOutput(output string) ([]PortInfo, error) {
	var ports []PortInfo
	portRe := regexp.MustCompile(`(\d+)/(tcp|udp)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := portRe.FindStringSubmatch(line)
		if len(matches) > 2 {
			if port, err := strconv.Atoi(matches[1]); err == nil && port > 0 {
				ports = append(ports, PortInfo{Port: port, Protocol: matches[2]})
			}
		}
	}
	return ports, nil
}

func isHTTPPort(port int) bool {
	httpPorts := map[int]bool{
		80: true, 443: true, 8080: true, 8000: true, 8443: true,
		3000: true, 4000: true, 5000: true, 9000: true,
	}
	return httpPorts[port]
}
