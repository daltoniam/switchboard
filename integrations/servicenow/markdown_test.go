package servicenow

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	s := &servicenow{}
	_, ok := s.RenderMarkdown("servicenow_list_incidents", []byte(`{}`))
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

	for name := range toolNames {
		md.RenderMarkdown(name, []byte("{}"))
	}

	markdownTools := []mcp.ToolName{
		"servicenow_get_incident",
		"servicenow_get_problem",
		"servicenow_get_change_request",
		"servicenow_get_knowledge_article",
		"servicenow_list_comments",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}

func TestRenderIncidentMD(t *testing.T) {
	payload := []byte(`{"result":{"sys_id":"1","number":"INC0010001","short_description":"VPN down","state":"In Progress","priority":"1 - Critical","assigned_to":"Alice","assignment_group":"Network","description":"Users cannot connect","caller_id":"Bob"}}`)
	md, ok := renderIncidentMD(payload)
	require.True(t, ok)
	assert.Contains(t, string(md), "INC0010001: VPN down")
	assert.Contains(t, string(md), "State: In Progress")
	assert.Contains(t, string(md), "Users cannot connect")
	assert.Contains(t, string(md), "sys_id=1")
}

func TestRenderIncidentMD_HTMLDescription(t *testing.T) {
	payload := []byte(`{"result":{"sys_id":"1","number":"INC0010001","short_description":"VPN down","description":"<p>Users cannot connect</p>","close_notes":"<p>Restarted the gateway</p>"}}`)
	md, ok := renderIncidentMD(payload)
	require.True(t, ok)
	assert.Contains(t, string(md), "Users cannot connect")
	assert.Contains(t, string(md), "Restarted the gateway")
	assert.NotContains(t, string(md), "<p>")
}

func TestRenderIncidentMD_DisplayValueObject(t *testing.T) {
	payload := []byte(`{"result":{"sys_id":"1","number":"INC0010001","short_description":"VPN down","state":{"display_value":"New","value":"1"},"assigned_to":{"display_value":"Alice A","value":"u1"}}}`)
	md, ok := renderIncidentMD(payload)
	require.True(t, ok)
	assert.Contains(t, string(md), "State: New")
	assert.Contains(t, string(md), "Assigned: Alice A")
}

func TestRenderArticleMD_StripsHTML(t *testing.T) {
	payload := []byte(`{"result":{"sys_id":"k1","number":"KB001","short_description":"Reset VPN","text":"<p>Restart the <b>client</b></p>"}}`)
	md, ok := renderArticleMD(payload)
	require.True(t, ok)
	assert.Contains(t, string(md), "KB001: Reset VPN")
	assert.Contains(t, string(md), "Restart the **client**")
	assert.NotContains(t, string(md), "<p>")
}

func TestRenderCommentsMD_Empty(t *testing.T) {
	md, ok := renderCommentsMD([]byte(`{"result":[]}`))
	require.True(t, ok)
	assert.Equal(t, "No comments.\n", string(md))
}

func TestRenderCommentsMD(t *testing.T) {
	payload := []byte(`{"result":[{"sys_created_by":"alice","sys_created_on":"2026-01-01","element":"work_notes","value":"looking into it"}]}`)
	md, ok := renderCommentsMD(payload)
	require.True(t, ok)
	assert.Contains(t, string(md), "alice")
	assert.Contains(t, string(md), "looking into it")
}

func TestRenderCommentsMD_HTMLBody(t *testing.T) {
	payload := []byte(`{"result":[{"sys_created_by":"alice","sys_created_on":"2026-01-01","element":"work_notes","value":"<p>looking into it</p>"}]}`)
	md, ok := renderCommentsMD(payload)
	require.True(t, ok)
	assert.Contains(t, string(md), "looking into it")
	assert.NotContains(t, string(md), "<p>")
}
