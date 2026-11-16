package snipeit

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

const assetFixture = `{"id":7,"name":"Incident laptop","asset_tag":"IT & 7","serial":"SERIAL-7","model":{"id":2,"name":"Laptop","extra":"SECRET"},"status_label":{"id":3,"name":"Ready"},"location":{"id":4,"name":"Office"},"assigned_to":{"id":5,"type":"user","name":"SECRET","email":"SECRET"},"notes":"SECRET","custom_fields":{"password":"SECRET"},"purchase_cost":"SECRET"}`

func testAdapter(t *testing.T, handler http.HandlerFunc) (*snipeit, *httptest.Server) {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	adapter := New().(*snipeit)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL + "/inventory", "token": "test-secret", "ca_cert": string(cert)}))
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	return adapter, provider
}

func TestConfigure(t *testing.T) {
	cases := []struct{ name, key, value string }{
		{"missing url", "base_url", ""}, {"http", "base_url", "http://inventory.example"},
		{"userinfo", "base_url", "https://user:secret@inventory.example"}, {"query", "base_url", "https://inventory.example?x=1"},
		{"fragment", "base_url", "https://inventory.example#x"}, {"traversal", "base_url", "https://inventory.example/a/../b"},
		{"encoded path", "base_url", "https://inventory.example/%2f"}, {"port", "base_url", "https://inventory.example:0"},
		{"missing token", "token", ""}, {"token newline", "token", "secret\n"}, {"ca", "ca_cert", "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter := New()
			creds := mcp.Credentials{"base_url": "https://inventory.example", "token": "test-secret"}
			require.NoError(t, adapter.Configure(t.Context(), creds))
			creds[tc.key] = tc.value
			err := adapter.Configure(t.Context(), creds)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "test-secret")
			require.False(t, adapter.Healthy(t.Context()))
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	adapter := New()
	require.Equal(t, "snipeit", adapter.Name())
	require.Len(t, adapter.Tools(), 3)
	require.Contains(t, adapter.Tools()[0].Description, "Start here")
	seen := map[mcp.ToolName]bool{}
	for _, tool := range adapter.Tools() {
		require.NotEmpty(t, tool.Description)
		require.True(t, strings.HasPrefix(string(tool.Name), "snipeit_"))
		require.Equal(t, mcp.AccessRead, tool.Access)
		require.False(t, seen[tool.Name])
		seen[tool.Name] = true
		require.Contains(t, dispatch, tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	for name := range dispatch {
		found := false
		for _, tool := range New().Tools() {
			found = found || name == tool.Name
		}
		require.True(t, found, name)
	}
}

func TestAssets(t *testing.T) {
	cases := []struct {
		name              mcp.ToolName
		args              map[string]any
		path, query, body string
	}{
		{"snipeit_list_assets", map[string]any{"search": "IT & 7", "status": "ready", "offset": 10, "limit": 1}, "/inventory/api/v1/hardware", "limit=1&offset=10&order=asc&search=IT+%26+7&sort=id&status=RTD", `{"total":12,"rows":[` + assetFixture + `],"next_page_url":"SECRET"}`},
		{"snipeit_get_asset", map[string]any{"id": 7}, "/inventory/api/v1/hardware/7", "", assetFixture},
		{"snipeit_get_asset_by_tag", map[string]any{"tag": "IT & 7"}, "/inventory/api/v1/hardware/bytag/IT & 7", "limit=100&offset=0", assetFixture},
		{"snipeit_get_asset_by_tag", map[string]any{"tag": "IT & 7"}, "/inventory/api/v1/hardware/bytag/IT & 7", "limit=100&offset=0", `{"total":1,"rows":[` + assetFixture + `]}`},
	}
	for index, tc := range cases {
		t.Run(string(tc.name)+string(rune('a'+index)), func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
				if tc.name == "snipeit_get_asset_by_tag" && r.URL.Path == "/inventory/api/v1/hardware/7" {
					require.Empty(t, r.URL.RawQuery)
					_, _ = io.WriteString(w, assetFixture)
					return
				}
				require.Equal(t, tc.path, r.URL.Path)
				require.Equal(t, tc.query, r.URL.RawQuery)
				_, _ = io.WriteString(w, tc.body)
			})
			result, err := adapter.Execute(t.Context(), tc.name, tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, "Incident laptop")
			require.Contains(t, result.Data, `"type":"user"`)
			require.NotContains(t, result.Data, "SECRET")
			require.NotContains(t, result.Data, "test-secret")
			var value any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
			if tc.name == "snipeit_list_assets" {
				require.Equal(t, float64(11), value.(map[string]any)["next_offset"])
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	var calls atomic.Int32
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); _, _ = io.WriteString(w, assetFixture) })
	cases := []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"snipeit_get_asset", nil}, {"snipeit_get_asset", map[string]any{"id": -1}}, {"snipeit_get_asset", map[string]any{"id": "7/checkout"}},
		{"snipeit_get_asset_by_tag", map[string]any{"tag": "../users"}}, {"snipeit_get_asset_by_tag", map[string]any{"tag": "x%2fy"}},
		{"snipeit_get_asset_by_tag", map[string]any{"tag": "a/b"}}, {"snipeit_get_asset_by_tag", map[string]any{"tag": "\n"}},
		{"snipeit_list_assets", map[string]any{"limit": 0}}, {"snipeit_list_assets", map[string]any{"limit": 101}},
		{"snipeit_list_assets", map[string]any{"offset": -1}}, {"snipeit_list_assets", map[string]any{"offset": 1.5}},
		{"snipeit_list_assets", map[string]any{"status": "arbitrary"}}, {"snipeit_list_assets", map[string]any{"search": true}},
		{"snipeit_list_assets", map[string]any{"base_url": "https://other.example"}},
	}
	for index, tc := range cases {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), tc.name, tc.args)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
		})
	}
	require.Zero(t, calls.Load())
}

func TestUpstreamErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"missing", 200, `{"status":"error","messages":"test-secret SECRET","payload":null}`},
		{"forbidden", 403, `test-secret SECRET`}, {"rate limited", 429, `test-secret SECRET`},
		{"redirect", 302, ""}, {"empty", 204, ""}, {"bad json", 200, `not json test-secret SECRET`},
		{"oversized", 200, strings.Repeat(" ", 2*1024*1024+1)},
		{"wrong shape", 200, `{}`}, {"null", 200, `null`},
		{"mismatched id", 200, strings.Replace(assetFixture, `"id":7`, `"id":8`, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			result, err := adapter.Execute(t.Context(), "snipeit_get_asset", map[string]any{"id": 7})
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "test-secret")
			require.NotContains(t, result.Data, "SECRET")
		})
	}
}

func TestLifecycle(t *testing.T) {
	adapter := New().(*snipeit)
	require.False(t, adapter.Healthy(t.Context()))
	for _, name := range []mcp.ToolName{"unknown", "snipeit_list_assets"} {
		result, err := adapter.Execute(t.Context(), name, nil)
		require.NoError(t, err)
		require.True(t, result.IsError)
	}
	configured, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"total":0,"rows":[]}`) })
	require.True(t, configured.Healthy(t.Context()))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.False(t, configured.Healthy(ctx))
	require.NoError(t, configured.Close())
	require.NoError(t, configured.Close())
	require.False(t, configured.Healthy(t.Context()))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	adapter := New().(*snipeit)
	require.Len(t, fieldCompactionSpecs, 3)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
		fields, ok := adapter.CompactSpec(name)
		require.True(t, ok)
		require.NotEmpty(t, fields)
		var projectedAsset asset
		require.NoError(t, json.Unmarshal([]byte(assetFixture), &projectedAsset))
		var input any = projectedAsset
		if name != "snipeit_get_asset" {
			input = assetPage{Total: 1, Rows: []asset{projectedAsset}}
		}
		payload, err := json.Marshal(input)
		require.NoError(t, err)
		var raw any
		require.NoError(t, json.Unmarshal(payload, &raw))
		projected := mcp.CompactAny(raw, fields)
		encoded, err := json.Marshal(projected)
		require.NoError(t, err)
		require.Contains(t, string(encoded), "Incident laptop")
		require.NotContains(t, string(encoded), "SECRET")
	}
}
