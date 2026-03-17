package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func searchIssues(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"jql": argStr(args, "jql"),
	}
	if v := argStr(args, "fields"); v != "" {
		body["fields"] = strings.Split(v, ",")
	} else {
		body["fields"] = []string{"summary", "status", "assignee", "priority", "issuetype"}
	}
	if v := argInt(args, "start_at"); v > 0 {
		body["startAt"] = v
	}
	if v := argInt(args, "max_results"); v > 0 {
		body["maxResults"] = v
	} else {
		body["maxResults"] = 50
	}

	data, err := j.post(ctx, "/search", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argStr(args, "fields"); v != "" {
		params["fields"] = v
	}
	if v := argStr(args, "expand"); v != "" {
		params["expand"] = v
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/issue/%s%s", url.PathEscape(argStr(args, "issue_key")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	fields := map[string]any{
		"project":   map[string]string{"key": argStr(args, "project_key")},
		"issuetype": map[string]string{"name": argStr(args, "issue_type")},
		"summary":   argStr(args, "summary"),
	}
	if v := argStr(args, "description"); v != "" {
		fields["description"] = textToADF(v)
	}
	if v := argStr(args, "priority"); v != "" {
		fields["priority"] = map[string]string{"name": v}
	}
	if v := argStr(args, "assignee_id"); v != "" {
		fields["assignee"] = map[string]string{"accountId": v}
	}
	if v := argStr(args, "labels"); v != "" {
		rawLabels := strings.Split(v, ",")
		for i, l := range rawLabels {
			rawLabels[i] = strings.TrimSpace(l)
		}
		fields["labels"] = rawLabels
	}
	if v := argStr(args, "parent_key"); v != "" {
		fields["parent"] = map[string]string{"key": v}
	}

	data, err := j.post(ctx, "/issue", map[string]any{"fields": fields})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	fields := map[string]any{}
	if v := argStr(args, "summary"); v != "" {
		fields["summary"] = v
	}
	if v := argStr(args, "description"); v != "" {
		fields["description"] = textToADF(v)
	}
	if v := argStr(args, "priority"); v != "" {
		fields["priority"] = map[string]string{"name": v}
	}
	if _, ok := args["assignee_id"]; ok {
		v := argStr(args, "assignee_id")
		if v == "" {
			fields["assignee"] = nil
		} else {
			fields["assignee"] = map[string]string{"accountId": v}
		}
	}
	if v := argStr(args, "labels"); v != "" {
		rawLabels := strings.Split(v, ",")
		for i, l := range rawLabels {
			rawLabels[i] = strings.TrimSpace(l)
		}
		fields["labels"] = rawLabels
	}

	path := fmt.Sprintf("/issue/%s", url.PathEscape(argStr(args, "issue_key")))
	data, err := j.put(ctx, path, map[string]any{"fields": fields})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	q := ""
	if argBool(args, "delete_subtasks") {
		q = "?deleteSubtasks=true"
	}
	data, err := j.del(ctx, "/issue/%s%s", url.PathEscape(argStr(args, "issue_key")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func transitionIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"transition": map[string]string{"id": argStr(args, "transition_id")},
	}
	path := fmt.Sprintf("/issue/%s/transitions", url.PathEscape(argStr(args, "issue_key")))
	data, err := j.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTransitions(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/issue/%s/transitions", url.PathEscape(argStr(args, "issue_key")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func assignIssue(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	accountID := argStr(args, "account_id")
	var body any
	if accountID == "" || accountID == "-1" {
		body = map[string]any{"accountId": nil}
	} else {
		body = map[string]string{"accountId": accountID}
	}
	path := fmt.Sprintf("/issue/%s/assignee", url.PathEscape(argStr(args, "issue_key")))
	data, err := j.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listComments(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/issue/%s/comment%s", url.PathEscape(argStr(args, "issue_key")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addComment(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"body": textToADF(argStr(args, "body"))}
	path := fmt.Sprintf("/issue/%s/comment", url.PathEscape(argStr(args, "issue_key")))
	data, err := j.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateComment(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"body": textToADF(argStr(args, "body"))}
	path := fmt.Sprintf("/issue/%s/comment/%s", url.PathEscape(argStr(args, "issue_key")), url.PathEscape(argStr(args, "comment_id")))
	data, err := j.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteComment(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.del(ctx, "/issue/%s/comment/%s", url.PathEscape(argStr(args, "issue_key")), url.PathEscape(argStr(args, "comment_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueLinks(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/issue/%s?fields=issuelinks", url.PathEscape(argStr(args, "issue_key")))
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
	body := map[string]any{
		"type":         map[string]string{"name": argStr(args, "type_name")},
		"inwardIssue":  map[string]string{"key": argStr(args, "inward_issue")},
		"outwardIssue": map[string]string{"key": argStr(args, "outward_issue")},
	}
	data, err := j.post(ctx, "/issueLink", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIssueLink(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.del(ctx, "/issueLink/%s", url.PathEscape(argStr(args, "link_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
