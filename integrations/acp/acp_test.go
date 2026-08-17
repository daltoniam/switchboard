package acp

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

func TestName(t *testing.T) {
	i := New()
	assert.Equal(t, "acp", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	ts := newMockAgentsServer(t, []AgentManifest{{Name: "echo"}})
	defer ts.Close()

	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"config": fmt.Sprintf(`{"servers":[{"name":"test","url":"%s"}]}`, ts.URL),
	})
	assert.NoError(t, err)
}

func TestConfigure_EmptyConfig(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.NoError(t, err)
}

func TestConfigure_InvalidJSON(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"config": "not json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config JSON")
}

func TestConfigure_EmptyServersArray(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"config": `{"servers":[]}`})
	assert.NoError(t, err)
}

func TestConfigure_ServerMissingURL(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"config": `{"servers":[{"name":"bad"}]}`,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no url")
}

func TestConfigure_MultipleServers(t *testing.T) {
	ts1 := newMockAgentsServer(t, []AgentManifest{{Name: "a1"}})
	defer ts1.Close()
	ts2 := newMockAgentsServer(t, []AgentManifest{{Name: "a2"}})
	defer ts2.Close()

	i := New()
	cfg := fmt.Sprintf(`{"servers":[{"name":"s1","url":"%s"},{"name":"s2","url":"%s"}]}`, ts1.URL, ts2.URL)
	err := i.Configure(context.Background(), mcp.Credentials{"config": cfg})
	assert.NoError(t, err)

	a := i.(*acpIntegration)
	assert.Len(t, a.clients, 2)
}

func TestTools(t *testing.T) {
	i := New()
	tl := i.Tools()
	assert.Len(t, tl, 3)
	for _, tool := range tl {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
	}
}

func TestTools_AllHaveACPPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "acp_", "tool %s missing acp_ prefix", tool.Name)
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

func TestTools_AllHaveServerURLParam(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := tool.Parameters["server_url"]
		assert.True(t, ok, "tool %s missing server_url parameter", tool.Name)
	}
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

func TestExecute_UnknownTool(t *testing.T) {
	a := &acpIntegration{clients: make(map[string]*client)}
	result, err := a.Execute(context.Background(), "acp_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestHealthy(t *testing.T) {
	ts := newMockAgentsServer(t, []AgentManifest{{Name: "echo"}})
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	assert.True(t, a.Healthy(context.Background()))
}

func TestHealthy_NoClients(t *testing.T) {
	a := &acpIntegration{clients: make(map[string]*client)}
	assert.True(t, a.Healthy(context.Background()))
}

func TestHealthy_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	assert.False(t, a.Healthy(context.Background()))
}

func TestPlainTextKeys(t *testing.T) {
	a := New().(*acpIntegration)
	assert.Equal(t, []string{"config"}, a.PlainTextKeys())
}

func TestOptionalKeys(t *testing.T) {
	a := New().(*acpIntegration)
	assert.Equal(t, []string{"config"}, a.OptionalKeys())
}

func TestPlaceholders(t *testing.T) {
	a := New().(*acpIntegration)
	ph := a.Placeholders()
	assert.Contains(t, ph["config"], "servers")
}

// --- Tool handler tests: pre-configured servers ---

func TestListAgents(t *testing.T) {
	agents := []AgentManifest{
		{Name: "echo", Description: "Echoes input"},
		{Name: "summarizer", Description: "Summarizes text"},
	}
	ts := newMockAgentsServer(t, agents)
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	result, err := a.Execute(context.Background(), "acp_list_agents", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "echo")
	assert.Contains(t, result.Data, "summarizer")
}

