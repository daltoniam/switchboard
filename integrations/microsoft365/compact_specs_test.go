package microsoft365

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	m := &m365{}
	fields, ok := m.CompactSpec("microsoft365_list_messages")
	require.True(t, ok)
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	m := &m365{}
	_, ok := m.CompactSpec("microsoft365_nonexistent")
	assert.False(t, ok)
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"microsoft365_get_me":                `{"id":"u1","displayName":"Ada","givenName":"Ada","surname":"Lovelace","mail":"ada@contoso.com","userPrincipalName":"ada@contoso.com","jobTitle":"Engineer","officeLocation":"London","mobilePhone":"+1","businessPhones":["+2"]}`,
		"microsoft365_list_users":            `{"value":[{"id":"u1","displayName":"Ada","mail":"ada@contoso.com","userPrincipalName":"ada@contoso.com","jobTitle":"Engineer"}],"next_link":"https://graph.microsoft.com/v1.0/users?$skiptoken=a"}`,
		"microsoft365_search_people":         `{"value":[{"id":"p1","displayName":"Ada","scoredEmailAddresses":[{"address":"ada@contoso.com"}],"jobTitle":"Engineer","companyName":"Contoso"}],"next_link":"n"}`,
		"microsoft365_list_messages":         `{"value":[{"id":"m1","subject":"Hi","from":{"emailAddress":{"address":"ada@contoso.com","name":"Ada"}},"receivedDateTime":"2024-01-01T00:00:00Z","isRead":false,"hasAttachments":false,"conversationId":"c1","bodyPreview":"preview"}],"next_link":"n"}`,
		"microsoft365_get_message":           `{"id":"m1","subject":"Hi","from":{"emailAddress":{"address":"ada@contoso.com","name":"Ada"}},"toRecipients":[{"emailAddress":{"address":"g@c.com"}}],"receivedDateTime":"2024-01-01T00:00:00Z","sentDateTime":"2024-01-01T00:00:00Z","isRead":true,"hasAttachments":false,"conversationId":"c1","webLink":"https://outlook.office.com/m1","body":{"contentType":"text","content":"hello"},"bodyPreview":"hello"}`,
		"microsoft365_list_mail_folders":     `{"value":[{"id":"inbox","displayName":"Inbox","parentFolderId":"root","totalItemCount":10,"unreadItemCount":2}],"next_link":"n"}`,
		"microsoft365_list_calendars":        `{"value":[{"id":"cal1","name":"Calendar","isDefaultCalendar":true,"canEdit":true,"owner":{"address":"ada@contoso.com"}}],"next_link":"n"}`,
		"microsoft365_list_events":           `{"value":[{"id":"e1","subject":"Standup","start":{"dateTime":"2024-01-01T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2024-01-01T09:30:00","timeZone":"UTC"},"location":{"displayName":"Teams"},"organizer":{"emailAddress":{"address":"ada@contoso.com"}},"isOnlineMeeting":true,"onlineMeeting":{"joinUrl":"https://teams.microsoft.com/l/meetup"},"webLink":"https://outlook.office.com/e1"}],"next_link":"n"}`,
		"microsoft365_get_event":             `{"id":"e1","subject":"Standup","bodyPreview":"daily","start":{"dateTime":"2024-01-01T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2024-01-01T09:30:00","timeZone":"UTC"},"location":{"displayName":"Teams"},"attendees":[{"emailAddress":{"address":"g@c.com"},"status":{"response":"accepted"}}],"organizer":{"emailAddress":{"address":"ada@contoso.com"}},"isOnlineMeeting":true,"onlineMeeting":{"joinUrl":"https://teams.microsoft.com/l/meetup"},"webLink":"https://outlook.office.com/e1","showAs":"busy","importance":"normal"}`,
		"microsoft365_list_drive_items":      `{"value":[{"id":"f1","name":"Notes.txt","size":12,"webUrl":"https://onedrive","lastModifiedDateTime":"2024-01-01T00:00:00Z","file":{"mimeType":"text/plain"},"folder":{"childCount":0},"parentReference":{"path":"/drive/root:"}}],"next_link":"n"}`,
		"microsoft365_get_drive_item":        `{"id":"f1","name":"Notes.txt","size":12,"webUrl":"https://onedrive","createdDateTime":"2024-01-01T00:00:00Z","lastModifiedDateTime":"2024-01-01T00:00:00Z","file":{"mimeType":"text/plain"},"folder":{"childCount":0},"parentReference":{"path":"/drive/root:"},"createdBy":{"user":{"displayName":"Ada"}},"lastModifiedBy":{"user":{"displayName":"Ada"}}}`,
		"microsoft365_search_drive":          `{"value":[{"id":"f1","name":"budget.xlsx","size":100,"webUrl":"https://onedrive","lastModifiedDateTime":"2024-01-01T00:00:00Z","file":{"mimeType":"application/vnd.ms-excel"},"folder":{"childCount":0}}],"next_link":"n"}`,
		"microsoft365_download_drive_item":   `{"content_type":"text/plain","bytes":5,"truncated":false,"content":"hello"}`,
		"microsoft365_list_teams":            `{"value":[{"id":"t1","displayName":"Eng","description":"team","isArchived":false}],"next_link":"n"}`,
		"microsoft365_list_channels":         `{"value":[{"id":"c1","displayName":"General","description":"","membershipType":"standard","webUrl":"https://teams"}],"next_link":"n"}`,
		"microsoft365_list_channel_messages": `{"value":[{"id":"msg1","createdDateTime":"2024-01-01T00:00:00Z","from":{"user":{"displayName":"Ada"}},"body":{"content":"hi"},"webUrl":"https://teams"}],"next_link":"n"}`,
		"microsoft365_list_chats":            `{"value":[{"id":"ch1","topic":"Ada","chatType":"oneOnOne","createdDateTime":"2024-01-01T00:00:00Z","lastUpdatedDateTime":"2024-01-01T00:00:00Z"}],"next_link":"n"}`,
		"microsoft365_list_chat_messages":    `{"value":[{"id":"msg1","createdDateTime":"2024-01-01T00:00:00Z","from":{"user":{"displayName":"Ada"}},"body":{"content":"hi"}}],"next_link":"n"}`,
		"microsoft365_list_todo_lists":       `{"value":[{"id":"l1","displayName":"Tasks","isOwner":true,"isShared":false,"wellknownListName":"defaultList"}],"next_link":"n"}`,
		"microsoft365_list_todo_tasks":       `{"value":[{"id":"task1","title":"Ship","status":"notStarted","importance":"normal","dueDateTime":{"dateTime":"2024-01-02T00:00:00"},"completedDateTime":{"dateTime":""},"createdDateTime":"2024-01-01T00:00:00Z"}],"next_link":"n"}`,
	}

	for toolName, payload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok, "missing compaction spec for %s", toolName)
			compacted, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			assert.NotEqual(t, "{}", string(compacted), "compaction returned empty object for %s: %s", toolName, compacted)
			assert.NotEqual(t, "[]", string(compacted), "compaction returned empty array for %s", toolName)
			assert.NotEqual(t, "[{}]", string(compacted), "compaction returned array of empty objects for %s: %s", toolName, compacted)
		})
	}
}
