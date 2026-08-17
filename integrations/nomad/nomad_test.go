package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "nomad", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"address": "http://localhost:4646", "token": "secret"})
	assert.NoError(t, err)
}

func TestConfigure_SuccessWithoutToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"address": "http://localhost:4646"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAddress(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"address": "", "token": "secret"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "address is required")
}

func TestConfigure_TrimsTrailingSlash(t *testing.T) {
	n := &nomad{client: &http.Client{}}
	err := n.Configure(context.Background(), mcp.Credentials{"address": "http://localhost:4646/"})
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:4646", n.address)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveNomadPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "nomad_", "tool %s missing nomad_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	n := &nomad{address: "http://localhost:4646", client: &http.Client{}}
	result, err := n.Execute(context.Background(), "nomad_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ID":"test-job"}`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	data, err := n.get(context.Background(), "/v1/jobs")
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-job")
}

func TestDoRequest_WithToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "my-token", r.Header.Get("X-Nomad-Token"))
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, token: "my-token", client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/agent/self")
	require.NoError(t, err)
}

func TestDoRequest_NoTokenHeader_WhenEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("X-Nomad-Token"))
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/agent/self")
	require.NoError(t, err)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`Permission denied`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/jobs")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nomad API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	data, err := n.doRequest(context.Background(), "PUT", "/v1/system/gc", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["Job"])
		w.Write([]byte(`{"EvalID":"eval-123"}`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	data, err := n.post(context.Background(), "/v1/jobs", map[string]any{"Job": map[string]string{"ID": "test"}})
	require.NoError(t, err)
	assert.Contains(t, string(data), "eval-123")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`rate limited`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/jobs")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "429 should produce RetryableError")

	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
	assert.Equal(t, 30*time.Second, re.RetryAfter)
}

func TestDoRequest_RetryableOn5xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte(`service unavailable`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/jobs")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "503 should produce RetryableError")
}

func TestDoRequest_NonRetryableOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`not found`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/jobs")
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "404 should NOT be retryable")
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"namespace": "default", "empty": ""})
		assert.Contains(t, result, "namespace=default")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"", 0},
		{"-1", -1},
		{"30s", 30_000_000_000},
		{"5m", 300_000_000_000},
		{"1h", 3_600_000_000_000},
		{"1h30m", 5_400_000_000_000},
		{"100ms", 100_000_000},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseDuration(tt.input))
		})
	}
}

// --- Handler tests ---

func TestListJobs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/jobs", r.URL.Path)
		w.Write([]byte(`[{"ID":"web","Status":"running"}]`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	result, err := n.Execute(context.Background(), "nomad_list_jobs", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "web")
}

func TestGetJob(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/job/web", r.URL.Path)
		w.Write([]byte(`{"ID":"web","Name":"web","Status":"running"}`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	result, err := n.Execute(context.Background(), "nomad_get_job", map[string]any{"job_id": "web"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "web")
}

func TestListNodes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/nodes", r.URL.Path)
		w.Write([]byte(`[{"ID":"node-1","Status":"ready"}]`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	result, err := n.Execute(context.Background(), "nomad_list_nodes", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "node-1")
}

func TestGetClusterStatus(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/v1/status/leader":
			w.Write([]byte(`"192.168.0.23:4647"`))
		case "/v1/status/peers":
			w.Write([]byte(`["192.168.0.23:4647"]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	result, err := n.Execute(context.Background(), "nomad_get_cluster_status", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "leader")
	assert.Contains(t, result.Data, "peers")
	assert.Equal(t, 2, callCount)
}

func TestDrainNode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/node/node-1/drain", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["DrainSpec"])
		w.Write([]byte(`{"EvalIDs":["eval-1"]}`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	result, err := n.Execute(context.Background(), "nomad_drain_node", map[string]any{"node_id": "node-1", "enable": true, "deadline": "1h"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestNodeEligibility(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/node/node-1/eligibility", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "ineligible", body["Eligibility"])
		w.Write([]byte(`{"EvalIDs":[]}`))
	}))
	defer ts.Close()

	n := &nomad{address: ts.URL, client: ts.Client()}
	result, err := n.Execute(context.Background(), "nomad_node_eligibility", map[string]any{"node_id": "node-1", "eligible": false})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestErrResult_PropagatesRetryableError(t *testing.T) {
	retryErr := &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
	result, err := mcp.ErrResult(retryErr)
	assert.Nil(t, result, "retryable error should not produce a ToolResult")
	assert.Error(t, err, "retryable error should be propagated as Go error")
	assert.True(t, mcp.IsRetryable(err))
}

func TestHealthy(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(`{"config":{}}`))
		}))
		defer ts.Close()

		n := &nomad{address: ts.URL, client: ts.Client()}
		assert.True(t, n.Healthy(context.Background()))
	})

	t.Run("unhealthy", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(500)
			w.Write([]byte(`error`))
		}))
		defer ts.Close()

		n := &nomad{address: ts.URL, client: ts.Client()}
		assert.False(t, n.Healthy(context.Background()))
	})

	t.Run("nil client", func(t *testing.T) {
		n := &nomad{}
		assert.False(t, n.Healthy(context.Background()))
	})
}
