package websocket

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/protocol/httputil"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// frame opcodes
const (
	opText  = 0x1
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xA
)

// maxFrameSize limits the payload of a single WebSocket frame to prevent
// unbounded memory allocation from a malicious or misconfigured server.
const maxFrameSize = 10 << 20 // 10 MiB

// Runner executes WebSocket requests and extracts streaming metrics.
type Runner struct {
	target       string
	subproto     string
	connDeadline time.Duration
	useTLS       bool
	tlsConfig    *tls.Config
}

// NewRunner creates a WebSocket runner. TLS is auto-detected from wss:// URLs.
func NewRunner(target string, subproto string, timeout time.Duration) *Runner {
	return &Runner{
		target:       target,
		subproto:     subproto,
		connDeadline: timeout,
		useTLS:       strings.HasPrefix(target, "wss://"),
	}
}

// SetTLSConfig overrides the TLS configuration used for wss:// connections.
// If nil (default), the system certificate pool is used.
func (r *Runner) SetTLSConfig(cfg *tls.Config) {
	r.tlsConfig = cfg
}

// Run performs one WebSocket connection lifecycle.
func (r *Runner) Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error) {
	start := time.Now()

	// Dial with handshake.
	conn, err := r.dial(ctx, req)
	if err != nil {
		cat := httputil.ClassifyRequestError(err)
		return httputil.FailedResult(cat, err, time.Since(start)), err
	}
	defer conn.Close()

	// Set deadline.
	if r.connDeadline > 0 {
		conn.SetDeadline(time.Now().Add(r.connDeadline))
	}

	// Send the body as a text frame.
	if err := writeFrame(conn, opText, req.Body); err != nil {
		cat := httputil.ClassifyRequestError(err)
		result := httputil.FailedResult(cat, err, time.Since(start))
		result.StreamingAborted = true
		return result, err
	}

	result := types.RunResult{
		Success:       true,
		ErrorCategory: types.ErrorCategoryNone,
	}

	var (
		bytesRead      int64
		firstChunkTime time.Time
		lastChunkTime  time.Time
		prevChunkTime  time.Time
		itlSamples     []time.Duration
		textBuilder    strings.Builder
		sawClose       bool
		chunkCount     int
	)

	for {
		opcode, payload, err := readFrame(conn)
		if err != nil {
			if errors.Is(err, io.EOF) || isCloseError(err) {
				sawClose = true
				break
			}
			cat := httputil.ClassifyRequestError(err)
			result.Success = false
			result.ErrorCategory = cat
			result.ErrorMessage = err.Error()
			result.BytesRead = bytesRead
			result.Latency = time.Since(start)
			result.StreamingAborted = true
			return result, err
		}

		chunkCount++
		bytesRead += int64(len(payload))
		now := time.Now()
		if firstChunkTime.IsZero() {
			firstChunkTime = now
		} else {
			itlSamples = append(itlSamples, now.Sub(prevChunkTime))
		}
		prevChunkTime = now
		lastChunkTime = now

		switch opcode {
		case opText:
			textBuilder.Write(payload)
			textBuilder.WriteByte(' ')
		case opClose:
			sawClose = true
			break
		case opPing:
			// Reply with pong as required by RFC 6455 §5.5.2.
			if err := writeFrame(conn, opPong, payload); err != nil {
				cat := httputil.ClassifyRequestError(err)
				result.Success = false
				result.ErrorCategory = cat
				result.ErrorMessage = err.Error()
				result.BytesRead = bytesRead
				result.Latency = time.Since(start)
				result.StreamingAborted = true
				return result, err
			}
		}
		// opPong, continuation (0x0), binary (0x2) — silently ignored.

		if opcode == opClose {
			break
		}
	}

	result.BytesRead = bytesRead
	result.Latency = time.Since(start)
	result.ITLSamples = itlSamples
	if !firstChunkTime.IsZero() {
		result.TTFT = firstChunkTime.Sub(start)
	}
	if !lastChunkTime.IsZero() && !firstChunkTime.IsZero() {
		result.GenerationTime = lastChunkTime.Sub(firstChunkTime)
	}
	collectedText := strings.TrimSpace(textBuilder.String())
	if collectedText != "" {
		result.OutputTokens = int64(len(strings.Fields(collectedText)))
		result.TokensEstimated = true
	}
	if result.OutputTokens > 0 && result.GenerationTime > 0 {
		result.TokensPerSecond = float64(result.OutputTokens) / result.GenerationTime.Seconds()
	}
	if !sawClose {
		result.StreamingAborted = true
	}

	return result, nil
}

