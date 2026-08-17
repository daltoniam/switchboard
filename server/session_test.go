package server

import (
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_SetAndGetContext(t *testing.T) {
	s := newSession("test-1")
	s.SetContext(map[string]any{"owner": "daltoniam", "repo": "switchboard"})
	got := s.GetContext()
	assert.Equal(t, "daltoniam", got["owner"])
	assert.Equal(t, "switchboard", got["repo"])
}

func TestSession_SetContextMerges(t *testing.T) {
	s := newSession("test-1")
	s.SetContext(map[string]any{"owner": "daltoniam"})
	s.SetContext(map[string]any{"repo": "switchboard"})
	got := s.GetContext()
	assert.Equal(t, "daltoniam", got["owner"])
	assert.Equal(t, "switchboard", got["repo"])
}

func TestSession_SetContextOverwrites(t *testing.T) {
	s := newSession("test-1")
	s.SetContext(map[string]any{"owner": "old"})
	s.SetContext(map[string]any{"owner": "new"})
	got := s.GetContext()
	assert.Equal(t, "new", got["owner"])
}

func TestSession_ClearContext(t *testing.T) {
	s := newSession("test-1")
	s.SetContext(map[string]any{"owner": "daltoniam"})
	s.ClearContext()
	got := s.GetContext()
	assert.Empty(t, got)
}

func TestSession_GetContextReturnsCopy(t *testing.T) {
	s := newSession("test-1")
	s.SetContext(map[string]any{"owner": "daltoniam"})
	got := s.GetContext()
	got["owner"] = "mutated"
	assert.Equal(t, "daltoniam", s.GetContext()["owner"])
}

func TestSession_MergeDefaults(t *testing.T) {
	tests := []struct {
		name    string
		ctx     map[string]any
		args    map[string]any
		wantKey string
		wantVal any
	}{
		{
			name:    "session value used when arg missing",
			ctx:     map[string]any{"owner": "daltoniam"},
			args:    map[string]any{"state": "open"},
			wantKey: "owner",
			wantVal: "daltoniam",
		},
		{
			name:    "explicit arg overrides session",
			ctx:     map[string]any{"owner": "session-val"},
			args:    map[string]any{"owner": "explicit-val"},
			wantKey: "owner",
			wantVal: "explicit-val",
		},
		{
			name:    "empty context returns args unchanged",
			ctx:     map[string]any{},
			args:    map[string]any{"state": "open"},
			wantKey: "state",
			wantVal: "open",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSession("test")
			s.SetContext(tt.ctx)
			merged := s.MergeDefaults(tt.args)
			assert.Equal(t, tt.wantVal, merged[tt.wantKey])
		})
	}
}

func TestSession_MergeDefaultsDoesNotMutateOriginalArgs(t *testing.T) {
	s := newSession("test")
	s.SetContext(map[string]any{"owner": "daltoniam"})
	args := map[string]any{"state": "open"}
	s.MergeDefaults(args)
	_, exists := args["owner"]
	assert.False(t, exists, "original args map should not be mutated")
}

func TestSession_AddBreadcrumb(t *testing.T) {
	s := newSession("test")
	s.AddBreadcrumb("github_list_issues", map[string]any{"owner": "o"}, `[{"id":1}]`, false)
	require.Len(t, s.Breadcrumbs, 1)
	bc := s.Breadcrumbs[0]
	assert.Equal(t, 1, bc.Seq)
	assert.Equal(t, mcp.ToolName("github_list_issues"), bc.Tool)
	assert.False(t, bc.IsError)
}

func TestSession_BreadcrumbCap(t *testing.T) {
	s := newSession("test")
	for i := range MaxBreadcrumbs + 50 {
		s.AddBreadcrumb("tool", nil, "result", false)
		_ = i
	}
	assert.Len(t, s.Breadcrumbs, MaxBreadcrumbs)
	assert.Equal(t, 51, s.Breadcrumbs[0].Seq)
}

func TestSession_BreadcrumbSummaryTruncation(t *testing.T) {
	s := newSession("test")
	longResult := strings.Repeat("x", 500)
	s.AddBreadcrumb("tool", nil, longResult, false)
	assert.LessOrEqual(t, len(s.Breadcrumbs[0].Summary), maxBreadcrumbSummary+3)
}

func TestSession_RecentBreadcrumbs(t *testing.T) {
	s := newSession("test")
	s.AddBreadcrumb("tool_a", nil, "a", false)
	s.AddBreadcrumb("tool_b", nil, "b", false)
	s.AddBreadcrumb("tool_a", nil, "c", false)

	t.Run("last 2", func(t *testing.T) {
		bcs := s.RecentBreadcrumbs(2, "")
		require.Len(t, bcs, 2)
		assert.Equal(t, mcp.ToolName("tool_b"), bcs[0].Tool)
		assert.Equal(t, mcp.ToolName("tool_a"), bcs[1].Tool)
	})

	t.Run("filter by tool", func(t *testing.T) {
		bcs := s.RecentBreadcrumbs(10, "tool_a")
		require.Len(t, bcs, 2)
		for _, bc := range bcs {
			assert.Equal(t, mcp.ToolName("tool_a"), bc.Tool)
		}
	})
}

func TestSessionStore_GetOrCreate(t *testing.T) {
	ss := NewMemorySessionStore(time.Hour)
	s1 := ss.GetOrCreate("sess-1")
	s2 := ss.GetOrCreate("sess-1")
	assert.Same(t, s1, s2, "should return same session")
	assert.Equal(t, 1, ss.Len())
}

func TestSessionStore_TTLExpiry(t *testing.T) {
	ss := NewMemorySessionStore(10 * time.Millisecond)
	s := ss.GetOrCreate("sess-1")
	_ = s
	time.Sleep(20 * time.Millisecond)

	_, ok := ss.Get("sess-1")
	assert.False(t, ok, "session should be expired")
}

func TestSessionStore_GetOrCreateEvictsExpired(t *testing.T) {
	ss := NewMemorySessionStore(10 * time.Millisecond)
	ss.GetOrCreate("old-sess")
	time.Sleep(20 * time.Millisecond)

	ss.GetOrCreate("new-sess")
	assert.Equal(t, 1, ss.Len(), "expired session should be evicted")
}

func TestSessionStore_Delete(t *testing.T) {
	ss := NewMemorySessionStore(time.Hour)
	ss.GetOrCreate("sess-1")
	ss.Delete("sess-1")
	_, ok := ss.Get("sess-1")
	assert.False(t, ok)
}

func TestSessionContext_RoundTrip(t *testing.T) {
	s := newSession("ctx-test")
	s.SetContext(map[string]any{"owner": "test"})

	ctx := withSession(t.Context(), s)
	got := sessionFromCtx(ctx)
	require.NotNil(t, got)
	assert.Equal(t, "test", got.GetContext()["owner"])
}

func TestSessionContext_MissingReturnsNil(t *testing.T) {
	got := sessionFromCtx(t.Context())
	assert.Nil(t, got)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
}

func TestSummarizeArgs(t *testing.T) {
	args := map[string]any{
		"short":   "hello",
		"long":    strings.Repeat("x", 200),
		"numeric": 42,
	}
	summary := summarizeArgs(args)
	assert.Equal(t, "hello", summary["short"])
	assert.LessOrEqual(t, len(summary["long"].(string)), 103)
	assert.Equal(t, 42, summary["numeric"])
}

func TestSummarizeArgs_Nil(t *testing.T) {
	assert.Nil(t, summarizeArgs(nil))
}
