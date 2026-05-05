package runner

import (
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

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
