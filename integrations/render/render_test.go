package render

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// upstreamCatalog is the hosted Render MCP tool list (render-oss/render-mcp-server
// at 358418e). Every name is either exposed through dispatch or deliberately
// excluded, so a catalog change upstream shows up here as a reviewed diff.
var upstreamCatalog = []string{
	"approve_blueprint", "cancel_cron_job_run", "cancel_flow_run", "create_blueprint",
	"create_cron_job", "create_key_value", "create_postgres", "create_static_site",
	"create_web_service", "create_workflow", "create_workflow_version", "get_blueprint",
	"get_deploy", "get_flow", "get_flow_run", "get_flow_run_results", "get_flow_run_step",
	"get_key_value", "get_metrics", "get_postgres", "get_selected_workspace", "get_service",
	"get_workflow", "get_workflow_run", "get_workflow_run_results", "get_workflow_task",
	"get_workflow_version", "list_deploys", "list_events", "list_flow_run_steps",
	"list_flow_runs", "list_flows", "list_key_value", "list_log_label_values", "list_logs",
	"list_postgres_instances", "list_services", "list_workflow_runs", "list_workflow_tasks",
	"list_workflow_versions", "list_workflows", "list_workspaces", "plan_blueprint",
	"query_render_postgres", "run_cron_job", "select_workspace", "start_flow_run",
	"trigger_deploy", "update_blueprint", "update_environment_variables",
}

var excludedUpstream = []string{"select_workspace", "get_selected_workspace"}

type hostedCall struct {
	name string
	args map[string]any
	auth string
}

// hostedRender fakes mcp.render.com: like the real server it only speaks the
// legacy session handshake, accepts any bearer token at the transport, and
// reports a bad API key as a tool error once a tool calls the Render API.
type hostedRender struct {
	server  *httptest.Server
	mu      sync.Mutex
	calls   []hostedCall
	auths   []string
	keys    map[string]bool
	replies map[string]string
}

func newHostedRender(t *testing.T, keys ...string) *hostedRender {
	t.Helper()
	fixture := &hostedRender{keys: map[string]bool{}, replies: map[string]string{
		"list_workspaces": `[{"id":"tea-1","name":"Fixture Team","email":"ops@example.com","type":"team"}]`,
		"list_deploys":    `[{"id":"dep-2","status":"live"},{"id":"dep-1","status":"deactivated"}]` + "\n\n cursor: c-dep-1",
		"list_events":     `[{"id":"evt-1","type":"server_failed"}]` + "\n\n cursor: \"\"",
	}}
	for _, key := range keys {
		fixture.keys[key] = true
	}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-render", Version: "1"}, nil)
	for _, tool := range upstreamCatalog {
		server.AddTool(&mcpsdk.Tool{Name: tool, InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			auth := req.Extra.Header.Get("Authorization")
			fixture.mu.Lock()
			fixture.calls = append(fixture.calls, hostedCall{name: req.Params.Name, args: args, auth: auth})
			reply, ok := fixture.replies[req.Params.Name]
			valid := fixture.keys[strings.TrimPrefix(auth, "Bearer ")]
			fixture.mu.Unlock()
			if !valid {
				return &mcpsdk.CallToolResult{IsError: true, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "received response code 401: unauthorized"}}}, nil
			}
			if !ok {
				reply = req.Params.Name
			}
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: reply}}}, nil
		})
	}
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		fixture.auths = append(fixture.auths, r.Header.Get("Authorization"))
		fixture.mu.Unlock()
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var msg struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			_ = json.Unmarshal(body, &msg)
			if msg.Method == "server/discover" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(msg.ID) + `,"error":{"code":-32601,"message":"Method not found"}}`))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *hostedRender) snapshot() ([]hostedCall, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]hostedCall(nil), f.calls...), append([]string(nil), f.auths...)
}

func newIntegration(t *testing.T) mcp.Integration {
	t.Helper()
	integration := New()
	closer, ok := integration.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() { assert.NoError(t, closer.Close()) })
	return integration
}

