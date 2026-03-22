package linear

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
	mutationPrefixes := []string{"create", "update", "archive", "unarchive", "delete"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	l := &linear{}
	fields, ok := l.CompactSpec("linear_list_issues")
	require.True(t, ok, "linear_list_issues should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	l := &linear{}
	_, ok := l.CompactSpec("linear_create_issue")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	l := &linear{}
	_, ok := l.CompactSpec("linear_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

// TestFieldCompactionSpecs_ShapeParity verifies that compaction specs match
// the actual handler output structure (GraphQL envelope). A spec with flat
// fields like "id" when the handler returns {"issues":{"nodes":[...]}}
// produces {} — this test catches that.
func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	// Representative handler output shapes matching the GraphQL query structure.
	handlerOutputs := map[string]string{
		// Top-level collection: root.nodes[] + root.pageInfo
		"linear_list_issues":   `{"issues":{"nodes":[{"id":"i1","identifier":"ENG-1","title":"Bug","url":"https://linear.app/i1","priority":1,"estimate":3,"dueDate":"2024-01-01","createdAt":"2024-01-01","updatedAt":"2024-01-02","state":{"name":"In Progress","type":"started"},"assignee":{"name":"Alice"},"labels":{"nodes":[{"name":"bug"}]},"project":{"name":"Alpha"},"projectMilestone":{"name":"MVP"},"cycle":{"name":"Sprint 1"}}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"}}}`,
		"linear_search_issues": `{"searchIssues":{"nodes":[{"id":"i1","identifier":"ENG-1","title":"Bug","url":"https://linear.app/i1","priority":1,"createdAt":"2024-01-01","updatedAt":"2024-01-02","state":{"name":"Done","type":"completed"},"assignee":{"name":"Bob"},"labels":{"nodes":[{"name":"fix"}]}}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"}}}`,

		// Single record: root.field
		"linear_get_issue": `{"issue":{"id":"i1","identifier":"ENG-1","title":"Bug","description":"A bug","url":"https://linear.app/i1","priority":2,"estimate":5,"dueDate":"2024-01-15","createdAt":"2024-01-01","updatedAt":"2024-01-02","state":{"name":"Todo","type":"unstarted"},"assignee":{"name":"Alice","email":"a@b.com"},"labels":{"nodes":[{"name":"bug"}]},"project":{"name":"Alpha"},"projectMilestone":{"id":"pm1","name":"MVP"},"cycle":{"name":"Sprint 1"},"parent":{"identifier":"ENG-0"},"comments":{"nodes":[{"id":"c1","body":"comment","user":{"name":"Bob"},"createdAt":"2024-01-03"}]}}}`,

		// Nested collection: root.child.nodes[]
		"linear_list_issue_comments":  `{"issue":{"comments":{"nodes":[{"id":"c1","body":"comment","createdAt":"2024-01-01","updatedAt":"2024-01-02","user":{"name":"Alice"}}]}}}`,
		"linear_list_issue_relations": `{"issue":{"relations":{"nodes":[{"id":"r1","type":"blocks","relatedIssue":{"identifier":"ENG-2","title":"Blocked"}}]},"inverseRelations":{"nodes":[{"id":"r2","type":"blocked-by","issue":{"identifier":"ENG-0","title":"Blocker"}}]}}}`,
		"linear_list_issue_labels":    `{"issue":{"labels":{"nodes":[{"id":"l1","name":"bug","color":"#ff0000","description":"Bug label"}]}}}`,

		// Attachments
		"linear_list_attachments": `{"attachments":{"nodes":[{"id":"a1","title":"PR","url":"https://github.com/pr/1","subtitle":"merged","createdAt":"2024-01-01"}]}}`,

		// Projects
		"linear_list_projects":           `{"projects":{"nodes":[{"id":"p1","name":"Alpha","slugId":"alpha","state":"started","progress":0.5,"lead":{"name":"Alice"},"startDate":"2024-01-01","targetDate":"2024-06-01","createdAt":"2024-01-01","updatedAt":"2024-01-02"}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"}}}`,
		"linear_search_projects":         `{"searchProjects":{"nodes":[{"id":"p1","name":"Alpha","slugId":"alpha","state":"started","progress":0.5,"lead":{"name":"Alice"},"startDate":"2024-01-01","targetDate":"2024-06-01"}]}}`,
		"linear_get_project":             `{"project":{"id":"p1","name":"Alpha","slugId":"alpha","description":"Main project","url":"https://linear.app/p1","state":"started","progress":0.5,"lead":{"name":"Alice","email":"a@b.com"},"startDate":"2024-01-01","targetDate":"2024-06-01","createdAt":"2024-01-01","updatedAt":"2024-01-02","projectUpdates":{"nodes":[{"id":"pu1","health":"onTrack","createdAt":"2024-01-05"}]},"projectMilestones":{"nodes":[{"id":"pm1","name":"MVP","targetDate":"2024-03-01"}]}}}`,
		"linear_list_project_updates":    `{"project":{"projectUpdates":{"nodes":[{"id":"pu1","body":"On track","health":"onTrack","createdAt":"2024-01-05","user":{"name":"Alice"}}]}}}`,
		"linear_list_project_milestones": `{"project":{"projectMilestones":{"nodes":[{"id":"pm1","name":"MVP","description":"Minimum viable","targetDate":"2024-03-01","sortOrder":1}]}}}`,

		// Cycles
		"linear_list_cycles": `{"cycles":{"nodes":[{"id":"cy1","name":"Sprint 1","number":1,"startsAt":"2024-01-01","endsAt":"2024-01-14","progress":0.8,"team":{"name":"Engineering"}}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"}}}`,
		"linear_get_cycle":   `{"cycle":{"id":"cy1","name":"Sprint 1","number":1,"description":"First sprint","startsAt":"2024-01-01","endsAt":"2024-01-14","progress":0.8,"team":{"name":"Engineering"},"issues":{"nodes":[{"id":"i1","identifier":"ENG-1","title":"Bug","state":{"name":"Done"},"assignee":{"name":"Alice"}}]}}}`,

		// Teams
		"linear_list_teams": `{"teams":{"nodes":[{"id":"t1","name":"Engineering","key":"ENG","description":"Engineering team"}]}}`,
		"linear_get_team":   `{"team":{"id":"t1","name":"Engineering","key":"ENG","description":"Engineering team","members":{"nodes":[{"id":"u1","name":"Alice"}]},"states":{"nodes":[{"id":"s1","name":"Todo","type":"unstarted"}]},"projects":{"nodes":[{"id":"p1","name":"Alpha","state":"started"}]}}}`,

		// Users
		"linear_viewer":     `{"viewer":{"id":"u1","name":"Alice","email":"a@b.com","displayName":"alice","admin":true,"active":true,"organization":{"name":"Acme","urlKey":"acme"}}}`,
		"linear_list_users": `{"users":{"nodes":[{"id":"u1","name":"Alice","email":"a@b.com","displayName":"alice","admin":true,"active":true}]}}`,
		"linear_get_user":   `{"user":{"id":"u1","name":"Alice","email":"a@b.com","displayName":"alice","admin":true,"active":true,"assignedIssues":{"nodes":[{"identifier":"ENG-1","title":"Bug","state":{"name":"Done"}}]}}}`,

		// Labels
		"linear_list_labels": `{"issueLabels":{"nodes":[{"id":"l1","name":"bug","color":"#ff0000","description":"Bugs","parent":{"name":"Type"}}]}}`,

		// Workflow States
		"linear_list_workflow_states": `{"workflowStates":{"nodes":[{"id":"ws1","name":"Todo","type":"unstarted","color":"#ccc","position":0}]}}`,

		// Documents
		"linear_list_documents":   `{"documents":{"nodes":[{"id":"d1","title":"RFC","icon":"📄","project":{"name":"Alpha"},"creator":{"name":"Alice"},"createdAt":"2024-01-01","updatedAt":"2024-01-02"}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"}}}`,
		"linear_search_documents": `{"searchDocuments":{"nodes":[{"id":"d1","title":"RFC","icon":"📄","project":{"name":"Alpha"},"creator":{"name":"Alice"},"createdAt":"2024-01-01","updatedAt":"2024-01-02"}]}}`,
		"linear_get_document":     `{"document":{"id":"d1","title":"RFC","icon":"📄","content":"# RFC","project":{"name":"Alpha"},"creator":{"name":"Alice"},"createdAt":"2024-01-01","updatedAt":"2024-01-02"}}`,

		// Initiatives
		"linear_list_initiatives": `{"initiatives":{"nodes":[{"id":"in1","name":"Q1 Goals","status":"inProgress","targetDate":"2024-03-31","createdAt":"2024-01-01","owner":{"name":"Alice"}}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"}}}`,
		"linear_get_initiative":   `{"initiative":{"id":"in1","name":"Q1 Goals","description":"Q1 objectives","status":"inProgress","targetDate":"2024-03-31","createdAt":"2024-01-01","updatedAt":"2024-01-02","owner":{"name":"Alice"},"projects":{"nodes":[{"name":"Alpha","state":"started"}]}}}`,

		// Misc
		"linear_list_favorites":     `{"favorites":{"nodes":[{"id":"f1","type":"issue","issue":{"identifier":"ENG-1"},"project":null,"cycle":null,"customView":null}]}}`,
		"linear_list_webhooks":      `{"webhooks":{"nodes":[{"id":"w1","url":"https://hook.com","enabled":true,"resourceTypes":["Issue"],"createdAt":"2024-01-01"}]}}`,
		"linear_list_notifications": `{"notifications":{"nodes":[{"id":"n1","type":"issueAssigned","readAt":null,"createdAt":"2024-01-01","issue":{"identifier":"ENG-1","title":"Bug"}}]}}`,
		"linear_list_templates":     `{"templates":[{"id":"t1","name":"Bug Report","type":"issue","description":"Template for bugs"}]}`,
		"linear_get_organization":   `{"organization":{"id":"o1","name":"Acme","urlKey":"acme","createdAt":"2024-01-01","userCount":10}}`,
		"linear_list_custom_views":  `{"customViews":{"nodes":[{"id":"cv1","name":"My Issues","description":"Assigned to me","filterData":{"assignee":{"isMe":true}},"createdAt":"2024-01-01"}]}}`,
		"linear_rate_limit":         `{"rateLimitStatus":{"identifier":"org:acme","requestLimit":1500,"remainingRequests":1400,"payloadLimit":250000,"remainingPayload":249000,"reset":"2024-01-01T01:00:00Z"}}`,
	}

	for toolName, jsonPayload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[toolName]
			require.True(t, ok, "spec must exist for %s", toolName)

			compacted, err := mcp.CompactJSON([]byte(jsonPayload), fields)
			require.NoError(t, err, "compaction should not error for %s", toolName)

			assert.NotEqual(t, "{}", string(compacted), "compacted %s should not be empty object", toolName)
		})
	}
}
