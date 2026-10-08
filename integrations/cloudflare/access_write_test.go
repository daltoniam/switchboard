package cloudflare

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRequest records the method, path and decoded JSON body of the last
// request and replies with a fixed Cloudflare envelope.
type capturedRequest struct {
	method string
	path   string
	body   map[string]any
}

func captureClient(t *testing.T, reply string) (*cloudflare, *capturedRequest, func()) {
	t.Helper()
	captured := &capturedRequest{}
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.EscapedPath()
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			require.NoError(t, json.Unmarshal(raw, &captured.body))
		}
		_, _ = w.Write([]byte(reply))
	}))
	return c, captured, ts.Close
}

func TestGetAccessApp(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{"id":"a1","aud":"aud-tag"}}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_get_access_app", map[string]any{"app_id": "a1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/accounts/test-acct/access/apps/a1", got.path)
	assert.Contains(t, result.Data, "aud-tag")
}

func TestCreateAccessApp(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{"id":"a1","aud":"aud-tag"}}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_create_access_app", map[string]any{
		"name":                      "overload",
		"domain":                    "overload.example.com",
		"session_duration":          "12h",
		"allowed_idps":              "idp-1,idp-2",
		"auto_redirect_to_identity": true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/accounts/test-acct/access/apps", got.path)
	assert.Equal(t, map[string]any{
		"name":                      "overload",
		"domain":                    "overload.example.com",
		"type":                      "self_hosted",
		"session_duration":          "12h",
		"allowed_idps":              []any{"idp-1", "idp-2"},
		"auto_redirect_to_identity": true,
	}, got.body)
}

func TestCreateAccessApp_Validation(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing name", map[string]any{"domain": "a.example.com"}},
		{"missing domain", map[string]any{"name": "a"}},
		{"domain with scheme", map[string]any{"name": "a", "domain": "https://a.example.com"}},
		{"unknown type", map[string]any{"name": "a", "domain": "a.example.com", "type": "nope"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, got, done := captureClient(t, `{"success":true,"result":{}}`)
			defer done()
			result, err := c.Execute(context.Background(), "cloudflare_create_access_app", tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Empty(t, got.method, "no request should be sent")
		})
	}
}

func TestDeleteAccessApp(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{"id":"a1"}}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_delete_access_app", map[string]any{"app_id": "a1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "DELETE", got.method)
	assert.Equal(t, "/accounts/test-acct/access/apps/a1", got.path)
}

func TestCreateAccessAppPolicy(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		include []any
	}{
		{
			name: "emails and domains",
			args: map[string]any{"emails": "a@example.com,b@example.com", "email_domains": "example.org"},
			include: []any{
				map[string]any{"email": map[string]any{"email": "a@example.com"}},
				map[string]any{"email": map[string]any{"email": "b@example.com"}},
				map[string]any{"email_domain": map[string]any{"domain": "example.org"}},
			},
		},
		{
			name:    "everyone",
			args:    map[string]any{"everyone": true, "decision": "bypass"},
			include: []any{map[string]any{"everyone": map[string]any{}}},
		},
		{
			name: "login methods and raw rules",
			args: map[string]any{
				"login_methods": []any{"idp-1"},
				"include":       []any{map[string]any{"group": map[string]any{"id": "g1"}}},
			},
			include: []any{
				map[string]any{"login_method": map[string]any{"id": "idp-1"}},
				map[string]any{"group": map[string]any{"id": "g1"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, got, done := captureClient(t, `{"success":true,"result":{"id":"p1"}}`)
			defer done()
			args := map[string]any{"app_id": "a1", "name": "people"}
			for k, v := range tt.args {
				args[k] = v
			}
			result, err := c.Execute(context.Background(), "cloudflare_create_access_app_policy", args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, "POST", got.method)
			assert.Equal(t, "/accounts/test-acct/access/apps/a1/policies", got.path)
			assert.Equal(t, "people", got.body["name"])
			decision, _ := tt.args["decision"].(string)
			if decision == "" {
				decision = "allow"
			}
			assert.Equal(t, decision, got.body["decision"])
			assert.Equal(t, tt.include, got.body["include"])
		})
	}
}

func TestCreateAccessAppPolicy_Validation(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
	}{
		{"no include rules", map[string]any{"app_id": "a1", "name": "p"}},
		{"bad decision", map[string]any{"app_id": "a1", "name": "p", "everyone": true, "decision": "maybe"}},
		{"missing app", map[string]any{"name": "p", "everyone": true}},
		{"include not a list of objects", map[string]any{"app_id": "a1", "name": "p", "include": []any{"x"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, got, done := captureClient(t, `{"success":true,"result":{}}`)
			defer done()
			result, err := c.Execute(context.Background(), "cloudflare_create_access_app_policy", tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Empty(t, got.method, "no request should be sent")
		})
	}
}

func TestDeleteAccessAppPolicy(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{"id":"p1"}}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_delete_access_app_policy", map[string]any{"app_id": "a1", "policy_id": "p1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "DELETE", got.method)
	assert.Equal(t, "/accounts/test-acct/access/apps/a1/policies/p1", got.path)
}

