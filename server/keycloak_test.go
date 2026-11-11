package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/keycloak"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func newKeycloakServer(t *testing.T, handler http.HandlerFunc) *Server {
	t.Helper()
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	reg := newMockRegistry()
	require.NoError(t, reg.Register(keycloak.New()))
	return New(&mcp.Services{Registry: reg, Config: newMockConfigService(map[string]*mcp.IntegrationConfig{
		"keycloak": {Enabled: true, Credentials: mcp.Credentials{"base_url": upstream.URL, "realm": "support", "client_id": "reader", "client_secret": "fixture-secret"}},
	})})
}

func TestKeycloakDiscoveryAndVisibility(t *testing.T) {
	var calls atomic.Int32
	server := newKeycloakServer(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	cases := []struct{ query, expected string }{
		{"Keycloak disabled accounts", "keycloak_list_users"},
		{"Keycloak user profile", "keycloak_get_user"},
		{"Keycloak groups", "keycloak_list_groups"},
		{"Keycloak failed login audit", "keycloak_list_events"},
	}
	for _, test := range cases {
		t.Run(test.query, func(t *testing.T) {
			result, err := server.handleSearch(t.Context(), searchRequest(map[string]any{"query": test.query, "limit": 3}))
			require.NoError(t, err)
			require.False(t, result.IsError)
			require.Contains(t, searchToolNames(t, parseSearchResponse(t, result)), test.expected)
		})
	}
	integration, ok := server.services.Config.GetIntegration("keycloak")
	require.True(t, ok)
	integration.Enabled = false
	require.NoError(t, server.services.Config.SetIntegration("keycloak", integration))
	server.RefreshSearchIndex()
	result, err := server.handleSearch(t.Context(), searchRequest(map[string]any{"integration": "keycloak"}))
	require.NoError(t, err)
	require.Zero(t, parseSearchResponse(t, result).Total)
	require.Zero(t, calls.Load())
}

func TestKeycloakRawAndCompactedPipeline(t *testing.T) {
	for _, script := range []bool{false, true} {
		t.Run(map[bool]string{false: "compacted", true: "script"}[script], func(t *testing.T) {
			var reads atomic.Int32
			server := newKeycloakServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/realms/support/protocol/openid-connect/token" {
					_, _ = io.WriteString(w, `{"access_token":"fixture-token","token_type":"Bearer","expires_in":60}`)
					return
				}
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "Bearer fixture-token", r.Header.Get("Authorization"))
				reads.Add(1)
				_, _ = io.WriteString(w, `[{"id":"user-1","username":"alice","enabled":false,"credentials":[{"value":"NEVER-RETURN"}],"attributes":{"secret":"NEVER-RETURN"}}]`)
			})
			request := executeRequest("keycloak_list_users", map[string]any{})
			if script {
				request = &mcpsdk.CallToolRequest{Params: &mcpsdk.CallToolParamsRaw{Name: "execute", Arguments: json.RawMessage(`{"script":"api.call('keycloak_list_users', {});"}`)}}
			}
			result, err := server.handleExecute(t.Context(), request)
			require.NoError(t, err)
			require.False(t, result.IsError)
			text := result.Content[0].(*mcpsdk.TextContent).Text
			require.Contains(t, text, "alice")
			require.NotContains(t, text, "NEVER-RETURN")
			require.NotContains(t, text, "credentials")
			require.NotEqual(t, "{}", strings.TrimSpace(text))
			require.EqualValues(t, 1, reads.Load())
		})
	}
}

func TestKeycloakRateLimitsDoNotTriggerServerRetryStorm(t *testing.T) {
	var reads atomic.Int32
	server := newKeycloakServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/realms/support/protocol/openid-connect/token" {
			_, _ = io.WriteString(w, `{"access_token":"fixture-token","token_type":"Bearer","expires_in":60}`)
			return
		}
		reads.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	result, err := server.handleExecute(t.Context(), executeRequest("keycloak_list_users", map[string]any{}))
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "429")
	require.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "1m0s")
	require.EqualValues(t, 1, reads.Load())
}
