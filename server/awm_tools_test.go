package server

import (
	"context"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/awm"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWM_ToolsOnMainMCP(t *testing.T) {
	root := t.TempDir()
	store := awm.NewStore(root)
	reg := newMockRegistry()
	services := &mcp.Services{
		Config:   newMockConfigService(map[string]*mcp.IntegrationConfig{}),
		Registry: reg,
	}
	s := New(services, WithAWM(store))
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "awm-test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/mcp",
		HTTPClient: httpSrv.Client(),
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	for _, n := range []string{
		"awm.work_profile.put", "awm.work_session.create", "awm.agent_profile.list",
	} {
		assert.True(t, names[n], "missing %s", n)
	}

	put, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "awm.work_profile.put",
		Arguments: map[string]any{
			"version": "1", "work_profile_id": "review", "display_name": "Review",
		},
	})
	require.NoError(t, err)
	require.False(t, put.IsError, "%v", put.StructuredContent)

	ap, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "awm.agent_profile.put",
		Arguments: map[string]any{
			"version": "1", "agent_profile_id": "reviewer", "display_name": "Reviewer",
		},
	})
	require.NoError(t, err)
	require.False(t, ap.IsError, "%v", ap.StructuredContent)

	ws, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "awm.work_session.create",
		Arguments: map[string]any{
			"version": "1", "work_session_id": "ws-1", "display_name": "PR review",
			"project_id": "switchboard", "work_profile_id": "review",
			"agent_profile_ids": []string{"reviewer"}, "state": "proposed",
		},
	})
	require.NoError(t, err)
	if ws.IsError {
		msg := ""
		if len(ws.Content) > 0 {
			if tc, ok := ws.Content[0].(*mcpsdk.TextContent); ok {
				msg = tc.Text
			}
		}
		t.Fatalf("create err: %s structured=%#v", msg, ws.StructuredContent)
	}

	tr, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "awm.work_session.transition",
		Arguments: map[string]any{"id": "ws-1", "state": "open"},
	})
	require.NoError(t, err)
	require.False(t, tr.IsError, "%v", tr.StructuredContent)
}
