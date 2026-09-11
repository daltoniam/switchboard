package front

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
	f := &front{}
	fields, ok := f.CompactSpec("front_search_conversations")
	require.True(t, ok, "front_search_conversations should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	f := &front{}
	_, ok := f.CompactSpec("front_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"front_search_conversations":       `{"_total":1,"_pagination":{"next":"https://api2.frontapp.com/conversations/search/x?page_token=n1"},"_results":[{"id":"cnv_1","type":"conversation","subject":"Help","status":"unassigned","status_id":"sts_1","status_category":"open","ticket_ids":["T-1"],"assignee":{"id":"tea_1","email":"a@b.com","username":"ada"},"recipient":{"handle":"user@example.com","name":"User"},"tags":[{"id":"tag_1","name":"billing"}],"created_at":1701292649.333,"is_private":false}]}`,
		"front_list_conversations":         `{"_pagination":{"next":null},"_results":[{"id":"cnv_1","type":"conversation","subject":"Help","status":"assigned","status_id":"sts_1","status_category":"open","ticket_ids":["T-1"],"assignee":{"id":"tea_1","email":"a@b.com","username":"ada"},"recipient":{"handle":"user@example.com","name":"User"},"tags":[{"id":"tag_1","name":"billing"}],"created_at":1701292649.333,"is_private":false}]}`,
		"front_get_conversation":           `{"id":"cnv_1","type":"conversation","subject":"Help","status":"assigned","status_id":"sts_1","status_category":"open","ticket_ids":["T-1"],"assignee":{"id":"tea_1","email":"a@b.com","username":"ada","first_name":"Ada","last_name":"Lovelace"},"recipient":{"handle":"user@example.com","name":"User","role":"from"},"tags":[{"id":"tag_1","name":"billing"}],"created_at":1701292649.333,"is_private":false,"scheduled_reminders":[]}`,
		"front_list_conversation_messages": `{"_pagination":{"next":null},"_results":[{"id":"msg_1","type":"email","is_inbound":true,"created_at":1701292649,"subject":"Help","blurb":"I need help","text":"I need help","author":{"id":"tea_1","email":"a@b.com","username":"ada"},"recipients":[{"handle":"user@example.com","role":"from"}],"attachments":[{"id":"fil_1","filename":"a.png"}]}]}`,
		"front_list_conversation_comments": `{"_pagination":{"next":null},"_results":[{"id":"com_1","body":"note","posted_at":1701292649.378,"is_pinned":false,"author":{"id":"tea_1","email":"a@b.com","username":"ada"}}]}`,
		"front_list_inboxes":               `{"_pagination":{"next":null},"_results":[{"id":"inb_1","name":"Support","is_private":false,"address":"support@acme.com","send_as":"support@acme.com","type":"team"}]}`,
		"front_list_teammates":             `{"_pagination":{"next":null},"_results":[{"id":"tea_1","email":"a@b.com","username":"ada","first_name":"Ada","last_name":"Lovelace","is_admin":true,"is_available":true,"is_blocked":false,"type":"user"}]}`,
		"front_list_tags":                  `{"_pagination":{"next":null},"_results":[{"id":"tag_1","name":"billing","description":"invoices","highlight":"blue","is_private":false,"is_visible_in_conversation_lists":true}]}`,
		"front_list_channels":              `{"_pagination":{"next":null},"_results":[{"id":"cha_1","name":"Support email","address":"support@acme.com","type":"smtp","send_as":"support@acme.com","is_private":false}]}`,
		"front_list_contacts":              `{"_pagination":{"next":null},"_results":[{"id":"crd_1","name":"Ada","description":"customer","handles":[{"handle":"ada@example.com","source":"email"}],"is_private":false}]}`,
		"front_search_contacts":            `{"id":"crd_1","name":"Ada","description":"customer","handles":[{"handle":"ada@example.com","source":"email"}],"is_private":false,"account_names":["Acme"]}`,
		"front_get_contact":                `{"id":"crd_1","name":"Ada","description":"customer","handles":[{"handle":"ada@example.com","source":"email"}],"is_private":false,"account_names":["Acme"],"links":[]}`,
		"front_list_accounts":              `{"_pagination":{"next":null},"_results":[{"id":"acc_1","name":"Acme","description":"customer","domains":["acme.com"],"external_id":"ext-1"}]}`,
		"front_get_account":                `{"id":"acc_1","name":"Acme","description":"customer","domains":["acme.com"],"external_id":"ext-1"}`,
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
