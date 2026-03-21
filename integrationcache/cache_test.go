package integrationcache

import (
	"context"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type stubIntegration struct {
	name string
}

func (s *stubIntegration) Name() string                                     { return s.name }
func (s *stubIntegration) Configure(context.Context, mcp.Credentials) error { return nil }
func (s *stubIntegration) Tools() []mcp.ToolDefinition                      { return nil }
func (s *stubIntegration) Execute(context.Context, string, map[string]any) (*mcp.ToolResult, error) {
	return nil, nil
}
func (s *stubIntegration) Healthy(context.Context) bool { return true }

func TestCache_PutGet(t *testing.T) {
	c := New(10*time.Minute, 100)
	inst := &stubIntegration{name: "github"}

	c.Put("tenant-1", "github", inst, "hash1")

	got, ok := c.Get("tenant-1", "github", "hash1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Name() != "github" {
		t.Errorf("Name() = %q, want %q", got.Name(), "github")
	}
}

func TestCache_Miss(t *testing.T) {
	c := New(10*time.Minute, 100)

	_, ok := c.Get("tenant-1", "github", "hash1")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	now := time.Now()
	c := New(5*time.Minute, 100)
	c.now = func() time.Time { return now }

	inst := &stubIntegration{name: "github"}
	c.Put("tenant-1", "github", inst, "hash1")

	c.now = func() time.Time { return now.Add(6 * time.Minute) }
	_, ok := c.Get("tenant-1", "github", "hash1")
	if ok {
		t.Fatal("expected cache miss after TTL")
	}
}

func TestCache_CredHashMismatch(t *testing.T) {
	c := New(10*time.Minute, 100)
	inst := &stubIntegration{name: "github"}

	c.Put("tenant-1", "github", inst, "hash1")

	_, ok := c.Get("tenant-1", "github", "hash2")
	if ok {
		t.Fatal("expected cache miss on cred hash mismatch")
	}
}

func TestCache_Evict(t *testing.T) {
	c := New(10*time.Minute, 100)
	c.Put("tenant-1", "github", &stubIntegration{name: "github"}, "h1")
	c.Put("tenant-1", "datadog", &stubIntegration{name: "datadog"}, "h2")
	c.Put("tenant-2", "github", &stubIntegration{name: "github"}, "h3")

	c.Evict("tenant-1")

	if _, ok := c.Get("tenant-1", "github", "h1"); ok {
		t.Error("tenant-1/github should be evicted")
	}
	if _, ok := c.Get("tenant-1", "datadog", "h2"); ok {
		t.Error("tenant-1/datadog should be evicted")
	}
	if _, ok := c.Get("tenant-2", "github", "h3"); !ok {
		t.Error("tenant-2/github should still be cached")
	}
}

func TestCache_LRUEviction(t *testing.T) {
	now := time.Now()
	c := New(10*time.Minute, 2)
	c.now = func() time.Time { return now }

	c.Put("t1", "a", &stubIntegration{name: "a"}, "h1")

	c.now = func() time.Time { return now.Add(1 * time.Second) }
	c.Put("t1", "b", &stubIntegration{name: "b"}, "h2")

	c.now = func() time.Time { return now.Add(2 * time.Second) }
	c.Put("t1", "c", &stubIntegration{name: "c"}, "h3")

	if c.Len() > 2 {
		t.Errorf("Len() = %d, want <= 2", c.Len())
	}
	if _, ok := c.Get("t1", "a", "h1"); ok {
		t.Error("oldest entry 'a' should have been evicted")
	}
}

func TestHashCreds(t *testing.T) {
	creds := mcp.Credentials{"token": "abc", "key": "xyz"}
	h1 := HashCreds(creds)
	h2 := HashCreds(creds)

	if h1 == "" {
		t.Error("HashCreds returned empty")
	}
	if len(h1) != 64 {
		t.Errorf("HashCreds len = %d, want 64", len(h1))
	}
	if h1 != h2 {
		t.Error("HashCreds not deterministic for same input")
	}
}
