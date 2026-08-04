package server

import (
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeSession_RoundTrip(t *testing.T) {
	s := newSession("roundtrip")
	s.SetContext(map[string]any{"owner": "daltoniam", "repo": "switchboard"})
	s.AddBreadcrumb("github_list_issues", map[string]any{"state": "open"}, `[{"n":1}]`, false)
	handle := s.PinResult("github_list_issues", `{"items":[1,2,3]}`)
	assert.Equal(t, "$1", handle)

	raw, err := EncodeSession(s)
	require.NoError(t, err)

	got, err := DecodeSession("roundtrip", raw)
	require.NoError(t, err)
	assert.Equal(t, "roundtrip", got.ID)
	assert.Equal(t, "daltoniam", got.GetContext()["owner"])
	assert.Equal(t, 1, got.TotalBreadcrumbs())
	assert.Equal(t, 1, got.PinnedCount())

	pr, ok := got.GetPinned("$1")
	require.True(t, ok)
	assert.Equal(t, mcp.ToolName("github_list_issues"), pr.Tool)
	assert.JSONEq(t, `{"items":[1,2,3]}`, string(pr.Data))

	// Store key wins over payload id.
	renamed, err := DecodeSession("other-id", raw)
	require.NoError(t, err)
	assert.Equal(t, "other-id", renamed.ID)
}

func TestEncodeSession_Nil(t *testing.T) {
	_, err := EncodeSession(nil)
	require.Error(t, err)
}

func TestDecodeSession_Empty(t *testing.T) {
	_, err := DecodeSession("x", nil)
	require.Error(t, err)
	_, err = DecodeSession("x", []byte{})
	require.Error(t, err)
}

func TestDecodeSession_PreservesTimestamps(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	s := &Session{
		ID:        "ts",
		Context:   map[string]any{},
		CreatedAt: created,
		LastUsed:  created.Add(time.Hour),
	}
	raw, err := EncodeSession(s)
	require.NoError(t, err)
	got, err := DecodeSession("ts", raw)
	require.NoError(t, err)
	assert.True(t, got.CreatedAt.Equal(created))
	assert.True(t, got.LastUsed.Equal(created.Add(time.Hour)))
}

func TestDecodeSession_RecoversNextHandle(t *testing.T) {
	// Snapshot with pins but next_handle omitted (0).
	raw := []byte(`{
		"id":"h",
		"context":{},
		"created_at":"2026-01-01T00:00:00Z",
		"last_used":"2026-01-01T00:00:00Z",
		"pinned":{"$3":{"handle":"$3","tool":"t","data":{"x":1},"pinned_at":"2026-01-01T00:00:00Z","size_bytes":7}},
		"next_handle":0,
		"pinned_size":0
	}`)
	got, err := DecodeSession("h", raw)
	require.NoError(t, err)
	assert.Equal(t, 3, got.nextHandle)
	handle := got.PinResult("t2", `{"y":2}`)
	assert.Equal(t, "$4", handle)
	_, ok := got.GetPinned("$3")
	assert.True(t, ok)
}
