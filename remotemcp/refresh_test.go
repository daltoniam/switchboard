package remotemcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type refreshFixture struct {
	server        *httptest.Server
	mu            sync.Mutex
	validToken    string
	rejectStatus  int
	refreshForms  []url.Values
	toolCalls     int
	refreshStatus int
}

func newRefreshFixture(t *testing.T, endpointPath string) *refreshFixture {
	t.Helper()
	f := &refreshFixture{validToken: "access-1", rejectStatus: http.StatusUnauthorized, refreshStatus: http.StatusOK}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fixture", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{Name: "search", Description: "Search", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		f.mu.Lock()
		f.toolCalls++
		f.mu.Unlock()
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"ok":true}`}}}, nil
	})
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		writeTestJSON(w, http.StatusOK, map[string]any{
			"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/oauth/authorize",
			"token_endpoint": f.server.URL + "/oauth/token", "registration_endpoint": f.server.URL + "/oauth/register",
		})
	})
	mux.HandleFunc("POST /oauth/register", func(w http.ResponseWriter, _ *http.Request) {
		writeTestJSON(w, http.StatusCreated, map[string]string{"client_id": "client-1"})
	})
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.PostForm.Get("grant_type") {
		case "refresh_token":
			f.refreshForms = append(f.refreshForms, r.PostForm)
			if f.refreshStatus != http.StatusOK {
				writeTestJSON(w, f.refreshStatus, map[string]string{"error": "invalid_grant"})
				return
			}
			f.validToken = "access-2"
			writeTestJSON(w, http.StatusOK, map[string]string{"access_token": "access-2", "refresh_token": "refresh-2", "token_type": "Bearer"})
		default:
			writeTestJSON(w, http.StatusOK, map[string]any{"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 3600})
		}
	})
	mux.HandleFunc(endpointPath, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		valid, status := f.validToken, f.rejectStatus
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+valid {
			http.Error(w, "unauthorized", status)
			return
		}
		handler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func writeTestJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func TestNewWithOptions_EndpointPath(t *testing.T) {
	f := newRefreshFixture(t, "/api/metabase-mcp")
	integration := NewWithOptions("metabase", f.server.URL, Options{EndpointPath: "/api/metabase-mcp"})
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"access_token": "access-1"}))
	assert.True(t, integration.Healthy(t.Context()))
	tools := integration.Tools()
	require.Len(t, tools, 1)
	assert.Equal(t, mcp.ToolName("metabase_search"), tools[0].Name)
}

func TestNewWithOptions_DefaultsEndpointPath(t *testing.T) {
	r := NewWithOptions("test", "https://example.com", Options{}).(*remote)
	assert.Equal(t, "/mcp", r.endpointPath)
	assert.Equal(t, "/mcp", New("test", "https://example.com").(*remote).endpointPath)
}

func TestConfigure_ReadsRefreshCredentials(t *testing.T) {
	r := New("test", "https://example.com").(*remote)
	require.NoError(t, r.Configure(t.Context(), mcp.Credentials{
		"access_token": "tok", "refresh_token": "ref", "client_id": "cid", "client_secret": "sec",
	}))
	assert.Equal(t, "tok", r.token)
	assert.Equal(t, "ref", r.refreshToken)
	assert.Equal(t, "cid", r.clientID)
	assert.Equal(t, "sec", r.clientSecret)
}

func TestTokenSource(t *testing.T) {
	r := New("test", "https://example.com").(*remote)
	source, err := r.TokenSource(t.Context())
	require.NoError(t, err)
	assert.Nil(t, source, "no token configured means no Authorization header")

	require.NoError(t, r.Configure(t.Context(), mcp.Credentials{"access_token": "tok"}))
	source, err = r.TokenSource(t.Context())
	require.NoError(t, err)
	token, err := source.Token()
	require.NoError(t, err)
	assert.Equal(t, "tok", token.AccessToken)
}

func TestAuthorize_Refresh(t *testing.T) {
	for _, tc := range []struct {
		name           string
		creds          mcp.Credentials
		rejectStatus   int
		refreshStatus  int
		wantRefreshes  int
		wantExecuteOK  bool
		wantErrContain string
		wantToken      string
		wantRefreshTok string
	}{
		{
			name:           "refreshes rotated token on 401",
			creds:          mcp.Credentials{"access_token": "stale", "refresh_token": "refresh-1", "client_id": "client-1"},
			rejectStatus:   http.StatusUnauthorized,
			refreshStatus:  http.StatusOK,
			wantRefreshes:  1,
			wantExecuteOK:  true,
			wantToken:      "access-2",
			wantRefreshTok: "refresh-2",
		},
		{
			name:           "no refresh token asks to reconnect",
			creds:          mcp.Credentials{"access_token": "stale"},
			rejectStatus:   http.StatusUnauthorized,
			refreshStatus:  http.StatusOK,
			wantErrContain: "reconnect",
			wantToken:      "stale",
		},
		{
			name:           "403 is not refreshed",
			creds:          mcp.Credentials{"access_token": "stale", "refresh_token": "refresh-1", "client_id": "client-1"},
			rejectStatus:   http.StatusForbidden,
			refreshStatus:  http.StatusOK,
			wantErrContain: "403",
			wantToken:      "stale",
			wantRefreshTok: "refresh-1",
		},
		{
			name:           "invalid_grant keeps old credentials",
			creds:          mcp.Credentials{"access_token": "stale", "refresh_token": "refresh-1", "client_id": "client-1"},
			rejectStatus:   http.StatusUnauthorized,
			refreshStatus:  http.StatusBadRequest,
			wantRefreshes:  1,
			wantErrContain: "invalid_grant",
			wantToken:      "stale",
			wantRefreshTok: "refresh-1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRefreshFixture(t, "/api/metabase-mcp")
			f.rejectStatus = tc.rejectStatus
			f.refreshStatus = tc.refreshStatus
			var persisted []TokenSet
			integration := NewWithOptions("metabase", f.server.URL, Options{
				EndpointPath:   "/api/metabase-mcp",
				OnTokenRefresh: func(set TokenSet) { persisted = append(persisted, set) },
			})
			r := integration.(*remote)
			require.NoError(t, r.Configure(t.Context(), tc.creds))

			result, err := r.Execute(t.Context(), "metabase_search", map[string]any{"q": "x"})
			require.NoError(t, err)

			f.mu.Lock()
			defer f.mu.Unlock()
			require.Len(t, f.refreshForms, tc.wantRefreshes)
			if tc.wantRefreshes > 0 {
				form := f.refreshForms[0]
				assert.Equal(t, "refresh_token", form.Get("grant_type"))
				assert.Equal(t, "refresh-1", form.Get("refresh_token"))
				assert.Equal(t, "client-1", form.Get("client_id"))
				assert.Empty(t, form.Get("client_secret"))
			}
			if tc.wantExecuteOK {
				assert.False(t, result.IsError, result.Data)
				assert.Equal(t, 1, f.toolCalls)
				require.Len(t, persisted, 1)
				assert.Equal(t, TokenSet{AccessToken: "access-2", RefreshToken: "refresh-2", ClientID: "client-1"}, persisted[0])
			} else {
				assert.True(t, result.IsError)
				assert.Contains(t, result.Data, tc.wantErrContain)
				assert.Empty(t, persisted)
				assert.Equal(t, 0, f.toolCalls)
			}
			assert.Equal(t, tc.wantToken, r.token)
			assert.Equal(t, tc.wantRefreshTok, r.refreshToken)
		})
	}
}

func TestOAuthFlow_PollOAuthTokens(t *testing.T) {
	f := newRefreshFixture(t, "/api/metabase-mcp")
	authorizeURL, err := StartOAuth("metabase-poll", f.server.URL, "http://localhost:3847/cb", OAuthOptions{Scope: "agent:content:read", Resource: f.server.URL + "/api/metabase-mcp"})
	require.NoError(t, err)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)

	status, tokens, errStr := PollOAuthTokens("metabase-poll")
	assert.Equal(t, "pending", status)
	assert.Empty(t, errStr)
	assert.Equal(t, TokenSet{}, tokens)

	require.NoError(t, HandleOAuthCallback("metabase-poll", "code-1", parsed.Query().Get("state")))

	status, tokens, errStr = PollOAuthTokens("metabase-poll")
	assert.Equal(t, "complete", status)
	assert.Empty(t, errStr)
	assert.Equal(t, TokenSet{AccessToken: "access-1", RefreshToken: "refresh-1", ClientID: "client-1"}, tokens)

	legacyStatus, legacyToken, _ := PollOAuth("metabase-poll")
	assert.Equal(t, "complete", legacyStatus)
	assert.Equal(t, "access-1", legacyToken)

	_, tokens, errStr = PollOAuthTokens("nonexistent")
	assert.Equal(t, TokenSet{}, tokens)
	assert.NotEmpty(t, errStr)
}
