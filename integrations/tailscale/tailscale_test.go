package tailscale

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "tailscale", i.Name())
}

func TestConfigure_APIKey(t *testing.T) {
	ts := &tailscale{}
	err := ts.Configure(context.Background(), mcp.Credentials{"api_key": " tskey-api-abc "})
	require.NoError(t, err)
	assert.Equal(t, "tskey-api-abc", ts.apiKey)
	assert.Equal(t, "-", ts.tailnet)
	assert.Equal(t, defaultBaseURL, ts.baseURL)
	assert.NotNil(t, ts.httpClient)
}

func TestConfigure_CustomTailnetAndBaseURL(t *testing.T) {
	ts := &tailscale{}
	err := ts.Configure(context.Background(), mcp.Credentials{
		"api_key":  "tskey-api-abc",
		"tailnet":  "example.com",
		"base_url": "https://headscale.example.com/",
	})
	require.NoError(t, err)
	assert.Equal(t, "example.com", ts.tailnet)
	assert.Equal(t, "https://headscale.example.com", ts.baseURL)
}

func TestConfigure_Errors(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  string
	}{
		{"missing everything", mcp.Credentials{}, "api_key or oauth_client_id + oauth_client_secret is required"},
		{"client id only", mcp.Credentials{"oauth_client_id": "k123"}, "oauth_client_id and oauth_client_secret must both be set"},
		{"secret only", mcp.Credentials{"oauth_client_secret": "tskey-client-x"}, "oauth_client_id and oauth_client_secret must both be set"},
		{"both auth modes", mcp.Credentials{"api_key": "tskey-api-a", "oauth_client_id": "k", "oauth_client_secret": "s"}, "not both"},
		{"auth key as api key", mcp.Credentials{"api_key": "tskey-auth-abc"}, "auth keys"},
		{"client secret as api key", mcp.Credentials{"api_key": "tskey-client-abc"}, "oauth_client_secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New().Configure(context.Background(), tt.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestConfigure_OAuthClientCredentials(t *testing.T) {
	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/oauth/token":
			tokenCalls.Add(1)
			require.NoError(t, r.ParseForm())
			id, secret, ok := r.BasicAuth()
			if !ok {
				id, secret = r.Form.Get("client_id"), r.Form.Get("client_secret")
			}
			assert.Equal(t, "client-123", id)
			assert.Equal(t, "tskey-client-secret", secret)
			assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"short-lived","token_type":"Bearer","expires_in":3600}`))
		case "/api/v2/tailnet/-/devices":
			assert.Equal(t, "Bearer short-lived", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"devices":[]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	ts := &tailscale{client: srv.Client()}
	require.NoError(t, ts.Configure(context.Background(), mcp.Credentials{
		"oauth_client_id":     "client-123",
		"oauth_client_secret": "tskey-client-secret",
		"base_url":            srv.URL,
	}))
	result, err := ts.Execute(context.Background(), "tailscale_list_devices", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	_, err = ts.Execute(context.Background(), "tailscale_list_devices", nil)
	require.NoError(t, err)
	assert.Equal(t, int32(1), tokenCalls.Load(), "token should be cached across calls")
}

func TestHealthy_Unconfigured(t *testing.T) {
	assert.False(t, (&tailscale{}).Healthy(context.Background()))
}

func TestHealthy(t *testing.T) {
	ts, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/tailnet/-/devices", r.URL.Path)
		_, _ = w.Write([]byte(`{"devices":[]}`))
	})
	assert.True(t, ts.Healthy(context.Background()))
}

func TestTools_Metadata(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Description, "tool %s", tool.Name)
		assert.True(t, strings.HasPrefix(string(tool.Name), "tailscale_"), "tool %s lacks prefix", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		for _, req := range tool.Required {
			_, ok := tool.Parameters[req]
			assert.True(t, ok, "tool %s requires undeclared param %s", tool.Name, req)
		}
	}
}

func TestTools_GetDNSExplainsOmittedFields(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "tailscale_get_dns" {
			assert.Contains(t, tool.Description, "omitted")
			assert.Contains(t, tool.Description, "none configured")
			return
		}
	}
	t.Fatal("tailscale_get_dns not found")
}

func TestTools_EntryPointHasStartHere(t *testing.T) {
	for _, tool := range New().Tools() {
		if tool.Name == "tailscale_list_devices" {
			assert.Contains(t, tool.Description, "Start here")
			return
		}
	}
	t.Fatal("tailscale_list_devices not found")
}

func TestExecute_UnknownTool(t *testing.T) {
	result, err := (&tailscale{}).Execute(context.Background(), "tailscale_nope", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "dispatch handler %s has no tool definition", name)
	}
}

type recorded struct {
	method  string
	path    string
	query   string
	body    string
	headers http.Header
}

func testServer(t *testing.T, handler http.HandlerFunc) (*tailscale, *[]recorded) {
	t.Helper()
	var reqs []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reqs = append(reqs, recorded{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(body), headers: r.Header.Clone()})
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	ts := &tailscale{client: srv.Client()}
	require.NoError(t, ts.Configure(context.Background(), mcp.Credentials{"api_key": "tskey-api-test", "base_url": srv.URL}))
	return ts, &reqs
}

func okJSON(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}
}

func TestDoRequest_BearerAuth(t *testing.T) {
	ts, reqs := testServer(t, okJSON(`{"devices":[]}`))
	_, err := ts.doJSON(context.Background(), http.MethodGet, "/tailnet/-/devices", nil)
	require.NoError(t, err)
	require.Len(t, *reqs, 1)
	assert.Equal(t, "Bearer tskey-api-test", (*reqs)[0].headers.Get("Authorization"))
}

func TestDoRequest_APIError(t *testing.T) {
	ts, _ := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API token invalid"}`))
	})
	_, err := ts.doJSON(context.Background(), http.MethodGet, "/tailnet/-/devices", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tailscale API error (403)")
	assert.Contains(t, err.Error(), "API token invalid")
	assert.False(t, mcp.IsRetryable(err))
}

