package launchdarkly

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

func TestFieldCompactionSpecs_ReadToolsCovered(t *testing.T) {
	for _, name := range []string{
		"launchdarkly_list_projects",
		"launchdarkly_list_environments",
		"launchdarkly_list_flags",
		"launchdarkly_get_flag",
		"launchdarkly_list_flag_statuses",
	} {
		_, ok := fieldCompactionSpecs[mcp.ToolName(name)]
		assert.True(t, ok, "read tool %s should have a compaction spec", name)
	}
}

func TestFieldCompactionSpecs_NoSpecOnMutationTools(t *testing.T) {
	_, ok := fieldCompactionSpecs["launchdarkly_toggle_flag"]
	assert.False(t, ok, "mutation tools should return full confirmation responses")
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	l := &launchdarkly{}
	fields, ok := l.CompactSpec("launchdarkly_list_flags")
	require.True(t, ok, "launchdarkly_list_flags should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	l := &launchdarkly{}
	_, ok := l.CompactSpec("launchdarkly_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"launchdarkly_list_projects":      `{"_links":{"self":{"href":"/api/v2/projects"}},"items":[{"_id":"p1","key":"default","name":"Default Project","tags":["web"],"_links":{"self":{"href":"/api/v2/projects/default"}}}],"totalCount":1}`,
		"launchdarkly_list_environments":  `{"_links":{"self":{"href":"/api/v2/projects/default/environments"}},"items":[{"_id":"e1","key":"production","name":"Production","color":"417505","critical":true,"tags":[],"defaultTtl":0,"secureMode":false,"apiKey":"sdk-secret","mobileKey":"mob-secret","_links":{}}],"totalCount":1}`,
		"launchdarkly_list_flags":         `{"_links":{},"items":[{"_id":"f1","key":"dark-mode","name":"Dark Mode","kind":"boolean","description":"Enable dark theme","creationDate":1700000000000,"tags":["ui"],"archived":false,"temporary":true,"_maintainer":{"_id":"m1","email":"ada@example.com","firstName":"Ada"},"variations":[{"_id":"v0","value":true},{"_id":"v1","value":false}],"environments":{"production":{"on":true,"archived":false,"salt":"abc","sel":"def","lastModified":1700000001000,"version":3,"_site":{"href":"/default/production/features/dark-mode"},"_environmentName":"Production"}}}],"totalCount":1,"totalCountWithDifferences":0}`,
		"launchdarkly_get_flag":           `{"_id":"f1","key":"dark-mode","name":"Dark Mode","kind":"boolean","description":"Enable dark theme","creationDate":1700000000000,"tags":["ui"],"archived":false,"temporary":true,"clientSideAvailability":{"usingEnvironmentId":true,"usingMobileKey":false},"_maintainer":{"_id":"m1","email":"ada@example.com","firstName":"Ada"},"variations":[{"_id":"v0","value":true,"name":"On"},{"_id":"v1","value":false,"name":"Off"}],"environments":{"production":{"on":true,"archived":false,"salt":"abc","sel":"def","lastModified":1700000001000,"version":3,"fallthrough":{"variation":0},"offVariation":1,"rules":[],"targets":[],"prerequisites":[],"_site":{"href":"/default/production/features/dark-mode"},"_environmentName":"Production"}},"_links":{"self":{"href":"/api/v2/flags/default/dark-mode"}}}`,
		"launchdarkly_list_flag_statuses": `{"_links":{"self":{"href":"/api/v2/flag-statuses/default/production"}},"items":[{"name":"active","lastRequested":"2026-01-01T00:00:00Z","default":false,"_links":{"parent":{"href":"/api/v2/flags/default/dark-mode","type":"application/json"},"self":{"href":"/api/v2/flag-statuses/default/production/dark-mode","type":"application/json"}}}]}`,
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

func TestFieldCompactionSpecs_ListEnvironmentsDropsSDKKeys(t *testing.T) {
	fields := fieldCompactionSpecs["launchdarkly_list_environments"]
	payload := `{"items":[{"key":"production","name":"Production","apiKey":"sdk-secret","mobileKey":"mob-secret"}],"totalCount":1}`
	compacted, err := mcp.CompactJSON([]byte(payload), fields)
	require.NoError(t, err)
	assert.NotContains(t, string(compacted), "sdk-secret")
	assert.NotContains(t, string(compacted), "mob-secret")
	assert.Contains(t, string(compacted), "Production")
}

func TestFieldCompactionSpecs_ListFlagsKeepsEnvironmentState(t *testing.T) {
	fields := fieldCompactionSpecs["launchdarkly_list_flags"]
	payload := `{"items":[{"key":"dark-mode","salt":"noise","environments":{"production":{"on":true,"version":3}}}],"totalCount":1}`
	compacted, err := mcp.CompactJSON([]byte(payload), fields)
	require.NoError(t, err)
	assert.Contains(t, string(compacted), `"on":true`)
	assert.Contains(t, string(compacted), `"production"`)
	assert.NotContains(t, string(compacted), "noise")
}

func TestFieldCompactionSpecs_FlagStatusesKeepParentLink(t *testing.T) {
	fields := fieldCompactionSpecs["launchdarkly_list_flag_statuses"]
	payload := `{"items":[{"name":"active","lastRequested":"2026-01-01T00:00:00Z","default":false,"_links":{"parent":{"href":"/api/v2/flags/default/dark-mode","type":"application/json"},"self":{"href":"/x"}}}]}`
	compacted, err := mcp.CompactJSON([]byte(payload), fields)
	require.NoError(t, err)
	assert.Contains(t, string(compacted), "dark-mode", "flag key must be recoverable from the parent link")
	assert.NotContains(t, string(compacted), `"/x"`)
}
