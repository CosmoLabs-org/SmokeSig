package runner

import (
	"net"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// --- Validation tests for new assertion types ---

func TestValidate_Ping_MissingHost(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []schema.Test{
			{Name: "ping-test", Expect: schema.Expect{Ping: &schema.PingCheck{}}},
		},
	}
	err := schema.Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing ping.host")
	}
	ve, ok := err.(*schema.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	found := false
	for _, e := range ve.Errors {
		if contains(e, "ping.host is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'ping.host is required' error, got: %v", ve.Errors)
	}
}

func TestValidate_Kafka_MissingBrokers(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []schema.Test{
			{Name: "kafka-test", Expect: schema.Expect{Kafka: &schema.KafkaCheck{}}},
		},
	}
	err := schema.Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing kafka.brokers")
	}
	ve := err.(*schema.ValidationError)
	found := false
	for _, e := range ve.Errors {
		if contains(e, "kafka_broker.brokers is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'kafka_broker.brokers is required' error, got: %v", ve.Errors)
	}
}

func TestValidate_LDAP_MissingHost(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []schema.Test{
			{Name: "ldap-test", Expect: schema.Expect{LDAP: &schema.LDAPCheck{}}},
		},
	}
	err := schema.Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing ldap.host")
	}
}

func TestValidate_MQTT_MissingBroker(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []schema.Test{
			{Name: "mqtt-test", Expect: schema.Expect{MQTT: &schema.MQTTCheck{}}},
		},
	}
	err := schema.Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing mqtt.broker")
	}
}

func TestValidate_K8sResource_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		check *schema.K8sResourceCheck
		wantErr string
	}{
		{"missing namespace", &schema.K8sResourceCheck{Kind: "pod", Name: "test"}, "k8s_resource.namespace is required"},
		{"missing kind", &schema.K8sResourceCheck{Namespace: "default", Name: "test"}, "k8s_resource.kind is required"},
		{"missing name", &schema.K8sResourceCheck{Namespace: "default", Kind: "pod"}, "k8s_resource.name is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &schema.SmokeConfig{
				Version: 1,
				Project: "test",
				Tests: []schema.Test{
					{Name: "k8s-test", Expect: schema.Expect{K8sResource: tc.check}},
				},
			}
			err := schema.Validate(cfg)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			ve := err.(*schema.ValidationError)
			found := false
			for _, e := range ve.Errors {
				if contains(e, tc.wantErr) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected '%s' error, got: %v", tc.wantErr, ve.Errors)
			}
		})
	}
}

func TestValidate_NewTypes_Standalone(t *testing.T) {
	// All new types should be standalone (no run command required)
	types := []struct {
		name string
		expect schema.Expect
	}{
		{"ping", schema.Expect{Ping: &schema.PingCheck{Host: "localhost"}}},
		{"mongo", schema.Expect{Mongo: &schema.MongoCheck{}}},
		{"kafka", schema.Expect{Kafka: &schema.KafkaCheck{Brokers: []string{"localhost:9092"}}}},
		{"ldap", schema.Expect{LDAP: &schema.LDAPCheck{Host: "localhost"}}},
		{"mqtt", schema.Expect{MQTT: &schema.MQTTCheck{Broker: "localhost:1883"}}},
		{"ntp", schema.Expect{NTP: &schema.NTPCheck{}}},
		{"k8s", schema.Expect{K8sResource: &schema.K8sResourceCheck{Namespace: "default", Kind: "pod", Name: "test"}}},
	}

	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &schema.SmokeConfig{
				Version: 1,
				Project: "test",
				Tests: []schema.Test{
					{Name: tc.name + "-test", Expect: tc.expect},
				},
			}
			err := schema.Validate(cfg)
			if err != nil {
				t.Errorf("expected %s standalone assertion to pass validation, got: %v", tc.name, err)
			}
		})
	}
}

// --- Malformed wire response tests ---

func TestCheckMongoPing_MalformedResponse(t *testing.T) {
	ln := mustListen(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		// Send garbage response
		conn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	}()

	result := CheckMongoPing(&schema.MongoCheck{Host: "127.0.0.1", Port: portFromListener(t, ln)})
	if result.Passed {
		t.Error("expected mongo ping to fail with malformed response")
	}
}

func TestCheckMongoPing_WrongOpCode(t *testing.T) {
	ln := mustListen(t)
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
		if n < 16 {
			return
		}
		reqID := leUint32(buf[4:8])
		// Response with wrong opCode (2000 instead of 1)
		resp := make([]byte, 20)
		lePutUint32(resp[0:4], 20)
		lePutUint32(resp[4:8], 2)
		lePutUint32(resp[8:12], reqID)
		lePutUint32(resp[12:16], 2000) // wrong opCode
		lePutUint32(resp[16:20], 0)
		conn.Write(resp)
	}()

	result := CheckMongoPing(&schema.MongoCheck{Host: "127.0.0.1", Port: portFromListener(t, ln)})
	if result.Passed {
		t.Error("expected mongo ping to fail with wrong opCode")
	}
}

func TestCheckKafkaBroker_MalformedResponse(t *testing.T) {
	ln := mustListen(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		// Send only 3 bytes — too short for header
		conn.Write([]byte{0x00, 0x00, 0x00})
	}()

	result := CheckKafkaBroker(&schema.KafkaCheck{Brokers: []string{ln.Addr().String()}})
	if result.Passed {
		t.Error("expected kafka check to fail with truncated response")
	}
}

