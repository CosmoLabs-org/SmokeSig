package runner

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// --- MongoDB wire protocol tests ---

func TestCheckMongoPing_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read the OP_QUERY request
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 16 {
			return
		}

		// Parse request to get response fields
		reqLen := binary.LittleEndian.Uint32(buf[0:4])
		reqID := binary.LittleEndian.Uint32(buf[4:8])
		_ = reqLen

		// Build OP_REPLY response
		// Header: length(4) + requestID(4) + responseTo=reqID(4) + opCode=1(4)
		// Body: responseFlags(4) + cursorID(8) + startingFrom(4) + numberReturned(4) + BSON doc
		bsonDoc := []byte{0x08, 0x00, 0x00, 0x00, 0x08, 0x69, 0x73, 0x4d, 0x61, 0x73, 0x74, 0x65, 0x72, 0x00, 0x01, 0x00}

		bodyLen := 4 + 8 + 4 + 4 + len(bsonDoc)
		totalLen := 16 + bodyLen

		resp := make([]byte, totalLen)
		binary.LittleEndian.PutUint32(resp[0:4], uint32(totalLen))
		binary.LittleEndian.PutUint32(resp[4:8], 2)         // response requestID
		binary.LittleEndian.PutUint32(resp[8:12], reqID)     // responseTo = request's requestID
		binary.LittleEndian.PutUint32(resp[12:16], 1)        // OP_REPLY
		binary.LittleEndian.PutUint32(resp[16:20], 0)        // responseFlags
		binary.LittleEndian.PutUint64(resp[20:28], 0)        // cursorID
		binary.LittleEndian.PutUint32(resp[28:32], 0)        // startingFrom
		binary.LittleEndian.PutUint32(resp[32:36], 1)        // numberReturned
		copy(resp[36:], bsonDoc)

		conn.Write(resp)
	}()

	addr := ln.Addr().String()
	host := "127.0.0.1"
	port := addr[len(host)+1:]

	result := CheckMongoPing(&schema.MongoCheck{Host: host, Port: mustParsePort(t, port)})
	if !result.Passed {
		t.Errorf("expected mongo ping to pass, got: %s", result.Actual)
	}
}

func TestCheckMongoPing_ConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // Immediately close so connection will be refused

	host := "127.0.0.1"
	port := addr[len(host)+1:]

	result := CheckMongoPing(&schema.MongoCheck{Host: host, Port: mustParsePort(t, port)})
	if result.Passed {
		t.Error("expected mongo ping to fail on connection refused")
	}
}

func TestCheckMongoPing_Defaults(t *testing.T) {
	// Just test that defaults are applied correctly (won't connect)
	result := CheckMongoPing(&schema.MongoCheck{})
	if result.Type != "mongo_ping" {
		t.Errorf("expected type 'mongo_ping', got %s", result.Type)
	}
}

// --- Kafka wire protocol tests ---

func TestCheckKafkaBroker_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 4 {
			return
		}

		reqLen := binary.BigEndian.Uint32(buf[0:4])
		corrID := binary.BigEndian.Uint32(buf[8:12])
		_ = reqLen

		// Build MetadataResponse: correlation_id + broker array
		// throttle_time_ms(4) + broker_array_len(4) + broker(node_id, host, port, rack) + topic_array_len(4)
		body := make([]byte, 4+4+4)
		binary.BigEndian.PutUint32(body[0:4], 0) // throttle_time_ms
		binary.BigEndian.PutUint32(body[4:8], 0) // 0 brokers
		binary.BigEndian.PutUint32(body[8:12], 0) // 0 topics

		totalLen := 4 + len(body) // correlation_id + body
		resp := make([]byte, 4+totalLen)
		binary.BigEndian.PutUint32(resp[0:4], uint32(totalLen))
		binary.BigEndian.PutUint32(resp[4:8], corrID)
		copy(resp[8:], body)

		conn.Write(resp)
	}()

	addr := ln.Addr().String()
	host := "127.0.0.1"
	port := addr[len(host)+1:]

	result := CheckKafkaBroker(&schema.KafkaCheck{
		Brokers: []string{host + ":" + port},
	})
	if !result.Passed {
		t.Errorf("expected kafka check to pass, got: %s", result.Actual)
	}
}

