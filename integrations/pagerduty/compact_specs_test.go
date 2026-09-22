package pagerduty

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
	p := &pagerduty{}
	fields, ok := p.CompactSpec("pagerduty_list_incidents")
	require.True(t, ok, "pagerduty_list_incidents should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	p := &pagerduty{}
	_, ok := p.CompactSpec("pagerduty_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"pagerduty_list_incidents": `{"incidents":[{"id":"P1","incident_number":12,"title":"DB down","status":"triggered","urgency":"high","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:01:00Z","html_url":"https://example.pagerduty.com/incidents/P1","service":{"id":"S1","summary":"API"},"assignments":[{"assignee":{"id":"U1","summary":"Ada"}}]}],"limit":25,"offset":0,"more":false}`,
		"pagerduty_get_incident":   `{"incident":{"id":"P1","incident_number":12,"title":"DB down","status":"triggered","urgency":"high","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:01:00Z","html_url":"https://example.pagerduty.com/incidents/P1","description":"Postgres primary unreachable","service":{"id":"S1","summary":"API"},"assignments":[{"assignee":{"id":"U1","summary":"Ada"}}],"escalation_policy":{"id":"E1","summary":"Eng"}}}`,
		"pagerduty_list_oncalls":   `{"oncalls":[{"escalation_level":1,"start":"2026-01-01T00:00:00Z","end":"2026-01-02T00:00:00Z","user":{"id":"U1","summary":"Ada","html_url":"https://example.pagerduty.com/users/U1"},"schedule":{"id":"SC1","summary":"Primary"},"escalation_policy":{"id":"E1","summary":"Eng"}}],"limit":25,"offset":0,"more":false}`,
		"pagerduty_list_services":  `{"services":[{"id":"S1","name":"API","summary":"API","status":"active","html_url":"https://example.pagerduty.com/services/S1"}],"limit":25,"offset":0,"more":false}`,
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
