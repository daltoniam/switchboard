package linear

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Issues ────────────────────────────────────────────────────────
	// GQL root: issues { nodes { ... } pageInfo { ... } }
	"linear_list_issues": {"issues.nodes[].id", "issues.nodes[].identifier", "issues.nodes[].title", "issues.nodes[].url", "issues.nodes[].priority", "issues.nodes[].estimate", "issues.nodes[].dueDate", "issues.nodes[].createdAt", "issues.nodes[].updatedAt", "issues.nodes[].state.name", "issues.nodes[].state.type", "issues.nodes[].assignee.name", "issues.nodes[].labels.nodes[].name", "issues.nodes[].project.name", "issues.nodes[].projectMilestone.name", "issues.nodes[].cycle.name", "issues.pageInfo.hasNextPage", "issues.pageInfo.endCursor"},
	// GQL root: searchIssues { nodes { ... } pageInfo { ... } }
	"linear_search_issues": {"searchIssues.nodes[].id", "searchIssues.nodes[].identifier", "searchIssues.nodes[].title", "searchIssues.nodes[].url", "searchIssues.nodes[].priority", "searchIssues.nodes[].createdAt", "searchIssues.nodes[].updatedAt", "searchIssues.nodes[].state.name", "searchIssues.nodes[].state.type", "searchIssues.nodes[].assignee.name", "searchIssues.nodes[].labels.nodes[].name", "searchIssues.pageInfo.hasNextPage", "searchIssues.pageInfo.endCursor"},
	// GQL root: issue { ... comments { nodes { ... } } }
	"linear_get_issue": {"issue.id", "issue.identifier", "issue.title", "issue.description", "issue.url", "issue.priority", "issue.estimate", "issue.dueDate", "issue.createdAt", "issue.updatedAt", "issue.state.name", "issue.state.type", "issue.assignee.name", "issue.assignee.email", "issue.labels.nodes[].name", "issue.project.name", "issue.projectMilestone.id", "issue.projectMilestone.name", "issue.cycle.name", "issue.parent.identifier", "issue.comments.nodes[].id", "issue.comments.nodes[].body", "issue.comments.nodes[].user.name", "issue.comments.nodes[].createdAt"},
	// GQL root: issue { comments { nodes { ... } } }
	"linear_list_issue_comments": {"issue.comments.nodes[].id", "issue.comments.nodes[].body", "issue.comments.nodes[].createdAt", "issue.comments.nodes[].updatedAt", "issue.comments.nodes[].user.name"},
	// GQL root: issue { relations { nodes { ... } } inverseRelations { nodes { ... } } }
	"linear_list_issue_relations": {"issue.relations.nodes[].id", "issue.relations.nodes[].type", "issue.relations.nodes[].relatedIssue.identifier", "issue.relations.nodes[].relatedIssue.title", "issue.inverseRelations.nodes[].id", "issue.inverseRelations.nodes[].type", "issue.inverseRelations.nodes[].issue.identifier", "issue.inverseRelations.nodes[].issue.title"},
	// GQL root: issue { labels { nodes { ... } } }
	"linear_list_issue_labels": {"issue.labels.nodes[].id", "issue.labels.nodes[].name", "issue.labels.nodes[].color", "issue.labels.nodes[].description"},
	// GQL root: attachments { nodes { ... } }
	"linear_list_attachments": {"attachments.nodes[].id", "attachments.nodes[].title", "attachments.nodes[].url", "attachments.nodes[].subtitle", "attachments.nodes[].createdAt"},

	// ── Projects ──────────────────────────────────────────────────────
	// GQL root: projects { nodes { ... } pageInfo { ... } }
	"linear_list_projects": {"projects.nodes[].id", "projects.nodes[].name", "projects.nodes[].slugId", "projects.nodes[].state", "projects.nodes[].progress", "projects.nodes[].lead.name", "projects.nodes[].startDate", "projects.nodes[].targetDate", "projects.nodes[].createdAt", "projects.nodes[].updatedAt", "projects.pageInfo.hasNextPage", "projects.pageInfo.endCursor"},
	// GQL root: searchProjects { nodes { ... } }
	"linear_search_projects": {"searchProjects.nodes[].id", "searchProjects.nodes[].name", "searchProjects.nodes[].slugId", "searchProjects.nodes[].state", "searchProjects.nodes[].progress", "searchProjects.nodes[].lead.name", "searchProjects.nodes[].startDate", "searchProjects.nodes[].targetDate"},
	// GQL root: project { ... projectUpdates { nodes { ... } } projectMilestones { nodes { ... } } }
	"linear_get_project": {"project.id", "project.name", "project.slugId", "project.description", "project.url", "project.state", "project.progress", "project.lead.name", "project.lead.email", "project.startDate", "project.targetDate", "project.createdAt", "project.updatedAt", "project.projectUpdates.nodes[].id", "project.projectUpdates.nodes[].health", "project.projectUpdates.nodes[].createdAt", "project.projectMilestones.nodes[].id", "project.projectMilestones.nodes[].name", "project.projectMilestones.nodes[].targetDate"},
	// GQL root: project { projectUpdates { nodes { ... } } }
	"linear_list_project_updates": {"project.projectUpdates.nodes[].id", "project.projectUpdates.nodes[].body", "project.projectUpdates.nodes[].health", "project.projectUpdates.nodes[].createdAt", "project.projectUpdates.nodes[].user.name"},
	// GQL root: project { projectMilestones { nodes { ... } } }
	"linear_list_project_milestones": {"project.projectMilestones.nodes[].id", "project.projectMilestones.nodes[].name", "project.projectMilestones.nodes[].description", "project.projectMilestones.nodes[].targetDate", "project.projectMilestones.nodes[].sortOrder"},

	// ── Cycles ────────────────────────────────────────────────────────
	// GQL root: cycles { nodes { ... } pageInfo { ... } }
	"linear_list_cycles": {"cycles.nodes[].id", "cycles.nodes[].name", "cycles.nodes[].number", "cycles.nodes[].startsAt", "cycles.nodes[].endsAt", "cycles.nodes[].progress", "cycles.nodes[].team.name", "cycles.pageInfo.hasNextPage", "cycles.pageInfo.endCursor"},
	// GQL root: cycle { ... issues { nodes { ... } } }
	"linear_get_cycle": {"cycle.id", "cycle.name", "cycle.number", "cycle.description", "cycle.startsAt", "cycle.endsAt", "cycle.progress", "cycle.team.name", "cycle.issues.nodes[].id", "cycle.issues.nodes[].identifier", "cycle.issues.nodes[].title", "cycle.issues.nodes[].state.name", "cycle.issues.nodes[].assignee.name"},

	// ── Teams ─────────────────────────────────────────────────────────
	// GQL root: teams { nodes { ... } }
	"linear_list_teams": {"teams.nodes[].id", "teams.nodes[].name", "teams.nodes[].key", "teams.nodes[].description"},
	// GQL root: team { ... }
	"linear_get_team": {"team.id", "team.name", "team.key", "team.description", "team.members.nodes[].id", "team.members.nodes[].name", "team.states.nodes[].id", "team.states.nodes[].name", "team.states.nodes[].type", "team.projects.nodes[].id", "team.projects.nodes[].name", "team.projects.nodes[].state"},

	// ── Users ─────────────────────────────────────────────────────────
	// GQL root: viewer { ... }
	"linear_viewer": {"viewer.id", "viewer.name", "viewer.email", "viewer.displayName", "viewer.admin", "viewer.active", "viewer.organization.name", "viewer.organization.urlKey"},
	// GQL root: users { nodes { ... } }
	"linear_list_users": {"users.nodes[].id", "users.nodes[].name", "users.nodes[].email", "users.nodes[].displayName", "users.nodes[].admin", "users.nodes[].active"},
	// GQL root: user { ... }
	"linear_get_user": {"user.id", "user.name", "user.email", "user.displayName", "user.admin", "user.active", "user.assignedIssues.nodes[].identifier", "user.assignedIssues.nodes[].title", "user.assignedIssues.nodes[].state.name"},

	// ── Labels ────────────────────────────────────────────────────────
	// GQL root: issueLabels { nodes { ... } }
	"linear_list_labels": {"issueLabels.nodes[].id", "issueLabels.nodes[].name", "issueLabels.nodes[].color", "issueLabels.nodes[].description", "issueLabels.nodes[].parent.name"},

	// ── Workflow States ───────────────────────────────────────────────
	// GQL root: workflowStates { nodes { ... } }
	"linear_list_workflow_states": {"workflowStates.nodes[].id", "workflowStates.nodes[].name", "workflowStates.nodes[].type", "workflowStates.nodes[].color", "workflowStates.nodes[].position"},

	// ── Documents ─────────────────────────────────────────────────────
	// GQL root: documents { nodes { ... } pageInfo { ... } }
	"linear_list_documents": {"documents.nodes[].id", "documents.nodes[].title", "documents.nodes[].icon", "documents.nodes[].project.name", "documents.nodes[].creator.name", "documents.nodes[].createdAt", "documents.nodes[].updatedAt", "documents.pageInfo.hasNextPage", "documents.pageInfo.endCursor"},
	// GQL root: searchDocuments { nodes { ... } }
	"linear_search_documents": {"searchDocuments.nodes[].id", "searchDocuments.nodes[].title", "searchDocuments.nodes[].icon", "searchDocuments.nodes[].project.name", "searchDocuments.nodes[].creator.name", "searchDocuments.nodes[].createdAt", "searchDocuments.nodes[].updatedAt"},
	// GQL root: document { ... }
	"linear_get_document": {"document.id", "document.title", "document.icon", "document.content", "document.project.name", "document.creator.name", "document.createdAt", "document.updatedAt"},

	// ── Initiatives ───────────────────────────────────────────────────
	// GQL root: initiatives { nodes { ... } pageInfo { ... } }
	"linear_list_initiatives": {"initiatives.nodes[].id", "initiatives.nodes[].name", "initiatives.nodes[].status", "initiatives.nodes[].targetDate", "initiatives.nodes[].createdAt", "initiatives.nodes[].owner.name", "initiatives.pageInfo.hasNextPage", "initiatives.pageInfo.endCursor"},
	// GQL root: initiative { ... }
	"linear_get_initiative": {"initiative.id", "initiative.name", "initiative.description", "initiative.status", "initiative.targetDate", "initiative.createdAt", "initiative.updatedAt", "initiative.owner.name", "initiative.projects.nodes[].name", "initiative.projects.nodes[].state"},

	// ── Misc ──────────────────────────────────────────────────────────
	// GQL root: favorites { nodes { ... } }
	"linear_list_favorites": {"favorites.nodes[].id", "favorites.nodes[].type", "favorites.nodes[].issue.identifier", "favorites.nodes[].project.name", "favorites.nodes[].cycle.name", "favorites.nodes[].customView.name"},
	// GQL root: webhooks { nodes { ... } }
	"linear_list_webhooks": {"webhooks.nodes[].id", "webhooks.nodes[].url", "webhooks.nodes[].enabled", "webhooks.nodes[].resourceTypes", "webhooks.nodes[].createdAt"},
	// GQL root: notifications { nodes { ... } }
	"linear_list_notifications": {"notifications.nodes[].id", "notifications.nodes[].type", "notifications.nodes[].readAt", "notifications.nodes[].createdAt", "notifications.nodes[].issue.identifier", "notifications.nodes[].issue.title"},
	// GQL root: templates (direct array, no .nodes wrapper)
	"linear_list_templates": {"templates[].id", "templates[].name", "templates[].type", "templates[].description"},
	// GQL root: organization { ... }
	"linear_get_organization": {"organization.id", "organization.name", "organization.urlKey", "organization.createdAt", "organization.userCount"},
	// GQL root: customViews { nodes { ... } }
	"linear_list_custom_views": {"customViews.nodes[].id", "customViews.nodes[].name", "customViews.nodes[].description", "customViews.nodes[].filterData", "customViews.nodes[].createdAt"},
	// GQL root: rateLimitStatus { ... }
	"linear_rate_limit": {"rateLimitStatus.identifier", "rateLimitStatus.requestLimit", "rateLimitStatus.remainingRequests", "rateLimitStatus.payloadLimit", "rateLimitStatus.remainingPayload", "rateLimitStatus.reset"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("linear: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
