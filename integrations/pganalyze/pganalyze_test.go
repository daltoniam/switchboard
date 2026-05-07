package pganalyze

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "pganalyze", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-token-123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	err := p.Configure(context.Background(), mcp.Credentials{"api_key": "key", "base_url": "https://pganalyze.example.com/"})
	assert.NoError(t, err)
	assert.Equal(t, "https://pganalyze.example.com/mcp", p.mcpURL)
}

func TestConfigure_DefaultMCPURL(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	err := p.Configure(context.Background(), mcp.Credentials{"api_key": "key"})
	assert.NoError(t, err)
	assert.Equal(t, defaultMCPEndpoint, p.mcpURL)
}

func TestConfigure_BaseURLWithMCPSuffix(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	err := p.Configure(context.Background(), mcp.Credentials{"api_key": "key", "base_url": "https://app.pganalyze.com/mcp"})
	assert.NoError(t, err)
	assert.Equal(t, "https://app.pganalyze.com/mcp", p.mcpURL)
}

func TestPlainTextKeys(t *testing.T) {
	p := New()
	ptc, ok := p.(mcp.PlainTextCredentials)
	require.True(t, ok, "pganalyze must implement PlainTextCredentials")
	keys := ptc.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
}

func TestExecute_ProxyNotInitialized(t *testing.T) {
	// Server that fails initialization
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`unauthorized`))
	}))
	defer ts.Close()

	p := &pganalyze{apiKey: "bad-key", mcpURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "pganalyze_list_servers", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "proxy not initialized")
}

// --- Proxy tests ---

func newMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(400)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"serverInfo":      map[string]string{"name": "pganalyze", "version": "1.0.0"},
				},
			})
		case "notifications/initialized":
			w.WriteHeader(202)
		case "tools/list":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{
							"name":        "list_servers",
							"description": "List monitored PostgreSQL servers",
							"inputSchema": map[string]any{
								"type":       "object",
								"properties": map[string]any{},
							},
						},
						{
							"name":        "get_server_details",
							"description": "Get details for a specific server",
							"inputSchema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"server_id": map[string]any{
										"type":        "string",
										"description": "The server ID",
									},
								},
								"required": []string{"server_id"},
							},
						},
						{
							"name":        "get_query_stats",
							"description": "Get top queries by runtime percentage",
							"inputSchema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"database_id": map[string]any{
										"type":        "string",
										"description": "Database ID",
									},
									"limit": map[string]any{
										"type":        "integer",
										"description": "Number of queries",
									},
								},
								"required": []string{"database_id"},
							},
						},
						{
							"name":        "get_issues",
							"description": "Get active check-up issues and alerts",
							"inputSchema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"server_id": map[string]any{
										"type":        "string",
										"description": "Server ID to filter",
									},
								},
							},
						},
						{
							"name":        "get_tables",
							"description": "List tables with filtering and pagination",
							"inputSchema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"database_id": map[string]any{
										"type":        "string",
										"description": "Database ID",
									},
								},
								"required": []string{"database_id"},
							},
						},
					},
				},
			})
		case "tools/call":
			params, _ := req.Params.(map[string]any)
			name, _ := params["name"].(string)
			switch name {
			case "list_servers":
				json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": `[{"id":"srv-1","name":"prod-db"}]`},
						},
					},
				})
			case "get_issues":
				json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": `[{"id":"issue-1","severity":"warning"}]`},
						},
					},
				})
			default:
				json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": `{"result":"ok"}`},
						},
					},
				})
			}
		}
	}))
}

func newTestPganalyze(t *testing.T) (*pganalyze, *httptest.Server) {
	t.Helper()
	ts := newMCPServer(t)
	t.Cleanup(ts.Close)
	p := &pganalyze{
		apiKey: "test-key",
		mcpURL: ts.URL,
		client: ts.Client(),
	}
	p.startProxy(context.Background())
	require.NotNil(t, p.proxy, "proxy should be initialized")
	return p, ts
}

func TestProxy_Initialize(t *testing.T) {
	p, _ := newTestPganalyze(t)
	assert.NotNil(t, p.proxy)
	assert.True(t, p.started)
}

