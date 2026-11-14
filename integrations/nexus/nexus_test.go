package nexus

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

const componentFixture = `{"id":"bWF2ZW46MQ","repository":"maven-releases","format":"maven2","group":"org.example","name":"demo","version":"1.0","downloadUrl":"SECRET_SENTINEL","assets":[{"id":"asset-1","path":"org/example/demo/1.0/demo-1.0.pom","contentType":"application/xml","fileSize":42,"checksum":{"sha1":"abc","unexpected":"SECRET_SENTINEL"},"lastModified":"2026-11-01T10:00:00Z","uploader":"SECRET_SENTINEL","downloadUrl":"SECRET_SENTINEL"}]}`

func fixture(t *testing.T, handler http.HandlerFunc) (*nexus, mcp.Credentials) {
	t.Helper()
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "reader", user)
		require.Equal(t, "fixture-password", password)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(provider.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}))
	creds := mcp.Credentials{"base_url": provider.URL + "/prefix", "username": "reader", "password": "fixture-password", "repositories": "maven-releases,npm-private", "ca_cert": cert}
	adapter := New().(*nexus)
	require.NoError(t, adapter.Configure(t.Context(), creds))
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	return adapter, creds
}

func execute(t *testing.T, adapter mcp.Integration, name mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := adapter.Execute(t.Context(), name, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestConfigure(t *testing.T) {
	for _, test := range []struct{ name, key, value string }{
		{"missing URL", "base_url", ""}, {"HTTP", "base_url", "http://localhost"},
		{"userinfo", "base_url", "https://reader:secret@example.com"}, {"query", "base_url", "https://example.com?token=secret"},
		{"fragment", "base_url", "https://example.com#secret"}, {"encoded path", "base_url", "https://example.com/%2e%2e"},
		{"traversal", "base_url", "https://example.com/a/../b"}, {"bad port", "base_url", "https://example.com:99999"},
		{"missing username", "username", ""}, {"admin", "username", "admin"}, {"anonymous", "username", "anonymous"},
		{"Basic separator", "username", "user:secret"}, {"missing password", "password", ""}, {"newline password", "password", "secret\n"},
		{"empty scope", "repositories", ""}, {"wildcard", "repositories", "*"}, {"scope path", "repositories", "../repo"},
		{"empty entry", "repositories", "repo,,other"}, {"invalid CA", "ca_cert", "not PEM"},
	} {
		t.Run(test.name, func(t *testing.T) {
			creds := mcp.Credentials{"base_url": "https://example.com", "username": "reader", "password": "secret", "repositories": "maven-releases"}
			creds[test.key] = test.value
			err := New().Configure(t.Context(), creds)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestToolsAndLifecycle(t *testing.T) {
	adapter := New().(*nexus)
	require.Equal(t, "nexus", adapter.Name())
	require.False(t, adapter.Healthy(t.Context()))
	require.True(t, execute(t, adapter, "nexus_list_components", nil).IsError)
	require.Contains(t, execute(t, adapter, "unknown", nil).Data, "unknown tool")
	require.Len(t, adapter.Tools(), 3)
	require.Contains(t, adapter.Tools()[0].Description, "Start here")
	names := map[mcp.ToolName]bool{}
	for _, tool := range adapter.Tools() {
		require.False(t, names[tool.Name])
		names[tool.Name] = true
		require.True(t, strings.HasPrefix(string(tool.Name), "nexus_"))
		require.NotEmpty(t, tool.Description)
		require.Equal(t, mcp.AccessRead, tool.Access)
	}
	require.NoError(t, adapter.Close())
	require.NoError(t, adapter.Close())
	configured, creds := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/prefix/service/rest/v1/components", r.URL.Path)
		require.Equal(t, "maven-releases", r.URL.Query().Get("repository"))
		_, _ = io.WriteString(w, `{"items":[],"continuationToken":null}`)
	})
	require.True(t, configured.Healthy(t.Context()))
	require.Contains(t, configured.Tools()[0].Description, "maven-releases")
	require.Contains(t, configured.Tools()[0].Parameters["repository"], "npm-private")
	require.NoError(t, configured.Close())
	require.False(t, configured.Healthy(t.Context()))
	require.NoError(t, configured.Configure(t.Context(), creds))
	require.True(t, configured.Healthy(t.Context()))
	creds["password"] = ""
	require.Error(t, configured.Configure(t.Context(), creds))
	require.False(t, configured.Healthy(t.Context()))
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		require.Contains(t, dispatch, tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	for name := range dispatch {
		found := false
		for _, tool := range New().Tools() {
			found = found || tool.Name == name
		}
		require.True(t, found, name)
	}
}

func TestReadProjection(t *testing.T) {
	for _, test := range []struct {
		tool              mcp.ToolName
		path, query, body string
		args              map[string]any
	}{
		{"nexus_list_components", "/prefix/service/rest/v1/components", "continuationToken=opaque%2B%2F%3D&repository=maven-releases", `{"items":[` + componentFixture + `],"continuationToken":"next+/="}`, map[string]any{"repository": "maven-releases", "continuation_token": "opaque+/="}},
		{"nexus_search_components", "/prefix/service/rest/v1/search", "group=org.example&name=demo&repository=maven-releases&version=1.0", `{"items":[` + componentFixture + `],"continuationToken":null}`, map[string]any{"repository": "maven-releases", "group": "org.example", "name": "demo", "version": "1.0"}},
		{"nexus_get_component", "/prefix/service/rest/v1/components/bWF2ZW46MQ", "", componentFixture, map[string]any{"id": "bWF2ZW46MQ"}},
	} {
		t.Run(string(test.tool), func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, test.path, r.URL.Path)
				require.Equal(t, test.query, r.URL.RawQuery)
				_, _ = io.WriteString(w, test.body)
			})
			result := execute(t, adapter, test.tool, test.args)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, "org.example")
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
			if test.tool == "nexus_get_component" {
				require.Contains(t, result.Data, "asset_count")
				require.Contains(t, result.Data, "lastModified")
			} else {
				require.NotContains(t, result.Data, "assets")
				require.Contains(t, result.Data, "continuationToken")
			}
			fields, ok := adapter.CompactSpec(test.tool)
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
			require.NoError(t, err)
			require.Contains(t, string(compacted), "demo")
			require.NotContains(t, string(compacted), "SECRET_SENTINEL")
		})
	}
}