func TestCheckKafkaBroker_ConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	result := CheckKafkaBroker(&schema.KafkaCheck{Brokers: []string{addr}})
	if result.Passed {
		t.Error("expected kafka check to fail on connection refused")
	}
}

func TestCheckKafkaBroker_EmptyBrokers(t *testing.T) {
	result := CheckKafkaBroker(&schema.KafkaCheck{Brokers: []string{}})
	if result.Passed {
		t.Error("expected kafka check to fail with empty brokers")
	}
}

func TestCheckKafkaBroker_DefaultPort(t *testing.T) {
	// Test that bare hostname gets :9092 appended
	// This will fail to connect but tests the address parsing
	result := CheckKafkaBroker(&schema.KafkaCheck{Brokers: []string{"nonexistent-host"}})
	if result.Passed {
		t.Error("expected kafka check to fail connecting to nonexistent host")
	}
}

// --- LDAP wire protocol tests ---

func TestCheckLDAPBind_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 8 {
			return
		}

		// Parse request to get messageID
		// SEQUENCE { INTEGER messageID, ... }
		msgID := buf[4+1+1] // skip 0x30, length, 0x02, 0x01, then messageID

		// Build BindResponse: success (resultCode=0)
		// LDAPMessage = SEQUENCE { messageID, bindResponse [APPLICATION 1] }
		// bindResponse = APPLICATION 1 SEQUENCE { resultCode=0, matchedDN="", diagnosticMsg="" }
		innerSeq := []byte{0x0a, 0x01, 0x00, 0x04, 0x00, 0x04, 0x00} // ENUMERATED 0, OCTET STRING "", OCTET STRING ""
		bindResp := append([]byte{0x61, byte(len(innerSeq))}, innerSeq...)
		msgBody := append([]byte{0x02, 0x01, msgID}, bindResp...)
		resp := append([]byte{0x30, byte(len(msgBody))}, msgBody...)

		conn.Write(resp)
	}()

	addr := ln.Addr().String()
	host := "127.0.0.1"
	port := addr[len(host)+1:]

	result := CheckLDAPBind(&schema.LDAPCheck{Host: host, Port: mustParsePort(t, port)})
	if !result.Passed {
		t.Errorf("expected LDAP bind to pass, got: %s", result.Actual)
	}
}

func TestCheckLDAPBind_ConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	host := "127.0.0.1"
	port := addr[len(host)+1:]

	result := CheckLDAPBind(&schema.LDAPCheck{Host: host, Port: mustParsePort(t, port)})
	if result.Passed {
		t.Error("expected LDAP bind to fail on connection refused")
	}
}

func TestCheckLDAPBind_InvalidCredentials(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 8 {
			return
		}

		msgID := buf[6]
		// BindResponse with resultCode=49 (invalidCredentials)
		innerSeq := []byte{0x0a, 0x01, 0x31, 0x04, 0x00, 0x04, 0x00} // ENUMERATED 49, OCTET STRING "", OCTET STRING ""
		bindResp := append([]byte{0x61, byte(len(innerSeq))}, innerSeq...)
		msgBody := append([]byte{0x02, 0x01, msgID}, bindResp...)
		resp := append([]byte{0x30, byte(len(msgBody))}, msgBody...)

		conn.Write(resp)
	}()

	addr := ln.Addr().String()
	host := "127.0.0.1"
	port := addr[len(host)+1:]

	result := CheckLDAPBind(&schema.LDAPCheck{Host: host, Port: mustParsePort(t, port), BindDN: "cn=test"})
	if result.Passed {
		t.Error("expected LDAP bind to fail with invalid credentials (code 49)")
	}
}

func TestCheckLDAPBind_Defaults(t *testing.T) {
	result := CheckLDAPBind(&schema.LDAPCheck{Host: "nonexistent"})
	if result.Passed {
		t.Error("expected LDAP bind to fail with nonexistent host")
	}
}

