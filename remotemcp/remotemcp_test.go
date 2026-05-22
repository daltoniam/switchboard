package remotemcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New("test", "https://example.com")
	require.NotNil(t, i)
	assert.Equal(t, "test", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New("test", "https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "tok123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New("test", "https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_EmptyToken(t *testing.T) {
	i := New("test", "https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_TokenChange_ResetsSession(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	err := r.Configure(context.Background(), mcp.Credentials{"access_token": "tok1"})
	assert.NoError(t, err)
	assert.Equal(t, "tok1", r.token)

	err = r.Configure(context.Background(), mcp.Credentials{"access_token": "tok2"})
	assert.NoError(t, err)
	assert.Equal(t, "tok2", r.token)
	assert.False(t, r.toolsFetched)
	assert.Nil(t, r.cachedTools)
}

func TestConfigure_SameToken_NoReset(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	err := r.Configure(context.Background(), mcp.Credentials{"access_token": "tok1"})
	assert.NoError(t, err)

	r.toolsFetched = true
	r.cachedTools = []mcp.ToolDefinition{{Name: mcp.ToolName("test_foo")}}

	err = r.Configure(context.Background(), mcp.Credentials{"access_token": "tok1"})
	assert.NoError(t, err)
	assert.True(t, r.toolsFetched)
	assert.Len(t, r.cachedTools, 1)
}

func TestBearerTransport(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	bt := &bearerTransport{token: "mytoken"}
	client := &http.Client{Transport: bt}
	_, err := client.Get(srv.URL)
	assert.NoError(t, err)
	assert.Equal(t, "Bearer mytoken", gotAuth)
}

func TestConvertTools(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Issue title",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Issue body",
			},
		},
		"required": []any{"title"},
	}

	// Simulate what the MCP SDK returns: InputSchema is typed as json.RawMessage
	// but in practice the SDK unmarshals to the struct. We test via our own toMap path.
	schemaJSON, err := json.Marshal(schema)
	require.NoError(t, err)

	var rawSchema json.RawMessage
	err = json.Unmarshal(schemaJSON, &rawSchema)
	require.NoError(t, err)

	params := extractParams(rawSchema)
	assert.Equal(t, "Issue title", params["title"])
	assert.Equal(t, "Issue body", params["body"])

	required := extractRequired(rawSchema)
	assert.Equal(t, []string{"title"}, required)
}

func TestConvertTools_EmptySchema(t *testing.T) {
	params := extractParams(nil)
	assert.Empty(t, params)

	required := extractRequired(nil)
	assert.Nil(t, required)
}

func TestConvertResult_Nil(t *testing.T) {
	result := convertResult(nil)
	assert.True(t, result.IsError)
	assert.Equal(t, "no result", result.Data)
}

func TestToMap_Direct(t *testing.T) {
	m := map[string]any{"foo": "bar"}
	result, ok := toMap(m)
	assert.True(t, ok)
	assert.Equal(t, "bar", result["foo"])
}

func TestToMap_StructToMap(t *testing.T) {
	type s struct {
		Foo string `json:"foo"`
	}
	result, ok := toMap(s{Foo: "bar"})
	assert.True(t, ok)
	assert.Equal(t, "bar", result["foo"])
}

func TestToMap_InvalidInput(t *testing.T) {
	_, ok := toMap(func() {})
	assert.False(t, ok)
}

func TestServerURL(t *testing.T) {
	i := New("test", "https://example.com")
	assert.Equal(t, "https://example.com", ServerURL(i))
}

func TestServerURL_NonRemote(t *testing.T) {
	url := ServerURL(&mockIntegration{})
	assert.Equal(t, "", url)
}

func TestHealthy_NoToken(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	assert.False(t, r.Healthy(context.Background()))
}

func TestExecute_NoConnection(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://invalid.example.com"}
	result, err := r.Execute(context.Background(), "test_foo", nil)
	assert.NoError(t, err)
	assert.True(t, result.IsError)
}