func (r *Runner) dial(ctx context.Context, req types.RequestSpec) (net.Conn, error) {
	// Strip ws:// / wss:// scheme. WebSocket always uses HTTP upgrade over TCP/TLS.
	target := r.target
	target = strings.TrimPrefix(target, "ws://")
	target = strings.TrimPrefix(target, "wss://")
	// Remove trailing path if present — we only need host:port for dial.
	if idx := strings.Index(target, "/"); idx >= 0 {
		target = target[:idx]
	}
	defaultPort := "80"
	if r.useTLS {
		defaultPort = "443"
	}
	if !strings.Contains(target, ":") {
		target = target + ":" + defaultPort
	}

	var raw net.Conn
	var err error
	if r.useTLS {
		d := tls.Dialer{Config: r.tlsConfig}
		raw, err = d.DialContext(ctx, "tcp", target)
	} else {
		var d net.Dialer
		raw, err = d.DialContext(ctx, "tcp", target)
	}
	if err != nil {
		return nil, err
	}

	// Build WebSocket upgrade request.
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		raw.Close()
		return nil, fmt.Errorf("websocket: generate key: %w", err)
	}
	keyStr := base64.StdEncoding.EncodeToString(key)

	host := r.target
	if strings.Contains(host, "://") {
		host = host[strings.Index(host, "://")+3:]
	}
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}

	path := "/"
	if strings.Contains(r.target, "://") {
		rest := r.target[strings.Index(r.target, "://")+3:]
		if idx := strings.Index(rest, "/"); idx >= 0 {
			path = rest[idx:]
		}
	} else if idx := strings.Index(r.target, "/"); idx >= 0 {
		path = r.target[idx:]
	}

	reqLine := fmt.Sprintf("GET %s HTTP/1.1\r\n", path)
	reqLine += fmt.Sprintf("Host: %s\r\n", host)
	reqLine += "Upgrade: websocket\r\n"
	reqLine += "Connection: Upgrade\r\n"
	reqLine += fmt.Sprintf("Sec-WebSocket-Key: %s\r\n", keyStr)
	reqLine += "Sec-WebSocket-Version: 13\r\n"
	if r.subproto != "" {
		reqLine += fmt.Sprintf("Sec-WebSocket-Protocol: %s\r\n", r.subproto)
	}
	for key, value := range req.Headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || isReservedHandshakeHeader(key) {
			continue
		}
		reqLine += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	reqLine += "\r\n"

	if _, err := raw.Write([]byte(reqLine)); err != nil {
		raw.Close()
		return nil, err
	}

	// Read HTTP response manually to avoid bufio.Reader buffering WebSocket frame data.
	respReader := bufio.NewReader(raw)
	resp, err := http.ReadResponse(respReader, nil)
	if err != nil {
		raw.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		raw.Close()
		return nil, fmt.Errorf("websocket upgrade failed: %s", resp.Status)
	}
	expectedAccept := computeAcceptKey(keyStr)
	if resp.Header.Get("Sec-WebSocket-Accept") != expectedAccept {
		raw.Close()
		return nil, fmt.Errorf("websocket: invalid accept key")
	}

	// Preserve any buffered bytes that http.ReadResponse consumed past the headers.
	return &prebufferedConn{Conn: raw, r: respReader}, nil
}

// prebufferedConn wraps a net.Conn with a bufio.Reader that may contain
// extra bytes read past the HTTP response headers.
type prebufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *prebufferedConn) Read(p []byte) (int, error) {
	if c.r.Buffered() > 0 {
		return c.r.Read(p)
	}
	return c.Conn.Read(p)
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func isReservedHandshakeHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host", "upgrade", "connection", "sec-websocket-key", "sec-websocket-version", "sec-websocket-protocol":
		return true
	default:
		return false
	}
}

func writeFrame(conn net.Conn, opcode byte, payload []byte) error {
	// FIN(1) + RSV(0) + Opcode(4) = 0x80 | opcode for FIN=1.
	header := []byte{0x80 | opcode}

	length := len(payload)
	if length <= 125 {
		header = append(header, 0x80|byte(length)) // MASK=1
	} else if length <= 65535 {
		header = append(header, 0x80|126) // MASK=1
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(length))
		header = append(header, ext...)
	} else {
		header = append(header, 0x80|127) // MASK=1
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(length))
		header = append(header, ext...)
	}

	// Masking key (4 random bytes).
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header = append(header, mask...)

	// Masked payload.
	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ mask[i%4]
	}

	frame := append(header, masked...)
	_, err := conn.Write(frame)
	return err
}

func readFrame(conn net.Conn) (opcode byte, payload []byte, err error) {
	// Read first 2 bytes.
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, nil, err
	}

	opcode = header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	length := uint64(header[1] & 0x7F)

	// Extended payload length.
	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext)
	}

	if length > maxFrameSize {
		return 0, nil, fmt.Errorf("websocket: frame payload too large (%d > %d)", length, maxFrameSize)
	}

	// Masking key (if present).
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(conn, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	// Payload.
	payload = make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, nil, err
	}

	// Unmask.
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}

func isCloseError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "close") || strings.Contains(msg, "closed")
}