func TestArgumentBoundary(t *testing.T) {
	var requests atomic.Int32
	adapter, _ := fixture(t, func(http.ResponseWriter, *http.Request) { requests.Add(1) })
	for _, test := range []struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{
		{"missing repository", "nexus_list_components", nil},
		{"outside scope", "nexus_list_components", map[string]any{"repository": "other"}},
		{"wrong type", "nexus_list_components", map[string]any{"repository": []any{"maven-releases"}}},
		{"unknown arg", "nexus_list_components", map[string]any{"repository": "maven-releases", "base_url": "https://evil.example"}},
		{"oversize cursor", "nexus_list_components", map[string]any{"repository": "maven-releases", "continuation_token": strings.Repeat("x", 2049)}},
		{"control cursor", "nexus_list_components", map[string]any{"repository": "maven-releases", "continuation_token": "token\n"}},
		{"path ID", "nexus_get_component", map[string]any{"id": "../security/users"}},
		{"encoded ID", "nexus_get_component", map[string]any{"id": "%2fsecret"}},
		{"missing ID", "nexus_get_component", nil},
		{"oversize filter", "nexus_search_components", map[string]any{"repository": "maven-releases", "name": strings.Repeat("x", 513)}},
	} {
		t.Run(test.name, func(t *testing.T) { require.True(t, execute(t, adapter, test.tool, test.args).IsError) })
	}
	require.Zero(t, requests.Load())
}

