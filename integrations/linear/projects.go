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
	r := mcp.NewArgs(args)
	state := r.Str("state")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	filter := map[string]any{}
	if state != "" {
		filter["state"] = map[string]any{"eq": state}
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

	data, err := l.gql(ctx, fmt.Sprintf(`query($first: Int, $after: String, $filter: ProjectFilter) {
		projects(first: $first, after: $after, filter: $filter, orderBy: updatedAt) {
			nodes { %s }
			pageInfo { hasNextPage endCursor }
		}
	}`, projectFields), vars)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchProjects(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	query, err := mcp.ArgStr(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, fmt.Sprintf(`query($term: String!, $first: Int) {
		searchProjects(term: $term, first: $first) {
			nodes { %s }
		}
	}`, projectFields), map[string]any{
		"term":  query,
		"first": mcp.OptInt(args, "first", 50),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, fmt.Sprintf(`query($id: String!) {
		project(id: $id) { %s
			projectUpdates(first: 5) {
				nodes { id body health createdAt user { name } }
			}
			projectMilestones { nodes { id name targetDate } }
		}
	}`, projectFields), map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	name := r.Str("name")
	description := r.Str("description")
	state := r.Str("state")
	targetDate := r.Str("target_date")
	startDate := r.Str("start_date")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	teamIDs := []string{}
	if team != "" {
		teamID, err := l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		teamIDs = append(teamIDs, teamID)
	}

	input := map[string]any{
		"name":    name,
		"teamIds": teamIDs,
	}
	if description != "" {
		input["description"] = description
	}
	if state != "" {
		input["state"] = state
	}
	if targetDate != "" {
		input["targetDate"] = targetDate
	}
	if startDate != "" {
		input["startDate"] = startDate
	}

	data, err := l.gql(ctx, `mutation($input: ProjectCreateInput!) {
		projectCreate(input: $input) {
			project { id name url state }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	description := r.Str("description")
	state := r.Str("state")
	targetDate := r.Str("target_date")
	startDate := r.Str("start_date")
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
	if state != "" {
		input["state"] = state
	}
	if targetDate != "" {
		input["targetDate"] = targetDate
	}
	if startDate != "" {
		input["startDate"] = startDate
	}

	data, err := l.gql(ctx, fmt.Sprintf(`mutation($id: String!, $input: ProjectUpdateInput!) {
		projectUpdate(id: $id, input: $input) {
			project { %s }
		}
	}`, projectFields), map[string]any{"id": id, "input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func archiveProject(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		projectArchive(id: $id) { success }
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectUpdates(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	projectID, err := mcp.ArgStr(args, "project_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!, $first: Int) {
		project(id: $id) {
			projectUpdates(first: $first) {
				nodes { id body health createdAt updatedAt user { id name } }
			}
		}
	}`, map[string]any{
		"id":    projectID,
		"first": mcp.OptInt(args, "first", 10),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProjectUpdate(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	body := r.Str("body")
	health := r.Str("health")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	input := map[string]any{
		"projectId": projectID,
		"body":      body,
	}
	if health != "" {
		input["health"] = health
	}
	data, err := l.gql(ctx, `mutation($input: ProjectUpdateCreateInput!) {
		projectUpdateCreate(input: $input) {
			projectUpdate { id body health createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectMilestones(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	projectID, err := mcp.ArgStr(args, "project_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!, $first: Int) {
		project(id: $id) {
			projectMilestones(first: $first) {
				nodes { id name description targetDate sortOrder }
			}
		}
	}`, map[string]any{
		"id":    projectID,
		"first": mcp.OptInt(args, "first", 50),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProjectMilestone(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	name := r.Str("name")
	description := r.Str("description")
	targetDate := r.Str("target_date")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	input := map[string]any{
		"projectId": projectID,
		"name":      name,
	}
	if description != "" {
		input["description"] = description
	}
	if targetDate != "" {
		input["targetDate"] = targetDate
	}

	data, err := l.gql(ctx, `mutation($input: ProjectMilestoneCreateInput!) {
		projectMilestoneCreate(input: $input) {
			projectMilestone { id name targetDate }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
