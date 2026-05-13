package elasticsearch

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

var compactResult = compact.MustLoadWithOverlay("elasticsearch", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.Integration                = (*esInt)(nil)
	_ mcp.FieldCompactionIntegration = (*esInt)(nil)
	_ mcp.PlainTextCredentials       = (*esInt)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*esInt)(nil)
)

type esInt struct {
	client  *http.Client
	baseURL string
	apiKey  string
	user    string
	pass    string
}

func New() mcp.Integration {
	return &esInt{}
}

func (e *esInt) Name() string { return "elasticsearch" }

func (e *esInt) Configure(_ context.Context, creds mcp.Credentials) error {
	base := creds["base_url"]
	if base == "" {
		base = "http://localhost:9200"
	}
	e.baseURL = strings.TrimRight(base, "/")
	e.apiKey = creds["api_key"]
	e.user = creds["username"]
	e.pass = creds["password"]
	e.client = &http.Client{Timeout: 30 * time.Second}
	return nil
}

func (e *esInt) Healthy(ctx context.Context) bool {
	if e.client == nil {
		return false
	}
	resp, err := e.do(ctx, http.MethodGet, "/_cluster/health", "", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close() //nolint:errcheck
	return resp.StatusCode == http.StatusOK
}

func (e *esInt) Tools() []mcp.ToolDefinition {
	return tools
}

func (e *esInt) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (e *esInt) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (e *esInt) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if e.client == nil {
		return &mcp.ToolResult{Data: "elasticsearch: not configured", IsError: true}, nil
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, e, args)
}

func (e *esInt) PlainTextKeys() []string {
	return []string{"base_url", "username"}
}

type handlerFunc func(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error)

const maxResponseBytes = 2 * 1024 * 1024

func (e *esInt) do(ctx context.Context, method, path, contentType string, body io.Reader) (*http.Response, error) {
	u := e.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	if e.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+e.apiKey)
	} else if e.user != "" {
		req.SetBasicAuth(e.user, e.pass)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 500 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close() //nolint:errcheck
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("elasticsearch %d: %s", resp.StatusCode, string(respBody)),
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}

	return resp, nil
}

func (e *esInt) doJSON(ctx context.Context, method, path string, body io.Reader) (json.RawMessage, error) {
	resp, err := e.do(ctx, method, path, "", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("elasticsearch %d: %s", resp.StatusCode, string(data))
	}

	return json.RawMessage(data), nil
}

func (e *esInt) doNDJSON(ctx context.Context, path string, body io.Reader) (json.RawMessage, error) {
	resp, err := e.do(ctx, http.MethodPost, path, "application/x-ndjson", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("elasticsearch %d: %s", resp.StatusCode, string(data))
	}

	return json.RawMessage(data), nil
}

func jsonBody(v any) (io.Reader, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func pathEscape(s string) string {
	return url.PathEscape(s)
}
