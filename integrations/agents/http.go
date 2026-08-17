package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// httpClient handles HTTP/1.1 JSON requests to the A2A proxy endpoints.
type httpClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newHTTPClient(baseURL, token string) *httpClient {
	return &httpClient{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// get performs an HTTP GET and returns the response body as raw JSON.
func (h *httpClient) get(ctx context.Context, path string) (json.RawMessage, error) {
	reqURL := h.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	h.setHeaders(req)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("A2A GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("A2A error (HTTP %d): %s", resp.StatusCode, truncate(string(body), 500))
	}

	// If the response body is empty, return an informative error rather than
	// silently returning empty JSON. This surfaces daemon-side issues clearly.
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("A2A endpoint %s returned empty response (HTTP %d) — the ARP daemon may not be serving A2A endpoints on %s", path, resp.StatusCode, h.baseURL)
	}

	return json.RawMessage(body), nil
}

// post performs an HTTP POST with a JSON body and returns the response.
func (h *httpClient) post(ctx context.Context, path string, payload any) (json.RawMessage, error) {
	reqURL := h.baseURL + path

	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	h.setHeaders(req)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("A2A POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("A2A error (HTTP %d): %s", resp.StatusCode, truncate(string(body), 500))
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("A2A endpoint %s returned empty response (HTTP %d) — the ARP daemon may not be serving A2A endpoints on %s", path, resp.StatusCode, h.baseURL)
	}

	return json.RawMessage(body), nil
}

func (h *httpClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
}

// encodePath percent-encodes a path segment for use in URLs.
func encodePath(s string) string {
	return url.PathEscape(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
