package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// reviewThreadsQuery reads a pull request's review threads, which only the
// GraphQL API exposes with their resolution state.
const reviewThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!, $first: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: $first, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          isOutdated
          path
          line
          comments(first: 1) {
            nodes { author { login } body url createdAt }
          }
        }
      }
    }
  }
}`

type reviewThreadsResponse struct {
	Data struct {
		Repository *struct {
			PullRequest *struct {
				ReviewThreads struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						ID         string `json:"id"`
						IsResolved bool   `json:"isResolved"`
						IsOutdated bool   `json:"isOutdated"`
						Path       string `json:"path"`
						Line       *int   `json:"line"`
						Comments   struct {
							Nodes []struct {
								Author *struct {
									Login string `json:"login"`
								} `json:"author"`
								Body      string `json:"body"`
								URL       string `json:"url"`
								CreatedAt string `json:"createdAt"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type reviewThread struct {
	ID         string `json:"id"`
	IsResolved bool   `json:"is_resolved"`
	IsOutdated bool   `json:"is_outdated"`
	Path       string `json:"path"`
	Line       *int   `json:"line,omitempty"`
	Author     string `json:"author"`
	Body       string `json:"body"`
	URL        string `json:"url"`
	CreatedAt  string `json:"created_at"`
}

type reviewThreadsResult struct {
	Threads     []reviewThread `json:"threads"`
	Unresolved  int            `json:"unresolved"`
	HasNextPage bool           `json:"has_next_page"`
	EndCursor   string         `json:"end_cursor,omitempty"`
}

func listReviewThreads(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	pull := r.Int("pull_number")
	first := r.OptInt("per_page", 100)
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if owner == "" || repo == "" || pull <= 0 {
		return mcp.ErrResult(fmt.Errorf("owner, repo and pull_number are required"))
	}
	if first < 1 || first > 100 {
		first = 100
	}

	variables := map[string]any{"owner": owner, "repo": repo, "number": pull, "first": first}
	if after != "" {
		variables["after"] = after
	}
	req, err := g.client.NewRequest(http.MethodPost, "graphql", map[string]any{
		"query":     reviewThreadsQuery,
		"variables": variables,
	})
	if err != nil {
		return errResult(err)
	}
	var resp reviewThreadsResponse
	if _, err := g.client.Do(ctx, req, &resp); err != nil {
		return errResult(err)
	}
	if len(resp.Errors) > 0 {
		messages := make([]string, 0, len(resp.Errors))
		for _, e := range resp.Errors {
			messages = append(messages, e.Message)
		}
		return mcp.ErrResult(fmt.Errorf("github graphql: %s", strings.Join(messages, "; ")))
	}
	if resp.Data.Repository == nil || resp.Data.Repository.PullRequest == nil {
		return mcp.ErrResult(fmt.Errorf("pull request %s/%s#%d not found", owner, repo, pull))
	}

	threads := resp.Data.Repository.PullRequest.ReviewThreads
	out := reviewThreadsResult{
		Threads:     make([]reviewThread, 0, len(threads.Nodes)),
		HasNextPage: threads.PageInfo.HasNextPage,
		EndCursor:   threads.PageInfo.EndCursor,
	}
	for _, node := range threads.Nodes {
		thread := reviewThread{
			ID:         node.ID,
			IsResolved: node.IsResolved,
			IsOutdated: node.IsOutdated,
			Path:       node.Path,
			Line:       node.Line,
		}
		if len(node.Comments.Nodes) > 0 {
			first := node.Comments.Nodes[0]
			if first.Author != nil {
				thread.Author = first.Author.Login
			}
			thread.Body = first.Body
			thread.URL = first.URL
			thread.CreatedAt = first.CreatedAt
		}
		if !thread.IsResolved {
			out.Unresolved++
		}
		out.Threads = append(out.Threads, thread)
	}
	return mcp.JSONResult(out)
}
