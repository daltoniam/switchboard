package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/prometheus"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestPrometheusMCPDiscoveryAndExecute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer fixture-token", req.Header.Get("Authorization"))
		if req.URL.Path == "/api/v1/query" {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up","job":"api"},"value":[100,"1"]}]},"warnings":["fixture warning"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"3.0"}}`))
	}))
	defer upstream.Close()
	reg := newMockRegistry()
	require.NoError(t, reg.Register(prometheus.New()))
	instance := New(&mcp.Services{Config: newMockConfigService(map[string]*mcp.IntegrationConfig{"prometheus": {Enabled: true, Credentials: mcp.Credentials{"base_url": upstream.URL, "bearer_token": "fixture-token"}}}), Registry: reg})
	httpServer := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: instance.StatelessHandler()}))
	defer httpServer.Close()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "prometheus-smoke", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp", HTTPClient: httpServer.Client(), DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()
	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "prometheus", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	for _, tool := range prometheus.New().Tools() {
		require.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, string(tool.Name))
	}
	scenarios := []struct {
		query    string
		expected string
	}{
		{"instant PromQL metrics", "prometheus_query"},
		{"scrape target health", "prometheus_list_targets"},
		{"firing alerts", "prometheus_list_alerts"},
	}
	for _, item := range scenarios {
		t.Run(item.query, func(t *testing.T) {
			result, err := instance.handleSearch(t.Context(), searchRequest(map[string]any{"query": item.query, "limit": 3}))
			require.NoError(t, err)
			require.Contains(t, searchToolNames(t, parseSearchResponse(t, result)), item.expected)
		})
	}
	result, err = session.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "prometheus_query", "arguments": map[string]any{"query": "up"}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text := result.Content[0].(*mcpsdk.TextContent).Text
	require.Contains(t, text, "api")
	require.Contains(t, text, "fixture warning")
	require.NotEqual(t, "{}", text)
	require.NotContains(t, text, "fixture-token")
}
