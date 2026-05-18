package gtasks

import (
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Parity ──────────────────────────────────────────────────────────

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	// Every tool that returns document-style data must have a renderer
	// registered. Tools that return raw JSON envelopes (create/update/move/
	// delete/clear) intentionally have no markdown renderer.
	wantRendered := map[mcp.ToolName]bool{
		"gtasks_list_tasklists": true,
		"gtasks_list_tasks":     true,
		"gtasks_get_task":       true,
	}
	for name := range wantRendered {
		_, ok := markdownRenderers[name]
		assert.True(t, ok, "tool %s must have a markdown renderer", name)
	}
	// No orphan renderers.
	for name := range markdownRenderers {
		_, ok := wantRendered[name]
		assert.True(t, ok, "renderer %s has no corresponding intended tool", name)
	}
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gtasks{}
	_, ok := g.RenderMarkdown("gtasks_create_task", []byte(`{}`))
	assert.False(t, ok)
}

// ── List tasklists ──────────────────────────────────────────────────

func TestRenderTasklistsMD_Basic(t *testing.T) {
	in := []byte(`{
        "items": [
            {"id": "tl-1", "title": "My Tasks",  "updated": "2024-05-01T00:00:00Z"},
            {"id": "tl-2", "title": "Groceries", "updated": "2024-05-02T00:00:00Z"}
        ]
    }`)
	md, ok := renderTasklistsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Tasklists")
	assert.Contains(t, s, "My Tasks")
	assert.Contains(t, s, "tl-1")
	assert.Contains(t, s, "Groceries")
}

func TestRenderTasklistsMD_Empty(t *testing.T) {
	md, ok := renderTasklistsMD([]byte(`{"items":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No tasklists._")
}

func TestRenderTasklistsMD_WithPageToken(t *testing.T) {
	md, ok := renderTasklistsMD([]byte(`{"items":[{"id":"tl-1","title":"X"}],"nextPageToken":"abc"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "next_page_token: abc")
}

func TestRenderTasklistsMD_InvalidJSON(t *testing.T) {
	_, ok := renderTasklistsMD([]byte(`not json`))
	assert.False(t, ok)
}

func TestRenderTasklistsMD_WrongShape(t *testing.T) {
	// Neither items nor nextPageToken keys present → refuse to render.
	_, ok := renderTasklistsMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── List tasks ──────────────────────────────────────────────────────

func TestRenderTasksMD_BasicHierarchy(t *testing.T) {
	in := []byte(`{
        "items": [
            {"id": "t-1", "title": "Buy milk",     "status": "needsAction", "position": "001"},
            {"id": "t-2", "title": "Walk dog",     "status": "completed",   "position": "002"},
            {"id": "t-3", "title": "Subtask A",    "status": "needsAction", "parent": "t-1", "position": "001"},
            {"id": "t-4", "title": "Subtask B",    "status": "needsAction", "parent": "t-1", "position": "002"}
        ]
    }`)
	md, ok := renderTasksMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Tasks")
	assert.Contains(t, s, "- [ ] Buy milk")
	assert.Contains(t, s, "- [x] Walk dog")
	// Subtasks indented under their parent.
	assert.Contains(t, s, "  - [ ] Subtask A")
	assert.Contains(t, s, "  - [ ] Subtask B")
	// Subtask A should appear before B (position-sorted).
	idxA := strings.Index(s, "Subtask A")
	idxB := strings.Index(s, "Subtask B")
	assert.True(t, idxA > 0 && idxB > 0 && idxA < idxB, "subtask A should precede subtask B")
	// And Buy milk's subtasks should appear before Walk dog at top level.
	idxParent := strings.Index(s, "Buy milk")
	idxWalk := strings.Index(s, "Walk dog")
	assert.True(t, idxParent < idxA && idxA < idxWalk, "hierarchy ordering: parent → subtasks → next parent")
}

func TestRenderTasksMD_DueAndID(t *testing.T) {
	in := []byte(`{"items":[{"id":"t-1","title":"X","status":"needsAction","due":"2024-07-04T00:00:00Z"}]}`)
	md, ok := renderTasksMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "due 2024-07-04T00:00:00Z")
	assert.Contains(t, s, "id=t-1")
}

func TestRenderTasksMD_Notes(t *testing.T) {
	in := []byte(`{"items":[{"id":"t-1","title":"X","status":"needsAction","notes":"line one\nline two"}]}`)
	md, ok := renderTasksMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "> line one")
	assert.Contains(t, s, "> line two")
}

func TestRenderTasksMD_Links(t *testing.T) {
	in := []byte(`{"items":[{"id":"t-1","title":"X","status":"needsAction","links":[{"type":"email","description":"Original message","link":"https://mail.example/abc"}]}]}`)
	md, ok := renderTasksMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "🔗 [Original message](https://mail.example/abc)")
}

