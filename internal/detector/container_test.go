package detector

import (
	"fmt"
	"testing"
)

// --- Original tests (preserved) ---

func TestParseSSOutput(t *testing.T) {
	output := `State   Recv-Q  Send-Q  Local Address:Port   Peer Address:Port
LISTEN  0       128           *:8080              *:*
LISTEN  0       128           *:3000              *:*
LISTEN  0       128     [::]:22                [::]:*`

	ports, err := parseSSOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ports) != 3 {
		t.Errorf("expected 3 ports, got %d", len(ports))
	}

	// Check first port
	found8080 := false
	for _, p := range ports {
		if p.Port == 8080 {
			found8080 = true
			if p.Protocol != "tcp" {
				t.Errorf("expected tcp protocol, got %s", p.Protocol)
			}
		}
	}
	if !found8080 {
		t.Error("expected to find port 8080")
	}
}

func TestParseNetstatOutput(t *testing.T) {
	output := `Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 0.0.0.0:8080            0.0.0.0:*               LISTEN      1/node
tcp        0      0 0.0.0.0:443             0.0.0.0:*               LISTEN      1/nginx`

	ports, err := parseNetstatOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}
}

func TestParseDockerPortOutput(t *testing.T) {
	output := `8080/tcp -> 0.0.0.0:8080
443/tcp -> 0.0.0.0:443
53/udp -> 0.0.0.0:53`

	ports, err := parseDockerPortOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ports) != 3 {
		t.Errorf("expected 3 ports, got %d", len(ports))
	}

	// Check UDP port
	foundUDP := false
	for _, p := range ports {
		if p.Port == 53 && p.Protocol == "udp" {
			foundUDP = true
		}
	}
	if !foundUDP {
		t.Error("expected to find UDP port 53")
	}
}

func TestIsHTTPPort(t *testing.T) {
	tests := []struct {
		port     int
		expected bool
	}{
		{80, true},
		{443, true},
		{8080, true},
		{3000, true},
		{22, false},
		{5432, false},
		{27017, false},
	}

	for _, tc := range tests {
		if got := isHTTPPort(tc.port); got != tc.expected {
			t.Errorf("isHTTPPort(%d) = %v, want %v", tc.port, got, tc.expected)
		}
	}
}

// --- Edge case tests ---

func TestParseSSOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantLen   int
		wantPorts []PortInfo
	}{
		{
			name:      "empty output",
			output:    "",
			wantLen:   0,
			wantPorts: nil,
		},
		{
			name:      "malformed lines without colon",
			output:    "LISTEN something broken no colon here\nanother bad line",
			wantLen:   0,
			wantPorts: nil,
		},
		{
			name: "IPv6 bracket address",
			output: `State   Recv-Q  Send-Q  Local Address:Port   Peer Address:Port
LISTEN  0       128     [::]:443              [::]:*`,
			wantLen: 1,
			wantPorts: []PortInfo{{Port: 443, Protocol: "tcp"}},
		},
		{
			name: "multiple ports mixed formats",
			output: `State   Recv-Q  Send-Q  Local Address:Port   Peer Address:Port
LISTEN  0       128           *:8080              *:*
LISTEN  0       128     [::]:9090              [::]:*
LISTEN  0       128           *:3000              *:*`,
			wantLen: 3,
			wantPorts: []PortInfo{
				{Port: 8080, Protocol: "tcp"},
				{Port: 9090, Protocol: "tcp"},
				{Port: 3000, Protocol: "tcp"},
			},
		},
		{
			name: "non-LISTEN lines ignored",
			output: `State   Recv-Q  Send-Q  Local Address:Port   Peer Address:Port
ESTAB   0       0           *:8080              *:*
LISTEN  0       128           *:9090              *:*`,
			wantLen: 1,
			wantPorts: []PortInfo{{Port: 9090, Protocol: "tcp"}},
		},
		{
			name:      "whitespace only",
			output:    "   \n\t\n   ",
			wantLen:   0,
			wantPorts: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ports, err := parseSSOutput(tc.output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ports) != tc.wantLen {
				t.Fatalf("got %d ports, want %d; ports: %+v", len(ports), tc.wantLen, ports)
			}
			for i, want := range tc.wantPorts {
				if ports[i].Port != want.Port {
					t.Errorf("ports[%d].Port = %d, want %d", i, ports[i].Port, want.Port)
				}
				if ports[i].Protocol != want.Protocol {
					t.Errorf("ports[%d].Protocol = %q, want %q", i, ports[i].Protocol, want.Protocol)
				}
			}
		})
	}
}

func TestParseNetstatOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantLen   int
		wantPorts []PortInfo
	}{
		{
			name:      "empty output",
			output:    "",
			wantLen:   0,
			wantPorts: nil,
		},
		{
			name: "non-numeric port skipped",
			output: `Proto Recv-Q Send-Q Local Address           Foreign Address         State
tcp        0      0 0.0.0.0:abc            0.0.0.0:*               LISTEN`,
			wantLen:   0,
			wantPorts: nil,
		},
		{
			name: "mixed TCP lines only TCP captured",
			output: `Proto Recv-Q Send-Q Local Address           Foreign Address         State
tcp        0      0 0.0.0.0:8080            0.0.0.0:*               LISTEN
tcp        0      0 0.0.0.0:443             0.0.0.0:*               LISTEN`,
			wantLen: 2,
			wantPorts: []PortInfo{
				{Port: 8080, Protocol: "tcp"},
				{Port: 443, Protocol: "tcp"},
			},
		},
		{
			name: "header only no data",
			output: `Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name`,
			wantLen:   0,
			wantPorts: nil,
		},
		{
			name:      "whitespace only",
			output:    "   \n\n   ",
			wantLen:   0,
			wantPorts: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ports, err := parseNetstatOutput(tc.output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ports) != tc.wantLen {
				t.Fatalf("got %d ports, want %d; ports: %+v", len(ports), tc.wantLen, ports)
			}
			for i, want := range tc.wantPorts {
				if ports[i].Port != want.Port {
					t.Errorf("ports[%d].Port = %d, want %d", i, ports[i].Port, want.Port)
				}
				if ports[i].Protocol != want.Protocol {
					t.Errorf("ports[%d].Protocol = %q, want %q", i, ports[i].Protocol, want.Protocol)
				}
			}
		})
	}
}

func TestParseDockerPortOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantLen   int
		wantPorts []PortInfo
	}{
		{
			name:      "empty output",
			output:    "",
			wantLen:   0,
			wantPorts: nil,
		},
		{
			name: "multiple port mappings",
			output: `8080/tcp -> 0.0.0.0:8080
443/tcp -> 0.0.0.0:443
53/udp -> 0.0.0.0:53
3000/tcp -> 0.0.0.0:3000`,
			wantLen: 4,
			wantPorts: []PortInfo{
				{Port: 8080, Protocol: "tcp"},
				{Port: 443, Protocol: "tcp"},
				{Port: 53, Protocol: "udp"},
				{Port: 3000, Protocol: "tcp"},
			},
		},
		{
			name:      "whitespace only",
			output:    "   \n\n   ",
			wantLen:   0,
			wantPorts: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ports, err := parseDockerPortOutput(tc.output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ports) != tc.wantLen {
				t.Fatalf("got %d ports, want %d; ports: %+v", len(ports), tc.wantLen, ports)
			}
			for i, want := range tc.wantPorts {
				if ports[i].Port != want.Port {
					t.Errorf("ports[%d].Port = %d, want %d", i, ports[i].Port, want.Port)
				}
				if ports[i].Protocol != want.Protocol {
					t.Errorf("ports[%d].Protocol = %q, want %q", i, ports[i].Protocol, want.Protocol)
				}
			}
		})
	}
}

func TestIsHTTPPortAdditional(t *testing.T) {
	tests := []struct {
		port     int
		expected bool
	}{
		// All known HTTP ports
		{80, true},
		{443, true},
		{8080, true},
		{8000, true},
		{8443, true},
		{3000, true},
		{4000, true},
		{5000, true},
		{9000, true},
		// Non-HTTP ports
		{22, false},
		{3306, false},
		{5432, false},
		{6379, false},
		{27017, false},
		{0, false},
		{1, false},
		{65535, false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("port_%d", tc.port), func(t *testing.T) {
			if got := isHTTPPort(tc.port); got != tc.expected {
				t.Errorf("isHTTPPort(%d) = %v, want %v", tc.port, got, tc.expected)
			}
		})
	}
}