func TestProxy_FetchTools(t *testing.T) {
	p, _ := newTestPganalyze(t)
	tools := p.Tools()
	assert.Len(t, tools, 5)

	names := make(map[mcp.ToolName]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	assert.True(t, names["pganalyze_list_servers"])
	assert.True(t, names["pganalyze_get_server_details"])
	assert.True(t, names["pganalyze_get_query_stats"])
	assert.True(t, names["pganalyze_get_issues"])
	assert.True(t, names["pganalyze_get_tables"])
}

func TestProxy_ToolDefinitions_HavePrefix(t *testing.T) {
	p, _ := newTestPganalyze(t)
	for _, tool := range p.Tools() {
		assert.Contains(t, string(tool.Name), "pganalyze_", "tool %s missing pganalyze_ prefix", tool.Name)
	}
}

func TestProxy_ToolDefinitions_NoDuplicates(t *testing.T) {
	p, _ := newTestPganalyze(t)
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range p.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestProxy_ToolDefinitions_RequiredFields(t *testing.T) {
	p, _ := newTestPganalyze(t)
	for _, tool := range p.Tools() {
		if tool.Name == "pganalyze_get_server_details" {
			assert.Contains(t, tool.Required, "server_id")
		}
		if tool.Name == "pganalyze_get_query_stats" {
			assert.Contains(t, tool.Required, "database_id")
		}
	}
}

func TestProxy_Execute_ListServers(t *testing.T) {
	p, _ := newTestPganalyze(t)
	result, err := p.Execute(context.Background(), "pganalyze_list_servers", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "srv-1")
	assert.Contains(t, result.Data, "prod-db")
}

func TestProxy_Execute_GetIssues(t *testing.T) {
	p, _ := newTestPganalyze(t)
	result, err := p.Execute(context.Background(), "pganalyze_get_issues", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "issue-1")
	assert.Contains(t, result.Data, "warning")
}

func TestProxy_Execute_UnknownTool(t *testing.T) {
	p, _ := newTestPganalyze(t)
	result, err := p.Execute(context.Background(), "pganalyze_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestProxy_Execute_WithArgs(t *testing.T) {
	p, _ := newTestPganalyze(t)
	result, err := p.Execute(context.Background(), "pganalyze_get_tables", map[string]any{"database_id": "db-123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ok")
}

func TestProxy_RateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	_, err := proxy.send(context.Background(), "tools/list", nil)
	assert.Error(t, err)

	var re *mcp.RetryableError
	assert.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
}

func TestProxy_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`internal error`))
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	_, err := proxy.send(context.Background(), "tools/list", nil)
	assert.Error(t, err)

	var re *mcp.RetryableError
	assert.ErrorAs(t, err, &re)
	assert.Equal(t, 500, re.StatusCode)
}

func TestProxy_JSONRPCError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error":   map[string]any{"code": -32600, "message": "invalid request"},
		})
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	_, err := proxy.send(context.Background(), "bad_method", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestProxy_MCPErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "Error: server not found"},
				},
				"isError": true,
			},
		})
	}))
	defer ts.Close()

	proxy := newProxyClient(ts.URL, "test-key", ts.Client())
	proxy.tools = []proxyToolDef{{Name: "list_servers"}}
	result, err := proxy.execute(context.Background(), "pganalyze_list_servers", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "server not found")
}

func TestHealthy_WithWorkingProxy(t *testing.T) {
	p, _ := newTestPganalyze(t)
	assert.True(t, p.Healthy(context.Background()))
}

func TestHealthy_EmptyAPIKey(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	assert.False(t, p.Healthy(context.Background()))
}

func TestTools_StaticFallbackWhenNoProxy(t *testing.T) {
	p := &pganalyze{client: &http.Client{}}
	tools := p.Tools()
	assert.Equal(t, len(staticTools), len(tools))
	for _, tool := range tools {
		assert.Contains(t, string(tool.Name), "pganalyze_")
		assert.NotEmpty(t, tool.Description)
	}
}

func TestTools_StaticToolsNoDuplicates(t *testing.T) {
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range staticTools {
		assert.False(t, seen[tool.Name], "duplicate static tool: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestTools_StaticToolsCount(t *testing.T) {
	assert.Equal(t, 20, len(staticTools), "expected 20 tools matching pganalyze MCP server")
}