func TestDoRequest_Retryable(t *testing.T) {
	ts, _ := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := ts.doJSON(context.Background(), http.MethodGet, "/tailnet/-/devices", nil)
	require.Error(t, err)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
}

func TestReadLimited(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		limit   int64
		wantErr bool
	}{
		{"under limit", "abc", 5, false},
		{"exactly at limit", "abcde", 5, false},
		{"over limit", "abcdef", 5, true},
		{"empty", "", 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := readLimited(strings.NewReader(tt.body), tt.limit)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "exceeds")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.body, string(data))
		})
	}
}

func TestDoRequest_OversizedBodyFails(t *testing.T) {
	big := strings.Repeat("a", maxResponseSize+1)
	ts, _ := testServer(t, okJSON(big))
	_, err := ts.doJSON(context.Background(), http.MethodGet, "/tailnet/-/devices", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestDoRequest_EmptyBody(t *testing.T) {
	ts, _ := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	data, err := ts.doJSON(context.Background(), http.MethodPost, "/device/1/expire", nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"success"}`, string(data))
}

func TestHandlers_RequestShapes(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		args       map[string]any
		method     string
		path       string
		query      string
		body       string
		respBody   string
		wantResult string
	}{
		{
			name:   "list devices with filters",
			tool:   "tailscale_list_devices",
			args:   map[string]any{"hostname": "web-1", "tags": []any{"tag:prod", "tag:web"}, "fields": "all"},
			method: http.MethodGet, path: "/api/v2/tailnet/-/devices",
			query: "fields=all&hostname=web-1&tags=tag%3Aprod&tags=tag%3Aweb",
		},
		{
			name:   "list devices defaults",
			tool:   "tailscale_list_devices",
			args:   nil,
			method: http.MethodGet, path: "/api/v2/tailnet/-/devices",
			query: "fields=default",
		},
		{
			name:   "get device defaults to all fields",
			tool:   "tailscale_get_device",
			args:   map[string]any{"device_id": "n123"},
			method: http.MethodGet, path: "/api/v2/device/n123", query: "fields=all",
		},
		{
			name:   "authorize device defaults true",
			tool:   "tailscale_authorize_device",
			args:   map[string]any{"device_id": "n1"},
			method: http.MethodPost, path: "/api/v2/device/n1/authorized", body: `{"authorized":true}`,
		},
		{
			name:   "deauthorize device",
			tool:   "tailscale_authorize_device",
			args:   map[string]any{"device_id": "n1", "authorized": false},
			method: http.MethodPost, path: "/api/v2/device/n1/authorized", body: `{"authorized":false}`,
		},
		{
			name:   "delete device",
			tool:   "tailscale_delete_device",
			args:   map[string]any{"device_id": "n1"},
			method: http.MethodDelete, path: "/api/v2/device/n1",
		},
		{
			name:   "rename device",
			tool:   "tailscale_set_device_name",
			args:   map[string]any{"device_id": "n1", "name": "db-primary"},
			method: http.MethodPost, path: "/api/v2/device/n1/name", body: `{"name":"db-primary"}`,
		},
		{
			name:   "set tags",
			tool:   "tailscale_set_device_tags",
			args:   map[string]any{"device_id": "n1", "tags": []any{"tag:server"}},
			method: http.MethodPost, path: "/api/v2/device/n1/tags", body: `{"tags":["tag:server"]}`,
		},
		{
			name:   "clear tags with empty list",
			tool:   "tailscale_set_device_tags",
			args:   map[string]any{"device_id": "n1", "tags": []any{}},
			method: http.MethodPost, path: "/api/v2/device/n1/tags", body: `{"tags":[]}`,
		},
		{
			name:   "disable key expiry",
			tool:   "tailscale_set_device_key_expiry",
			args:   map[string]any{"device_id": "n1", "key_expiry_disabled": true},
			method: http.MethodPost, path: "/api/v2/device/n1/key", body: `{"keyExpiryDisabled":true}`,
		},
		{
			name:   "expire key",
			tool:   "tailscale_expire_device_key",
			args:   map[string]any{"device_id": "n1"},
			method: http.MethodPost, path: "/api/v2/device/n1/expire",
		},
		{
			name:   "set ip",
			tool:   "tailscale_set_device_ip",
			args:   map[string]any{"device_id": "n1", "ipv4": "100.80.0.1"},
			method: http.MethodPost, path: "/api/v2/device/n1/ip", body: `{"ipv4":"100.80.0.1"}`,
		},
		{
			name:   "get routes",
			tool:   "tailscale_get_device_routes",
			args:   map[string]any{"device_id": "n1"},
			method: http.MethodGet, path: "/api/v2/device/n1/routes",
		},
		{
			name:   "set routes for exit node",
			tool:   "tailscale_set_device_routes",
			args:   map[string]any{"device_id": "n1", "routes": []any{"0.0.0.0/0", "::/0"}},
			method: http.MethodPost, path: "/api/v2/device/n1/routes", body: `{"routes":["0.0.0.0/0","::/0"]}`,
		},
		{
			name:   "get posture attributes",
			tool:   "tailscale_get_device_posture_attributes",
			args:   map[string]any{"device_id": "n1"},
			method: http.MethodGet, path: "/api/v2/device/n1/attributes",
		},
		{
			name:   "set posture attribute",
			tool:   "tailscale_set_device_posture_attribute",
			args:   map[string]any{"device_id": "n1", "attribute_key": "custom:tier", "value": "gold", "comment": "promo"},
			method: http.MethodPost, path: "/api/v2/device/n1/attributes/custom:tier", body: `{"value":"gold","comment":"promo"}`,
		},
		{
			name:   "delete posture attribute",
			tool:   "tailscale_delete_device_posture_attribute",
			args:   map[string]any{"device_id": "n1", "attribute_key": "custom:tier"},
			method: http.MethodDelete, path: "/api/v2/device/n1/attributes/custom:tier",
		},
		{
			name:   "list keys all",
			tool:   "tailscale_list_keys",
			args:   map[string]any{"all": true},
			method: http.MethodGet, path: "/api/v2/tailnet/-/keys", query: "all=true",
		},
		{
			name:   "get key",
			tool:   "tailscale_get_key",
			args:   map[string]any{"key_id": "k1"},
			method: http.MethodGet, path: "/api/v2/tailnet/-/keys/k1",
		},
		{
			name: "create auth key",
			tool: "tailscale_create_auth_key",
			args: map[string]any{
				"reusable": true, "ephemeral": true, "preauthorized": true,
				"tags": []any{"tag:ci"}, "expiry_seconds": 3600, "description": "ci runners",
			},
			method: http.MethodPost, path: "/api/v2/tailnet/-/keys",
			body: `{"keyType":"auth","capabilities":{"devices":{"create":{"reusable":true,"ephemeral":true,"preauthorized":true,"tags":["tag:ci"]}}},"expirySeconds":3600,"description":"ci runners"}`,
		},
		{
			name:   "create minimal auth key",
			tool:   "tailscale_create_auth_key",
			args:   map[string]any{},
			method: http.MethodPost, path: "/api/v2/tailnet/-/keys",
			body: `{"keyType":"auth","capabilities":{"devices":{"create":{"reusable":false,"ephemeral":false,"preauthorized":false}}}}`,
		},
		{
			name:   "delete key",
			tool:   "tailscale_delete_key",
			args:   map[string]any{"key_id": "k1"},
			method: http.MethodDelete, path: "/api/v2/tailnet/-/keys/k1",
		},
		{
			name:   "get dns",
			tool:   "tailscale_get_dns",
			method: http.MethodGet, path: "/api/v2/tailnet/-/dns/configuration",
		},
		{
			name:   "set nameservers",
			tool:   "tailscale_set_dns_nameservers",
			args:   map[string]any{"nameservers": []any{"1.1.1.1", "8.8.8.8"}},
			method: http.MethodPost, path: "/api/v2/tailnet/-/dns/nameservers", body: `{"dns":["1.1.1.1","8.8.8.8"]}`,
		},
		{
			name:   "set magic dns",
			tool:   "tailscale_set_dns_preferences",
			args:   map[string]any{"magic_dns": true},
			method: http.MethodPost, path: "/api/v2/tailnet/-/dns/preferences", body: `{"magicDNS":true}`,
		},
		{
			name:   "set search paths",
			tool:   "tailscale_set_dns_search_paths",
			args:   map[string]any{"search_paths": []any{"corp.example.com"}},
			method: http.MethodPost, path: "/api/v2/tailnet/-/dns/searchpaths", body: `{"searchPaths":["corp.example.com"]}`,
		},
		{
			name:   "update split dns",
			tool:   "tailscale_update_split_dns",
			args:   map[string]any{"split_dns": map[string]any{"corp.example.com": []any{"10.0.0.53"}, "old.example.com": nil}},
			method: http.MethodPatch, path: "/api/v2/tailnet/-/dns/split-dns", body: `{"corp.example.com":["10.0.0.53"],"old.example.com":null}`,
		},
		{
			name:   "list users filtered",
			tool:   "tailscale_list_users",
			args:   map[string]any{"type": "all", "role": "admin"},
			method: http.MethodGet, path: "/api/v2/tailnet/-/users", query: "role=admin&type=all",
		},
		{
			name:   "get user",
			tool:   "tailscale_get_user",
			args:   map[string]any{"user_id": "u1"},
			method: http.MethodGet, path: "/api/v2/users/u1",
		},
		{
			name:   "update user role",
			tool:   "tailscale_update_user_role",
			args:   map[string]any{"user_id": "u1", "role": "admin"},
			method: http.MethodPost, path: "/api/v2/users/u1/role", body: `{"role":"admin"}`,
		},
		{name: "approve user", tool: "tailscale_approve_user", args: map[string]any{"user_id": "u1"}, method: http.MethodPost, path: "/api/v2/users/u1/approve"},
		{name: "suspend user", tool: "tailscale_suspend_user", args: map[string]any{"user_id": "u1"}, method: http.MethodPost, path: "/api/v2/users/u1/suspend"},
		{name: "restore user", tool: "tailscale_restore_user", args: map[string]any{"user_id": "u1"}, method: http.MethodPost, path: "/api/v2/users/u1/restore"},
		{name: "delete user", tool: "tailscale_delete_user", args: map[string]any{"user_id": "u1"}, method: http.MethodPost, path: "/api/v2/users/u1/delete"},
		{
			name:   "list user invites",
			tool:   "tailscale_list_user_invites",
			method: http.MethodGet, path: "/api/v2/tailnet/-/user-invites",
			respBody: `[]`,
		},
		{
			name:   "create user invite",
			tool:   "tailscale_create_user_invite",
			args:   map[string]any{"email": "ada@example.com", "role": "admin"},
			method: http.MethodPost, path: "/api/v2/tailnet/-/user-invites", body: `[{"email":"ada@example.com","role":"admin"}]`,
			respBody: `[{"id":"1"}]`,
		},
		{
			name:   "delete user invite",
			tool:   "tailscale_delete_user_invite",
			args:   map[string]any{"invite_id": "i1"},
			method: http.MethodDelete, path: "/api/v2/user-invites/i1",
		},
		{
			name:   "get settings",
			tool:   "tailscale_get_tailnet_settings",
			method: http.MethodGet, path: "/api/v2/tailnet/-/settings",
		},
		{
			name:   "update settings",
			tool:   "tailscale_update_tailnet_settings",
			args:   map[string]any{"settings": map[string]any{"devicesApprovalOn": true, "devicesKeyDurationDays": float64(30)}},
			method: http.MethodPatch, path: "/api/v2/tailnet/-/settings", body: `{"devicesApprovalOn":true,"devicesKeyDurationDays":30}`,
		},
		{
			name:   "create webhook",
			tool:   "tailscale_create_webhook",
			args:   map[string]any{"endpoint_url": "https://hooks.example.com/ts", "subscriptions": []any{"nodeCreated"}, "provider_type": "slack"},
			method: http.MethodPost, path: "/api/v2/tailnet/-/webhooks",
			body: `{"endpointUrl":"https://hooks.example.com/ts","subscriptions":["nodeCreated"],"providerType":"slack"}`,
		},
		{
			name:   "delete webhook",
			tool:   "tailscale_delete_webhook",
			args:   map[string]any{"webhook_id": "w1"},
			method: http.MethodDelete, path: "/api/v2/webhooks/w1",
		},
		{
			name:   "test webhook",
			tool:   "tailscale_test_webhook",
			args:   map[string]any{"webhook_id": "w1"},
			method: http.MethodPost, path: "/api/v2/webhooks/w1/test",
		},
		{
			name:   "get contacts",
			tool:   "tailscale_get_contacts",
			method: http.MethodGet, path: "/api/v2/tailnet/-/contacts",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.respBody
			if resp == "" {
				resp = `{"ok":true}`
			}
			ts, reqs := testServer(t, okJSON(resp))
			result, err := ts.Execute(context.Background(), mcp.ToolName(tt.tool), tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Len(t, *reqs, 1)
			got := (*reqs)[0]
			assert.Equal(t, tt.method, got.method)
			assert.Equal(t, tt.path, got.path)
			assert.Equal(t, tt.query, got.query)
			if tt.body == "" {
				assert.Empty(t, got.body)
			} else {
				assert.JSONEq(t, tt.body, got.body)
				assert.Equal(t, "application/json", got.headers.Get("Content-Type"))
			}
		})
	}
}

func TestHandlers_CustomTailnetInPath(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"devices":[]}`))
	}))
	defer srv.Close()
	ts := &tailscale{client: srv.Client()}
	require.NoError(t, ts.Configure(context.Background(), mcp.Credentials{"api_key": "tskey-api-x", "tailnet": "example.com", "base_url": srv.URL}))
	_, err := ts.Execute(context.Background(), "tailscale_list_devices", nil)
	require.NoError(t, err)
	assert.Equal(t, "/api/v2/tailnet/example.com/devices", path)
}

