package ashby

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureToken = "fixture-token"

type upstreamCall struct {
	name string
	args map[string]any
}

// hostedFixture is a fake Ashby hosted MCP server. Like the real one it
// serves MCP only at /mcp/v1 and requires the bearer token; every upstream
// tool takes a "query" string, and search_records_by_name also requires it.
type hostedFixture struct {
	server     *httptest.Server
	mu         sync.Mutex
	calls      []upstreamCall
	listCalls  int
	tokenCalls atomic.Int32
	// oauth, when set, serves every path other than /mcp/v1.
	oauth http.Handler
}

func newHostedFixture(t *testing.T, results map[string]string, extra ...string) *hostedFixture {
	t.Helper()
	f := &hostedFixture{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-ashby", Version: "1"}, nil)
	names := make([]string, 0, len(dispatch)+len(extra))
	for _, upstream := range dispatch {
		names = append(names, upstream)
	}
	names = append(names, extra...)
	for _, name := range names {
		text := results[name]
		if text == "" {
			text = `{"tool":"` + name + `"}`
		}
		schema := map[string]any{
			"type":       "object",
			"properties": map[string]any{"query": map[string]any{"type": "string", "description": "Upstream " + name + " query"}},
		}
		if name == "search_records_by_name" {
			schema["required"] = []any{"query"}
		}
		server.AddTool(&mcpsdk.Tool{Name: name, InputSchema: schema}, func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			f.mu.Lock()
			f.calls = append(f.calls, upstreamCall{name: req.Params.Name, args: args})
			f.mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}}}, nil
		})
	}
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointPath {
			if f.oauth != nil {
				f.oauth.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+fixtureToken {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.Contains(string(body), `"tools/list"`) {
			f.mu.Lock()
			f.listCalls++
			f.mu.Unlock()
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *hostedFixture) snapshot() []upstreamCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]upstreamCall(nil), f.calls...)
}

func (f *hostedFixture) toolLists() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listCalls
}

func newIntegration(t *testing.T) mcp.Integration {
	t.Helper()
	integration := New()
	closer, ok := integration.(io.Closer)
	require.True(t, ok)
	t.Cleanup(func() { assert.NoError(t, closer.Close()) })
	return integration
}

func configured(t *testing.T, baseURL, token string) mcp.Integration {
	t.Helper()
	integration := newIntegration(t)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": baseURL, "mcp_access_token": token}))
	return integration
}

func toolByName(t *testing.T, tools []mcp.ToolDefinition, name mcp.ToolName) mcp.ToolDefinition {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("%s not found", name)
	return mcp.ToolDefinition{}
}

func TestNew(t *testing.T) {
	integration := New()
	require.NotNil(t, integration)
	assert.Equal(t, "ashby", integration.Name())
	assert.Equal(t, defaultBaseURL, MCPServerURL(integration))
	assert.Equal(t, defaultBaseURL+"/mcp/v1", MCPResource(integration))
	assert.Empty(t, MCPServerURL(nil))
	assert.Empty(t, MCPResource(nil))
}

func TestCredentialKeys(t *testing.T) {
	integration := New()
	plain, ok := integration.(mcp.PlainTextCredentials)
	require.True(t, ok)
	assert.Equal(t, []string{"base_url"}, plain.PlainTextKeys())
	optional, ok := integration.(mcp.OptionalCredentials)
	require.True(t, ok)
	assert.Equal(t, []string{"base_url"}, optional.OptionalKeys())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		wantURL string
	}{
		{name: "missing token", creds: mcp.Credentials{}, wantErr: "mcp_access_token is required"},
		{name: "blank token", creds: mcp.Credentials{"mcp_access_token": "  "}, wantErr: "mcp_access_token is required"},
		{name: "bad scheme", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "ftp://mcp.ashbyhq.com"}, wantErr: "invalid base_url"},
		{name: "no host", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "https://"}, wantErr: "invalid base_url"},
		{name: "default url", creds: mcp.Credentials{"mcp_access_token": "t"}, wantURL: defaultBaseURL},
		{name: "full endpoint", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "https://example.test/mcp/v1/"}, wantURL: "https://example.test"},
		{name: "origin", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": " https://example.test/ "}, wantURL: "https://example.test"},
		{name: "query dropped", creds: mcp.Credentials{"mcp_access_token": "t", "base_url": "https://example.test/mcp/v1?x=1#f"}, wantURL: "https://example.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := newIntegration(t)
			err := integration.Configure(t.Context(), tt.creds)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.NotContains(t, err.Error(), "  ")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, MCPServerURL(integration))
			assert.Equal(t, tt.wantURL+"/mcp/v1", MCPResource(integration))
		})
	}
}

