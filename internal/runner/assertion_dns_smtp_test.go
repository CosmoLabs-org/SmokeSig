package runner

import (
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// fakeSMTP starts a minimal SMTP server on a random port. It sends a 220
// greeting, reads the EHLO, responds 250, then waits for QUIT. This mirrors
// the real protocol just enough to validate CheckSMTP's single-handshake flow.
func fakeSMTP(t *testing.T) (port int, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port = ln.Addr().(*net.TCPAddr).Port

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Send greeting — smtp.NewClient expects to read this first.
		fmt.Fprintf(conn, "220 fake-smtp ready\r\n")

		buf := make([]byte, 1024)

		// Read EHLO (sent by smtp.NewClient internally)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		_ = string(buf[:n]) // e.g. "EHLO ..."

		// Respond to EHLO
		fmt.Fprintf(conn, "250 ok\r\n")

		// Read the second EHLO from client.Hello("smoke-test.local")
		n, err = conn.Read(buf)
		if err != nil {
			return
		}
		_ = string(buf[:n])
		fmt.Fprintf(conn, "250 ok\r\n")

		// Read QUIT
		conn.Read(buf)
		fmt.Fprintf(conn, "221 bye\r\n")
	}()

	return port, func() {
		ln.Close()
		<-done
	}
}

func TestCheckSMTP_Handshake(t *testing.T) {
	port, stop := fakeSMTP(t)
	defer stop()

	result := CheckSMTP(&schema.SMTPCheck{
		Host:    "127.0.0.1",
		Port:    port,
		Timeout: schema.Duration{Duration: 5 * time.Second},
	})
	if !result.Passed {
		t.Errorf("CheckSMTP: expected pass, got %q", result.Actual)
	}
	if result.Type != "smtp_ping" {
		t.Errorf("type = %q, want smtp_ping", result.Type)
	}
}

func TestCheckSMTP_ConnectionRefused(t *testing.T) {
	// Use a port that nothing listens on
	result := CheckSMTP(&schema.SMTPCheck{
		Host:    "127.0.0.1",
		Port:    19, // chargen — almost certainly not running
		Timeout: schema.Duration{Duration: 1 * time.Second},
	})
	if result.Passed {
		t.Error("CheckSMTP on closed port: expected fail")
	}
	if result.Type != "smtp_ping" {
		t.Errorf("type = %q, want smtp_ping", result.Type)
	}
}

func TestCheckSMTP_DefaultPort(t *testing.T) {
	// Verify defaults are applied (port 25, 10s timeout) without connecting
	result := CheckSMTP(&schema.SMTPCheck{
		Host: "127.0.0.1",
	})
	// Will fail to connect on port 25 — we just care it doesn't panic and reports failure
	if result.Passed {
		t.Skip("port 25 actually responded — unusual in test env")
	}
	if result.Type != "smtp_ping" {
		t.Errorf("type = %q, want smtp_ping", result.Type)
	}
}

func TestCheckDNS_Localhost(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname: "localhost",
	})
	if !result.Passed {
		t.Errorf("DNS localhost: expected pass, got %q", result.Actual)
	}
}

func TestCheckDNS_ExpectedIP(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "localhost",
		ExpectedIP: "127.0.0.1",
	})
	if !result.Passed {
		t.Errorf("DNS localhost → 127.0.0.1: expected pass, got %q", result.Actual)
	}
}

func TestCheckDNS_WrongIP(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "localhost",
		ExpectedIP: "1.2.3.4",
	})
	if result.Passed {
		t.Error("DNS localhost → 1.2.3.4: expected fail")
	}
}

func TestCheckDNS_Unresolvable(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname: "this-domain-definitely-does-not-exist-xyz123.invalid",
	})
	if result.Passed {
		t.Error("DNS unresolvable: expected fail")
	}
}

func TestCheckDNS_InvalidRecordType(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "localhost",
		RecordType: "INVALID",
	})
	if result.Passed {
		t.Error("DNS invalid record type: expected fail")
	}
}

func TestCheckDNS_TXTRecord(t *testing.T) {
	// Google's TXT record for spf is well-known
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "google.com",
		RecordType: "TXT",
	})
	if !result.Passed {
		t.Errorf("DNS google.com TXT: expected pass, got %q", result.Actual)
	}
}

func TestCheckDNS_DefaultRecordType(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname: "localhost",
	})
	if !result.Passed {
		t.Errorf("DNS localhost (default type): expected pass, got %q", result.Actual)
	}
	if result.Type != "dns_resolve" {
		t.Errorf("expected type dns_resolve, got %s", result.Type)
	}
}

func TestCheckDNS_MXRecord(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "google.com",
		RecordType: "MX",
	})
	if !result.Passed {
		t.Errorf("DNS MX google.com: expected pass, got %q", result.Actual)
	}
	if result.Type != "dns_resolve" {
		t.Errorf("type = %q, want dns_resolve", result.Type)
	}
}

func TestCheckDNS_AAAARecord(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "google.com",
		RecordType: "AAAA",
	})
	// AAAA may or may not resolve depending on network — just verify no panic and correct type
	if result.Type != "dns_resolve" {
		t.Errorf("type = %q, want dns_resolve", result.Type)
	}
	_ = result
}

func TestCheckDNS_CNAMERecord(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "www.google.com",
		RecordType: "CNAME",
	})
	// CNAME resolution may vary; verify no panic and correct type field
	if result.Type != "dns_resolve" {
		t.Errorf("type = %q, want dns_resolve", result.Type)
	}
}

func TestCheckDNS_EmptyHostname(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname: "",
	})
	if result.Passed {
		t.Error("DNS empty hostname: expected fail")
	}
	if result.Type != "dns_resolve" {
		t.Errorf("type = %q, want dns_resolve", result.Type)
	}
}

func TestCheckDNS_CustomTimeout(t *testing.T) {
	result := CheckDNS(&schema.DNSCheck{
		Hostname: "localhost",
		Timeout:  schema.Duration{Duration: 1 * time.Second},
	})
	if !result.Passed {
		t.Errorf("DNS localhost with custom timeout: expected pass, got %q", result.Actual)
	}
}

func TestCheckDNS_WindowsSkip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific path not applicable on this OS")
	}
	// On Windows the A/AAAA branch still uses LookupHost — verify it resolves localhost
	result := CheckDNS(&schema.DNSCheck{
		Hostname:   "localhost",
		RecordType: "A",
	})
	if !result.Passed {
		t.Errorf("DNS localhost (A, windows): expected pass, got %q", result.Actual)
	}
}
