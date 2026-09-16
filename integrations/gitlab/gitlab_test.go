package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedCall struct {
	auth string
	name string
	args map[string]any
}

func rawArgsMap(req *mcpsdk.CallToolRequest) map[string]any {
	out := map[string]any{}
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return out
	}
	_ = json.Unmarshal(req.Params.Arguments, &out)
	return out
}

func fakeGitLabMCP(t *testing.T, calls *[]recordedCall, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-gitlab-mcp", Version: "v0.0.1"}, nil)

		mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "list_merge_requests", Description: "upstream list mrs"}, func(_ context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
			mu.Lock()
			*calls = append(*calls, recordedCall{auth: auth, name: "list_merge_requests", args: rawArgsMap(req)})
			mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"merge_requests":[{"iid":1,"title":"Fix"}]}`}}}, nil, nil
		})
		mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "save_note", Description: "upstream note"}, func(_ context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
			mu.Lock()
			*calls = append(*calls, recordedCall{auth: auth, name: "save_note", args: rawArgsMap(req)})
			mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"id":99,"body":"ok"}`}}}, nil, nil
		})
		mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "get_job", Description: "upstream job"}, func(_ context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
			mu.Lock()
			*calls = append(*calls, recordedCall{auth: auth, name: "get_job", args: rawArgsMap(req)})
			mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"id":88,"status":"failed","trace":"log line"}`}}}, nil, nil
		})

		stream := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
		stream.ServeHTTP(w, r)
	})
	mux := http.NewServeMux()
	mux.Handle("/api/v4/mcp", handler)
	return httptest.NewServer(mux)
}

type fakeRemote struct {
	configured mcp.Credentials
	apiBase    string
	tools      []mcp.ToolDefinition
	executed   mcp.ToolName
	args       map[string]any
}

func (f *fakeRemote) Name() string { return "gitlab" }

func (f *fakeRemote) Configure(_ context.Context, creds mcp.Credentials) error {
	f.configured = creds
	return nil
}

func (f *fakeRemote) Healthy(context.Context) bool { return true }

func (f *fakeRemote) Tools() []mcp.ToolDefinition { return f.tools }

func (f *fakeRemote) Execute(_ context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	f.executed = name
	f.args = args
	return mcp.RawResult([]byte("ok"))
}

func TestNew_DefaultURLs(t *testing.T) {
	i := New()
	assert.Equal(t, "gitlab", i.Name())
	assert.Equal(t, defaultInstanceURL, MCPServerURL(i))
	assert.Equal(t, defaultInstanceURL+apiV4Suffix, MCPAPIBaseURL(i))
}

func TestNormalizeInstanceURL(t *testing.T) {
	assert.Equal(t, "https://gitlab.com", normalizeInstanceURL(""))
	assert.Equal(t, "https://gitlab.example.com", normalizeInstanceURL("https://gitlab.example.com/api/v4/mcp"))
	assert.Equal(t, "https://gitlab.example.com", normalizeInstanceURL("gitlab.example.com"))
}

func TestMCPAPIBaseURL_SelfHostedSubpath(t *testing.T) {
	got := mcpAPIBaseURL("https://gitlab.example.com/gitlab")
	assert.Equal(t, "https://gitlab.example.com/gitlab/api/v4", got)
}

func TestConfigure_RequiresToken(t *testing.T) {
	err := New().Configure(context.Background(), mcp.Credentials{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp_access_token")
}

func TestConfigure_AcceptsTokenAlias(t *testing.T) {
	remote := &fakeRemote{apiBase: defaultInstanceURL + apiV4Suffix}
	g := &gitlab{
		instanceURL: defaultInstanceURL,
		mcpAPIBase:  defaultInstanceURL + apiV4Suffix,
		newRemote:   func(_ string) mcp.Integration { return remote },
	}
	err := g.Configure(context.Background(), mcp.Credentials{"token": "glpat-test"})
	require.NoError(t, err)
	assert.Equal(t, "glpat-test", remote.configured["access_token"])
}

func TestTools_EnrichesStartHere(t *testing.T) {
	remote := &fakeRemote{tools: []mcp.ToolDefinition{
		{Name: "gitlab_list_merge_requests", Description: "upstream"},
		{Name: "gitlab_search", Description: "keep upstream"},
	}}
	g := &gitlab{remote: remote}
	tools := g.Tools()
	names := map[mcp.ToolName]mcp.ToolDefinition{}
	for _, tool := range tools {
		names[tool.Name] = tool
	}
	assert.Contains(t, names["gitlab_list_merge_requests"].Description, "Start here")
	assert.Equal(t, "keep upstream", names["gitlab_search"].Description)
}

func TestExecute_ForwardsToRemote(t *testing.T) {
	remote := &fakeRemote{}
	g := &gitlab{remote: remote}
	_, err := g.Execute(context.Background(), "gitlab_save_note", map[string]any{"body": "hi"})
	require.NoError(t, err)
	assert.Equal(t, mcp.ToolName("gitlab_save_note"), remote.executed)
}

func TestExecute_UnknownTool(t *testing.T) {
	g := &gitlab{remote: &fakeRemote{}}
	res, err := g.Execute(context.Background(), "not_gitlab_tool", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestExecute_NotConfigured(t *testing.T) {
	res, err := New().Execute(context.Background(), "gitlab_list_merge_requests", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, mcp.ErrNotConfigured.Error())
}

func TestIntegration_LiveProxyRouting(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeGitLabMCP(t, &calls, &mu)
	defer srv.Close()

	g := New().(*gitlab)
	g.newRemote = func(base string) mcp.Integration {
		return remotemcp.New(integrationName, base)
	}
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{
		"mcp_access_token": "glpat-x",
		"base_url":         srv.URL,
	}))
	require.True(t, g.Healthy(context.Background()))

	tools := g.Tools()
	names := map[string]bool{}
	for _, tool := range tools {
		names[string(tool.Name)] = true
	}
	assert.True(t, names["gitlab_list_merge_requests"])
	assert.True(t, names["gitlab_get_job"])

	res, err := g.Execute(context.Background(), "gitlab_list_merge_requests", nil)
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, calls, 1)
	assert.Equal(t, "Bearer glpat-x", calls[0].auth)
	assert.Equal(t, "list_merge_requests", calls[0].name)
}
