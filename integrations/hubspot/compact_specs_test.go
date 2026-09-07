package hubspot

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

func TestFieldCompactionSpecs_OnlyReadTools(t *testing.T) {
	mutationPrefixes := []string{"create", "update", "delete"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, string(toolName), "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	h := &hubspot{}
	fields, ok := h.CompactSpec("hubspot_search_contacts")
	require.True(t, ok, "hubspot_search_contacts should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	h := &hubspot{}
	_, ok := h.CompactSpec("hubspot_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	h := &hubspot{}
	_, ok := h.CompactSpec("hubspot_create_contact")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	record := `{"id":"1","properties":{"email":"a@b.com","firstname":"Ada"},"createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-01-02T00:00:00Z","archived":false,"associations":{"companies":{"results":[{"id":"9","type":"contact_to_company"}]}}}`
	listPayload := `{"results":[` + record + `],"paging":{"next":{"after":"10"}}}`
	searchPayload := `{"total":1,"results":[` + record + `],"paging":{"next":{"after":"10"}}}`

	handlerOutputs := map[string]string{
		"hubspot_search_contacts":   searchPayload,
		"hubspot_list_contacts":     listPayload,
		"hubspot_get_contact":       record,
		"hubspot_search_companies":  searchPayload,
		"hubspot_list_companies":    listPayload,
		"hubspot_get_company":       record,
		"hubspot_search_deals":      searchPayload,
		"hubspot_list_deals":        listPayload,
		"hubspot_get_deal":          record,
		"hubspot_search_tickets":    searchPayload,
		"hubspot_list_tickets":      listPayload,
		"hubspot_get_ticket":        record,
		"hubspot_search_objects":    searchPayload,
		"hubspot_list_objects":      listPayload,
		"hubspot_get_object":        record,
		"hubspot_list_associations": `{"results":[{"id":"9","type":"contact_to_company"}],"paging":{"next":{"after":"2"}}}`,
		"hubspot_list_owners":       `{"results":[{"id":"41629779","email":"a@b.com","firstName":"Ada","lastName":"Lovelace","userId":1,"archived":false,"teams":[{"id":"1","name":"Sales"}]}],"paging":{"next":{"after":"2"}}}`,
		"hubspot_list_pipelines":    `{"results":[{"id":"default","label":"Sales Pipeline","displayOrder":0,"archived":false,"stages":[{"id":"appointmentscheduled","label":"Appointment Scheduled","displayOrder":0,"metadata":{"probability":"0.2"}}]}]}`,
		"hubspot_list_properties":   `{"results":[{"name":"email","label":"Email","type":"string","fieldType":"text","groupName":"contactinformation","hidden":false,"archived":false,"hasUniqueValue":true,"options":[{"label":"A","value":"a"}]}]}`,
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
