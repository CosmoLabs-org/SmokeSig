package runner

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckDNS resolves a hostname and optionally verifies the result.
func CheckDNS(check *schema.DNSCheck) AssertionResult {
	recordType := check.RecordType
	if recordType == "" {
		recordType = "A"
	}

	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := net.Resolver{}
	var addrs []string
	var err error

	switch strings.ToUpper(recordType) {
	case "A", "AAAA":
		// net.LookupHost returns both A and AAAA records
		addrs, err = resolver.LookupHost(ctx, check.Hostname)
	case "TXT":
		addrs, err = resolver.LookupTXT(ctx, check.Hostname)
	case "MX":
		records, mxErr := resolver.LookupMX(ctx, check.Hostname)
		err = mxErr
		if err == nil {
			for _, r := range records {
				addrs = append(addrs, fmt.Sprintf("%s (pref %d)", strings.TrimSuffix(r.Host, "."), r.Pref))
			}
		}
	case "CNAME":
		cname, cnameErr := resolver.LookupCNAME(ctx, check.Hostname)
		err = cnameErr
		if err == nil {
			addrs = []string{strings.TrimSuffix(cname, ".")}
		}
	default:
		return AssertionResult{
			Type:     "dns_resolve",
			Expected: check.Hostname,
			Actual:   fmt.Sprintf("unsupported record type: %s", recordType),
			Passed:   false,
		}
	}

	if err != nil {
		return AssertionResult{
			Type:     "dns_resolve",
			Expected: fmt.Sprintf("%s resolves (%s)", check.Hostname, recordType),
			Actual:   fmt.Sprintf("lookup failed: %v", err),
			Passed:   false,
		}
	}

	if check.ExpectedIP != "" {
		for _, addr := range addrs {
			if addr == check.ExpectedIP {
				return AssertionResult{
					Type:     "dns_resolve",
					Expected: fmt.Sprintf("%s → %s", check.Hostname, check.ExpectedIP),
					Actual:   strings.Join(addrs, ", "),
					Passed:   true,
				}
			}
		}
		return AssertionResult{
			Type:     "dns_resolve",
			Expected: fmt.Sprintf("%s → %s", check.Hostname, check.ExpectedIP),
			Actual:   strings.Join(addrs, ", "),
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "dns_resolve",
		Expected: fmt.Sprintf("%s resolves (%s)", check.Hostname, recordType),
		Actual:   strings.Join(addrs, ", "),
		Passed:   true,
	}
}