func TestHandlers_PathEscapesIDs(t *testing.T) {
	var escaped string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	ts := &tailscale{client: srv.Client()}
	require.NoError(t, ts.Configure(context.Background(), mcp.Credentials{"api_key": "tskey-api-x", "base_url": srv.URL}))
	_, err := ts.Execute(context.Background(), "tailscale_get_device", map[string]any{"device_id": "../keys"})
	require.NoError(t, err)
	assert.Equal(t, "/api/v2/device/..%2Fkeys", escaped)
}

func TestHandlers_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"get device missing id", "tailscale_get_device", map[string]any{}, "device_id is required"},
		{"rename missing name", "tailscale_set_device_name", map[string]any{"device_id": "n1"}, "name is required"},
		{"tags missing", "tailscale_set_device_tags", map[string]any{"device_id": "n1"}, "tags is required"},
		{"routes missing", "tailscale_set_device_routes", map[string]any{"device_id": "n1"}, "routes is required"},
		{"key expiry missing flag", "tailscale_set_device_key_expiry", map[string]any{"device_id": "n1"}, "key_expiry_disabled is required"},
		{"set ip missing", "tailscale_set_device_ip", map[string]any{"device_id": "n1"}, "ipv4 is required"},
		{"posture non-custom key", "tailscale_set_device_posture_attribute", map[string]any{"device_id": "n1", "attribute_key": "node:os", "value": "x"}, "custom:"},
		{"posture missing value", "tailscale_set_device_posture_attribute", map[string]any{"device_id": "n1", "attribute_key": "custom:x"}, "value is required"},
		{"posture bad value type", "tailscale_set_device_posture_attribute", map[string]any{"device_id": "n1", "attribute_key": "custom:x", "value": []any{"a"}}, "string, number, or boolean"},
		{"get key missing", "tailscale_get_key", map[string]any{}, "key_id is required"},
		{"magic dns missing", "tailscale_set_dns_preferences", map[string]any{}, "magic_dns is required"},
		{"nameservers missing", "tailscale_set_dns_nameservers", map[string]any{}, "nameservers is required"},
		{"split dns missing", "tailscale_update_split_dns", map[string]any{}, "split_dns is required"},
		{"role missing", "tailscale_update_user_role", map[string]any{"user_id": "u1"}, "role is required"},
		{"settings missing", "tailscale_update_tailnet_settings", map[string]any{}, "settings is required"},
		{"webhook url missing", "tailscale_create_webhook", map[string]any{"subscriptions": []any{"nodeCreated"}}, "endpoint_url is required"},
		{"webhook subs missing", "tailscale_create_webhook", map[string]any{"endpoint_url": "https://x"}, "subscriptions is required"},
		{"policy missing", "tailscale_set_policy_file", map[string]any{"if_match": "abc"}, "policy is required"},
		{"policy if_match missing", "tailscale_set_policy_file", map[string]any{"policy": "{}"}, "if_match is required"},
		{"validate nothing", "tailscale_validate_policy_file", map[string]any{}, "policy or tests is required"},
		{"preview missing type", "tailscale_preview_policy_rules", map[string]any{"preview_for": "a@b.c"}, "type is required"},
		{"preview bad type", "tailscale_preview_policy_rules", map[string]any{"type": "group", "preview_for": "a@b.c"}, "user or ipport"},
		{"wrong arg type", "tailscale_list_keys", map[string]any{"all": []any{}}, "cannot convert"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, reqs := testServer(t, okJSON(`{}`))
			result, err := ts.Execute(context.Background(), mcp.ToolName(tt.tool), tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
			assert.Empty(t, *reqs, "no API call should be made on validation failure")
		})
	}
}

