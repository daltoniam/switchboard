package remotemcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// versionedServer serves MCP only at /mcp/v1, like Ashby's hosted server.
func versionedServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "versioned", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{
		Name: "lookup",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string", "description": "Record name"}},
			"required":   []any{"name"},
		},
	}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "summary"}}, StructuredContent: map[string]any{"found": true}}, nil
	})
	stream := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp/v1", stream)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNewWithOptions_UsesEndpointPathAndStructuredContent(t *testing.T) {
	srv := versionedServer(t)
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{name: "structured", opts: Options{EndpointPath: "/mcp/v1", Structured: true}, want: `{"found":true}`},
		{name: "text", opts: Options{EndpointPath: "/mcp/v1"}, want: "summary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := NewWithOptions("test", srv.URL, tt.opts)
			t.Cleanup(func() { _ = integration.(io.Closer).Close() })
			require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"access_token": "t"}))
			result, err := integration.Execute(context.Background(), "test_lookup", map[string]any{"name": "Jordan"})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, tt.want, result.Data)
		})
	}
}

func TestNewWithOptions_DefaultEndpointIsMCP(t *testing.T) {
	srv := versionedServer(t)
	integration := NewWithOptions("test", srv.URL, Options{})
	t.Cleanup(func() { _ = integration.(io.Closer).Close() })
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"access_token": "t"}))
	assert.False(t, integration.Healthy(context.Background()), "the server has nothing at /mcp")
}

func TestListTools_FetchesWithContextAndCaches(t *testing.T) {
	srv := versionedServer(t)
	integration := NewWithOptions("test", srv.URL, Options{EndpointPath: "/mcp/v1"})
	t.Cleanup(func() { _ = integration.(io.Closer).Close() })
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"access_token": "t"}))
	lister, ok := integration.(ToolLister)
	require.True(t, ok)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := lister.ListTools(canceled)
	require.Error(t, err, "a canceled context must stop the fetch")

	tools, err := lister.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, mcp.ToolName("test_lookup"), tools[0].Name)
	assert.Equal(t, "Record name", tools[0].Parameters["name"])
	assert.Equal(t, []string{"name"}, tools[0].Required)

	srv.Close()
	cached, err := lister.ListTools(context.Background())
	require.NoError(t, err, "a fetched list is cached")
	assert.Equal(t, tools, cached)
	assert.Equal(t, tools, integration.Tools())
}

// ashbyStyleDiscovery models a server whose MCP endpoint is not /mcp: it
// publishes protected resource metadata only at the origin, naming the
// versioned endpoint as the resource.
func ashbyStyleDiscovery(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issuer := srv.URL + "/oidc"
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource":
			json.NewEncoder(w).Encode(map[string]any{"resource": srv.URL + "/mcp/v1", "authorization_servers": []string{issuer}})
		case "/.well-known/oauth-authorization-server/oidc":
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 issuer,
				"authorization_endpoint": issuer + "/auth",
				"token_endpoint":         issuer + "/token",
				"registration_endpoint":  issuer + "/reg",
			})
		case "/oidc/reg":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"client_id": "versioned-client"})
		default:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>app shell</html>"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOAuth_DiscoverAcceptsExpectedVersionedResource(t *testing.T) {
	srv := ashbyStyleDiscovery(t)

	_, err := discoverOAuth(srv.URL)
	require.Error(t, err, "without the expected resource a versioned endpoint is rejected")
	assert.Contains(t, err.Error(), "protected resource metadata is for")

	meta, err := discoverOAuth(srv.URL, srv.URL+"/mcp/v1")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/oidc/token", meta.TokenEndpoint)

	_, err = discoverOAuth(srv.URL, srv.URL+"/other")
	require.Error(t, err, "a resource other than the expected one is still rejected")
}

func TestOAuth_StartBindsVersionedResourceAndPrompt(t *testing.T) {
	srv := ashbyStyleDiscovery(t)
	authorizeURL, err := StartOAuth("versioned-test", srv.URL, "http://localhost/callback", OAuthOptions{
		Scope:    "openid mcp offline_access",
		Resource: srv.URL + "/mcp/v1",
		Prompt:   "consent",
	})
	require.NoError(t, err)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/oidc/auth", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	assert.Equal(t, "openid mcp offline_access", parsed.Query().Get("scope"))
	assert.Equal(t, srv.URL+"/mcp/v1", parsed.Query().Get("resource"))
	assert.Equal(t, "consent", parsed.Query().Get("prompt"))
	assert.Equal(t, "versioned-client", parsed.Query().Get("client_id"))
}

func TestOAuth_StartOmitsPromptByDefault(t *testing.T) {
	srv := ashbyStyleDiscovery(t)
	authorizeURL, err := StartOAuth("versioned-default", srv.URL, "http://localhost/callback", OAuthOptions{Resource: srv.URL + "/mcp/v1"})
	require.NoError(t, err)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	assert.False(t, parsed.Query().Has("prompt"))
}