func configured(t *testing.T, fixture *hostedRender, creds mcp.Credentials) mcp.Integration {
	t.Helper()
	integration := newIntegration(t)
	creds["base_url"] = fixture.server.URL
	require.NoError(t, integration.Configure(t.Context(), creds))
	return integration
}

func toolByName(name mcp.ToolName) mcp.ToolDefinition {
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	return mcp.ToolDefinition{}
}

func TestNew(t *testing.T) {
	integration := New()
	require.NotNil(t, integration)
	assert.Equal(t, "render", integration.Name())
	assert.Equal(t, defaultBaseURL, serverURL(integration))
}

func TestConfigure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		wantURL string
	}{
		{name: "missing key", creds: mcp.Credentials{}, wantErr: "api_key is required"},
		{name: "blank key", creds: mcp.Credentials{"api_key": "  "}, wantErr: "api_key is required"},
		{name: "invalid base url", creds: mcp.Credentials{"api_key": "rnd_x", "base_url": "ftp://x"}, wantErr: "invalid base_url"},
		{name: "relative base url", creds: mcp.Credentials{"api_key": "rnd_x", "base_url": "/mcp"}, wantErr: "invalid base_url"},
		{name: "workspace with spaces", creds: mcp.Credentials{"api_key": "rnd_x", "workspace_id": "tea 1"}, wantErr: "invalid workspace_id"},
		{name: "default url", creds: mcp.Credentials{"api_key": "rnd_x"}, wantURL: defaultBaseURL},
		{name: "trimmed key and workspace", creds: mcp.Credentials{"api_key": " rnd_x ", "workspace_id": " tea-1 "}, wantURL: defaultBaseURL},
		{name: "full endpoint", creds: mcp.Credentials{"api_key": "rnd_x", "base_url": "https://proxy.example/mcp/"}, wantURL: "https://proxy.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			integration := newIntegration(t)
			err := integration.Configure(t.Context(), tc.creds)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.NotContains(t, err.Error(), "rnd_x", "errors must not echo the API key")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantURL, serverURL(integration))
		})
	}
}

func TestCredentialMetadata(t *testing.T) {
	integration := New()
	assert.ElementsMatch(t, []string{"base_url", "workspace_id"}, integration.(mcp.PlainTextCredentials).PlainTextKeys())
	assert.ElementsMatch(t, []string{"base_url", "workspace_id"}, integration.(mcp.OptionalCredentials).OptionalKeys())
	placeholders := integration.(mcp.PlaceholderHints).Placeholders()
	assert.Contains(t, placeholders["api_key"], "Account Settings")
	assert.Contains(t, placeholders["workspace_id"], "tea-")
	assert.Contains(t, placeholders["base_url"], defaultBaseURL)
}

func TestTools_Metadata(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			assert.True(t, strings.HasPrefix(string(tool.Name), toolPrefix))
			assert.Contains(t, tool.Description, "Render")
			assert.True(t, tool.Access.Declared(), "access must be classified")
			assert.False(t, seen[tool.Name], "duplicate tool")
			assert.NotContains(t, tool.Description, "\u2014", "no em dashes")
			seen[tool.Name] = true
			for _, required := range tool.Required {
				assert.Contains(t, tool.Parameters, required)
			}
			if tool.Name != "render_list_workspaces" {
				assert.Contains(t, tool.Parameters, "workspaceId")
			}
		})
	}
	assert.Len(t, seen, 48)
	assert.Contains(t, toolByName("render_list_services").Description, "Start here")
	assert.Contains(t, toolByName("render_list_logs").Description, "Start here")
}

