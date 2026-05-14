package teams

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// sanitizeSearchTerm strips characters that would break the OData $search
// expression. Inner double-quotes terminate the quoted string and yield a
// Graph 400; backslashes can introduce escape ambiguity. We never need them
// for the displayName/UPN lookups in this package.
func sanitizeSearchTerm(s string) string {
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.ReplaceAll(s, `\`, "")
	return strings.TrimSpace(s)
}

// listUsers -> GET /users
func listUsers(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	top := r.OptInt("top", 20)
	filter := r.Str("filter")
	selectFields := r.Str("select")
	search := r.Str("search")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if top > 100 {
		top = 100
	}
	q := url.Values{}
	q.Set("$top", fmt.Sprintf("%d", top))
	if filter != "" {
		q.Set("$filter", filter)
	}
	if selectFields != "" {
		q.Set("$select", selectFields)
	}
	if search != "" {
		safe := sanitizeSearchTerm(search)
		q.Set("$search", fmt.Sprintf(`"displayName:%s" OR "userPrincipalName:%s"`, safe, safe))
	}
	path := "/users?" + q.Encode()
	if search != "" {
		// $search on /users requires ConsistencyLevel: eventual.
		return t.graphGetWithHeaders(ctx, tn.TenantID, path, http.Header{"ConsistencyLevel": []string{"eventual"}})
	}
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// getUser -> GET /users/{id-or-upn}
func getUser(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	user := r.Str("user")
	selectFields := r.Str("select")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if user == "" {
		return mcp.ErrResult(fmt.Errorf("user is required"))
	}
	path := "/users/" + url.PathEscape(user)
	if selectFields != "" {
		path += "?$select=" + url.QueryEscape(selectFields)
	}
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// searchUsers -> /users?$search=... with ConsistencyLevel: eventual
func searchUsers(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	query := r.Str("query")
	top := r.OptInt("top", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if query == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}
	if top > 25 {
		top = 25
	}
	safe := sanitizeSearchTerm(query)
	q := url.Values{}
	q.Set("$top", fmt.Sprintf("%d", top))
	q.Set("$search", fmt.Sprintf(`"displayName:%s" OR "userPrincipalName:%s"`, safe, safe))
	path := "/users?" + q.Encode()
	return t.graphGetWithHeaders(ctx, tn.TenantID, path, http.Header{"ConsistencyLevel": []string{"eventual"}})
}

// getPresence -> GET /users/{id}/presence or /me/presence
func getPresence(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	user := r.Str("user")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := "/me/presence"
	if user != "" {
		path = "/users/" + url.PathEscape(user) + "/presence"
	}
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
