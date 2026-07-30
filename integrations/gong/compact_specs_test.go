package gong

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
	g := &gong{}
	fields, ok := g.CompactSpec("gong_list_calls")
	require.True(t, ok, "gong_list_calls should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	g := &gong{}
	_, ok := g.CompactSpec("gong_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"gong_list_calls":             `{"requestId":"r1","records":{"totalRecords":1,"currentPageSize":1,"cursor":"c1"},"calls":[{"id":"1","title":"Demo","started":"2024-01-01T00:00:00Z","duration":60,"direction":"Outbound","system":"Zoom","scope":"Internal","media":"Video","language":"eng","workspaceId":"w1","primaryUserId":"u1","url":"https://app.gong.io/call?id=1"}]}`,
		"gong_get_call":               `{"requestId":"r1","call":{"id":"1","title":"Demo","started":"2024-01-01T00:00:00Z","duration":60,"direction":"Outbound","system":"Zoom","scope":"Internal","media":"Video","language":"eng","workspaceId":"w1","primaryUserId":"u1","url":"https://app.gong.io/call?id=1"}}`,
		"gong_list_calls_extensive":   `{"requestId":"r1","records":{"totalRecords":1,"cursor":"c1"},"calls":[{"metaData":{"id":"1","title":"Demo","started":"2024-01-01T00:00:00Z","duration":60,"direction":"Outbound","primaryUserId":"u1","workspaceId":"w1","url":"https://app.gong.io/call?id=1"},"parties":[{"emailAddress":"a@b.com","name":"A","userId":"u1"}],"content":{"trackers":[{"name":"pricing"}],"topics":[{"name":"intro"}],"brief":"Spotlight brief"}}]}`,
		"gong_get_transcripts":        `{"requestId":"r1","records":{"totalRecords":1,"cursor":"c1"},"callTranscripts":[{"callId":"1","transcript":[{"speakerId":"s1","topic":"pricing","sentences":[{"text":"hello","start":0,"end":1}]}]}]}`,
		"gong_list_users":             `{"requestId":"r1","records":{"totalRecords":1,"cursor":"c1"},"users":[{"id":"u1","emailAddress":"a@b.com","firstName":"A","lastName":"B","title":"AE","active":true,"managerId":"m1"}]}`,
		"gong_get_user":               `{"requestId":"r1","user":{"id":"u1","emailAddress":"a@b.com","firstName":"A","lastName":"B","title":"AE","active":true,"managerId":"m1"}}`,
		"gong_list_users_extensive":   `{"requestId":"r1","records":{"cursor":"c1"},"users":[{"id":"u1","emailAddress":"a@b.com","firstName":"A","lastName":"B","title":"AE","active":true,"managerId":"m1"}]}`,
		"gong_list_workspaces":        `{"requestId":"r1","workspaces":[{"id":"w1","name":"Main","description":"default"}]}`,
		"gong_list_library_folders":   `{"requestId":"r1","folders":[{"id":"f1","name":"Coaching","parentFolderId":"","createdBy":"u1"}]}`,
		"gong_get_library_folder":     `{"requestId":"r1","id":"f1","name":"Coaching","createdBy":"u1","updated":"2024-01-02T00:00:00Z","calls":[{"id":"1","title":"Demo","url":"https://app.gong.io/call?id=1","note":"good"}]}`,
		"gong_list_stats_activity":    `{"requestId":"r1","records":{"cursor":"c1"},"usersAggregateActivityStats":[{"userId":"u1","callsAsHost":3}]}`,
		"gong_list_stats_interaction": `{"requestId":"r1","records":{"cursor":"c1"},"peopleInteractionStats":[{"userId":"u1","talkRatio":0.4}]}`,
		"gong_list_stats_scorecards":  `{"requestId":"r1","records":{"cursor":"c1"},"answeredScorecards":[{"scorecardId":"s1","reviewedUserId":"u1"}]}`,
		"gong_list_logs":              `{"requestId":"r1","records":{"cursor":"c1"},"logEntries":[{"userId":"u1","event":"login"}]}`,
		"gong_get_data_privacy":       `{"requestId":"r1","emails":["a@b.com"],"calls":[{"id":"1"}],"meetings":[],"customerData":[],"customerEngagement":[],"suppliedPhoneNumber":"+15551212","matchingPhoneNumbers":["+15551212"],"emailAddresses":["a@b.com"]}`,
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
