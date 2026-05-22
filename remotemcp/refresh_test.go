package remotemcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOAuthServer fakes the subset of the OAuth metadata + token endpoints
// that RefreshAccessToken touches: discovery returns a token_endpoint
// pointing at /token, and /token returns whatever response the test sets.
func fakeOAuthServer(t *testing.T, response any, statusCode int) *httptest.Server {
	t.Helper()
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
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_ = r.ParseForm()
		assert.Equal(t, "refresh_token", r.FormValue("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		if statusCode != 0 {
			w.WriteHeader(statusCode)
		}
		if response != nil {
			_ = json.NewEncoder(w).Encode(response)
		}
	})
	srv.Start()
	return srv
}

func TestRefreshAccessToken_Success(t *testing.T) {
	srv := fakeOAuthServer(t, map[string]any{
		"access_token": "new-acc",
		"token_type":   "Bearer",
		"expires_in":   3600,
	}, 200)
	defer srv.Close()

	tokens, err := RefreshAccessToken(context.Background(), srv.URL, "cid", "csec", "ref-tok")
	require.NoError(t, err)
	assert.Equal(t, "new-acc", tokens.AccessToken)
	assert.Empty(t, tokens.RefreshToken, "no rotation when upstream omits refresh_token")
	assert.Equal(t, 3600, tokens.ExpiresIn)
	assert.Equal(t, "cid", tokens.ClientID)
	assert.Equal(t, "csec", tokens.ClientSecret)
}

func TestRefreshAccessToken_RotatedRefresh(t *testing.T) {
	srv := fakeOAuthServer(t, map[string]any{
		"access_token":  "new-acc",
		"refresh_token": "new-ref",
		"token_type":    "Bearer",
	}, 200)
	defer srv.Close()

	tokens, err := RefreshAccessToken(context.Background(), srv.URL, "cid", "", "ref-tok")
	require.NoError(t, err)
	assert.Equal(t, "new-acc", tokens.AccessToken)
	assert.Equal(t, "new-ref", tokens.RefreshToken)
}

