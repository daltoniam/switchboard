package metabase

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("metabase", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*metabase)(nil)
	_ mcp.FieldCompactionIntegration = (*metabase)(nil)
	_ mcp.PlainTextCredentials       = (*metabase)(nil)
	_ mcp.PlaceholderHints           = (*metabase)(nil)
	_ mcp.OptionalCredentials        = (*metabase)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*metabase)(nil)
	_ io.Closer                      = (*metabase)(nil)
)

func (m *metabase) PlainTextKeys() []string {
	return []string{"url", "mcp_client_id", mcp.CredKeyTokenSource}
}

func (m *metabase) OptionalKeys() []string {
	return []string{"api_key", "mcp_access_token", "mcp_refresh_token", "mcp_client_id", mcp.CredKeyTokenSource}
}

func (m *metabase) Placeholders() map[string]string {
	return map[string]string{"url": "https://your-metabase-instance.com"}
}

type metabase struct {
	mu        sync.RWMutex
	apiKey    string
	baseURL   string
	client    *http.Client
	remote    mcp.Integration
	useRemote bool

	cfgMu     sync.Mutex
	configSvc mcp.ConfigService
}

func New() mcp.Integration {
	return &metabase{
		client: &http.Client{},
	}
}

func (m *metabase) Name() string { return "metabase" }

// Configure selects hosted-MCP (OAuth) or REST (API key) mode. token_source
// decides when both credentials are present; otherwise an OAuth token wins.
func (m *metabase) Configure(ctx context.Context, creds mcp.Credentials) error {
	baseURL := strings.TrimRight(strings.TrimSpace(creds["url"]), "/")
	if baseURL == "" {
		return fmt.Errorf("metabase: url is required")
	}
	apiKey := creds["api_key"]
	token := creds["mcp_access_token"]
	if apiKey == "" && token == "" {
		return fmt.Errorf("metabase: api_key or mcp_access_token is required")
	}
	useRemote := token != "" && !(creds[mcp.CredKeyTokenSource] == "api_key" && apiKey != "")

	m.mu.Lock()
	defer m.mu.Unlock()
	m.apiKey = apiKey
	m.useRemote = useRemote
	if !useRemote {
		m.baseURL = baseURL
		return nil
	}
	remote, err := m.remoteFor(baseURL)
	if err != nil {
		return err
	}
	if err := remote.Configure(ctx, mcp.Credentials{
		"access_token":  token,
		"refresh_token": creds["mcp_refresh_token"],
		"client_id":     creds["mcp_client_id"],
	}); err != nil {
		return fmt.Errorf("metabase: configure hosted MCP: %w", err)
	}
	m.baseURL = baseURL
	return nil
}

func (m *metabase) Healthy(ctx context.Context) bool {
	if remote := m.activeRemote(); remote != nil {
		return remote.Healthy(ctx)
	}
	_, err := m.get(ctx, "/api/user/current")
	return err == nil
}

func (m *metabase) Tools() []mcp.ToolDefinition {
	if remote := m.activeRemote(); remote != nil {
		return remoteTools(remote)
	}
	return tools
}

// CompactSpec and MaxBytes describe REST response shapes; hosted MCP results
// already arrive shaped for LLMs, so both are disabled in OAuth mode.
func (m *metabase) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	if m.activeRemote() != nil {
		return nil, false
	}
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (m *metabase) MaxBytes(toolName mcp.ToolName) (int, bool) {
	if m.activeRemote() != nil {
		return 0, false
	}
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (m *metabase) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if remote := m.activeRemote(); remote != nil {
		return remote.Execute(ctx, toolName, args)
	}
	m.mu.RLock()
	closed := m.useRemote && m.remote == nil
	m.mu.RUnlock()
	if closed {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
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

	m.mu.RLock()
	baseURL, apiKey := m.baseURL, m.apiKey
	m.mu.RUnlock()
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
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
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("metabase API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
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
