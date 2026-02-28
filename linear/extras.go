package linear

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// ── Cycles ────────────────────────────────────────────────────────

func listCycles(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	filter := map[string]any{}
	if team := argStr(args, "team"); team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return errResult(err)
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": teamID}}
	}

	vars := map[string]any{
		"first": optInt(args, "first", 50),
	}
	if len(filter) > 0 {
		vars["filter"] = filter
	}
	if after := argStr(args, "after"); after != "" {
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
		return errResult(err)
	}
	return rawResult(data)
}

func getCycle(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
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
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createCycle(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	teamID, err := l.resolveTeamID(ctx, argStr(args, "team"))
	if err != nil {
		return errResult(err)
	}

	input := map[string]any{
		"teamId":   teamID,
		"startsAt": argStr(args, "starts_at"),
		"endsAt":   argStr(args, "ends_at"),
	}
	if v := argStr(args, "name"); v != "" {
		input["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}

	data, err := l.gql(ctx, `mutation($input: CycleCreateInput!) {
		cycleCreate(input: $input) {
			cycle { id name number startsAt endsAt team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateCycle(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		input["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argStr(args, "starts_at"); v != "" {
		input["startsAt"] = v
	}
	if v := argStr(args, "ends_at"); v != "" {
		input["endsAt"] = v
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: CycleUpdateInput!) {
		cycleUpdate(id: $id, input: $input) {
			cycle { id name number startsAt endsAt }
		}
	}`, map[string]any{"id": argStr(args, "id"), "input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Labels ────────────────────────────────────────────────────────

func listLabels(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	filter := map[string]any{}
	if team := argStr(args, "team"); team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return errResult(err)
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": teamID}}
	}

	vars := map[string]any{
		"first": optInt(args, "first", 100),
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
		return errResult(err)
	}
	return rawResult(data)
}

func createLabel(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{
		"name": argStr(args, "name"),
	}
	if v := argStr(args, "color"); v != "" {
		input["color"] = v
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if team := argStr(args, "team"); team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return errResult(err)
		}
		input["teamId"] = teamID
	}

	data, err := l.gql(ctx, `mutation($input: IssueLabelCreateInput!) {
		issueLabelCreate(input: $input) {
			issueLabel { id name color description team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateLabel(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		input["name"] = v
	}
	if v := argStr(args, "color"); v != "" {
		input["color"] = v
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: IssueLabelUpdateInput!) {
		issueLabelUpdate(id: $id, input: $input) {
			issueLabel { id name color description }
		}
	}`, map[string]any{"id": argStr(args, "id"), "input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteLabel(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueLabelArchive(id: $id) { success }
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Workflow States ───────────────────────────────────────────────

func listWorkflowStates(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	filter := map[string]any{}
	if team := argStr(args, "team"); team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return errResult(err)
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": teamID}}
	}

	vars := map[string]any{
		"first": optInt(args, "first", 50),
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
		return errResult(err)
	}
	return rawResult(data)
}

func createWorkflowState(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	teamID, err := l.resolveTeamID(ctx, argStr(args, "team"))
	if err != nil {
		return errResult(err)
	}

	input := map[string]any{
		"teamId": teamID,
		"name":   argStr(args, "name"),
		"type":   argStr(args, "type"),
		"color":  argStr(args, "color"),
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}

	data, err := l.gql(ctx, `mutation($input: WorkflowStateCreateInput!) {
		workflowStateCreate(input: $input) {
			workflowState { id name type color position team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Documents ─────────────────────────────────────────────────────

func listDocuments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	filter := map[string]any{}
	if project := argStr(args, "project"); project != "" {
		filter["project"] = map[string]any{"name": map[string]any{"eqIgnoreCase": project}}
	}

	vars := map[string]any{
		"first": optInt(args, "first", 50),
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
		return errResult(err)
	}
	return rawResult(data)
}

func searchDocuments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
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
		"term":  argStr(args, "query"),
		"first": optInt(args, "first", 25),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getDocument(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($id: String!) {
		document(id: $id) {
			id title icon color content slugId
			createdAt updatedAt
			creator { id name }
			project { id name }
		}
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createDocument(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{
		"title": argStr(args, "title"),
	}
	if v := argStr(args, "content"); v != "" {
		input["content"] = v
	}
	if v := argStr(args, "icon"); v != "" {
		input["icon"] = v
	}
	if v := argStr(args, "project"); v != "" {
		input["projectId"] = v
	}

	data, err := l.gql(ctx, `mutation($input: DocumentCreateInput!) {
		documentCreate(input: $input) {
			document { id title slugId createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateDocument(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{}
	if v := argStr(args, "title"); v != "" {
		input["title"] = v
	}
	if v := argStr(args, "content"); v != "" {
		input["content"] = v
	}
	if v := argStr(args, "icon"); v != "" {
		input["icon"] = v
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: DocumentUpdateInput!) {
		documentUpdate(id: $id, input: $input) {
			document { id title slugId updatedAt }
		}
	}`, map[string]any{"id": argStr(args, "id"), "input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
	}`, map[string]any{"first": optInt(args, "first", 50)})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getInitiative(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($id: String!) {
		initiative(id: $id) {
			id name description status icon color
			targetDate createdAt updatedAt
			owner { id name }
			projects { nodes { id name state progress } }
		}
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createInitiative(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{
		"name": argStr(args, "name"),
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argStr(args, "target_date"); v != "" {
		input["targetDate"] = v
	}
	if v := argStr(args, "status"); v != "" {
		input["status"] = v
	}

	data, err := l.gql(ctx, `mutation($input: InitiativeCreateInput!) {
		initiativeCreate(input: $input) {
			initiative { id name status createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateInitiative(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		input["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argStr(args, "target_date"); v != "" {
		input["targetDate"] = v
	}
	if v := argStr(args, "status"); v != "" {
		input["status"] = v
	}

	data, err := l.gql(ctx, `mutation($id: String!, $input: InitiativeUpdateInput!) {
		initiativeUpdate(id: $id, input: $input) {
			initiative { id name status updatedAt }
		}
	}`, map[string]any{"id": argStr(args, "id"), "input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
}

func createFavorite(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{}
	if v := argStr(args, "issue_id"); v != "" {
		input["issueId"] = v
	}
	if v := argStr(args, "project_id"); v != "" {
		input["projectId"] = v
	}
	if v := argStr(args, "cycle_id"); v != "" {
		input["cycleId"] = v
	}
	if v := argStr(args, "custom_view_id"); v != "" {
		input["customViewId"] = v
	}

	data, err := l.gql(ctx, `mutation($input: FavoriteCreateInput!) {
		favoriteCreate(input: $input) {
			favorite { id type }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteFavorite(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `mutation($id: String!) {
		favoriteDelete(id: $id) { success }
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
}

func createWebhook(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{
		"url": argStr(args, "url"),
	}
	if v := argStr(args, "label"); v != "" {
		input["label"] = v
	}
	if v := argStr(args, "resource_types"); v != "" {
		input["resourceTypes"] = strings.Split(v, ",")
	}
	if team := argStr(args, "team"); team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return errResult(err)
		}
		input["teamId"] = teamID
	}
	if argBool(args, "all_public_teams") {
		input["allPublicTeams"] = true
	}

	data, err := l.gql(ctx, `mutation($input: WebhookCreateInput!) {
		webhookCreate(input: $input) {
			webhook { id url label enabled resourceTypes team { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteWebhook(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `mutation($id: String!) {
		webhookDelete(id: $id) { success }
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
	}`, map[string]any{"first": optInt(args, "first", 50)})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
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
	}`, map[string]any{"first": optInt(args, "first", 50)})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createCustomView(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{
		"name": argStr(args, "name"),
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if team := argStr(args, "team"); team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return errResult(err)
		}
		input["teamId"] = teamID
	}

	filterData := map[string]any{}
	if v := argStr(args, "filter_state"); v != "" {
		names := strings.Split(v, ",")
		filterData["state"] = map[string]any{"name": map[string]any{"in": names}}
	}
	if v := argStr(args, "filter_assignee"); v != "" {
		if v == "me" {
			filterData["assignee"] = map[string]any{"isMe": map[string]any{"eq": true}}
		} else {
			filterData["assignee"] = map[string]any{"name": map[string]any{"eqIgnoreCase": v}}
		}
	}
	if v := argStr(args, "filter_label"); v != "" {
		filterData["labels"] = map[string]any{"name": map[string]any{"eqIgnoreCase": v}}
	}
	if v := argInt(args, "filter_priority"); v > 0 {
		filterData["priority"] = map[string]any{"eq": v}
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
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
}

var _ = fmt.Sprintf
