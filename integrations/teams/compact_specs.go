package teams

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// rawFieldCompactionSpecs declares which response fields to retain for the
// list/get tools that benefit from compaction. The Microsoft Graph response
// envelope is { "value": [...] } for list endpoints; the columnarizer treats
// "value[]" as the array root.
var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("teams_list_chats"): {
		"value[].id",
		"value[].topic",
		"value[].chatType",
		"value[].createdDateTime",
		"value[].lastUpdatedDateTime",
		"value[].webUrl",
		"value[].members[].displayName",
		"value[].lastMessagePreview.body.content",
		"@odata.nextLink",
	},
	mcp.ToolName("teams_list_chat_messages"): {
		"value[].id",
		"value[].chatId",
		"value[].createdDateTime",
		"value[].lastModifiedDateTime",
		"value[].from.user.id",
		"value[].from.user.displayName",
		"value[].subject",
		"value[].body.contentType",
		"value[].body.content",
		"value[].messageType",
		"value[].importance",
		"@odata.nextLink",
	},
	mcp.ToolName("teams_list_chat_members"): {
		"value[].id",
		"value[].displayName",
		"value[].roles",
		"value[].userId",
		"value[].email",
	},
	mcp.ToolName("teams_list_joined_teams"): {
		"value[].id",
		"value[].displayName",
		"value[].description",
		"value[].visibility",
		"value[].webUrl",
	},
	mcp.ToolName("teams_list_channels"): {
		"value[].id",
		"value[].displayName",
		"value[].description",
		"value[].membershipType",
		"value[].webUrl",
	},
	mcp.ToolName("teams_list_channel_messages"): {
		"value[].id",
		"value[].createdDateTime",
		"value[].from.user.id",
		"value[].from.user.displayName",
		"value[].subject",
		"value[].body.contentType",
		"value[].body.content",
		"value[].importance",
		"@odata.nextLink",
	},
	mcp.ToolName("teams_list_message_replies"): {
		"value[].id",
		"value[].createdDateTime",
		"value[].from.user.id",
		"value[].from.user.displayName",
		"value[].body.contentType",
		"value[].body.content",
		"@odata.nextLink",
	},
	mcp.ToolName("teams_list_users"): {
		"value[].id",
		"value[].displayName",
		"value[].userPrincipalName",
		"value[].mail",
		"value[].jobTitle",
		"value[].officeLocation",
		"@odata.nextLink",
	},
	mcp.ToolName("teams_search_users"): {
		"value[].id",
		"value[].displayName",
		"value[].userPrincipalName",
		"value[].mail",
		"value[].jobTitle",
		"@odata.nextLink",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("teams: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