func TestGetPolicyFile_ReturnsHuJSONAndETag(t *testing.T) {
	policy := "// comment kept\n{\"acls\": [{\"action\": \"accept\", \"src\": [\"*\"], \"dst\": [\"*:*\"]},],}"
	ts, reqs := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"e0b2816b418"`)
		w.Header().Set("Content-Type", "application/hujson")
		_, _ = w.Write([]byte(policy))
	})
	result, err := ts.Execute(context.Background(), "tailscale_get_policy_file", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "application/hujson", (*reqs)[0].headers.Get("Accept"))
	assert.Equal(t, "/api/v2/tailnet/-/acl", (*reqs)[0].path)
	var out map[string]string
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	assert.Equal(t, `"e0b2816b418"`, out["etag"])
	assert.Equal(t, policy, out["policy"])
}

func TestSetPolicyFile_SendsHuJSONWithIfMatch(t *testing.T) {
	tests := []struct {
		name     string
		ifMatch  string
		wantHdr  string
		policy   string
		respETag string
	}{
		{"quoted etag passes through", `"abc123"`, `"abc123"`, "{\"acls\":[]}", `"def456"`},
		{"bare etag gets quoted", "abc123", `"abc123"`, "// c\n{}", `"def456"`},
		{"ts-default sentinel quoted like the official tailscale-client-go-v2", "ts-default", `"ts-default"`, "{}", `"x"`},
		{"quoted ts-default passes through", `"ts-default"`, `"ts-default"`, "{}", `"x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, reqs := testServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", tt.respETag)
				_, _ = w.Write([]byte(tt.policy))
			})
			result, err := ts.Execute(context.Background(), "tailscale_set_policy_file", map[string]any{"policy": tt.policy, "if_match": tt.ifMatch})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			got := (*reqs)[0]
			assert.Equal(t, http.MethodPost, got.method)
			assert.Equal(t, "/api/v2/tailnet/-/acl", got.path)
			assert.Equal(t, "application/hujson", got.headers.Get("Content-Type"))
			assert.Equal(t, tt.wantHdr, got.headers.Get("If-Match"))
			assert.Equal(t, tt.policy, got.body)
			var out map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
			assert.Equal(t, "updated", out["status"])
			assert.Equal(t, tt.respETag, out["etag"])
		})
	}
}

