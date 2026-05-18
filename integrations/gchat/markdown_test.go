package gchat

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Parity ──────────────────────────────────────────────────────────

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	// Every tool that returns document-style data must have a renderer.
	// Tools that return raw JSON envelopes (create/update/delete) have
	// no markdown renderer.
	wantRendered := map[mcp.ToolName]bool{
		"gchat_list_spaces":   true,
		"gchat_get_space":     true,
		"gchat_list_messages": true,
		"gchat_get_message":   true,
		"gchat_list_members":  true,
	}
	for name := range wantRendered {
		_, ok := markdownRenderers[name]
		assert.True(t, ok, "tool %s must have a markdown renderer", name)
	}
	for name := range markdownRenderers {
		_, ok := wantRendered[name]
		assert.True(t, ok, "renderer %s has no corresponding intended tool", name)
	}
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gchat{}
	_, ok := g.RenderMarkdown("gchat_create_message", []byte(`{}`))
	assert.False(t, ok)
}

// ── List spaces ─────────────────────────────────────────────────────

func TestRenderSpacesMD_Basic(t *testing.T) {
	in := []byte(`{
        "spaces": [
            {"name":"spaces/A","displayName":"Engineering","spaceType":"SPACE"},
            {"name":"spaces/B","displayName":"","spaceType":"DIRECT_MESSAGE"}
        ]
    }`)
	md, ok := renderSpacesMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Chat Spaces")
	assert.Contains(t, s, "Engineering")
	assert.Contains(t, s, "spaces/A")
	assert.Contains(t, s, "DIRECT_MESSAGE")
	// Unnamed DM falls back to placeholder.
	assert.Contains(t, s, "(no name)")
}

