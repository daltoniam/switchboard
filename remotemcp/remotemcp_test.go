package remotemcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// --- Token Refresh ---

func TestConfigure_StoresRefreshToken(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	err := r.Configure(context.Background(), mcp.Credentials{
		"access_token":  "tok1",
		"refresh_token": "refresh-abc",
		"client_id":     "client-123",
		"client_secret": "secret-xyz",
	})
	assert.NoError(t, err)
	assert.Equal(t, "tok1", r.token)
	assert.Equal(t, "refresh-abc", r.refreshToken)
	assert.Equal(t, "client-123", r.clientID)
	assert.Equal(t, "secret-xyz", r.clientSecret)
}

func TestRefreshToken_Success(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":         srvURL,
				"token_endpoint": srvURL + "/oauth/token",
			})
		case "/oauth/token":
			assert.Equal(t, "refresh_token", r.FormValue("grant_type"))
			assert.Equal(t, "old-refresh", r.FormValue("refresh_token"))
			assert.Equal(t, "client-id", r.FormValue("client_id"))
			json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	result, err := RefreshToken(srv.URL, "client-id", "", "old-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-access", result.AccessToken)
	assert.Equal(t, "new-refresh", result.RefreshToken)
	assert.Equal(t, 3600, result.ExpiresIn)
}

func TestRefreshToken_PreservesOldRefreshWhenNotReturned(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":         srvURL,
				"token_endpoint": srvURL + "/oauth/token",
			})
		case "/oauth/token":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "new-access",
				"expires_in":   3600,
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	result, err := RefreshToken(srv.URL, "client-id", "", "old-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-access", result.AccessToken)
	assert.Equal(t, "old-refresh", result.RefreshToken)
}

func TestRefreshToken_Failure(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":         srvURL,
				"token_endpoint": srvURL + "/oauth/token",
			})
		case "/oauth/token":
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{
				"error": "invalid_grant",
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	_, err := RefreshToken(srv.URL, "client-id", "", "bad-refresh")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refresh token failed")
}

func TestPollOAuthFull_NoFlow(t *testing.T) {
	status, result, errStr := PollOAuthFull("nonexistent-full")
	assert.Equal(t, "no_flow", status)
	assert.Nil(t, result)
	assert.NotEmpty(t, errStr)
}

func TestSetTokenRefreshCallback(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	var called bool
	SetTokenRefreshCallback(r, func(_, _ string, _ int) {
		called = true
	})
	assert.NotNil(t, r.onTokenRefresh)
	r.onTokenRefresh("a", "b", 1)
	assert.True(t, called)
}

func TestTokenNeedsRefresh_ZeroExpiry(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com"}
	assert.False(t, r.tokenNeedsRefresh())
}

func TestTryRefreshToken_NoRefreshToken(t *testing.T) {
	r := &remote{name: "test", serverURL: "https://example.com", token: "tok"}
	assert.False(t, r.tryRefreshToken())
}
