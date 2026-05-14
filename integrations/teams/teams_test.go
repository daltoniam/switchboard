package teams

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "teams", i.Name())
}

func TestTools(t *testing.T) {
	i := New()
	defs := i.Tools()
	assert.NotEmpty(t, defs)
	for _, d := range defs {
		assert.NotEmpty(t, d.Name, "tool has empty name")
		assert.NotEmpty(t, d.Description, "tool %s has empty description", d.Name)
	}
}

func TestTools_AllHaveTeamsPrefix(t *testing.T) {
	for _, d := range New().Tools() {
		assert.True(t, strings.HasPrefix(string(d.Name), "teams_"),
			"tool %s missing teams_ prefix", d.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	seen := make(map[mcp.ToolName]bool)
	for _, d := range New().Tools() {
		assert.False(t, seen[d.Name], "duplicate tool name: %s", d.Name)
		seen[d.Name] = true
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, d := range New().Tools() {
		_, ok := dispatch[d.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", d.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := make(map[mcp.ToolName]bool)
	for _, d := range New().Tools() {
		names[d.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	ti := newTestIntegration(t, nil)
	result, err := ti.Execute(context.Background(), "teams_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NoTenantConfigured(t *testing.T) {
	ti := newTestIntegration(t, nil)
	result, err := ti.Execute(context.Background(), "teams_get_me", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "no teams tenant configured")
}

func TestCompactSpec_KnownTool(t *testing.T) {
	ti := newTestIntegration(t, nil)
	fields, ok := ti.CompactSpec(mcp.ToolName("teams_list_chats"))
	assert.True(t, ok)
	assert.NotEmpty(t, fields)
}

func TestCompactSpec_UnknownTool(t *testing.T) {
	ti := newTestIntegration(t, nil)
	_, ok := ti.CompactSpec(mcp.ToolName("teams_does_not_exist"))
	assert.False(t, ok)
}

func TestPlainTextKeys(t *testing.T) {
	i := New().(*teamsIntegration)
	keys := i.PlainTextKeys()
	assert.Contains(t, keys, "tenant_id")
	assert.Contains(t, keys, "client_id")
}

func TestOptionalKeys(t *testing.T) {
	i := New().(*teamsIntegration)
	keys := i.OptionalKeys()
	assert.Contains(t, keys, "access_token")
	assert.Contains(t, keys, "refresh_token")
	assert.Contains(t, keys, "client_secret")
}

func TestPlaceholders(t *testing.T) {
	i := New().(*teamsIntegration)
	p := i.Placeholders()
	assert.NotEmpty(t, p["tenant_id"])
	assert.Equal(t, defaultGraphBaseURL, p["graph_base_url"])
}

// --- Configure ---

func TestConfigure_DefaultsApplied(t *testing.T) {
	ti := newTestIntegration(t, nil)
	err := ti.Configure(context.Background(), mcp.Credentials{})
	require.NoError(t, err)
	assert.Equal(t, defaultClientID, ti.clientID)
	assert.Equal(t, "", ti.clientSecret)
	assert.Equal(t, defaultLoginBaseURL, ti.loginBaseURL)
	assert.Equal(t, defaultScopes, ti.scopes)
}

func TestConfigure_ReadsClientSecret(t *testing.T) {
	ti := newTestIntegration(t, nil)
	err := ti.Configure(context.Background(), mcp.Credentials{
		"client_id":     "custom-client",
		"client_secret": "shhh",
	})
	require.NoError(t, err)
	assert.Equal(t, "custom-client", ti.clientID)
	assert.Equal(t, "shhh", ti.clientSecret)
}

func TestConfigure_AcceptsExplicitTokenAsTenant(t *testing.T) {
	ti := newTestIntegration(t, nil)
	err := ti.Configure(context.Background(), mcp.Credentials{
		"tenant_id":     "tenant-abc",
		"access_token":  "at-1",
		"refresh_token": "rt-1",
	})
	require.NoError(t, err)
	tn := ti.store.get("tenant-abc")
	require.NotNil(t, tn)
	assert.Equal(t, "at-1", tn.AccessToken)
	assert.Equal(t, "rt-1", tn.RefreshToken)
	assert.Equal(t, "tenant-abc", ti.store.defaultID())
}

// --- HTTP helpers ---

func TestGraphGet_AddsBearerToken(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":[{"id":"abc"}]}`))
	}))
	defer srv.Close()

	ti := newTestIntegration(t, srv)
	ti.store.upsert(&tenant{TenantID: "t1", AccessToken: "hello"})

	data, err := ti.graphGet(context.Background(), "t1", "/me/chats")
	require.NoError(t, err)
	assert.Contains(t, string(data), "abc")
	assert.Equal(t, "Bearer hello", seenAuth)
}

func TestGraphRequest_RetryOn401WithRefresh(t *testing.T) {
	var graphCalls, refreshCalls int
	var mu sync.Mutex

	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		graphCalls++
		auth := r.Header.Get("Authorization")
		mu.Unlock()
		if auth == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"InvalidAuthenticationToken"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"after-refresh"}`))
	}))
	defer graph.Close()

	login := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		refreshCalls++
		mu.Unlock()
		assert.Contains(t, r.URL.Path, "/oauth2/v2.0/token")
		body, _ := json.Marshal(accessTokenResponse{AccessToken: "fresh", ExpiresIn: 3600})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer login.Close()

	ti := newTestIntegration(t, graph)
	ti.loginBaseURL = login.URL
	ti.store.upsert(&tenant{TenantID: "t1", AccessToken: "stale", RefreshToken: "rt"})

	data, err := ti.graphGet(context.Background(), "t1", "/me")
	require.NoError(t, err)
	assert.Contains(t, string(data), "after-refresh")
	assert.Equal(t, 1, refreshCalls)
	assert.Equal(t, 2, graphCalls)
	assert.Equal(t, "fresh", ti.store.get("t1").AccessToken)
}

func TestGraphRequest_RetryableOn429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"throttled"}}`))
	}))
	defer srv.Close()

	ti := newTestIntegration(t, srv)
	ti.store.upsert(&tenant{TenantID: "t1", AccessToken: "ok"})
	_, err := ti.graphGet(context.Background(), "t1", "/me")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestGraphRequest_204NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ti := newTestIntegration(t, srv)
	ti.store.upsert(&tenant{TenantID: "t1", AccessToken: "ok"})
	data, err := ti.graphDelete(context.Background(), "t1", "/anything")
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestProactiveRefresh_OnSoonExpiry(t *testing.T) {
	var refreshed int
	var mu sync.Mutex
	login := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		refreshed++
		mu.Unlock()
		body, _ := json.Marshal(accessTokenResponse{AccessToken: "new", ExpiresIn: 3600})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer login.Close()
	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer graph.Close()

	ti := newTestIntegration(t, graph)
	ti.loginBaseURL = login.URL
	ti.store.upsert(&tenant{
		TenantID:     "t1",
		AccessToken:  "expiring",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(10 * time.Second), // within refreshSkew
	})
	_, err := ti.graphGet(context.Background(), "t1", "/me")
	require.NoError(t, err)
	assert.Equal(t, 1, refreshed)
	assert.Equal(t, "new", ti.store.get("t1").AccessToken)
}

