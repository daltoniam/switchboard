package gmail

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"gmail_list_messages":             {"messages[].id", "messages[].threadId", "resultSizeEstimate", "nextPageToken"},
	"gmail_list_threads":              {"threads[].id", "threads[].historyId", "threads[].snippet", "resultSizeEstimate", "nextPageToken"},
	"gmail_list_labels":               {"labels[].id", "labels[].name", "labels[].type", "labels[].messagesTotal", "labels[].messagesUnread", "labels[].threadsTotal", "labels[].threadsUnread"},
	"gmail_list_drafts":               {"drafts[].id", "drafts[].message.id", "drafts[].message.threadId", "resultSizeEstimate", "nextPageToken"},
	"gmail_list_history":              {"history[].id", "history[].messages[].id", "historyId", "nextPageToken"},
	"gmail_list_filters":              {"filter[].id", "filter[].criteria.from", "filter[].criteria.to", "filter[].criteria.subject", "filter[].criteria.query", "filter[].action.addLabelIds", "filter[].action.removeLabelIds"},
	"gmail_list_forwarding_addresses": {"forwardingAddresses[].forwardingEmail", "forwardingAddresses[].verificationStatus"},
	"gmail_list_send_as":              {"sendAs[].sendAsEmail", "sendAs[].displayName", "sendAs[].isPrimary", "sendAs[].isDefault", "sendAs[].verificationStatus"},
	"gmail_list_delegates":            {"delegates[].delegateEmail", "delegates[].verificationStatus"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("gmail: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
