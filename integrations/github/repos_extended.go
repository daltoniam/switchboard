package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Repo Settings ─────────────────────────────────────────────────

func editRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	rep := &gh.Repository{}
	if v, err := mcp.ArgStr(args, "description"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		rep.Description = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "homepage"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		rep.Homepage = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "default_branch"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		rep.DefaultBranch = gh.Ptr(v)
	}
	if _, ok := args["private"]; ok {
		if v, err := mcp.ArgBool(args, "private"); err != nil {
			return mcp.ErrResult(err)
		} else {
			rep.Private = gh.Ptr(v)
		}
	}
	if _, ok := args["archived"]; ok {
		if v, err := mcp.ArgBool(args, "archived"); err != nil {
			return mcp.ErrResult(err)
		} else {
			rep.Archived = gh.Ptr(v)
		}
	}
	if _, ok := args["has_issues"]; ok {
		if v, err := mcp.ArgBool(args, "has_issues"); err != nil {
			return mcp.ErrResult(err)
		} else {
			rep.HasIssues = gh.Ptr(v)
		}
	}
	if _, ok := args["has_projects"]; ok {
		if v, err := mcp.ArgBool(args, "has_projects"); err != nil {
			return mcp.ErrResult(err)
		} else {
			rep.HasProjects = gh.Ptr(v)
		}
	}
	if _, ok := args["has_wiki"]; ok {
		if v, err := mcp.ArgBool(args, "has_wiki"); err != nil {
			return mcp.ErrResult(err)
		} else {
			rep.HasWiki = gh.Ptr(v)
		}
	}
	result, _, err := g.client.Repositories.Edit(ctx, owner, repo, rep)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func replaceTopics(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	topics := r.StrSlice("topics")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	result, _, err := g.client.Repositories.ReplaceAllTopics(ctx, owner, repo, topics)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func renameBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	branch := r.Str("branch")
	newName := r.Str("new_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	result, _, err := g.client.Repositories.RenameBranch(ctx, owner, repo, branch, newName)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

// ── Collaborators ─────────────────────────────────────────────────

func addCollaborator(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	username := r.Str("username")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryAddCollaboratorOptions{}
	if perm, err := mcp.ArgStr(args, "permission"); err != nil {
		return mcp.ErrResult(err)
	} else if perm != "" {
		opts.Permission = perm
	}
	invite, _, err := g.client.Repositories.AddCollaborator(ctx, owner, repo, username, opts)
	if err != nil {
		return errResult(err)
	}
	if invite != nil {
		return mcp.JSONResult(invite)
	}
	return mcp.JSONResult(map[string]string{"status": "added"})
}

func removeCollaborator(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	username := r.Str("username")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Repositories.RemoveCollaborator(ctx, owner, repo, username)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

// ── Commit Status ─────────────────────────────────────────────────

func getCombinedStatus(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	status, _, err := g.client.Repositories.GetCombinedStatus(ctx, owner, repo, ref, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(status)
}

func listStatuses(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	statuses, _, err := g.client.Repositories.ListStatuses(ctx, owner, repo, ref, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(statuses)
}

func createStatus(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	sha := r.Str("sha")
	state := r.Str("state")
	targetURL := r.Str("target_url")
	description := r.Str("description")
	ctxStr := r.Str("context")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	status := &gh.RepoStatus{
		State:       gh.Ptr(state),
		TargetURL:   gh.Ptr(targetURL),
		Description: gh.Ptr(description),
		Context:     gh.Ptr(ctxStr),
	}
	s, _, err := g.client.Repositories.CreateStatus(ctx, owner, repo, sha, status)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(s)
}

// ── Deployments ───────────────────────────────────────────────────

func listDeployments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	environment := r.Str("environment")
	ref := r.Str("ref")
	task := r.Str("task")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.DeploymentsListOptions{
		Environment: environment,
		Ref:         ref,
		Task:        task,
		ListOptions: lo,
	}
	deployments, _, err := g.client.Repositories.ListDeployments(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployments)
}

func getDeployment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	deploymentID := r.Int64("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	deployment, _, err := g.client.Repositories.GetDeployment(ctx, owner, repo, deploymentID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployment)
}

func createDeployment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	task := r.Str("task")
	environment := r.Str("environment")
	description := r.Str("description")
	autoMerge := r.Bool("auto_merge")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.DeploymentRequest{
		Ref:         gh.Ptr(ref),
		Task:        gh.Ptr(task),
		Environment: gh.Ptr(environment),
		Description: gh.Ptr(description),
	}
	if autoMerge {
		req.AutoMerge = gh.Ptr(true)
	}
	deployment, _, err := g.client.Repositories.CreateDeployment(ctx, owner, repo, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(deployment)
}

func listDeploymentStatuses(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	deploymentID := r.Int64("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	statuses, _, err := g.client.Repositories.ListDeploymentStatuses(ctx, owner, repo, deploymentID, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(statuses)
}

func createDeploymentStatus(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	deploymentID := r.Int64("deployment_id")
	state := r.Str("state")
	description := r.Str("description")
	logURL := r.Str("log_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.DeploymentStatusRequest{
		State:       gh.Ptr(state),
		Description: gh.Ptr(description),
		LogURL:      gh.Ptr(logURL),
	}
	if env, err := mcp.ArgStr(args, "environment"); err != nil {
		return mcp.ErrResult(err)
	} else if env != "" {
		req.Environment = gh.Ptr(env)
	}
	status, _, err := g.client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deploymentID, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(status)
}

// ── Environments ──────────────────────────────────────────────────

func listEnvironments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.EnvironmentListOptions{ListOptions: lo}
	envs, _, err := g.client.Repositories.ListEnvironments(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(envs.Environments)
}

func getEnvironment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	environment := r.Str("environment")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	env, _, err := g.client.Repositories.GetEnvironment(ctx, owner, repo, environment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(env)
}

// ── Branch Protection ─────────────────────────────────────────────

func getBranchProtection(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	branch := r.Str("branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	protection, _, err := g.client.Repositories.GetBranchProtection(ctx, owner, repo, branch)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(protection)
}

func removeBranchProtection(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	branch := r.Str("branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Repositories.RemoveBranchProtection(ctx, owner, repo, branch)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

// ── Rulesets ──────────────────────────────────────────────────────

func listRulesets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	rulesets, _, err := g.client.Repositories.GetAllRulesets(ctx, owner, repo, false)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(rulesets)
}

func getRuleset(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	rulesetID := r.Int64("ruleset_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ruleset, _, err := g.client.Repositories.GetRuleset(ctx, owner, repo, rulesetID, false)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(ruleset)
}

func getRulesForBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	branch := r.Str("branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	rules, _, err := g.client.Repositories.GetRulesForBranch(ctx, owner, repo, branch)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(rules)
}

// ── Traffic ───────────────────────────────────────────────────────

func listTrafficViews(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.TrafficBreakdownOptions{}
	if per, err := mcp.ArgStr(args, "per"); err != nil {
		return mcp.ErrResult(err)
	} else if per != "" {
		opts.Per = per
	}
	views, _, err := g.client.Repositories.ListTrafficViews(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(views)
}

func listTrafficClones(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.TrafficBreakdownOptions{}
	if per, err := mcp.ArgStr(args, "per"); err != nil {
		return mcp.ErrResult(err)
	} else if per != "" {
		opts.Per = per
	}
	clones, _, err := g.client.Repositories.ListTrafficClones(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(clones)
}

func listTrafficReferrers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	referrers, _, err := g.client.Repositories.ListTrafficReferrers(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(referrers)
}

func listTrafficPaths(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	paths, _, err := g.client.Repositories.ListTrafficPaths(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(paths)
}

// ── Community Health ──────────────────────────────────────────────

func getCommunityHealth(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	metrics, _, err := g.client.Repositories.GetCommunityHealthMetrics(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(metrics)
}

// ── Dispatch ──────────────────────────────────────────────────────

func dispatchEvent(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	eventType := r.Str("event_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := gh.DispatchRequestOptions{EventType: eventType}
	_, _, err := g.client.Repositories.Dispatch(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "dispatched"})
}

// ── Merge ─────────────────────────────────────────────────────────

func mergeBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	base := r.Str("base")
	head := r.Str("head")
	commitMessage := r.Str("commit_message")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.RepositoryMergeRequest{
		Base:          gh.Ptr(base),
		Head:          gh.Ptr(head),
		CommitMessage: gh.Ptr(commitMessage),
	}
	commit, _, err := g.client.Repositories.Merge(ctx, owner, repo, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(commit)
}

// ── Releases Extended ─────────────────────────────────────────────

func editRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	releaseID := r.Int64("release_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	rel := &gh.RepositoryRelease{}
	if v, err := mcp.ArgStr(args, "tag_name"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		rel.TagName = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "name"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		rel.Name = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "body"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		rel.Body = gh.Ptr(v)
	}
	if _, ok := args["draft"]; ok {
		if v, err := mcp.ArgBool(args, "draft"); err != nil {
			return mcp.ErrResult(err)
		} else {
			rel.Draft = gh.Ptr(v)
		}
	}
	if _, ok := args["prerelease"]; ok {
		if v, err := mcp.ArgBool(args, "prerelease"); err != nil {
			return mcp.ErrResult(err)
		} else {
			rel.Prerelease = gh.Ptr(v)
		}
	}
	release, _, err := g.client.Repositories.EditRelease(ctx, owner, repo, releaseID, rel)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(release)
}

func generateReleaseNotes(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	tagName := r.Str("tag_name")
	previousTagName := r.Str("previous_tag_name")
	targetCommitish := r.Str("target_commitish")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.GenerateNotesOptions{
		TagName:         tagName,
		PreviousTagName: gh.Ptr(previousTagName),
		TargetCommitish: gh.Ptr(targetCommitish),
	}
	notes, _, err := g.client.Repositories.GenerateReleaseNotes(ctx, owner, repo, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(notes)
}

// ── Commit Comments ───────────────────────────────────────────────

func listCommitComments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	sha := r.Str("sha")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	comments, _, err := g.client.Repositories.ListCommitComments(ctx, owner, repo, sha, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(comments)
}

func createCommitComment(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	sha := r.Str("sha")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	comment := &gh.RepositoryComment{
		Body: gh.Ptr(body),
	}
	if path, err := mcp.ArgStr(args, "path"); err != nil {
		return mcp.ErrResult(err)
	} else if path != "" {
		comment.Path = gh.Ptr(path)
	}
	if pos, err := mcp.ArgInt(args, "position"); err != nil {
		return mcp.ErrResult(err)
	} else if pos > 0 {
		comment.Position = gh.Ptr(pos)
	}
	c, _, err := g.client.Repositories.CreateComment(ctx, owner, repo, sha, comment)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(c)
}
