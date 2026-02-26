package github

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

func searchRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SearchOptions{ListOptions: listOpts(args)}
	result, _, err := g.client.Search.Repositories(ctx, argStr(args, "query"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

func getRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	repo, _, err := g.client.Repositories.Get(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(repo)
}

func listUserRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryListOptions{
		Type:        argStr(args, "type"),
		Sort:        argStr(args, "sort"),
		ListOptions: listOpts(args),
	}
	repos, _, err := g.client.Repositories.List(ctx, argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(repos)
}

func listOrgRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryListByOrgOptions{
		Type:        argStr(args, "type"),
		Sort:        argStr(args, "sort"),
		ListOptions: listOpts(args),
	}
	repos, _, err := g.client.Repositories.ListByOrg(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(repos)
}

func createRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := &gh.Repository{
		Name:        gh.Ptr(argStr(args, "name")),
		Description: gh.Ptr(argStr(args, "description")),
		Private:     gh.Ptr(argBool(args, "private")),
		AutoInit:    gh.Ptr(argBool(args, "auto_init")),
	}
	repo, _, err := g.client.Repositories.Create(ctx, argStr(args, "org"), r)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(repo)
}

func deleteRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Repositories.Delete(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

func listBranches(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.BranchListOptions{ListOptions: listOpts(args)}
	branches, _, err := g.client.Repositories.ListBranches(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(branches)
}

func getBranch(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	branch, _, err := g.client.Repositories.GetBranch(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "branch"), 0)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(branch)
}

func listTags(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	tags, _, err := g.client.Repositories.ListTags(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(tags)
}

func listContributors(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListContributorsOptions{ListOptions: listOpts(args)}
	contributors, _, err := g.client.Repositories.ListContributors(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(contributors)
}

func listLanguages(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	langs, _, err := g.client.Repositories.ListLanguages(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(langs)
}

func listTopics(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	topics, _, err := g.client.Repositories.ListAllTopics(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(topics)
}

func getReadme(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryContentGetOptions{Ref: argStr(args, "ref")}
	readme, _, err := g.client.Repositories.GetReadme(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	content, err := readme.GetContent()
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"name": readme.GetName(), "path": readme.GetPath(), "content": content})
}

func getFileContents(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryContentGetOptions{Ref: argStr(args, "ref")}
	fileContent, dirContent, _, err := g.client.Repositories.GetContents(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "path"), opts)
	if err != nil {
		return errResult(err)
	}
	if fileContent != nil {
		content, err := fileContent.GetContent()
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"type": "file", "name": fileContent.GetName(), "path": fileContent.GetPath(), "sha": fileContent.GetSHA(), "size": fileContent.GetSize(), "content": content})
	}
	return jsonResult(map[string]any{"type": "dir", "entries": dirContent})
}

func createOrUpdateFile(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryContentFileOptions{
		Message: gh.Ptr(argStr(args, "message")),
		Content: []byte(argStr(args, "content")),
		Branch:  gh.Ptr(argStr(args, "branch")),
	}
	if sha := argStr(args, "sha"); sha != "" {
		opts.SHA = gh.Ptr(sha)
	}
	resp, _, err := g.client.Repositories.CreateFile(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "path"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func deleteFile(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryContentFileOptions{
		Message: gh.Ptr(argStr(args, "message")),
		SHA:     gh.Ptr(argStr(args, "sha")),
		Branch:  gh.Ptr(argStr(args, "branch")),
	}
	resp, _, err := g.client.Repositories.DeleteFile(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "path"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func listForks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryListForksOptions{
		Sort:        argStr(args, "sort"),
		ListOptions: listOpts(args),
	}
	forks, _, err := g.client.Repositories.ListForks(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(forks)
}

func createFork(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.RepositoryCreateForkOptions{}
	if org := argStr(args, "organization"); org != "" {
		opts.Organization = org
	}
	repo, _, err := g.client.Repositories.CreateFork(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		if _, ok := err.(*gh.AcceptedError); ok {
			return jsonResult(map[string]string{"status": "forking", "message": "Fork is being created asynchronously"})
		}
		return errResult(err)
	}
	return jsonResult(repo)
}

func listCollaborators(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListCollaboratorsOptions{ListOptions: listOpts(args)}
	users, _, err := g.client.Repositories.ListCollaborators(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(users)
}

func listCommitActivity(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	activity, _, err := g.client.Repositories.ListCommitActivity(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(activity)
}

func listRepoTeams(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	teams, _, err := g.client.Repositories.ListTeams(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(teams)
}

func compareCommits(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	comparison, _, err := g.client.Repositories.CompareCommits(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "base"), argStr(args, "head"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(comparison)
}

func mergeUpstream(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &gh.RepoMergeUpstreamRequest{Branch: gh.Ptr(argStr(args, "branch"))}
	result, _, err := g.client.Repositories.MergeUpstream(ctx, argStr(args, "owner"), argStr(args, "repo"), req)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

func listAutolinks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: argInt(args, "page")}
	links, _, err := g.client.Repositories.ListAutolinks(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(links)
}

// ── Releases ──────────────────────────────────────────────────────

func listReleases(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	releases, _, err := g.client.Repositories.ListReleases(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(releases)
}

func getRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	release, _, err := g.client.Repositories.GetRelease(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "release_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(release)
}

func getLatestRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	release, _, err := g.client.Repositories.GetLatestRelease(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(release)
}

func createRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := &gh.RepositoryRelease{
		TagName:         gh.Ptr(argStr(args, "tag_name")),
		Name:            gh.Ptr(argStr(args, "name")),
		Body:            gh.Ptr(argStr(args, "body")),
		Draft:           gh.Ptr(argBool(args, "draft")),
		Prerelease:      gh.Ptr(argBool(args, "prerelease")),
		TargetCommitish: gh.Ptr(argStr(args, "target_commitish")),
	}
	release, _, err := g.client.Repositories.CreateRelease(ctx, argStr(args, "owner"), argStr(args, "repo"), r)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(release)
}

func deleteRelease(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Repositories.DeleteRelease(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "release_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

func listReleaseAssets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	assets, _, err := g.client.Repositories.ListReleaseAssets(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "release_id"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(assets)
}

// ── Deploy Keys ───────────────────────────────────────────────────

func listDeployKeys(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	keys, _, err := g.client.Repositories.ListKeys(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(keys)
}

// ── Webhooks ──────────────────────────────────────────────────────

func listHooks(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	hooks, _, err := g.client.Repositories.ListHooks(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(hooks)
}

func createHook(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	contentType := argStr(args, "content_type")
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
	events := argStrSlice(args, "events")
	if len(events) == 0 {
		events = []string{"push"}
	}
	hook := &gh.Hook{
		Config: &gh.HookConfig{
			URL:         gh.Ptr(argStr(args, "url")),
			ContentType: gh.Ptr(contentType),
		},
		Events: events,
		Active: gh.Ptr(active),
	}
	h, _, err := g.client.Repositories.CreateHook(ctx, argStr(args, "owner"), argStr(args, "repo"), hook)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(h)
}

func deleteHook(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Repositories.DeleteHook(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "hook_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

// ── Rate Limit ────────────────────────────────────────────────────

func getRateLimit(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	rl, _, err := g.client.RateLimit.Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(rl)
}

// unused but keeps the import
var _ = fmt.Sprint