func TestTools_Access(t *testing.T) {
	want := map[mcp.ToolName]mcp.ToolAccess{
		"render_create_web_service":           mcp.AccessWrite,
		"render_create_static_site":           mcp.AccessWrite,
		"render_create_cron_job":              mcp.AccessWrite,
		"render_create_postgres":              mcp.AccessWrite,
		"render_create_key_value":             mcp.AccessWrite,
		"render_create_blueprint":             mcp.AccessWrite,
		"render_plan_blueprint":               mcp.AccessWrite,
		"render_create_workflow":              mcp.AccessWrite,
		"render_start_flow_run":               mcp.AccessWrite,
		"render_trigger_deploy":               mcp.AccessDestructive,
		"render_update_environment_variables": mcp.AccessDestructive,
		"render_run_cron_job":                 mcp.AccessDestructive,
		"render_cancel_cron_job_run":          mcp.AccessDestructive,
		"render_approve_blueprint":            mcp.AccessDestructive,
		"render_update_blueprint":             mcp.AccessDestructive,
		"render_create_workflow_version":      mcp.AccessDestructive,
		"render_cancel_flow_run":              mcp.AccessDestructive,
	}
	for _, tool := range tools {
		t.Run(string(tool.Name), func(t *testing.T) {
			expected, ok := want[tool.Name]
			if !ok {
				expected = mcp.AccessRead
			}
			assert.Equal(t, expected, tool.Access)
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range tools {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch entry", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	defined := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		defined[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, defined[name], "dispatch entry %s has no tool definition", name)
	}
}

func TestDispatchMap_MirrorsUpstreamCatalog(t *testing.T) {
	var exposed []string
	for _, upstream := range dispatch {
		exposed = append(exposed, upstream)
	}
	assert.ElementsMatch(t, upstreamCatalog, append(exposed, excludedUpstream...))
	for _, name := range excludedUpstream {
		assert.NotContains(t, exposed, name)
	}
}

func TestExecute_ForwardsEveryToolUpstream(t *testing.T) {
	fixture := newHostedRender(t, "rnd_good")
	integration := configured(t, fixture, mcp.Credentials{"api_key": "rnd_good"})
	for _, tool := range integration.Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), tool.Name, map[string]any{"serviceId": "srv-1"})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
		})
	}
	calls, auths := fixture.snapshot()
	require.Len(t, calls, len(tools))
	var names []string
	for _, call := range calls {
		names = append(names, call.name)
		assert.Equal(t, "srv-1", call.args["serviceId"])
		assert.NotContains(t, call.args, "workspaceId", "no default workspace configured")
	}
	assert.Contains(t, names, "query_render_postgres")
	for _, auth := range auths {
		assert.Equal(t, "Bearer rnd_good", auth)
	}
}

func TestExecute_DefaultWorkspace(t *testing.T) {
	fixture := newHostedRender(t, "rnd_good")
	integration := configured(t, fixture, mcp.Credentials{"api_key": "rnd_good", "workspace_id": "tea-default"})
	for _, tc := range []struct {
		name string
		tool mcp.ToolName
		args map[string]any
		want any
	}{
		{name: "injected when omitted", tool: "render_list_services", args: map[string]any{}, want: "tea-default"},
		{name: "injected into nil args", tool: "render_list_postgres_instances", args: nil, want: "tea-default"},
		{name: "explicit argument wins", tool: "render_list_services", args: map[string]any{"workspaceId": "tea-other"}, want: "tea-other"},
		{name: "not sent to unscoped tool", tool: "render_list_workspaces", args: map[string]any{}, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before map[string]any
			if tc.args != nil {
				before = map[string]any{}
				for k, v := range tc.args {
					before[k] = v
				}
			}
			result, err := integration.Execute(t.Context(), tc.tool, tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			calls, _ := fixture.snapshot()
			assert.Equal(t, tc.want, calls[len(calls)-1].args["workspaceId"])
			if tc.args != nil {
				assert.Equal(t, before, tc.args, "caller arguments must not be mutated")
			}
		})
	}
}

func TestExecute_CursorPages(t *testing.T) {
	fixture := newHostedRender(t, "rnd_good")
	integration := configured(t, fixture, mcp.Credentials{"api_key": "rnd_good"})
	for _, tc := range []struct {
		tool mcp.ToolName
		want string
	}{
		{"render_list_deploys", `{"items":[{"id":"dep-2","status":"live"},{"id":"dep-1","status":"deactivated"}],"next_cursor":"c-dep-1"}`},
		{"render_list_events", `{"items":[{"id":"evt-1","type":"server_failed"}],"next_cursor":null}`},
	} {
		t.Run(string(tc.tool), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), tc.tool, map[string]any{"serviceId": "srv-1"})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.JSONEq(t, tc.want, result.Data)
		})
	}
}

