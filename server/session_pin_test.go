package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_PinResult(t *testing.T) {
	s := newSession("test")
	h := s.PinResult("github_get_repo", `{"id":1,"name":"switchboard"}`)
	assert.Equal(t, "$1", h)
	assert.Equal(t, 1, s.PinnedCount())

	pr, ok := s.GetPinned("$1")
	require.True(t, ok)
	assert.Equal(t, "github_get_repo", string(pr.Tool))
	assert.Contains(t, string(pr.Data), "switchboard")
}

func TestSession_PinResultIncrementsHandle(t *testing.T) {
	s := newSession("test")
	h1 := s.PinResult("tool_a", `"a"`)
	h2 := s.PinResult("tool_b", `"b"`)
	assert.Equal(t, "$1", h1)
	assert.Equal(t, "$2", h2)
}

func TestSession_Unpin(t *testing.T) {
	s := newSession("test")
	s.PinResult("tool", `"data"`)
	assert.True(t, s.Unpin("$1"))
	assert.Equal(t, 0, s.PinnedCount())
	assert.False(t, s.Unpin("$1"))
}

func TestSession_ListPinned(t *testing.T) {
	s := newSession("test")
	s.PinResult("tool_a", `"a"`)
	s.PinResult("tool_b", `"b"`)
	list := s.ListPinned()
	assert.Len(t, list, 2)
}

func TestSession_PinnedBytes(t *testing.T) {
	s := newSession("test")
	data := `{"key":"value"}`
	s.PinResult("tool", data)
	assert.Equal(t, len(data), s.PinnedBytes())
}

func TestSession_PinnedEviction(t *testing.T) {
	s := newSession("test")
	big := strings.Repeat("x", MaxPinnedBytes)
	s.PinResult("tool_big", big)
	assert.Equal(t, 1, s.PinnedCount())
	assert.True(t, s.PinnedBytes() >= MaxPinnedBytes)

	s.PinResult("tool_new", `"small"`)
	assert.Equal(t, 1, s.PinnedCount())
	_, ok := s.GetPinned("$1")
	assert.False(t, ok, "old result should be evicted")
	_, ok = s.GetPinned("$2")
	assert.True(t, ok, "new result should exist")
}

func TestSession_ResolveRef_FullObject(t *testing.T) {
	s := newSession("test")
	s.PinResult("tool", `{"id":42,"name":"test"}`)

	val, err := s.ResolveRef("$1")
	require.NoError(t, err)
	m, ok := val.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(42), m["id"])
}

func TestSession_ResolveRef_DotPath(t *testing.T) {
	s := newSession("test")
	s.PinResult("tool", `{"user":{"login":"daltoniam","id":123}}`)

	val, err := s.ResolveRef("$1.user.login")
	require.NoError(t, err)
	assert.Equal(t, "daltoniam", val)

	val, err = s.ResolveRef("$1.user.id")
	require.NoError(t, err)
	assert.Equal(t, float64(123), val)
}

func TestSession_ResolveRef_ArrayIndex(t *testing.T) {
	s := newSession("test")
	s.PinResult("tool", `[{"id":1},{"id":2}]`)

	val, err := s.ResolveRef("$1.0.id")
	require.NoError(t, err)
	assert.Equal(t, float64(1), val)

	val, err = s.ResolveRef("$1.1.id")
	require.NoError(t, err)
	assert.Equal(t, float64(2), val)
}

func TestSession_ResolveRef_NotFound(t *testing.T) {
	s := newSession("test")
	_, err := s.ResolveRef("$99")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pinned result")
}

func TestSession_ResolveRef_BadPath(t *testing.T) {
	s := newSession("test")
	s.PinResult("tool", `{"id":1}`)

	_, err := s.ResolveRef("$1.nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSplitRef(t *testing.T) {
	tests := []struct {
		input      string
		wantHandle string
		wantPath   string
	}{
		{"$1", "$1", ""},
		{"$1.id", "$1", "id"},
		{"$1.user.login", "$1", "user.login"},
		{"$12.items.0.name", "$12", "items.0.name"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, p := splitRef(tt.input)
			assert.Equal(t, tt.wantHandle, h)
			assert.Equal(t, tt.wantPath, p)
		})
	}
}

func TestExtractPath(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{
			"b": []any{
				map[string]any{"c": "found"},
			},
		},
	}
	val, err := extractPath(data, "a.b.0.c")
	require.NoError(t, err)
	assert.Equal(t, "found", val)
}

func TestExtractPath_Errors(t *testing.T) {
	data := map[string]any{"x": "leaf"}

	_, err := extractPath(data, "missing")
	assert.Error(t, err)

	_, err = extractPath(data, "x.deeper")
	assert.Error(t, err)
}
