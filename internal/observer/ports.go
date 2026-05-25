package observer

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// DetectPorts finds listening TCP ports opened by the given PID.
// Uses lsof on macOS/Linux. Returns nil on unsupported platforms.
func DetectPorts(pid int) ([]PortBinding, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, nil
	}

	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-nP", "-p", strconv.Itoa(pid))
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	return parseLsofOutput(string(out)), nil
}

// parseLsofOutput parses lsof output lines to extract port bindings.
func parseLsofOutput(output string) []PortBinding {
	var results []PortBinding
	seen := make(map[int]bool)

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		// Find the field containing the address:port, e.g. "TCP *:3000 (LISTEN)"
		for i, f := range fields {
			if strings.HasPrefix(f, "TCP") || strings.HasPrefix(f, "UDP") {
				// Next field is host:port, or it's combined like "TCP *:3000"
				var addrField string
				if i+1 < len(fields) {
					addrField = fields[i+1]
				}
				// Strip (LISTEN) suffix if present
				addrField = strings.TrimSuffix(addrField, "(LISTEN)")
				addrField = strings.TrimSpace(addrField)

				host, port, ok := parseAddr(addrField)
				if !ok {
					continue
				}
				if seen[port] {
					continue
				}
				seen[port] = true
				results = append(results, PortBinding{
					Port:     port,
					Protocol: "tcp",
					Host:     host,
				})
				break
			}
		}
	}

	return results
}

// parseAddr splits "HOST:PORT" into components, handling IPv6 brackets.
func parseAddr(s string) (host string, port int, ok bool) {
	// Handle [::1]:8080
	if strings.HasPrefix(s, "[") {
		closeBracket := strings.Index(s, "]")
		if closeBracket < 0 {
			return "", 0, false
		}
		host = s[1:closeBracket]
		rest := s[closeBracket+1:]
		if !strings.HasPrefix(rest, ":") {
			return "", 0, false
		}
		p, err := strconv.Atoi(rest[1:])
		if err != nil {
			return "", 0, false
		}
		return host, p, true
	}

	// Handle HOST:PORT
	lastColon := strings.LastIndex(s, ":")
	if lastColon < 0 {
		return "", 0, false
	}
	host = s[:lastColon]
	p, err := strconv.Atoi(s[lastColon+1:])
	if err != nil {
		return "", 0, false
	}
	return host, p, true
}
