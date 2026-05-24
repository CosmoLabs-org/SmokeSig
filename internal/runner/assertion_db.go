package runner

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckRedisPing issues a PING to a Redis server and expects +PONG.
func CheckRedisPing(check *schema.RedisCheck) AssertionResult {
	host := check.Host
	if host == "" {
		host = "localhost"
	}
	port := check.Port
	if port == 0 {
		port = 6379
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return AssertionResult{Type: "redis_ping", Expected: addr, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck

	// Optional AUTH
	if check.Password != "" {
		authCmd := fmt.Sprintf("*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n", len(check.Password), check.Password)
		if _, err := conn.Write([]byte(authCmd)); err != nil {
			return AssertionResult{Type: "redis_ping", Expected: addr, Actual: "auth write error: "+err.Error(), Passed: false}
		}
		buf := make([]byte, 128)
		n, _ := conn.Read(buf)
		if !strings.HasPrefix(string(buf[:n]), "+OK") {
			return AssertionResult{Type: "redis_ping", Expected: "+OK", Actual: strings.TrimSpace(string(buf[:n])), Passed: false}
		}
	}

	if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		return AssertionResult{Type: "redis_ping", Expected: addr, Actual: err.Error(), Passed: false}
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return AssertionResult{Type: "redis_ping", Expected: "+PONG", Actual: err.Error(), Passed: false}
	}
	reply := strings.TrimSpace(string(buf[:n]))
	if !strings.HasPrefix(reply, "+PONG") {
		return AssertionResult{Type: "redis_ping", Expected: "+PONG", Actual: reply, Passed: false}
	}
	return AssertionResult{Type: "redis_ping", Expected: addr, Actual: "PONG", Passed: true}
}

// CheckMemcachedVersion issues `version` to Memcached and expects a VERSION line.
func CheckMemcachedVersion(check *schema.MemcachedCheck) AssertionResult {
	host := check.Host
	if host == "" {
		host = "localhost"
	}
	port := check.Port
	if port == 0 {
		port = 11211
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return AssertionResult{Type: "memcached_version", Expected: addr, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck

	if _, err := conn.Write([]byte("version\r\n")); err != nil {
		return AssertionResult{Type: "memcached_version", Expected: addr, Actual: err.Error(), Passed: false}
	}
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		return AssertionResult{Type: "memcached_version", Expected: "VERSION", Actual: err.Error(), Passed: false}
	}
	reply := strings.TrimSpace(string(buf[:n]))
	if !strings.HasPrefix(reply, "VERSION") {
		return AssertionResult{Type: "memcached_version", Expected: "VERSION ...", Actual: reply, Passed: false}
	}
	return AssertionResult{Type: "memcached_version", Expected: addr, Actual: reply, Passed: true}
}

// CheckPostgresPing sends an SSLRequest to a Postgres server and verifies a protocol-valid response.
func CheckPostgresPing(check *schema.PostgresCheck) AssertionResult {
	host := check.Host
	if host == "" {
		host = "localhost"
	}
	port := check.Port
	if port == 0 {
		port = 5432
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return AssertionResult{Type: "postgres_ping", Expected: addr, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck

	// SSLRequest message: int32 length (8), int32 code (80877103)
	// Bytes: 00 00 00 08 04 D2 16 2F
	sslReq := []byte{0x00, 0x00, 0x00, 0x08, 0x04, 0xD2, 0x16, 0x2F}
	if _, err := conn.Write(sslReq); err != nil {
		return AssertionResult{Type: "postgres_ping", Expected: addr, Actual: "write error: " + err.Error(), Passed: false}
	}
	buf := make([]byte, 1)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return AssertionResult{Type: "postgres_ping", Expected: "S or N", Actual: fmt.Sprintf("read error: %v", err), Passed: false}
	}
	reply := buf[0]
	// 'S' = SSL supported, 'N' = SSL not supported, 'E' = error message follows (still valid postgres)
	if reply == 'S' || reply == 'N' || reply == 'E' {
		return AssertionResult{Type: "postgres_ping", Expected: addr, Actual: string(reply), Passed: true}
	}
	return AssertionResult{Type: "postgres_ping", Expected: "S/N/E", Actual: fmt.Sprintf("0x%02x", reply), Passed: false}
}

// CheckMySQLPing verifies a MySQL server sends a valid v10 handshake packet on connection.
func CheckMySQLPing(check *schema.MySQLCheck) AssertionResult {
	host := check.Host
	if host == "" {
		host = "localhost"
	}
	port := check.Port
	if port == 0 {
		port = 3306
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return AssertionResult{Type: "mysql_ping", Expected: addr, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck

	// MySQL server immediately sends a handshake packet:
	// [3 bytes: payload length][1 byte: sequence id][payload starts with 1 byte: protocol version]
	// Protocol version 10 (0x0a) is "v10", the current universally-used version.
	hdr := make([]byte, 5)
	n, err := conn.Read(hdr)
	if err != nil || n < 5 {
		return AssertionResult{Type: "mysql_ping", Expected: "handshake", Actual: fmt.Sprintf("read error: %v (n=%d)", err, n), Passed: false}
	}
	protocolVersion := hdr[4]
	if protocolVersion != 0x0a {
		return AssertionResult{Type: "mysql_ping", Expected: "protocol v10 (0x0a)", Actual: fmt.Sprintf("0x%02x", protocolVersion), Passed: false}
	}
	return AssertionResult{Type: "mysql_ping", Expected: addr, Actual: "v10", Passed: true}
}
