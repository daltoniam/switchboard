package wasm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/tetratelabs/wazero/api"
	"golang.org/x/net/http2"
)

type httpRequest struct {
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	BodyBase64 string            `json:"body_base64,omitempty"`
}

type httpResponse struct {
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
	BodyBase64 string            `json:"body_base64,omitempty"`
}

const maxHostResponseSize = 10 * 1024 * 1024

// h2cHeaderKey is the header plugins set to request HTTP/2 cleartext (h2c)
// transport. Any non-empty value enables h2c for that request.
const h2cHeaderKey = "X-H2C"

// rawBodyHeaderKey is the header plugins set to request the response body
// as raw base64-encoded bytes in the body_base64 field instead of a UTF-8
// string in the body field. Required for binary payloads (protobuf, images, etc.).
const rawBodyHeaderKey = "X-Raw-Body"

var (
	hostHTTPClient = &http.Client{Timeout: 30 * time.Second}

	// h2cClient uses an http2.Transport configured for cleartext (no TLS)
	// connections. Plugins opt in by setting the X-H2C header.
	h2cClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
)

func hostHTTPRequest(ctx context.Context, mod api.Module, ptrSize uint64) uint64 {
	ptr, size := unpackPtrSize(ptrSize)

	reqData, err := readFromGuest(mod, ptr, size)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("read request: %v", err))
	}

	var req httpRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("parse request: %v", err))
	}

	result, err := doHostHTTP(ctx, &req)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("http error: %v", err))
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("marshal response: %v", err))
	}

	rPtr, rSize, err := writeToGuest(ctx, mod, resultData)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("write response: %v", err))
	}
	return packPtrSize(rPtr, rSize)
}

// doHostHTTP executes an HTTP request on behalf of a WASM guest.
// If the request includes an X-H2C header, the h2c (HTTP/2 cleartext) client
// is used instead of the default HTTP/1.1 client.
func doHostHTTP(ctx context.Context, req *httpRequest) (*httpResponse, error) {
	var bodyReader io.Reader
	if req.BodyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.BodyBase64)
		if err != nil {
			return nil, fmt.Errorf("decode body_base64: %w", err)
		}
		bodyReader = bytes.NewReader(decoded)
	} else if req.Body != "" {
		bodyReader = bytes.NewBufferString(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Check for control headers before copying to the outgoing request.
	useH2C := req.Headers[h2cHeaderKey] != ""
	rawBody := req.Headers[rawBodyHeaderKey] != ""
	for k, v := range req.Headers {
		if k == h2cHeaderKey || k == rawBodyHeaderKey {
			continue
		}
		httpReq.Header.Set(k, v)
	}

	client := hostHTTPClient
	if useH2C {
		client = h2cClient
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHostResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	result := &httpResponse{
		Status:  resp.StatusCode,
		Headers: headers,
	}
	if rawBody {
		result.BodyBase64 = base64.StdEncoding.EncodeToString(body)
	} else {
		result.Body = string(body)
	}
	return result, nil
}

func writeErrorResponse(ctx context.Context, mod api.Module, msg string) uint64 {
	result := httpResponse{
		Status: 0,
		Body:   msg,
	}
	data, _ := json.Marshal(result)
	rPtr, rSize, err := writeToGuest(ctx, mod, data)
	if err != nil {
		slog.Error("wasm: failed to write error response to guest", "err", err)
		return 0
	}
	return packPtrSize(rPtr, rSize)
}

func hostLog(_ context.Context, mod api.Module, ptr, size uint32) {
	data, err := readFromGuest(mod, ptr, size)
	if err != nil {
		slog.Warn("wasm: host_log read failed", "err", err)
		return
	}
	slog.Info("wasm guest", "msg", string(data))
}