func TestCursorPage(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"cursor", "[{\"id\":\"a\"}]\n\n cursor: abc", `{"items":[{"id":"a"}],"next_cursor":"abc"}`},
		{"quoted empty cursor", "[]\n\n cursor: \"\"", `{"items":[],"next_cursor":null}`},
		{"bare empty cursor", "[]\n\n cursor: ", `{"items":[],"next_cursor":null}`},
		{"plain text passes through", "No deploys found", "No deploys found"},
		{"invalid json passes through", "[{\n\n cursor: abc", "[{\n\n cursor: abc"},
		{"object passes through", "{\"a\":1}\n\n cursor: abc", "{\"a\":1}\n\n cursor: abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, cursorPage(tc.in))
		})
	}
}

func TestExecute_UnknownAndUnconfigured(t *testing.T) {
	fixture := newHostedRender(t, "rnd_good")
	unconfigured := newIntegration(t)
	result, err := unconfigured.Execute(t.Context(), "render_list_services", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
	assert.False(t, unconfigured.Healthy(t.Context()))

	integration := configured(t, fixture, mcp.Credentials{"api_key": "rnd_good"})
	for _, name := range []mcp.ToolName{"", "render_", "list_services", "render_select_workspace", "render_get_selected_workspace", "render_query_render_postgres", "render_render_list_services", "mobbin_search_screens"} {
		t.Run(string(name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), name, nil)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "unknown tool")
		})
	}
	calls, _ := fixture.snapshot()
	assert.Empty(t, calls)
}

func TestHealthy(t *testing.T) {
	fixture := newHostedRender(t, "rnd_good")
	for _, tc := range []struct {
		key  string
		want bool
	}{{key: "rnd_good", want: true}, {key: "rnd_revoked", want: false}} {
		t.Run(tc.key, func(t *testing.T) {
			integration := configured(t, fixture, mcp.Credentials{"api_key": tc.key})
			assert.Equal(t, tc.want, integration.Healthy(t.Context()))
		})
	}
	calls, _ := fixture.snapshot()
	require.NotEmpty(t, calls)
	assert.True(t, slices.ContainsFunc(calls, func(c hostedCall) bool { return c.name == "list_workspaces" }), "health must call the Render API, not just list tools")
}

func TestHealthy_UnreachableServer(t *testing.T) {
	fixture := newHostedRender(t, "rnd_good")
	integration := configured(t, fixture, mcp.Credentials{"api_key": "rnd_good"})
	fixture.server.Close()
	assert.False(t, integration.Healthy(t.Context()))
}

func TestConfigure_RotatesKeyAndCloseIsIdempotent(t *testing.T) {
	fixture := newHostedRender(t, "rnd_first", "rnd_second")
	integration := configured(t, fixture, mcp.Credentials{"api_key": "rnd_first"})
	require.True(t, integration.Healthy(t.Context()))
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"api_key": "rnd_second", "base_url": fixture.server.URL}))
	result, err := integration.Execute(t.Context(), "render_list_services", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	calls, _ := fixture.snapshot()
	assert.Equal(t, "Bearer rnd_second", calls[len(calls)-1].auth)

	closer := integration.(io.Closer)
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	result, err = integration.Execute(t.Context(), "render_list_services", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Len(t, integration.Tools(), len(tools), "static catalog stays searchable while unconfigured")
}

func TestConfigure_FailureKeepsPreviousConnection(t *testing.T) {
	fixture := newHostedRender(t, "rnd_good")
	integration := configured(t, fixture, mcp.Credentials{"api_key": "rnd_good"})
	require.Error(t, integration.Configure(t.Context(), mcp.Credentials{"api_key": ""}))
	result, err := integration.Execute(t.Context(), "render_list_services", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
}
