package ntfy

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestUpstreamFailuresAreExplicitAndSanitized(t *testing.T) {
	for _, tc := range []struct {
		name              string
		status            int
		contentType, body string
	}{
		{"unauthorized", 401, "text/plain", "SECRET test-secret"}, {"forbidden", 403, "application/json", `{"error":"SECRET"}`},
		{"inaccessible", 404, "text/plain", "SECRET"}, {"no content", 204, "application/x-ndjson", ""},
		{"HTML", 200, "text/html", "SECRET"}, {"bad JSON", 200, "application/x-ndjson", "SECRET"},
		{"array", 200, "application/x-ndjson", "[]"}, {"null", 200, "application/x-ndjson", "null"},
		{"trailing JSON", 200, "application/x-ndjson", fixture + ` {"event":"message"}`},
		{"partial JSON", 200, "application/x-ndjson", fixture + "\n{"},
		{"invalid ID", 200, "application/x-ndjson", `{"event":"message","topic":"ops","id":"SECRET","time":1}`},
		{"missing time", 200, "application/x-ndjson", `{"event":"message","topic":"ops","id":"AbCd1234EfGh"}`},
		{"wrong priority", 200, "application/x-ndjson", `{"event":"message","topic":"ops","id":"AbCd1234EfGh","time":1,"priority":9}`},
		{"missing event", 200, "application/x-ndjson", `{}`},
		{"long line", 200, "application/x-ndjson", strings.Repeat(" ", 8193)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			result, value := resultJSON(t, adapter, "ntfy_list_messages", nil)
			require.True(t, result.IsError, result.Data)
			require.Equal(t, true, value["incomplete"])
			require.NotEmpty(t, value["error"])
			require.NotContains(t, result.Data, "SECRET")
			require.NotContains(t, result.Data, "test-secret")
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestByteCapAndNoPartialRetry(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprint(compressed), func(t *testing.T) {
			var calls atomic.Int32
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/x-ndjson")
				var writer io.Writer = w
				if compressed {
					w.Header().Set("Content-Encoding", "gzip")
					compressor := gzip.NewWriter(w)
					defer func() { _ = compressor.Close() }()
					writer = compressor
				}
				_, _ = io.WriteString(writer, strings.Repeat(fixture+"\n", 300))
			})
			result, value := resultJSON(t, adapter, "ntfy_list_messages", nil)
			require.True(t, result.IsError, result.Data)
			require.Equal(t, true, value["adapter_truncated"])
			require.Equal(t, true, value["incomplete"])
			require.EqualValues(t, 1, calls.Load())
		})
	}
	var calls atomic.Int32
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Length", "9999")
		_, _ = io.WriteString(w, fixture+"\n")
	})
	result, value := resultJSON(t, adapter, "ntfy_list_messages", nil)
	require.True(t, result.IsError, result.Data)
	require.Equal(t, true, value["incomplete"])
	require.EqualValues(t, 1, calls.Load())
}

func TestSanitizationAndFinalByteBudget(t *testing.T) {
	body := ""
	for index := range 20 {
		value := map[string]any{"event": "message", "topic": "ops", "id": fmt.Sprintf("%012d", index), "time": index + 1, "title": strings.Repeat("界", 140), "message": strings.Repeat("<", 1100), "tags": []string{strings.Repeat("界", 70), "a", "b", "c", "d", "e", "f", "g", "h"}, "attachment": map[string]any{"url": "SECRET"}}
		data, err := json.Marshal(value)
		require.NoError(t, err)
		body += string(data) + "\n"
	}
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, body)
	})
	result, err := adapter.Execute(t.Context(), "ntfy_list_messages", map[string]any{"limit": 20})
	require.NoError(t, err)
	require.Less(t, len(result.Data), maxOutputBytes)
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
	require.NotContains(t, result.Data, "SECRET")
	require.Equal(t, true, value["adapter_truncated"])
	require.Equal(t, true, value["incomplete"])
	require.NotEmpty(t, value["messages"])
	require.True(t, result.IsError)
	require.Equal(t, "error", value["status"])
	require.Contains(t, value["error"], "64 KiB")
	require.Equal(t, true, value["truncations"].(map[string]any)["response_bytes"])
	require.Equal(t, true, value["truncations"].(map[string]any)["output_bytes"])
	for _, item := range value["messages"].([]any) {
		message := item.(map[string]any)
		require.LessOrEqual(t, utf8.RuneCountInString(message["title"].(string)), 128)
		require.LessOrEqual(t, utf8.RuneCountInString(message["message"].(string)), 1024)
		require.LessOrEqual(t, len(message["tags"].([]any)), 8)
		for _, tag := range message["tags"].([]any) {
			require.LessOrEqual(t, utf8.RuneCountInString(tag.(string)), 64)
		}
	}
	require.NotEmpty(t, value["truncations"])
}

func TestOrderingLocalLimitAndDefaults(t *testing.T) {
	body := ""
	for index := range 15 {
		body += fmt.Sprintf(`{"event":"message","topic":"ops","id":"%012d","time":%d,"message":"hello"}`+"\n", index, index+1)
	}
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, body)
	})
	result, value := resultJSON(t, adapter, "ntfy_list_messages", nil)
	require.False(t, result.IsError, result.Data)
	messages := value["messages"].([]any)
	require.Len(t, messages, 10)
	require.Equal(t, "000000000014", messages[0].(map[string]any)["id"])
	require.EqualValues(t, 3, messages[0].(map[string]any)["priority"])
	require.Equal(t, true, value["adapter_truncated"])
	require.Equal(t, true, value["incomplete"])
}

func TestNoRedirects(t *testing.T) {
	var calls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(target.Close)
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL+"/SECRET", status) })
			result, _ := resultJSON(t, adapter, "ntfy_list_messages", nil)
			require.True(t, result.IsError)
			require.NotContains(t, result.Data, "SECRET")
			require.Zero(t, calls.Load())
		})
	}
}

func TestRateLimitBudget(t *testing.T) {
	for _, retry := range []string{"0", "600", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), "99999999999999999999999"} {
		t.Run(retry, func(t *testing.T) {
			var calls atomic.Int32
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", retry)
				w.WriteHeader(429)
				_, _ = io.WriteString(w, "SECRET")
			})
			result, _ := resultJSON(t, adapter, "ntfy_list_messages", nil)
			require.True(t, result.IsError)
			require.NotContains(t, result.Data, "SECRET")
			if retry == "0" {
				require.EqualValues(t, 3, calls.Load())
			} else {
				require.EqualValues(t, 1, calls.Load())
			}
		})
	}
	var calls atomic.Int32
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
	})
	result, _ := resultJSON(t, adapter, "ntfy_list_messages", nil)
	require.False(t, result.IsError, result.Data)
	require.EqualValues(t, 3, calls.Load())
}

func TestDeadline(t *testing.T) {
	adapter, provider := testAdapter(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	start := time.Now()
	result, err := adapter.Execute(ctx, "ntfy_list_messages", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Less(t, time.Since(start), time.Second)
	require.NotContains(t, result.Data, provider.URL)
}

func TestTLSCertificateVerification(t *testing.T) {
	var calls atomic.Int32
	adapter, provider := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/x-ndjson")
	})
	require.True(t, adapter.Healthy(t.Context()))
	require.EqualValues(t, 1, calls.Load())
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL + "/ntfy", "topic": "ops", "api_token": "test-secret"}))
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.False(t, adapter.Healthy(ctx))
	require.NoError(t, ctx.Err())
	require.EqualValues(t, 1, calls.Load())
}
