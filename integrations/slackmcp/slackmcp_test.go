package slackmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
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

// fakeHostedSlackMCP serves Streamable HTTP MCP with per-token tool catalogs.
func fakeHostedSlackMCP(t *testing.T, calls *[]recordedCall, mu *sync.Mutex) *httptest.Server {
	t.Helper()

	// One MCP server instance that branches on bearer token via request context is hard;
	// instead use a custom handler that creates per-request servers keyed by auth.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")

		server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-slack-mcp", Version: "v0.0.1"}, nil)

		// Common tool
		type sendIn struct {
			ChannelID string `json:"channel_id" jsonschema:"channel id"`
			Message   string `json:"message" jsonschema:"message body"`
		}
		mcpsdk.AddTool(server, &mcpsdk.Tool{
			Name:        "slack_send_message",
			Description: "Send a message",
		}, func(_ context.Context, req *mcpsdk.CallToolRequest, in sendIn) (*mcpsdk.CallToolResult, any, error) {
			args := rawArgsMap(req)
			mu.Lock()
			*calls = append(*calls, recordedCall{auth: auth, name: "slack_send_message", args: args})
			mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "sent:" + token + ":" + in.ChannelID}}}, nil, nil
		})

		// Token-specific exclusive tools
		switch token {
		case "tok-work":
			type searchIn struct {
				Query string `json:"query" jsonschema:"search query"`
			}
			mcpsdk.AddTool(server, &mcpsdk.Tool{
				Name:        "slack_search_public",
				Description: "Search public",
			}, func(_ context.Context, req *mcpsdk.CallToolRequest, in searchIn) (*mcpsdk.CallToolResult, any, error) {
				args := rawArgsMap(req)
				mu.Lock()
				*calls = append(*calls, recordedCall{auth: auth, name: "slack_search_public", args: args})
				mu.Unlock()
				return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "search-work:" + in.Query}}}, nil, nil
			})
		case "tok-personal":
			type readIn struct {
				ChannelID string `json:"channel_id" jsonschema:"channel id"`
			}
			mcpsdk.AddTool(server, &mcpsdk.Tool{
				Name:        "slack_read_channel",
				Description: "Read channel",
			}, func(_ context.Context, req *mcpsdk.CallToolRequest, in readIn) (*mcpsdk.CallToolResult, any, error) {
				args := rawArgsMap(req)
				mu.Lock()
				*calls = append(*calls, recordedCall{auth: auth, name: "slack_read_channel", args: args})
				mu.Unlock()
				return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "read-personal:" + in.ChannelID}}}, nil, nil
			})
		}

		stream := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
			return server
		}, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
		stream.ServeHTTP(w, r)
	})

	// Mount at /mcp to match remotemcp client path construction.
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.Handle("/mcp/", handler)
	return httptest.NewServer(mux)
}

func configureTwoIdentities(t *testing.T, baseURL string) mcp.Integration {
	t.Helper()
	i := New()
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"base_url": baseURL}))
	multi := i.(mcp.MultiIdentityIntegration)
	require.NoError(t, multi.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {
			Credentials: mcp.Credentials{"access_token": "tok-work"},
			Metadata:    map[string]string{"label": "Work", "team": "T-WORK"},
		},
		"personal": {
			Credentials: mcp.Credentials{"access_token": "tok-personal"},
			Metadata:    map[string]string{"label": "Personal", "team": "T-PERSONAL"},
		},
	}))
	return i
}

func TestName(t *testing.T) {
	assert.Equal(t, "slackmcp", New().Name())
}

func TestConfigure_BaseURLOptional(t *testing.T) {
	i := New()
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{}))
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"base_url": "https://example.test"}))
}

func TestConfigureIdentities_RequiresAccessToken(t *testing.T) {
	i := New().(mcp.MultiIdentityIntegration)
	err := i.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {Credentials: mcp.Credentials{}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
	assert.NotContains(t, err.Error(), "xox")
}

func TestConfigureIdentities_RejectsEmptyID(t *testing.T) {
	i := New().(mcp.MultiIdentityIntegration)
	err := i.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"": {Credentials: mcp.Credentials{"access_token": "tok"}},
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "identity")
}

func TestPlainTextAndOptionalKeys(t *testing.T) {
	i := New()
	pt, ok := i.(mcp.PlainTextCredentials)
	require.True(t, ok)
	assert.Contains(t, pt.PlainTextKeys(), "base_url")
	opt, ok := i.(mcp.OptionalCredentials)
	require.True(t, ok)
	assert.Contains(t, opt.OptionalKeys(), "base_url")
}

