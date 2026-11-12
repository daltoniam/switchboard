package groupme

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("groupme", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const (
	defaultBaseURL  = "https://api.groupme.com/v3"
	maxResponseSize = 10 * 1024 * 1024 // 10 MB

	// statusEnhanceYourCalm is GroupMe's historical rate-limit status.
	statusEnhanceYourCalm = 420
)

type groupme struct {
	token   string
	baseURL string
	client  *http.Client
}

var (
	_ mcp.Integration                = (*groupme)(nil)
	_ mcp.FieldCompactionIntegration = (*groupme)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*groupme)(nil)
	_ mcp.PlainTextCredentials       = (*groupme)(nil)
	_ mcp.OptionalCredentials        = (*groupme)(nil)
	_ mcp.PlaceholderHints           = (*groupme)(nil)
)

func New() mcp.Integration { return &groupme{} }

func (*groupme) Name() string            { return "groupme" }
func (*groupme) PlainTextKeys() []string { return []string{"base_url"} }
func (*groupme) OptionalKeys() []string  { return []string{"base_url"} }
func (*groupme) Placeholders() map[string]string {
	return map[string]string{
		"access_token": "Access token from dev.groupme.com (top right, Access Token)",
		"base_url":     defaultBaseURL,
	}
}

func (g *groupme) Configure(_ context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["access_token"])
	if token == "" {
		return fmt.Errorf("groupme: access_token is required")
	}
	g.token = token
	g.baseURL = defaultBaseURL
	if v := strings.TrimSpace(creds["base_url"]); v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	g.client = &http.Client{
		Timeout: 30 * time.Second,
		// The token travels in a custom header that net/http would forward on
		// a cross-host redirect, so redirects are never followed.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return nil
}

func (g *groupme) Healthy(ctx context.Context) bool {
	if g.client == nil {
		return false
	}
	_, err := g.doRequest(ctx, http.MethodGet, "/users/me", nil, nil)
	return err == nil
}

func (*groupme) Tools() []mcp.ToolDefinition { return tools }

func (g *groupme) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("groupme: unknown tool: %s", toolName))
	}
	if g.client == nil {
		return mcp.ErrResult(fmt.Errorf("groupme: integration is not configured"))
	}
	return fn(ctx, g, args)
}

func (*groupme) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (*groupme) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// doRequest calls the GroupMe API and returns the payload inside the
// {"response": ..., "meta": ...} envelope. It returns nil data, without an
// error, for 304 Not Modified (no messages match the cursor) and for
// successful responses with no payload (bot posts).
func (g *groupme) doRequest(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	target := g.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Access-Token", g.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		apiErr := fmt.Errorf("groupme API error (%d): %s", resp.StatusCode, errorDetail(data))
		if retryable(method, resp.StatusCode) {
			return nil, &mcp.RetryableError{StatusCode: resp.StatusCode, Err: apiErr, RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))}
		}
		return nil, apiErr
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var envelope struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("groupme: invalid JSON response: %w", err)
	}
	if len(envelope.Response) == 0 || string(envelope.Response) == "null" {
		return nil, nil
	}
	return envelope.Response, nil
}

// retryable reports whether the server may safely call the tool again. Rate
// limits mean the request was not processed. A server error on a send may
// still have delivered the message, so only reads retry on 5xx.
func retryable(method string, status int) bool {
	if status == http.StatusTooManyRequests || status == statusEnhanceYourCalm {
		return true
	}
	return method == http.MethodGet && status >= 500
}

// errorDetail extracts meta.errors from a GroupMe error body, falling back
// to the raw body.
func errorDetail(data []byte) string {
	var envelope struct {
		Meta struct {
			Errors []string `json:"errors"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Meta.Errors) > 0 {
		return strings.Join(envelope.Meta.Errors, "; ")
	}
	if detail := strings.TrimSpace(string(data)); detail != "" {
		return detail
	}
	return "no error detail"
}
