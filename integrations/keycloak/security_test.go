package keycloak

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/require"
)

func TestTokenErrorsAndRedirects(t *testing.T) {
	var leaked atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked.Add(1) }))
	t.Cleanup(destination.Close)
	cases := []struct {
		status int
		body   string
	}{
		{302, `fixture-secret`}, {401, `fixture-secret`}, {429, `fixture-secret`}, {500, `fixture-secret`},
		{200, `{"access_token":"", "token_type":"Bearer", "expires_in":60}`},
		{200, `{"access_token":"token", "token_type":"Basic", "expires_in":60}`},
		{200, `{"access_token":"token", "token_type":"Bearer", "expires_in":0}`},
		{200, `{"access_token":"token\nsecret", "token_type":"Bearer", "expires_in":60}`},
		{200, `{"access_token":"token", "token_type":"Bearer", "expires_in":9999999999999}`},
		{200, `{"access_token":"token", "token_type":"Bearer", "expires_in":60} {}`},
		{200, `null`}, {200, `<html>fixture-secret</html>`},
	}
	for index, test := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, "/realms/support/protocol/openid-connect/token", r.URL.Path)
				w.Header().Set("Location", destination.URL)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			t.Cleanup(provider.Close)
			adapter := New()
			require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL, "realm": "support", "client_id": "reader", "client_secret": "fixture-secret"}))
			result := execute(t, adapter, "keycloak_list_users", nil)
			require.True(t, result.IsError)
			require.NotContains(t, result.Data, "fixture-secret")
			require.EqualValues(t, 1, calls.Load())
		})
	}
	require.Zero(t, leaked.Load())
}

func TestAPIRedirectNeverForwarded(t *testing.T) {
	var leaked atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked.Add(1) }))
	t.Cleanup(destination.Close)
	adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", destination.URL)
		w.WriteHeader(http.StatusFound)
	})
	require.True(t, execute(t, adapter, "keycloak_list_users", nil).IsError)
	require.Zero(t, leaked.Load())
}

func TestGzipDecodedResponseLimit(t *testing.T) {
	adapter, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		compressed := gzip.NewWriter(w)
		_, _ = io.WriteString(compressed, `[{"username":"`+strings.Repeat("x", maxResponseBytes)+`"}]`)
		require.NoError(t, compressed.Close())
	})
	require.True(t, execute(t, adapter, "keycloak_list_users", nil).IsError)
}

func TestDeadlineAndCancellation(t *testing.T) {
	adapter, _ := fixture(t, func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	result, err := adapter.Execute(ctx, "keycloak_list_users", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "deadline")
}

func TestTLSVerification(t *testing.T) {
	var called atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called.Add(1) }))
	t.Cleanup(provider.Close)
	adapter := New()
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL, "realm": "support", "client_id": "reader", "client_secret": "fixture-secret"}))
	require.True(t, execute(t, adapter, "keycloak_list_users", nil).IsError)
	require.Zero(t, called.Load())
}

func TestResponseBoundaries(t *testing.T) {
	cases := []struct {
		name mcp.ToolName
		args map[string]any
		body string
	}{
		{"keycloak_get_user", map[string]any{"user_id": "user-1"}, `{"id":"other"}`},
		{"keycloak_get_user", map[string]any{"user_id": "user-1"}, `null`},
		{"keycloak_list_groups", nil, `null`},
		{"keycloak_list_users", map[string]any{"max": 1}, `[{},{}]`},
		{"keycloak_list_events", map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-02"}, `[{"time":1790812800000,"type":"UPDATE_PASSWORD"}]`},
		{"keycloak_list_events", map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-02", "user": "user-1"}, `[{"time":1790812800000,"type":"LOGIN_ERROR","userId":"other"}]`},
		{"keycloak_list_events", map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-02"}, `[{"time":1,"type":"LOGIN"}]`},
	}
	for index, test := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			adapter, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/events/config") {
					_, _ = io.WriteString(w, `{"eventsEnabled":true}`)
					return
				}
				_, _ = io.WriteString(w, test.body)
			})
			require.True(t, execute(t, adapter, test.name, test.args).IsError)
		})
	}
}

func TestFieldCompactionStrictAndShape(t *testing.T) {
	loaded, err := compact.Load(compactYAML, compact.Options{Strict: true})
	require.NoError(t, err)
	require.Empty(t, loaded.Warnings)
	require.Len(t, loaded.Specs, len(dispatch))
	cases := []struct {
		name mcp.ToolName
		data string
	}{
		{"keycloak_list_users", `{"items":[{"id":"1","username":"alice","enabled":false,"email":"alice@example.test","emailVerified":false}],"first":0,"max":20,"returned_count":1,"has_more_possible":false}`},
		{"keycloak_get_user", `{"id":"1","username":"alice","enabled":false,"email":"alice@example.test","emailVerified":true,"firstName":"Alice","lastName":"Example","createdTimestamp":1}`},
		{"keycloak_list_groups", `{"items":[{"id":"1","name":"support","path":"/support","subGroupCount":0}],"first":0,"max":20,"returned_count":1,"has_more_possible":false}`},
		{"keycloak_list_events", `{"items":[{"time":1,"type":"LOGIN_ERROR","userId":"1","clientId":"app","error":"invalid_credentials"}],"first":0,"max":20,"returned_count":1,"has_more_possible":false,"events_enabled":true,"date_from":"2026-10-01","date_to":"2026-10-01"}`},
	}
	for _, test := range cases {
		t.Run(string(test.name), func(t *testing.T) {
			projected, err := mcp.CompactJSON([]byte(test.data), loaded.Specs[test.name])
			require.NoError(t, err)
			require.JSONEq(t, test.data, string(projected))
		})
	}
}
