package runner

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckKafkaBroker sends a MetadataRequest to a Kafka broker and verifies a valid response.
func CheckKafkaBroker(check *schema.KafkaCheck) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	var lastErr string
	for _, broker := range check.Brokers {
		if !strings.Contains(broker, ":") {
			broker = broker + ":9092"
		}
		conn, err := net.DialTimeout("tcp", broker, timeout)
		if err != nil {
			lastErr = fmt.Sprintf("%s: %s", broker, err.Error())
			continue
		}

		result := checkKafkaConn(conn, broker, check.Topic, timeout)
		conn.Close()
		if result.Passed {
			return result
		}
		lastErr = result.Actual
	}

	return AssertionResult{Type: "kafka_broker", Expected: strings.Join(check.Brokers, ","), Actual: lastErr, Passed: false}
}

func checkKafkaConn(conn net.Conn, broker, topic string, timeout time.Duration) AssertionResult {
	conn.SetDeadline(time.Now().Add(timeout))

	// Kafka MetadataRequest (API key 3, version 0)
	// Request header: api_key(2) + api_version(2) + correlation_id(4) + client_id_len(2) + client_id
	// Body: topic_array_len(4) [+ topic_name_len(2) + topic_name]

	clientID := "cosmo-smoke"
	clientIDBytes := []byte(clientID)

	var topicBytes []byte
	if topic != "" {
		topicBytes = []byte(topic)
	}

	// Calculate sizes
	headerSize := 2 + 2 + 4 + 2 + len(clientIDBytes) // api_key + api_version + correlation + client_id_len + client_id
	bodySize := 4                                       // topic array length
	if topic != "" {
		bodySize += 2 + len(topicBytes) // topic string
	}
	requestSize := headerSize + bodySize

	// Total message: int32(size) + payload
	msg := make([]byte, 4+requestSize)
	binary.BigEndian.PutUint32(msg[0:4], uint32(requestSize)) // length prefix
	binary.BigEndian.PutUint16(msg[4:6], 3)                  // API key: Metadata
	binary.BigEndian.PutUint16(msg[6:8], 0)                  // API version: 0
	binary.BigEndian.PutUint32(msg[8:12], 1)                 // correlation ID
	binary.BigEndian.PutUint16(msg[12:14], uint16(len(clientIDBytes)))
	copy(msg[14:14+len(clientIDBytes)], clientIDBytes)

	offset := 14 + len(clientIDBytes)
	if topic != "" {
		binary.BigEndian.PutUint32(msg[offset:offset+4], 1) // 1 topic
		offset += 4
		binary.BigEndian.PutUint16(msg[offset:offset+2], uint16(len(topicBytes)))
		offset += 2
		copy(msg[offset:], topicBytes)
	} else {
		binary.BigEndian.PutUint32(msg[offset:offset+4], 0) // 0 topics = all topics
	}

	if _, err := conn.Write(msg); err != nil {
		return AssertionResult{Type: "kafka_broker", Expected: broker, Actual: "write error: " + err.Error(), Passed: false}
	}

	// Read response: int32(size) + int32(correlation_id)
	resp := make([]byte, 8)
	n, err := conn.Read(resp)
	if err != nil || n < 8 {
		return AssertionResult{Type: "kafka_broker", Expected: broker, Actual: fmt.Sprintf("read error: %v (n=%d)", err, n), Passed: false}
	}

	respSize := binary.BigEndian.Uint32(resp[0:4])
	corrID := binary.BigEndian.Uint32(resp[4:8])

	if corrID != 1 {
		return AssertionResult{Type: "kafka_broker", Expected: broker, Actual: fmt.Sprintf("correlation_id mismatch: got %d", corrID), Passed: false}
	}

	// Read rest of response
	remaining := int(respSize) - 4 // already read correlation_id
	if remaining > 0 {
		body := make([]byte, remaining)
		n, err := conn.Read(body)
		if err != nil || n < remaining {
			// Partial read is OK for a connectivity check
			_ = n
		}
	}

	return AssertionResult{Type: "kafka_broker", Expected: broker, Actual: fmt.Sprintf("metadata OK (%d bytes)", respSize), Passed: true}
}