func TestListAgents_SpecificServer(t *testing.T) {
	ts := newMockAgentsServer(t, []AgentManifest{{Name: "echo"}})
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	result, err := a.Execute(context.Background(), "acp_list_agents", map[string]any{"server": "test"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "echo")
}

func TestListAgents_UnknownServer(t *testing.T) {
	a := newTestACP(t, "http://unused")
	result, err := a.Execute(context.Background(), "acp_list_agents", map[string]any{"server": "nonexistent"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
}

func TestRunAgent_Sync(t *testing.T) {
	ts := newEchoServer(t)
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "echo",
		"input":      "hello world",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "hello world")
}

func TestRunAgent_Stream(t *testing.T) {
	ts := newStreamServer(t)
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "echo",
		"input":      "hi",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello World")
}

func TestRunAgent_MissingParams(t *testing.T) {
	a := newTestACP(t, "http://unused")
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestRunAgent_NoServersConfigured(t *testing.T) {
	a := &acpIntegration{clients: make(map[string]*client)}
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "echo",
		"input":      "hi",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "no ACP server specified")
}

func TestRunAgent_Awaiting(t *testing.T) {
	ts := newAwaitServer(t)
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "approval",
		"input":      "please approve",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "awaiting")
	assert.Contains(t, result.Data, "run-await-1")
}

func TestResumeRun(t *testing.T) {
	ts := newResumeServer(t)
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	result, err := a.Execute(context.Background(), "acp_resume_run", map[string]any{
		"run_id": "run-await-1",
		"input":  "yes, proceed",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Approved")
}

func TestResumeRun_MissingParams(t *testing.T) {
	a := newTestACP(t, "http://unused")
	result, err := a.Execute(context.Background(), "acp_resume_run", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListAgents_MultipleServers(t *testing.T) {
	ts1 := newMockAgentsServer(t, []AgentManifest{{Name: "agent1", Description: "First"}})
	defer ts1.Close()
	ts2 := newMockAgentsServer(t, []AgentManifest{{Name: "agent2", Description: "Second"}})
	defer ts2.Close()

	a := &acpIntegration{
		clients: map[string]*client{
			"server1": newClient("server1", ts1.URL, nil),
			"server2": newClient("server2", ts2.URL, nil),
		},
	}

	result, err := a.Execute(context.Background(), "acp_list_agents", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "agent1")
	assert.Contains(t, result.Data, "agent2")
}

func TestRunAgent_WithSessionID(t *testing.T) {
	var capturedSessionID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RunCreateRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedSessionID = req.SessionID

		if req.Mode == RunModeStream {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ACPError{Message: "no stream"})
			return
		}

		json.NewEncoder(w).Encode(Run{
			AgentName: req.AgentName,
			RunID:     "run-1",
			Status:    RunStatusCompleted,
			Output:    []Message{NewAgentMessage("done")},
		})
	}))
	defer ts.Close()

	a := newTestACP(t, ts.URL)
	_, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "echo",
		"input":      "hi",
		"session_id": "ses-123",
	})
	require.NoError(t, err)
	assert.Equal(t, "ses-123", capturedSessionID)
}

// --- Tool handler tests: inline server_url ---

func TestListAgents_InlineServerURL(t *testing.T) {
	ts := newMockAgentsServer(t, []AgentManifest{{Name: "remote-agent", Description: "Remote"}})
	defer ts.Close()

	a := &acpIntegration{clients: make(map[string]*client)}
	result, err := a.Execute(context.Background(), "acp_list_agents", map[string]any{
		"server_url": ts.URL,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "remote-agent")
}

func TestRunAgent_InlineServerURL(t *testing.T) {
	ts := newEchoServer(t)
	defer ts.Close()

	a := &acpIntegration{clients: make(map[string]*client), timeout: 10 * time.Second}
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "echo",
		"input":      "hello via url",
		"server_url": ts.URL,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "hello via url")
}

func TestRunAgent_InlineServerURLStream(t *testing.T) {
	ts := newStreamServer(t)
	defer ts.Close()

	a := &acpIntegration{clients: make(map[string]*client), timeout: 10 * time.Second}
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "echo",
		"input":      "hi",
		"server_url": ts.URL,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello World")
}