// --- OAuth ---

func TestParseIDToken(t *testing.T) {
	// payload: {"tid":"tenantX","oid":"user1","upn":"alice@x.com","name":"Alice"}
	// header.payload.signature, payload is base64url-encoded.
	payload := `eyJ0aWQiOiJ0ZW5hbnRYIiwib2lkIjoidXNlcjEiLCJ1cG4iOiJhbGljZUB4LmNvbSIsIm5hbWUiOiJBbGljZSJ9`
	jwt := "header." + payload + ".sig"
	claims, err := parseIDToken(jwt)
	require.NoError(t, err)
	assert.Equal(t, "tenantX", claims.TID)
	assert.Equal(t, "alice@x.com", claims.UPN)
	assert.Equal(t, "Alice", claims.Name)
}

func TestRefreshTenant_NoRefreshToken(t *testing.T) {
	ti := newTestIntegration(t, nil)
	err := ti.refreshTenant(context.Background(), &tenant{TenantID: "t1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no refresh_token")
}

func TestRefreshTenant_NilTenant(t *testing.T) {
	ti := newTestIntegration(t, nil)
	// Should not panic — the nil guard must return an error rather than
	// dereferencing the nil pointer in the error message.
	err := ti.refreshTenant(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil tenant")
}

func TestConfigure_ParsesExpiresAt(t *testing.T) {
	ti := newTestIntegration(t, nil)
	exp := "2099-01-02T15:04:05Z"
	err := ti.Configure(context.Background(), mcp.Credentials{
		"tenant_id":    "tenant-z",
		"access_token": "at-z",
		"expires_at":   exp,
	})
	require.NoError(t, err)
	tn := ti.store.get("tenant-z")
	require.NotNil(t, tn)
	want, _ := time.Parse(time.RFC3339, exp)
	assert.True(t, tn.ExpiresAt.Equal(want), "ExpiresAt %v != %v", tn.ExpiresAt, want)
}

// --- Token store ---

func TestTokenStore_UpsertSetsDefault(t *testing.T) {
	store := &tokenStore{tenants: map[string]*tenant{}, filePath: t.TempDir() + "/tokens.json"}
	store.upsert(&tenant{TenantID: "first", AccessToken: "a"})
	assert.Equal(t, "first", store.defaultID())
	store.upsert(&tenant{TenantID: "second", AccessToken: "b"})
	assert.Equal(t, "first", store.defaultID(), "default should not change on second upsert")
}

func TestTokenStore_RemovePromotesNewDefault(t *testing.T) {
	store := &tokenStore{tenants: map[string]*tenant{}, filePath: t.TempDir() + "/tokens.json"}
	store.upsert(&tenant{TenantID: "alpha", AccessToken: "a"})
	store.upsert(&tenant{TenantID: "bravo", AccessToken: "b"})
	store.remove("alpha")
	assert.Equal(t, "bravo", store.defaultID())
	store.remove("bravo")
	assert.Equal(t, "", store.defaultID())
}

func TestTokenStore_PersistRoundTrip(t *testing.T) {
	path := t.TempDir() + "/tokens.json"
	store := &tokenStore{tenants: map[string]*tenant{}, filePath: path}
	store.upsert(&tenant{TenantID: "t1", AccessToken: "a", RefreshToken: "r", UserUPN: "x@y.com", ExpiresAt: time.Now().Add(time.Hour)})
	require.NoError(t, store.saveToFile())

	store2 := &tokenStore{tenants: map[string]*tenant{}, filePath: path}
	store2.loadFromFile()
	tn := store2.get("t1")
	require.NotNil(t, tn)
	assert.Equal(t, "a", tn.AccessToken)
	assert.Equal(t, "x@y.com", tn.UserUPN)
}

// --- Auth handler smoke ---

func TestTeamsListTenants_Empty(t *testing.T) {
	ti := newTestIntegration(t, nil)
	res, err := ti.Execute(context.Background(), "teams_list_tenants", nil)
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, `"count":0`)
}

func TestTeamsListTenants_OneTenant(t *testing.T) {
	ti := newTestIntegration(t, nil)
	ti.store.upsert(&tenant{TenantID: "t1", AccessToken: "a", UserUPN: "x@y.com"})
	res, err := ti.Execute(context.Background(), "teams_list_tenants", nil)
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, `"count":1`)
	assert.Contains(t, res.Data, "x@y.com")
}