func TestRenderTasksMD_UntitledTask(t *testing.T) {
	md, ok := renderTasksMD([]byte(`{"items":[{"id":"t-1","title":"","status":"needsAction"}]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "(untitled)")
}

func TestRenderTasksMD_Empty(t *testing.T) {
	md, ok := renderTasksMD([]byte(`{"items":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No tasks._")
}

func TestRenderTasksMD_WithPageToken(t *testing.T) {
	md, ok := renderTasksMD([]byte(`{"items":[{"id":"t-1","title":"X","status":"needsAction"}],"nextPageToken":"abc"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "next_page_token: abc")
}

func TestRenderTasksMD_OrphanSubtask(t *testing.T) {
	// Parent task isn't in the page (paginated boundary). The orphan should
	// still be rendered at top level rather than dropped.
	in := []byte(`{"items":[{"id":"t-orphan","title":"Orphan","status":"needsAction","parent":"t-missing","position":"001"}]}`)
	md, ok := renderTasksMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "Orphan")
}

func TestRenderTasksMD_InvalidJSON(t *testing.T) {
	_, ok := renderTasksMD([]byte(`not json`))
	assert.False(t, ok)
}

func TestRenderTasksMD_WrongShape(t *testing.T) {
	_, ok := renderTasksMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Get task ────────────────────────────────────────────────────────

func TestRenderTaskMD_Basic(t *testing.T) {
	in := []byte(`{
        "id":     "t-1",
        "title":  "Buy milk",
        "notes":  "From the corner store",
        "status": "needsAction",
        "due":    "2024-07-04T00:00:00Z"
    }`)
	md, ok := renderTaskMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Buy milk")
	assert.Contains(t, s, "status: needsAction")
	assert.Contains(t, s, "due: 2024-07-04T00:00:00Z")
	assert.Contains(t, s, "id=t-1")
	assert.Contains(t, s, "From the corner store")
}

func TestRenderTaskMD_Completed(t *testing.T) {
	in := []byte(`{"id":"t-1","title":"Done","status":"completed","completed":"2024-07-04T12:00:00Z"}`)
	md, ok := renderTaskMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "status: completed")
	assert.Contains(t, s, "completed: 2024-07-04T12:00:00Z")
}

func TestRenderTaskMD_WithParent(t *testing.T) {
	in := []byte(`{"id":"t-1","title":"Sub","status":"needsAction","parent":"p-1"}`)
	md, ok := renderTaskMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "parent: p-1")
}

func TestRenderTaskMD_WithLinks(t *testing.T) {
	in := []byte(`{"id":"t-1","title":"X","status":"needsAction","links":[{"type":"email","description":"Source email","link":"https://mail.example/1"}]}`)
	md, ok := renderTaskMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "## Links")
	assert.Contains(t, s, "[Source email](https://mail.example/1)")
	assert.Contains(t, s, "_(email)_")
}

func TestRenderTaskMD_UntitledTask(t *testing.T) {
	md, ok := renderTaskMD([]byte(`{"id":"t-1","status":"needsAction"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "# (untitled)")
}

func TestRenderTaskMD_MissingID(t *testing.T) {
	_, ok := renderTaskMD([]byte(`{"title":"X"}`))
	assert.False(t, ok)
}

func TestRenderTaskMD_InvalidJSON(t *testing.T) {
	_, ok := renderTaskMD([]byte(`not json`))
	assert.False(t, ok)
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestPipeSafe(t *testing.T) {
	assert.Equal(t, "no special chars", pipeSafe("no special chars"))
	assert.Equal(t, "a b c", pipeSafe("a\nb\nc"))
	assert.Equal(t, `a\|b`, pipeSafe("a|b"))
}
