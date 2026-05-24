package runner

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// websocketGUID is the magic GUID from RFC 6455 used for accept key computation.
const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsUpgrade performs a WebSocket handshake over a raw TCP connection.
func wsUpgrade(conn net.Conn, host, path string, timeout time.Duration, extraHeaders map[string]string) error {
	key := make([]byte, 16)
	rand.Read(key)
	clientKey := base64.StdEncoding.EncodeToString(key)

	var hdrs strings.Builder
	for k, v := range extraHeaders {
		hdrs.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	upgradeReq := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n%s\r\n", path, host, clientKey, hdrs.String())
	conn.SetDeadline(time.Now().Add(timeout))
	conn.Write([]byte(upgradeReq))

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("read upgrade response: %w", err)
	}

	resp := string(buf[:n])
	if !strings.Contains(resp, " 101 ") {
		return fmt.Errorf("upgrade failed: %s", strings.Split(resp, "\r\n")[0])
	}

	// Verify accept key
	expectedAccept := computeAcceptKey(clientKey)
	if !strings.Contains(resp, expectedAccept) {
		return fmt.Errorf("invalid accept key")
	}

	return nil
}

func computeAcceptKey(clientKey string) string {
	h := sha1.New()
	h.Write([]byte(clientKey + websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// wsReadFrame reads a single WebSocket frame and returns the payload as a string.
// Handles text (opcode 1), binary (opcode 2), close (opcode 8), and ping (opcode 9).
func wsReadFrame(conn net.Conn) (string, bool, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", false, fmt.Errorf("read frame header: %w", err)
	}

	opcode := header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	payloadLen := int64(header[1] & 0x7F)

	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return "", false, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return "", false, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext))
	}

	// Skip mask key if present (server frames should not be masked, but handle it)
	if masked {
		mask := make([]byte, 4)
		if _, err := io.ReadFull(conn, mask); err != nil {
			return "", false, err
		}
	}

	if opcode == 0x08 {
		// Close frame
		reason := ""
		if payloadLen > 0 {
			payload := make([]byte, payloadLen)
			if _, err := io.ReadFull(conn, payload); err != nil {
				return "", true, err
			}
			reason = string(payload)
		}
		return reason, true, nil
	}

	if opcode == 0x09 {
		// Ping — read payload, ignore (no pong response in this minimal client)
		if payloadLen > 0 {
			payload := make([]byte, payloadLen)
			if _, err := io.ReadFull(conn, payload); err != nil {
				return "", false, err
			}
		}
		return "", false, nil
	}

	// Text (1) or Binary (2) — read payload
	var payload []byte
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return "", false, err
		}
	}

	return string(payload), false, nil
}

// wsSendMessage writes a masked text frame (required by RFC 6455 for client frames).
func wsSendMessage(conn net.Conn, msg string) error {
	frame := []byte{0x81} // FIN + text opcode
	msgLen := len(msg)
	if msgLen <= 125 {
		frame = append(frame, byte(0x80|msgLen))
	} else if msgLen <= 65535 {
		frame = append(frame, 0x80|126)
		frame = append(frame, byte(msgLen>>8), byte(msgLen))
	} else {
		frame = append(frame, 0x80|127)
		for i := 7; i >= 0; i-- {
			frame = append(frame, byte(msgLen>>(i*8)))
		}
	}

	mask := make([]byte, 4)
	rand.Read(mask)
	frame = append(frame, mask...)

	masked := make([]byte, msgLen)
	for i, b := range []byte(msg) {
		masked[i] = b ^ mask[i%4]
	}
	frame = append(frame, masked...)

	_, err := conn.Write(frame)
	return err
}

