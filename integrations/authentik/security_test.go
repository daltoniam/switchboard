package authentik

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestRejectedResponseShapes(t *testing.T) {
	for _, test := range []struct {
		name string
		tool mcp.ToolName
		body string
	}{
		{"null user", "authentik_get_user", "null"},
		{"missing active", "authentik_get_user", strings.Replace(userFixture, `"is_active":false,`, "", 1)},
		{"missing username", "authentik_get_user", strings.Replace(userFixture, `"username":"ada",`, "", 1)},
		{"fractional ID", "authentik_get_user", strings.Replace(userFixture, `"pk":42`, `"pk":42.5`, 1)},
		{"bad last login", "authentik_get_user", strings.Replace(userFixture, `"last_login":null`, `"last_login":"SECRET"`, 1)},
		{"expanded groups", "authentik_get_user", strings.Replace(userFixture, `"groups":["28e38d9e-72e6-4c8b-9c2f-10f58456caa1"]`, `"groups":[{"name":"SECRET"}]`, 1)},
		{"invalid group reference", "authentik_get_user", strings.Replace(userFixture, "28e38d9e-72e6-4c8b-9c2f-10f58456caa1", "SECRET", 1)},
		{"null groups", "authentik_get_user", strings.Replace(userFixture, `"groups":["28e38d9e-72e6-4c8b-9c2f-10f58456caa1"]`, `"groups":null`, 1)},
		{"missing pagination", "authentik_list_users", `{"results":[]}`},
		{"negative pagination", "authentik_list_users", strings.Replace(pageFixture(userFixture), `"count":3`, `"count":-1`, 1)},
		{"missing next", "authentik_list_users", strings.Replace(pageFixture(userFixture), `"next":2,`, "", 1)},
		{"wrong page", "authentik_list_users", strings.Replace(pageFixture(userFixture), `"current":1`, `"current":2`, 1)},
		{"too many records", "authentik_list_users", pageFixture(strings.Repeat(userFixture+",", 20) + userFixture)},
		{"null results", "authentik_list_users", `{"pagination":{},"results":null}`},
		{"invalid app ID", "authentik_list_user_applications", pageFixture(strings.Replace(appFixture, "28e38d9e-72e6-4c8b-9c2f-10f58456caa2", "SECRET", 1))},
		{"missing app name", "authentik_list_user_applications", pageFixture(strings.Replace(appFixture, `"name":"Payroll",`, "", 1))},
		{"missing app slug", "authentik_list_user_applications", pageFixture(strings.Replace(appFixture, `"slug":"payroll",`, "", 1))},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.body)
			})
			args := map[string]any{}
			if test.tool != "authentik_list_users" {
				args["user_id"] = 42
			}
			result, err := adapter.Execute(t.Context(), test.tool, args)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET")
		})
	}
}

func TestDecodedAndOutputCaps(t *testing.T) {
	t.Run("compressed response", func(t *testing.T) {
		adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "gzip")
			compressed := gzip.NewWriter(w)
			_, _ = io.WriteString(compressed, strings.Repeat(" ", (256<<10)+1))
			require.NoError(t, compressed.Close())
		})
		result, err := adapter.Execute(t.Context(), "authentik_list_users", nil)
		require.NoError(t, err)
		require.True(t, result.IsError)
		require.Contains(t, result.Data, "256 KiB")
	})
	t.Run("safe output remains bounded", func(t *testing.T) {
		group := `"28e38d9e-72e6-4c8b-9c2f-10f58456caa1"`
		record := strings.Replace(userFixture, `"groups":[`+group+`]`, `"groups":[`+strings.Repeat(group+",", 19)+group+`]`, 1)
		record = strings.Replace(record, `"username":"ada"`, `"username":"`+strings.Repeat("界", 128)+`"`, 1)
		body := pageFixture(strings.Repeat(record+",", 49) + record)
		adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		})
		result, err := adapter.Execute(t.Context(), "authentik_list_users", map[string]any{"page_size": 50})
		require.NoError(t, err)
		require.True(t, result.IsError)
		require.Contains(t, result.Data, "40 KiB")
	})
}

func TestRetryBudgets(t *testing.T) {
	for _, test := range []struct {
		name, retry string
		attempts    int
	}{
		{"zero", "0", 3}, {"default", "", 3}, {"too long", "10", 1},
		{"invalid", "invalid", 1}, {"past HTTP date", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), 3},
		{"future HTTP date", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", test.retry)
				w.WriteHeader(429)
			})
			result, err := adapter.Execute(t.Context(), "authentik_list_users", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
			require.Contains(t, result.Data, "429")
			require.EqualValues(t, test.attempts, calls.Load())
		})
	}
	t.Run("cancel during retry", func(t *testing.T) {
		adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(429) })
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		result, err := adapter.Execute(ctx, "authentik_list_users", nil)
		require.NoError(t, err)
		require.True(t, result.IsError)
	})
}

func TestConcurrentCloseAndReads(t *testing.T) {
	adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, pageFixture(userFixture))
	})
	var workers sync.WaitGroup
	for index := 0; index < 10; index++ {
		workers.Go(func() { _, _ = adapter.Execute(t.Context(), "authentik_list_users", nil) })
	}
	require.NoError(t, adapter.Close())
	workers.Wait()
}

func TestEmptyAndCappedValues(t *testing.T) {
	for index, body := range []string{
		strings.Replace(userFixture, `"groups":["28e38d9e-72e6-4c8b-9c2f-10f58456caa1"]`, `"groups":[]`, 1),
		strings.Replace(userFixture, `"last_login":null`, `"last_login":"2026-11-19T12:30:00Z"`, 1),
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			adapter := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			})
			result, err := adapter.Execute(t.Context(), "authentik_get_user", map[string]any{"user_id": 42})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
		})
	}
}
