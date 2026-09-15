package grist

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

var compactResult = compact.MustLoadWithOverlay("grist", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type grist struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

const (
	maxResponseSize = 10 * 1024 * 1024
	defaultBaseURL  = "https://docs.getgrist.com"
	defaultLimit    = 50
	maxLimit        = 500
)

var (
	_ mcp.Integration                = (*grist)(nil)
	_ mcp.FieldCompactionIntegration = (*grist)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*grist)(nil)
	_ mcp.PlainTextCredentials       = (*grist)(nil)
	_ mcp.PlaceholderHints           = (*grist)(nil)
	_ mcp.OptionalCredentials        = (*grist)(nil)
)

func (g *grist) PlainTextKeys() []string { return []string{"base_url"} }

func (g *grist) Placeholders() map[string]string {
	return map[string]string{
		"api_key":  "Grist API key from Profile → API",
		"base_url": "https://docs.getgrist.com, https://{team}.getgrist.com, or a self-hosted origin",
	}
}

func (g *grist) OptionalKeys() []string { return []string{"base_url"} }

func New() mcp.Integration {
	return &grist{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultBaseURL,
	}
}

func (g *grist) Name() string { return "grist" }

func (g *grist) Configure(_ context.Context, creds mcp.Credentials) error {
	g.apiKey = creds["api_key"]
	if g.apiKey == "" {
		return fmt.Errorf("grist: api_key is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
		g.baseURL = strings.TrimSuffix(g.baseURL, "/api")
	} else {
		g.baseURL = defaultBaseURL
	}
	if g.client == nil {
		g.client = &http.Client{Timeout: 30 * time.Second}
	}
	return nil
}

func (g *grist) Healthy(ctx context.Context) bool {
	if g.client == nil || g.apiKey == "" || g.baseURL == "" {
		return false
	}
	_, err := g.get(ctx, "/api/profile/user")
	return err == nil
}

func (g *grist) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *grist) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *grist) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *grist) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *grist) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
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
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("grist API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("grist API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *grist) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (g *grist) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, http.MethodPost, path, body)
}

func (g *grist) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, http.MethodPatch, path, body)
}

func (g *grist) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, http.MethodPut, path, body)
}

func (g *grist) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, http.MethodDelete, fmt.Sprintf(pathFmt, args...), nil)
}

type handlerFunc func(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error)

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

func unwrapKey(data json.RawMessage, key string) json.RawMessage {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(data, &envelope) != nil {
		return data
	}
	inner, ok := envelope[key]
	if !ok || len(inner) == 0 {
		return data
	}
	return inner
}
