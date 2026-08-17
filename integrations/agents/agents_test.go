package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

func TestName(t *testing.T) {
	i := New()
	if i.Name() != "agents" {
		t.Fatalf("expected name 'agents', got %q", i.Name())
	}
}

func TestConfigureRequiresBaseURL(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	if err == nil {
		t.Fatal("expected error for empty credentials")
	}
	if err.Error() != "agents: base_url is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigureSuccess(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  "http://localhost:9099",
		"token":    "test-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigureDeriveA2AURL(t *testing.T) {
	i := New().(*integration)
	err := i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if i.a2aURL != "http://localhost:9099" {
		t.Fatalf("expected derived a2a_url 'http://localhost:9099', got %q", i.a2aURL)
	}
}

func TestDeriveA2AURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://localhost:9098", "http://localhost:9099"},
		{"http://example.com:8080", "http://example.com:8081"},
		{"http://192.168.1.1:3000", "http://192.168.1.1:3001"},
		{"http://noport", "http://noport"}, // no port to increment
	}
	for _, tt := range tests {
		got := deriveA2AURL(tt.input)
		if got != tt.expected {
			t.Errorf("deriveA2AURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	if len(tools) == 0 {
		t.Fatal("expected tools, got none")
	}

	// Check that all expected proxy tools exist.
	expectedNames := []string{
		"agents_proxy_list",
		"agents_proxy_send_message",
		"agents_proxy_get_task",
		"agents_proxy_cancel_task",
		"agents_route_message",
		"agents_agent_card",
	}
	nameSet := make(map[mcp.ToolName]bool)
	for _, tool := range tools {
		nameSet[tool.Name] = true
	}
	for _, name := range expectedNames {
		if !nameSet[mcp.ToolName(name)] {
			t.Errorf("missing tool %q", name)
		}
	}
}

func TestPlainTextKeys(t *testing.T) {
	i := New().(mcp.PlainTextCredentials)
	keys := i.PlainTextKeys()
	expected := map[string]bool{"base_url": true, "a2a_url": true}
	for _, k := range keys {
		if !expected[k] {
			t.Errorf("unexpected plain text key: %q", k)
		}
		delete(expected, k)
	}
	for k := range expected {
		t.Errorf("missing plain text key: %q", k)
	}
}

func TestOptionalKeys(t *testing.T) {
	i := New().(mcp.OptionalCredentials)
	keys := i.OptionalKeys()
	expected := map[string]bool{"token": true, "a2a_url": true}
	for _, k := range keys {
		if !expected[k] {
			t.Errorf("unexpected optional key: %q", k)
		}
		delete(expected, k)
	}
	for k := range expected {
		t.Errorf("missing optional key: %q", k)
	}
}

// ---------------------------------------------------------------------------
// A2A proxy handler tests with mock HTTP server
// ---------------------------------------------------------------------------

func TestProxyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a/agents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		if r.Method != "GET" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"test-agent","description":"A test agent"}]`)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098", // won't be used for proxy
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_proxy_list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Data)
	}
	if result.Data == "{}" {
		t.Fatal("got empty {} response — this was the original bug")
	}
	var agents []map[string]any
	if err := json.Unmarshal([]byte(result.Data), &agents); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0]["name"] != "test-agent" {
		t.Fatalf("expected agent name 'test-agent', got %v", agents[0]["name"])
	}
}

func TestProxySendMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a/agents/test-agent/message:send" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected content-type: %s", ct)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		msg, _ := body["message"].(map[string]any)
		if msg == nil {
			t.Fatal("expected 'message' in body")
		}
		if msg["role"] != "ROLE_USER" {
			t.Errorf("expected role ROLE_USER, got %v", msg["role"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"task":{"id":"task-123","status":{"state":"working"}}}`)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_proxy_send_message", map[string]any{
		"agent_id": "test-agent",
		"message":  "Hello, agent!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Data)
	}
	if result.Data == "{}" {
		t.Fatal("got empty {} response — this was the original bug")
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(result.Data), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	task, _ := resp["task"].(map[string]any)
	if task == nil {
		t.Fatal("expected 'task' in response")
	}
	if task["id"] != "task-123" {
		t.Fatalf("expected task id 'task-123', got %v", task["id"])
	}
}

func TestProxyGetTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a/agents/agent-1/tasks/task-abc" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"task-abc","status":{"state":"completed"},"artifacts":[{"text":"done"}]}`)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_proxy_get_task", map[string]any{
		"agent_id": "agent-1",
		"task_id":  "task-abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Data)
	}
	if result.Data == "{}" {
		t.Fatal("got empty {} response — this was the original bug")
	}
}

func TestProxyCancelTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a/agents/agent-1/tasks/task-abc:cancel" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"task-abc","status":{"state":"canceled"}}`)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_proxy_cancel_task", map[string]any{
		"agent_id": "agent-1",
		"task_id":  "task-abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Data)
	}
}

func TestRouteMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a/route/message:send" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		routing, _ := body["routing"].(map[string]any)
		if routing == nil {
			t.Fatal("expected 'routing' in body")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"task":{"id":"routed-task-1","status":{"state":"working"}}}`)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_route_message", map[string]any{
		"message": "Help me with coding",
		"tags":    `["coding", "go"]`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Data)
	}
}

func TestProxyListEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate the old daemon bug: returns 200 with empty body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_proxy_list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return an error result explaining the issue, not empty {}
	if !result.IsError {
		t.Fatal("expected error result for empty response")
	}
	if result.Data == "{}" {
		t.Fatal("got empty {} — should explain the problem")
	}
}

func TestProxyListHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_proxy_list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for 401 response")
	}
}

func TestAgentCardPathEncoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The agent ID "workspace/agent" should be encoded.
		expected := "/a2a/agents/workspace%2Fagent/.well-known/agent-card.json"
		if r.URL.RawPath != expected && r.URL.Path != "/a2a/agents/workspace%2Fagent/.well-known/agent-card.json" {
			t.Errorf("unexpected path: %s (raw: %s)", r.URL.Path, r.URL.RawPath)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"my-agent","skills":[]}`)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
	})

	result, err := i.Execute(context.Background(), "agents_agent_card", map[string]any{
		"agent_id": "workspace/agent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Data)
	}
}

func TestProxySendMessageWithAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-token" {
			t.Errorf("expected 'Bearer my-token' auth header, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"task":{"id":"t1"}}`)
	}))
	defer srv.Close()

	i := New().(*integration)
	_ = i.Configure(context.Background(), mcp.Credentials{
		"base_url": "http://localhost:9098",
		"a2a_url":  srv.URL,
		"token":    "my-token",
	})

	result, err := i.Execute(context.Background(), "agents_proxy_send_message", map[string]any{
		"agent_id": "agent-1",
		"message":  "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Data)
	}
}
