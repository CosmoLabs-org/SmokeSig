package observer

import (
	"os"
	"testing"
)

const sampleLsofOutput = `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node    12345  gab    20u  IPv4 0x1234567890abcdef      0t0  TCP *:3000 (LISTEN)
node    12345  gab    21u  IPv6 0xfedcba0987654321      0t0  TCP [::1]:3000 (LISTEN)
node    12345  gab    22u  IPv4 0xabcdef1234567890      0t0  TCP 127.0.0.1:9229 (LISTEN)
`

func TestParseLsofOutput_ExtractsPorts(t *testing.T) {
	bindings := parseLsofOutput(sampleLsofOutput)
	if len(bindings) == 0 {
		t.Fatal("expected bindings, got nil")
	}

	// Should have 2 unique ports: 3000 and 9229 (3000 deduplicated)
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(bindings))
	}

	found3000 := false
	found9229 := false
	for _, b := range bindings {
		if b.Port == 3000 {
			found3000 = true
			if b.Protocol != "tcp" {
				t.Errorf("port 3000: expected protocol tcp, got %s", b.Protocol)
			}
		}
		if b.Port == 9229 {
			found9229 = true
			if b.Host != "127.0.0.1" {
				t.Errorf("port 9229: expected host 127.0.0.1, got %s", b.Host)
			}
		}
	}
	if !found3000 {
		t.Error("expected port 3000 to be found")
	}
	if !found9229 {
		t.Error("expected port 9229 to be found")
	}
}

func TestParseLsofOutput_EmptyInput(t *testing.T) {
	bindings := parseLsofOutput("")
	if bindings != nil {
		t.Fatalf("expected nil for empty input, got %v", bindings)
	}
}

func TestParseLsofOutput_NoDuplicates(t *testing.T) {
	input := `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node    12345  gab    20u  IPv4 0x1234567890abcdef      0t0  TCP *:3000 (LISTEN)
node    12345  gab    21u  IPv6 0xfedcba0987654321      0t0  TCP [::1]:3000 (LISTEN)
node    12345  gab    22u  IPv4 0xabcdef1234567890      0t0  TCP *:3000 (LISTEN)
`
	bindings := parseLsofOutput(input)
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding (deduplicated), got %d", len(bindings))
	}
	if bindings[0].Port != 3000 {
		t.Errorf("expected port 3000, got %d", bindings[0].Port)
	}
}

// TestParseAddr_IPv6 verifies parseAddr handles [::1]:port form.
func TestParseAddr_IPv6(t *testing.T) {
	host, port, ok := parseAddr("[::1]:8080")
	if !ok {
		t.Fatal("expected ok=true for IPv6 address")
	}
	if host != "::1" {
		t.Errorf("host = %q, want %q", host, "::1")
	}
	if port != 8080 {
		t.Errorf("port = %d, want 8080", port)
	}
}

// TestParseAddr_IPv6_MissingCloseBracket verifies parseAddr returns ok=false for malformed IPv6.
func TestParseAddr_IPv6_MissingCloseBracket(t *testing.T) {
	_, _, ok := parseAddr("[::1:8080")
	if ok {
		t.Error("expected ok=false for IPv6 missing closing bracket")
	}
}

// TestParseAddr_IPv6_MissingColon verifies parseAddr returns ok=false when no colon after bracket.
func TestParseAddr_IPv6_MissingColon(t *testing.T) {
	_, _, ok := parseAddr("[::1]8080")
	if ok {
		t.Error("expected ok=false for IPv6 missing colon after bracket")
	}
}

// TestParseAddr_IPv6_InvalidPort verifies parseAddr returns ok=false for non-numeric port.
func TestParseAddr_IPv6_InvalidPort(t *testing.T) {
	_, _, ok := parseAddr("[::1]:noport")
	if ok {
		t.Error("expected ok=false for IPv6 with invalid port")
	}
}

// TestParseAddr_InvalidAddress verifies parseAddr returns ok=false with no colon at all.
func TestParseAddr_InvalidAddress(t *testing.T) {
	_, _, ok := parseAddr("nohost")
	if ok {
		t.Error("expected ok=false for address with no colon")
	}
}

// TestParseAddr_InvalidPort verifies parseAddr returns ok=false when port is non-numeric.
func TestParseAddr_InvalidPort(t *testing.T) {
	_, _, ok := parseAddr("*:notaport")
	if ok {
		t.Error("expected ok=false for invalid port string")
	}
}

// TestParseAddr_WildcardHost verifies parseAddr handles *:port correctly.
func TestParseAddr_WildcardHost(t *testing.T) {
	host, port, ok := parseAddr("*:3000")
	if !ok {
		t.Fatal("expected ok=true for *:3000")
	}
	if host != "*" {
		t.Errorf("host = %q, want %q", host, "*")
	}
	if port != 3000 {
		t.Errorf("port = %d, want 3000", port)
	}
}

func TestParseLsofOutput_IPv6Address(t *testing.T) {
	input := `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
app     99999  user   10u  IPv6 0xabcdef1234567890      0t0  TCP [::1]:8080 (LISTEN)
`
	bindings := parseLsofOutput(input)
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0].Port != 8080 {
		t.Errorf("expected port 8080, got %d", bindings[0].Port)
	}
	if bindings[0].Host != "::1" {
		t.Errorf("expected host ::1, got %s", bindings[0].Host)
	}
}

// TestDetectPorts_OwnPID exercises the DetectPorts function with our own PID on darwin/linux.
// The test process is unlikely to have LISTEN sockets, so we expect an empty (non-error) result.
func TestDetectPorts_OwnPID(t *testing.T) {
	bindings, err := DetectPorts(os.Getpid())
	// Should not error — DetectPorts swallows lsof errors and returns nil.
	if err != nil {
		t.Fatalf("DetectPorts returned unexpected error: %v", err)
	}
	// bindings may be nil or empty; both are fine. Just ensure the function ran.
	_ = bindings
}

// TestDetectPorts_InvalidPID verifies DetectPorts with an invalid PID returns nil (not error).
func TestDetectPorts_InvalidPID(t *testing.T) {
	bindings, err := DetectPorts(-99999)
	if err != nil {
		t.Fatalf("DetectPorts with invalid PID returned error: %v", err)
	}
	// lsof will fail for an invalid PID; function should return nil silently.
	_ = bindings
}

// TestParseLsofOutput_InvalidAddrSkipped verifies that lines with unparseable addresses are skipped (continue branch).
func TestParseLsofOutput_InvalidAddrSkipped(t *testing.T) {
	// Line with TCP but no valid address in next field — should be silently skipped.
	input := `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node    12345  gab    20u  IPv4 0x1234567890abcdef      0t0  TCP invalidaddr (LISTEN)
node    12345  gab    22u  IPv4 0xabcdef1234567890      0t0  TCP 127.0.0.1:9229 (LISTEN)
`
	bindings := parseLsofOutput(input)
	// Only the valid line should produce a binding.
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding (invalid addr skipped), got %d: %v", len(bindings), bindings)
	}
	if bindings[0].Port != 9229 {
		t.Errorf("expected port 9229, got %d", bindings[0].Port)
	}
}
