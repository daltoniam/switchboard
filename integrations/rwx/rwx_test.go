package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		{"`rwx dispatch my-workflow`", "rwx_dispatch_run"},
		{"`rwx run .rwx/ci.yml`", "rwx_launch_ci_run"},
		{"Use rwx logs to see output", "rwx_get_task_logs"},
		{"Use rwx results to check", "rwx_get_run_results"},
		{"Use rwx dispatch to trigger", "rwx_dispatch_run"},
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

// --- parseResultsPrompt tests ---

func TestParseResultsPrompt_Empty(t *testing.T) {
	tasks, tests, problems := parseResultsPrompt("")
	assert.Nil(t, tasks)
	assert.Nil(t, tests)
	assert.Nil(t, problems)
}

func TestParseResultsPrompt_FailedTasksWithArtifacts(t *testing.T) {
	prompt := `# Failed tests:

- packages/curri-db/src/models/Orders/tests/OrderQuotes.integration.test.ts
  - OrderQuotes > OrderQuotesCrud.findById (numeric) > finds existing order quote by numeric id
  - OrderQuotes > OrderQuotesCrud.findById (string external_id) > finds existing order quote by string external id

# Failed tasks:

You can pull the logs for these tasks using ` + "`rwx logs <task-id>`" + ` and see available artifacts using ` + "`rwx artifacts list <task-id>`" + `

- jest-integration-tests.jest-integration-tests-0 (task-id: 191a345d35d42c0e9b9c7a6150cf7d32) (has artifacts)
- jest-integration-tests.jest-integration-tests-2 (task-id: 0b5fe5a97b7ede3068820c00653e8666) (has artifacts)

For more documentation on the RWX CLI, see ` + "`rwx --help`" + `
`
	tasks, tests, problems := parseResultsPrompt(prompt)

	require.Len(t, tasks, 2)
	assert.Equal(t, "jest-integration-tests.jest-integration-tests-0", tasks[0].Key)
	assert.Equal(t, "191a345d35d42c0e9b9c7a6150cf7d32", tasks[0].TaskID)
	assert.True(t, tasks[0].HasArtifacts)
	assert.Equal(t, "jest-integration-tests.jest-integration-tests-2", tasks[1].Key)
	assert.Equal(t, "0b5fe5a97b7ede3068820c00653e8666", tasks[1].TaskID)
	assert.True(t, tasks[1].HasArtifacts)

	require.Len(t, tests, 3)
	assert.Contains(t, tests[0], "OrderQuotes.integration.test.ts")
	assert.Contains(t, tests[1], "findById (numeric)")
	assert.Contains(t, tests[2], "findById (string external_id)")

	assert.Empty(t, problems)
}

func TestParseResultsPrompt_OtherProblems(t *testing.T) {
	prompt := `# Other problems:

- [build:tsgo] src/graphql/resolvers/DriverResolver.ts
  - [Error] Type assignment mismatch [tsc - 2322] (line 337:9)

# Failed tasks:

You can pull the logs for these tasks using ` + "`rwx logs <task-id>`" + `

- build (task-id: 06ca539c77d27a637433e81110bddb97)
- db-and-api-build (task-id: a72aafdaf5f759c3c1fe3ba16c5137d5)

For more documentation on the RWX CLI, see ` + "`rwx --help`" + `
`
	tasks, tests, problems := parseResultsPrompt(prompt)

	require.Len(t, tasks, 2)
	assert.Equal(t, "build", tasks[0].Key)
	assert.Equal(t, "06ca539c77d27a637433e81110bddb97", tasks[0].TaskID)
	assert.False(t, tasks[0].HasArtifacts)

	assert.Empty(t, tests)

	require.Len(t, problems, 2)
	assert.Contains(t, problems[0], "DriverResolver.ts")
	assert.Contains(t, problems[1], "tsc - 2322")
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

// --- Dispatch handler tests ---

func fakeBin(t *testing.T, script string) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "rwx")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"+script), 0o755))
	return bin
}