func TestResumeRun_InlineServerURL(t *testing.T) {
	ts := newResumeServer(t)
	defer ts.Close()

	a := &acpIntegration{clients: make(map[string]*client)}
	result, err := a.Execute(context.Background(), "acp_resume_run", map[string]any{
		"run_id":     "run-await-1",
		"input":      "yes",
		"server_url": ts.URL,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Approved")
}

func TestRunAgent_InlineServerHeaders(t *testing.T) {
	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if r.URL.Path == "/runs" {
			var req RunCreateRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Mode == RunModeStream {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(ACPError{Message: "no stream"})
				return
			}
			json.NewEncoder(w).Encode(Run{
				AgentName: req.AgentName,
				RunID:     "run-1",
				Status:    RunStatusCompleted,
				Output:    []Message{NewAgentMessage("done")},
			})
		}
	}))
	defer ts.Close()

	a := &acpIntegration{clients: make(map[string]*client), timeout: 10 * time.Second}
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name":     "echo",
		"input":          "hi",
		"server_url":     ts.URL,
		"server_headers": `{"Authorization":"Bearer sk-test-123"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "Bearer sk-test-123", capturedAuth)
}

func TestRunAgent_InlineServerURLOverridesNamed(t *testing.T) {
	tsNamed := newMockAgentsServer(t, []AgentManifest{{Name: "named"}})
	defer tsNamed.Close()
	tsInline := newEchoServer(t)
	defer tsInline.Close()

	a := &acpIntegration{
		clients: map[string]*client{
			"named": newClient("named", tsNamed.URL, nil),
		},
		timeout: 10 * time.Second,
	}
	result, err := a.Execute(context.Background(), "acp_run_agent", map[string]any{
		"agent_name": "echo",
		"input":      "hi from inline",
		"server_url": tsInline.URL,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "hi from inline")
}

func TestParseHeaders(t *testing.T) {
	h := parseHeaders(map[string]any{"server_headers": `{"Authorization":"Bearer x","X-Custom":"val"}`})
	assert.Equal(t, "Bearer x", h["Authorization"])
	assert.Equal(t, "val", h["X-Custom"])
}

func TestParseHeaders_Empty(t *testing.T) {
	h := parseHeaders(map[string]any{})
	assert.Nil(t, h)
}

func TestParseHeaders_InvalidJSON(t *testing.T) {
	h := parseHeaders(map[string]any{"server_headers": "not json"})
	assert.Nil(t, h)
}

// --- Client tests ---

func TestClient_ListAgents(t *testing.T) {
	agents := []AgentManifest{
		{Name: "echo", Description: "Echoes input"},
	}
	ts := newMockAgentsServer(t, agents)
	defer ts.Close()

	c := newClient("test", ts.URL, nil)
	result, err := c.listAgents(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "echo", result[0].Name)
}

func TestClient_ListAgents_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(ACPError{Message: "internal error"})
	}))
	defer ts.Close()

	c := newClient("test", ts.URL, nil)
	_, err := c.listAgents(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal error")
}

func TestClient_CustomHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(AgentsListResponse{Agents: []AgentManifest{}})
	}))
	defer ts.Close()

	c := newClient("test", ts.URL, map[string]string{"Authorization": "Bearer test-token"})
	_, err := c.listAgents(context.Background())
	assert.NoError(t, err)
}

func TestClient_CreateRunSync(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RunCreateRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, RunModeSync, req.Mode)
		json.NewEncoder(w).Encode(Run{
			AgentName: req.AgentName,
			RunID:     "run-1",
			Status:    RunStatusCompleted,
			Output:    []Message{NewAgentMessage("Echo: " + TextContent(req.Input))},
		})
	}))
	defer ts.Close()

	c := newClient("test", ts.URL, nil)
	run, err := c.createRunSync(context.Background(), "echo", []Message{NewUserMessage("hello")}, "")
	require.NoError(t, err)
	assert.Equal(t, RunStatusCompleted, run.Status)
	assert.Equal(t, "Echo: hello", TextContent(run.Output))
}

func TestClient_CreateRunStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ContentTypeNDJSON)
		w.WriteHeader(http.StatusOK)
		events := []string{
			`{"type":"run.in-progress","run":{"agent_name":"echo","run_id":"r1","status":"in-progress","output":[],"created_at":"2025-01-01T00:00:00Z"}}`,
			`{"type":"message.part","part":{"content_type":"text/plain","content":"Hello"}}`,
			`{"type":"message.part","part":{"content_type":"text/plain","content":" World"}}`,
			`{"type":"run.completed","run":{"agent_name":"echo","run_id":"r1","status":"completed","output":[],"created_at":"2025-01-01T00:00:00Z"}}`,
		}
		for _, e := range events {
			fmt.Fprintf(w, "%s\n", e)
		}
	}))
	defer ts.Close()

	c := newClient("test", ts.URL, nil)
	ch, err := c.createRunStream(context.Background(), "echo", []Message{NewUserMessage("hi")}, "")
	require.NoError(t, err)

	var collected []Event
	for e := range ch {
		collected = append(collected, e)
	}
	require.Len(t, collected, 4)
	assert.Equal(t, EventMessagePart, collected[1].Type)
	assert.Equal(t, "Hello", collected[1].Part.Content)
}

func TestClient_ResumeRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RunResumeRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "run-await-1", req.RunID)
		json.NewEncoder(w).Encode(Run{
			RunID:  "run-await-1",
			Status: RunStatusCompleted,
			Output: []Message{NewAgentMessage("Approved")},
		})
	}))
	defer ts.Close()

	c := newClient("test", ts.URL, nil)
	resume := &AwaitResume{Message: &Message{
		Role:  "user",
		Parts: []MessagePart{{ContentType: "text/plain", Content: "yes"}},
	}}
	run, err := c.resumeRun(context.Background(), "run-await-1", resume)
	require.NoError(t, err)
	assert.Equal(t, RunStatusCompleted, run.Status)
}

// --- Type tests ---

func TestRunStatus_IsTerminal(t *testing.T) {
	assert.True(t, RunStatusCompleted.IsTerminal())
	assert.True(t, RunStatusFailed.IsTerminal())
	assert.True(t, RunStatusCancelled.IsTerminal())
	assert.False(t, RunStatusCreated.IsTerminal())
	assert.False(t, RunStatusInProgress.IsTerminal())
	assert.False(t, RunStatusAwaiting.IsTerminal())
	assert.False(t, RunStatusCancelling.IsTerminal())
}

func TestTextContent(t *testing.T) {
	msgs := []Message{
		{Role: "agent", Parts: []MessagePart{{ContentType: "text/plain", Content: "Hello"}}},
		{Role: "agent", Parts: []MessagePart{{ContentType: "text/plain", Content: "World"}}},
	}
	assert.Equal(t, "Hello\nWorld", TextContent(msgs))
}

func TestTextContent_Empty(t *testing.T) {
	assert.Equal(t, "", TextContent(nil))
	assert.Equal(t, "", TextContent([]Message{}))
}

func TestNewUserMessage(t *testing.T) {
	msg := NewUserMessage("hello")
	assert.Equal(t, "user", msg.Role)
	assert.Len(t, msg.Parts, 1)
	assert.Equal(t, "text/plain", msg.Parts[0].ContentType)
	assert.Equal(t, "hello", msg.Parts[0].Content)
}

func TestNewAgentMessage(t *testing.T) {
	msg := NewAgentMessage("done")
	assert.Equal(t, "agent", msg.Role)
	assert.Equal(t, "done", msg.Parts[0].Content)
}

// --- Test helpers ---

func newTestACP(t *testing.T, serverURL string) *acpIntegration {
	t.Helper()
	return &acpIntegration{
		clients: map[string]*client{
			"test": newClient("test", serverURL, nil),
		},
		timeout: 10 * time.Second,
	}
}

func newMockAgentsServer(t *testing.T, agents []AgentManifest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(AgentsListResponse{Agents: agents})
	}))
}

func newEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/runs" && r.Method == http.MethodPost {
			var req RunCreateRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Mode == RunModeStream {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(ACPError{Message: "stream not supported"})
				return
			}

			json.NewEncoder(w).Encode(Run{
				AgentName: req.AgentName,
				RunID:     "run-echo-1",
				Status:    RunStatusCompleted,
				Output:    []Message{NewAgentMessage("Echo: " + TextContent(req.Input))},
			})
		}
	}))
}

func newStreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ContentTypeNDJSON)
		w.WriteHeader(http.StatusOK)
		events := []string{
			`{"type":"run.in-progress","run":{"agent_name":"echo","run_id":"r1","status":"in-progress","output":[],"created_at":"2025-01-01T00:00:00Z"}}`,
			`{"type":"message.part","part":{"content_type":"text/plain","content":"Hello"}}`,
			`{"type":"message.part","part":{"content_type":"text/plain","content":" World"}}`,
			`{"type":"run.completed","run":{"agent_name":"echo","run_id":"r1","status":"completed","output":[],"created_at":"2025-01-01T00:00:00Z"}}`,
		}
		for _, e := range events {
			fmt.Fprintf(w, "%s\n", e)
		}
	}))
}

func newAwaitServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ContentTypeNDJSON)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"type":"run.awaiting","run":{"agent_name":"approval","run_id":"run-await-1","status":"awaiting","output":[],"await_request":{"message":{"role":"agent","parts":[{"content_type":"text/plain","content":"Do you approve?"}]}},"created_at":"2025-01-01T00:00:00Z"}}`+"\n")
	}))
}

func newResumeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var req RunResumeRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Mode == RunModeStream {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(ACPError{Message: "stream not supported"})
				return
			}

			json.NewEncoder(w).Encode(Run{
				AgentName: "approval",
				RunID:     req.RunID,
				Status:    RunStatusCompleted,
				Output:    []Message{NewAgentMessage("Approved and processed")},
			})
		}
	}))
}