func TestMultiIdentity_DistinctBearerTokensAndUnion(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()

	i := configureTwoIdentities(t, srv.URL)
	tools := i.Tools()

	names := map[string]mcp.ToolDefinition{}
	for _, td := range tools {
		names[string(td.Name)] = td
	}

	// Discovery tool
	require.Contains(t, names, "slackmcp_list_available_identites")
	assert.Contains(t, names["slackmcp_list_available_identites"].Description, "Start here")
	_, hasID := names["slackmcp_list_available_identites"].Parameters["identity_id"]
	assert.False(t, hasID, "list tool must not require identity_id")

	// Proxied tools: translated once, no slackmcp_slack_
	require.Contains(t, names, "slackmcp_send_message")
	require.Contains(t, names, "slackmcp_search_public") // work-only
	require.Contains(t, names, "slackmcp_read_channel")  // personal-only
	assert.NotContains(t, names, "slackmcp_slack_send_message")

	// identity_id injected as required on every proxied tool
	for _, n := range []string{"slackmcp_send_message", "slackmcp_search_public", "slackmcp_read_channel"} {
		td := names[n]
		require.Contains(t, td.Parameters, "identity_id")
		assert.Contains(t, td.Required, "identity_id")
	}

	// Execute through work identity
	res, err := i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "work",
		"channel_id":  "C1",
		"message":     "hi",
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.Contains(t, res.Data, "tok-work")

	// Execute through personal identity exclusive tool
	res, err = i.Execute(context.Background(), mcp.ToolName("slackmcp_read_channel"), map[string]any{
		"identity_id": "personal",
		"channel_id":  "C2",
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.Contains(t, res.Data, "read-personal")

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(calls), 2)
	var sawWork, sawPersonal bool
	for _, c := range calls {
		assert.NotContains(t, c.args, "identity_id", "identity_id must be stripped before upstream call")
		if c.auth == "Bearer tok-work" {
			sawWork = true
		}
		if c.auth == "Bearer tok-personal" {
			sawPersonal = true
		}
		// Never leak tokens in args payload
		b, _ := json.Marshal(c.args)
		assert.NotContains(t, string(b), "tok-work")
		assert.NotContains(t, string(b), "tok-personal")
	}
	assert.True(t, sawWork)
	assert.True(t, sawPersonal)
}

func TestExecute_MissingAndUnknownIdentity(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()
	i := configureTwoIdentities(t, srv.URL)

	res, err := i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"channel_id": "C1",
		"message":    "hi",
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, "identity_id")
	assert.NotContains(t, res.Data, "tok-")

	res, err = i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "missing",
		"channel_id":  "C1",
		"message":     "hi",
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	assert.Contains(t, strings.ToLower(res.Data), "unknown identity")
	assert.NotContains(t, res.Data, "tok-")
}

func TestListAvailableIdentities_NoTokens(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()
	i := configureTwoIdentities(t, srv.URL)

	res, err := i.Execute(context.Background(), mcp.ToolName("slackmcp_list_available_identites"), map[string]any{})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.NotContains(t, res.Data, "tok-work")
	assert.NotContains(t, res.Data, "tok-personal")
	assert.NotContains(t, res.Data, "access_token")
	assert.Contains(t, res.Data, "work")
	assert.Contains(t, res.Data, "personal")
	assert.Contains(t, res.Data, "Work")
	assert.Contains(t, res.Data, "Personal")
}

func TestReconfigure_RemovesStaleAndUpdatesTokens(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()
	i := configureTwoIdentities(t, srv.URL)

	// Drop personal; keep work and rotate its token.
	multi := i.(mcp.MultiIdentityIntegration)
	require.NoError(t, multi.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {Credentials: mcp.Credentials{"access_token": "tok-rotated"}, Metadata: map[string]string{"label": "Work"}},
	}))

	res, err := i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "personal",
		"channel_id":  "C1",
		"message":     "x",
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	assert.Contains(t, strings.ToLower(res.Data), "unknown identity")

	// list should only include work
	res, err = i.Execute(context.Background(), mcp.ToolName("slackmcp_list_available_identites"), nil)
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.Contains(t, res.Data, "work")
	assert.NotContains(t, res.Data, "personal")

	// Next proxied call must use the rotated bearer, not the old token.
	mu.Lock()
	calls = calls[:0]
	mu.Unlock()
	res, err = i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "work",
		"channel_id":  "C1",
		"message":     "after-rotate",
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.Contains(t, res.Data, "tok-rotated")

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, calls)
	last := calls[len(calls)-1]
	assert.Equal(t, "Bearer tok-rotated", last.auth)
	assert.NotEqual(t, "Bearer tok-work", last.auth)
	assert.NotContains(t, last.auth, "tok-work")
}