func TestSetPolicyFile_PreconditionFailed(t *testing.T) {
	ts, _ := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`{"message":"precondition failed, invalid old hash"}`))
	})
	result, err := ts.Execute(context.Background(), "tailscale_set_policy_file", map[string]any{"policy": "{}", "if_match": "stale"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "412")
	assert.Contains(t, result.Data, "get_policy_file")
}

func TestValidatePolicyFile(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		resp        string
		wantCT      string
		wantBody    string
		wantContain string
	}{
		{
			name:        "valid policy",
			args:        map[string]any{"policy": "// hi\n{\"acls\":[]}"},
			resp:        `{}`,
			wantCT:      "application/hujson",
			wantBody:    "// hi\n{\"acls\":[]}",
			wantContain: `"valid":true`,
		},
		{
			name:        "invalid policy surfaces message",
			args:        map[string]any{"policy": "{}"},
			resp:        `{"message":"test(s) failed","data":[{"user":"a@b.c","errors":["x"]}]}`,
			wantCT:      "application/hujson",
			wantBody:    "{}",
			wantContain: "test(s) failed",
		},
		{
			name:        "tests array",
			args:        map[string]any{"tests": []any{map[string]any{"src": "a@b.c", "accept": []any{"web:443"}}}},
			resp:        ``,
			wantCT:      "application/json",
			wantBody:    `[{"src":"a@b.c","accept":["web:443"]}]`,
			wantContain: `"valid":true`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, reqs := testServer(t, okJSON(tt.resp))
			result, err := ts.Execute(context.Background(), "tailscale_validate_policy_file", tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			got := (*reqs)[0]
			assert.Equal(t, "/api/v2/tailnet/-/acl/validate", got.path)
			assert.Equal(t, tt.wantCT, got.headers.Get("Content-Type"))
			if tt.wantCT == "application/json" {
				assert.JSONEq(t, tt.wantBody, got.body)
			} else {
				assert.Equal(t, tt.wantBody, got.body)
			}
			assert.Contains(t, result.Data, tt.wantContain)
		})
	}
}

