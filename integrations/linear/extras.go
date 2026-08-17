package linear

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// ── Cycles ────────────────────────────────────────────────────────

func listCycles(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	filter := map[string]any{}
	if team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": teamID}}
	}

	vars := map[string]any{
		"first": mcp.OptInt(args, "first", 50),
	}
	if len(filter) > 0 {
		vars["filter"] = filter
	}
	if after != "" {
		vars["after"] = after
	}

	data, err := l.gql(ctx, `query($first: Int, $after: String, $filter: CycleFilter) {
		cycles(first: $first, after: $after, filter: $filter, orderBy: createdAt) {
			nodes {
				id name number description
				startsAt endsAt completedAt
				progress completedIssueCountHistory issueCountHistory
				team { id name key }
			}
			pageInfo { hasNextPage endCursor }
		}
	}`, vars)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCycle(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!) {
		cycle(id: $id) {
			id name number description
			startsAt endsAt completedAt
			progress completedIssueCountHistory issueCountHistory
			team { id name key }
			issues(first: 50) {
				nodes { id identifier title state { name } assignee { name } priority }
			}
		}
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCycle(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	startsAt := r.Str("starts_at")
	endsAt := r.Str("ends_at")
	name := r.Str("name")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	teamID, err := l.resolveTeamID(ctx, team)
	if err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"teamId":   teamID,
		"startsAt": startsAt,
		"endsAt":   endsAt,
	}
	if name != "" {
		input["name"] = name
	}
	if description != "" {
		input["description"] = description
	}

	data, err := l.gql(ctx, `mutation($input: CycleCreateInput!) {
		cycleCreate(input: $input) {
			cycle { id name number startsAt endsAt team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCycle(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	description := r.Str("description")
	startsAt := r.Str("starts_at")
	endsAt := r.Str("ends_at")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{}
	if name != "" {
		input["name"] = name
	}
	if description != "" {
		input["description"] = description
	}
	if startsAt != "" {
		input["startsAt"] = startsAt
	}
	if endsAt != "" {
		input["endsAt"] = endsAt
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: CycleUpdateInput!) {
		cycleUpdate(id: $id, input: $input) {
			cycle { id name number startsAt endsAt }
		}
	}`, map[string]any{"id": id, "input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Labels ────────────────────────────────────────────────────────

func listLabels(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	team, err := mcp.ArgStr(args, "team")
	if err != nil {
		return mcp.ErrResult(err)
	}

	filter := map[string]any{}
	if team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": teamID}}
	}

	vars := map[string]any{
		"first": mcp.OptInt(args, "first", 100),
	}
	if len(filter) > 0 {
		vars["filter"] = filter
	}

	data, err := l.gql(ctx, `query($first: Int, $filter: IssueLabelFilter) {
		issueLabels(first: $first, filter: $filter) {
			nodes {
				id name color description isGroup
				parent { id name }
				team { id name key }
			}
		}
	}`, vars)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createLabel(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	color := r.Str("color")
	description := r.Str("description")
	team := r.Str("team")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"name": name,
	}
	if color != "" {
		input["color"] = color
	}
	if description != "" {
		input["description"] = description
	}
	if team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["teamId"] = teamID
	}

	data, err := l.gql(ctx, `mutation($input: IssueLabelCreateInput!) {
		issueLabelCreate(input: $input) {
			issueLabel { id name color description team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateLabel(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	color := r.Str("color")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{}
	if name != "" {
		input["name"] = name
	}
	if color != "" {
		input["color"] = color
	}
	if description != "" {
		input["description"] = description
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: IssueLabelUpdateInput!) {
		issueLabelUpdate(id: $id, input: $input) {
			issueLabel { id name color description }
		}
	}`, map[string]any{"id": id, "input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteLabel(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueLabelArchive(id: $id) { success }
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Workflow States ───────────────────────────────────────────────

func listWorkflowStates(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	team, err := mcp.ArgStr(args, "team")
	if err != nil {
		return mcp.ErrResult(err)
	}

	filter := map[string]any{}
	if team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": teamID}}
	}

	vars := map[string]any{
		"first": mcp.OptInt(args, "first", 50),
	}
	if len(filter) > 0 {
		vars["filter"] = filter
	}

	data, err := l.gql(ctx, `query($first: Int, $filter: WorkflowStateFilter) {
		workflowStates(first: $first, filter: $filter) {
			nodes {
				id name type color description position
				team { id name key }
			}
		}
	}`, vars)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createWorkflowState(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	name := r.Str("name")
	stateType := r.Str("type")
	color := r.Str("color")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	teamID, err := l.resolveTeamID(ctx, team)
	if err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"teamId": teamID,
		"name":   name,
		"type":   stateType,
		"color":  color,
	}
	if description != "" {
		input["description"] = description
	}

	data, err := l.gql(ctx, `mutation($input: WorkflowStateCreateInput!) {
		workflowStateCreate(input: $input) {
			workflowState { id name type color position team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Documents ─────────────────────────────────────────────────────

func listDocuments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	project, err := mcp.ArgStr(args, "project")
	if err != nil {
		return mcp.ErrResult(err)
	}

	filter := map[string]any{}
	if project != "" {
		filter["project"] = map[string]any{"name": map[string]any{"eqIgnoreCase": project}}
	}

	vars := map[string]any{
		"first": mcp.OptInt(args, "first", 50),
	}
	if len(filter) > 0 {
		vars["filter"] = filter
	}

	data, err := l.gql(ctx, `query($first: Int, $filter: DocumentFilter) {
		documents(first: $first, filter: $filter) {
			nodes {
				id title icon color content slugId
				createdAt updatedAt
				creator { id name }
				project { id name }
			}
			pageInfo { hasNextPage endCursor }
		}
	}`, vars)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchDocuments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	query, err := mcp.ArgStr(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($term: String!, $first: Int) {
		searchDocuments(term: $term, first: $first) {
			nodes {
				id title icon color content slugId
				createdAt updatedAt
				creator { id name }
				project { id name }
			}
		}
	}`, map[string]any{
		"term":  query,
		"first": mcp.OptInt(args, "first", 25),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDocument(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!) {
		document(id: $id) {
			id title icon color content slugId
			createdAt updatedAt
			creator { id name }
			project { id name }
		}
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDocument(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	content := r.Str("content")
	icon := r.Str("icon")
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"title": title,
	}
	if content != "" {
		input["content"] = content
	}
	if icon != "" {
		input["icon"] = icon
	}
	if project != "" {
		input["projectId"] = project
	}

	data, err := l.gql(ctx, `mutation($input: DocumentCreateInput!) {
		documentCreate(input: $input) {
			document { id title slugId createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDocument(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	title := r.Str("title")
	content := r.Str("content")
	icon := r.Str("icon")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{}
	if title != "" {
		input["title"] = title
	}
	if content != "" {
		input["content"] = content
	}
	if icon != "" {
		input["icon"] = icon
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: DocumentUpdateInput!) {
		documentUpdate(id: $id, input: $input) {
			document { id title slugId updatedAt }
		}
	}`, map[string]any{"id": id, "input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Initiatives ───────────────────────────────────────────────────

func listInitiatives(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($first: Int) {
		initiatives(first: $first) {
			nodes {
				id name description status icon color
				targetDate createdAt updatedAt
				owner { id name }
				projects { nodes { id name state } }
			}
			pageInfo { hasNextPage endCursor }
		}
	}`, map[string]any{"first": mcp.OptInt(args, "first", 50)})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getInitiative(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!) {
		initiative(id: $id) {
			id name description status icon color
			targetDate createdAt updatedAt
			owner { id name }
			projects { nodes { id name state progress } }
		}
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createInitiative(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	targetDate := r.Str("target_date")
	status := r.Str("status")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"name": name,
	}
	if description != "" {
		input["description"] = description
	}
	if targetDate != "" {
		input["targetDate"] = targetDate
	}
	if status != "" {
		input["status"] = status
	}

	data, err := l.gql(ctx, `mutation($input: InitiativeCreateInput!) {
		initiativeCreate(input: $input) {
			initiative { id name status createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateInitiative(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	description := r.Str("description")
	targetDate := r.Str("target_date")
	status := r.Str("status")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{}
	if name != "" {
		input["name"] = name
	}
	if description != "" {
		input["description"] = description
	}
	if targetDate != "" {
		input["targetDate"] = targetDate
	}
	if status != "" {
		input["status"] = status
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: InitiativeUpdateInput!) {
		initiativeUpdate(id: $id, input: $input) {
			initiative { id name status updatedAt }
		}
	}`, map[string]any{"id": id, "input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Favorites ─────────────────────────────────────────────────────

func listFavorites(ctx context.Context, l *linear, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `{
		favorites {
			nodes {
				id type sortOrder
				issue { id identifier title }
				project { id name }
				cycle { id name number }
				customView { id name }
				label { id name }
			}
		}
	}`, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createFavorite(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueID := r.Str("issue_id")
	projectID := r.Str("project_id")
	cycleID := r.Str("cycle_id")
	customViewID := r.Str("custom_view_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{}
	if issueID != "" {
		input["issueId"] = issueID
	}
	if projectID != "" {
		input["projectId"] = projectID
	}
	if cycleID != "" {
		input["cycleId"] = cycleID
	}
	if customViewID != "" {
		input["customViewId"] = customViewID
	}

	data, err := l.gql(ctx, `mutation($input: FavoriteCreateInput!) {
		favoriteCreate(input: $input) {
			favorite { id type }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteFavorite(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		favoriteDelete(id: $id) { success }
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Webhooks ──────────────────────────────────────────────────────

func listWebhooks(ctx context.Context, l *linear, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `{
		webhooks {
			nodes {
				id url label enabled allPublicTeams
				resourceTypes
				team { id name key }
				creator { id name }
				createdAt updatedAt
			}
		}
	}`, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createWebhook(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	url := r.Str("url")
	label := r.Str("label")
	resourceTypes := r.Str("resource_types")
	team := r.Str("team")
	allPublicTeams := r.Bool("all_public_teams")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"url": url,
	}
	if label != "" {
		input["label"] = label
	}
	if resourceTypes != "" {
		input["resourceTypes"] = strings.Split(resourceTypes, ",")
	}
	if team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["teamId"] = teamID
	}
	if allPublicTeams {
		input["allPublicTeams"] = true
	}

	data, err := l.gql(ctx, `mutation($input: WebhookCreateInput!) {
		webhookCreate(input: $input) {
			webhook { id url label enabled resourceTypes team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteWebhook(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		webhookDelete(id: $id) { success }
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Notifications ─────────────────────────────────────────────────

func listNotifications(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($first: Int) {
		notifications(first: $first) {
			nodes {
				... on IssueNotification {
					id type readAt createdAt
					issue { id identifier title }
					comment { id body }
					actor { id name }
				}
				... on ProjectNotification {
					id type readAt createdAt
					project { id name }
					projectUpdate { id health }
				}
			}
		}
	}`, map[string]any{"first": mcp.OptInt(args, "first", 50)})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Templates ─────────────────────────────────────────────────────

func listTemplates(ctx context.Context, l *linear, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `{
		templates {
			id name description type
			team { id name key }
			creator { id name }
		}
	}`, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Organization ──────────────────────────────────────────────────

func getOrganization(ctx context.Context, l *linear, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `{
		organization {
			id name urlKey logoUrl
			createdAt
			userCount
			allowedAuthServices
			samlEnabled scimEnabled
			subscription { type seats }
		}
	}`, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Custom Views ──────────────────────────────────────────────────

func listCustomViews(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($first: Int) {
		customViews(first: $first) {
			nodes {
				id name description icon color
				filterData
				shared
				creator { id name }
				team { id name key }
				createdAt updatedAt
			}
		}
	}`, map[string]any{"first": mcp.OptInt(args, "first", 50)})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCustomView(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	team := r.Str("team")
	filterState := r.Str("filter_state")
	filterAssignee := r.Str("filter_assignee")
	filterLabel := r.Str("filter_label")
	filterPriority := r.Int("filter_priority")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"name": name,
	}
	if description != "" {
		input["description"] = description
	}
	if team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["teamId"] = teamID
	}

	filterData := map[string]any{}
	if filterState != "" {
		names := strings.Split(filterState, ",")
		filterData["state"] = map[string]any{"name": map[string]any{"in": names}}
	}
	if filterAssignee != "" {
		if filterAssignee == "me" {
			filterData["assignee"] = map[string]any{"isMe": map[string]any{"eq": true}}
		} else {
			filterData["assignee"] = map[string]any{"name": map[string]any{"eqIgnoreCase": filterAssignee}}
		}
	}
	if filterLabel != "" {
		filterData["labels"] = map[string]any{"name": map[string]any{"eqIgnoreCase": filterLabel}}
	}
	if filterPriority > 0 {
		filterData["priority"] = map[string]any{"eq": filterPriority}
	}
	if len(filterData) > 0 {
		input["filterData"] = filterData
	}

	data, err := l.gql(ctx, `mutation($input: CustomViewCreateInput!) {
		customViewCreate(input: $input) {
			customView { id name description shared }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Rate Limit ────────────────────────────────────────────────────

func rateLimitStatus(ctx context.Context, l *linear, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `{
		rateLimitStatus {
			identifier
			requestLimit
			remainingRequests
			payloadLimit
			remainingPayload
			reset
		}
	}`, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

var _ = fmt.Sprintf
