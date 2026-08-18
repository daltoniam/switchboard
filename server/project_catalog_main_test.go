package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectCatalog_OnMainMCPEndpoint(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	cat := NewProjectCatalogServer(store, store, store, store, ProjectCatalogOptions{WritesEnabled: true})

	reg := newMockRegistry()
	services := &mcp.Services{
		Config:   newMockConfigService(map[string]*mcp.IntegrationConfig{}),
		Registry: reg,
	}
	s := New(services, WithProjectCatalog(cat))
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	status, _, body := postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{"_meta": modernRequestMeta()},
	}, modernHTTPHeaders("tools/list", ""))
	require.Equal(t, http.StatusOK, status, body)
	names := map[string]bool{}
	for _, raw := range resultArray(t, jsonRPCResult(t, body), "tools") {
		m, _ := raw.(map[string]any)
		names[fmt.Sprint(m["name"])] = true
	}
	for _, n := range []string{
		"search", "execute",
		"project.list", "project.get", "project.create", "project.update", "project.delete",
		// transition aliases
		"project.search", "project.resolve", "project.patch",
	} {
		assert.True(t, names[n], "missing tool %s", n)
	}

	status, _, body = postJSONRPC(t, httpSrv.URL+"/mcp", jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/templates/list",
		Params:  map[string]any{"_meta": modernRequestMeta()},
	}, modernHTTPHeaders("resources/templates/list", ""))
	require.Equal(t, http.StatusOK, status, body)
	assert.Equal(t, "private", jsonRPCResult(t, body)["cacheScope"])

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "main-catalog", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/mcp",
		HTTPClient: httpSrv.Client(),
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	created, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project.create",
		Arguments: map[string]any{
			"definition": map[string]any{
				"version": "1",
				"name":    "on-main",
				"resources": map[string]any{
					"main": map[string]any{"type": "repo", "path": "/tmp/on-main", "branch": "main"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, created.IsError, "%v", created.StructuredContent)
}