func TestTeamsSetDefault_UnknownTenant(t *testing.T) {
	ti := newTestIntegration(t, nil)
	res, err := ti.Execute(context.Background(), "teams_set_default", map[string]any{"tenant_id": "bogus"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "unknown tenant")
}

func TestTeamsRemoveTenant_OK(t *testing.T) {
	ti := newTestIntegration(t, nil)
	ti.store.upsert(&tenant{TenantID: "t1", AccessToken: "a"})
	res, err := ti.Execute(context.Background(), "teams_remove_tenant", map[string]any{"tenant_id": "t1"})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Nil(t, ti.store.get("t1"))
}

// --- helpers ---

// newTestIntegration builds an integration wired to optional httptest server URLs.
// It avoids Configure() (which spawns background goroutines and reads disk) and
// instead initializes the fields needed by the tests directly.
func newTestIntegration(t *testing.T, graphServer *httptest.Server) *teamsIntegration {
	t.Helper()
	ti := &teamsIntegration{
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		store:        &tokenStore{tenants: map[string]*tenant{}, filePath: t.TempDir() + "/tokens.json"},
		clientID:     defaultClientID,
		loginBaseURL: defaultLoginBaseURL,
		graphBaseURL: defaultGraphBaseURL,
		scopes:       defaultScopes,
	}
	if graphServer != nil {
		ti.graphBaseURL = graphServer.URL
		ti.httpClient = graphServer.Client()
	}
	return ti
}
