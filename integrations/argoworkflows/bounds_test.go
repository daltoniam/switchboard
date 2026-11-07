package argoworkflows

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFailedNodeBoundAndIdentity(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantError bool
	}{
		{"bounded nodes", func(map[string]any) {}, false},
		{"wrong namespace", func(wf map[string]any) { wf["metadata"].(map[string]any)["namespace"] = "other" }, true},
		{"wrong name", func(wf map[string]any) { wf["metadata"].(map[string]any)["name"] = "other-run" }, true},
		{"unsafe node key", func(wf map[string]any) {
			wf["status"].(map[string]any)["nodes"] = map[string]any{"secret/value": map[string]any{"phase": "Failed"}}
		}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var workflow map[string]any
			require.NoError(t, json.Unmarshal([]byte(workflowFixture), &workflow))
			nodes := map[string]any{}
			for index := range 105 {
				nodes[fmt.Sprintf("failed-run-%03d", index)] = map[string]any{"phase": "Failed", "message": "secret-sentinel"}
			}
			workflow["status"].(map[string]any)["nodes"] = nodes
			test.mutate(workflow)
			adapter, _ := configured(t, func(writer http.ResponseWriter, _ *http.Request) {
				require.NoError(t, json.NewEncoder(writer).Encode(workflow))
			})
			result, err := adapter.Execute(t.Context(), "argoworkflows_get_workflow", map[string]any{"name": "failed-run"})
			require.NoError(t, err)
			require.Equal(t, test.wantError, result.IsError, result.Data)
			require.NotContains(t, result.Data, "secret-sentinel")
			if !test.wantError {
				var response struct {
					FailedNodes []failedNode `json:"failed_nodes"`
					Truncated   bool         `json:"nodes_truncated"`
				}
				require.NoError(t, json.Unmarshal([]byte(result.Data), &response))
				require.Len(t, response.FailedNodes, 100)
				require.True(t, response.Truncated)
				require.Equal(t, "failed-run-000", response.FailedNodes[0].Name)
			}
		})
	}
}

func TestCompressedResponseBound(t *testing.T) {
	adapter, _ := configured(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Encoding", "gzip")
		compressed := gzip.NewWriter(writer)
		_, err := io.WriteString(compressed, strings.Repeat(" ", maxResponseBytes+1))
		require.NoError(t, err)
		require.NoError(t, compressed.Close())
	})
	result, err := adapter.Execute(t.Context(), "argoworkflows_list_workflows", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "1 MiB")
}

func TestRedirectDoesNotForwardToken(t *testing.T) {
	var calls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(destination.Close)
	adapter, _ := configured(t, func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	})
	require.False(t, adapter.Healthy(t.Context()))
	require.Zero(t, calls.Load())
}

func TestBoundedRateLimitRetry(t *testing.T) {
	tests := []struct {
		name, retryAfter string
		wantCalls        int32
		wantError        bool
	}{
		{"one retry succeeds", "1", 2, false},
		{"long retry fails", "3600", 1, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			adapter, _ := configured(t, func(writer http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					writer.Header().Set("Retry-After", test.retryAfter)
					writer.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = io.WriteString(writer, `{"items":[]}`)
			})
			result, err := adapter.Execute(t.Context(), "argoworkflows_list_workflows", nil)
			require.NoError(t, err)
			require.Equal(t, test.wantError, result.IsError)
			require.Equal(t, test.wantCalls, calls.Load())
		})
	}
}
