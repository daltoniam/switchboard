package zendesk

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
	z := &zendesk{}
	fields, ok := z.CompactSpec("zendesk_search_tickets")
	require.True(t, ok, "zendesk_search_tickets should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	z := &zendesk{}
	_, ok := z.CompactSpec("zendesk_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"zendesk_search_tickets":            `{"count":1,"next_page":null,"previous_page":null,"results":[{"id":1,"subject":"Login broken","status":"open","priority":"high","type":"incident","requester_id":10,"assignee_id":20,"group_id":30,"organization_id":40,"tags":["auth"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","url":"https://acme.zendesk.com/api/v2/tickets/1.json"}]}`,
		"zendesk_list_tickets":              `{"tickets":[{"id":1,"subject":"Login broken","status":"open","priority":"high","type":"incident","requester_id":10,"assignee_id":20,"group_id":30,"organization_id":40,"tags":["auth"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","url":"https://acme.zendesk.com/api/v2/tickets/1.json"}],"meta":{"has_more":false,"after_cursor":"a","before_cursor":"b"},"links":{"next":null}}`,
		"zendesk_get_ticket":                `{"ticket":{"id":1,"subject":"Login broken","description":"Cannot sign in","status":"open","priority":"high","type":"incident","requester_id":10,"assignee_id":20,"submitter_id":10,"group_id":30,"organization_id":40,"tags":["auth"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","due_at":null,"url":"https://acme.zendesk.com/api/v2/tickets/1.json","custom_fields":[{"id":100,"value":"web"}]},"users":[{"id":10,"name":"Ada","email":"ada@ex.com"}],"groups":[{"id":30,"name":"Support"}]}`,
		"zendesk_list_ticket_comments":      `{"comments":[{"id":5,"author_id":10,"body":"hello","public":true,"created_at":"2024-01-01T00:00:00Z","attachments":[{"file_name":"log.txt","content_url":"https://ex.com/log.txt"}]}],"meta":{"has_more":false,"after_cursor":"c"},"links":{"next":null}}`,
		"zendesk_list_ticket_audits":        `{"audits":[{"id":8,"ticket_id":1,"author_id":20,"created_at":"2024-01-01T00:00:00Z","events":[{"id":9,"type":"Change","field_name":"status","value":"open","previous_value":"new"}]}],"meta":{"has_more":false,"after_cursor":"d"},"links":{"next":null}}`,
		"zendesk_search":                    `{"count":1,"next_page":null,"previous_page":null,"results":[{"id":10,"result_type":"user","name":"Ada","email":"ada@ex.com","subject":"","status":"","priority":"","role":"end-user","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","url":"https://acme.zendesk.com/api/v2/users/10.json"}]}`,
		"zendesk_list_users":                `{"users":[{"id":10,"name":"Ada","email":"ada@ex.com","role":"end-user","active":true,"organization_id":40,"default_group_id":30,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}],"meta":{"has_more":false,"after_cursor":"e"},"links":{"next":null}}`,
		"zendesk_get_user":                  `{"user":{"id":10,"name":"Ada","email":"ada@ex.com","role":"end-user","active":true,"verified":true,"organization_id":40,"default_group_id":30,"phone":"+15551212","tags":["vip"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","url":"https://acme.zendesk.com/api/v2/users/10.json"}}`,
		"zendesk_get_current_user":          `{"user":{"id":20,"name":"Agent","email":"agent@acme.com","role":"agent","active":true,"organization_id":0,"default_group_id":30,"url":"https://acme.zendesk.com/api/v2/users/20.json"}}`,
		"zendesk_list_organizations":        `{"organizations":[{"id":40,"name":"Acme","domain_names":["acme.com"],"details":"customer","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}],"meta":{"has_more":false,"after_cursor":"f"},"links":{"next":null}}`,
		"zendesk_get_organization":          `{"organization":{"id":40,"name":"Acme","domain_names":["acme.com"],"details":"customer","notes":"vip","tags":["enterprise"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","url":"https://acme.zendesk.com/api/v2/organizations/40.json"}}`,
		"zendesk_list_groups":               `{"groups":[{"id":30,"name":"Support","description":"L1","default":true,"deleted":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}],"meta":{"has_more":false,"after_cursor":"g"},"links":{"next":null}}`,
		"zendesk_get_group":                 `{"group":{"id":30,"name":"Support","description":"L1","default":true,"deleted":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","url":"https://acme.zendesk.com/api/v2/groups/30.json"}}`,
		"zendesk_list_views":                `{"views":[{"id":50,"title":"Your unsolved tickets","active":true,"position":1,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}],"meta":{"has_more":false,"after_cursor":"h"},"links":{"next":null}}`,
		"zendesk_list_view_tickets":         `{"tickets":[{"id":1,"subject":"Login broken","status":"open","priority":"high","type":"incident","requester_id":10,"assignee_id":20,"group_id":30,"tags":["auth"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","url":"https://acme.zendesk.com/api/v2/tickets/1.json"}],"meta":{"has_more":false,"after_cursor":"i"},"links":{"next":null}}`,
		"zendesk_list_macros":               `{"macros":[{"id":60,"title":"Close and notify","active":true,"description":"solved","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}],"meta":{"has_more":false,"after_cursor":"j"},"links":{"next":null}}`,
		"zendesk_search_articles":           `{"count":1,"next_page":null,"previous_page":null,"results":[{"id":70,"title":"Reset password","snippet":"how to reset","locale":"en-us","html_url":"https://acme.zendesk.com/hc/en-us/articles/70","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}]}`,
		"zendesk_get_article":               `{"article":{"id":70,"title":"Reset password","body":"<p>Click reset</p>","locale":"en-us","author_id":20,"section_id":80,"draft":false,"html_url":"https://acme.zendesk.com/hc/en-us/articles/70","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}}`,
		"zendesk_list_tags":                 `{"tags":[{"name":"auth","count":12}],"meta":{"has_more":false,"after_cursor":"k"},"links":{"next":null}}`,
		"zendesk_list_satisfaction_ratings": `{"satisfaction_ratings":[{"id":90,"score":"good","comment":"fast","ticket_id":1,"assignee_id":20,"requester_id":10,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}],"meta":{"has_more":false,"after_cursor":"l"},"links":{"next":null}}`,
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