func TestRefreshAccessToken_MissingRefreshToken(t *testing.T) {
	_, err := RefreshAccessToken(context.Background(), "https://example.com", "cid", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refresh_token")
}

func TestRefreshAccessToken_MissingClientID(t *testing.T) {
	_, err := RefreshAccessToken(context.Background(), "https://example.com", "", "", "ref-tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestRefreshAccessToken_DiscoveryFails(t *testing.T) {
	_, err := RefreshAccessToken(context.Background(), "https://invalid.example.invalid", "cid", "", "ref-tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discover")
}

func TestRefreshAccessToken_TokenEndpointError(t *testing.T) {
	srv := fakeOAuthServer(t, map[string]any{"error": "invalid_grant"}, 400)
	defer srv.Close()

	_, err := RefreshAccessToken(context.Background(), srv.URL, "cid", "", "ref-tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestRefreshAccessToken_MissingAccessTokenInResponse(t *testing.T) {
	srv := fakeOAuthServer(t, map[string]any{"token_type": "Bearer"}, 200)
	defer srv.Close()

	_, err := RefreshAccessToken(context.Background(), srv.URL, "cid", "", "ref-tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

func TestRefreshAccessToken_ErrorField(t *testing.T) {
	srv := fakeOAuthServer(t, map[string]any{
		"access_token": "ignored",
		"error":        "invalid_grant",
	}, 200)
	defer srv.Close()

	_, err := RefreshAccessToken(context.Background(), srv.URL, "cid", "", "ref-tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
}

// --- refreshingTransport integration tests ---

// startRefreshingMCP spins up an httptest server that simulates a remote MCP
// requiring refresh: the first request with the old access token returns 401,
// subsequent requests with the new access token return 200 + a JSON-RPC
// envelope. The OAuth /token endpoint exchanges any refresh_token for a fixed
// new access token. Used by the transport-level tests below.
type refreshScenario struct {
	srv             *httptest.Server
	mu              sync.Mutex
	requestsByToken map[string]int
	tokenCalls      atomic.Int32
}

func startRefreshingMCP(t *testing.T) *refreshScenario {
	t.Helper()
	s := &refreshScenario{requestsByToken: map[string]int{}}
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
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		s.tokenCalls.Add(1)
		body, _ := url.ParseQuery(readBody(r))
		assert.Equal(t, "refresh_token", body.Get("grant_type"))
		assert.NotEmpty(t, body.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-acc",
			"refresh_token": "rotated-ref",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		s.mu.Lock()
		s.requestsByToken[auth]++
		s.mu.Unlock()
		if auth == "Bearer old-acc" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	srv.Start()
	s.srv = srv
	return s
}

func readBody(r *http.Request) string {
	defer func() { _ = r.Body.Close() }()
	buf := make([]byte, r.ContentLength)
	if r.ContentLength > 0 {
		_, _ = r.Body.Read(buf)
	}
	return string(buf)
}

func TestRefreshingTransport_RefreshesAndRetries(t *testing.T) {
	s := startRefreshingMCP(t)
	defer s.srv.Close()

	r := &remote{
		name:         "test",
		serverURL:    s.srv.URL,
		token:        "old-acc",
		refreshToken: "ref-tok",
		clientID:     "cid",
	}
	var sinkCalls []mcp.Credentials
	r.tokenSink = func(creds mcp.Credentials) {
		sinkCalls = append(sinkCalls, creds)
	}

	transport := &refreshingTransport{remote: r}
	client := &http.Client{Transport: transport}

	resp, err := client.Post(s.srv.URL+"/mcp", "application/json", strings.NewReader(`{"id":1}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "request should succeed after refresh+retry")
	assert.Equal(t, int32(1), s.tokenCalls.Load(), "exactly one refresh attempt")

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Equal(t, 1, s.requestsByToken["Bearer old-acc"], "old token tried once")
	assert.Equal(t, 1, s.requestsByToken["Bearer new-acc"], "new token retried once")

	// Sink should observe the new access token and the rotated refresh token.
	require.Len(t, sinkCalls, 1)
	assert.Equal(t, "new-acc", sinkCalls[0]["access_token"])
	assert.Equal(t, "rotated-ref", sinkCalls[0]["refresh_token"])

	// remote's in-memory state should reflect the rotation.
	assert.Equal(t, "new-acc", r.token)
	assert.Equal(t, "rotated-ref", r.refreshToken)
}

func TestRefreshingTransport_NoRefreshCreds_Returns401(t *testing.T) {
	s := startRefreshingMCP(t)
	defer s.srv.Close()

	r := &remote{
		name:      "test",
		serverURL: s.srv.URL,
		token:     "old-acc",
		// No refreshToken / clientID — canRefresh() == false
	}
	transport := &refreshingTransport{remote: r}
	client := &http.Client{Transport: transport}

	resp, err := client.Post(s.srv.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, int32(0), s.tokenCalls.Load(), "must not attempt refresh without creds")
}

func TestRefreshingTransport_BodyReplayedOnRetry(t *testing.T) {
	// Verify that the original request body survives the refresh + retry —
	// i.e. the retried request carries the same payload, not an empty body.
	var bodies []string
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	var tokCalls atomic.Int32

	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":         srv.URL,
			"token_endpoint": srv.URL + "/token",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tokCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fresh",
			"token_type":   "Bearer",
		})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		b := readBody(r)
		bodies = append(bodies, b)
		if r.Header.Get("Authorization") == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv.Start()
	defer srv.Close()

	r := &remote{
		name:         "test",
		serverURL:    srv.URL,
		token:        "stale",
		refreshToken: "ref",
		clientID:     "cid",
	}
	transport := &refreshingTransport{remote: r}
	client := &http.Client{Transport: transport}

	payload := `{"jsonrpc":"2.0","method":"tools/list","id":1}`
	resp, err := client.Post(srv.URL+"/mcp", "application/json", strings.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	require.Len(t, bodies, 2, "expect two requests: original + retry")
	assert.Equal(t, payload, bodies[0], "first request carries the original body")
	assert.Equal(t, payload, bodies[1], "retried request carries the SAME body — replay must work")
}
