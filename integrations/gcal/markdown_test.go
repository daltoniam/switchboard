package gcal

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_Event(t *testing.T) {
	g := &gcal{}
	data := `{
		"id": "ev1",
		"summary": "Quarterly Review",
		"status": "confirmed",
		"description": "Review Q1 outcomes.",
		"location": "Conf Room A",
		"htmlLink": "https://calendar.google.com/event?eid=abc",
		"hangoutLink": "https://meet.google.com/xyz-abcd-efg",
		"start": {"dateTime": "2024-03-15T14:00:00-07:00", "timeZone": "America/Los_Angeles"},
		"end": {"dateTime": "2024-03-15T15:00:00-07:00", "timeZone": "America/Los_Angeles"},
		"organizer": {"email": "boss@example.com", "displayName": "The Boss"},
		"attendees": [
			{"email": "alice@example.com", "displayName": "Alice", "responseStatus": "accepted"},
			{"email": "bob@example.com", "responseStatus": "needsAction", "optional": true}
		]
	}`

	md, ok := g.RenderMarkdown("gcal_get_event", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "<!-- gcal:event_id=ev1 status=confirmed -->")
	assert.Contains(t, s, "# Quarterly Review")
	assert.Contains(t, s, "When: 2024-03-15T14:00:00-07:00 (America/Los_Angeles)")
	assert.Contains(t, s, "Where: Conf Room A")
	assert.Contains(t, s, "The Boss <boss@example.com>")
	assert.Contains(t, s, "Meet: https://meet.google.com/xyz-abcd-efg")
	assert.Contains(t, s, "## Attendees")
	assert.Contains(t, s, "Alice <alice@example.com>")
	assert.Contains(t, s, "(accepted)")
	assert.Contains(t, s, "(optional)")
	assert.Contains(t, s, "## Description")
	assert.Contains(t, s, "Review Q1 outcomes.")
}

func TestRenderMarkdown_Event_AllDay(t *testing.T) {
	g := &gcal{}
	data := `{
		"id": "ev2",
		"summary": "Company Holiday",
		"status": "confirmed",
		"start": {"date": "2024-12-25"},
		"end": {"date": "2024-12-26"}
	}`

	md, ok := g.RenderMarkdown("gcal_get_event", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Company Holiday")
	assert.Contains(t, s, "2024-12-25 (all-day)")
}

func TestRenderMarkdown_Event_NoSummary(t *testing.T) {
	g := &gcal{}
	data := `{"id":"ev3","status":"confirmed"}`
	md, ok := g.RenderMarkdown("gcal_get_event", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "(no title)")
}

func TestRenderMarkdown_Event_ConferenceFallback(t *testing.T) {
	// No hangoutLink, but conferenceData has a video entry point.
	g := &gcal{}
	data := `{
		"id":"ev4",
		"summary":"Sync",
		"status":"confirmed",
		"conferenceData": {"entryPoints":[
			{"entryPointType":"phone","uri":"tel:+1-555-0100"},
			{"entryPointType":"video","uri":"https://meet.example.com/sync"}
		]}
	}`
	md, ok := g.RenderMarkdown("gcal_get_event", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "Meet: https://meet.example.com/sync")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gcal{}
	_, ok := g.RenderMarkdown("gcal_list_events", []byte(`{}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	g := &gcal{}
	_, ok := g.RenderMarkdown("gcal_get_event", []byte(`{bad`))
	assert.False(t, ok)
}

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	adapter := New()
	md, ok := adapter.(mcp.MarkdownIntegration)
	require.True(t, ok, "adapter should implement MarkdownIntegration")

	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range adapter.Tools() {
		toolNames[tool.Name] = true
	}

	// Call RenderMarkdown for every tool — should not panic
	for name := range toolNames {
		md.RenderMarkdown(name, []byte("{}"))
	}

	// Verify the tools RenderMarkdown claims to handle actually exist
	markdownTools := []mcp.ToolName{
		"gcal_get_event",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
