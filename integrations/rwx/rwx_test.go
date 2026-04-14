package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "rwx", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "rwx_test_token", "org": "my-org"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "", "org": "my-org"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestTools(t *testing.T) {
	i := New()
	toolDefs := i.Tools()
	assert.NotEmpty(t, toolDefs)

	for _, tool := range toolDefs {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveRwxPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.True(t, len(tool.Name) > 4 && tool.Name[:4] == "rwx_",
			"tool %s missing rwx_ prefix", tool.Name)
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
	r := &rwx{accessToken: "test", org: "my-org", client: &http.Client{}, logCache: newLogCache()}
	result, err := r.Execute(context.Background(), "rwx_nonexistent", nil)
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

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	result, err := mcp.RawResult([]byte(`{"key":"value"}`))
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key":"value"`)
}

// --- Utility tests ---

func TestExtractRunID(t *testing.T) {
	assert.Equal(t, "abc123", extractRunID("abc123"))
	assert.Equal(t, "abc123", extractRunID("https://cloud.rwx.com/mint/my-org/runs/abc123"))
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"succeeded", "success"},
		{"Succeeded", "success"},
		{"failed", "failure"},
		{"Failed", "failure"},
		{"cancelled", "cancelled"},
		{"", "unknown"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, normalizeStatus(tc.input), "input: %q", tc.input)
	}
}

func TestIsVersionGTE(t *testing.T) {
	assert.True(t, isVersionGTE("3.0.0", "3.0.0"))
	assert.True(t, isVersionGTE("3.1.0", "3.0.0"))
	assert.True(t, isVersionGTE("4.0.0", "3.0.0"))
	assert.False(t, isVersionGTE("2.9.9", "3.0.0"))
	assert.False(t, isVersionGTE("3.0.0", "3.0.1"))
}

// --- Log cache tests ---

func TestLogCache(t *testing.T) {
	cache := newLogCache()

	_, ok := cache.get("test-id")
	assert.False(t, ok)

	cache.set("test-id", "log content here")
	logs, ok := cache.get("test-id")
	assert.True(t, ok)
	assert.Equal(t, "log content here", logs)
}

// --- Transform CLI references tests ---

func TestTransformCLIReferences(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"`rwx logs abc`", "rwx_get_task_logs"},
		{"`rwx results abc`", "rwx_get_run_results"},
		{"`rwx artifacts abc`", "rwx_get_artifacts"},
		{"`rwx run .rwx/ci.yml`", "rwx_launch_ci_run"},
		{"Use rwx logs to see output", "rwx_get_task_logs"},
		{"Use rwx results to check", "rwx_get_run_results"},
	}
	for _, tc := range tests {
		result := transformCLIReferences(tc.input)
		assert.Contains(t, result, tc.contains, "input: %q", tc.input)
	}
}

// --- Healthy test ---

