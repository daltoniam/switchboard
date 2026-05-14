package teams

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
)

// TestFieldCompactionSpecs_NoOrphanSpecs asserts every spec entry corresponds
// to a real tool definition so old specs can't silently linger after a rename.
func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	defs := make(map[mcp.ToolName]bool)
	for _, d := range New().Tools() {
		defs[d.Name] = true
	}
	for name := range rawFieldCompactionSpecs {
		assert.True(t, defs[name], "compact spec for unknown tool: %s", name)
	}
}

// TestFieldCompactionSpecs_AllListToolsCovered keeps every list/search tool
// honest about declaring a spec — guards against the "ship a list tool that
// dumps the whole envelope" regression.
func TestFieldCompactionSpecs_AllListToolsCovered(t *testing.T) {
	// Tools that intentionally don't need compaction (single-entity GETs,
	// auth flows, sends that return small confirmations).
	exempt := map[mcp.ToolName]bool{
		mcp.ToolName("teams_login"):                    true,
		mcp.ToolName("teams_login_poll"):               true,
		mcp.ToolName("teams_token_status"):             true,
		mcp.ToolName("teams_refresh_tokens"):           true,
		mcp.ToolName("teams_list_tenants"):             true,
		mcp.ToolName("teams_remove_tenant"):            true,
		mcp.ToolName("teams_set_default"):              true,
		mcp.ToolName("teams_get_me"):                   true,
		mcp.ToolName("teams_get_chat"):                 true,
		mcp.ToolName("teams_get_chat_message"):         true,
		mcp.ToolName("teams_send_chat_message"):        true,
		mcp.ToolName("teams_get_channel"):              true,
		mcp.ToolName("teams_get_channel_message"):      true,
		mcp.ToolName("teams_send_channel_message"):     true,
		mcp.ToolName("teams_reply_to_channel_message"): true,
		mcp.ToolName("teams_get_user"):                 true,
		mcp.ToolName("teams_get_presence"):             true,
	}
	for _, d := range New().Tools() {
		if exempt[d.Name] {
			continue
		}
		_, ok := rawFieldCompactionSpecs[d.Name]
		assert.True(t, ok, "tool %s should declare a field compaction spec (or be exempted)", d.Name)
	}
}
