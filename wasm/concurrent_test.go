package wasm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

// TestModule_ConcurrentExecute verifies that concurrent calls into a single
// *Module do not panic or corrupt guest memory. Wazero's api.Function.Call is
// not goroutine-safe, so Module must serialize calls internally.
//
// Reproduces the production panic at wasm/memory.go:51 where Goja scripts
// firing many api.call() invocations in parallel hit nil pointer derefs
// inside readFromGuest.
func TestModule_ConcurrentExecute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"item-1","name":"widget"}]`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	if err := mod.Configure(context.Background(), mcp.Credentials{
		"base_url": srv.URL,
		"api_key":  "test-key",
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	const goroutines = 32
	const iterations = 8

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)
	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := range iterations {
				result, err := mod.Execute(context.Background(), "example_echo", map[string]any{
					"message": "hello",
				})
				if err != nil {
					errCh <- err
					return
				}
				if result.IsError {
					errCh <- &execErr{data: result.Data}
					return
				}
				if !strings.Contains(result.Data, "hello") {
					errCh <- &execErr{data: result.Data}
					return
				}
				_ = id
				_ = i
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent Execute error: %v", err)
	}
}

// TestModule_ConcurrentMixed exercises Execute, Tools, Name, Healthy in parallel.
// All of these touch m.mod and must be serialized.
func TestModule_ConcurrentMixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"item-1","name":"widget"}]`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	if err := mod.Configure(context.Background(), mcp.Credentials{
		"base_url": srv.URL,
		"api_key":  "test-key",
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	const goroutines = 16
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			switch id % 4 {
			case 0:
				_, _ = mod.Execute(context.Background(), "example_echo", map[string]any{"message": "x"})
			case 1:
				_ = mod.Tools()
			case 2:
				_ = mod.Name()
			case 3:
				_ = mod.Healthy(context.Background())
			}
		}(g)
	}
	wg.Wait()
}

// TestModule_ExecuteAfterClose verifies that calls into a closed module return
// a clean error instead of panicking with a nil pointer deref.
func TestModule_ExecuteAfterClose(t *testing.T) {
	mod := loadTestModule(t)
	ctx := context.Background()
	if err := mod.Configure(ctx, mcp.Credentials{
		"base_url": "https://example.com",
		"api_key":  "test-key",
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if err := mod.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Subsequent calls must not panic.
	result, err := mod.Execute(ctx, "example_echo", map[string]any{"message": "x"})
	if err == nil && (result == nil || !result.IsError) {
		t.Fatal("expected Execute after Close to return an error")
	}

	if mod.Healthy(ctx) {
		t.Error("Healthy() should return false after Close")
	}
	if tools := mod.Tools(); tools != nil {
		t.Errorf("Tools() should return nil after Close, got %d tools", len(tools))
	}
}

// TestModule_ConcurrentExecuteAndClose races Close against in-flight Execute calls.
// The Execute caller must receive either a valid result or a clean error — never a panic.
func TestModule_ConcurrentExecuteAndClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"item-1","name":"widget"}]`))
	}))
	defer srv.Close()

	mod := loadTestModule(t)
	if err := mod.Configure(context.Background(), mcp.Credentials{
		"base_url": srv.URL,
		"api_key":  "test-key",
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	const callers = 8
	var wg sync.WaitGroup
	var panicked atomic.Bool
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Store(true)
					t.Errorf("Execute panicked: %v", r)
				}
			}()
			for range 50 {
				_, _ = mod.Execute(context.Background(), "example_echo", map[string]any{
					"message": "hello",
				})
			}
		}()
	}

	// Close concurrently with the in-flight callers.
	_ = mod.Close(context.Background())
	wg.Wait()

	if panicked.Load() {
		t.Fatal("at least one goroutine panicked")
	}
}

type execErr struct{ data string }

func (e *execErr) Error() string { return e.data }
