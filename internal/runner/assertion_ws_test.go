package runner

import (
	"crypto/sha1"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

const testWSGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsTestServer creates an httptest server that upgrades to WebSocket and echoes messages.
func wsTestServer(handler func(conn net.Conn, msg string) string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Sec-WebSocket-Key")
		h := sha1.New()
		h.Write([]byte(key + testWSGUID))
		acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
		conn.Write([]byte(resp))

		defer conn.Close()
		for {
			msg, closed, err := wsReadFrame(conn)
			if err != nil || closed {
				return
			}
			if msg == "" {
				continue
			}
			reply := handler(conn, msg)
			if reply != "" {
				wsWriteTextFrame(conn, reply)
			}
		}
	}))
}

func wsWriteTextFrame(conn net.Conn, msg string) {
	frame := []byte{0x81}
	msgLen := len(msg)
	if msgLen <= 125 {
		frame = append(frame, byte(msgLen))
	} else if msgLen <= 65535 {
		frame = append(frame, 126, byte(msgLen>>8), byte(msgLen))
	}
	frame = append(frame, []byte(msg)...)
	conn.Write(frame)
}

func TestCheckWebSocket_ExpectContains_Pass(t *testing.T) {
	ts := wsTestServer(func(conn net.Conn, msg string) string {
		return "pong:" + msg
	})
	defer ts.Close()

	wsURL := "ws://" + strings.TrimPrefix(ts.URL, "http://")
	result := CheckWebSocket(&schema.WebSocketCheck{
		URL:            wsURL,
		Send:           "ping",
		ExpectContains: "pong",
		Timeout:        schema.Duration{Duration: 5 * time.Second},
	})
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}

func TestCheckWebSocket_ExpectMatches_Pass(t *testing.T) {
	ts := wsTestServer(func(conn net.Conn, msg string) string {
		return `{"status":"connected","id":42}`
	})
	defer ts.Close()

	wsURL := "ws://" + strings.TrimPrefix(ts.URL, "http://")
	result := CheckWebSocket(&schema.WebSocketCheck{
		URL:           wsURL,
		Send:          "hello",
		ExpectMatches: `connected.*42`,
		Timeout:       schema.Duration{Duration: 5 * time.Second},
	})
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}

func TestCheckWebSocket_NoMatch_Fail(t *testing.T) {
	ts := wsTestServer(func(conn net.Conn, msg string) string {
		return "hello world"
	})
	defer ts.Close()

	wsURL := "ws://" + strings.TrimPrefix(ts.URL, "http://")
	result := CheckWebSocket(&schema.WebSocketCheck{
		URL:            wsURL,
		Send:           "ping",
		ExpectContains: "pong",
		Timeout:        schema.Duration{Duration: 5 * time.Second},
	})
	if result.Passed {
		t.Error("expected fail")
	}
}

func TestCheckWebSocket_ConnectionRefused(t *testing.T) {
	result := CheckWebSocket(&schema.WebSocketCheck{
		URL:            "ws://127.0.0.1:1",
		ExpectContains: "anything",
		Timeout:        schema.Duration{Duration: 1 * time.Second},
	})
	if result.Passed {
		t.Error("expected fail for connection refused")
	}
}

func TestCheckWebSocket_ConnectOnly(t *testing.T) {
	ts := wsTestServer(func(conn net.Conn, msg string) string {
		return ""
	})
	defer ts.Close()

	wsURL := "ws://" + strings.TrimPrefix(ts.URL, "http://")
	result := CheckWebSocket(&schema.WebSocketCheck{
		URL:     wsURL,
		Timeout: schema.Duration{Duration: 5 * time.Second},
	})
	if !result.Passed {
		t.Errorf("expected pass for connect-only, got: %s", result.Actual)
	}
}

func TestCheckWebSocket_TraceparentInjected(t *testing.T) {
	ts := wsTestServer(func(conn net.Conn, msg string) string {
		return "ok:" + msg
	})
	defer ts.Close()

	check := &schema.WebSocketCheck{
		URL:            "ws://" + strings.TrimPrefix(ts.URL, "http://") + "/ws",
		Send:           "ping",
		ExpectContains: "ok:",
	}
	span := NewTraceContext().NewSpan()
	result := CheckWebSocketWithTrace(check, span)
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
	// The traceparent was injected into check.Headers before the call
	if check.Headers["traceparent"] != span.Traceparent() {
		t.Errorf("headers traceparent = %q, want %q", check.Headers["traceparent"], span.Traceparent())
	}
}
