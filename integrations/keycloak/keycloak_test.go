package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, handler http.HandlerFunc) (*keycloak, *atomic.Int32) {
	t.Helper()
	var tokens atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/realms/support/protocol/openid-connect/token" {
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
			require.Equal(t, "reader", r.Form.Get("client_id"))
			require.Equal(t, "fixture-secret", r.Form.Get("client_secret"))
			_, _ = fmt.Fprintf(w, `{"access_token":"fixture-token-%d","token_type":"Bearer","expires_in":60}`, tokens.Add(1))
			return
		}
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer fixture-token-"))
		require.True(t, strings.HasPrefix(r.URL.Path, "/auth/admin/realms/support/"))
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	adapter := New().(*keycloak)
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": server.URL + "/auth/", "realm": "support", "client_id": "reader", "client_secret": "fixture-secret"}))
	return adapter, &tokens
}

func execute(t *testing.T, adapter mcp.Integration, name mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := adapter.Execute(t.Context(), name, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestConfigure(t *testing.T) {
	valid := mcp.Credentials{"base_url": "https://identity.example.test/auth/", "realm": "support", "client_id": "reader", "client_secret": "secret"}
	cases := []struct{ key, value string }{
		{"base_url", ""}, {"realm", ""}, {"client_id", ""}, {"client_secret", ""},
		{"base_url", "http://identity.example.test"}, {"base_url", "https://user:pass@example.test"},
		{"base_url", "https://example.test?query=1"}, {"base_url", "https://example.test/#fragment"},
		{"base_url", "https://example.test/a/../b"}, {"base_url", "https://example.test/%2e%2e"},
		{"realm", "../master"}, {"realm", "."}, {"realm", "support%2fmaster"}, {"client_id", "  "},
	}
	for _, test := range cases {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			creds := mcp.Credentials{}
			for key, value := range valid {
				creds[key] = value
			}
			creds[test.key] = test.value
			require.Error(t, New().Configure(t.Context(), creds))
		})
	}
	adapter := New()
	require.Equal(t, "keycloak", adapter.Name())
	require.False(t, adapter.Healthy(t.Context()))
	require.True(t, execute(t, adapter, "keycloak_list_users", nil).IsError)
	require.NoError(t, adapter.Configure(t.Context(), valid))
	require.Equal(t, []string{"base_url", "realm", "client_id"}, adapter.(mcp.PlainTextCredentials).PlainTextKeys())
	require.Len(t, adapter.(mcp.PlaceholderHints).Placeholders(), 4)
}

