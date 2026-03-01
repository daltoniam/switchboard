package script

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockExecutor struct {
	calls   []executorCall
	results map[string]*mcp.ToolResult
	err     error
}

type executorCall struct {
	ToolName string
	Args     map[string]any
}

func (m *mockExecutor) Execute(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	m.calls = append(m.calls, executorCall{ToolName: toolName, Args: args})
	if m.err != nil {
		return nil, m.err
	}
	if r, ok := m.results[toolName]; ok {
		return r, nil
	}
	return &mcp.ToolResult{Data: `{"ok":true}`, IsError: false}, nil
}

func TestEngine_SimpleScript(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `1 + 2`)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "3", result.Data)
}

func TestEngine_ReturnObject(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `({name: "test", count: 42})`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "test", parsed["name"])
	assert.Equal(t, float64(42), parsed["count"])
}

func TestEngine_ApiCall(t *testing.T) {
	exec := &mockExecutor{
		results: map[string]*mcp.ToolResult{
			"github_list_issues": {Data: `[{"id":1,"title":"Bug"}]`},
		},
	}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `
		var issues = api.call("github_list_issues", {owner: "test", repo: "repo"});
		issues[0].title;
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `"Bug"`, result.Data)

	require.Len(t, exec.calls, 1)
	assert.Equal(t, "github_list_issues", exec.calls[0].ToolName)
	assert.Equal(t, "test", exec.calls[0].Args["owner"])
}

func TestEngine_ChainedCalls(t *testing.T) {
	exec := &mockExecutor{
		results: map[string]*mcp.ToolResult{
			"linear_search_issues": {Data: `[{"id":"ISS-1","assignee":{"email":"alice@example.com"}}]`},
			"postgres_execute_query": {Data: `[{"name":"Alice","role":"admin"}]`},
		},
	}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `
		var issues = api.call("linear_search_issues", {query: "BUG-1234"});
		var email = issues[0].assignee.email;
		var user = api.call("postgres_execute_query", {query: "SELECT * FROM users WHERE email = $1", params: [email]});
		({issue: issues[0], dbUser: user[0]});
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	require.Len(t, exec.calls, 2)
	assert.Equal(t, "linear_search_issues", exec.calls[0].ToolName)
	assert.Equal(t, "postgres_execute_query", exec.calls[1].ToolName)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	issue, _ := parsed["issue"].(map[string]any)
	assert.Equal(t, "ISS-1", issue["id"])
	dbUser, _ := parsed["dbUser"].(map[string]any)
	assert.Equal(t, "Alice", dbUser["name"])
}

func TestEngine_ApiCallError(t *testing.T) {
	exec := &mockExecutor{
		results: map[string]*mcp.ToolResult{
			"bad_tool": {Data: "not found", IsError: true},
		},
	}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `api.call("bad_tool", {})`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "returned error")
}

func TestEngine_ApiCallGoError(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("connection refused")}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `api.call("some_tool", {})`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "connection refused")
}

func TestEngine_MaxCalls(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec, WithMaxCalls(3))

	result, err := engine.Run(context.Background(), `
		for (var i = 0; i < 10; i++) {
			api.call("tool_" + i, {});
		}
	`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "exceeded maximum")
}

func TestEngine_Timeout(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec, WithTimeout(50*time.Millisecond))

	result, err := engine.Run(context.Background(), `
		while(true) {}
	`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestEngine_ConsoleLog(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `
		console.log("hello", "world");
		console.log("debug info");
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var logs []string
	require.NoError(t, json.Unmarshal([]byte(result.Data), &logs))
	assert.Equal(t, []string{"hello world", "debug info"}, logs)
}

func TestEngine_SyntaxError(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `function(`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestEngine_NullReturn(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `null`)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "null", result.Data)
}

func TestEngine_UndefinedReturn(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `undefined`)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "null", result.Data)
}

func TestEngine_ApiCallNoArgs(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `api.call("some_tool")`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	require.Len(t, exec.calls, 1)
	assert.Equal(t, map[string]any{}, exec.calls[0].Args)
}

func TestEngine_ApiCallEmptyToolName(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `api.call("")`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "requires a tool name")
}

func TestEngine_ApiCallStringResult(t *testing.T) {
	exec := &mockExecutor{
		results: map[string]*mcp.ToolResult{
			"some_tool": {Data: "plain text result"},
		},
	}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `api.call("some_tool")`)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `"plain text result"`, result.Data)
}

func TestEngine_ConsoleLogOnError(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `
		console.log("got here");
		throw new Error("boom");
	`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "boom")
	assert.Contains(t, result.Data, "got here")
}

func TestEngine_FilteringResults(t *testing.T) {
	issues := make([]map[string]any, 100)
	for i := range issues {
		issues[i] = map[string]any{
			"id":    float64(i),
			"title": fmt.Sprintf("Issue %d", i),
			"state": "open",
		}
	}
	issues[50]["state"] = "closed"
	issues[75]["state"] = "closed"

	data, _ := json.Marshal(issues)
	exec := &mockExecutor{
		results: map[string]*mcp.ToolResult{
			"github_list_issues": {Data: string(data)},
		},
	}
	engine := New(exec)

	result, err := engine.Run(context.Background(), `
		var issues = api.call("github_list_issues", {owner: "test", repo: "repo"});
		var closed = issues.filter(function(i) { return i.state === "closed"; });
		({count: closed.length, titles: closed.map(function(i) { return i.title; })});
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, float64(2), parsed["count"])
	titles, _ := parsed["titles"].([]any)
	assert.Contains(t, titles, "Issue 50")
	assert.Contains(t, titles, "Issue 75")
}

func TestEngine_WithOptions(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}

	engine := New(exec, WithTimeout(5*time.Second), WithMaxCalls(10))
	assert.Equal(t, 5*time.Second, engine.timeout)
	assert.Equal(t, 10, engine.maxCalls)
}

func TestEngine_ContextCancellation(t *testing.T) {
	exec := &mockExecutor{results: map[string]*mcp.ToolResult{}}
	engine := New(exec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := engine.Run(ctx, `
		api.call("some_tool", {});
	`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.True(t, strings.Contains(result.Data, "cancelled") || strings.Contains(result.Data, "timeout") || strings.Contains(result.Data, "context"))
}