func TestTools_Metadata(t *testing.T) {
	tools := New().Tools()
	require.Len(t, tools, len(dispatch))
	seen := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		assert.True(t, strings.HasPrefix(string(tool.Name), "ashby_"), "%s lacks prefix", tool.Name)
		assert.NotEmpty(t, tool.Description, "%s has no description", tool.Name)
		assert.NotContains(t, tool.Description, "\u2014", "%s uses an em dash", tool.Name)
		assert.True(t, tool.Access.Declared(), "%s does not declare Access", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		assert.NotNil(t, tool.Parameters, "%s has nil parameters", tool.Name)
		seen[tool.Name] = true
	}
}

func TestTools_StartHere(t *testing.T) {
	var starts []mcp.ToolName
	for _, tool := range New().Tools() {
		if strings.Contains(tool.Description, "Start here") {
			starts = append(starts, tool.Name)
		}
	}
	assert.Equal(t, []mcp.ToolName{"ashby_search_records_by_name"}, starts)
}

func TestTools_Access(t *testing.T) {
	want := map[mcp.ToolName]mcp.ToolAccess{
		"ashby_create_candidate":           mcp.AccessWrite,
		"ashby_add_note_to_candidate":      mcp.AccessWrite,
		"ashby_consider_candidate_for_job": mcp.AccessWrite,
		"ashby_change_application_stage":   mcp.AccessDestructive,
	}
	for _, tool := range New().Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			expected, ok := want[tool.Name]
			if !ok {
				expected = mcp.AccessRead
			}
			assert.Equal(t, expected, tool.Access)
		})
	}
}

func TestTools_ReturnsCopy(t *testing.T) {
	integration := New()
	tools := integration.Tools()
	tools[0].Parameters["injected"] = "x"
	tools[0].Description = "changed"
	again := integration.Tools()
	assert.NotEqual(t, "changed", again[0].Description)
	_, injected := again[0].Parameters["injected"]
	assert.False(t, injected)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch entry", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name, upstream := range dispatch {
		assert.True(t, names[name], "dispatch entry %s has no tool definition", name)
		assert.Equal(t, "ashby_"+upstream, string(name), "exposed name must be the prefixed upstream name")
	}
}

func TestExecute_Unconfigured(t *testing.T) {
	integration := newIntegration(t)
	assert.False(t, integration.Healthy(t.Context()))
	result, err := integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
}

func TestExecute_UnknownTool(t *testing.T) {
	fixture := newHostedFixture(t, nil, "submit_mcp_feedback")
	integration := configured(t, fixture.server.URL, fixtureToken)
	for _, name := range []mcp.ToolName{"ashby_nope", "ashby_submit_mcp_feedback", "get_candidate", "other_get_candidate"} {
		t.Run(string(name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), name, nil)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "unknown tool")
		})
	}
	assert.Empty(t, fixture.snapshot(), "unknown tools must never reach the hosted server")
}

func TestExecute_ForwardsToHostedMCP(t *testing.T) {
	fixture := newHostedFixture(t, map[string]string{"search_records_by_name": `{"results":[{"type":"candidate","id":"cand-1","name":"Jordan Lee"}]}`})
	integration := configured(t, fixture.server.URL+"/mcp/v1", fixtureToken)
	assert.True(t, integration.Healthy(t.Context()))

	args := map[string]any{"query": "Jordan Lee"}
	result, err := integration.Execute(t.Context(), "ashby_search_records_by_name", args)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.JSONEq(t, `{"results":[{"type":"candidate","id":"cand-1","name":"Jordan Lee"}]}`, result.Data)

	result, err = integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)

	calls := fixture.snapshot()
	require.Len(t, calls, 2)
	assert.Equal(t, "search_records_by_name", calls[0].name)
	assert.Equal(t, args, calls[0].args)
	assert.Equal(t, "get_pending_tasks", calls[1].name)
	assert.Empty(t, calls[1].args)
}