func TestHealthy_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"runs":[]}`))
	}))
	defer ts.Close()

	r := &rwx{accessToken: "test-token", org: "my-org", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}
	assert.True(t, r.Healthy(context.Background()))
}

func TestHealthy_NoToken(t *testing.T) {
	r := &rwx{accessToken: "", org: "my-org", client: &http.Client{}, logCache: newLogCache()}
	assert.False(t, r.Healthy(context.Background()))
}

// --- API handler tests ---

func TestGetRecentRuns(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/mint/api/runs")
		_, _ = w.Write([]byte(`{"runs":[
			{"id":"run-1","branch":"main","commit_sha":"abc123","result_status":"succeeded","execution_status":"finished","title":"CI Run","definition_path":".rwx/ci.yml"},
			{"id":"run-2","branch":"develop","commit_sha":"def456","result_status":"failed","execution_status":"finished","title":"CI Run 2","definition_path":".rwx/ci.yml"}
		]}`))
	}))
	defer ts.Close()

	r := &rwx{accessToken: "test-token", org: "my-org", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}

	result, err := r.Execute(context.Background(), "rwx_get_recent_runs", map[string]any{
		"ref": "main",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "main", parsed["ref"])
	assert.Equal(t, float64(1), parsed["count"])
}

func TestGetRecentRuns_Response(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"runs":[
			{"id":"run-1","branch":"main","commit_sha":"abc","result_status":"succeeded","execution_status":"finished","title":"Test","definition_path":".rwx/ci.yml"},
			{"id":"run-2","branch":"main","commit_sha":"def","result_status":"failed","execution_status":"finished","title":"Test 2","definition_path":".rwx/ci.yml"},
			{"id":"run-3","branch":"feat","commit_sha":"ghi","result_status":"succeeded","execution_status":"finished","title":"Other","definition_path":".rwx/ci.yml"},
			{"id":"run-4","branch":"main","commit_sha":"jkl","result_status":"succeeded","execution_status":"finished","title":"Deploy","definition_path":".rwx/auto-deploy.yml"}
		]}`))
	}))
	defer ts.Close()

	r := &rwx{accessToken: "test-token", org: "my-org", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}

	t.Run("no definition_path returns all workflows for branch", func(t *testing.T) {
		result, err := getRecentRuns(context.Background(), r, map[string]any{"ref": "main", "limit": float64(10)})
		require.NoError(t, err)
		assert.False(t, result.IsError)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
		assert.Equal(t, float64(3), parsed["count"], "should return all 3 main branch runs across workflows")

		runs := parsed["runs"].([]any)
		run4 := runs[2].(map[string]any)
		assert.Equal(t, "run-4", run4["run_id"])
		assert.Equal(t, ".rwx/auto-deploy.yml", run4["definition_path"])
	})

	t.Run("definition_path filters to specific workflow", func(t *testing.T) {
		result, err := getRecentRuns(context.Background(), r, map[string]any{"ref": "main", "limit": float64(10), "definition_path": ".rwx/ci.yml"})
		require.NoError(t, err)
		assert.False(t, result.IsError)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
		assert.Equal(t, float64(2), parsed["count"], "should return only ci.yml runs")
	})

	t.Run("definition_path filters to deploy workflow", func(t *testing.T) {
		result, err := getRecentRuns(context.Background(), r, map[string]any{"ref": "main", "limit": float64(10), "definition_path": ".rwx/auto-deploy.yml"})
		require.NoError(t, err)
		assert.False(t, result.IsError)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
		assert.Equal(t, float64(1), parsed["count"], "should return only auto-deploy.yml runs")

		runs := parsed["runs"].([]any)
		run := runs[0].(map[string]any)
		assert.Equal(t, "run-4", run["run_id"])
		assert.Equal(t, ".rwx/auto-deploy.yml", run["definition_path"])
	})
}

