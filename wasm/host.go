package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/tetratelabs/wazero/api"
)

type httpRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type httpResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

const maxHostResponseSize = 10 * 1024 * 1024

var hostHTTPClient = &http.Client{Timeout: 30 * time.Second}

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

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = bytes.NewBufferString(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("create request: %v", err))
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := hostHTTPClient.Do(httpReq)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("http error: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHostResponseSize))
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("read response: %v", err))
	}

	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	result := httpResponse{
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    string(body),
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
