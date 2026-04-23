package pganalyze

import (
	"context"
	"encoding/json"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getIssues(ctx context.Context, p *pganalyze, args map[string]any) (*mcp.ToolResult, error) {
	query := `
		query GetIssues($organizationSlug: ID, $serverId: ID, $databaseId: ID) {
			getIssues(organizationSlug: $organizationSlug, serverId: $serverId, databaseId: $databaseId) {
				id
				checkGroupAndName
				description
				severity
				state
				createdAt
				updatedAt
				references {
					kind
					name
					url
					resolvedAt
				}
			}
		}
	`

	r := mcp.NewArgs(args)
	serverID := r.Str("server_id")
	databaseID := r.Str("database_id")
	includeResolved := r.Bool("include_resolved")
	severity := r.Str("severity")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	variables := make(map[string]any)
	if v := p.orgSlug(args); v != "" {
		variables["organizationSlug"] = v
	}
	if serverID != "" {
		variables["serverId"] = serverID
	}
	if databaseID != "" {
		variables["databaseId"] = databaseID
	}

	data, err := p.gql(ctx, query, variables)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var resp struct {
		GetIssues json.RawMessage `json:"getIssues"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return mcp.ErrResult(err)
	}

	var issues []map[string]any
	if err := json.Unmarshal(resp.GetIssues, &issues); err != nil {
		return mcp.ErrResult(err)
	}

	filtered := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		state, _ := issue["state"].(string)
		if !includeResolved && strings.EqualFold(state, "resolved") {
			continue
		}
		if severity != "" {
			sev, _ := issue["severity"].(string)
			if !strings.EqualFold(sev, severity) {
				continue
			}
		}
		filtered = append(filtered, issue)
	}

	return mcp.JSONResult(filtered)
}