func TestTools_UsesUpstreamSchemasAfterHealthCheck(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	integration := configured(t, fixture.server.URL, fixtureToken)
	require.True(t, integration.Healthy(t.Context()))

	tools := integration.Tools()
	search := toolByName(t, tools, "ashby_search_records_by_name")
	assert.Equal(t, "Upstream search_records_by_name query", search.Parameters["query"])
	assert.Equal(t, []string{"query"}, search.Required)
	assert.Contains(t, search.Description, "Start here", "reviewed descriptions are kept")
	assert.Equal(t, mcp.AccessRead, search.Access)
	assert.Len(t, tools, len(dispatch))

	stage := toolByName(t, tools, "ashby_change_application_stage")
	assert.Equal(t, mcp.AccessDestructive, stage.Access, "upstream schemas never change the access class")
}

func TestTools_FetchesUpstreamSchemasInBackground(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	integration := configured(t, fixture.server.URL, fixtureToken)

	first := toolByName(t, integration.Tools(), "ashby_get_candidate")
	assert.Empty(t, first.Parameters, "search does not wait for the hosted server")
	require.Eventually(t, func() bool {
		return toolByName(t, integration.Tools(), "ashby_get_candidate").Parameters["query"] != ""
	}, 5*time.Second, 10*time.Millisecond)
	for range 3 {
		integration.Tools()
	}
	integration.(*ashby).fetches.Wait()
	assert.Equal(t, 1, fixture.toolLists(), "the schema list is fetched once")
}

func TestTools_HiddenToolsStayHidden(t *testing.T) {
	fixture := newHostedFixture(t, nil, "submit_mcp_feedback")
	integration := configured(t, fixture.server.URL, fixtureToken)
	require.True(t, integration.Healthy(t.Context()))
	for _, tool := range integration.Tools() {
		assert.NotEqual(t, mcp.ToolName("ashby_submit_mcp_feedback"), tool.Name)
	}
}

func TestHealthy_RejectedToken(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	integration := configured(t, fixture.server.URL, "wrong-token")
	assert.False(t, integration.Healthy(t.Context()))
}

func TestConfigure_ReplacesRemoteWhenBaseURLChanges(t *testing.T) {
	first := newHostedFixture(t, nil)
	second := newHostedFixture(t, nil)
	integration := configured(t, first.server.URL, fixtureToken)
	_, err := integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)

	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": second.server.URL, "mcp_access_token": fixtureToken}))
	assert.Equal(t, second.server.URL, MCPServerURL(integration))
	result, err := integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Len(t, first.snapshot(), 1)
	assert.Len(t, second.snapshot(), 1)
}

func TestClose_Idempotent(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	integration := configured(t, fixture.server.URL, fixtureToken)
	integration.Tools()
	closer := integration.(io.Closer)
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	result, err := integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)

	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": fixture.server.URL, "mcp_access_token": fixtureToken}))
	result, err = integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
}

func TestMaxResponseBytesForTool(t *testing.T) {
	integration := New().(mcp.PerToolMaxResponseBytesIntegration)
	limit, ok := integration.MaxResponseBytesForTool("ashby_get_candidate")
	assert.True(t, ok)
	assert.Greater(t, limit, 50*1024)
	_, ok = integration.MaxResponseBytesForTool("ashby_get_pending_tasks")
	assert.False(t, ok)
}

type failingRemote struct{ mcp.Integration }

func (failingRemote) Configure(context.Context, mcp.Credentials) error {
	return errors.New("remote refused")
}

func TestConfigure_KeepsWorkingLoginWhenNewRemoteFails(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	integration := configured(t, fixture.server.URL, fixtureToken)
	a := integration.(*ashby)
	a.newRemote = func(string) mcp.Integration { return failingRemote{} }

	err := integration.Configure(t.Context(), mcp.Credentials{"base_url": "https://other.test", "mcp_access_token": "other-token"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote refused")

	a.mu.RLock()
	assert.Equal(t, fixtureToken, a.creds["mcp_access_token"], "credentials stay paired with the remote that still serves them")
	assert.Equal(t, fixture.server.URL, a.baseURL)
	a.mu.RUnlock()
	result, err := integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
}
