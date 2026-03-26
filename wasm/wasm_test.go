package wasm

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed testdata/overmind.wasm
var overmindWasm []byte

func loadTestModule(t *testing.T) *Module {
	t.Helper()
	ctx := context.Background()
	rt, err := NewRuntime(ctx)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { rt.Close(ctx) })

	mod, err := rt.LoadModule(ctx, overmindWasm)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	t.Cleanup(func() { mod.Close(ctx) })

	return mod
}

func TestName(t *testing.T) {
	mod := loadTestModule(t)
	if got := mod.Name(); got != "overmind" {
		t.Errorf("Name() = %q, want %q", got, "overmind")
	}
}

func TestTools(t *testing.T) {
	mod := loadTestModule(t)
	tools := mod.Tools()
	if len(tools) == 0 {
		t.Fatal("Tools() returned empty slice")
	}

	// Verify we have the expected tool count (46 tools, matching native overmind)
	if got := len(tools); got != 46 {
		t.Errorf("Tools() returned %d tools, want 46", got)
	}

	// Spot-check a few tool names
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, want := range []string{
		"overmind_list_available_agents",
		"overmind_launch_agent",
		"overmind_complete_flow",
		"overmind_list_agents",
		"overmind_list_mcp_roles",
	} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestConfigure_Success(t *testing.T) {
	mod := loadTestModule(t)
	err := mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     "https://example.com",
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
}

func TestConfigure_MissingBaseURL(t *testing.T) {
	mod := loadTestModule(t)
	err := mod.Configure(context.Background(), mcp.Credentials{
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
	if !strings.Contains(err.Error(), "base_url") {
		t.Errorf("error should mention base_url: %v", err)
	}
}

func TestConfigure_MissingToken(t *testing.T) {
	mod := loadTestModule(t)
	err := mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     "https://example.com",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error should mention token: %v", err)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     "https://example.com",
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "nonexistent_tool", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown tool")
	}
	if !strings.Contains(result.Data, "unknown tool") {
		t.Errorf("error should mention unknown tool: %v", result.Data)
	}
}

func TestExecute_ListAvailableAgents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/flow_runs/flow-456/available_agents") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("unexpected auth: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"agent-1","name":"test-agent"}]`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     srv.URL,
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_list_available_agents", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
	if !strings.Contains(result.Data, "agent-1") {
		t.Errorf("response should contain agent-1: %s", result.Data)
	}
}

func TestExecute_LaunchAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		if req["agent_id"] != "agent-1" {
			t.Errorf("expected agent_id=agent-1, got %v", req["agent_id"])
		}
		if req["prompt"] != "do something" {
			t.Errorf("expected prompt='do something', got %v", req["prompt"])
		}
		if req["parent_run_id"] != "run-123" {
			t.Errorf("expected parent_run_id=run-123, got %v", req["parent_run_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"agent_run_id":"new-run-789"}`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     srv.URL,
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_launch_agent", map[string]any{
		"agent_id": "agent-1",
		"prompt":   "do something",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
	if !strings.Contains(result.Data, "new-run-789") {
		t.Errorf("response should contain new-run-789: %s", result.Data)
	}
}

func TestExecute_LaunchAgent_MissingArgs(t *testing.T) {
	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     "https://example.com",
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_launch_agent", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing args")
	}
	if !strings.Contains(result.Data, "agent_id") {
		t.Errorf("error should mention agent_id: %s", result.Data)
	}
}

func TestExecute_CompleteFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		if req["summary"] != "all done" {
			t.Errorf("expected summary='all done', got %v", req["summary"])
		}
		if req["status"] != "success" {
			t.Errorf("expected status='success', got %v", req["status"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"completed"}`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     srv.URL,
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_complete_flow", map[string]any{
		"summary": "all done",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
}

func TestExecute_CompleteFlow_InvalidStatus(t *testing.T) {
	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     "https://example.com",
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_complete_flow", map[string]any{
		"summary": "done",
		"status":  "invalid",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid status")
	}
}

func TestExecute_GetAgentStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/agent_runs/run-abc/status") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"state":"completed"}`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     srv.URL,
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_get_agent_status", map[string]any{
		"agent_run_id": "run-abc",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
	if !strings.Contains(result.Data, "completed") {
		t.Errorf("response should contain completed: %s", result.Data)
	}
}

func TestExecute_ListAgents_Admin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`[{"id":"a1","name":"agent1"}]`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     srv.URL,
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_list_agents", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
	if !strings.Contains(result.Data, "agent1") {
		t.Errorf("response should contain agent1: %s", result.Data)
	}
}

func TestExecute_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     srv.URL,
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	result, err := mod.Execute(context.Background(), "overmind_list_agents", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url":     srv.URL,
		"token":        "test-token",
		"agent_run_id": "run-123",
		"flow_run_id":  "flow-456",
	})

	if !mod.Healthy(context.Background()) {
		t.Error("expected Healthy() to return true")
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	mod := loadTestModule(t)
	tools := mod.Tools()

	for _, tool := range tools {
		result, err := mod.Execute(context.Background(), tool.Name, map[string]any{})
		if err != nil {
			// This would be a runtime/wasm error, which means the tool was dispatched
			continue
		}
		// The tool should either succeed (unlikely without config) or return an error result
		// but should NOT return "unknown tool"
		if result != nil && result.IsError && strings.Contains(result.Data, "unknown tool") {
			t.Errorf("tool %q is defined but not in dispatch map", tool.Name)
		}
	}
}

func TestIntegrationInterface(t *testing.T) {
	mod := loadTestModule(t)
	var _ mcp.Integration = mod
}