func TestReads_ProjectBeforeRawAndCompact(t *testing.T) {
	cases := []struct {
		name             mcp.ToolName
		args             map[string]any
		path, body, want string
	}{
		{"keycloak_list_users", map[string]any{"search": "alice & bob", "first": 2, "max": 1}, "/users", `[{"id":"user-1","username":"alice","enabled":false,"email":"alice@example.test","credentials":[{"value":"NEVER-RETURN"}],"attributes":{"secret":"NEVER-RETURN"},"requiredActions":["NEVER-RETURN"]}]`, `"username":"alice"`},
		{"keycloak_get_user", map[string]any{"user_id": "user-1"}, "/users/user-1", `{"id":"user-1","username":"alice","enabled":true,"credentials":[{"value":"NEVER-RETURN"}],"access":{"secret":"NEVER-RETURN"},"requiredActions":["NEVER-RETURN"]}`, `"enabled":true`},
		{"keycloak_list_groups", map[string]any{"search": "support", "max": 1}, "/groups", `[{"id":"group-1","name":"support","path":"/support","subGroupCount":2,"subGroups":[{"name":"NEVER-RETURN"}],"attributes":{"secret":"NEVER-RETURN"},"realmRoles":["NEVER-RETURN"]}]`, `"name":"support"`},
		{"keycloak_list_events", map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-02", "user": "user-1", "type": "LOGIN_ERROR", "max": 1}, "/events", `[{"time":1790812800000,"type":"LOGIN_ERROR","userId":"user-1","error":"invalid_user_credentials","details":{"password":"NEVER-RETURN"},"ipAddress":"NEVER-RETURN","sessionId":"NEVER-RETURN"}]`, `"type":"LOGIN_ERROR"`},
	}
	for _, test := range cases {
		t.Run(string(test.name), func(t *testing.T) {
			var reads atomic.Int32
			adapter, tokens := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				reads.Add(1)
				if r.URL.Path == "/auth/admin/realms/support/events/config" {
					_, _ = io.WriteString(w, `{"eventsEnabled":true}`)
					return
				}
				require.Equal(t, "/auth/admin/realms/support"+test.path, r.URL.Path)
				query := r.URL.Query()
				if test.path != "/users/user-1" {
					require.Equal(t, "1", query.Get("max"))
				}
				if test.path == "/users" {
					require.Equal(t, "alice & bob", query.Get("search"))
					require.Equal(t, "2", query.Get("first"))
					require.Equal(t, "true", query.Get("briefRepresentation"))
				}
				if test.path == "/groups" {
					require.Equal(t, "false", query.Get("populateHierarchy"))
					require.Equal(t, "true", query.Get("briefRepresentation"))
				}
				if test.path == "/events" {
					require.Equal(t, "2026-10-01", query.Get("dateFrom"))
					require.Equal(t, "2026-10-02", query.Get("dateTo"))
					require.Equal(t, "user-1", query.Get("user"))
					require.Equal(t, []string{"LOGIN_ERROR"}, query["type"])
				}
				_, _ = io.WriteString(w, test.body)
			})
			for range 2 {
				result := execute(t, adapter, test.name, test.args)
				require.False(t, result.IsError, result.Data)
				require.Contains(t, result.Data, test.want)
				require.NotContains(t, result.Data, "NEVER-RETURN")
				if test.path != "/users/user-1" {
					require.Contains(t, result.Data, `"has_more_possible":true`)
				}
				fields, ok := adapter.CompactSpec(test.name)
				require.True(t, ok)
				projected, err := mcp.CompactJSON([]byte(result.Data), fields)
				require.NoError(t, err)
				require.Contains(t, string(projected), test.want)
				require.NotContains(t, string(projected), "NEVER-RETURN")
			}
			require.EqualValues(t, 1, tokens.Load())
			wantReads := int32(2)
			if test.path == "/events" {
				wantReads = 4
			}
			require.Equal(t, wantReads, reads.Load())
		})
	}
}

func TestArgumentsRejectedBeforeHTTP(t *testing.T) {
	cases := []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"keycloak_get_user", nil}, {"keycloak_get_user", map[string]any{"user_id": "../users"}},
		{"keycloak_get_user", map[string]any{"user_id": "%2e%2e"}}, {"keycloak_list_users", map[string]any{"realm": "master"}},
		{"keycloak_list_users", map[string]any{"max": 0}}, {"keycloak_list_users", map[string]any{"max": 101}},
		{"keycloak_list_users", map[string]any{"first": -1}}, {"keycloak_list_users", map[string]any{"first": 1.5}},
		{"keycloak_list_users", map[string]any{"max": map[string]any{}}}, {"keycloak_list_users", map[string]any{"search": strings.Repeat("x", 513)}},
		{"keycloak_list_events", nil}, {"keycloak_list_events", map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-08"}},
		{"keycloak_list_events", map[string]any{"date_from": "2026-10-02", "date_to": "2026-10-01"}},
		{"keycloak_list_events", map[string]any{"date_from": "bad", "date_to": "2026-10-01"}},
		{"keycloak_list_events", map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-02", "type": "UPDATE_PASSWORD"}},
		{"keycloak_unknown", nil},
	}
	for index, test := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			adapter, tokens := fixture(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected read") })
			require.True(t, execute(t, adapter, test.name, test.args).IsError)
			require.Zero(t, tokens.Load())
		})
	}
}

func TestErrorsAreSanitizedAndNotRetried(t *testing.T) {
	for _, status := range []int{301, 400, 401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var reads atomic.Int32
			adapter, tokens := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				reads.Add(1)
				w.Header().Set("Retry-After", "3600")
				w.Header().Set("Location", "https://example.test/stolen")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "NEVER-RETURN fixture-secret fixture-token-1")
			})
			result := execute(t, adapter, "keycloak_list_users", nil)
			require.True(t, result.IsError)
			require.Contains(t, result.Data, fmt.Sprint(status))
			require.NotContains(t, result.Data, "NEVER-RETURN")
			require.NotContains(t, result.Data, "fixture-secret")
			require.NotContains(t, result.Data, "fixture-token")
			if status == 401 {
				require.EqualValues(t, 2, reads.Load())
				require.EqualValues(t, 2, tokens.Load())
			} else {
				require.EqualValues(t, 1, reads.Load())
				require.EqualValues(t, 1, tokens.Load())
			}
		})
	}
}