func TestCheckKafkaBroker_WrongCorrelationID(t *testing.T) {
	ln := mustListen(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		// Response with wrong correlation ID
		resp := make([]byte, 8)
		bePutUint32(resp[0:4], 4)
		bePutUint32(resp[4:8], 999) // wrong correlation ID
		conn.Write(resp)
	}()

	result := CheckKafkaBroker(&schema.KafkaCheck{Brokers: []string{ln.Addr().String()}})
	if result.Passed {
		t.Error("expected kafka check to fail with wrong correlation ID")
	}
}

func TestCheckLDAPBind_MalformedResponse(t *testing.T) {
	ln := mustListen(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		// Send invalid BER (not a SEQUENCE)
		conn.Write([]byte{0x05, 0x00}) // NULL TLV
	}()

	result := CheckLDAPBind(&schema.LDAPCheck{Host: "127.0.0.1", Port: portFromListener(t, ln)})
	if result.Passed {
		t.Error("expected LDAP bind to fail with malformed response")
	}
}

func TestCheckLDAPBind_TruncatedResponse(t *testing.T) {
	ln := mustListen(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		// Send only 3 bytes
		conn.Write([]byte{0x30, 0x02, 0x02})
	}()

	result := CheckLDAPBind(&schema.LDAPCheck{Host: "127.0.0.1", Port: portFromListener(t, ln)})
	if result.Passed {
		t.Error("expected LDAP bind to fail with truncated response")
	}
}

func TestCheckMQTTPing_MalformedResponse(t *testing.T) {
	ln := mustListen(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		// Send wrong packet type (0x30 = PUBLISH instead of 0x20 CONNACK)
		conn.Write([]byte{0x30, 0x02, 0x00, 0x00})
	}()

	result := CheckMQTTPing(&schema.MQTTCheck{Broker: ln.Addr().String()})
	if result.Passed {
		t.Error("expected MQTT ping to fail with wrong packet type")
	}
}

func TestCheckMQTTPing_TruncatedResponse(t *testing.T) {
	ln := mustListen(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf)
		// Send only 2 bytes
		conn.Write([]byte{0x20, 0x02})
	}()

	result := CheckMQTTPing(&schema.MQTTCheck{Broker: ln.Addr().String()})
	if result.Passed {
		t.Error("expected MQTT ping to fail with truncated response")
	}
}

func TestCheckMQTTPing_AllReturnCodes(t *testing.T) {
	codes := []struct {
		code   byte
		reason string
	}{
		{2, "identifier rejected"},
		{3, "server unavailable"},
		{4, "bad username or password"},
	}

	for _, tc := range codes {
		t.Run(tc.reason, func(t *testing.T) {
			ln := mustListen(t)
			defer ln.Close()

			go func(code byte) {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				buf := make([]byte, 4096)
				conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				conn.Read(buf)
				conn.Write([]byte{0x20, 0x02, 0x00, code})
			}(tc.code)

			result := CheckMQTTPing(&schema.MQTTCheck{Broker: ln.Addr().String()})
			if result.Passed {
				t.Errorf("expected failure for code %d", tc.code)
			}
			if !contains(result.Actual, tc.reason) {
				t.Errorf("expected '%s', got '%s'", tc.reason, result.Actual)
			}
		})
	}
}

// --- Shell-out edge case tests ---

func TestCheckPing_CustomCount(t *testing.T) {
	result := CheckPing(&schema.PingCheck{Host: "127.0.0.1", Count: 3})
	if !result.Passed {
		t.Errorf("expected ping with count=3 to pass, got: %s", result.Actual)
	}
}

func TestCheckPing_InvalidHost(t *testing.T) {
	result := CheckPing(&schema.PingCheck{Host: "256.256.256.256"})
	if result.Passed {
		t.Error("expected ping to invalid host to fail")
	}
}

func TestCheckK8sResource_WithCondition(t *testing.T) {
	result := CheckK8sResource(&schema.K8sResourceCheck{
		Namespace: "default",
		Kind:      "pod",
		Name:      "test",
		Condition: "Ready",
	})
	if result.Type != "k8s_resource" {
		t.Errorf("expected type 'k8s_resource', got %s", result.Type)
	}
	// Will likely fail (no kubectl or no cluster), just verify structure
}

func TestCheckK8sResource_WithContext(t *testing.T) {
	result := CheckK8sResource(&schema.K8sResourceCheck{
		Context:   "my-context",
		Namespace: "kube-system",
		Kind:      "service",
		Name:      "kubernetes",
	})
	if result.Type != "k8s_resource" {
		t.Errorf("expected type 'k8s_resource', got %s", result.Type)
	}
}

func TestCheckK8sResource_ExpectedFormat(t *testing.T) {
	result := CheckK8sResource(&schema.K8sResourceCheck{
		Namespace: "default",
		Kind:      "deployment",
		Name:      "nginx",
	})
	expected := "deployment/nginx in default"
	if result.Expected != expected {
		t.Errorf("expected '%s', got '%s'", expected, result.Expected)
	}
}

// --- Helper functions ---

func mustListen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func portFromListener(t *testing.T, ln net.Listener) int {
	t.Helper()
	return ln.Addr().(*net.TCPAddr).Port
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func leUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func lePutUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func bePutUint32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}
