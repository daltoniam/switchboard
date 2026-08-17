package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// notionV1 is the public-API backend (api.notion.com/v1 with OAuth bearer
// tokens). It is selected by Configure when creds["access_token"] is set.
//
// Unlike the v3 backend (which talks to www.notion.so/api/v3 via a session
// cookie), the v1 backend uses Notion's documented REST API and a versioned
// header. Pages must be explicitly shared with the integration by the user
// during the OAuth install flow (or via Add Connections later) — there is
// no implicit workspace-wide visibility.
type notionV1 struct {
	accessToken string
	baseURL     string
	apiVersion  string
	client      *http.Client
}

// defaultV1APIVersion is the date-stamped Notion API version we send in
// the Notion-Version header. 2025-09-03 introduced the data_sources model
// that matches the v3 backend's split between "database container" and
// "data source schema", so the public surface aligns naturally.
const defaultV1APIVersion = "2025-09-03"

func newV1Backend(accessToken string) *notionV1 {
	return &notionV1{
		accessToken: accessToken,
		baseURL:     "https://api.notion.com/v1",
		apiVersion:  defaultV1APIVersion,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxConnsPerHost: 10,
			},
		},
	}
}

// configure runs any one-time setup against the API. For v1 there is no
// space-resolution step (every request is scoped by the OAuth token), so
// configure just confirms the token works by hitting /users/me.
func (n *notionV1) configure(ctx context.Context) error {
	if n.accessToken == "" {
		return fmt.Errorf("notion: access_token is required")
	}
	if _, err := n.doRequest(ctx, http.MethodGet, "/users/me", nil); err != nil {
		return fmt.Errorf("notion: token check failed: %w", err)
	}
	return nil
}

func (n *notionV1) healthy(ctx context.Context) bool {
	if n.client == nil || n.accessToken == "" {
		return false
	}
	_, err := n.doRequest(ctx, http.MethodGet, "/users/me", nil)
	return err == nil
}

// doRequest sends a JSON request to api.notion.com/v1 with the OAuth bearer
// token and the required Notion-Version header. 429/5xx responses surface
// as *mcp.RetryableError so the runtime backs off; other 4xx codes return
// a formatted error.
//
// pathAndQuery may include a query string; the caller is responsible for
// url-encoding it.
func (n *notionV1) doRequest(ctx context.Context, method, pathAndQuery string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, n.baseURL+pathAndQuery, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+n.accessToken)
	req.Header.Set("Notion-Version", n.apiVersion)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        formatV1APIError(resp.StatusCode, respData),
		}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, formatV1APIError(resp.StatusCode, respData)
	}
	if resp.StatusCode == http.StatusNoContent || len(respData) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(respData), nil
}

// Verb shortcuts mirror the Sentry/Vercel pattern; pathFmt may contain
// fmt-style placeholders for path segments.
func (n *notionV1) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (n *notionV1) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodPost, path, body)
}

func (n *notionV1) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodPatch, path, body)
}

func (n *notionV1) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodDelete, fmt.Sprintf(pathFmt, args...), nil)
}

// formatV1APIError parses Notion's documented error envelope:
//
//	{ "object": "error", "status": 400, "code": "validation_error", "message": "..." }
//
// Falls back to the truncated raw body for anything that doesn't match.
func formatV1APIError(statusCode int, body []byte) error {
	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Code != "" {
		return fmt.Errorf("notion API error (%d): %s: %s", statusCode, errResp.Code, errResp.Message)
	}
	s := string(body)
	if len(s) > maxErrorLen {
		s = s[:maxErrorLen] + "... (truncated)"
	}
	return fmt.Errorf("notion API error (%d): %s", statusCode, s)
}

// v1HandlerFunc is the signature for v1 tool handlers. Mirrors handlerFunc
// (v3) but takes *notionV1 instead of *notion.
type v1HandlerFunc func(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error)

// queryEncode is a small helper for building "?a=1&b=2" suffixes without
// dragging url.Values into every handler. Empty values are skipped.
func queryEncode(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	v := url.Values{}
	for k, val := range params {
		if val == "" {
			continue
		}
		v.Set(k, val)
	}
	enc := v.Encode()
	if enc == "" {
		return ""
	}
	return "?" + enc
}
