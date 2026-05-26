package observer

import "testing"

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
