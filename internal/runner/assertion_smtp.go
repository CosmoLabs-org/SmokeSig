package runner

import (
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckSMTP verifies an SMTP server is accepting connections by performing
// a basic EHLO handshake. Uses stdlib net/smtp.
func CheckSMTP(check *schema.SMTPCheck) AssertionResult {
	host := check.Host
	port := check.Port
	if port == 0 {
		port = 25
	}
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return AssertionResult{
			Type:     "smtp_ping",
			Expected: fmt.Sprintf("SMTP at %s", addr),
			Actual:   fmt.Sprintf("connection refused: %v", err),
			Passed:   false,
		}
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// smtp.NewClient reads the greeting banner and sends EHLO internally.
	// Do NOT manually read the greeting — that would consume the 220 line
	// and leave NewClient waiting on the next server response (BUG-004).
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return AssertionResult{
			Type:     "smtp_ping",
			Expected: "SMTP handshake",
			Actual:   fmt.Sprintf("handshake failed: %v", err),
			Passed:   false,
		}
	}
	defer client.Close()

	if err := client.Hello("smoke-test.local"); err != nil {
		return AssertionResult{
			Type:     "smtp_ping",
			Expected: "EHLO accepted",
			Actual:   fmt.Sprintf("EHLO failed: %v", err),
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "smtp_ping",
		Expected: fmt.Sprintf("SMTP at %s", addr),
		Actual:   "connected, handshake OK",
		Passed:   true,
	}
}
