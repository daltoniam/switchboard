package wazuh

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPaginationIgnoresResponseControlledFields(t *testing.T) {
	payload := strings.Replace(agentPageFixture(), `"total_affected_items":2`, `"total_affected_items":1,"next_offset":-10,"partial":true`, 1)
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, payload) }))
	result, err := adapter.Execute(t.Context(), "wazuh_list_agents", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	require.Contains(t, result.Data, `"next_offset":null`)
	require.Contains(t, result.Data, `"partial":false`)
}

func TestInconsistentPagesFail(t *testing.T) {
	for _, test := range []struct{ name, from, to string }{
		{"missing failure details", `"total_failed_items":0`, `"total_failed_items":1`},
		{"partial without failures", `"error":0`, `"error":2`},
		{"failed with successful items", `"error":0`, `"error":1`},
		{"negative total", `"total_affected_items":2`, `"total_affected_items":-1`},
		{"too large total", `"total_affected_items":2`, `"total_affected_items":2147483648`},
		{"unexpected missing page", `"total_affected_items":2`, `"total_affected_items":0`},
		{"invalid id", `"id":"001"`, `"id":"../security"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := strings.Replace(agentPageFixture(), test.from, test.to, 1)
			adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, payload) }))
			result, err := adapter.Execute(t.Context(), "wazuh_list_agents", nil)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
		})
	}
}

func TestExactReadRejectsAnotherAgent(t *testing.T) {
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, agentPageFixture()) }))
	result, err := adapter.Execute(t.Context(), "wazuh_get_agent", map[string]any{"agent_id": "002"})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestReadRetriesUnauthorizedOnlyOnce(t *testing.T) {
	var logins, reads atomic.Int32
	adapter, _ := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/authenticate") {
			logins.Add(1)
			_, _ = fmt.Fprintf(writer, `{"error":0,"data":{"token":%q}}`, testToken(time.Now().Add(time.Hour)))
			return
		}
		reads.Add(1)
		writer.WriteHeader(401)
	})
	result, err := adapter.Execute(t.Context(), "wazuh_list_agents", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.EqualValues(t, 2, logins.Load())
	require.EqualValues(t, 2, reads.Load())
}

func TestRetryAfterBounds(t *testing.T) {
	for _, retryAfter := range []string{"3", "9223372036854775807", "invalid", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)} {
		t.Run(retryAfter, func(t *testing.T) {
			var reads atomic.Int32
			adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) {
				reads.Add(1)
				writer.Header().Set("Retry-After", retryAfter)
				writer.WriteHeader(429)
			}))
			result, err := adapter.Execute(t.Context(), "wazuh_list_agents", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
			require.EqualValues(t, 1, reads.Load())
		})
	}
}

func TestGzipResponseLimit(t *testing.T) {
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Encoding", "gzip")
		compressed := gzip.NewWriter(writer)
		_, _ = io.WriteString(compressed, strings.Repeat("x", maxResponseBytes+1))
		_ = compressed.Close()
	}))
	result, err := adapter.Execute(t.Context(), "wazuh_list_agents", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "2 MiB")
}

func TestDeadlineAndRetryCancellation(t *testing.T) {
	for _, rateLimited := range []bool{false, true} {
		t.Run(fmt.Sprint(rateLimited), func(t *testing.T) {
			adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, request *http.Request) {
				if rateLimited {
					writer.Header().Set("Retry-After", "1")
					writer.WriteHeader(429)
					return
				}
				<-request.Context().Done()
			}))
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()
			result, err := adapter.Execute(ctx, "wazuh_list_agents", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestSummaryRejectsInvalidCounts(t *testing.T) {
	for _, payload := range []string{
		`{"error":0,"data":{"connection":{},"configuration":{}}}`,
		`{"error":0,"data":{"connection":{"active":-1,"disconnected":0,"never_connected":0,"pending":0,"total":0},"configuration":{"synced":0,"not_synced":0,"total":0}}}`,
		`{"error":0,"data":{"connection":{"active":1,"disconnected":0,"never_connected":0,"pending":0,"total":0},"configuration":{"synced":0,"not_synced":0,"total":0}}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, payload) }))
			result, err := adapter.Execute(t.Context(), "wazuh_get_agent_status_summary", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestUnknownAndAbsentFields(t *testing.T) {
	adapter, _ := testAdapter(t, authOr(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"error":0,"data":{"affected_items":[{"id":"001","status":"future_state","name":null,"version":null}],"total_affected_items":1,"total_failed_items":0,"failed_items":[]}}`)
	}))
	result, err := adapter.Execute(t.Context(), "wazuh_get_agent", map[string]any{"agent_id": "001"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	var page map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &page))
	item := page["affected_items"].([]any)[0].(map[string]any)
	require.Equal(t, "future_state", item["status"])
	require.NotContains(t, item, "name")
	require.NotContains(t, item, "lastKeepAlive")
}