// CheckWebSocket verifies a WebSocket endpoint is reachable and optionally matches response.
func CheckWebSocket(check *schema.WebSocketCheck) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// Parse URL to get host and path
	url := check.URL
	useTLS := strings.HasPrefix(url, "wss://")
	hostPath := strings.TrimPrefix(url, "ws://")
	hostPath = strings.TrimPrefix(hostPath, "wss://")
	parts := strings.SplitN(hostPath, "/", 2)
	host := parts[0]
	path := "/"
	if len(parts) == 2 {
		path = "/" + parts[1]
	}

	// Add default port if not specified
	if !strings.Contains(host, ":") {
		if useTLS {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}

	// Connect
	start := time.Now()
	var conn net.Conn
	var err error
	if useTLS {
		dialer := &net.Dialer{Timeout: timeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", host, nil)
	} else {
		conn, err = net.DialTimeout("tcp", host, timeout)
	}
	if err != nil {
		return AssertionResult{
			Type:     "websocket",
			Expected: fmt.Sprintf("%s reachable", check.URL),
			Actual:   fmt.Sprintf("connection failed: %v", err),
			Passed:   false,
		}
	}
	defer conn.Close()
	elapsed := time.Since(start)

	// Upgrade
	if err := wsUpgrade(conn, host, path, timeout, check.Headers); err != nil {
		return AssertionResult{
			Type:     "websocket",
			Expected: fmt.Sprintf("%s upgrade", check.URL),
			Actual:   fmt.Sprintf("upgrade failed: %v", err),
			Passed:   false,
		}
	}

	// Connect-only mode: no send and no expectations
	if check.Send == "" && check.ExpectContains == "" && check.ExpectMatches == "" {
		return AssertionResult{
			Type:     "websocket",
			Expected: fmt.Sprintf("%s connected", check.URL),
			Actual:   fmt.Sprintf("connected (%s)", elapsed.Round(time.Millisecond)),
			Passed:   true,
		}
	}

	// Send message if provided
	if check.Send != "" {
		if err := wsSendMessage(conn, check.Send); err != nil {
			return AssertionResult{
				Type:     "websocket",
				Expected: "send message",
				Actual:   fmt.Sprintf("send failed: %v", err),
				Passed:   false,
			}
		}
	}

	// Read response and match
	conn.SetDeadline(time.Now().Add(timeout))
	for {
		msg, closed, err := wsReadFrame(conn)
		if err != nil {
			return AssertionResult{
				Type:     "websocket",
				Expected: "receive message",
				Actual:   fmt.Sprintf("read failed: %v", err),
				Passed:   false,
			}
		}
		if closed {
			return AssertionResult{
				Type:     "websocket",
				Expected: "receive message",
				Actual:   fmt.Sprintf("server closed: %s", msg),
				Passed:   false,
			}
		}
		// Skip empty frames (e.g. pong responses)
		if msg == "" {
			continue
		}

		if check.ExpectContains != "" {
			if strings.Contains(msg, check.ExpectContains) {
				return AssertionResult{
					Type:     "websocket",
					Expected: fmt.Sprintf("contains %q", check.ExpectContains),
					Actual:   msg,
					Passed:   true,
				}
			}
			return AssertionResult{
				Type:     "websocket",
				Expected: fmt.Sprintf("contains %q", check.ExpectContains),
				Actual:   fmt.Sprintf("received %q did not contain", msg),
				Passed:   false,
			}
		}

		if check.ExpectMatches != "" {
			matched, _ := regexp.MatchString(check.ExpectMatches, msg)
			if matched {
				return AssertionResult{
					Type:     "websocket",
					Expected: fmt.Sprintf("matches %q", check.ExpectMatches),
					Actual:   msg,
					Passed:   true,
				}
			}
			return AssertionResult{
				Type:     "websocket",
				Expected: fmt.Sprintf("matches %q", check.ExpectMatches),
				Actual:   fmt.Sprintf("received %q did not match", msg),
				Passed:   false,
			}
		}
	}
}

// CheckWebSocketWithTrace is like CheckWebSocket but injects a traceparent header.
func CheckWebSocketWithTrace(check *schema.WebSocketCheck, span *SpanContext) AssertionResult {
	clone := *check
	clone.Headers = make(map[string]string, len(check.Headers)+1)
	for k, v := range check.Headers {
		clone.Headers[k] = v
	}
	clone.Headers["traceparent"] = span.Traceparent()
	return CheckWebSocket(&clone)
}
