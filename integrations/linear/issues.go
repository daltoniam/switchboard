package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

const issueFields = `
	id identifier title description url priority estimate
	createdAt updatedAt dueDate
	state { id name type color }
	assignee { id name email }
	team { id name key }
	project { id name }
	projectMilestone { id name }
	cycle { id name number }
	labels { nodes { id name color } }
	parent { id identifier title }
	children { nodes { id identifier title } }
`

func listIssues(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	assignee := r.Str("assignee")
	state := r.Str("state")
	label := r.Str("label")
	priority := r.Int("priority")
	project := r.Str("project")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	filter := map[string]any{}
	if team != "" {
		filter["team"] = map[string]any{"or": []map[string]any{
			{"name": map[string]any{"eqIgnoreCase": team}},
			{"key": map[string]any{"eqIgnoreCase": team}},
		}}
	}
	if assignee != "" {
		if assignee == "me" {
			filter["assignee"] = map[string]any{"isMe": map[string]any{"eq": true}}
		} else {
			filter["assignee"] = map[string]any{"name": map[string]any{"eqIgnoreCase": assignee}}
		}
	}
	if state != "" {
		filter["state"] = map[string]any{"name": map[string]any{"eqIgnoreCase": state}}
	}
	if label != "" {
		filter["labels"] = map[string]any{"name": map[string]any{"eqIgnoreCase": label}}
	}
	if priority > 0 {
		filter["priority"] = map[string]any{"eq": priority}
	}
	if project != "" {
		filter["project"] = map[string]any{"name": map[string]any{"eqIgnoreCase": project}}
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

	data, err := l.gql(ctx, fmt.Sprintf(`query($first: Int, $after: String, $filter: IssueFilter) {
		issues(first: $first, after: $after, filter: $filter, orderBy: updatedAt) {
			nodes { %s }
			pageInfo { hasNextPage endCursor }
		}
	}`, issueFields), vars)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchIssues(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, fmt.Sprintf(`query($term: String!, $first: Int, $after: String) {
		searchIssues(term: $term, first: $first, after: $after) {
			nodes { %s }
			pageInfo { hasNextPage endCursor }
		}
	}`, issueFields), map[string]any{
		"term":  query,
		"first": mcp.OptInt(args, "first", 50),
		"after": after,
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, fmt.Sprintf(`query($id: String!) {
		issue(id: $id) {
			%s
			comments { nodes { id body user { name } createdAt } }
		}
	}`, issueFields), map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// resolveIssueID resolves an identifier (ENG-123) to a UUID.
func (l *linear) resolveIssueID(ctx context.Context, idOrIdentifier string) (string, error) {
	data, err := l.gql(ctx, `query($id: String!) {
		issue(id: $id) { id }
	}`, map[string]any{"id": idOrIdentifier})
	if err != nil {
		return "", err
	}
	var resp struct {
		Issue struct {
			ID string `json:"id"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || resp.Issue.ID == "" {
		return "", fmt.Errorf("issue not found: %s", idOrIdentifier)
	}
	return resp.Issue.ID, nil
}

func createIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	title := r.Str("title")
	description := r.Str("description")
	priority := r.Int("priority")
	estimate := r.Int("estimate")
	dueDate := r.Str("due_date")
	parentID := r.Str("parent_id")
	state := r.Str("state")
	project := r.Str("project")
	assignee := r.Str("assignee")
	labels := r.Str("labels")
	milestone := r.Str("milestone")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	teamID, err := l.resolveTeamID(ctx, team)
	if err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{
		"title":  title,
		"teamId": teamID,
	}
	if description != "" {
		input["description"] = description
	}
	if priority > 0 {
		input["priority"] = priority
	}
	if estimate > 0 {
		input["estimate"] = estimate
	}
	if dueDate != "" {
		input["dueDate"] = dueDate
	}
	if parentID != "" {
		input["parentId"] = parentID
	}
	if state != "" {
		stateID, err := l.resolveStateID(ctx, state, teamID)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["stateId"] = stateID
	}
	var projectID string
	if project != "" {
		projectID, err = l.resolveProjectID(ctx, project)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["projectId"] = projectID
	}
	if milestone != "" {
		if projectID == "" {
			return mcp.ErrResult(fmt.Errorf("milestone requires a project"))
		}
		milestoneID, err := l.resolveMilestoneID(ctx, milestone, projectID)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["projectMilestoneId"] = milestoneID
	}
	if assignee != "" {
		userID, err := l.resolveUserID(ctx, assignee)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["assigneeId"] = userID
	}
	if labels != "" {
		labelIDs, err := l.resolveLabelIDs(ctx, strings.Split(labels, ","))
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["labelIds"] = labelIDs
	}

	data, err := l.gql(ctx, `mutation($input: IssueCreateInput!) {
		issueCreate(input: $input) {
			issue { id identifier title url state { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	title := r.Str("title")
	description := r.Str("description")
	priority := r.Int("priority")
	estimate := r.Int("estimate")
	dueDate := r.Str("due_date")
	team := r.Str("team")
	state := r.Str("state")
	project := r.Str("project")
	assignee := r.Str("assignee")
	labels := r.Str("labels")
	milestone := r.Str("milestone")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	issueID, err := l.resolveIssueID(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}

	input := map[string]any{}
	if title != "" {
		input["title"] = title
	}
	if description != "" {
		input["description"] = description
	}
	if priority >= 0 {
		if _, ok := args["priority"]; ok {
			input["priority"] = priority
		}
	}
	if estimate > 0 {
		input["estimate"] = estimate
	}
	if dueDate != "" {
		input["dueDate"] = dueDate
	}

	var teamID string
	if team != "" {
		teamID, err = l.resolveTeamID(ctx, team)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["teamId"] = teamID
	}
	if state != "" {
		if teamID == "" {
			teamID, err = l.resolveIssueTeamID(ctx, issueID)
			if err != nil {
				return mcp.ErrResult(err)
			}
		}
		stateID, err := l.resolveStateID(ctx, state, teamID)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["stateId"] = stateID
	}
	var projectID string
	if project != "" {
		projectID, err = l.resolveProjectID(ctx, project)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["projectId"] = projectID
	}
	if milestone != "" {
		if projectID == "" {
			projectID, err = l.resolveIssueProjectID(ctx, issueID)
			if err != nil {
				return mcp.ErrResult(err)
			}
		}
		milestoneID, err := l.resolveMilestoneID(ctx, milestone, projectID)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["projectMilestoneId"] = milestoneID
	}
	if assignee != "" {
		userID, err := l.resolveUserID(ctx, assignee)
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["assigneeId"] = userID
	}
	if labels != "" {
		labelIDs, err := l.resolveLabelIDs(ctx, strings.Split(labels, ","))
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["labelIds"] = labelIDs
	}

	data, err := l.gql(ctx, fmt.Sprintf(`mutation($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) {
			issue { %s }
		}
	}`, issueFields), map[string]any{"id": issueID, "input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func archiveIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueArchive(id: $id) { success }
	}`, map[string]any{"id": issueID})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unarchiveIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueUnarchive(id: $id) { success }
	}`, map[string]any{"id": issueID})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueComments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!, $first: Int) {
		issue(id: $id) {
			comments(first: $first) {
				nodes {
					id body createdAt updatedAt
					user { id name email }
				}
			}
		}
	}`, map[string]any{"id": issueID, "first": mcp.OptInt(args, "first", 50)})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createComment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueIDRaw := r.Str("issue_id")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, issueIDRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($input: CommentCreateInput!) {
		commentCreate(input: $input) {
			comment { id body createdAt user { name } }
		}
	}`, map[string]any{
		"input": map[string]any{
			"issueId": issueID,
			"body":    body,
		},
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateComment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!, $input: CommentUpdateInput!) {
		commentUpdate(id: $id, input: $input) {
			comment { id body updatedAt }
		}
	}`, map[string]any{
		"id":    id,
		"input": map[string]any{"body": body},
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteComment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		commentDelete(id: $id) { success }
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueRelations(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!) {
		issue(id: $id) {
			relations { nodes { id type relatedIssue { id identifier title } } }
			inverseRelations { nodes { id type issue { id identifier title } } }
		}
	}`, map[string]any{"id": issueID})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createIssueRelation(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueIDRaw := r.Str("issue_id")
	relatedIDRaw := r.Str("related_issue_id")
	relType := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, issueIDRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	relatedID, err := l.resolveIssueID(ctx, relatedIDRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($input: IssueRelationCreateInput!) {
		issueRelationCreate(input: $input) {
			issueRelation { id type issue { identifier } relatedIssue { identifier } }
		}
	}`, map[string]any{
		"input": map[string]any{
			"issueId":        issueID,
			"relatedIssueId": relatedID,
			"type":           relType,
		},
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIssueRelation(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueRelationDelete(id: $id) { success }
	}`, map[string]any{"id": id})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueLabels(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!) {
		issue(id: $id) {
			labels { nodes { id name color description } }
		}
	}`, map[string]any{"id": issueID})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAttachments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueIDRaw, err := mcp.ArgStr(args, "issue_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, issueIDRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.gql(ctx, `query($filter: AttachmentFilter, $first: Int) {
		attachments(filter: $filter, first: $first) {
			nodes { id title url subtitle source metadata createdAt }
		}
	}`, map[string]any{
		"filter": map[string]any{"issue": map[string]any{"id": map[string]any{"eq": issueID}}},
		"first":  mcp.OptInt(args, "first", 25),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAttachment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueIDRaw := r.Str("issue_id")
	url := r.Str("url")
	title := r.Str("title")
	subtitle := r.Str("subtitle")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	issueID, err := l.resolveIssueID(ctx, issueIDRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	input := map[string]any{
		"issueId": issueID,
		"url":     url,
	}
	if title != "" {
		input["title"] = title
	}
	if subtitle != "" {
		input["subtitle"] = subtitle
	}
	data, err := l.gql(ctx, `mutation($input: AttachmentCreateInput!) {
		attachmentCreate(input: $input) {
			attachment { id title url createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// unused but keeps import
var _ = (*mcp.ToolResult)(nil)
