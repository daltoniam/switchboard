package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

const maxResponseSize = 10 << 20 // 10 MB

const notionVersion = "2025-09-03"

type notion struct {
	integrationSecret string
	baseURL           string
	client            *http.Client
}

func New() mcp.Integration {
	return &notion{
		client:  &http.Client{},
		baseURL: "https://api.notion.com",
	}
}

func (n *notion) Name() string { return "notion" }

func (n *notion) Configure(creds mcp.Credentials) error {
	n.integrationSecret = creds["integration_secret"]
	if n.integrationSecret == "" {
		return fmt.Errorf("notion: integration_secret is required")
	}
	if v := creds["base_url"]; v != "" {
		n.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (n *notion) Healthy(ctx context.Context) bool {
	if n.client == nil || n.integrationSecret == "" {
		return false
	}
	_, err := n.get(ctx, "/v1/users/me")
	return err == nil
}

func (n *notion) Tools() []mcp.ToolDefinition {
	return tools
}

func (n *notion) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, n, args)
}

// --- HTTP helpers ---

func (n *notion) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, n.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+n.integrationSecret)
	req.Header.Set("Notion-Version", notionVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("notion API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (n *notion) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	escaped := make([]any, len(args))
	for i, a := range args {
		if s, ok := a.(string); ok {
			escaped[i] = url.PathEscape(s)
		} else {
			escaped[i] = a
		}
	}
	return n.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, escaped...), nil)
}

func (n *notion) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return n.doRequest(ctx, "POST", path, body)
}

func (n *notion) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return n.doRequest(ctx, "PATCH", path, body)
}

func (n *notion) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	escaped := make([]any, len(args))
	for i, a := range args {
		if s, ok := a.(string); ok {
			escaped[i] = url.PathEscape(s)
		} else {
			escaped[i] = a
		}
	}
	return n.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, escaped...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error)

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

func argMap(args map[string]any, key string) map[string]any {
	v, _ := args[key].(map[string]any)
	return v
}
