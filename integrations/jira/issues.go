package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// standardFields are the Jira field keys managed by explicit tool parameters.
// Custom fields passed via custom_fields must not collide with these to prevent
// overwriting already-formatted values (e.g. priority as {"name":"High"}).
var standardFields = map[string]bool{
	"project": true, "issuetype": true, "summary": true,
	"description": true, "priority": true, "assignee": true,
	"labels": true, "parent": true,
}

func searchIssues(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"jql": r.Str("jql"),
	}
	if v := r.Str("fields"); v != "" {
		rawFields := strings.Split(v, ",")
		for i, f := range rawFields {
			rawFields[i] = strings.TrimSpace(f)
		}
		body["fields"] = rawFields
	} else {
		body["fields"] = []string{"summary", "status", "assignee", "priority", "issuetype"}
	}
	if v := r.Str("next_page_token"); v != "" {
		body["nextPageToken"] = v
	}
	if v := r.Int("max_results"); v > 0 {
		body["maxResults"] = v
	} else {
		body["maxResults"] = 200
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := j.post(ctx, "/search/jql", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Str("fields"); v != "" {
		params["fields"] = v
	}
	if v := r.Str("expand"); v != "" {
		params["expand"] = v
	}
	q := queryEncode(params)
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/issue/%s%s", url.PathEscape(issueKey), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fields := map[string]any{
		"project":   map[string]string{"key": r.Str("project_key")},
		"issuetype": map[string]string{"name": r.Str("issue_type")},
		"summary":   r.Str("summary"),
	}
	if v := r.Str("description"); v != "" {
		fields["description"] = textToADF(v)
	}
	if v := r.Str("priority"); v != "" {
		fields["priority"] = map[string]string{"name": v}
	}
	if v := r.Str("assignee_id"); v != "" {
		fields["assignee"] = map[string]string{"accountId": v}
	}
	if v := r.Str("labels"); v != "" {
		rawLabels := strings.Split(v, ",")
		for i, l := range rawLabels {
			rawLabels[i] = strings.TrimSpace(l)
		}
		fields["labels"] = rawLabels
	}
	if v := r.Str("parent_key"); v != "" {
		fields["parent"] = map[string]string{"key": v}
	}
	if cfStr := r.Str("custom_fields"); cfStr != "" {
		var cf map[string]any
		if err := json.Unmarshal([]byte(cfStr), &cf); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for custom_fields: %w", err))
		}
		for k, v := range cf {
			if !standardFields[k] {
				fields[k] = v
			}
		}
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := j.post(ctx, "/issue", map[string]any{"fields": fields})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fields := map[string]any{}
	if v := r.Str("summary"); v != "" {
		fields["summary"] = v
	}
	if v := r.Str("description"); v != "" {
		fields["description"] = textToADF(v)
	}
	if v := r.Str("priority"); v != "" {
		fields["priority"] = map[string]string{"name": v}
	}
	if _, ok := args["assignee_id"]; ok {
		v := r.Str("assignee_id")
		if v == "" {
			fields["assignee"] = nil
		} else {
			fields["assignee"] = map[string]string{"accountId": v}
		}
	}
	if v := r.Str("labels"); v != "" {
		rawLabels := strings.Split(v, ",")
		for i, l := range rawLabels {
			rawLabels[i] = strings.TrimSpace(l)
		}
		fields["labels"] = rawLabels
	}
	if cfStr := r.Str("custom_fields"); cfStr != "" {
		var cf map[string]any
		if err := json.Unmarshal([]byte(cfStr), &cf); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for custom_fields: %w", err))
		}
		for k, v := range cf {
			if !standardFields[k] {
				fields[k] = v
			}
		}
	}
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	path := fmt.Sprintf("/issue/%s", url.PathEscape(issueKey))
	data, err := j.put(ctx, path, map[string]any{"fields": fields})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := ""
	if r.Bool("delete_subtasks") {
		q = "?deleteSubtasks=true"
	}
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.del(ctx, "/issue/%s%s", url.PathEscape(issueKey), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func transitionIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	transitionID := r.Str("transition_id")
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	path := fmt.Sprintf("/issue/%s/transitions", url.PathEscape(issueKey))
	data, err := j.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTransitions(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/issue/%s/transitions", url.PathEscape(issueKey))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func assignIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	accountID := r.Str("account_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body any
	if accountID == "" || accountID == "-1" {
		body = map[string]any{"accountId": nil}
	} else {
		body = map[string]string{"accountId": accountID}
	}
	path := fmt.Sprintf("/issue/%s/assignee", url.PathEscape(r.Str("issue_key")))
	data, err := j.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listComments(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	q := queryEncode(params)
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/issue/%s/comment%s", url.PathEscape(issueKey), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addComment(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	commentBody := r.Str("body")
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"body": textToADF(commentBody)}
	path := fmt.Sprintf("/issue/%s/comment", url.PathEscape(issueKey))
	data, err := j.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateComment(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	commentBody := r.Str("body")
	issueKey := r.Str("issue_key")
	commentID := r.Str("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"body": textToADF(commentBody)}
	path := fmt.Sprintf("/issue/%s/comment/%s", url.PathEscape(issueKey), url.PathEscape(commentID))
	data, err := j.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteComment(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueKey := r.Str("issue_key")
	commentID := r.Str("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.del(ctx, "/issue/%s/comment/%s", url.PathEscape(issueKey), url.PathEscape(commentID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueLinks(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"fields": "issuelinks"})
	data, err := j.get(ctx, "/issue/%s%s", url.PathEscape(issueKey), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	// Extract just the issuelinks field from the response.
	var issue struct {
		Fields struct {
			IssueLinks json.RawMessage `json:"issuelinks"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &issue); err != nil {
		return mcp.ErrResult(err)
	}
	if issue.Fields.IssueLinks == nil || string(issue.Fields.IssueLinks) == "null" {
		return mcp.RawResult(json.RawMessage(`[]`))
	}
	return mcp.RawResult(issue.Fields.IssueLinks)
}

func createIssueLink(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	typeName := r.Str("type_name")
	inwardIssue := r.Str("inward_issue")
	outwardIssue := r.Str("outward_issue")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"type":         map[string]string{"name": typeName},
		"inwardIssue":  map[string]string{"key": inwardIssue},
		"outwardIssue": map[string]string{"key": outwardIssue},
	}
	data, err := j.post(ctx, "/issueLink", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIssueLink(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	linkID := r.Str("link_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.del(ctx, "/issueLink/%s", url.PathEscape(linkID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