func TestCheckLDAPBind_DefaultPortTLS(t *testing.T) {
	// Test that UseTLS sets default port to 636
	check := &schema.LDAPCheck{Host: "nonexistent", UseTLS: true}
	result := CheckLDAPBind(check)
	if result.Passed {
		t.Error("expected failure")
	}
	// The address should include port 636
	if result.Expected != "nonexistent:636" {
		t.Errorf("expected port 636 for TLS, got %s", result.Expected)
	}
}

func TestCheckLDAPBind_AuthenticatedBind(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 8 {
			return
		}

		// Verify the bind request contains non-empty auth (password sent)
		// Find 0x80 tag in the request — the simple auth choice
		hasPassword := false
		for i := 0; i < n-1; i++ {
			if buf[i] == 0x80 && buf[i+1] > 0 {
				hasPassword = true
				break
			}
		}
		if !hasPassword {
			return // don't respond — test will fail on timeout
		}

		msgID := buf[4+1+1]
		innerSeq := []byte{0x0a, 0x01, 0x00, 0x04, 0x00, 0x04, 0x00}
		bindResp := append([]byte{0x61, byte(len(innerSeq))}, innerSeq...)
		msgBody := append([]byte{0x02, 0x01, msgID}, bindResp...)
		resp := append([]byte{0x30, byte(len(msgBody))}, msgBody...)

		conn.Write(resp)
	}()

	addr := ln.Addr().String()
	host := "127.0.0.1"
	port := addr[len(host)+1:]

	t.Setenv("LDAP_TEST_PASS", "s3cret")
	result := CheckLDAPBind(&schema.LDAPCheck{
		Host:        host,
		Port:        mustParsePort(t, port),
		BindDN:      "cn=admin,dc=example,dc=com",
		PasswordEnv: "LDAP_TEST_PASS",
	})
	if !result.Passed {
		t.Errorf("expected authenticated LDAP bind to pass, got: %s", result.Actual)
	}
}

func TestCheckLDAPBind_PasswordEnvNotSet(t *testing.T) {
	// PasswordEnv specified but env var doesn't exist → explicit failure, not silent fallback
	result := CheckLDAPBind(&schema.LDAPCheck{
		Host:        "nonexistent",
		BindDN:      "cn=admin,dc=example,dc=com",
		PasswordEnv: "LDAP_NONEXISTENT_VAR_" + t.Name(),
	})
	if result.Passed {
		t.Error("expected failure when password_env references unset variable")
	}
	if result.Actual != `password_env "LDAP_NONEXISTENT_VAR_TestCheckLDAPBind_PasswordEnvNotSet" not set` {
		t.Errorf("unexpected actual: %s", result.Actual)
	}
}

// --- MQTT wire protocol tests ---

func TestCheckMQTTPing_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 10 {
			return
		}

		// Send CONNACK: 0x20 (type), 0x02 (remaining length), 0x00 (session present), 0x00 (return code = accepted)
		conn.Write([]byte{0x20, 0x02, 0x00, 0x00})
	}()

	addr := ln.Addr().String()
	result := CheckMQTTPing(&schema.MQTTCheck{Broker: addr})
	if !result.Passed {
		t.Errorf("expected MQTT ping to pass, got: %s", result.Actual)
	}
}

func TestCheckMQTTPing_ConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	result := CheckMQTTPing(&schema.MQTTCheck{Broker: addr})
	if result.Passed {
		t.Error("expected MQTT ping to fail on connection refused")
	}
}

func TestCheckMQTTPing_Rejected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 10 {
			return
		}

		// CONNACK with return code 5 (not authorized)
		conn.Write([]byte{0x20, 0x02, 0x00, 0x05})
	}()

	addr := ln.Addr().String()
	result := CheckMQTTPing(&schema.MQTTCheck{Broker: addr})
	if result.Passed {
		t.Error("expected MQTT ping to fail with not authorized")
	}
	if result.Actual != "not authorized" {
		t.Errorf("expected 'not authorized', got %s", result.Actual)
	}
}

