package grist

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
	mutationPrefixes := []string{"create", "update", "delete", "add", "move", "copy", "upsert"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, string(toolName), "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	g := &grist{}
	fields, ok := g.CompactSpec("grist_list_orgs")
	require.True(t, ok, "grist_list_orgs should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	g := &grist{}
	_, ok := g.CompactSpec("grist_create_doc")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	g := &grist{}
	_, ok := g.CompactSpec("grist_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"grist_list_orgs":        `[{"id":1,"name":"Acme","domain":"acme","createdAt":"2024-01-01","updatedAt":"2024-01-02","owner":{"id":9,"name":"Ada","email":"ada@acme.com"},"access":"owners"}]`,
		"grist_get_org":          `{"id":1,"name":"Acme","domain":"acme","createdAt":"2024-01-01","updatedAt":"2024-01-02","owner":{"id":9,"name":"Ada","email":"ada@acme.com"},"access":"owners"}`,
		"grist_list_workspaces":  `[{"id":10,"name":"Ops","createdAt":"2024-01-01","updatedAt":"2024-01-02","access":"owners","docs":[{"id":"doc1","name":"CRM","isPinned":true,"urlId":"crm","access":"owners","updatedAt":"2024-01-03"}]}]`,
		"grist_get_workspace":    `{"id":10,"name":"Ops","createdAt":"2024-01-01","updatedAt":"2024-01-02","access":"owners","org":{"id":1,"name":"Acme","domain":"acme"},"docs":[{"id":"doc1","name":"CRM","isPinned":true,"urlId":"crm","access":"owners","updatedAt":"2024-01-03"}]}`,
		"grist_get_doc":          `{"id":"doc1","name":"CRM","createdAt":"2024-01-01","updatedAt":"2024-01-02","isPinned":false,"urlId":"crm","access":"owners","workspace":{"id":10,"name":"Ops","org":{"id":1,"name":"Acme","domain":"acme"}}}`,
		"grist_list_tables":      `[{"id":"People","fields":{"tableRef":1,"onDemand":false},"columns":[{"id":"Name","fields":{"label":"Name","type":"Text","isFormula":false,"formula":"","colRef":2}}]}]`,
		"grist_list_columns":     `[{"id":"Name","fields":{"label":"Name","type":"Text","isFormula":false,"formula":"","colRef":2,"parentId":1,"widgetOptions":""}}]`,
		"grist_list_records":     `[{"id":1,"fields":{"Name":"Ada","Age":36}}]`,
		"grist_query_sql":        `[{"fields":{"id":1,"Name":"Ada","Age":36}}]`,
		"grist_list_webhooks":    `[{"id":"wh1","fields":{"name":"notify","url":"https://example.com","enabled":true,"eventTypes":["add"],"tableId":"People","isReadyColumn":null},"usage":{"numWaiting":0,"status":"idle","lastSuccessTime":1,"lastFailureTime":null,"lastHttpStatus":200}}]`,
		"grist_list_attachments": `[{"id":3,"fields":{"fileName":"a.png","fileSize":12,"timeUploaded":1700000000,"imageHeight":10,"imageWidth":20}}]`,
		"grist_get_attachment":   `{"fileName":"a.png","fileSize":12,"timeUploaded":1700000000,"imageHeight":10,"imageWidth":20}`,
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