type mockIntegration struct{}

func (m *mockIntegration) Name() string                                         { return "mock" }
func (m *mockIntegration) Configure(_ context.Context, _ mcp.Credentials) error { return nil }
func (m *mockIntegration) Tools() []mcp.ToolDefinition                          { return nil }
func (m *mockIntegration) Execute(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
	return nil, nil
}
func (m *mockIntegration) Healthy(_ context.Context) bool { return false }

func TestOAuth_RandomString(t *testing.T) {
	s1 := randomString(32)
	s2 := randomString(32)
	assert.Len(t, s1, 32)
	assert.Len(t, s2, 32)
	assert.NotEqual(t, s1, s2)
}

func TestOAuth_PkceChallenge(t *testing.T) {
	verifier := "test-verifier-string"
	challenge := pkceChallenge(verifier)
	assert.NotEmpty(t, challenge)
	assert.NotEqual(t, verifier, challenge)
}

func TestMCPResourceURL(t *testing.T) {
	assert.Equal(t, "https://api.smith.langchain.com/mcp", mcpResourceURL("https://api.smith.langchain.com"))
	assert.Equal(t, "https://api.smith.langchain.com/mcp", mcpResourceURL("https://api.smith.langchain.com/"))
	assert.Equal(t, "https://api.smith.langchain.com/mcp", mcpResourceURL("https://api.smith.langchain.com/mcp"))
	assert.Equal(t, "https://mcp.linear.app/mcp", mcpResourceURL("https://mcp.linear.app"))
}

func TestOAuth_StartOAuth_IncludesResource(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"registration_endpoint":  srv.URL + "/register",
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"client_id": "test-client-id"})
	})
	srv.Start()
	defer srv.Close()

	authorizeURL, err := StartOAuth("test", srv.URL, "http://localhost:3847/callback")
	require.NoError(t, err)

	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	assert.Equal(t, mcpResourceURL(srv.URL), parsed.Query().Get("resource"))
}

func TestOAuth_PollNoFlow(t *testing.T) {
	status, token, errStr := PollOAuth("nonexistent")
	assert.Equal(t, "no_flow", status)
	assert.Empty(t, token)
	assert.NotEmpty(t, errStr)
}

func TestOAuth_DiscoverBadURL(t *testing.T) {
	_, err := discoverOAuth("https://invalid.example.com")
	assert.Error(t, err)
}

func TestOAuth_DiscoverSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 "https://example.com",
				"authorization_endpoint": "https://example.com/authorize",
				"token_endpoint":         "https://example.com/token",
				"registration_endpoint":  "https://example.com/register",
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	meta, err := discoverOAuth(srv.URL)
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com", meta.Issuer)
	assert.Equal(t, "https://example.com/authorize", meta.AuthorizationEndpoint)
	assert.Equal(t, "https://example.com/token", meta.TokenEndpoint)
	assert.Equal(t, "https://example.com/register", meta.RegistrationEndpoint)
}

func TestOAuth_RegisterClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{
			"client_id": "test-client-id",
		})
	}))
	defer srv.Close()

	clientID, clientSecret, err := registerClient(srv.URL, "http://localhost:3847/callback")
	assert.NoError(t, err)
	assert.Equal(t, "test-client-id", clientID)
	assert.Empty(t, clientSecret)
}

