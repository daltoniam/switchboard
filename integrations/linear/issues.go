package linear

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

const issueFields = `
	id identifier title description url priority estimate
	createdAt updatedAt dueDate
	state { id name type color }
	assignee { id name email }
	team { id name key }
	project { id name }
	cycle { id name number }
	labels { nodes { id name color } }
	parent { id identifier title }
	children { nodes { id identifier title } }
`

func listIssues(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	filter := map[string]any{}
	if team := argStr(args, "team"); team != "" {
		filter["team"] = map[string]any{"or": []map[string]any{
			{"name": map[string]any{"eqIgnoreCase": team}},
			{"key": map[string]any{"eqIgnoreCase": team}},
		}}
	}
	if assignee := argStr(args, "assignee"); assignee != "" {
		if assignee == "me" {
			filter["assignee"] = map[string]any{"isMe": map[string]any{"eq": true}}
		} else {
			filter["assignee"] = map[string]any{"name": map[string]any{"eqIgnoreCase": assignee}}
		}
	}
	if state := argStr(args, "state"); state != "" {
		filter["state"] = map[string]any{"name": map[string]any{"eqIgnoreCase": state}}
	}
	if label := argStr(args, "label"); label != "" {
		filter["labels"] = map[string]any{"name": map[string]any{"eqIgnoreCase": label}}
	}
	if priority := argInt(args, "priority"); priority > 0 {
		filter["priority"] = map[string]any{"eq": priority}
	}
	if project := argStr(args, "project"); project != "" {
		filter["project"] = map[string]any{"name": map[string]any{"eqIgnoreCase": project}}
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

	data, err := l.gql(ctx, fmt.Sprintf(`query($first: Int, $after: String, $filter: IssueFilter) {
		issues(first: $first, after: $after, filter: $filter, orderBy: updatedAt) {
			nodes { %s }
			pageInfo { hasNextPage endCursor }
		}
	}`, issueFields), vars)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func searchIssues(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, fmt.Sprintf(`query($term: String!, $first: Int, $after: String) {
		searchIssues(term: $term, first: $first, after: $after) {
			nodes { %s }
			pageInfo { hasNextPage endCursor }
		}
	}`, issueFields), map[string]any{
		"term":  argStr(args, "query"),
		"first": optInt(args, "first", 50),
		"after": argStr(args, "after"),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	data, err := l.gql(ctx, fmt.Sprintf(`query($id: String!) {
		issue(id: $id) {
			%s
			comments { nodes { id body user { name } createdAt } }
		}
	}`, issueFields), map[string]any{"id": id})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
	teamID, err := l.resolveTeamID(ctx, argStr(args, "team"))
	if err != nil {
		return errResult(err)
	}

	input := map[string]any{
		"title":  argStr(args, "title"),
		"teamId": teamID,
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argInt(args, "priority"); v > 0 {
		input["priority"] = v
	}
	if v := argInt(args, "estimate"); v > 0 {
		input["estimate"] = v
	}
	if v := argStr(args, "due_date"); v != "" {
		input["dueDate"] = v
	}
	if v := argStr(args, "parent_id"); v != "" {
		input["parentId"] = v
	}

	data, err := l.gql(ctx, `mutation($input: IssueCreateInput!) {
		issueCreate(input: $input) {
			issue { id identifier title url state { name } }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}

	input := map[string]any{}
	if v := argStr(args, "title"); v != "" {
		input["title"] = v
	}
	if v := argStr(args, "description"); v != "" {
		input["description"] = v
	}
	if v := argInt(args, "priority"); v >= 0 {
		if _, ok := args["priority"]; ok {
			input["priority"] = v
		}
	}
	if v := argInt(args, "estimate"); v > 0 {
		input["estimate"] = v
	}
	if v := argStr(args, "due_date"); v != "" {
		input["dueDate"] = v
	}

	data, err := l.gql(ctx, fmt.Sprintf(`mutation($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) {
			issue { %s }
		}
	}`, issueFields), map[string]any{"id": issueID, "input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func archiveIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueArchive(id: $id) { success }
	}`, map[string]any{"id": issueID})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func unarchiveIssue(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueUnarchive(id: $id) { success }
	}`, map[string]any{"id": issueID})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listIssueComments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
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
	}`, map[string]any{"id": issueID, "first": optInt(args, "first", 50)})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createComment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "issue_id"))
	if err != nil {
		return errResult(err)
	}
	data, err := l.gql(ctx, `mutation($input: CommentCreateInput!) {
		commentCreate(input: $input) {
			comment { id body createdAt user { name } }
		}
	}`, map[string]any{
		"input": map[string]any{
			"issueId": issueID,
			"body":    argStr(args, "body"),
		},
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateComment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `mutation($id: String!, $input: CommentUpdateInput!) {
		commentUpdate(id: $id, input: $input) {
			comment { id body updatedAt }
		}
	}`, map[string]any{
		"id":    argStr(args, "id"),
		"input": map[string]any{"body": argStr(args, "body")},
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteComment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `mutation($id: String!) {
		commentDelete(id: $id) { success }
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listIssueRelations(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!) {
		issue(id: $id) {
			relations { nodes { id type relatedIssue { id identifier title } } }
			inverseRelations { nodes { id type issue { id identifier title } } }
		}
	}`, map[string]any{"id": issueID})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createIssueRelation(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "issue_id"))
	if err != nil {
		return errResult(err)
	}
	relatedID, err := l.resolveIssueID(ctx, argStr(args, "related_issue_id"))
	if err != nil {
		return errResult(err)
	}
	relType := argStr(args, "type")
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
		return errResult(err)
	}
	return rawResult(data)
}

func deleteIssueRelation(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `mutation($id: String!) {
		issueRelationDelete(id: $id) { success }
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listIssueLabels(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	data, err := l.gql(ctx, `query($id: String!) {
		issue(id: $id) {
			labels { nodes { id name color description } }
		}
	}`, map[string]any{"id": issueID})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listAttachments(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "issue_id"))
	if err != nil {
		return errResult(err)
	}
	data, err := l.gql(ctx, `query($filter: AttachmentFilter, $first: Int) {
		attachments(filter: $filter, first: $first) {
			nodes { id title url subtitle source metadata createdAt }
		}
	}`, map[string]any{
		"filter": map[string]any{"issue": map[string]any{"id": map[string]any{"eq": issueID}}},
		"first":  optInt(args, "first", 25),
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createAttachment(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	issueID, err := l.resolveIssueID(ctx, argStr(args, "issue_id"))
	if err != nil {
		return errResult(err)
	}
	input := map[string]any{
		"issueId": issueID,
		"url":     argStr(args, "url"),
	}
	if v := argStr(args, "title"); v != "" {
		input["title"] = v
	}
	if v := argStr(args, "subtitle"); v != "" {
		input["subtitle"] = v
	}
	data, err := l.gql(ctx, `mutation($input: AttachmentCreateInput!) {
		attachmentCreate(input: $input) {
			attachment { id title url createdAt }
		}
	}`, map[string]any{"input": input})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// unused but keeps import
var _ = (*mcp.ToolResult)(nil)
