package gmail

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("gmail_list_messages"):             {"messages[].id", "messages[].threadId", "resultSizeEstimate", "nextPageToken"},
	mcp.ToolName("gmail_list_threads"):              {"threads[].id", "threads[].historyId", "threads[].snippet", "resultSizeEstimate", "nextPageToken"},
	mcp.ToolName("gmail_list_labels"):               {"labels[].id", "labels[].name", "labels[].type", "labels[].messagesTotal", "labels[].messagesUnread", "labels[].threadsTotal", "labels[].threadsUnread"},
	mcp.ToolName("gmail_list_drafts"):               {"drafts[].id", "drafts[].message.id", "drafts[].message.threadId", "resultSizeEstimate", "nextPageToken"},
	mcp.ToolName("gmail_list_history"):              {"history[].id", "history[].messages[].id", "historyId", "nextPageToken"},
	mcp.ToolName("gmail_list_filters"):              {"filter[].id", "filter[].criteria.from", "filter[].criteria.to", "filter[].criteria.subject", "filter[].criteria.query", "filter[].action.addLabelIds", "filter[].action.removeLabelIds"},
	mcp.ToolName("gmail_list_forwarding_addresses"): {"forwardingAddresses[].forwardingEmail", "forwardingAddresses[].verificationStatus"},
	mcp.ToolName("gmail_list_send_as"):              {"sendAs[].sendAsEmail", "sendAs[].displayName", "sendAs[].isPrimary", "sendAs[].isDefault", "sendAs[].verificationStatus"},
	mcp.ToolName("gmail_list_delegates"):            {"delegates[].delegateEmail", "delegates[].verificationStatus"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("gmail: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
