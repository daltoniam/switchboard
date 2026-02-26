package linear

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

const projectFields = `
	id name description url state slugId color icon
	startDate targetDate createdAt updatedAt
	lead { id name email }
	teams { nodes { id name key } }
	members { nodes { id name } }
	progress completedIssueCountHistory issueCountHistory
`

func listProjects(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	filter := map[string]any{}
	if state := argStr(args, "state"); state != "" {
		filter["state"] = map[string]any{"eq": state}
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

	data, err := l.gql(ctx, fmt.Sprintf(`query($first: Int, $after: String, $filter: ProjectFilter) {
		projects(first: $first, after: $after, filter: $filter, orderBy: updatedAt) {
			nodes { %s }
			pageInfo { hasNextPage endCursor }
		}
	}`, projectFields), vars)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func searchProjects(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, fmt.Sprintf(`query($query: String!, $first: Int) {
		searchProjects(term: $query, first: $first) {
			nodes { %s }
		}
	}`, projectFields), map[string]any{
		"query": argStr(args, "query"),
		"first": optInt(args, "first", 50),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, fmt.Sprintf(`query($id: String!) {
		project(id: $id) { %s
			projectUpdates(first: 5) {
				nodes { id body health createdAt user { name } }
			}
			projectMilestones { nodes { id name targetDate } }
		}
	}`, projectFields), map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	teamIDs := []string{}
	if team := argStr(args, "team"); team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return errResult(err)
		}
		teamIDs = append(teamIDs, teamID)
	}

	input := map[string]any{
		"name":    argStr(args, "name"),
		"teamIds": teamIDs,
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argStr(args, "state"); v != "" {
		input["state"] = v
	}
	if v := argStr(args, "target_date"); v != "" {
		input["targetDate"] = v
	}
	if v := argStr(args, "start_date"); v != "" {
		input["startDate"] = v
	}

	data, err := l.gql(ctx, `mutation($input: ProjectCreateInput!) {
		projectCreate(input: $input) {
			project { id name url state }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		input["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argStr(args, "state"); v != "" {
		input["state"] = v
	}
	if v := argStr(args, "target_date"); v != "" {
		input["targetDate"] = v
	}
	if v := argStr(args, "start_date"); v != "" {
		input["startDate"] = v
	}

	data, err := l.gql(ctx, fmt.Sprintf(`mutation($id: String!, $input: ProjectUpdateInput!) {
		projectUpdate(id: $id, input: $input) {
			project { %s }
		}
	}`, projectFields), map[string]any{"id": argStr(args, "id"), "input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func archiveProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `mutation($id: String!) {
		projectArchive(id: $id) { success }
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listProjectUpdates(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($id: String!, $first: Int) {
		project(id: $id) {
			projectUpdates(first: $first) {
				nodes { id body health createdAt updatedAt user { id name } }
			}
		}
	}`, map[string]any{
		"id":    argStr(args, "project_id"),
		"first": optInt(args, "first", 10),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createProjectUpdate(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{
		"projectId": argStr(args, "project_id"),
		"body":      argStr(args, "body"),
	}
	if v := argStr(args, "health"); v != "" {
		input["health"] = v
	}
	data, err := l.gql(ctx, `mutation($input: ProjectUpdateCreateInput!) {
		projectUpdateCreate(input: $input) {
			projectUpdate { id body health createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listProjectMilestones(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($id: String!, $first: Int) {
		project(id: $id) {
			projectMilestones(first: $first) {
				nodes { id name description targetDate sortOrder }
			}
		}
	}`, map[string]any{
		"id":    argStr(args, "project_id"),
		"first": optInt(args, "first", 50),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createProjectMilestone(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{
		"projectId": argStr(args, "project_id"),
		"name":      argStr(args, "name"),
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argStr(args, "target_date"); v != "" {
		input["targetDate"] = v
	}

	data, err := l.gql(ctx, `mutation($input: ProjectMilestoneCreateInput!) {
		projectMilestoneCreate(input: $input) {
			projectMilestone { id name targetDate }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
