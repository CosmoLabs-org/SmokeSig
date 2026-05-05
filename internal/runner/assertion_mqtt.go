package runner

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckMQTTPing sends a CONNECT packet to an MQTT broker and expects a CONNACK response.
func CheckMQTTPing(check *schema.MQTTCheck) AssertionResult {
	broker := check.Broker
	if !strings.HasPrefix(broker, "tcp://") && !strings.HasPrefix(broker, "ssl://") {
		broker = "tcp://" + broker
	}

	// Parse broker address
	if strings.HasPrefix(broker, "ssl://") || strings.HasPrefix(broker, "tls://") {
		return AssertionResult{Type: "mqtt_ping", Expected: broker, Actual: "ssl:// not yet supported, use tcp:// or bare host:port", Passed: false}
	}
	addr := strings.TrimPrefix(broker, "tcp://")
	if !strings.Contains(addr, ":") {
		addr = addr + ":1883"
	}

	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return AssertionResult{Type: "mqtt_ping", Expected: broker, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// Build MQTT CONNECT packet
	clientID := check.ClientID
	if clientID == "" {
		clientID = "cosmo-smoke"
	}

	// Variable header: protocol name "MQTT", protocol level 4, connect flags, keep alive
	protoName := encodeMQTTString("MQTT")
	protocolLevel := []byte{0x04}        // MQTT 3.1.1
	connectFlags := []byte{0x02}          // Clean session flag only
	keepAlive := make([]byte, 2)
	binary.BigEndian.PutUint16(keepAlive, 60) // 60 seconds

	varHeader := append(append(append(protoName, protocolLevel...), connectFlags...), keepAlive...)

	// Payload: client identifier
	payload := encodeMQTTString(clientID)

	// Fixed header: packet type (CONNECT=0x10) + remaining length
	remaining := len(varHeader) + len(payload)
	fixedHeader := append([]byte{0x10}, encodeRemainingLength(remaining)...)

	packet := append(append(fixedHeader, varHeader...), payload...)

	if _, err := conn.Write(packet); err != nil {
		return AssertionResult{Type: "mqtt_ping", Expected: broker, Actual: "write error: " + err.Error(), Passed: false}
	}

	// Read CONNACK: fixed header (0x20) + remaining length + session present + return code
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil || n < 4 {
		return AssertionResult{Type: "mqtt_ping", Expected: broker, Actual: fmt.Sprintf("read error: %v (n=%d)", err, n), Passed: false}
	}

	if buf[0] != 0x20 {
		return AssertionResult{Type: "mqtt_ping", Expected: "CONNACK (0x20)", Actual: fmt.Sprintf("0x%02x", buf[0]), Passed: false}
	}

	// Return code is at offset 3 (fixed header + remaining length + session present)
	returnCode := buf[3]
	if returnCode == 0 {
		return AssertionResult{Type: "mqtt_ping", Expected: broker, Actual: "connection accepted", Passed: true}
	}

	reasons := map[byte]string{
		1: "unacceptable protocol version",
		2: "identifier rejected",
		3: "server unavailable",
		4: "bad username or password",
		5: "not authorized",
	}
	reason := reasons[returnCode]
	if reason == "" {
		reason = fmt.Sprintf("code %d", returnCode)
	}

	return AssertionResult{Type: "mqtt_ping", Expected: broker, Actual: reason, Passed: false}
}

// encodeMQTTString encodes a UTF-8 string in MQTT format: 2-byte length prefix + bytes.
func encodeMQTTString(s string) []byte {
	b := make([]byte, 2+len(s))
	binary.BigEndian.PutUint16(b[0:2], uint16(len(s)))
	copy(b[2:], s)
	return b
}

// encodeRemainingLength encodes an MQTT remaining length field (variable-length encoding).
func encodeRemainingLength(length int) []byte {
	var encoded []byte
	for {
		byte_ := byte(length % 128)
		length /= 128
		if length > 0 {
			byte_ |= 0x80
		}
		encoded = append(encoded, byte_)
		if length == 0 {
			break
		}
	}
	return encoded
}
