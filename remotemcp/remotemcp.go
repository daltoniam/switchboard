package remotemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultTimeout = 30 * time.Second
)

type remote struct {
	name      string
	serverURL string

	mu           sync.RWMutex
	token        string
	session      *mcpsdk.ClientSession
	client       *mcpsdk.Client
	cachedTools  []mcp.ToolDefinition
	toolsFetched bool
}

// New creates a remote MCP integration that proxies to the given server URL.
func New(name, serverURL string) mcp.Integration {
	return &remote{
		name:      name,
		serverURL: serverURL,
	}
}

func (r *remote) Name() string { return r.name }

func (r *remote) Configure(_ context.Context, creds mcp.Credentials) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	token := creds["access_token"]
	if token == "" {
		return fmt.Errorf("%s: access_token is required", r.name)
	}

	if token != r.token {
		if r.session != nil {
			_ = r.session.Close()
			r.session = nil
		}
		r.token = token
		r.toolsFetched = false
		r.cachedTools = nil
	}
	return nil
}

// bearerTransport injects an Authorization header into every request.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

func (r *remote) connect(ctx context.Context) (*mcpsdk.ClientSession, error) {
	r.mu.RLock()
	if r.session != nil {
		sess := r.session
		r.mu.RUnlock()
		return sess, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.session != nil {
		return r.session, nil
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "switchboard",
		Version: "0.2.0",
	}, nil)

	transport := &mcpsdk.StreamableClientTransport{
		Endpoint: r.serverURL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerTransport{token: r.token},
			Timeout:   defaultTimeout,
		},
		DisableStandaloneSSE: true,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", r.serverURL, err)
	}

	r.client = client
	r.session = session
	return session, nil
}

func (r *remote) disconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session != nil {
		_ = r.session.Close()
		r.session = nil
	}
}

func (r *remote) Healthy(ctx context.Context) bool {
	session, err := r.connect(ctx)
	if err != nil {
		return false
	}
	_, err = session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		r.disconnect()
		return false
	}
	return true
}

func (r *remote) Tools() []mcp.ToolDefinition {
	r.mu.RLock()
	if r.toolsFetched {
		tools := r.cachedTools
		r.mu.RUnlock()
		return tools
	}
	r.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	session, err := r.connect(ctx)
	if err != nil {
		return nil
	}

	result, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		return nil
	}

	tools := convertTools(r.name, result.Tools)

	r.mu.Lock()
	r.cachedTools = tools
	r.toolsFetched = true
	r.mu.Unlock()

	return tools
}

func (r *remote) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	session, err := r.connect(ctx)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	remoteName := strings.TrimPrefix(toolName, r.name+"_")

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      remoteName,
		Arguments: args,
	})
	if err != nil {
		r.disconnect()
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	return convertResult(result), nil
}

func convertTools(prefix string, tools []*mcpsdk.Tool) []mcp.ToolDefinition {
	var defs []mcp.ToolDefinition
	for _, t := range tools {
		params := extractParams(t.InputSchema)
		required := extractRequired(t.InputSchema)

		defs = append(defs, mcp.ToolDefinition{
			Name:        prefix + "_" + t.Name,
			Description: t.Description,
			Parameters:  params,
			Required:    required,
		})
	}
	return defs
}

func extractParams(schema any) map[string]string {
	params := make(map[string]string)
	schemaMap, ok := toMap(schema)
	if !ok {
		return params
	}
	props, ok := toMap(schemaMap["properties"])
	if !ok {
		return params
	}
	for k, v := range props {
		desc := ""
		if vMap, ok := toMap(v); ok {
			if d, ok := vMap["description"].(string); ok {
				desc = d
			}
		}
		params[k] = desc
	}
	return params
}

func extractRequired(schema any) []string {
	schemaMap, ok := toMap(schema)
	if !ok {
		return nil
	}
	req, ok := schemaMap["required"]
	if !ok {
		return nil
	}
	reqArr, ok := req.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, v := range reqArr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func toMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

func convertResult(result *mcpsdk.CallToolResult) *mcp.ToolResult {
	if result == nil {
		return &mcp.ToolResult{Data: "no result", IsError: true}
	}

	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, tc.Text)
		} else {
			data, err := json.Marshal(c)
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}

	return &mcp.ToolResult{
		Data:    strings.Join(parts, "\n"),
		IsError: result.IsError,
	}
}