func TestAccessIDsAreEscaped(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{}}`)
	defer done()
	_, err := c.Execute(context.Background(), "cloudflare_get_access_app", map[string]any{"app_id": "../zones"})
	require.NoError(t, err)
	assert.Equal(t, "/accounts/test-acct/access/apps/..%2Fzones", got.path)
}

func TestCreateTunnel(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{"id":"t1","name":"home"}}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_create_tunnel", map[string]any{"name": "home"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "POST", got.method)
	assert.Equal(t, "/accounts/test-acct/cfd_tunnel", got.path)
	assert.Equal(t, map[string]any{"name": "home", "config_src": "cloudflare"}, got.body)
}

func TestGetTunnelToken(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":"eyJ0b2tlbiI6"}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_get_tunnel_token", map[string]any{"tunnel_id": "t1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/accounts/test-acct/cfd_tunnel/t1/token", got.path)
}

func TestGetTunnelConfig(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{"config":{"ingress":[]}}}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_get_tunnel_config", map[string]any{"tunnel_id": "t1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "GET", got.method)
	assert.Equal(t, "/accounts/test-acct/cfd_tunnel/t1/configurations", got.path)
}

func TestUpdateTunnelConfig(t *testing.T) {
	tests := []struct {
		name    string
		ingress []any
		want    []any
	}{
		{
			name:    "adds a 404 catch-all",
			ingress: []any{map[string]any{"hostname": "app.example.com", "service": "http://app:8080"}},
			want: []any{
				map[string]any{"hostname": "app.example.com", "service": "http://app:8080"},
				map[string]any{"service": "http_status:404"},
			},
		},
		{
			name: "keeps an explicit catch-all",
			ingress: []any{
				map[string]any{"hostname": "app.example.com", "path": "^/webhooks$", "service": "http://app:8080"},
				map[string]any{"service": "http_status:403"},
			},
			want: []any{
				map[string]any{"hostname": "app.example.com", "path": "^/webhooks$", "service": "http://app:8080"},
				map[string]any{"service": "http_status:403"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, got, done := captureClient(t, `{"success":true,"result":{}}`)
			defer done()
			result, err := c.Execute(context.Background(), "cloudflare_update_tunnel_config", map[string]any{"tunnel_id": "t1", "ingress": tt.ingress})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, "PUT", got.method)
			assert.Equal(t, "/accounts/test-acct/cfd_tunnel/t1/configurations", got.path)
			assert.Equal(t, map[string]any{"config": map[string]any{"ingress": tt.want}}, got.body)
		})
	}
}

func TestUpdateTunnelConfig_Validation(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing ingress", map[string]any{"tunnel_id": "t1"}},
		{"rule without service", map[string]any{"tunnel_id": "t1", "ingress": []any{map[string]any{"hostname": "a.example.com"}}}},
		{"rule not an object", map[string]any{"tunnel_id": "t1", "ingress": []any{"http://app"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, got, done := captureClient(t, `{"success":true,"result":{}}`)
			defer done()
			result, err := c.Execute(context.Background(), "cloudflare_update_tunnel_config", tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Empty(t, got.method, "no request should be sent")
		})
	}
}

func TestObjectList_AcceptsJSONStrings(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    []map[string]any
		wantErr bool
	}{
		{"json array", `[{"service":"http://app:8080"}]`, []map[string]any{{"service": "http://app:8080"}}, false},
		{"blank string", "  ", nil, false},
		{"invalid json", `[{"service"`, nil, true},
		{"json object, not array", `{"service":"x"}`, nil, true},
		{"json array of strings", `["x"]`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := objectList(map[string]any{"ingress": tt.value}, "ingress")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUpdateTunnelConfig_JSONStringIngress(t *testing.T) {
	c, got, done := captureClient(t, `{"success":true,"result":{}}`)
	defer done()
	result, err := c.Execute(context.Background(), "cloudflare_update_tunnel_config", map[string]any{
		"tunnel_id": "t1",
		"ingress":   `[{"hostname":"app.example.com","service":"http://app:8080"}]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, map[string]any{"config": map[string]any{"ingress": []any{
		map[string]any{"hostname": "app.example.com", "service": "http://app:8080"},
		map[string]any{"service": "http_status:404"},
	}}}, got.body)
}
