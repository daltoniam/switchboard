package authentik

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

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

const userFixture = `{"pk":42,"username":"ada","is_active":false,"last_login":null,"groups":["28e38d9e-72e6-4c8b-9c2f-10f58456caa1"],"email":"SECRET","name":"SECRET","attributes":{"secret":"SECRET"},"groups_obj":[{"name":"SECRET"}],"roles_obj":[{"name":"SECRET"}],"future":"SECRET"}`
const appFixture = `{"pk":"28e38d9e-72e6-4c8b-9c2f-10f58456caa2","name":"Payroll","slug":"payroll","provider_obj":{"secret":"SECRET"},"launch_url":"SECRET","future":"SECRET"}`

func pageFixture(record string) string {
	return `{"pagination":{"next":2,"previous":0,"current":1,"total_pages":3,"count":3,"future":"SECRET"},"results":[` + record + `],"autocomplete":{"secret":"SECRET"}}`
}

func testAdapter(t *testing.T, handler http.HandlerFunc) *authentik {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	adapter := New().(*authentik)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL + "/tenant", "token": "test-secret", "ca_cert": string(cert)}))
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	return adapter
}

func TestConfigure(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"missing URL", "base_url", ""}, {"HTTP", "base_url", "http://tenant.example"},
		{"userinfo", "base_url", "https://user:secret@tenant.example"}, {"query", "base_url", "https://tenant.example?x=1"},
		{"fragment", "base_url", "https://tenant.example#x"}, {"encoded path", "base_url", "https://tenant.example/%2f"},
		{"traversal", "base_url", "https://tenant.example/a/../b"}, {"port zero", "base_url", "https://tenant.example:0"},
		{"port large", "base_url", "https://tenant.example:65536"}, {"port empty", "base_url", "https://tenant.example:"},
		{"missing token", "token", ""}, {"token newline", "token", "a\nb"}, {"token space", "token", "a b"},
		{"invalid CA", "ca_cert", "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := New()
			creds := mcp.Credentials{"base_url": "https://tenant.example", "token": "test-secret"}
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
	require.Equal(t, "authentik", adapter.Name())
	require.Len(t, adapter.Tools(), 3)
	require.Contains(t, adapter.Tools()[0].Description, "Start here")
	seen := map[mcp.ToolName]bool{}
	for _, tool := range adapter.Tools() {
		require.NotEmpty(t, tool.Description)
		require.True(t, strings.HasPrefix(string(tool.Name), "authentik_"))
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

func TestTools(t *testing.T) {
	for _, tc := range []struct {
		name                    mcp.ToolName
		args                    map[string]any
		path, query, body, want string
	}{
		{"authentik_list_users", nil, "/tenant/api/v3/core/users/", "include_groups=false&include_roles=false&page=1&page_size=20", pageFixture(userFixture), `"username":"ada"`},
		{"authentik_get_user", map[string]any{"user_id": "42"}, "/tenant/api/v3/core/users/42/", "include_groups=false&include_roles=false", userFixture, `"is_active":false`},
		{"authentik_list_user_applications", map[string]any{"user_id": 42, "search": "Payroll"}, "/tenant/api/v3/core/applications/", "for_user=42&page=1&page_size=20&search=Payroll&superuser_full_list=false", pageFixture(appFixture), "candidate-count-before-policy-filter"},
	} {
		t.Run(string(tc.name), func(t *testing.T) {
			var calls atomic.Int32
			adapter := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
				require.Equal(t, tc.path, r.URL.Path)
				require.Equal(t, tc.query, r.URL.RawQuery)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.body)
			})
			result, err := adapter.Execute(t.Context(), tc.name, tc.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, tc.want)
			require.NotContains(t, result.Data, "SECRET")
			require.EqualValues(t, 1, calls.Load())
			var raw any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &raw))
			fields, ok := adapter.CompactSpec(tc.name)
			require.True(t, ok)
			compacted := mcp.CompactAny(raw, fields)
			if tc.name == "authentik_get_user" {
				record := raw.(map[string]any)
				require.Contains(t, record, "last_login")
				require.Nil(t, record["last_login"])
				delete(record, "last_login")
			}
			require.Equal(t, raw, compacted)
		})
	}
}

func TestUserFilters(t *testing.T) {
	adapter := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "false", r.URL.Query().Get("is_active"))
		require.Equal(t, "ada & grace", r.URL.Query().Get("search"))
		require.Equal(t, "ada", r.URL.Query().Get("username"))
		require.Equal(t, "2", r.URL.Query().Get("page"))
		require.Equal(t, "50", r.URL.Query().Get("page_size"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.ReplaceAll(pageFixture(userFixture), `"current":1`, `"current":2`))
	})
	result, err := adapter.Execute(t.Context(), "authentik_list_users", map[string]any{"search": "ada & grace", "username": "ada", "is_active": false, "page": 2, "page_size": 50})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
}