func TestTokenRefreshAndConcurrency(t *testing.T) {
	var rejected atomic.Bool
	adapter, tokens := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if !rejected.Swap(true) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `[]`)
	})
	result := execute(t, adapter, "keycloak_list_users", nil)
	require.False(t, result.IsError, result.Data)
	require.EqualValues(t, 2, tokens.Load())
	var workers sync.WaitGroup
	for range 10 {
		workers.Go(func() {
			result, err := adapter.Execute(context.Background(), "keycloak_list_users", nil)
			if err != nil || result == nil || result.IsError {
				t.Error("concurrent read failed")
			}
		})
	}
	workers.Wait()
	require.EqualValues(t, 2, tokens.Load())
	adapter.mu.Lock()
	adapter.connection.mu.Lock()
	adapter.connection.expires = time.Now().Add(-time.Minute)
	adapter.connection.mu.Unlock()
	adapter.mu.Unlock()
	require.False(t, execute(t, adapter, "keycloak_list_users", nil).IsError)
	require.EqualValues(t, 3, tokens.Load())
}

func TestMalformedAndOverflow(t *testing.T) {
	for _, body := range []string{`not json`, `null`, `{}`, `[] []`, `[{"enabled":"yes"}]`, strings.Repeat(" ", 2*1024*1024+1), `[{"username":"` + strings.Repeat("x", 70*1024) + `"}]`} {
		t.Run(fmt.Sprint(len(body))+body[:min(len(body), 10)], func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) })
			require.True(t, execute(t, adapter, "keycloak_list_users", nil).IsError)
		})
	}
}

func TestEventsDisabledAndUnknown(t *testing.T) {
	for _, body := range []string{`{"eventsEnabled":false}`, `{}`, `null`} {
		t.Run(body, func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				require.True(t, strings.HasSuffix(r.URL.Path, "/events/config"))
				_, _ = io.WriteString(w, body)
			})
			result := execute(t, adapter, "keycloak_list_events", map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-01"})
			require.True(t, result.IsError)
			if strings.Contains(body, "false") {
				require.Contains(t, result.Data, "events not enabled")
			}
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	require.Len(t, New().Tools(), 4)
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		require.Contains(t, dispatch, tool.Name)
		require.False(t, seen[tool.Name])
		seen[tool.Name] = true
		require.True(t, strings.HasPrefix(string(tool.Name), "keycloak_"))
		require.NotEmpty(t, tool.Description)
		for _, key := range tool.Required {
			require.Contains(t, tool.Parameters, key)
		}
	}
	require.Contains(t, New().Tools()[0].Description, "Start here")
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	for name := range dispatch {
		found := false
		for _, tool := range New().Tools() {
			if name == tool.Name {
				found = true
			}
		}
		require.True(t, found, "orphan %s", name)
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	adapter := New().(*keycloak)
	require.Len(t, fieldCompactionSpecs, 4)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
		fields, ok := adapter.CompactSpec(name)
		require.True(t, ok)
		require.NotEmpty(t, fields)
	}
	_, ok := adapter.CompactSpec("keycloak_unknown")
	require.False(t, ok)
}

func TestHealthyAndReconfigure(t *testing.T) {
	adapter, tokens := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/auth/admin/realms/support/users", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("max"))
		_, _ = io.WriteString(w, `[]`)
	})
	require.True(t, adapter.Healthy(t.Context()))
	require.EqualValues(t, 1, tokens.Load())
	require.Error(t, adapter.Configure(t.Context(), nil))
	require.False(t, adapter.Healthy(t.Context()))
}

func TestPaginationDefaultsAndEmpty(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "20", r.URL.Query().Get("max"))
		require.Equal(t, "0", r.URL.Query().Get("first"))
		_, _ = io.WriteString(w, `[]`)
	})
	result := execute(t, adapter, "keycloak_list_users", nil)
	require.False(t, result.IsError, result.Data)
	var page map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &page))
	require.Equal(t, float64(0), page["returned_count"])
	require.Equal(t, false, page["has_more_possible"])
	require.Equal(t, []any{}, page["items"])
}
