package websocket

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// startWSServer starts a real HTTP server on a random port that upgrades to WebSocket.
func startWSServer(t *testing.T, handler func(net.Conn)) string {
	return startWSServerWithRequest(t, func(_ *http.Request, conn net.Conn) {
		handler(conn)
	})
}

func startWSServerWithRequest(t *testing.T, handler func(*http.Request, net.Conn)) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	s := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Sec-WebSocket-Key")
			if key == "" {
				http.Error(w, "missing key", http.StatusBadRequest)
				return
			}
			accept := computeAcceptKey(key)

			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack not supported", http.StatusInternalServerError)
				return
			}
			conn, bufrw, err := hj.Hijack()
			if err != nil {
				return
			}

			resp := "HTTP/1.1 101 Switching Protocols\r\n"
			resp += "Upgrade: websocket\r\n"
			resp += "Connection: Upgrade\r\n"
			resp += "Sec-WebSocket-Accept: " + accept + "\r\n"
			resp += "\r\n"
			bufrw.WriteString(resp)
			bufrw.Flush()

			handler(r, conn)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go s.Serve(lis)
	t.Cleanup(func() { s.Close(); lis.Close() })

	return lis.Addr().String()
}

func TestWSRunnerEcho(t *testing.T) {
	t.Parallel()

	addr := startWSServer(t, func(conn net.Conn) {
		defer conn.Close()
		opcode, payload, err := readFrame(conn)
		if err != nil || opcode != opText {
			return
		}
		// Echo back and close.
		writeFrame(conn, opText, payload)
		writeFrame(conn, opClose, nil)
	})

	runner := NewRunner(addr, "", 3*time.Second)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		Body: []byte(`{"prompt":"hello"}`),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.ErrorMessage)
	}
	if result.TTFT == 0 {
		t.Fatal("expected non-zero TTFT")
	}
	if result.OutputTokens == 0 {
		t.Fatal("expected non-zero token count from echoed text")
	}
}

func TestWSRunnerTTFT(t *testing.T) {
	t.Parallel()

	addr := startWSServer(t, func(conn net.Conn) {
		defer conn.Close()
		opcode, _, err := readFrame(conn)
		if err != nil || opcode != opText {
			return
		}
		time.Sleep(20 * time.Millisecond)
		for _, word := range []string{"hello", "world", "test"} {
			writeFrame(conn, opText, []byte(word))
			time.Sleep(5 * time.Millisecond)
		}
		writeFrame(conn, opClose, nil)
	})

	runner := NewRunner(addr, "", 3*time.Second)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		Body: []byte(`test`),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.ErrorMessage)
	}
	if result.TTFT < 20*time.Millisecond {
		t.Fatalf("TTFT too small: %s (expected >= 20ms)", result.TTFT)
	}
	if result.StreamingAborted {
		t.Fatal("expected clean close, not aborted")
	}
	if len(result.ITLSamples) < 2 {
		t.Fatalf("expected at least 2 ITL samples, got %d", len(result.ITLSamples))
	}
}

func TestWSRunnerConnectionRefused(t *testing.T) {
	t.Parallel()
	runner := NewRunner("127.0.0.1:19999", "", time.Second)
	_, err := runner.Run(context.Background(), types.RequestSpec{Body: []byte(`test`)})
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestWSRunnerComputeAcceptKey(t *testing.T) {
	key := "dGhlIHNhbXBsZSBub25jZQ=="
	expected := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got := computeAcceptKey(key); got != expected {
		t.Fatalf("computeAcceptKey = %q, want %q", got, expected)
	}
}

func TestWSRunnerReadWriteFrame(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	payload := []byte("hello websocket")
	go func() {
		writeFrame(serverConn, opText, payload)
		writeFrame(serverConn, opClose, nil)
	}()

	opcode, got, err := readFrame(clientConn)
	if err != nil {
		t.Fatalf("readFrame returned error: %v", err)
	}
	if opcode != opText {
		t.Fatalf("expected opText, got %d", opcode)
	}
	if string(got) != string(payload) {
		t.Fatalf("expected %q, got %q", payload, got)
	}
}

func TestWSRunnerSubprotocol(t *testing.T) {
	t.Parallel()

	addr := startWSServer(t, func(conn net.Conn) {
		defer conn.Close()
		readFrame(conn)
		writeFrame(conn, opText, []byte("ok"))
		writeFrame(conn, opClose, nil)
	})

	runner := NewRunner(addr, "graphql-ws", 3*time.Second)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		Body: []byte(`test`),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.ErrorMessage)
	}
}

func TestWSRunnerCustomHeaders(t *testing.T) {
	t.Parallel()

	addr := startWSServerWithRequest(t, func(req *http.Request, conn net.Conn) {
		defer conn.Close()
		if req.Header.Get("Authorization") != "Bearer test-token" {
			return
		}
		readFrame(conn)
		writeFrame(conn, opText, []byte("ok"))
		writeFrame(conn, opClose, nil)
	})

	runner := NewRunner(addr, "", 3*time.Second)
	result, err := runner.Run(context.Background(), types.RequestSpec{
		Headers: map[string]string{"Authorization": "Bearer test-token"},
		Body:    []byte(`test`),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.ErrorMessage)
	}
}

func selfSignedCert() tls.Certificate {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost", "127.0.0.1"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func startWSServerTLS(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	cert := selfSignedCert()
	tlsLis := tls.NewListener(lis, &tls.Config{Certificates: []tls.Certificate{cert}})

	s := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Sec-WebSocket-Key")
			if key == "" {
				http.Error(w, "missing key", http.StatusBadRequest)
				return
			}
			accept := computeAcceptKey(key)

			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack not supported", http.StatusInternalServerError)
				return
			}
			conn, bufrw, err := hj.Hijack()
			if err != nil {
				return
			}

			resp := "HTTP/1.1 101 Switching Protocols\r\n"
			resp += "Upgrade: websocket\r\n"
			resp += "Connection: Upgrade\r\n"
			resp += "Sec-WebSocket-Accept: " + accept + "\r\n"
			resp += "\r\n"
			bufrw.WriteString(resp)
			bufrw.Flush()

			handler(conn)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go s.Serve(tlsLis)
	t.Cleanup(func() { s.Close(); tlsLis.Close(); lis.Close() })

	return lis.Addr().String()
}

func TestWSRunnerTLS(t *testing.T) {
	t.Parallel()

	addr := startWSServerTLS(t, func(conn net.Conn) {
		defer conn.Close()
		opcode, payload, err := readFrame(conn)
		if err != nil || opcode != opText {
			return
		}
		writeFrame(conn, opText, payload)
		writeFrame(conn, opClose, nil)
	})

	runner := NewRunner("wss://"+addr, "", 3*time.Second)
	// Skip verification for self-signed test certificate.
	runner.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})

	result, err := runner.Run(context.Background(), types.RequestSpec{
		Body: []byte(`{"prompt":"tls test"}`),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.ErrorMessage)
	}
	if result.TTFT == 0 {
		t.Fatal("expected non-zero TTFT over wss://")
	}
}

func TestWSRunnerTLSDetection(t *testing.T) {
	// Verify TLS is auto-detected from URL scheme.
	plain := NewRunner("ws://example.com/ws", "", time.Second)
	if plain.useTLS {
		t.Fatal("ws:// should not enable TLS")
	}

	secure := NewRunner("wss://example.com/ws", "", time.Second)
	if !secure.useTLS {
		t.Fatal("wss:// should auto-enable TLS")
	}
}
