package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func searchRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	query := r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SearchOptions{ListOptions: lo}
	resp, _, err := g.client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Repositories)
}

func getRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	result, _, err := g.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func listUserRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	username := r.Str("username")
	typ := r.Str("type")
	sort := r.Str("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryListOptions{
		Type:        typ,
		Sort:        sort,
		ListOptions: lo,
	}
	repos, _, err := g.client.Repositories.List(ctx, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(repos)
}

func listOrgRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	org := r.Str("org")
	typ := r.Str("type")
	sort := r.Str("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryListByOrgOptions{
		Type:        typ,
		Sort:        sort,
		ListOptions: lo,
	}
	repos, _, err := g.client.Repositories.ListByOrg(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(repos)
}

func createRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	private := r.Bool("private")
	autoInit := r.Bool("auto_init")
	org := r.Str("org")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	repo := &gh.Repository{
		Name:        gh.Ptr(name),
		Description: gh.Ptr(description),
		Private:     gh.Ptr(private),
		AutoInit:    gh.Ptr(autoInit),
	}
	result, _, err := g.client.Repositories.Create(ctx, org, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func deleteRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Repositories.Delete(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

func listBranches(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.BranchListOptions{ListOptions: lo}
	branches, _, err := g.client.Repositories.ListBranches(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(branches)
}

func getBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	branch := r.Str("branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	result, _, err := g.client.Repositories.GetBranch(ctx, owner, repo, branch, 0)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func listTags(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	tags, _, err := g.client.Repositories.ListTags(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(tags)
}

func listContributors(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.ListContributorsOptions{ListOptions: lo}
	contributors, _, err := g.client.Repositories.ListContributors(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(contributors)
}

func listLanguages(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	langs, _, err := g.client.Repositories.ListLanguages(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(langs)
}

func listTopics(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	topics, _, err := g.client.Repositories.ListAllTopics(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(topics)
}

func getReadme(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryContentGetOptions{Ref: ref}
	readme, _, err := g.client.Repositories.GetReadme(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	content, err := readme.GetContent()
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"name": readme.GetName(), "path": readme.GetPath(), "content": content})
}

func getFileContents(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	path := r.Str("path")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryContentGetOptions{Ref: ref}
	fileContent, dirContent, _, err := g.client.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return errResult(err)
	}
	if fileContent != nil {
		content, err := fileContent.GetContent()
		if err != nil {
			return errResult(err)
		}
		return mcp.JSONResult(map[string]any{"type": "file", "name": fileContent.GetName(), "path": fileContent.GetPath(), "sha": fileContent.GetSHA(), "size": fileContent.GetSize(), "content": content})
	}
	return mcp.JSONResult(map[string]any{"type": "dir", "entries": dirContent})
}

func createOrUpdateFile(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	path := r.Str("path")
	message := r.Str("message")
	content := r.Str("content")
	branch := r.Str("branch")
	sha := r.Str("sha")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryContentFileOptions{
		Message: gh.Ptr(message),
		Content: []byte(content),
		Branch:  gh.Ptr(branch),
	}
	if sha != "" {
		opts.SHA = gh.Ptr(sha)
	}
	resp, _, err := g.client.Repositories.CreateFile(ctx, owner, repo, path, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteFile(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	path := r.Str("path")
	message := r.Str("message")
	sha := r.Str("sha")
	branch := r.Str("branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryContentFileOptions{
		Message: gh.Ptr(message),
		SHA:     gh.Ptr(sha),
		Branch:  gh.Ptr(branch),
	}
	resp, _, err := g.client.Repositories.DeleteFile(ctx, owner, repo, path, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp)
}

func listForks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	sort := r.Str("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryListForksOptions{
		Sort:        sort,
		ListOptions: lo,
	}
	forks, _, err := g.client.Repositories.ListForks(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(forks)
}

func createFork(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	organization := r.Str("organization")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.RepositoryCreateForkOptions{}
	if organization != "" {
		opts.Organization = organization
	}
	result, _, err := g.client.Repositories.CreateFork(ctx, owner, repo, opts)
	if err != nil {
		if _, ok := err.(*gh.AcceptedError); ok {
			return mcp.JSONResult(map[string]string{"status": "forking", "message": "Fork is being created asynchronously"})
		}
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func listCollaborators(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.ListCollaboratorsOptions{ListOptions: lo}
	users, _, err := g.client.Repositories.ListCollaborators(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(users)
}

func listCommitActivity(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	activity, _, err := g.client.Repositories.ListCommitActivity(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(activity)
}

func listRepoTeams(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	teams, _, err := g.client.Repositories.ListTeams(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(teams)
}

func compareCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	base := r.Str("base")
	head := r.Str("head")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	comparison, _, err := g.client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(comparison)
}

func mergeUpstream(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	branch := r.Str("branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &gh.RepoMergeUpstreamRequest{Branch: gh.Ptr(branch)}
	result, _, err := g.client.Repositories.MergeUpstream(ctx, owner, repo, req)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func listAutolinks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.Int("page")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page}
	links, _, err := g.client.Repositories.ListAutolinks(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(links)
}

// ── Releases ──────────────────────────────────────────────────────

func listReleases(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	releases, _, err := g.client.Repositories.ListReleases(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(releases)
}

func getRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	releaseID := r.Int64("release_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	release, _, err := g.client.Repositories.GetRelease(ctx, owner, repo, releaseID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(release)
}

func getLatestRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	release, _, err := g.client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(release)
}

func createRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	tagName := r.Str("tag_name")
	name := r.Str("name")
	body := r.Str("body")
	draft := r.Bool("draft")
	prerelease := r.Bool("prerelease")
	targetCommitish := r.Str("target_commitish")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	rel := &gh.RepositoryRelease{
		TagName:         gh.Ptr(tagName),
		Name:            gh.Ptr(name),
		Body:            gh.Ptr(body),
		Draft:           gh.Ptr(draft),
		Prerelease:      gh.Ptr(prerelease),
		TargetCommitish: gh.Ptr(targetCommitish),
	}
	release, _, err := g.client.Repositories.CreateRelease(ctx, owner, repo, rel)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(release)
}

func deleteRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	releaseID := r.Int64("release_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Repositories.DeleteRelease(ctx, owner, repo, releaseID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

func listReleaseAssets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	releaseID := r.Int64("release_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	assets, _, err := g.client.Repositories.ListReleaseAssets(ctx, owner, repo, releaseID, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(assets)
}

// ── Deploy Keys ───────────────────────────────────────────────────

func listDeployKeys(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	keys, _, err := g.client.Repositories.ListKeys(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(keys)
}

// ── Webhooks ──────────────────────────────────────────────────────

func listHooks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	hooks, _, err := g.client.Repositories.ListHooks(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(hooks)
}

func createHook(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	url := r.Str("url")
	contentType := r.Str("content_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if contentType == "" {
		contentType = "json"
	}
	active := true
	if v, ok := args["active"]; ok {
		if s, ok := v.(string); ok && s == "false" {
			active = false
		}
		if b, ok := v.(bool); ok {
			active = b
		}
	}
	events, err := mcp.ArgStrSlice(args, "events")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(events) == 0 {
		events = []string{"push"}
	}
	hook := &gh.Hook{
		Config: &gh.HookConfig{
			URL:         gh.Ptr(url),
			ContentType: gh.Ptr(contentType),
		},
		Events: events,
		Active: gh.Ptr(active),
	}
	h, _, err := g.client.Repositories.CreateHook(ctx, owner, repo, hook)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(h)
}

func deleteHook(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	hookID := r.Int64("hook_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Repositories.DeleteHook(ctx, owner, repo, hookID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Rate Limit ────────────────────────────────────────────────────

func getRateLimit(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	rl, _, err := g.client.RateLimit.Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(rl)
}