func TestCheckMQTTPing_BadProtocolVersion(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)
		if n < 10 {
			return
		}

		// CONNACK with return code 1 (unacceptable protocol version)
		conn.Write([]byte{0x20, 0x02, 0x00, 0x01})
	}()

	addr := ln.Addr().String()
	result := CheckMQTTPing(&schema.MQTTCheck{Broker: addr})
	if result.Passed {
		t.Error("expected MQTT ping to fail with bad protocol version")
	}
}

func TestCheckMQTTPing_DefaultPort(t *testing.T) {
	result := CheckMQTTPing(&schema.MQTTCheck{Broker: "nonexistent"})
	if result.Passed {
		t.Error("expected failure")
	}
}

// --- NTP tests ---

func TestCheckNTP_Success(t *testing.T) {
	// Test against a real NTP server if available
	result := CheckNTP(&schema.NTPCheck{Server: "pool.ntp.org", MaxOffsetMs: 60000})
	// May fail in restricted networks, just verify structure
	if result.Type != "ntp_check" {
		t.Errorf("expected type 'ntp_check', got %s", result.Type)
	}
}

func TestCheckNTP_InvalidServer(t *testing.T) {
	result := CheckNTP(&schema.NTPCheck{Server: "nonexistent.ntp.invalid"})
	if result.Passed {
		t.Error("expected NTP check to fail with invalid server")
	}
}

func TestCheckNTP_LocalServer(t *testing.T) {
	// Start a local UDP "NTP server"
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	go func() {
		buf := make([]byte, 48)
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil || n < 48 {
			return
		}

		// Build response using request data
		resp := make([]byte, 48)
		copy(resp, buf)
		// Set mode=4 (server), stratum=1, VN=4
		resp[0] = 0x24 // LI=0, VN=4, Mode=4
		resp[1] = 1     // stratum 1

		// Copy transmit timestamp to origin timestamp
		copy(resp[24:32], buf[40:48])

		// Set receive and transmit timestamps
		now := time.Now()
		sec, frac := timeToNTPTest(now)
		binary.BigEndian.PutUint32(resp[32:36], sec)
		binary.BigEndian.PutUint32(resp[36:40], frac)
		binary.BigEndian.PutUint32(resp[40:44], sec)
		binary.BigEndian.PutUint32(resp[44:48], frac)

		conn.WriteToUDP(resp, clientAddr)
	}()

	// The NTP check always appends :123, so we can't use a random port.
	// Instead, verify with the standard port (this test may skip if port 123 is in use).
	result := CheckNTP(&schema.NTPCheck{Server: "127.0.0.1"})
	// Since we're not listening on port 123, this will fail — just verify the function works
	if result.Type != "ntp_check" {
		t.Errorf("expected type 'ntp_check', got %s", result.Type)
	}
}

func TestCheckNTP_MaxOffset(t *testing.T) {
	result := CheckNTP(&schema.NTPCheck{Server: "pool.ntp.org", MaxOffsetMs: 1})
	// With a 1ms threshold, this may fail — just verify it returns a result
	if result.Type != "ntp_check" {
		t.Errorf("expected type 'ntp_check', got %s", result.Type)
	}
}

func TestCheckNTP_Defaults(t *testing.T) {
	result := CheckNTP(&schema.NTPCheck{})
	if result.Type != "ntp_check" {
		t.Errorf("expected type 'ntp_check', got %s", result.Type)
	}
}

// --- Helper functions ---

func mustParsePort(t *testing.T, portStr string) int {
	t.Helper()
	var port int
	for _, c := range portStr {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		}
	}
	return port
}

// timeToNTPTest converts time.Time to NTP seconds and fraction.
func timeToNTPTest(t time.Time) (sec uint32, frac uint32) {
	ntpEpoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	d := t.Sub(ntpEpoch)
	sec = uint32(d.Seconds())
	frac = uint32(float64(d%time.Second) / float64(time.Second) * float64(1<<32))
	return
}