func TestResponseBoundary(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{"foreign repository", `{"items":[{"id":"x","name":"x","format":"raw","repository":"outside"}]}`},
		{"different allowed repository", `{"items":[{"id":"x","name":"x","format":"npm","repository":"npm-private"}]}`},
		{"missing items", `{}`}, {"null items", `{"items":null}`}, {"malformed", `{"items":`},
		{"multiple JSON values", `{"items":[]} {}`}, {"oversized", strings.Repeat("x", maxResponseBytes+1)},
		{"large page", `{"items":[` + strings.TrimSuffix(strings.Repeat(componentFixture+",", 101), ",") + `]}`},
		{"long cursor", `{"items":[],"continuationToken":"` + strings.Repeat("x", 2049) + `"}`},
		{"repeated cursor", `{"items":[],"continuationToken":"previous"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, test.body) })
			result := execute(t, adapter, "nexus_list_components", map[string]any{"repository": "maven-releases", "continuation_token": "previous"})
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
		})
	}
}

func TestGetScopeAndAssetLimit(t *testing.T) {
	for _, test := range []struct {
		name, body string
		wantError  bool
	}{
		{"foreign", strings.Replace(componentFixture, "maven-releases", "outside", 1), true},
		{"mismatched ID", strings.Replace(componentFixture, "bWF2ZW46MQ", "different", 1), true},
		{"missing identity", `{}`, true},
		{"foreign asset repository", strings.Replace(componentFixture, `"path":"org/example`, `"repository":"outside","path":"org/example`, 1), true},
		{"negative asset size", strings.Replace(componentFixture, `"fileSize":42`, `"fileSize":-1`, 1), true},
		{"invalid checksum", strings.Replace(componentFixture, `"sha1":"abc"`, `"sha1":"SECRET_SENTINEL"`, 1), true},
		{"asset limit", strings.Replace(componentFixture, `"assets":[`, `"assets":[`+strings.Repeat(`{"id":"extra","path":"safe"},`, 25), 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, test.body) })
			result := execute(t, adapter, "nexus_get_component", map[string]any{"id": "bWF2ZW46MQ"})
			require.Equal(t, test.wantError, result.IsError, result.Data)
			if !test.wantError {
				var value struct {
					Assets    []any `json:"assets"`
					Count     int   `json:"asset_count"`
					Truncated bool  `json:"truncated"`
				}
				require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
				require.Len(t, value.Assets, 20)
				require.Equal(t, 26, value.Count)
				require.True(t, value.Truncated)
			}
		})
	}
}

func TestHTTPFailures(t *testing.T) {
	for _, status := range []int{204, 301, 400, 401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Location", "https://evil.example/secret")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "SECRET_SENTINEL")
			})
			result := execute(t, adapter, "nexus_list_components", map[string]any{"repository": "maven-releases"})
			require.True(t, result.IsError)
			require.NotContains(t, result.Data, "SECRET_SENTINEL")
			require.NotContains(t, result.Data, "evil.example")
		})
	}
}

func TestRetryAndCancellation(t *testing.T) {
	var calls atomic.Int32
	adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		_, _ = io.WriteString(w, `{"items":[],"continuationToken":null}`)
	})
	require.False(t, execute(t, adapter, "nexus_list_components", map[string]any{"repository": "maven-releases"}).IsError)
	require.Equal(t, int32(3), calls.Load())
	calls.Store(0)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	result, err := adapter.Execute(ctx, "nexus_list_components", map[string]any{"repository": "maven-releases"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.LessOrEqual(t, calls.Load(), int32(1))
}

func TestEmptyPage(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[],"continuationToken":null}`)
	})
	result := execute(t, adapter, "nexus_list_components", map[string]any{"repository": "maven-releases"})
	require.False(t, result.IsError)
	require.JSONEq(t, `{"items":[],"continuationToken":null}`, result.Data)
}

func TestRedirectNeverForwardsCredentials(t *testing.T) {
	var leaked atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked.Add(1) }))
	defer foreign.Close()
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/secret", http.StatusTemporaryRedirect)
	})
	result := execute(t, adapter, "nexus_list_components", map[string]any{"repository": "maven-releases"})
	require.True(t, result.IsError)
	require.Zero(t, leaked.Load())
}

func TestTLSVerificationCannotBeDisabled(t *testing.T) {
	adapter, creds := fixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"items":[]}`) })
	creds["ca_cert"] = ""
	require.NoError(t, adapter.Configure(t.Context(), creds))
	require.False(t, adapter.Healthy(t.Context()))
}

func TestOutputLimitFailsWithoutIncompletePage(t *testing.T) {
	longName := strings.Repeat("x", 512)
	body := strings.ReplaceAll(componentFixture, `"name":"demo"`, `"name":"`+longName+`"`)
	body = strings.ReplaceAll(body, `"group":"org.example"`, `"group":"`+longName+`"`)
	body = strings.ReplaceAll(body, `"version":"1.0"`, `"version":"`+longName+`"`)
	adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[`+strings.TrimSuffix(strings.Repeat(body+",", 100), ",")+`],"continuationToken":"next"}`)
	})
	result := execute(t, adapter, "nexus_list_components", map[string]any{"repository": "maven-releases"})
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "128 KiB")
	require.NotContains(t, result.Data, "continuationToken")
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
	for _, tool := range New().Tools() {
		fields, ok := New().(mcp.FieldCompactionIntegration).CompactSpec(tool.Name)
		require.True(t, ok)
		require.NotEmpty(t, fields)
	}
}