func TestRenderSpacesMD_Empty(t *testing.T) {
	md, ok := renderSpacesMD([]byte(`{"spaces":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No spaces._")
}

func TestRenderSpacesMD_WithPageToken(t *testing.T) {
	md, ok := renderSpacesMD([]byte(`{"spaces":[{"name":"spaces/A","displayName":"X"}],"nextPageToken":"abc"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "next_page_token: abc")
}

func TestRenderSpacesMD_InvalidJSON(t *testing.T) {
	_, ok := renderSpacesMD([]byte(`not json`))
	assert.False(t, ok)
}

func TestRenderSpacesMD_WrongShape(t *testing.T) {
	_, ok := renderSpacesMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Single space ────────────────────────────────────────────────────

func TestRenderSpaceMD_Basic(t *testing.T) {
	in := []byte(`{
        "name":"spaces/A",
        "displayName":"Engineering",
        "spaceType":"SPACE",
        "spaceThreadingState":"THREADED_MESSAGES",
        "spaceHistoryState":"HISTORY_ON",
        "spaceDetails":{"description":"Engineering team chat","guidelines":"Be excellent."}
    }`)
	md, ok := renderSpaceMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Engineering")
	assert.Contains(t, s, "type: SPACE")
	assert.Contains(t, s, "threading: THREADED_MESSAGES")
	assert.Contains(t, s, "history: HISTORY_ON")
	assert.Contains(t, s, "name=spaces/A")
	assert.Contains(t, s, "## Description")
	assert.Contains(t, s, "Engineering team chat")
	assert.Contains(t, s, "## Guidelines")
	assert.Contains(t, s, "Be excellent.")
}

func TestRenderSpaceMD_UnnamedFallback(t *testing.T) {
	md, ok := renderSpaceMD([]byte(`{"name":"spaces/X","spaceType":"DIRECT_MESSAGE"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "# (unnamed space)")
}

func TestRenderSpaceMD_MissingName(t *testing.T) {
	_, ok := renderSpaceMD([]byte(`{"displayName":"X"}`))
	assert.False(t, ok)
}

func TestRenderSpaceMD_InvalidJSON(t *testing.T) {
	_, ok := renderSpaceMD([]byte(`not json`))
	assert.False(t, ok)
}

// ── List messages ───────────────────────────────────────────────────

func TestRenderMessagesMD_Basic(t *testing.T) {
	in := []byte(`{
        "messages": [
            {
                "name":"spaces/A/messages/m1",
                "sender":{"name":"users/u1","displayName":"Alice","type":"HUMAN"},
                "createTime":"2024-05-01T10:00:00Z",
                "text":"Hello world"
            },
            {
                "name":"spaces/A/messages/m2",
                "sender":{"name":"users/u2","displayName":"Bob","type":"HUMAN"},
                "createTime":"2024-05-01T10:01:00Z",
                "lastUpdateTime":"2024-05-01T10:02:00Z",
                "text":"Edited reply"
            }
        ]
    }`)
	md, ok := renderMessagesMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Chat Messages")
	assert.Contains(t, s, "**Alice**")
	assert.Contains(t, s, "Hello world")
	assert.Contains(t, s, "**Bob**")
	assert.Contains(t, s, "Edited reply")
	// Edited timestamp surfaced.
	assert.Contains(t, s, "edited 2024-05-01T10:02:00Z")
}

func TestRenderMessagesMD_Deleted(t *testing.T) {
	in := []byte(`{"messages":[{"name":"spaces/A/messages/m1","sender":{"displayName":"Alice"},"createTime":"2024-05-01T10:00:00Z","deleteTime":"2024-05-02T10:00:00Z","text":""}]}`)
	md, ok := renderMessagesMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "_[deleted]_")
}

func TestRenderMessagesMD_NoTextNoDelete(t *testing.T) {
	in := []byte(`{"messages":[{"name":"spaces/A/messages/m1","sender":{"displayName":"Alice"},"createTime":"2024-05-01T10:00:00Z"}]}`)
	md, ok := renderMessagesMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "_(no text)_")
}

func TestRenderMessagesMD_UnknownSender(t *testing.T) {
	in := []byte(`{"messages":[{"name":"spaces/A/messages/m1","createTime":"2024-05-01T10:00:00Z","text":"hi"}]}`)
	md, ok := renderMessagesMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "**(unknown)**")
}

func TestRenderMessagesMD_Empty(t *testing.T) {
	md, ok := renderMessagesMD([]byte(`{"messages":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No messages._")
}

func TestRenderMessagesMD_WithPageToken(t *testing.T) {
	md, ok := renderMessagesMD([]byte(`{"messages":[{"name":"spaces/A/messages/m1","sender":{"displayName":"Alice"},"text":"hi"}],"nextPageToken":"abc"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "next_page_token: abc")
}

func TestRenderMessagesMD_InvalidJSON(t *testing.T) {
	_, ok := renderMessagesMD([]byte(`not json`))
	assert.False(t, ok)
}

func TestRenderMessagesMD_WrongShape(t *testing.T) {
	_, ok := renderMessagesMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Single message ──────────────────────────────────────────────────

func TestRenderMessageMD_Basic(t *testing.T) {
	in := []byte(`{
        "name":"spaces/A/messages/m1",
        "sender":{"name":"users/u1","displayName":"Alice","type":"HUMAN"},
        "createTime":"2024-05-01T10:00:00Z",
        "text":"Standup notes\n- Done X\n- Doing Y",
        "thread":{"name":"spaces/A/threads/t1"}
    }`)
	md, ok := renderMessageMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Message from Alice")
	assert.Contains(t, s, "created: 2024-05-01T10:00:00Z")
	assert.Contains(t, s, "thread: spaces/A/threads/t1")
	assert.Contains(t, s, "name=spaces/A/messages/m1")
	assert.Contains(t, s, "Standup notes")
	assert.Contains(t, s, "- Done X")
}

func TestRenderMessageMD_Edited(t *testing.T) {
	in := []byte(`{
        "name":"spaces/A/messages/m1",
        "sender":{"displayName":"Alice"},
        "createTime":"2024-05-01T10:00:00Z",
        "lastUpdateTime":"2024-05-01T11:00:00Z",
        "text":"edited"
    }`)
	md, ok := renderMessageMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "created: 2024-05-01T10:00:00Z")
	assert.Contains(t, s, "updated: 2024-05-01T11:00:00Z")
}

func TestRenderMessageMD_NoSenderName(t *testing.T) {
	in := []byte(`{"name":"spaces/A/messages/m1","text":"hi"}`)
	md, ok := renderMessageMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "# Message from (unknown sender)")
}

func TestRenderMessageMD_WithAttachments(t *testing.T) {
	in := []byte(`{"name":"spaces/A/messages/m1","sender":{"displayName":"Alice"},"text":"see attached","attachment":[{},{}]}`)
	md, ok := renderMessageMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "attachments: 2")
}

func TestRenderMessageMD_MissingName(t *testing.T) {
	_, ok := renderMessageMD([]byte(`{"text":"hi"}`))
	assert.False(t, ok)
}

func TestRenderMessageMD_InvalidJSON(t *testing.T) {
	_, ok := renderMessageMD([]byte(`not json`))
	assert.False(t, ok)
}

// ── List members ────────────────────────────────────────────────────

func TestRenderMembersMD_Basic(t *testing.T) {
	in := []byte(`{
        "memberships": [
            {"name":"spaces/A/members/u1","state":"JOINED","role":"ROLE_MANAGER","member":{"name":"users/u1","displayName":"Alice","type":"HUMAN"}},
            {"name":"spaces/A/members/u2","state":"JOINED","role":"ROLE_MEMBER","member":{"name":"users/u2","displayName":"Bot","type":"BOT"}}
        ]
    }`)
	md, ok := renderMembersMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Space Members")
	assert.Contains(t, s, "Alice")
	assert.Contains(t, s, "ROLE_MANAGER")
	assert.Contains(t, s, "Bot")
	assert.Contains(t, s, "BOT")
}

func TestRenderMembersMD_Empty(t *testing.T) {
	md, ok := renderMembersMD([]byte(`{"memberships":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No members._")
}

func TestRenderMembersMD_WithPageToken(t *testing.T) {
	md, ok := renderMembersMD([]byte(`{"memberships":[{"name":"spaces/A/members/u1","state":"JOINED","role":"ROLE_MEMBER","member":{"displayName":"X","type":"HUMAN"}}],"nextPageToken":"abc"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "next_page_token: abc")
}

func TestRenderMembersMD_InvalidJSON(t *testing.T) {
	_, ok := renderMembersMD([]byte(`not json`))
	assert.False(t, ok)
}

func TestRenderMembersMD_WrongShape(t *testing.T) {
	_, ok := renderMembersMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestPipeSafe(t *testing.T) {
	assert.Equal(t, "no special chars", pipeSafe("no special chars"))
	assert.Equal(t, "a b c", pipeSafe("a\nb\nc"))
	assert.Equal(t, `a\|b`, pipeSafe("a|b"))
}