func TestInvalidArguments(t *testing.T) {
	var calls atomic.Int32
	adapter := testAdapter(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	for index, tc := range []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"authentik_list_users", map[string]any{"page": 0}}, {"authentik_list_users", map[string]any{"page_size": 51}},
		{"authentik_list_users", map[string]any{"page": 1.5}}, {"authentik_list_users", map[string]any{"search": true}},
		{"authentik_list_users", map[string]any{"is_active": "false"}}, {"authentik_list_users", map[string]any{"search": "a\nb"}},
		{"authentik_list_users", map[string]any{"search": `username = "ada"`}}, {"authentik_list_users", map[string]any{"search": `attributes.secret ~ "x"`}},
		{"authentik_list_users", map[string]any{"search": strings.Repeat("a", 257)}}, {"authentik_list_users", map[string]any{"token": "override"}},
		{"authentik_get_user", nil}, {"authentik_get_user", map[string]any{"user_id": 0}},
		{"authentik_get_user", map[string]any{"user_id": "../1"}}, {"authentik_get_user", map[string]any{"user_id": 1.5}},
		{"authentik_list_user_applications", nil}, {"authentik_list_user_applications", map[string]any{"user_id": -1}},
		{"authentik_list_user_applications", map[string]any{"user_id": 42, "superuser_full_list": true}},
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			result, err := adapter.Execute(t.Context(), tc.tool, tc.args)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
		})
	}
	require.Zero(t, calls.Load())
}

func TestUpstreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		status                  int
		body, contentType, want string
	}{
		{"unauthenticated", 401, "test-secret SECRET", "text/plain", "401"},
		{"forbidden", 403, "SECRET", "application/json", "403"},
		{"not found", 404, "SECRET", "application/json", "404"},
		{"invalid user permissions", 400, "SECRET", "application/json", "400"},
		{"server error", 500, "SECRET", "application/json", "500"},
		{"empty", 204, "", "application/json", "204"},
		{"malformed", 200, "{", "application/json", "response"},
		{"HTML", 200, "SECRET", "text/html", "response"},
		{"oversized", 200, strings.Repeat(" ", (256<<10)+1), "application/json", "256 KiB"},
		{"missing fields", 200, `{}`, "application/json", "response"},
		{"identity mismatch", 200, strings.Replace(userFixture, `"pk":42`, `"pk":43`, 1), "application/json", "response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			result, err := adapter.Execute(t.Context(), "authentik_get_user", map[string]any{"user_id": 42})
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.Contains(t, result.Data, tc.want)
			require.NotContains(t, result.Data, "SECRET")
			require.NotContains(t, result.Data, "test-secret")
		})
	}
}

func TestEmptyPolicyFilteredPage(t *testing.T) {
	adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, pageFixture(""))
	})
	result, err := adapter.Execute(t.Context(), "authentik_list_user_applications", map[string]any{"user_id": 42})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Contains(t, result.Data, `"next":2`)
	require.Contains(t, result.Data, `"results":[]`)
	require.Contains(t, result.Data, `"count":3`)
	require.Contains(t, result.Data, `"page":`)
	fields, ok := adapter.CompactSpec("authentik_list_user_applications")
	require.True(t, ok)
	compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
	require.NoError(t, err)
	require.Contains(t, string(compacted), `"results":[]`)
	require.Contains(t, string(compacted), `"next":2`)
}

func TestTruncation(t *testing.T) {
	body := strings.Replace(userFixture, `"username":"ada"`, `"username":"`+strings.Repeat("界", 300)+`"`, 1)
	groups := strings.Repeat(`"28e38d9e-72e6-4c8b-9c2f-10f58456caa1",`, 29) + `"28e38d9e-72e6-4c8b-9c2f-10f58456caa1"`
	body = strings.Replace(body, `"groups":["28e38d9e-72e6-4c8b-9c2f-10f58456caa1"]`, `"groups":[`+groups+`]`, 1)
	adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	})
	result, err := adapter.Execute(t.Context(), "authentik_get_user", map[string]any{"user_id": 42})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Contains(t, result.Data, `"truncated":true`)
	var user struct {
		Username string
		Groups   []string
	}
	require.NoError(t, json.Unmarshal([]byte(result.Data), &user))
	require.Len(t, []rune(user.Username), 128)
	require.Len(t, user.Groups, 20)
}

func TestRetryRedirectHealthAndClose(t *testing.T) {
	var calls atomic.Int32
	adapter := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, pageFixture(userFixture))
	})
	require.True(t, adapter.Healthy(t.Context()))
	require.EqualValues(t, 2, calls.Load())
	var redirected atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	redirect := testAdapter(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) })
	require.False(t, redirect.Healthy(t.Context()))
	require.Zero(t, redirected.Load())
	result, err := adapter.Execute(t.Context(), "unknown", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "unknown tool")
	require.NoError(t, adapter.Close())
	require.NoError(t, adapter.Close())
	require.False(t, adapter.Healthy(context.Background()))
	result, err = adapter.Execute(t.Context(), "authentik_list_users", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	require.Len(t, fieldCompactionSpecs, 3)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
}
