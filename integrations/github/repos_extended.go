package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Repo Settings ─────────────────────────────────────────────────

func editRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := &gh.Repository{}
	if v := argStr(args, "description"); v != "" {
		r.Description = gh.Ptr(v)
	}
	if v := argStr(args, "homepage"); v != "" {
		r.Homepage = gh.Ptr(v)
	}
	if v := argStr(args, "default_branch"); v != "" {
		r.DefaultBranch = gh.Ptr(v)
	}
	if _, ok := args["private"]; ok {
		r.Private = gh.Ptr(argBool(args, "private"))
	}
	if _, ok := args["archived"]; ok {
		r.Archived = gh.Ptr(argBool(args, "archived"))
	}
	if _, ok := args["has_issues"]; ok {
		r.HasIssues = gh.Ptr(argBool(args, "has_issues"))
	}
	if _, ok := args["has_projects"]; ok {
		r.HasProjects = gh.Ptr(argBool(args, "has_projects"))
	}
	if _, ok := args["has_wiki"]; ok {
		r.HasWiki = gh.Ptr(argBool(args, "has_wiki"))
	}
	repo, _, err := g.client.Repositories.Edit(ctx, argStr(args, "owner"), argStr(args, "repo"), r)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(repo)
}

func replaceTopics(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	topics := argStrSlice(args, "topics")
	result, _, err := g.client.Repositories.ReplaceAllTopics(ctx, argStr(args, "owner"), argStr(args, "repo"), topics)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func renameBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	branch, _, err := g.client.Repositories.RenameBranch(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "branch"), argStr(args, "new_name"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(branch)
}

// ── Collaborators ─────────────────────────────────────────────────

func addCollaborator(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryAddCollaboratorOptions{}
	if perm := argStr(args, "permission"); perm != "" {
		opts.Permission = perm
	}
	invite, _, err := g.client.Repositories.AddCollaborator(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	if invite != nil {
		return mcp.JSONResult(invite)
	}
	return mcp.JSONResult(map[string]string{"status": "added"})
}

func removeCollaborator(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Repositories.RemoveCollaborator(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "username"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

// ── Commit Status ─────────────────────────────────────────────────

func getCombinedStatus(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	status, _, err := g.client.Repositories.GetCombinedStatus(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "ref"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(status)
}

func listStatuses(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	statuses, _, err := g.client.Repositories.ListStatuses(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "ref"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(statuses)
}

func createStatus(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	status := &gh.RepoStatus{
		State:       gh.Ptr(argStr(args, "state")),
		TargetURL:   gh.Ptr(argStr(args, "target_url")),
		Description: gh.Ptr(argStr(args, "description")),
		Context:     gh.Ptr(argStr(args, "context")),
	}
	s, _, err := g.client.Repositories.CreateStatus(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "sha"), status)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(s)
}

// ── Deployments ───────────────────────────────────────────────────

func listDeployments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.DeploymentsListOptions{
		Environment: argStr(args, "environment"),
		Ref:         argStr(args, "ref"),
		Task:        argStr(args, "task"),
		ListOptions: listOpts(args),
	}
	deployments, _, err := g.client.Repositories.ListDeployments(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployments)
}

func getDeployment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	deployment, _, err := g.client.Repositories.GetDeployment(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "deployment_id"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployment)
}

func createDeployment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.DeploymentRequest{
		Ref:         gh.Ptr(argStr(args, "ref")),
		Task:        gh.Ptr(argStr(args, "task")),
		Environment: gh.Ptr(argStr(args, "environment")),
		Description: gh.Ptr(argStr(args, "description")),
	}
	if argBool(args, "auto_merge") {
		req.AutoMerge = gh.Ptr(true)
	}
	deployment, _, err := g.client.Repositories.CreateDeployment(ctx, argStr(args, "owner"), argStr(args, "repo"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployment)
}

func listDeploymentStatuses(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	statuses, _, err := g.client.Repositories.ListDeploymentStatuses(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "deployment_id"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(statuses)
}

func createDeploymentStatus(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.DeploymentStatusRequest{
		State:       gh.Ptr(argStr(args, "state")),
		Description: gh.Ptr(argStr(args, "description")),
		LogURL:      gh.Ptr(argStr(args, "log_url")),
	}
	if env := argStr(args, "environment"); env != "" {
		req.Environment = gh.Ptr(env)
	}
	status, _, err := g.client.Repositories.CreateDeploymentStatus(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "deployment_id"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(status)
}

// ── Environments ──────────────────────────────────────────────────

func listEnvironments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.EnvironmentListOptions{ListOptions: listOpts(args)}
	envs, _, err := g.client.Repositories.ListEnvironments(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(envs.Environments)
}

func getEnvironment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	env, _, err := g.client.Repositories.GetEnvironment(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "environment"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(env)
}

// ── Branch Protection ─────────────────────────────────────────────

func getBranchProtection(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	protection, _, err := g.client.Repositories.GetBranchProtection(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "branch"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(protection)
}

func removeBranchProtection(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Repositories.RemoveBranchProtection(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "branch"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

// ── Rulesets ──────────────────────────────────────────────────────

func listRulesets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	rulesets, _, err := g.client.Repositories.GetAllRulesets(ctx, argStr(args, "owner"), argStr(args, "repo"), false)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(rulesets)
}

func getRuleset(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	ruleset, _, err := g.client.Repositories.GetRuleset(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "ruleset_id"), false)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(ruleset)
}

func getRulesForBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	rules, _, err := g.client.Repositories.GetRulesForBranch(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "branch"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(rules)
}

// ── Traffic ───────────────────────────────────────────────────────

func listTrafficViews(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.TrafficBreakdownOptions{}
	if per := argStr(args, "per"); per != "" {
		opts.Per = per
	}
	views, _, err := g.client.Repositories.ListTrafficViews(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(views)
}

func listTrafficClones(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.TrafficBreakdownOptions{}
	if per := argStr(args, "per"); per != "" {
		opts.Per = per
	}
	clones, _, err := g.client.Repositories.ListTrafficClones(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(clones)
}

func listTrafficReferrers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	referrers, _, err := g.client.Repositories.ListTrafficReferrers(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(referrers)
}

func listTrafficPaths(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	paths, _, err := g.client.Repositories.ListTrafficPaths(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(paths)
}

// ── Community Health ──────────────────────────────────────────────

func getCommunityHealth(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	metrics, _, err := g.client.Repositories.GetCommunityHealthMetrics(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(metrics)
}

// ── Dispatch ──────────────────────────────────────────────────────

func dispatchEvent(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := gh.DispatchRequestOptions{EventType: argStr(args, "event_type")}
	_, _, err := g.client.Repositories.Dispatch(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "dispatched"})
}

// ── Merge ─────────────────────────────────────────────────────────

func mergeBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.RepositoryMergeRequest{
		Base:          gh.Ptr(argStr(args, "base")),
		Head:          gh.Ptr(argStr(args, "head")),
		CommitMessage: gh.Ptr(argStr(args, "commit_message")),
	}
	commit, _, err := g.client.Repositories.Merge(ctx, argStr(args, "owner"), argStr(args, "repo"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(commit)
}

// ── Releases Extended ─────────────────────────────────────────────

func editRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := &gh.RepositoryRelease{}
	if v := argStr(args, "tag_name"); v != "" {
		r.TagName = gh.Ptr(v)
	}
	if v := argStr(args, "name"); v != "" {
		r.Name = gh.Ptr(v)
	}
	if v := argStr(args, "body"); v != "" {
		r.Body = gh.Ptr(v)
	}
	if _, ok := args["draft"]; ok {
		r.Draft = gh.Ptr(argBool(args, "draft"))
	}
	if _, ok := args["prerelease"]; ok {
		r.Prerelease = gh.Ptr(argBool(args, "prerelease"))
	}
	release, _, err := g.client.Repositories.EditRelease(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "release_id"), r)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(release)
}

func generateReleaseNotes(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.GenerateNotesOptions{
		TagName:         argStr(args, "tag_name"),
		PreviousTagName: gh.Ptr(argStr(args, "previous_tag_name")),
		TargetCommitish: gh.Ptr(argStr(args, "target_commitish")),
	}
	notes, _, err := g.client.Repositories.GenerateReleaseNotes(ctx, argStr(args, "owner"), argStr(args, "repo"), req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(notes)
}

// ── Commit Comments ───────────────────────────────────────────────

func listCommitComments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	comments, _, err := g.client.Repositories.ListCommitComments(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "sha"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(comments)
}

func createCommitComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	comment := &gh.RepositoryComment{
		Body: gh.Ptr(argStr(args, "body")),
	}
	if path := argStr(args, "path"); path != "" {
		comment.Path = gh.Ptr(path)
	}
	if pos := argInt(args, "position"); pos > 0 {
		comment.Position = gh.Ptr(pos)
	}
	c, _, err := g.client.Repositories.CreateComment(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "sha"), comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}