func TestReconfigure_TokenRotate_NextCallUsesNewBearer(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()
	i := configureTwoIdentities(t, srv.URL)

	res, err := i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "work",
		"channel_id":  "C1",
		"message":     "before",
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)

	multi := i.(mcp.MultiIdentityIntegration)
	require.NoError(t, multi.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work":     {Credentials: mcp.Credentials{"access_token": "tok-new-work"}, Metadata: map[string]string{"label": "Work"}},
		"personal": {Credentials: mcp.Credentials{"access_token": "tok-personal"}, Metadata: map[string]string{"label": "Personal"}},
	}))

	mu.Lock()
	calls = calls[:0]
	mu.Unlock()

	res, err = i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "work",
		"channel_id":  "C1",
		"message":     "after",
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.Contains(t, res.Data, "tok-new-work")
	assert.NotContains(t, res.Data, "tok-work")

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, calls)
	var sawNew, sawOld bool
	for _, c := range calls {
		if c.auth == "Bearer tok-new-work" {
			sawNew = true
		}
		if c.auth == "Bearer tok-work" {
			sawOld = true
		}
	}
	assert.True(t, sawNew, "expected Authorization Bearer tok-new-work on proxied call")
	assert.False(t, sawOld, "old token must not be sent after rotation")
}

func TestReconfigure_BaseURLChange_RoutesToNewEndpoint(t *testing.T) {
	var calls1, calls2 []recordedCall
	var mu1, mu2 sync.Mutex
	srv1 := fakeHostedSlackMCP(t, &calls1, &mu1)
	srv2 := fakeHostedSlackMCP(t, &calls2, &mu2)
	defer srv1.Close()
	defer srv2.Close()

	i := New()
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), i, &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"base_url": srv1.URL},
		Identities: map[string]mcp.IntegrationIdentity{
			"work": {Credentials: mcp.Credentials{"access_token": "tok-work"}, Metadata: map[string]string{"label": "Work"}},
		},
	}))

	res, err := i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "work",
		"channel_id":  "C1",
		"message":     "to-srv1",
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)

	mu1.Lock()
	n1Before := len(calls1)
	mu1.Unlock()
	mu2.Lock()
	n2Before := len(calls2)
	mu2.Unlock()
	require.Greater(t, n1Before, 0, "first call should hit endpoint 1")
	require.Equal(t, 0, n2Before, "endpoint 2 must be unused before base_url change")

	// Same identity token, new base_url — must rebuild remote and route to endpoint 2.
	require.NoError(t, mcp.ConfigureIntegration(context.Background(), i, &mcp.IntegrationConfig{
		Credentials: mcp.Credentials{"base_url": srv2.URL},
		Identities: map[string]mcp.IntegrationIdentity{
			"work": {Credentials: mcp.Credentials{"access_token": "tok-work"}, Metadata: map[string]string{"label": "Work"}},
		},
	}))

	res, err = i.Execute(context.Background(), mcp.ToolName("slackmcp_send_message"), map[string]any{
		"identity_id": "work",
		"channel_id":  "C1",
		"message":     "to-srv2",
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)

	mu1.Lock()
	n1After := len(calls1)
	mu1.Unlock()
	mu2.Lock()
	n2After := len(calls2)
	mu2.Unlock()

	assert.Equal(t, n1Before, n1After, "after base_url change, endpoint 1 must not receive further tool calls")
	assert.Greater(t, n2After, n2Before, "after base_url change, calls must hit endpoint 2")

	mu2.Lock()
	defer mu2.Unlock()
	require.NotEmpty(t, calls2)
	assert.Equal(t, "Bearer tok-work", calls2[len(calls2)-1].auth)
}

// closeTrackingRemote is a test double for per-identity remote clients.
type closeTrackingRemote struct {
	mu       sync.Mutex
	name     string
	closed   int
	token    string
	baseURL  string
	execs    int
	lastArgs map[string]any
}

