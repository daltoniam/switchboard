package pganalyze

import (
	"context"
	"encoding/json"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getIssues(ctx context.Context, p *pganalyze, args map[string]any) (*mcp.ToolResult, error) {
	query := `
		query GetIssues($organizationSlug: ID, $serverId: ID, $databaseId: ID, $state: String) {
			getIssues(organizationSlug: $organizationSlug, serverId: $serverId, databaseId: $databaseId, state: $state) {
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

	variables := make(map[string]any)
	if v := p.orgSlug(args); v != "" {
		variables["organizationSlug"] = v
	}
	if v := argStr(args, "server_id"); v != "" {
		variables["serverId"] = v
	}
	if v := argStr(args, "database_id"); v != "" {
		variables["databaseId"] = v
	}
	if !argBool(args, "include_resolved") {
		variables["state"] = "open"
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

	if severity := argStr(args, "severity"); severity != "" {
		var issues []map[string]any
		if err := json.Unmarshal(resp.GetIssues, &issues); err != nil {
			return mcp.ErrResult(err)
		}
		filtered := make([]map[string]any, 0, len(issues))
		for _, issue := range issues {
			if s, ok := issue["severity"].(string); ok && strings.EqualFold(s, severity) {
				filtered = append(filtered, issue)
			}
		}
		return mcp.JSONResult(filtered)
	}

	return mcp.RawResult(resp.GetIssues)
}
