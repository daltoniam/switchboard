// Package recoll provides access to the Recoll WebUI search endpoint.
package recoll

import (
	"context"
	_ "embed"
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

var compactResult = compact.MustLoadWithOverlay("recoll", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

var (
	_ mcp.Integration                = (*recoll)(nil)
	_ mcp.PlainTextCredentials       = (*recoll)(nil)
	_ mcp.PlaceholderHints           = (*recoll)(nil)
	_ mcp.FieldCompactionIntegration = (*recoll)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*recoll)(nil)
)

type recoll struct {
	baseURL string
	client  *http.Client
}

// New creates a Recoll WebUI integration.
func New() mcp.Integration {
	return &recoll{client: &http.Client{Timeout: 30 * time.Second}}
}

func (r *recoll) Name() string { return "recoll" }

func (r *recoll) Configure(_ context.Context, creds mcp.Credentials) error {
	r.baseURL = strings.TrimRight(creds["base_url"], "/")
	if r.baseURL == "" {
		return fmt.Errorf("recoll: base_url is required")
	}
	if _, err := url.ParseRequestURI(r.baseURL); err != nil {
		return fmt.Errorf("recoll: invalid base_url: %w", err)
	}
	if r.client == nil {
		r.client = &http.Client{Timeout: 30 * time.Second}
	}
	return nil
}

func (r *recoll) Healthy(ctx context.Context) bool {
	if r.client == nil || r.baseURL == "" {
		return false
	}
	_, err := r.get(ctx, "/")
	return err == nil
}

func (r *recoll) Tools() []mcp.ToolDefinition { return tools }

func (r *recoll) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (r *recoll) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := compactResult.MaxBytes[toolName]
	return n, ok
}

func (r *recoll) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if r.client == nil || r.baseURL == "" {
		return &mcp.ToolResult{Data: "recoll: not configured", IsError: true}, nil
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, r, args)
}

func (r *recoll) PlainTextKeys() []string { return []string{"base_url"} }

func (r *recoll) Placeholders() map[string]string {
	return map[string]string{"base_url": "http://localhost:8080"}
}

func (r *recoll) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("recoll: create request: %w", err)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("recoll: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("recoll: read response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("recoll WebUI error (%d): %s", resp.StatusCode, string(data)),
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("recoll WebUI error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return []byte(`{"status":"success"}`), nil
	}
	return data, nil
}

type handlerFunc func(context.Context, *recoll, map[string]any) (*mcp.ToolResult, error)
