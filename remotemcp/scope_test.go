package remotemcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopeDiscovery serves protected resource metadata for /mcp and
// authorization server metadata at the origin, each advertising the given
// scopes (nil leaves scopes_supported out). Without namedIssuer the
// resource metadata names no authorization server, so discovery falls back
// to the origin's metadata.
func scopeDiscovery(t *testing.T, resourceScopes, serverScopes []string, namedIssuer bool) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource/mcp":
			body := map[string]any{"resource": srv.URL + "/mcp"}
			if namedIssuer {
				body["authorization_servers"] = []string{srv.URL}
			}
			if resourceScopes != nil {
				body["scopes_supported"] = resourceScopes
			}
			_ = json.NewEncoder(w).Encode(body)
		case "/.well-known/oauth-authorization-server":
			body := map[string]any{
				"issuer":                 srv.URL,
				"authorization_endpoint": srv.URL + "/authorize",
				"token_endpoint":         srv.URL + "/token",
				"registration_endpoint":  srv.URL + "/register",
			}
			if serverScopes != nil {
				body["scopes_supported"] = serverScopes
			}
			_ = json.NewEncoder(w).Encode(body)
		case "/register":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"client_id": "scope-client"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOAuth_AdvertisedScope(t *testing.T) {
	tests := []struct {
		name           string
		resourceScopes []string
		serverScopes   []string
		originFallback bool
		wantScope      string
		wantPresent    bool
	}{
		{name: "protected resource scopes win", resourceScopes: []string{"alerts:read", "offline_access"}, serverScopes: []string{"openid"}, wantScope: "alerts:read offline_access", wantPresent: true},
		{name: "authorization server scopes as fallback", serverScopes: []string{"mcp", "offline_access"}, wantScope: "mcp offline_access", wantPresent: true},
		{name: "no advertised scopes omits the parameter", wantPresent: false},
		{name: "resource scopes survive the origin fallback", resourceScopes: []string{"alerts:read"}, serverScopes: []string{"openid"}, originFallback: true, wantScope: "alerts:read", wantPresent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := scopeDiscovery(t, tt.resourceScopes, tt.serverScopes, !tt.originFallback)
			authorizeURL, err := StartOAuth("scope-"+tt.name, srv.URL, "http://localhost/callback", OAuthOptions{
				AdvertisedScope: true,
				Resource:        srv.URL + "/mcp",
			})
			require.NoError(t, err)
			parsed, err := url.Parse(authorizeURL)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPresent, parsed.Query().Has("scope"))
			assert.Equal(t, tt.wantScope, parsed.Query().Get("scope"))
			assert.Equal(t, srv.URL+"/mcp", parsed.Query().Get("resource"))
		})
	}
}

func TestOAuth_DefaultScopeIgnoresAdvertisedScopes(t *testing.T) {
	srv := scopeDiscovery(t, []string{"alerts:read"}, nil, true)
	authorizeURL, err := StartOAuth("scope-default", srv.URL, "http://localhost/callback", OAuthOptions{Resource: srv.URL + "/mcp"})
	require.NoError(t, err)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	assert.Equal(t, "read,write", parsed.Query().Get("scope"))
}

func TestConvertTools_AccessFromAnnotations(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name        string
		annotations *mcpsdk.ToolAnnotations
		want        mcp.ToolAccess
	}{
		{name: "no annotations stay unclassified", want: mcp.AccessUnspecified},
		{name: "read only", annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true}, want: mcp.AccessRead},
		{name: "read only wins over destructive", annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &yes}, want: mcp.AccessRead},
		{name: "explicitly not destructive", annotations: &mcpsdk.ToolAnnotations{DestructiveHint: &no}, want: mcp.AccessWrite},
		{name: "explicitly destructive", annotations: &mcpsdk.ToolAnnotations{DestructiveHint: &yes}, want: mcp.AccessDestructive},
		{name: "unstated destructiveness defaults to destructive", annotations: &mcpsdk.ToolAnnotations{Title: "Update"}, want: mcp.AccessDestructive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := convertTools("test", []*mcpsdk.Tool{{Name: "op", Annotations: tt.annotations}})
			require.Len(t, tools, 1)
			assert.Equal(t, tt.want, tools[0].Access)
		})
	}
}