func TestFetchRunStatus_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"completed_at":"2024-01-01T00:00:00Z","run_status":{"execution":"finished","result":"succeeded"}}`))
	}))
	defer ts.Close()

	r := &rwx{accessToken: "test-token", org: "my-org", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}
	status, isComplete, err := fetchRunStatus(context.Background(), r, "run-123")
	require.NoError(t, err)
	assert.True(t, isComplete)
	assert.Equal(t, "success", status)
}

// --- Proxy tests ---

func TestProxyToolDefinitions_TransformsCLIReferences(t *testing.T) {
	p := &proxyClient{
		tools: []proxyToolDef{
			{
				Name:        "some_tool",
				Description: "Use `rwx logs` to view output",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Run rwx results to check",
						},
					},
					"required": []interface{}{"id"},
				},
			},
		},
	}

	defs := p.toolDefinitions()
	require.Len(t, defs, 1)
	assert.Equal(t, mcp.ToolName("rwx_proxy_some_tool"), defs[0].Name)
	assert.Contains(t, defs[0].Description, "rwx_get_task_logs")
	assert.Contains(t, defs[0].Parameters["id"], "rwx_get_run_results")
	assert.Equal(t, []string{"id"}, defs[0].Required)
}

// --- JSON marshal test ---

func TestJSONResult_Complex(t *testing.T) {
	resp := map[string]any{
		"status": "success",
		"count":  3,
		"items":  []string{"a", "b", "c"},
	}
	tr, err := mcp.JSONResult(resp)
	require.NoError(t, err)
	var parsed map[string]any
	err = json.Unmarshal([]byte(tr.Data), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "success", parsed["status"])
	assert.Equal(t, float64(3), parsed["count"])
}

// --- resolveRWXBinary tests ---

func TestResolveRWXBinary_ExplicitPath(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755))

	result := resolveRWXBinary(fakeBin)
	assert.Equal(t, fakeBin, result)
}

func TestResolveRWXBinary_ExplicitPathNotExecutable(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(fakeBin, []byte("not executable"), 0o644))

	result := resolveRWXBinary(fakeBin)
	assert.NotEqual(t, fakeBin, result, "should reject non-executable configured path")
}

func TestResolveRWXBinary_ExplicitPathMissing(t *testing.T) {
	result := resolveRWXBinary("/nonexistent/path/to/rwx")
	assert.NotEqual(t, "/nonexistent/path/to/rwx", result)
}

func TestResolveRWXBinary_EmptyConfigFallsBackToLookPath(t *testing.T) {
	result := resolveRWXBinary("")
	assert.NotEmpty(t, result)
}

func TestIsExecutable(t *testing.T) {
	tmp := t.TempDir()

	t.Run("executable file", func(t *testing.T) {
		p := filepath.Join(tmp, "good")
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755))
		assert.True(t, isExecutable(p))
	})

	t.Run("non-executable file", func(t *testing.T) {
		p := filepath.Join(tmp, "noexec")
		require.NoError(t, os.WriteFile(p, []byte("data"), 0o644))
		assert.False(t, isExecutable(p))
	})

	t.Run("directory", func(t *testing.T) {
		assert.False(t, isExecutable(tmp))
	})

	t.Run("nonexistent", func(t *testing.T) {
		assert.False(t, isExecutable(filepath.Join(tmp, "nope")))
	})
}

func TestConfigure_StoresCliPath(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755))

	r := &rwx{client: &http.Client{}, logCache: newLogCache()}
	err := r.Configure(context.Background(), mcp.Credentials{
		"access_token": "test",
		"org":          "my-org",
		"cli_path":     fakeBin,
	})
	require.NoError(t, err)
	assert.Equal(t, fakeBin, r.cliPath)
}

// --- runRWXCommand stdout/stderr separation tests ---

func TestRunRWXCommand_StdoutOnly(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho '{\"ok\":true}'\n"), 0o755))

	r := &rwx{cliPath: bin}
	out, err := r.runRWXCommand(nil, 0)
	require.NoError(t, err)
	assert.Equal(t, "{\"ok\":true}\n", out)
}

func TestRunRWXCommand_StderrIgnored(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho 'Authenticating...' >&2\necho '{\"ok\":true}'\n"), 0o755))

	r := &rwx{cliPath: bin}
	out, err := r.runRWXCommand(nil, 0)
	require.NoError(t, err)
	assert.Equal(t, "{\"ok\":true}\n", out)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &parsed), "stdout should be valid JSON without stderr contamination")
	assert.Equal(t, true, parsed["ok"])
}

func TestRunRWXCommand_FailureIncludesStderr(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho 'something went wrong' >&2\nexit 1\n"), 0o755))

	r := &rwx{cliPath: bin}
	_, err := r.runRWXCommand(nil, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "something went wrong")
}

func TestRunRWXCommand_FailureWithStdoutReturnsStdout(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho '{\"partial\":true}'\nexit 1\n"), 0o755))

	r := &rwx{cliPath: bin}
	out, err := r.runRWXCommand(nil, 0)
	require.NoError(t, err, "should return stdout on failure when stdout has content")
	assert.Contains(t, out, "{\"partial\":true}")
}
