package runner

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckNTP sends a 48-byte NTP client packet and verifies a valid server response.
func CheckNTP(check *schema.NTPCheck) AssertionResult {
	server := check.Server
	if server == "" {
		server = "pool.ntp.org"
	}
	addr := server + ":123"
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return AssertionResult{Type: "ntp_check", Expected: server, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// NTP v4 client request: 48 bytes
	req := make([]byte, 48)
	req[0] = 0x23 // LI=0, VN=4, Mode=3 (client)

	// Record origin timestamp (T1)
	t1 := time.Now()
	ntpSec, ntpFrac := timeToNTP(t1)
	binary.BigEndian.PutUint32(req[40:44], ntpSec)
	binary.BigEndian.PutUint32(req[44:48], ntpFrac)

	if _, err := conn.Write(req); err != nil {
		return AssertionResult{Type: "ntp_check", Expected: server, Actual: "write error: " + err.Error(), Passed: false}
	}

	resp := make([]byte, 48)
	n, err := conn.Read(resp)
	t4 := time.Now() // destination timestamp
	if err != nil || n < 48 {
		return AssertionResult{Type: "ntp_check", Expected: server, Actual: fmt.Sprintf("read error: %v (n=%d)", err, n), Passed: false}
	}

	// Verify response mode: bits 0-2 of first byte should be 4 (server)
	mode := resp[0] & 0x07
	if mode != 4 {
		return AssertionResult{Type: "ntp_check", Expected: server, Actual: fmt.Sprintf("not a server response (mode=%d)", mode), Passed: false}
	}

	// Check stratum: 1=primary, 2-15=secondary, 0=kiss-of-death
	stratum := resp[1]
	if stratum == 0 {
		code := string(resp[12:16])
		return AssertionResult{Type: "ntp_check", Expected: server, Actual: fmt.Sprintf("kiss-of-death: %s", code), Passed: false}
	}

	// Parse receive timestamp (T2) and transmit timestamp (T3)
	// T2 at bytes 32-39, T3 at bytes 40-47
	t2Sec := binary.BigEndian.Uint32(resp[32:36])
	t2Frac := binary.BigEndian.Uint32(resp[36:40])
	t2 := ntpToTime(t2Sec, t2Frac)

	t3Sec := binary.BigEndian.Uint32(resp[40:44])
	t3Frac := binary.BigEndian.Uint32(resp[44:48])
	t3 := ntpToTime(t3Sec, t3Frac)

	// NTP offset formula: ((T2 - T1) + (T3 - T4)) / 2
	offset := ((t2.Sub(t1) + t3.Sub(t4)) / 2)
	if offset < 0 {
		offset = -offset
	}

	if check.MaxOffsetMs > 0 {
		offsetMs := int(offset.Milliseconds())
		if offsetMs > check.MaxOffsetMs {
			return AssertionResult{Type: "ntp_check", Expected: fmt.Sprintf("offset <= %dms", check.MaxOffsetMs), Actual: fmt.Sprintf("offset=%dms", offsetMs), Passed: false}
		}
		return AssertionResult{Type: "ntp_check", Expected: server, Actual: fmt.Sprintf("stratum=%d offset=%dms", stratum, offsetMs), Passed: true}
	}

	return AssertionResult{Type: "ntp_check", Expected: server, Actual: fmt.Sprintf("stratum=%d offset=%s", stratum, offset.Round(time.Millisecond)), Passed: true}
}

// timeToNTP converts a time.Time to NTP seconds and fraction (since Jan 1, 1900).
func timeToNTP(t time.Time) (sec uint32, frac uint32) {
	ntpEpoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	d := t.Sub(ntpEpoch)
	sec = uint32(d.Seconds())
	frac = uint32(float64(d%time.Second) / float64(time.Second) * float64(1<<32))
	return
}

// ntpToTime converts NTP seconds and fraction to time.Time.
func ntpToTime(sec uint32, frac uint32) time.Time {
	ntpEpoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	d := time.Duration(sec)*time.Second + time.Duration(float64(frac)/float64(1<<32)*float64(time.Second))
	return ntpEpoch.Add(d)
}
