package panther

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureToken = "panther-oauth-token"

// remoteFixture is a fake Panther Remote MCP server at /mcp that requires
// the bearer token and annotates its tools the way Panther's local server
// does (read-only or destructive hints).
type remoteFixture struct {
	server    *httptest.Server
	mu        sync.Mutex
	calls     []string
	listCalls int
	// mcpRequests counts every request to /mcp, including rejected ones.
	mcpRequests int
	// listGate, when set, holds tools/list until it is closed.
	listGate chan struct{}
	// oauth, when set, serves every path other than /mcp.
	oauth http.Handler
}

func newRemoteFixture(t *testing.T) *remoteFixture {
	t.Helper()
	f := &remoteFixture{}
	notDestructive := false
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-panther", Version: "1"}, nil)
	tools := []*mcpsdk.Tool{
		{Name: "list_alerts", Description: "List alerts.", Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true}},
		{Name: "query_data_lake", Description: "Run SQL.", Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true}},
		{Name: "add_alert_comment", Description: "Comment.", Annotations: &mcpsdk.ToolAnnotations{DestructiveHint: &notDestructive}},
		{Name: "update_alert_status", Description: "Update status.", Annotations: &mcpsdk.ToolAnnotations{}},
	}
	for _, tool := range tools {
		tool.InputSchema = map[string]any{"type": "object", "properties": map[string]any{"alert_id": map[string]any{"type": "string", "description": "Alert id"}}}
		server.AddTool(tool, func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			f.mu.Lock()
			f.calls = append(f.calls, req.Params.Name)
			f.mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "{}"}}, StructuredContent: map[string]any{"tool": req.Params.Name}}, nil
		})
	}
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != mcpPath {
			if f.oauth != nil {
				f.oauth.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		f.mcpRequests++
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+fixtureToken {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"tools/list"`) {
			f.mu.Lock()
			f.listCalls++
			gate := f.listGate
			f.mu.Unlock()
			if gate != nil {
				select {
				case <-gate:
				case <-r.Context().Done():
					return
				}
			}
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *remoteFixture) snapshot() ([]string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...), f.listCalls
}

func signedIn(t *testing.T, f *remoteFixture) *panther {
	t.Helper()
	p := New().(*panther)
	t.Cleanup(func() { assert.NoError(t, p.Close()) })
	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": f.server.URL, "mcp_access_token": fixtureToken}))
	return p
}

func TestNew(t *testing.T) {
	p := New()
	assert.Equal(t, "panther", p.Name())
	assert.Equal(t, []string{"instance_url"}, p.(mcp.PlainTextCredentials).PlainTextKeys())
	assert.ElementsMatch(t, []string{"api_token", "mcp_access_token"}, p.(mcp.OptionalCredentials).OptionalKeys())
}

func TestConfigure_Failures(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  string
	}{
		{name: "missing instance", creds: mcp.Credentials{"api_token": "t"}, want: "instance_url is required"},
		{name: "insecure instance", creds: mcp.Credentials{"instance_url": "http://acme.runpanther.net", "api_token": "t"}, want: "must use https"},
		{name: "no credential", creds: mcp.Credentials{"instance_url": "https://acme.runpanther.net"}, want: "api_token (API token) or mcp_access_token"},
		{name: "blank credentials", creds: mcp.Credentials{"instance_url": "https://acme.runpanther.net", "api_token": " ", "mcp_access_token": " "}, want: "is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			err := p.Configure(context.Background(), tt.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestConfigure_APITokenMode(t *testing.T) {
	p := New().(*panther)
	t.Cleanup(func() { _ = p.Close() })
	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": "https://acme.runpanther.net", "api_token": " tok "}))
	require.NotNil(t, p.native)
	assert.Equal(t, "tok", p.native.token)
	assert.Equal(t, "https://api.acme.runpanther.net", p.native.inst.apiBase)
	assert.Nil(t, p.hosted)
}

func TestTools_Metadata(t *testing.T) {
	tools := New().Tools()
	require.NotEmpty(t, tools)
	seen := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		assert.True(t, strings.HasPrefix(string(tool.Name), "panther_"), tool.Name)
		assert.NotEmpty(t, tool.Description, tool.Name)
		assert.False(t, seen[tool.Name], "duplicate %s", tool.Name)
		seen[tool.Name] = true
		assert.True(t, tool.Access.Declared(), "%s has no access class", tool.Name)
		for _, required := range tool.Required {
			assert.Contains(t, tool.Parameters, required, "%s requires undeclared %s", tool.Name, required)
		}
		assert.NotContains(t, tool.Description, "\u2014", "%s uses an em dash", tool.Name)
	}
}

func TestTools_StartHere(t *testing.T) {
	for _, name := range []mcp.ToolName{"panther_list_alerts", "panther_list_rules", "panther_list_databases", "panther_list_log_sources"} {
		tool, ok := findTool(New().Tools(), name)
		require.True(t, ok, name)
		assert.Contains(t, tool.Description, "Start here", name)
	}
}

func TestTools_SearchKeywords(t *testing.T) {
	var all strings.Builder
	for _, tool := range New().Tools() {
		all.WriteString(strings.ToLower(tool.Description))
		all.WriteString(" ")
	}
	for _, keyword := range []string{"siem", "security alert", "detection", "rule", "log search", "data lake", "threat hunting", "incident"} {
		assert.Contains(t, all.String(), keyword)
	}
}

func TestTools_AccessClassification(t *testing.T) {
	writes := map[mcp.ToolName]bool{
		"panther_update_alert_status":    true,
		"panther_assign_alerts":          true,
		"panther_add_alert_comment":      true,
		"panther_set_detection_enabled":  true,
		"panther_cancel_data_lake_query": true,
	}
	counts := map[mcp.ToolAccess]int{}
	for _, tool := range New().Tools() {
		counts[tool.Access]++
		if writes[tool.Name] {
			assert.Equal(t, mcp.AccessWrite, tool.Access, tool.Name)
			continue
		}
		assert.Equal(t, mcp.AccessRead, tool.Access, tool.Name)
	}
	assert.Equal(t, map[mcp.ToolAccess]int{mcp.AccessRead: 37, mcp.AccessWrite: 5}, counts)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range nativeTools {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range nativeTools {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestTools_ReturnsDeepCopy(t *testing.T) {
	p := New()
	tools := p.Tools()
	tools[0].Parameters["injected"] = "x"
	tools[0].Description = "changed"
	again := p.Tools()
	assert.NotContains(t, again[0].Parameters, "injected")
	assert.NotEqual(t, "changed", again[0].Description)
}

func findTool(tools []mcp.ToolDefinition, name mcp.ToolName) (mcp.ToolDefinition, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return mcp.ToolDefinition{}, false
}

func TestSignIn_ToolsComeFromServerWithAccess(t *testing.T) {
	f := newRemoteFixture(t)
	p := signedIn(t, f)

	require.True(t, p.Healthy(context.Background()), "health reads the tool list")
	tools := p.Tools()
	want := map[mcp.ToolName]mcp.ToolAccess{
		"panther_list_alerts":         mcp.AccessRead,
		"panther_query_data_lake":     mcp.AccessRead,
		"panther_add_alert_comment":   mcp.AccessWrite,
		"panther_update_alert_status": mcp.AccessDestructive,
	}
	got := map[mcp.ToolName]mcp.ToolAccess{}
	for _, tool := range tools {
		got[tool.Name] = tool.Access
	}
	assert.Equal(t, want, got)
	entry, ok := findTool(tools, hostedEntryTool)
	require.True(t, ok)
	assert.Contains(t, entry.Description, "Start here")
	assert.Equal(t, "Alert id", entry.Parameters["alert_id"])
	_, isNative := findTool(tools, "panther_list_rules")
	assert.False(t, isNative, "sign-in mode never mixes in the native catalog")
}

func TestSignIn_ExecuteForwardsListedToolsOnly(t *testing.T) {
	f := newRemoteFixture(t)
	p := signedIn(t, f)

	result, err := p.Execute(context.Background(), "panther_list_alerts", map[string]any{"alert_id": "a1"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.JSONEq(t, `{"tool":"list_alerts"}`, result.Data)

	result, err = p.Execute(context.Background(), "panther_list_rules", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool: panther_list_rules")
	assert.Contains(t, result.Data, "consent")

	calls, _ := f.snapshot()
	assert.Equal(t, []string{"list_alerts"}, calls, "a tool the server did not list never reaches it")
}

func TestSignIn_ToolsWaitForFirstList(t *testing.T) {
	f := newRemoteFixture(t)
	p := signedIn(t, f)
	assert.Len(t, p.Tools(), 4, "search indexes the first list, so Tools waits for it")
	_, lists := f.snapshot()
	p.Tools()
	_, again := f.snapshot()
	assert.Equal(t, lists, again, "a known list is not fetched again")
}

func TestSignIn_ToolsWaitIsBounded(t *testing.T) {
	previous := firstListWait
	firstListWait = 50 * time.Millisecond
	t.Cleanup(func() { firstListWait = previous })
	f := newRemoteFixture(t)
	gate := make(chan struct{})
	f.mu.Lock()
	f.listGate = gate
	f.mu.Unlock()
	p := signedIn(t, f)

	done := make(chan []mcp.ToolDefinition, 1)
	go func() { done <- p.Tools() }()
	select {
	case tools := <-done:
		assert.Empty(t, tools, "a slow server cannot stall search")
	case <-time.After(5 * time.Second):
		t.Fatal("Tools waited past firstListWait")
	}
	f.mu.Lock()
	f.listGate = nil
	f.mu.Unlock()
	close(gate)
	require.Eventually(t, func() bool { return len(p.Tools()) == 4 }, 10*time.Second, 20*time.Millisecond)
}

func TestSignIn_CompactionNeverApplies(t *testing.T) {
	f := newRemoteFixture(t)
	p := signedIn(t, f)
	_, ok := p.CompactSpec("panther_list_alerts")
	assert.False(t, ok)
	_, ok = p.MaxBytes("panther_list_alerts")
	assert.False(t, ok)

	native, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {})
	_, ok = native.CompactSpec("panther_list_alerts")
	assert.True(t, ok)
}

func TestModeSwitching(t *testing.T) {
	f := newRemoteFixture(t)
	p := signedIn(t, f)
	require.NotNil(t, p.hosted)
	hosted := p.hosted

	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": f.server.URL, "api_token": "tok", "mcp_access_token": fixtureToken}))
	assert.NotNil(t, p.native, "an API token wins over a saved sign-in")
	assert.Nil(t, p.hosted)
	assert.Error(t, hosted.lifetime.Err(), "the replaced session is ended")
	_, ok := findTool(p.Tools(), "panther_list_rules")
	assert.True(t, ok)

	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": f.server.URL, "mcp_access_token": fixtureToken}))
	assert.Nil(t, p.native)
	assert.NotNil(t, p.hosted)
}

func TestSignIn_FailureKeepsWorkingAPIToken(t *testing.T) {
	p, _ := newNativeAdapter(t, func(w http.ResponseWriter, _ recorded) {})
	native := p.native
	p.newRemote = func(string) mcp.Integration { return failingRemote{} }
	err := p.Configure(context.Background(), mcp.Credentials{"instance_url": "https://acme.runpanther.net", "mcp_access_token": "t"})
	require.Error(t, err)
	assert.Same(t, native, p.native, "a failed sign-in leaves API token mode active")
	assert.Nil(t, p.hosted)
}

type failingRemote struct{ mcp.Integration }

func (failingRemote) Configure(context.Context, mcp.Credentials) error {
	return assert.AnError
}

func TestSignIn_NewTokenRereadsTools(t *testing.T) {
	f := newRemoteFixture(t)
	p := signedIn(t, f)
	require.True(t, p.Healthy(context.Background()))
	hosted := p.hosted
	_, known := hosted.cachedTools()
	require.True(t, known)

	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": f.server.URL, "mcp_access_token": fixtureToken}))
	_, known = hosted.cachedTools()
	assert.True(t, known, "the same login keeps its tool list")

	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": f.server.URL, "mcp_access_token": "other-token"}))
	assert.Same(t, hosted, p.hosted, "the same instance keeps its session")
	_, known = hosted.cachedTools()
	assert.False(t, known, "a new login may carry different consent")
}

func TestClose_IsIdempotentAndEndsSession(t *testing.T) {
	f := newRemoteFixture(t)
	p := signedIn(t, f)
	hosted := p.hosted
	require.NoError(t, p.Close())
	require.NoError(t, p.Close())
	assert.Error(t, hosted.lifetime.Err())
	assert.Equal(t, cloneTools(nativeTools), p.Tools(), "a closed adapter shows the native catalog like an unconfigured one")
	result, err := p.Execute(context.Background(), "panther_list_alerts", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestClose_WaitsForBackgroundFetch(t *testing.T) {
	f := newRemoteFixture(t)
	gate := make(chan struct{})
	f.mu.Lock()
	f.listGate = gate
	f.mu.Unlock()
	p := signedIn(t, f)
	p.hosted.startFetch()
	require.Eventually(t, func() bool { _, lists := f.snapshot(); return lists > 0 }, 5*time.Second, 10*time.Millisecond)

	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		close(gate)
		t.Fatal("Close did not cancel the background tool fetch")
	}
	close(gate)
}

// sharedStore is a config service that only supports shared updates, which
// is all renewal uses.
type sharedStore struct {
	mcp.ConfigService
	mu    sync.Mutex
	creds mcp.Credentials
}

func (s *sharedStore) UpdateIntegrationShared(_ context.Context, name string, update func(*mcp.IntegrationConfig) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name != integrationName {
		panic("unexpected integration " + name)
	}
	ic := &mcp.IntegrationConfig{Enabled: true, Credentials: maps.Clone(s.creds)}
	if err := update(ic); err != nil {
		return err
	}
	s.creds = ic.Credentials
	return nil
}

func (s *sharedStore) saved() mcp.Credentials {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.creds)
}

func TestSignIn_RenewsTokenBeforeExpiry(t *testing.T) {
	f := newRemoteFixture(t)
	var mu sync.Mutex
	var refreshes []url.Values
	f.oauth = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := f.server.URL
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource/mcp":
			_ = json.NewEncoder(w).Encode(map[string]any{"resource": origin + mcpPath, "authorization_servers": []string{origin}})
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": origin, "authorization_endpoint": origin + "/authorize", "token_endpoint": origin + "/token"})
		case "/token":
			_ = r.ParseForm()
			mu.Lock()
			refreshes = append(refreshes, r.PostForm)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": fixtureToken, "refresh_token": "refresh-2", "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	})

	expiring := mcp.Credentials{
		"instance_url":                f.server.URL,
		remotemcp.CredKeyAccessToken:  "old-token",
		remotemcp.CredKeyRefreshToken: "refresh-1",
		remotemcp.CredKeyTokenExpiry:  time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
		remotemcp.CredKeyTokenIssued:  time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		remotemcp.CredKeyClientID:     "client-1",
	}
	store := &sharedStore{creds: maps.Clone(expiring)}
	p := New().(*panther)
	t.Cleanup(func() { _ = p.Close() })
	SetConfigService(p, store)
	require.NoError(t, p.Configure(context.Background(), expiring))

	result, err := p.Execute(context.Background(), "panther_list_alerts", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, refreshes, 1, "the expiring login was renewed once")
	assert.Equal(t, "refresh_token", refreshes[0].Get("grant_type"))
	assert.Equal(t, "refresh-1", refreshes[0].Get("refresh_token"))
	assert.Equal(t, f.server.URL+mcpPath, refreshes[0].Get("resource"))
	saved := store.saved()
	assert.Equal(t, fixtureToken, saved[remotemcp.CredKeyAccessToken])
	assert.Equal(t, "refresh-2", saved[remotemcp.CredKeyRefreshToken])
}

func TestSignIn_ExecuteBacksOffAfterFailedListing(t *testing.T) {
	f := newRemoteFixture(t)
	p := New().(*panther)
	t.Cleanup(func() { assert.NoError(t, p.Close()) })
	require.NoError(t, p.Configure(context.Background(), mcp.Credentials{"instance_url": f.server.URL, "mcp_access_token": "revoked-token"}))

	result, err := p.Execute(context.Background(), "panther_list_alerts", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Data, "list Remote MCP tools")
	f.mu.Lock()
	first := f.mcpRequests
	f.mu.Unlock()
	require.Positive(t, first)

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			again, err := p.Execute(context.Background(), "panther_list_alerts", nil)
			assert.NoError(t, err)
			assert.True(t, again.IsError)
		}()
	}
	wg.Wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, first, f.mcpRequests, "a failed listing is not retried before toolRetryAfter")
}

func TestSignIn_ToolsRenewExpiredLoginBeforeListing(t *testing.T) {
	f := newRemoteFixture(t)
	f.oauth = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := f.server.URL
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource/mcp":
			_ = json.NewEncoder(w).Encode(map[string]any{"resource": origin + mcpPath, "authorization_servers": []string{origin}})
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": origin, "authorization_endpoint": origin + "/authorize", "token_endpoint": origin + "/token"})
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": fixtureToken, "refresh_token": "refresh-2", "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	})
	expired := mcp.Credentials{
		"instance_url":                f.server.URL,
		remotemcp.CredKeyAccessToken:  "expired-token",
		remotemcp.CredKeyRefreshToken: "refresh-1",
		remotemcp.CredKeyTokenExpiry:  time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		remotemcp.CredKeyClientID:     "client-1",
	}
	p := New().(*panther)
	t.Cleanup(func() { _ = p.Close() })
	SetConfigService(p, &sharedStore{creds: maps.Clone(expired)})
	require.NoError(t, p.Configure(context.Background(), expired))

	assert.Len(t, p.Tools(), 4, "search lists tools with the renewed login")
}