func (m *closeTrackingRemote) Name() string { return m.name }
func (m *closeTrackingRemote) Configure(_ context.Context, creds mcp.Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = creds["access_token"]
	return nil
}
func (m *closeTrackingRemote) Tools() []mcp.ToolDefinition {
	return []mcp.ToolDefinition{{
		Name:        mcp.ToolName(internalRemotePrefix + "_slack_send_message"),
		Description: "Send a message",
		Parameters:  map[string]string{"channel_id": "channel id", "message": "message body"},
		Required:    []string{"channel_id", "message"},
	}}
}
func (m *closeTrackingRemote) Execute(_ context.Context, _ mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execs++
	m.lastArgs = args
	return mcp.JSONResult(map[string]any{"ok": true, "token": m.token})
}
func (m *closeTrackingRemote) Healthy(_ context.Context) bool { return true }
func (m *closeTrackingRemote) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed++
	return nil
}
func (m *closeTrackingRemote) closeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestReconfigure_ClosesRemovedAndReplacedRemotesNotReused(t *testing.T) {
	s := New().(*slackmcp)
	var created []*closeTrackingRemote
	s.newRemote = func(baseURL string) mcp.Integration {
		r := &closeTrackingRemote{name: "mock", baseURL: baseURL}
		created = append(created, r)
		return r
	}

	require.NoError(t, s.Configure(context.Background(), mcp.Credentials{"base_url": "https://mcp-a.test"}))
	require.NoError(t, s.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work":     {Credentials: mcp.Credentials{"access_token": "tok-work"}},
		"personal": {Credentials: mcp.Credentials{"access_token": "tok-personal"}},
	}))
	require.Len(t, created, 2)
	work1, personal1 := created[0], created[1]
	// Deterministic sorted identity order is personal, work — re-resolve by token.
	for _, r := range created {
		switch r.token {
		case "tok-work":
			work1 = r
		case "tok-personal":
			personal1 = r
		}
	}
	assert.Equal(t, 0, work1.closeCount())
	assert.Equal(t, 0, personal1.closeCount())

	// Drop personal, keep work unchanged → close personal only.
	require.NoError(t, s.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {Credentials: mcp.Credentials{"access_token": "tok-work"}},
	}))
	assert.Equal(t, 0, work1.closeCount(), "reused remote must not be closed")
	assert.Equal(t, 1, personal1.closeCount(), "removed identity remote must be closed")
	require.Len(t, created, 2, "no new remote when work token+base unchanged")

	// Rotate work token → close previous work remote, create a new one.
	require.NoError(t, s.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {Credentials: mcp.Credentials{"access_token": "tok-work-2"}},
	}))
	require.Len(t, created, 3)
	work2 := created[2]
	assert.Equal(t, 1, work1.closeCount(), "token-replaced remote must be closed")
	assert.Equal(t, 0, work2.closeCount())
	assert.Equal(t, "tok-work-2", work2.token)

	// base_url change with same token → rebuild and close prior remote.
	require.NoError(t, s.Configure(context.Background(), mcp.Credentials{"base_url": "https://mcp-b.test"}))
	require.NoError(t, s.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {Credentials: mcp.Credentials{"access_token": "tok-work-2"}},
	}))
	require.Len(t, created, 4)
	work3 := created[3]
	assert.Equal(t, 1, work2.closeCount(), "base_url-replaced remote must be closed")
	assert.Equal(t, 0, work3.closeCount())
	assert.Equal(t, "https://mcp-b.test", work3.baseURL)

	// Unchanged reconfigure → reuse, no additional close.
	require.NoError(t, s.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {Credentials: mcp.Credentials{"access_token": "tok-work-2"}},
	}))
	require.Len(t, created, 4)
	assert.Equal(t, 0, work3.closeCount(), "unchanged identity must keep remote open")
}

func TestHealthy_AtLeastOneIdentity(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()
	i := configureTwoIdentities(t, srv.URL)
	assert.True(t, i.Healthy(context.Background()))

	// No identities -> unhealthy
	empty := New()
	require.NoError(t, empty.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL}))
	assert.False(t, empty.Healthy(context.Background()))
}

func TestUnknownTool(t *testing.T) {
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()
	i := configureTwoIdentities(t, srv.URL)
	res, err := i.Execute(context.Background(), mcp.ToolName("slackmcp_nope"), map[string]any{"identity_id": "work"})
	require.NoError(t, err)
	require.True(t, res.IsError)
	assert.Contains(t, res.Data, "unknown tool")
}

func TestDynamicRoutingParity_NoStaticDispatchMap(t *testing.T) {
	// Document that routing is dynamic: every proxied tool name from Tools()
	// is executable when identity_id is valid, without a static dispatch table.
	var calls []recordedCall
	var mu sync.Mutex
	srv := fakeHostedSlackMCP(t, &calls, &mu)
	defer srv.Close()
	i := configureTwoIdentities(t, srv.URL)

	for _, td := range i.Tools() {
		if td.Name == mcp.ToolName(listIdentitiesTool) {
			continue
		}
		args := map[string]any{"identity_id": "work"}
		// Fill required-ish args for known tools
		switch td.Name {
		case "slackmcp_send_message":
			args["channel_id"] = "C1"
			args["message"] = "m"
		case "slackmcp_search_public":
			args["query"] = "q"
		case "slackmcp_read_channel":
			// only on personal; use personal identity
			args["identity_id"] = "personal"
			args["channel_id"] = "C9"
		}
		res, err := i.Execute(context.Background(), td.Name, args)
		require.NoError(t, err, string(td.Name))
		require.False(t, res.IsError, "%s: %s", td.Name, res.Data)
	}
}

func TestConfigureIdentities_RejectsUnreachableHostedMCP(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	defer upstream.Close()

	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"base_url": upstream.URL}))
	multi := integration.(mcp.MultiIdentityIntegration)

	err := multi.ConfigureIdentities(context.Background(), map[string]mcp.IntegrationIdentity{
		"work": {Credentials: mcp.Credentials{"access_token": "revoked-token"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy")
}
