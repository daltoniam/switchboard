package forgejo

import (
	"fmt"
	"regexp"
	"time"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	mcp "github.com/daltoniam/switchboard"
)

func listIssues(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := sdk.ListIssueOption{ListOptions: pagination(r), Type: sdk.IssueTypeIssue, State: sdk.StateType(r.Str("state")), Labels: r.StrSlice("labels"), KeyWord: r.Str("query")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if opts.State == "" {
		opts.State = sdk.StateOpen
	}
	return finish(c.ListRepoIssues(owner, repo, opts))
}

func getIssue(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetIssue(owner, repo, number))
}

func createIssue(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := sdk.CreateIssueOption{Title: r.Str("title"), Body: r.Str("body"), Assignees: r.StrSlice("assignees"), Milestone: r.Int64("milestone"), Closed: r.Bool("closed")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	labels, err := labelIDs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts.Labels = labels
	return finish(c.CreateIssue(owner, repo, opts))
}

func updateIssue(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	body, state := r.Str("body"), sdk.StateType(r.Str("state"))
	opts := sdk.EditIssueOption{Title: r.Str("title"), Assignees: r.StrSlice("assignees")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, ok := args["body"]; ok {
		opts.Body = &body
	}
	if _, ok := args["state"]; ok {
		opts.State = &state
	}
	return finish(c.EditIssue(owner, repo, number, opts))
}

var commentTimestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]([01]\d|2[0-3]):[0-5]\d)$`)

func listIssueComments(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	since, before := r.Str("since"), r.Str("before")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := sdk.ListIssueCommentOptions{ListOptions: sdk.ListOptions{Page: -1}}
	for _, filter := range []struct {
		key, raw string
		target   *time.Time
	}{
		{"since", since, &opts.Since},
		{"before", before, &opts.Before},
	} {
		if _, ok := args[filter.key]; !ok {
			continue
		}
		value, err := time.Parse(time.RFC3339, filter.raw)
		if err != nil || value.IsZero() || !commentTimestampPattern.MatchString(filter.raw) {
			return mcp.ErrResult(fmt.Errorf("forgejo: %s must be a nonzero whole-second RFC3339 timestamp with timezone; fractional seconds are not supported by the SDK", filter.key))
		}
		*filter.target = value
	}
	if !opts.Since.IsZero() && !opts.Before.IsZero() && !opts.Since.Before(opts.Before) {
		return mcp.ErrResult(fmt.Errorf("forgejo: since must be before before"))
	}
	return finish(c.ListIssueComments(owner, repo, number, opts))
}

func createIssueComment(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, number := r.Str("owner"), r.Str("repo"), r.Int64("number")
	opts := sdk.CreateIssueCommentOption{Body: r.Str("body")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.CreateIssueComment(owner, repo, number, opts))
}
