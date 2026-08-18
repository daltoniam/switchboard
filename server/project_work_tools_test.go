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

func TestProjectWorkModel_ToolsOnMainMCP(t *testing.T) {
	root := t.TempDir()
	store := awm.NewStore(root)
	reg := newMockRegistry()
	services := &mcp.Services{
		Config:   newMockConfigService(map[string]*mcp.IntegrationConfig{}),
		Registry: reg,
	}
	s := New(services, WithProjectWorkModel(store))
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "project-work-test", Version: "0"}, nil)
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
		"project_work_profile_put", "project_work_session_create", "project_agent_profile_list",
	} {
		assert.True(t, names[n], "missing %s", n)
	}

	put, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_work_profile_put",
		Arguments: map[string]any{
			"version": "1", "work_profile_id": "review", "display_name": "Review",
		},
	})
	require.NoError(t, err)
	require.False(t, put.IsError, "%v", put.StructuredContent)

	ap, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_agent_profile_put",
		Arguments: map[string]any{
			"version": "1", "agent_profile_id": "reviewer", "display_name": "Reviewer",
		},
	})
	require.NoError(t, err)
	require.False(t, ap.IsError, "%v", ap.StructuredContent)

	_, err = store.PutProject(context.Background(), awm.Project{Version: "1", ProjectID: "switchboard", Description: "sb"})
	require.NoError(t, err)

	ws, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_work_session_create",
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
		Name:      "project_work_session_transition",
		Arguments: map[string]any{"id": "ws-1", "state": "open"},
	})
	require.NoError(t, err)
	require.False(t, tr.IsError, "%v", tr.StructuredContent)
}

func TestProjectWorkModel_DefaultProfileSeededViaStore(t *testing.T) {
	root := t.TempDir()
	store := awm.NewStore(root)
	_, err := store.EnsureDefaultWorkProfile(context.Background())
	require.NoError(t, err)

	reg := newMockRegistry()
	services := &mcp.Services{
		Config:   newMockConfigService(map[string]*mcp.IntegrationConfig{}),
		Registry: reg,
	}
	s := New(services, WithProjectWorkModel(store))
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "default-profile-test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/mcp",
		HTTPClient: httpSrv.Client(),
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	got, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project_work_profile_get",
		Arguments: map[string]any{"id": "default"},
	})
	require.NoError(t, err)
	require.False(t, got.IsError, "%v", got.StructuredContent)
}

func TestProjectWorkModel_DeleteReferencedProfileStableError(t *testing.T) {
	root := t.TempDir()
	store := awm.NewStore(root)
	ctx := context.Background()
	_, err := store.PutWorkProfile(ctx, awm.WorkProfile{Version: "1", WorkProfileID: "wp"})
	require.NoError(t, err)
	_, err = store.PutProject(ctx, awm.Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)
	_, err = store.CreateWorkSession(ctx, awm.WorkSession{
		Version: "1", WorkSessionID: "ws", ProjectID: "p", WorkProfileID: "wp", State: awm.StateOpen,
	})
	require.NoError(t, err)

	reg := newMockRegistry()
	services := &mcp.Services{
		Config:   newMockConfigService(map[string]*mcp.IntegrationConfig{}),
		Registry: reg,
	}
	s := New(services, WithProjectWorkModel(store))
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ref-test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/mcp",
		HTTPClient: httpSrv.Client(),
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	del, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "project_work_profile_delete",
		Arguments: map[string]any{"id": "wp"},
	})
	require.NoError(t, err)
	require.True(t, del.IsError)
	// Structured error must expose stable code.
	sc, _ := del.StructuredContent.(map[string]any)
	require.NotNil(t, sc)
	errBody, _ := sc["error"].(map[string]any)
	require.NotNil(t, errBody)
	assert.Equal(t, awm.CodeReferenced, errBody["code"])
}

func TestProjectWorkModel_RejectFabricatedSnapshot(t *testing.T) {
	root := t.TempDir()
	store := awm.NewStore(root)
	ctx := context.Background()
	_, err := store.EnsureDefaultWorkProfile(ctx)
	require.NoError(t, err)
	_, err = store.PutProject(ctx, awm.Project{Version: "1", ProjectID: "p"})
	require.NoError(t, err)

	reg := newMockRegistry()
	services := &mcp.Services{
		Config:   newMockConfigService(map[string]*mcp.IntegrationConfig{}),
		Registry: reg,
	}
	s := New(services, WithProjectWorkModel(store))
	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{MCP: s.StatelessHandler()}))
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "snap-test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/mcp",
		HTTPClient: httpSrv.Client(),
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	ws, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_work_session_create",
		Arguments: map[string]any{
			"version": "1", "work_session_id": "ws-bad",
			"project_id": "p", "work_profile_id": "default",
			"project_revision":    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"project_snapshot_id": "sha256:abc",
			"state":               "proposed",
		},
	})
	require.NoError(t, err)
	require.True(t, ws.IsError, "fabricated snapshot must fail")
	sc, _ := ws.StructuredContent.(map[string]any)
	require.NotNil(t, sc)
	errBody, _ := sc["error"].(map[string]any)
	require.NotNil(t, errBody)
	assert.Equal(t, awm.CodeInvalidReference, errBody["code"])
}
