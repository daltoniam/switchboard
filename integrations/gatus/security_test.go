package gatus

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

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestUpstreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name              string
		status            int
		contentType, body string
	}{
		{"unauthorized", 401, "text/plain", "SECRET"}, {"forbidden", 403, "application/json", `{"message":"SECRET"}`},
		{"missing endpoint", 404, "text/plain", "SECRET"}, {"no content", 204, "application/json", ""},
		{"OIDC HTML", 200, "text/html", "<html>SECRET</html>"}, {"malformed JSON", 200, "application/json", "{"},
		{"wrong shape", 200, "application/json", `{}`}, {"trailing data", 200, "application/json", "[] []"},
		{"invalid key", 200, "application/json", `[{"key":"../secret","results":[]}]`},
		{"long metadata", 200, "application/json", `[{"key":"ok","name":"` + strings.Repeat("x", 257) + `","results":[]}]`},
		{"missing success", 200, "application/json", `[{"key":"ok","results":[{"timestamp":"2026-11-18T12:00:00Z","duration":1}]}]`},
		{"missing duration", 200, "application/json", `[{"key":"ok","results":[{"timestamp":"2026-11-18T12:00:00Z","success":false}]}]`},
		{"negative duration", 200, "application/json", `[{"key":"ok","results":[{"timestamp":"2026-11-18T12:00:00Z","success":false,"duration":-1}]}]`},
		{"null probe", 200, "application/json", `[{"key":"ok","results":[null]}]`},
		{"inventory cap", 200, "application/json", "[" + strings.Repeat(`{"key":"ok","results":[]},`, 2000) + `{"key":"last","results":[]}]`},
		{"probe cap", 200, "application/json", `[{"key":"ok","results":[` + strings.Repeat(`{"timestamp":"2026-11-18T12:00:00Z","success":true,"duration":0},`, 20) + `{"timestamp":"2026-11-18T12:00:00Z","success":true,"duration":0}]}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			result, err := adapter.Execute(t.Context(), "gatus_list_endpoints", nil)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET")
			require.NotContains(t, result.Data, "test-secret")
			require.False(t, adapter.Healthy(t.Context()))
		})
	}
}

func TestEmptyInventoryAndUnknownState(t *testing.T) {
	for _, body := range []string{"[]", "null", `[{"key":"empty","results":[]}]`, `[{"key":"empty","results":null}]`} {
		t.Run(body, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			})
			require.True(t, adapter.Healthy(t.Context()))
			result, err := adapter.Execute(t.Context(), "gatus_list_endpoints", nil)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			if strings.Contains(body, "empty") {
				require.Contains(t, result.Data, `"latest":null`)
			}
		})
	}
}

func TestUptimeRatios(t *testing.T) {
	for _, body := range []string{"NaN", "Inf", "-0.1", "1.1", `{"uptime":0.5}`, "", "SECRET"} {
		t.Run(body, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = io.WriteString(w, body)
			})
			result, err := adapter.Execute(t.Context(), "gatus_get_uptime", map[string]any{"key": "ok"})
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET")
		})
	}
	for _, duration := range []string{"1h", "24h", "7d", "30d"} {
		t.Run(duration, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
				require.True(t, strings.HasSuffix(r.URL.Path, "/"+duration))
				w.Header().Set("Content-Type", "text/plain")
				_, _ = io.WriteString(w, "0.000000\n")
			})
			result, err := adapter.Execute(t.Context(), "gatus_get_uptime", map[string]any{"key": "ok", "duration": duration})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, `"ratio":0`)
			require.Contains(t, result.Data, "no observations")
		})
	}
}

func TestNoCredentialRedirects(t *testing.T) {
	var calls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); _, _ = io.WriteString(w, "[]") }))
	t.Cleanup(target.Close)
	for _, code := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, code) })
			result, err := adapter.Execute(t.Context(), "gatus_list_endpoints", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
			require.Zero(t, calls.Load())
		})
	}
}

func TestDecodedByteCap(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprint(compressed), func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var writer io.Writer = w
				if compressed {
					w.Header().Set("Content-Encoding", "gzip")
					compressor := gzip.NewWriter(w)
					defer func() { _ = compressor.Close() }()
					writer = compressor
				}
				_, _ = io.WriteString(writer, strings.Repeat(" ", 2<<20)+"[]")
			})
			result, err := adapter.Execute(t.Context(), "gatus_list_endpoints", nil)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.Contains(t, result.Data, "2 MiB")
		})
	}
}

func TestRetryBudget(t *testing.T) {
	for _, status := range []int{429, 500, 502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) < 3 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(status)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, "[]")
			})
			result, err := adapter.Execute(t.Context(), "gatus_list_endpoints", nil)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.EqualValues(t, 3, calls.Load())
		})
	}
	for _, retry := range []string{"0", "600", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)} {
		t.Run(retry, func(t *testing.T) {
			var calls atomic.Int32
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", retry)
				w.WriteHeader(429)
			})
			result, err := adapter.Execute(t.Context(), "gatus_list_endpoints", nil)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			if retry == "0" {
				require.EqualValues(t, 3, calls.Load())
			} else {
				require.EqualValues(t, 1, calls.Load())
			}
		})
	}
}

func TestCancellationAndVerifiedTLS(t *testing.T) {
	adapter, provider := testAdapter(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	result, err := adapter.Execute(ctx, "gatus_list_endpoints", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL, "username": "reader", "password": "test-secret"}))
	require.False(t, adapter.Healthy(t.Context()))
}

func TestHistoryShapeAndOrdering(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		size       int
		isError    bool
	}{
		{"ordered", endpointFixture, 10, false}, {"wrong key", endpointFixture, 10, true},
		{"over page size", endpointFixture, 1, true}, {"null", "null", 10, true}, {"malformed", "{", 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.body)
			})
			key := "production_api"
			if tc.name == "wrong key" {
				key = "other"
			}
			result, err := adapter.Execute(t.Context(), "gatus_get_endpoint", map[string]any{"key": key, "page_size": tc.size})
			require.NoError(t, err)
			require.Equal(t, tc.isError, result.IsError, result.Data)
			if !tc.isError {
				var raw struct {
					Results []struct {
						Timestamp string `json:"timestamp"`
					}
				}
				require.NoError(t, json.Unmarshal([]byte(result.Data), &raw))
				require.Equal(t, "2026-11-18T11:00:00Z", raw.Results[0].Timestamp)
			}
		})
	}
}
