package pganalyze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	mcp "github.com/daltoniam/switchboard"
)

// proxyClient manages communication with the pganalyze MCP server over HTTP.
type proxyClient struct {
	mu     sync.Mutex
	mcpURL string
	apiKey string
	client *http.Client
	tools  []proxyToolDef
	nextID atomic.Int64
}

type proxyToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newProxyClient(mcpURL, apiKey string, client *http.Client) *proxyClient {
	return &proxyClient{
		mcpURL: mcpURL,
		apiKey: apiKey,
		client: client,
	}
}

func (p *proxyClient) initialize(ctx context.Context) error {
	_, err := p.send(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "switchboard-pganalyze",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// Send initialized notification (no response expected, but send as request for HTTP).
	_, _ = p.send(ctx, "notifications/initialized", map[string]any{})
	return nil
}

func (p *proxyClient) fetchTools(ctx context.Context) error {
	result, err := p.send(ctx, "tools/list", map[string]any{})
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}

	var toolsResp struct {
		Tools []proxyToolDef `json:"tools"`
	}
	if err := json.Unmarshal(result, &toolsResp); err != nil {
		return fmt.Errorf("parse tools response: %w", err)
	}
	p.mu.Lock()
	p.tools = toolsResp.Tools
	p.mu.Unlock()
	return nil
}

func (p *proxyClient) send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := p.nextID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.mcpURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("pganalyze MCP error (%d): %s", resp.StatusCode, string(respBody)),
		}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode == 202 {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pganalyze MCP error (%d): %s", resp.StatusCode, string(respBody))
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func (p *proxyClient) toolDefinitions() []mcp.ToolDefinition {
	p.mu.Lock()
	defer p.mu.Unlock()

	defs := make([]mcp.ToolDefinition, 0, len(p.tools))
	for _, t := range p.tools {
		params := make(map[string]string)
		var requiredFields []string

		if props, ok := t.InputSchema["properties"].(map[string]any); ok {
			for key, val := range props {
				desc := ""
				if propMap, ok := val.(map[string]any); ok {
					if d, ok := propMap["description"].(string); ok {
						desc = d
					}
				}
				params[key] = desc
			}
		}
		if reqList, ok := t.InputSchema["required"].([]any); ok {
			for _, r := range reqList {
				if s, ok := r.(string); ok {
					requiredFields = append(requiredFields, s)
				}
			}
		}

		name := mcp.ToolName("pganalyze_" + t.Name)
		desc := t.Description

		defs = append(defs, mcp.ToolDefinition{
			Name:        name,
			Description: desc,
			Parameters:  params,
			Required:    requiredFields,
		})
	}
	return defs
}

func (p *proxyClient) execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	originalName := strings.TrimPrefix(string(toolName), "pganalyze_")

	p.mu.Lock()
	found := false
	for _, t := range p.tools {
		if t.Name == originalName {
			found = true
			break
		}
	}
	p.mu.Unlock()

	if !found {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}

	result, err := p.send(ctx, "tools/call", map[string]any{
		"name":      originalName,
		"arguments": args,
	})
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("proxy call %s: %w", originalName, err))
	}

	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &callResult); err != nil {
		return &mcp.ToolResult{Data: string(result)}, nil
	}

	var texts []string
	for _, c := range callResult.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}

	return &mcp.ToolResult{
		Data:    strings.Join(texts, "\n"),
		IsError: callResult.IsError,
	}, nil
}
