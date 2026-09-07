package microsoft365

import (
	"context"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getMe(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sel := r.Str("select")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.get(ctx, "/me"+queryEncode(map[string]string{"$select": sel}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapGraph(data))
}

func listUsers(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		search := r.Str("search")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		headers := map[string]string{}
		if search != "" {
			params["$search"] = directorySearch(search)
			headers["ConsistencyLevel"] = "eventual"
			params["$count"] = "true"
		}
		return "/users" + queryEncode(params), headers, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func searchPeople(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		q := r.Str("search")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		if q != "" {
			params["$search"] = quoteGraphSearch(q)
		}
		return "/me/people" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func splitEmails(s string) []map[string]any {
	var out []map[string]any
	for _, part := range splitCSV(s) {
		out = append(out, map[string]any{
			"emailAddress": map[string]any{"address": part},
		})
	}
	return out
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
