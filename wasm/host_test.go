package wasm

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// newH2CServer starts an HTTP/2 cleartext server on a random port.
// Returns the base URL (http://127.0.0.1:<port>) and a cleanup function.
func newH2CServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	h2s := &http2.Server{}
	srv := &http.Server{Handler: h2c.NewHandler(handler, h2s)}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { srv.Close() })

	return fmt.Sprintf("http://%s", ln.Addr().String())
}

func TestDoHostHTTP_H2C(t *testing.T) {
	var gotProto string
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProto = r.Proto
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	resp, err := doHostHTTP(context.Background(), &httpRequest{
		Method:  "GET",
		URL:     base + "/test",
		Headers: map[string]string{h2cHeaderKey: "1"},
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if resp.Body != `{"ok":true}` {
		t.Errorf("body = %q", resp.Body)
	}
	if gotProto != "HTTP/2.0" {
		t.Errorf("server saw proto = %q, want HTTP/2.0", gotProto)
	}
}

func TestDoHostHTTP_H2C_HeaderNotForwarded(t *testing.T) {
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get(h2cHeaderKey); v != "" {
			t.Errorf("X-H2C header leaked to upstream: %q", v)
		}
		_, _ = w.Write([]byte("ok"))
	}))

	_, err := doHostHTTP(context.Background(), &httpRequest{
		Method:  "GET",
		URL:     base + "/test",
		Headers: map[string]string{h2cHeaderKey: "1", "Authorization": "Bearer tok"},
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
}

func TestDoHostHTTP_HTTP1_Default(t *testing.T) {
	var gotProto string
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProto = r.Proto
		_, _ = w.Write([]byte("ok"))
	}))

	resp, err := doHostHTTP(context.Background(), &httpRequest{
		Method: "GET",
		URL:    base + "/test",
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	// Without the h2c header, the default client uses HTTP/1.1.
	if gotProto != "HTTP/1.1" {
		t.Errorf("server saw proto = %q, want HTTP/1.1", gotProto)
	}
}

func TestDoHostHTTP_H2C_POST(t *testing.T) {
	var gotProto, gotBody string
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProto = r.Proto
		b := make([]byte, 1024)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
		_, _ = w.Write([]byte(`{"received":true}`))
	}))

	resp, err := doHostHTTP(context.Background(), &httpRequest{
		Method:  "POST",
		URL:     base + "/rpc",
		Headers: map[string]string{h2cHeaderKey: "1", "Content-Type": "application/json"},
		Body:    `{"method":"ping"}`,
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if gotProto != "HTTP/2.0" {
		t.Errorf("server saw proto = %q, want HTTP/2.0", gotProto)
	}
	if gotBody != `{"method":"ping"}` {
		t.Errorf("server got body = %q", gotBody)
	}
}

func TestDoHostHTTP_RawBody_Response(t *testing.T) {
	binaryPayload := []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x80, 0x90}
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(binaryPayload)
	}))

	resp, err := doHostHTTP(context.Background(), &httpRequest{
		Method:  "GET",
		URL:     base + "/binary",
		Headers: map[string]string{rawBodyHeaderKey: "1"},
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if resp.Body != "" {
		t.Errorf("body should be empty when raw body requested, got %q", resp.Body)
	}
	if resp.BodyBase64 == "" {
		t.Fatal("body_base64 should be populated")
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.BodyBase64)
	if err != nil {
		t.Fatalf("decode body_base64: %v", err)
	}
	if len(decoded) != len(binaryPayload) {
		t.Fatalf("decoded len = %d, want %d", len(decoded), len(binaryPayload))
	}
	for i, b := range decoded {
		if b != binaryPayload[i] {
			t.Errorf("byte %d: got %#x, want %#x", i, b, binaryPayload[i])
		}
	}
}

func TestDoHostHTTP_RawBody_HeaderNotForwarded(t *testing.T) {
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get(rawBodyHeaderKey); v != "" {
			t.Errorf("X-Raw-Body header leaked to upstream: %q", v)
		}
		_, _ = w.Write([]byte("ok"))
	}))

	_, err := doHostHTTP(context.Background(), &httpRequest{
		Method:  "GET",
		URL:     base + "/test",
		Headers: map[string]string{rawBodyHeaderKey: "1"},
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
}

func TestDoHostHTTP_RawBody_NotRequestedReturnsString(t *testing.T) {
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello text"))
	}))

	resp, err := doHostHTTP(context.Background(), &httpRequest{
		Method: "GET",
		URL:    base + "/text",
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
	if resp.Body != "hello text" {
		t.Errorf("body = %q, want %q", resp.Body, "hello text")
	}
	if resp.BodyBase64 != "" {
		t.Errorf("body_base64 should be empty when raw body not requested, got %q", resp.BodyBase64)
	}
}

func TestDoHostHTTP_BodyBase64_Request(t *testing.T) {
	binaryPayload := []byte{0x00, 0x01, 0x02, 0xff}
	var gotBody []byte
	base := newH2CServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte("ok"))
	}))

	resp, err := doHostHTTP(context.Background(), &httpRequest{
		Method:     "POST",
		URL:        base + "/upload",
		BodyBase64: base64.StdEncoding.EncodeToString(binaryPayload),
	})
	if err != nil {
		t.Fatalf("doHostHTTP: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if len(gotBody) != len(binaryPayload) {
		t.Fatalf("server got %d bytes, want %d", len(gotBody), len(binaryPayload))
	}
	for i, b := range gotBody {
		if b != binaryPayload[i] {
			t.Errorf("byte %d: got %#x, want %#x", i, b, binaryPayload[i])
		}
	}
}

func TestDoHostHTTP_BodyBase64_InvalidEncoding(t *testing.T) {
	_, err := doHostHTTP(context.Background(), &httpRequest{
		Method:     "POST",
		URL:        "http://localhost:1/unused",
		BodyBase64: "not-valid-base64!!!",
	})
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}
