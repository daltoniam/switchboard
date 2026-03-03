package metabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

type metabase struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func New() mcp.Integration {
	return &metabase{
		client: &http.Client{},
	}
}

func (m *metabase) Name() string { return "metabase" }

func (m *metabase) Configure(creds mcp.Credentials) error {
	m.apiKey = creds["api_key"]
	m.baseURL = strings.TrimRight(creds["url"], "/")
	if m.apiKey == "" {
		return fmt.Errorf("metabase: api_key is required")
	}
	if m.baseURL == "" {
		return fmt.Errorf("metabase: url is required")
	}
	return nil
}

func (m *metabase) Healthy(ctx context.Context) bool {
	_, err := m.get(ctx, "/api/user/current")
	return err == nil
}

func (m *metabase) Tools() []mcp.ToolDefinition {
	return tools
}

func (m *metabase) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, m, args)
}

// --- HTTP helpers ---

func (m *metabase) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, m.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", m.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("metabase API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (m *metabase) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return m.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (m *metabase) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return m.doRequest(ctx, "POST", path, body)
}

func (m *metabase) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return m.doRequest(ctx, "PUT", path, body)
}

func (m *metabase) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return m.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}