func TestDispatchRun_Launched(t *testing.T) {
	bin := fakeBin(t, `echo '{"run_id":"dispatch-123","run_url":"https://cloud.rwx.com/mint/org/runs/dispatch-123"}'`)
	r := &rwx{cliPath: bin, org: "org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}

	result, err := dispatchRun(context.Background(), r, map[string]any{
		"dispatch_key": "deploy-staging",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "dispatch-123", parsed["run_id"])
	assert.Equal(t, "launched", parsed["status"])
	assert.Equal(t, false, parsed["completed"])
}

func TestDispatchRun_WaitFailure(t *testing.T) {
	bin := fakeBin(t, `echo '{"run_id":"dispatch-456","result":"failed"}'`)
	r := &rwx{cliPath: bin, org: "org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}

	result, err := dispatchRun(context.Background(), r, map[string]any{
		"dispatch_key": "deploy-staging",
		"wait":         true,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "failure", parsed["status"])
	assert.Equal(t, true, parsed["completed"])
}

func TestDispatchRun_WaitSuccess(t *testing.T) {
	bin := fakeBin(t, `echo '{"run_id":"dispatch-ok","result":"succeeded"}'`)
	r := &rwx{cliPath: bin, org: "org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}

	result, err := dispatchRun(context.Background(), r, map[string]any{
		"dispatch_key": "deploy-staging",
		"wait":         true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "success", parsed["status"])
	assert.Equal(t, "Run completed successfully", parsed["next_step"])
}

func TestDispatchRun_URLFallback(t *testing.T) {
	bin := fakeBin(t, `echo '{"run_id":"dispatch-789"}'`)
	r := &rwx{cliPath: bin, org: "my-org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}

	result, err := dispatchRun(context.Background(), r, map[string]any{
		"dispatch_key": "test",
	})
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "https://cloud.rwx.com/mint/my-org/runs/dispatch-789", parsed["url"])
}

// --- Docs handler tests ---

func TestDocsSearch(t *testing.T) {
	jsonOut := `{"Query":"caching","TotalHits":2,"Results":[{"url":"https://www.rwx.com/docs/caching","path":"/docs/caching","title":"Caching","body":"Short body here."}]}`
	bin := fakeBin(t, fmt.Sprintf(`echo '%s'`, jsonOut))
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := docsSearch(context.Background(), r, map[string]any{"query": "caching"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "caching", parsed["query"])
	assert.Equal(t, float64(2), parsed["total_hits"])
	assert.Equal(t, float64(1), parsed["count"])

	results := parsed["results"].([]any)
	first := results[0].(map[string]any)
	assert.Equal(t, "Caching", first["title"])
	assert.Equal(t, "Short body here.", first["snippet"])
}

func TestDocsSearch_SnippetTruncation(t *testing.T) {
	longBody := ""
	for i := 0; i < 600; i++ {
		longBody += "x"
	}
	jsonOut := fmt.Sprintf(`{"Query":"test","TotalHits":1,"Results":[{"url":"u","path":"p","title":"T","body":"%s"}]}`, longBody)
	bin := fakeBin(t, fmt.Sprintf(`echo '%s'`, jsonOut))
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := docsSearch(context.Background(), r, map[string]any{"query": "test"})
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	results := parsed["results"].([]any)
	first := results[0].(map[string]any)
	snippet := first["snippet"].(string)
	assert.True(t, len(snippet) <= 504, "snippet should be truncated to ~500+...")
	assert.True(t, strings.HasSuffix(snippet, "..."))
}

func TestDocsPull(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "rwx")
	script := "#!/bin/sh\nprintf '{\"URL\":\"https://www.rwx.com/docs/caching\",\"Body\":\"Caching content here.\"}'\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := docsPull(context.Background(), r, map[string]any{"url_or_path": "/docs/caching"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "https://www.rwx.com/docs/caching", parsed["url"])
	assert.Contains(t, parsed["content"], "Caching content here.")
}

// --- LaunchCIRun handler tests ---

func TestLaunchCIRun_WaitAddsFailFast(t *testing.T) {
	bin := fakeBin(t, `echo '{"run_id":"run-1","result":"succeeded"}'`)
	r := &rwx{cliPath: bin, org: "org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}

	result, err := launchCIRun(context.Background(), r, map[string]any{
		"wait": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "success", parsed["status"])
	assert.Equal(t, true, parsed["completed"])
}

func TestLaunchCIRun_TitleAndInit(t *testing.T) {
	bin := fakeBin(t, `echo '{"run_id":"run-2"}'`)
	r := &rwx{cliPath: bin, org: "org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}

	result, err := launchCIRun(context.Background(), r, map[string]any{
		"title": "Deploy v2",
		"init":  map[string]any{"env": "staging"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "run-2", parsed["run_id"])
	assert.Equal(t, "launched", parsed["status"])
}

// --- GetTaskLogs handler tests ---

func TestGetTaskLogs_MissingArgs(t *testing.T) {
	r := &rwx{cliPath: "rwx", logCache: newLogCache()}

	result, err := getTaskLogs(context.Background(), r, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "either task_id or run_id+task_key is required")
}

func TestGetTaskLogs_RunIDAndTaskKey(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "rwx")
	script := `#!/bin/sh
for arg in "$@"; do
  case "$prev" in
    --output-dir) echo "log output for task" > "$arg/task.log" ;;
  esac
  prev="$arg"
done
echo '{}'
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	r := &rwx{cliPath: bin, logCache: newLogCache(), client: &http.Client{}, baseURL: "http://localhost:0", org: "test"}

	result, err := getTaskLogs(context.Background(), r, map[string]any{
		"run_id":   "run-abc123",
		"task_key": "ci.checks.lint",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "log output for task")
}

// --- GetRunResults handler tests ---

func TestGetRunResults_MissingArgs(t *testing.T) {
	r := &rwx{cliPath: "rwx", org: "org", baseURL: "https://cloud.rwx.com", logCache: newLogCache()}

	result, err := getRunResults(context.Background(), r, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "either run_id or branch/commit is required")
}

func TestGetRunResults_BranchLookup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"run-abc","completed_runtime_seconds":120,"title":"CI","branch":"main","commit_sha":"abc123","definition_path":".rwx/ci.yml"}`))
	}))
	defer ts.Close()

	bin := fakeBin(t, `echo '{"RunID":"run-abc","ResultStatus":"succeeded","Completed":true}'`)
	r := &rwx{cliPath: bin, org: "org", baseURL: ts.URL, client: ts.Client(), logCache: newLogCache()}

	result, err := getRunResults(context.Background(), r, map[string]any{
		"branch": "main",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "run-abc", parsed["run_id"])
	assert.Equal(t, "success", parsed["status"])
	assert.Equal(t, "main", parsed["branch"])
}

// --- Vault handler tests ---

func TestVaultsVarShow(t *testing.T) {
	bin := fakeBin(t, `echo '{"name":"MY_VAR","value":"hello","vault":"default"}'`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsVarShow(context.Background(), r, map[string]any{"name": "MY_VAR"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "MY_VAR")
}

func TestVaultsVarShow_EmptyOutput(t *testing.T) {
	bin := fakeBin(t, `true`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsVarShow(context.Background(), r, map[string]any{"name": "MISSING"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "no output from CLI")
}

func TestVaultsVarSet_WithOutput(t *testing.T) {
	bin := fakeBin(t, `echo '{"ok":true}'`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsVarSet(context.Background(), r, map[string]any{"name": "K", "value": "V"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ok")
}

func TestVaultsVarSet_EmptyOutput(t *testing.T) {
	bin := fakeBin(t, `true`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsVarSet(context.Background(), r, map[string]any{"name": "K", "value": "V"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "set", parsed["status"])
	assert.Equal(t, "K", parsed["name"])
	assert.Equal(t, "default", parsed["vault"])
}

func TestVaultsVarDelete_EmptyOutput(t *testing.T) {
	bin := fakeBin(t, `true`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsVarDelete(context.Background(), r, map[string]any{"name": "K"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "deleted", parsed["status"])
	assert.Equal(t, "K", parsed["name"])
}

func TestVaultsSecretSet_EmptyOutput(t *testing.T) {
	bin := fakeBin(t, `true`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsSecretSet(context.Background(), r, map[string]any{"name": "S", "value": "secret"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "set", parsed["status"])
	assert.Equal(t, "S", parsed["name"])
}

func TestVaultsSecretDelete_EmptyOutput(t *testing.T) {
	bin := fakeBin(t, `true`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsSecretDelete(context.Background(), r, map[string]any{"name": "S"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "deleted", parsed["status"])
	assert.Equal(t, "S", parsed["name"])
}

func TestVaultsVarSet_CustomVault(t *testing.T) {
	bin := fakeBin(t, `true`)
	r := &rwx{cliPath: bin, logCache: newLogCache()}

	result, err := vaultsVarSet(context.Background(), r, map[string]any{"name": "K", "value": "V", "vault": "staging"})
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "staging", parsed["vault"])
}