func TestPreviewPolicyRules_UsesCurrentPolicyWhenOmitted(t *testing.T) {
	current := "{\"acls\":[{\"action\":\"accept\",\"src\":[\"*\"],\"dst\":[\"*:*\"]}]}"
	ts, reqs := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(current))
			return
		}
		_, _ = w.Write([]byte(`{"matches":[{"users":["*"],"ports":["*:*"],"lineNumber":1}],"type":"user","previewFor":"ada@example.com"}`))
	})
	result, err := ts.Execute(context.Background(), "tailscale_preview_policy_rules", map[string]any{"type": "user", "preview_for": "ada@example.com"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Len(t, *reqs, 2)
	assert.Equal(t, http.MethodGet, (*reqs)[0].method)
	post := (*reqs)[1]
	assert.Equal(t, "/api/v2/tailnet/-/acl/preview", post.path)
	assert.Equal(t, "previewFor=ada%40example.com&type=user", post.query)
	assert.Equal(t, current, post.body)
	assert.Equal(t, "application/hujson", post.headers.Get("Content-Type"))
	assert.Contains(t, result.Data, "matches")
}

func TestPreviewPolicyRules_WithProposedPolicy(t *testing.T) {
	ts, reqs := testServer(t, okJSON(`{"matches":[]}`))
	result, err := ts.Execute(context.Background(), "tailscale_preview_policy_rules", map[string]any{"type": "ipport", "preview_for": "100.64.0.1:22", "policy": "{}"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Len(t, *reqs, 1)
	assert.Equal(t, "{}", (*reqs)[0].body)
}

func TestListUserInvites_NormalizesEnvelope(t *testing.T) {
	tests := []struct {
		name string
		resp string
		want string
	}{
		{"bare array", `[{"id":"1","email":"a@b.c"}]`, `{"invites":[{"id":"1","email":"a@b.c"}]}`},
		{"already wrapped", `{"invites":[{"id":"1"}]}`, `{"invites":[{"id":"1"}]}`},
		{"empty array", `[]`, `{"invites":[]}`},
		{"null", `null`, `{"invites":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _ := testServer(t, okJSON(tt.resp))
			result, err := ts.Execute(context.Background(), "tailscale_list_user_invites", nil)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.JSONEq(t, tt.want, result.Data)
		})
	}
}

func TestListWebhooks_StripsSecrets(t *testing.T) {
	ts, _ := testServer(t, okJSON(`{"webhooks":[{"endpointId":"w1","endpointUrl":"https://x","secret":"SHHH","subscriptions":["nodeCreated"]}]}`))
	result, err := ts.Execute(context.Background(), "tailscale_list_webhooks", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.NotContains(t, result.Data, "SHHH")
	assert.Contains(t, result.Data, "w1")
}

func TestSetPostureAttribute_ValueTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"number", float64(42), `{"value":42}`},
		{"bool", true, `{"value":true}`},
		{"string with expiry", "gold", `{"value":"gold","expiry":"2030-01-01T00:00:00Z"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, reqs := testServer(t, okJSON(`{}`))
			args := map[string]any{"device_id": "n1", "attribute_key": "custom:x", "value": tt.value}
			if strings.Contains(tt.want, "expiry") {
				args["expiry"] = "2030-01-01T00:00:00Z"
			}
			result, err := ts.Execute(context.Background(), "tailscale_set_device_posture_attribute", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.JSONEq(t, tt.want, (*reqs)[0].body)
		})
	}
}
