package wasm

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed testdata/example.wasm
var exampleWasm []byte

func loadTestModule(t *testing.T) *Module {
	t.Helper()
	ctx := context.Background()
	rt, err := NewRuntime(ctx)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { rt.Close(ctx) })

	mod, err := rt.LoadModule(ctx, exampleWasm)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	t.Cleanup(func() { mod.Close(ctx) })

	return mod
}

func TestName(t *testing.T) {
	mod := loadTestModule(t)
	if got := mod.Name(); got != "example" {
		t.Errorf("Name() = %q, want %q", got, "example")
	}
}

func TestSetName(t *testing.T) {
	mod := loadTestModule(t)
	mod.SetName("custom")
	if got := mod.Name(); got != "custom" {
		t.Errorf("Name() = %q, want %q", got, "custom")
	}
}

func TestSetName_Empty(t *testing.T) {
	mod := loadTestModule(t)
	mod.SetName("")
	if got := mod.Name(); got != "example" {
		t.Errorf("Name() = %q, want %q (should fall back to wasm export)", got, "example")
	}
}

func TestTools(t *testing.T) {
	mod := loadTestModule(t)
	tools := mod.Tools()
	if len(tools) == 0 {
		t.Fatal("Tools() returned empty slice")
	}

	if got := len(tools); got != 3 {
		t.Errorf("Tools() returned %d tools, want 3", got)
	}

	names := make(map[mcp.ToolName]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, want := range []mcp.ToolName{
		"example_echo",
		"example_http_get",
		"example_list_items",
	} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestConfigure_Success(t *testing.T) {
	mod := loadTestModule(t)
	err := mod.Configure(context.Background(), mcp.Credentials{
		"base_url": "https://example.com",
		"api_key":  "test-key",
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
}

func TestConfigure_MissingBaseURL(t *testing.T) {
	mod := loadTestModule(t)
	err := mod.Configure(context.Background(), mcp.Credentials{
		"api_key": "test-key",
	})
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
	if !strings.Contains(err.Error(), "base_url") {
		t.Errorf("error should mention base_url: %v", err)
	}
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	mod := loadTestModule(t)
	err := mod.Configure(context.Background(), mcp.Credentials{
		"base_url": "https://example.com",
	})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Errorf("error should mention api_key: %v", err)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url": "https://example.com",
		"api_key":  "test-key",
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

func TestExecute_Echo(t *testing.T) {
	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url": "https://example.com",
		"api_key":  "test-key",
	})

	result, err := mod.Execute(context.Background(), "example_echo", map[string]any{
		"message": "hello world",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
	if !strings.Contains(result.Data, "hello world") {
		t.Errorf("response should contain 'hello world': %s", result.Data)
	}
}

func TestExecute_Echo_MissingArgs(t *testing.T) {
	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url": "https://example.com",
		"api_key":  "test-key",
	})

	result, err := mod.Execute(context.Background(), "example_echo", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing args")
	}
	if !strings.Contains(result.Data, "message") {
		t.Errorf("error should mention message: %s", result.Data)
	}
}

func TestExecute_HTTPGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/users") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("unexpected auth: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"name":"alice"}]`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url": srv.URL,
		"api_key":  "test-key",
	})

	result, err := mod.Execute(context.Background(), "example_http_get", map[string]any{
		"path": "/users",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
	if !strings.Contains(result.Data, "alice") {
		t.Errorf("response should contain alice: %s", result.Data)
	}
}

func TestExecute_ListItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":"item-1","name":"widget"}]`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url": srv.URL,
		"api_key":  "test-key",
	})

	result, err := mod.Execute(context.Background(), "example_list_items", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Data)
	}
	if !strings.Contains(result.Data, "widget") {
		t.Errorf("response should contain widget: %s", result.Data)
	}
}

func TestExecute_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url": srv.URL,
		"api_key":  "test-key",
	})

	result, err := mod.Execute(context.Background(), "example_list_items", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	_ = mod.Configure(context.Background(), mcp.Credentials{
		"base_url": srv.URL,
		"api_key":  "test-key",
	})

	if !mod.Healthy(context.Background()) {
		t.Error("expected Healthy() to return true")
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	mod := loadTestModule(t)
	tools := mod.Tools()

	for _, tool := range tools {
		result, err := mod.Execute(context.Background(), mcp.ToolName(tool.Name), map[string]any{})
		if err != nil {
			continue
		}
		if result != nil && result.IsError && strings.Contains(result.Data, "unknown tool") {
			t.Errorf("tool %q is defined but not in dispatch map", tool.Name)
		}
	}
}

func TestIntegrationInterface(t *testing.T) {
	mod := loadTestModule(t)
	var _ mcp.Integration = mod
}

func TestFieldCompactionInterface(t *testing.T) {
	mod := loadTestModule(t)
	var _ mcp.FieldCompactionIntegration = mod
}

func TestCompactSpec_HasSpecs(t *testing.T) {
	mod := loadTestModule(t)
	fields, ok := mod.CompactSpec("example_list_items")
	if !ok {
		t.Fatal("CompactSpec should return true for example_list_items")
	}
	if len(fields) == 0 {
		t.Fatal("CompactSpec should return non-empty fields for example_list_items")
	}
}

func TestCompactSpec_NoSpecs(t *testing.T) {
	mod := loadTestModule(t)
	_, ok := mod.CompactSpec("example_echo")
	if ok {
		t.Error("CompactSpec should return false for example_echo (no specs defined)")
	}
}

func TestCompactSpec_AppliesCorrectly(t *testing.T) {
	mod := loadTestModule(t)
	fields, ok := mod.CompactSpec("example_list_items")
	if !ok {
		t.Fatal("CompactSpec should return true for example_list_items")
	}

	input := []any{
		map[string]any{"id": "item-1", "name": "widget", "extra": "drop-me", "price": 9.99},
		map[string]any{"id": "item-2", "name": "gadget", "extra": "drop-me-too", "price": 19.99},
	}
	result := mcp.CompactAny(input, fields)

	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 items, got %d", len(arr))
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", arr[0])
	}
	if first["id"] != "item-1" {
		t.Errorf("expected id=item-1, got %v", first["id"])
	}
	if first["name"] != "widget" {
		t.Errorf("expected name=widget, got %v", first["name"])
	}
	if _, exists := first["extra"]; exists {
		t.Error("extra field should have been compacted out")
	}
	if _, exists := first["price"]; exists {
		t.Error("price field should have been compacted out")
	}
}

func TestCompactSpec_AtLeastOneTool(t *testing.T) {
	mod := loadTestModule(t)
	tools := mod.Tools()

	specsFound := 0
	for _, tool := range tools {
		if _, ok := mod.CompactSpec(tool.Name); ok {
			specsFound++
		}
	}
	if specsFound == 0 {
		t.Error("expected at least one tool to have compact specs (not all tools need specs)")
	}
}
