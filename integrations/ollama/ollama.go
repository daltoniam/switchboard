package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

var (
	_ mcp.Integration                = (*ollama)(nil)
	_ mcp.FieldCompactionIntegration = (*ollama)(nil)
	_ mcp.MarkdownIntegration        = (*ollama)(nil)
	_ mcp.PlainTextCredentials       = (*ollama)(nil)
	_ mcp.OptionalCredentials        = (*ollama)(nil)
)

const defaultBaseURL = "http://localhost:11434"

type ollama struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &ollama{
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *ollama) Name() string { return "ollama" }

func (o *ollama) PlainTextKeys() []string { return []string{"base_url"} }
func (o *ollama) OptionalKeys() []string  { return []string{"api_key"} }

func (o *ollama) Configure(_ context.Context, creds mcp.Credentials) error {
	o.baseURL = creds["base_url"]
	if o.baseURL == "" {
		o.baseURL = defaultBaseURL
	}
	o.baseURL = strings.TrimRight(o.baseURL, "/")
	o.apiKey = creds["api_key"]
	return nil
}

func (o *ollama) Healthy(ctx context.Context) bool {
	if o.baseURL == "" {
		return false
	}
	_, err := o.get(ctx, "/api/version")
	return err == nil
}

func (o *ollama) Tools() []mcp.ToolDefinition {
	return tools
}

func (o *ollama) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, o, args)
}

func (o *ollama) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (o *ollama) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	fn, ok := markdownRenderers[toolName]
	if !ok {
		return "", false
	}
	return fn(data)
}

type handlerFunc func(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	// Model management
	"ollama_list_models":  listModels,
	"ollama_show_model":   showModel,
	"ollama_pull_model":   pullModel,
	"ollama_delete_model": deleteModel,
	"ollama_copy_model":   copyModel,
	"ollama_create_model": createModel,
	"ollama_list_running": listRunning,
	"ollama_get_version":  getVersion,
	// Inference
	"ollama_chat":     chat,
	"ollama_generate": generate,
	"ollama_embed":    embed,
}

// --- HTTP helpers ---

func (o *ollama) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, o.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (o *ollama) get(ctx context.Context, path string) (json.RawMessage, error) {
	return o.doRequest(ctx, "GET", path, nil)
}

func (o *ollama) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return o.doRequest(ctx, "POST", path, body)
}

func (o *ollama) del(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return o.doRequest(ctx, "DELETE", path, body)
}
