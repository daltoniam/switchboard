package jira

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	assert.Equal(t, len(rawFieldCompactionSpecs), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_OnlyReadTools(t *testing.T) {
	mutationPrefixes := []string{"create", "update", "delete", "assign", "transition", "move", "add"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForReadTool(t *testing.T) {
	j := &jira{}
	fields, ok := j.CompactSpec("jira_search_issues")
	require.True(t, ok, "jira_search_issues should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	j := &jira{}
	_, ok := j.CompactSpec("jira_create_issue")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	j := &jira{}
	_, ok := j.CompactSpec("jira_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	// Representative payloads matching the real Jira API response shapes.
	handlerOutputs := map[string]string{
		// Issues
		"jira_search_issues":    `{"issues":[{"key":"PROJ-1","fields":{"summary":"Fix bug","status":{"name":"Open"},"assignee":{"displayName":"Alice"},"priority":{"name":"High"},"issuetype":{"name":"Bug"},"created":"2024-01-01","updated":"2024-01-02"}}],"isLast":false,"nextPageToken":"eyJhIjoiMTIzIn0="}`,
		"jira_get_issue":        `{"key":"PROJ-1","fields":{"summary":"Fix bug","status":{"name":"Open"},"assignee":{"displayName":"Alice","accountId":"abc"},"reporter":{"displayName":"Bob"},"priority":{"name":"High"},"issuetype":{"name":"Bug"},"description":{"type":"doc"},"created":"2024-01-01","updated":"2024-01-02","labels":["backend"],"components":[{"name":"API"}],"fixVersions":[{"name":"1.0"}]}}`,
		"jira_get_transitions":  `{"transitions":[{"id":"1","name":"Done","to":{"name":"Done"}}]}`,
		"jira_list_comments":    `{"comments":[{"id":"1","body":{"type":"doc"},"author":{"displayName":"Alice"},"created":"2024-01-01","updated":"2024-01-02"}]}`,
		"jira_list_issue_links": `[{"id":"1","type":{"name":"Blocks","inward":"is blocked by","outward":"blocks"},"inwardIssue":{"key":"PROJ-2","fields":{"summary":"Other"}},"outwardIssue":{"key":"PROJ-3","fields":{"summary":"Another"}}}]`,

		// Projects
		"jira_list_projects":           `{"values":[{"key":"PROJ","name":"Project","projectTypeKey":"software","style":"next-gen"}]}`,
		"jira_get_project":             `{"key":"PROJ","name":"Project","projectTypeKey":"software","description":"A project","lead":{"displayName":"Alice"},"components":[{"name":"API"}],"versions":[{"name":"1.0"}]}`,
		"jira_list_project_components": `[{"id":"1","name":"API","description":"API layer","lead":{"displayName":"Alice"},"assigneeType":"PROJECT_LEAD"}]`,
		"jira_list_project_versions":   `[{"id":"1","name":"1.0","description":"First release","released":true,"releaseDate":"2024-06-01","archived":false}]`,
		"jira_list_project_statuses":   `[{"id":"1","name":"Bug","statuses":[{"id":"10","name":"Open"},{"id":"11","name":"Done"}]}]`,

		// Boards & Sprints (Agile API)
		"jira_list_boards":        `{"values":[{"id":1,"name":"Board","type":"scrum","location":{"projectKey":"PROJ"}}]}`,
		"jira_get_board":          `{"id":1,"name":"Board","type":"scrum","location":{"projectKey":"PROJ"}}`,
		"jira_list_sprints":       `{"values":[{"id":1,"name":"Sprint 1","state":"active","startDate":"2024-01-01","endDate":"2024-01-14","goal":"Ship it"}]}`,
		"jira_get_sprint":         `{"id":1,"name":"Sprint 1","state":"active","startDate":"2024-01-01","endDate":"2024-01-14","goal":"Ship it","originBoardId":1}`,
		"jira_get_sprint_issues":  `{"issues":[{"key":"PROJ-1","fields":{"summary":"Fix bug","status":{"name":"Open"},"assignee":{"displayName":"Alice"},"priority":{"name":"High"}}}]}`,
		"jira_list_board_backlog": `{"issues":[{"key":"PROJ-2","fields":{"summary":"Backlog item","status":{"name":"To Do"},"assignee":{"displayName":"Bob"},"priority":{"name":"Low"}}}]}`,
		"jira_get_board_config":   `{"id":1,"name":"Board","columnConfig":{"columns":[{"name":"To Do","statuses":[{"id":"1"}]},{"name":"Done","statuses":[{"id":"2"}]}]}}`,

		// Users
		"jira_get_myself":   `{"accountId":"abc","displayName":"Alice","emailAddress":"a@b.com","active":true,"timeZone":"UTC"}`,
		"jira_search_users": `[{"accountId":"abc","displayName":"Alice","emailAddress":"a@b.com","active":true}]`,
		"jira_get_user":     `{"accountId":"abc","displayName":"Alice","emailAddress":"a@b.com","active":true,"timeZone":"UTC"}`,

		// Metadata
		"jira_list_issue_types": `[{"id":"1","name":"Bug","subtask":false,"description":"A bug"}]`,
		"jira_list_priorities":  `[{"id":"1","name":"High","description":"Important"}]`,
		"jira_list_statuses":    `[{"id":"1","name":"Open","statusCategory":{"name":"To Do"}}]`,
		"jira_list_labels":      `{"values":["backend","frontend"]}`,
		"jira_list_fields":      `[{"id":"summary","name":"Summary","custom":false,"schema":{"type":"string"}}]`,
		"jira_list_filters":     `{"values":[{"id":"1","name":"My Filter","jql":"project = PROJ","owner":{"displayName":"Alice"}}]}`,
		"jira_get_filter":       `{"id":"1","name":"My Filter","jql":"project = PROJ","owner":{"displayName":"Alice"},"description":"A filter"}`,

		// Worklogs & Info
		"jira_list_worklogs":   `{"worklogs":[{"id":"1","author":{"displayName":"Alice"},"timeSpent":"2h","started":"2024-01-01","comment":{"type":"doc"}}]}`,
		"jira_get_server_info": `{"baseUrl":"https://example.atlassian.net","version":"9.0","deploymentType":"Cloud","serverTitle":"Jira"}`,
	}

	for toolName, payload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok, "missing compaction spec for %s", toolName)
			compacted, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			assert.NotEqual(t, "{}", string(compacted), "compaction returned empty object — spec paths likely don't match response shape")
			assert.NotEqual(t, "[]", string(compacted), "compaction returned empty array — spec paths likely don't match response shape")
		})
	}
}