func TestRefreshGrant_DiscoversVersionedResource(t *testing.T) {
	var srv *httptest.Server
	var resource string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issuer := srv.URL + "/oidc"
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource":
			json.NewEncoder(w).Encode(map[string]any{"resource": srv.URL + "/mcp/v1", "authorization_servers": []string{issuer}})
		case "/.well-known/oauth-authorization-server/oidc":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/auth", "token_endpoint": issuer + "/token"})
		case "/oidc/token":
			require.NoError(t, r.ParseForm())
			resource = r.PostForm.Get("resource")
			json.NewEncoder(w).Encode(map[string]any{"access_token": "renewed", "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	creds := mcp.Credentials{CredKeyClientID: "client", CredKeyRefreshToken: "refresh"}
	grant, err := refreshGrant(context.Background(), RenewalTarget{Integration: "ashby", ServerURL: srv.URL, Resource: srv.URL + "/mcp/v1"}, creds, renewalNow)
	require.NoError(t, err)
	assert.Equal(t, "renewed", grant.accessToken)
	assert.Equal(t, srv.URL+"/mcp/v1", resource)
}

// A caller whose context ends must not wait for another caller's slow
// connection attempt to finish.
func TestListTools_CanceledWhileAnotherConnectIsInFlight(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "slow", Version: "1"}, nil)
	stream := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		stream.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })

	integration := New("test", srv.URL)
	t.Cleanup(func() { _ = integration.(io.Closer).Close() })
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"access_token": "t"}))
	var callers atomic.Int32
	secondAtGate := make(chan struct{})
	integration.(*remote).beforeGate = func() {
		if callers.Add(1) == 2 {
			close(secondAtGate)
		}
	}

	slowCtx, cancelSlow := context.WithCancel(context.Background())
	t.Cleanup(cancelSlow)
	go func() { _, _ = integration.(ToolLister).ListTools(slowCtx) }()
	<-entered

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := integration.(ToolLister).ListTools(ctx)
		done <- err
	}()
	select {
	case <-secondAtGate:
	case <-time.After(5 * time.Second):
		t.Fatal("second caller never reached the connection gate")
	}
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		releaseOnce.Do(func() { close(release) })
		t.Fatal("ListTools ignored its canceled context while another connection was in flight")
	}
}

// A connection that finishes after Configure replaced the token is not
// kept: the next call connects with the new token.
func TestConnect_DiscardsSessionOpenedWithReplacedToken(t *testing.T) {
	srv := versionedServer(t)
	r := NewWithOptions("test", srv.URL, Options{EndpointPath: "/mcp/v1"}).(*remote)
	t.Cleanup(func() { _ = r.Close() })
	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "old"}))
	r.afterDial = func() {
		r.afterDial = nil
		require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "new"}))
	}
	session, err := r.connect(context.Background())
	require.NoError(t, err)
	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Same(t, r.session, session)
	assert.Equal(t, "new", r.sessionToken)
}

func TestConnect_CloseDuringConnectKeepsAdapterClosed(t *testing.T) {
	srv := versionedServer(t)
	r := NewWithOptions("test", srv.URL, Options{EndpointPath: "/mcp/v1"}).(*remote)
	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "t"}))
	r.afterDial = func() {
		r.afterDial = nil
		require.NoError(t, r.Close())
	}
	_, err := r.connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed while connecting")
	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Nil(t, r.session)
}

func TestConnect_ConfigureThenCloseDuringConnectKeepsAdapterClosed(t *testing.T) {
	srv := versionedServer(t)
	r := NewWithOptions("test", srv.URL, Options{EndpointPath: "/mcp/v1"}).(*remote)
	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "a"}))
	r.afterDial = func() {
		r.afterDial = nil
		require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "b"}))
		require.NoError(t, r.Close())
	}
	_, err := r.connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed while connecting")
	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Nil(t, r.session, "no session may be published after Close")
}

// A call already waiting for the connection gate when Close runs must not
// open a session afterwards.
func TestConnect_QueuedCallDoesNotReopenAfterClose(t *testing.T) {
	srv := versionedServer(t)
	r := NewWithOptions("test", srv.URL, Options{EndpointPath: "/mcp/v1"}).(*remote)
	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "t"}))
	queued := make(chan error, 1)
	waiting := make(chan struct{})
	r.afterDial = func() {
		r.afterDial = nil
		r.beforeGate = func() { close(waiting) }
		go func() {
			_, err := r.connect(context.Background())
			queued <- err
		}()
		<-waiting
		assert.NoError(t, r.Close())
	}
	_, err := r.connect(context.Background())
	require.Error(t, err)
	select {
	case err := <-queued:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "closed while connecting")
	case <-time.After(5 * time.Second):
		t.Fatal("queued connect did not return")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Nil(t, r.session, "no session may be published after Close")
}

// A dial rejected because it used a token that Configure has since replaced
// is retried with the new token.
func TestConnect_RetriesRejectedDialAfterTokenChange(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "auth", Version: "1"}, nil)
	stream := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	var r *remote
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer new" {
			once.Do(func() {
				assert.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "new"}))
			})
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		stream.ServeHTTP(w, req)
	}))
	t.Cleanup(srv.Close)
	r = New("test", srv.URL).(*remote)
	t.Cleanup(func() { _ = r.Close() })
	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{"access_token": "old"}))

	session, err := r.connect(context.Background())
	require.NoError(t, err)
	require.NotNil(t, session)
	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Equal(t, "new", r.sessionToken)
}
