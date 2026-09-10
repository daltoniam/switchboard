package forgejo

import (
	"fmt"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	mcp "github.com/daltoniam/switchboard"
)

func listUserRepos(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	username := r.Str("username")
	opts := sdk.ListReposOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if username == "" {
		return finish(c.ListMyRepos(opts))
	}
	return finish(c.ListUserRepos(username, opts))
}

func searchRepos(c *sdk.Client, args map[string]any) (result *mcp.ToolResult, err error) {
	defer func() {
		if recover() != nil {
			result, err = mcp.ErrResult(fmt.Errorf("forgejo: SDK could not decode repository search response"))
		}
	}()
	r := mcp.NewArgs(args)
	opts := sdk.SearchRepoOptions{ListOptions: pagination(r), Keyword: r.Str("query"), Sort: r.Str("sort"), Order: r.Str("order")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.SearchRepos(opts))
}

func getRepo(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetRepo(owner, repo))
}

func listOrgRepos(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	opts := sdk.ListOrgReposOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.ListOrgRepos(org, opts))
}

func listUserOrgs(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	username := r.Str("username")
	opts := sdk.ListOrgsOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if username == "" {
		return finish(c.ListMyOrgs(opts))
	}
	return finish(c.ListUserOrgs(username, opts))
}

func getCurrentUser(c *sdk.Client, _ map[string]any) (*mcp.ToolResult, error) {
	return finish(c.GetMyUserInfo())
}

func listBranches(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := sdk.ListRepoBranchesOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.ListRepoBranches(owner, repo, opts))
}

func getBranch(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, branch := r.Str("owner"), r.Str("repo"), r.Str("branch")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetRepoBranch(owner, repo, branch))
}

func listCommits(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := sdk.ListCommitOptions{ListOptions: pagination(r), SHA: r.Str("sha"), Path: r.Str("path")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if opts.Path != "" {
		if _, ok := args["per_page"]; ok {
			return mcp.ErrResult(fmt.Errorf("forgejo: per_page cannot be used with path; Forgejo uses a server-controlled page size for path-filtered commits, use page to continue"))
		}
		opts.PageSize = 0
	}
	return finish(c.ListRepoCommits(owner, repo, opts))
}

func getCommit(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, sha := r.Str("owner"), r.Str("repo"), r.Str("sha")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetSingleCommit(owner, repo, sha))
}

func getFileContents(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, path, ref := r.Str("owner"), r.Str("repo"), r.Str("path"), r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetContents(owner, repo, ref, path))
}

func listDirectory(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, path, ref := r.Str("owner"), r.Str("repo"), r.Str("path"), r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.ListContents(owner, repo, ref, path))
}

func listReleases(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	draft, prerelease := r.Bool("draft"), r.Bool("prerelease")
	opts := sdk.ListReleasesOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, ok := args["draft"]; ok {
		opts.IsDraft = &draft
	}
	if _, ok := args["prerelease"]; ok {
		opts.IsPreRelease = &prerelease
	}
	return finish(c.ListReleases(owner, repo, opts))
}

func getRelease(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, id := r.Str("owner"), r.Str("repo"), r.Int64("release_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetRelease(owner, repo, id))
}

func listLabels(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := sdk.ListLabelsOptions{ListOptions: pagination(r)}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.ListRepoLabels(owner, repo, opts))
}
