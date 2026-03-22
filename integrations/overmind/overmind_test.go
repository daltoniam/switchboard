package overmind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range tools {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %q is defined but has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	toolNames := map[string]bool{}
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %q has no corresponding tool definition", name)
	}
}

func TestName(t *testing.T) {
	o := New()
	assert.Equal(t, "overmind", o.Name())
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
	}{
		{
			name: "valid credentials",
			creds: mcp.Credentials{
				"base_url":     "http://overmind:80",
				"token":        "tok_123",
				"agent_run_id": "ar_abc",
				"flow_run_id":  "fr_xyz",
			},
		},
		{
			name: "missing base_url",
			creds: mcp.Credentials{
				"token":        "tok_123",
				"agent_run_id": "ar_abc",
				"flow_run_id":  "fr_xyz",
			},
			wantErr: "base_url is required",
		},
		{
			name: "missing token",
			creds: mcp.Credentials{
				"base_url":     "http://overmind:80",
				"agent_run_id": "ar_abc",
				"flow_run_id":  "fr_xyz",
			},
			wantErr: "token is required",
		},
		{
			name: "missing agent_run_id",
			creds: mcp.Credentials{
				"base_url":    "http://overmind:80",
				"token":       "tok_123",
				"flow_run_id": "fr_xyz",
			},
			wantErr: "agent_run_id is required",
		},
		{
			name: "missing flow_run_id",
			creds: mcp.Credentials{
				"base_url":     "http://overmind:80",
				"token":        "tok_123",
				"agent_run_id": "ar_abc",
			},
			wantErr: "flow_run_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New()
			err := o.Configure(context.Background(), tt.creds)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigure_StripsTrailingSlash(t *testing.T) {
	o := New().(*overmind)
	err := o.Configure(context.Background(), mcp.Credentials{
		"base_url":     "http://overmind:80/",
		"token":        "tok_123",
		"agent_run_id": "ar_abc",
		"flow_run_id":  "fr_xyz",
	})
	require.NoError(t, err)
	assert.Equal(t, "http://overmind:80", o.baseURL)
}

func TestTools(t *testing.T) {
	o := New()
	tt := o.Tools()
	assert.Len(t, tt, 11)

	names := make([]string, len(tt))
	for i, tool := range tt {
		names[i] = tool.Name
	}
	assert.ElementsMatch(t, []string{
		"overmind_list_available_agents",
		"overmind_launch_agent",
		"overmind_get_agent_status",
		"overmind_get_agent_result",
		"overmind_complete_flow",
		"overmind_list_mcp_roles",
		"overmind_create_mcp_role",
		"overmind_list_agents",
		"overmind_create_agent",
		"overmind_list_flows",
		"overmind_create_flow",
	}, names)
}

func TestExecute_UnknownTool(t *testing.T) {
	o := New()
	_ = o.Configure(context.Background(), mcp.Credentials{
		"base_url":     "http://overmind:80",
		"token":        "tok_123",
		"agent_run_id": "ar_abc",
		"flow_run_id":  "fr_xyz",
	})

	result, err := o.Execute(context.Background(), "nonexistent_tool", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func newTestServer(t *testing.T, handler http.HandlerFunc) (*overmind, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	o := New().(*overmind)
	err := o.Configure(context.Background(), mcp.Credentials{
		"base_url":     ts.URL,
		"token":        "test_token",
		"agent_run_id": "ar_test123",
		"flow_run_id":  "fr_test456",
	})
	require.NoError(t, err)
	return o, ts
}

func TestLaunchAgent(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	var gotAuth string

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"agent_run_id":"ar_child_789","status":"pending"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_launch_agent", map[string]any{
		"agent_id": "agent_writer",
		"prompt":   "Write the report",
		"context":  "Q4 results",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ar_child_789")

	assert.Equal(t, "/api/flow_runs/fr_test456/launch_agent", gotPath)
	assert.Equal(t, "Bearer test_token", gotAuth)
	assert.Equal(t, "agent_writer", gotBody["agent_id"])
	assert.Equal(t, "Write the report", gotBody["prompt"])
	assert.Equal(t, "Q4 results", gotBody["context"])
	assert.Equal(t, "ar_test123", gotBody["parent_run_id"])
}

func TestLaunchAgent_NoContext(t *testing.T) {
	var gotBody map[string]any

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"agent_run_id":"ar_child"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_launch_agent", map[string]any{
		"agent_id": "agent_writer",
		"prompt":   "Do the thing",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.NotContains(t, gotBody, "context")
}

func TestGetAgentStatus(t *testing.T) {
	var gotPath string

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"agent_run_id":"ar_child","status":"running"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_agent_status", map[string]any{
		"agent_run_id": "ar_child",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "running")
	assert.Equal(t, "/api/agent_runs/ar_child/status", gotPath)
}

func TestGetAgentResult(t *testing.T) {
	var gotPath string

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"agent_run_id":"ar_child","result":"The report is ready."}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_agent_result", map[string]any{
		"agent_run_id": "ar_child",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "The report is ready.")
	assert.Equal(t, "/api/agent_runs/ar_child/result", gotPath)
}

func TestCompleteFlow(t *testing.T) {
	var gotPath string
	var gotBody map[string]any

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status":"completed"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_complete_flow", map[string]any{
		"summary": "All agents completed successfully",
		"status":  "success",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "completed")

	assert.Equal(t, "/api/flow_runs/fr_test456/complete", gotPath)
	assert.Equal(t, "All agents completed successfully", gotBody["summary"])
	assert.Equal(t, "success", gotBody["status"])
	assert.Equal(t, "ar_test123", gotBody["agent_run_id"])
}

func TestCompleteFlow_DefaultStatus(t *testing.T) {
	var gotBody map[string]any

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status":"completed"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_complete_flow", map[string]any{
		"summary": "Done",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", gotBody["status"])
}

func TestCompleteFlow_InvalidStatus(t *testing.T) {
	o := New().(*overmind)
	_ = o.Configure(context.Background(), mcp.Credentials{
		"base_url":     "http://overmind:80",
		"token":        "tok_123",
		"agent_run_id": "ar_abc",
		"flow_run_id":  "fr_xyz",
	})

	result, err := o.Execute(context.Background(), "overmind_complete_flow", map[string]any{
		"summary": "Done",
		"status":  "done",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "must be 'success' or 'failure'")
}

func TestCompleteFlow_FailureStatus(t *testing.T) {
	var gotBody map[string]any

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status":"completed"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_complete_flow", map[string]any{
		"summary": "Agent crashed",
		"status":  "failure",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "failure", gotBody["status"])
}

func TestEmptyRequiredArgs(t *testing.T) {
	o := New().(*overmind)
	_ = o.Configure(context.Background(), mcp.Credentials{
		"base_url":     "http://overmind:80",
		"token":        "tok",
		"agent_run_id": "ar_1",
		"flow_run_id":  "fr_1",
	})

	tests := []struct {
		name    string
		tool    string
		args    map[string]any
		wantErr string
	}{
		{"launch_agent empty agent_id", "overmind_launch_agent", map[string]any{"agent_id": "", "prompt": "go"}, "agent_id is required"},
		{"launch_agent empty prompt", "overmind_launch_agent", map[string]any{"agent_id": "a1", "prompt": ""}, "prompt is required"},
		{"get_agent_status empty id", "overmind_get_agent_status", map[string]any{"agent_run_id": ""}, "agent_run_id is required"},
		{"get_agent_result empty id", "overmind_get_agent_result", map[string]any{"agent_run_id": ""}, "agent_run_id is required"},
		{"complete_flow empty summary", "overmind_complete_flow", map[string]any{"summary": ""}, "summary is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.wantErr)
		})
	}
}

func TestHTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantRetry  bool
	}{
		{"400 not retryable", 400, false},
		{"404 not retryable", 404, false},
		{"429 retryable", 429, true},
		{"500 retryable", 500, true},
		{"503 retryable", 503, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				fmt.Fprintf(w, `{"error":"test error"}`)
			})

			result, err := o.Execute(context.Background(), "overmind_get_agent_status", map[string]any{
				"agent_run_id": "ar_test",
			})

			if tt.wantRetry {
				assert.Nil(t, result)
				require.Error(t, err)
				assert.True(t, mcp.IsRetryable(err))
			} else {
				require.NoError(t, err)
				assert.True(t, result.IsError)
			}
		})
	}
}

func TestPathEscape(t *testing.T) {
	var gotURI string

	o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	result, err := o.Execute(context.Background(), "overmind_get_agent_status", map[string]any{
		"agent_run_id": "ar/../secret",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, gotURI, "ar%2F..%2Fsecret")
}

func TestHealthy(t *testing.T) {
	t.Run("healthy when 200", func(t *testing.T) {
		o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/health", r.URL.Path)
			w.WriteHeader(200)
		})
		assert.True(t, o.Healthy(context.Background()))
	})

	t.Run("unhealthy when 500", func(t *testing.T) {
		o, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		})
		assert.False(t, o.Healthy(context.Background()))
	})
}
