package intercom

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
	c := &intercom{}
	fields, ok := c.CompactSpec("intercom_search_conversations")
	require.True(t, ok, "intercom_search_conversations should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	c := &intercom{}
	_, ok := c.CompactSpec("intercom_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"intercom_search_conversations": `{"total_count":1,"pages":{"page":1,"per_page":20,"total_pages":1,"next":{"starting_after":"c1"}},"conversations":[{"id":"1","title":"Help","state":"open","open":true,"read":true,"priority":"not_priority","created_at":1,"updated_at":2,"waiting_since":3,"snoozed_until":0,"admin_assignee_id":"a1","team_assignee_id":"t1","source":{"subject":"Help","type":"conversation","author":{"name":"Ada","email":"a@b.com"}},"contacts":{"contacts":[{"id":"u1"}]},"tags":{"tags":[{"id":"tag1","name":"billing"}]}}]}`,
		"intercom_list_conversations":   `{"total_count":1,"pages":{"page":1,"per_page":20,"total_pages":1,"next":{"starting_after":"c1"}},"conversations":[{"id":"1","title":"Help","state":"open","open":true,"read":true,"priority":"not_priority","created_at":1,"updated_at":2,"waiting_since":3,"snoozed_until":0,"admin_assignee_id":"a1","team_assignee_id":"t1","source":{"subject":"Help","type":"conversation","author":{"name":"Ada","email":"a@b.com"}},"contacts":{"contacts":[{"id":"u1"}]},"tags":{"tags":[{"id":"tag1","name":"billing"}]}}]}`,
		"intercom_get_conversation":     `{"id":"1","title":"Help","state":"open","open":true,"read":true,"priority":"not_priority","created_at":1,"updated_at":2,"waiting_since":3,"snoozed_until":0,"admin_assignee_id":"a1","team_assignee_id":"t1","source":{"subject":"Help","type":"conversation","body":"<p>hi</p>","author":{"id":"u1","name":"Ada","email":"a@b.com","type":"user"}},"contacts":{"contacts":[{"id":"u1"}]},"tags":{"tags":[{"id":"tag1"}]},"conversation_parts":{"conversation_parts":[{"id":"p1","part_type":"comment","body":"<p>reply</p>","created_at":4,"author":{"id":"a1","name":"Admin","email":"ad@b.com","type":"admin"}}]}}`,
		"intercom_search_contacts":      `{"total_count":1,"pages":{"page":1,"per_page":20,"total_pages":1,"next":{"starting_after":"u1"}},"data":[{"id":"u1","role":"user","email":"a@b.com","name":"Ada","phone":"+1","external_id":"x1","created_at":1,"updated_at":2,"last_seen_at":3,"unsubscribed_from_emails":false}]}`,
		"intercom_list_contacts":        `{"total_count":1,"pages":{"page":1,"per_page":20,"total_pages":1,"next":{"starting_after":"u1"}},"data":[{"id":"u1","role":"user","email":"a@b.com","name":"Ada","phone":"+1","external_id":"x1","created_at":1,"updated_at":2,"last_seen_at":3,"unsubscribed_from_emails":false}]}`,
		"intercom_get_contact":          `{"id":"u1","role":"user","email":"a@b.com","name":"Ada","phone":"+1","external_id":"x1","created_at":1,"updated_at":2,"last_seen_at":3,"signed_up_at":1,"unsubscribed_from_emails":false,"custom_attributes":{"plan":"pro"},"companies":{"data":[{"id":"co1"}]}}`,
		"intercom_list_companies":       `{"total_count":1,"pages":{"page":1,"per_page":20,"total_pages":1,"next":"n1"},"data":[{"id":"co1","name":"Acme","company_id":"acme","plan":"pro","size":10,"website":"https://acme.test","industry":"saas","monthly_spend":100,"created_at":1,"updated_at":2}]}`,
		"intercom_get_company":          `{"id":"co1","name":"Acme","company_id":"acme","plan":"pro","size":10,"website":"https://acme.test","industry":"saas","monthly_spend":100,"created_at":1,"updated_at":2,"custom_attributes":{"tier":"a"},"user_count":3}`,
		"intercom_search_tickets":       `{"total_count":1,"pages":{"page":1,"per_page":20,"total_pages":1,"next":{"starting_after":"t1"}},"tickets":[{"id":"t1","ticket_id":"12","ticket_state":"submitted","open":true,"created_at":1,"updated_at":2,"admin_assignee_id":"a1","team_assignee_id":"tm1","ticket_type":{"id":"tt1","name":"Bug"},"ticket_attributes":{"_default_title_":"Help"},"contacts":{"contacts":[{"id":"u1"}]}}]}`,
		"intercom_get_ticket":           `{"id":"t1","ticket_id":"12","ticket_state":"submitted","open":true,"created_at":1,"updated_at":2,"admin_assignee_id":"a1","team_assignee_id":"tm1","ticket_type":{"id":"tt1","name":"Bug"},"ticket_attributes":{"_default_title_":"Help"},"contacts":{"contacts":[{"id":"u1"}]},"ticket_parts":{"ticket_parts":[{"id":"p1"}]}}`,
		"intercom_search_articles":      `{"total_count":1,"pages":{"page":1,"per_page":20,"next":{"starting_after":"a1"}},"data":{"articles":[{"id":"a1","title":"Billing","description":"how to pay","state":"published","url":"https://help.test/a1","author_id":1,"created_at":1,"updated_at":2}]},"highlights":{"a1":["pay"]}}`,
		"intercom_list_articles":        `{"total_count":1,"pages":{"page":1,"per_page":20,"total_pages":1,"next":{"starting_after":"a1"}},"data":[{"id":"a1","title":"Billing","description":"how to pay","state":"published","url":"https://help.test/a1","author_id":1,"created_at":1,"updated_at":2}]}`,
		"intercom_get_article":          `{"id":"a1","title":"Billing","description":"how to pay","body":"<p>pay here</p>","state":"published","url":"https://help.test/a1","author_id":1,"created_at":1,"updated_at":2,"parent_id":"c1","parent_type":"collection"}`,
		"intercom_list_admins":          `{"type":"admin.list","admins":[{"id":"a1","name":"Ada","email":"a@b.com","away_mode_enabled":false,"has_inbox_seat":true,"team_ids":["t1"]}]}`,
		"intercom_get_me":               `{"id":"a1","name":"Ada","email":"a@b.com","type":"admin","away_mode_enabled":false,"has_inbox_seat":true,"team_ids":["t1"],"app":{"id_code":"abc","name":"Acme"}}`,
		"intercom_list_teams":           `{"type":"team.list","teams":[{"id":"t1","name":"Support","admin_ids":["a1"]}]}`,
		"intercom_get_team":             `{"id":"t1","name":"Support","admin_ids":["a1"]}`,
		"intercom_list_tags":            `{"type":"list","data":[{"id":"tag1","name":"billing"}]}`,
		"intercom_list_ticket_types":    `{"type":"list","data":[{"id":"tt1","name":"Bug","description":"product bugs","category":"Customer","archived":false,"icon":"🐞"}]}`,
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