func TestOAuth_RegisterClient_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"bad_request"}`))
	}))
	defer srv.Close()

	_, _, err := registerClient(srv.URL, "http://localhost:3847/callback")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "registration failed")
}

func TestOAuth_HandleCallback_NoFlow(t *testing.T) {
	err := HandleOAuthCallback("nonexistent", "code123", "state123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow in progress")
}

func TestConfigure_StoresRefreshMaterial(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	err := r.Configure(context.Background(), mcp.Credentials{
		"access_token":  "acc",
		"refresh_token": "ref",
		"client_id":     "cid",
		"client_secret": "csec",
	})
	require.NoError(t, err)
	assert.Equal(t, "acc", r.token)
	assert.Equal(t, "ref", r.refreshToken)
	assert.Equal(t, "cid", r.clientID)
	assert.Equal(t, "csec", r.clientSecret)
	assert.True(t, r.canRefresh())
}

func TestConfigure_AccessTokenOnly_CannotRefresh(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	err := r.Configure(context.Background(), mcp.Credentials{"access_token": "acc"})
	require.NoError(t, err)
	assert.False(t, r.canRefresh(), "Configure must not pretend we can refresh without creds")
}

func TestConfigure_ClearsStaleRefreshMaterial(t *testing.T) {
	// Calling Configure with a different set of credentials must overwrite
	// stale refresh material — otherwise a re-auth that captures only an
	// access token would silently reuse the previous owner's refresh token.
	r := &remote{name: "test", serverURL: "https://example.com"}
	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{
		"access_token":  "acc1",
		"refresh_token": "ref1",
		"client_id":     "cid1",
	}))

	require.NoError(t, r.Configure(context.Background(), mcp.Credentials{
		"access_token": "acc2",
	}))
	assert.Equal(t, "acc2", r.token)
	assert.Empty(t, r.refreshToken)
	assert.Empty(t, r.clientID)
	assert.False(t, r.canRefresh())
}

func TestWithTokenSink_OptionApplied(t *testing.T) {
	var called bool
	sink := TokenSink(func(_ mcp.Credentials) { called = true })
	i := New("test", "https://example.com", WithTokenSink(sink))
	r := i.(*remote)
	require.NotNil(t, r.tokenSink)
	r.tokenSink(mcp.Credentials{})
	assert.True(t, called)
}

func TestGetOAuthTokens_NoFlow(t *testing.T) {
	_, err := GetOAuthTokens("does-not-exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth flow")
}

func TestGetOAuthTokens_Complete(t *testing.T) {
	const name = "test-gettokens-complete"
	activeRemoteOAuth.mu.Lock()
	activeRemoteOAuth.states[name] = &OAuthState{
		clientID:     "cid",
		clientSecret: "csec",
		token:        "acc",
		refreshToken: "ref",
		expiresIn:    3600,
		done:         true,
	}
	activeRemoteOAuth.mu.Unlock()
	defer func() {
		activeRemoteOAuth.mu.Lock()
		delete(activeRemoteOAuth.states, name)
		activeRemoteOAuth.mu.Unlock()
	}()

	tokens, err := GetOAuthTokens(name)
	require.NoError(t, err)
	assert.Equal(t, "acc", tokens.AccessToken)
	assert.Equal(t, "ref", tokens.RefreshToken)
	assert.Equal(t, "cid", tokens.ClientID)
	assert.Equal(t, "csec", tokens.ClientSecret)
	assert.Equal(t, 3600, tokens.ExpiresIn)
}

func TestGetOAuthTokens_Pending(t *testing.T) {
	const name = "test-gettokens-pending"
	activeRemoteOAuth.mu.Lock()
	activeRemoteOAuth.states[name] = &OAuthState{} // done=false
	activeRemoteOAuth.mu.Unlock()
	defer func() {
		activeRemoteOAuth.mu.Lock()
		delete(activeRemoteOAuth.states, name)
		activeRemoteOAuth.mu.Unlock()
	}()

	_, err := GetOAuthTokens(name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pending")
}

func TestGetOAuthTokens_Errored(t *testing.T) {
	const name = "test-gettokens-errored"
	activeRemoteOAuth.mu.Lock()
	activeRemoteOAuth.states[name] = &OAuthState{done: true, err: "upstream rejected"}
	activeRemoteOAuth.mu.Unlock()
	defer func() {
		activeRemoteOAuth.mu.Lock()
		delete(activeRemoteOAuth.states, name)
		activeRemoteOAuth.mu.Unlock()
	}()

	_, err := GetOAuthTokens(name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream rejected")
}
