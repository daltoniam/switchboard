package pganalyze

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	mcp "github.com/daltoniam/switchboard"
)

// proxyClient manages communication with the pganalyze MCP server over HTTP.
type proxyClient struct {
	mu        sync.Mutex
	mcpURL    string
	apiKey    string
	client    *http.Client
	tools     []proxyToolDef
	sessionID string
	nextID    atomic.Int64
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

type jsonRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id"`
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

	if err := p.notify(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}
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
	body, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	resp, respBody, err := p.post(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}

	rpcResp, err := decodeResponse(resp.Header.Get("Content-Type"), respBody, id)
	if err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// notify sends a JSON-RPC notification, which carries no id and gets no reply.
func (p *proxyClient) notify(ctx context.Context, method string, params any) error {
	body, err := json.Marshal(jsonRPCNotification{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	_, _, err = p.post(ctx, body)
	return err
}

// post performs one Streamable HTTP exchange: it advertises both response
// encodings, echoes the server-assigned session id, and maps HTTP failures.
func (p *proxyClient) post(ctx context.Context, body []byte) (*http.Response, []byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.mcpURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	p.mu.Lock()
	if p.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", p.sessionID)
	}
	p.mu.Unlock()

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("pganalyze MCP error (%d): %s", resp.StatusCode, string(respBody)),
		}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("pganalyze MCP error (%d): %s", resp.StatusCode, string(respBody))
	}

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		p.mu.Lock()
		p.sessionID = sid
		p.mu.Unlock()
	}
	return resp, respBody, nil
}

// decodeResponse extracts the JSON-RPC response for id from either a plain
// JSON body or a text/event-stream body, skipping interleaved server messages.
func decodeResponse(contentType string, body []byte, id int64) (*jsonRPCResponse, error) {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "text/event-stream" {
		var rpcResp jsonRPCResponse
		if err := json.Unmarshal(body, &rpcResp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		return &rpcResp, nil
	}

	var data []string
	var found *jsonRPCResponse
	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		payload := strings.Join(data, "\n")
		data = data[:0]
		var rpcResp jsonRPCResponse
		if err := json.Unmarshal([]byte(payload), &rpcResp); err != nil {
			return fmt.Errorf("parse event-stream message: %w", err)
		}
		if rpcResp.ID != nil && *rpcResp.ID == id {
			found = &rpcResp
		}
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), len(body)+1)
	for scanner.Scan() && found == nil {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read event stream: %w", err)
	}
	if found == nil {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	if found == nil {
		return nil, fmt.Errorf("event stream ended without a response to request %d", id)
	}
	return found, nil
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
