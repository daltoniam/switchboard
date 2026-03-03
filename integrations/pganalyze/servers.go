package pganalyze

import (
	"context"
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
)

func getServers(ctx context.Context, p *pganalyze, args map[string]any) (*mcp.ToolResult, error) {
	query := `
		query GetServers($organizationSlug: ID) {
			getServers(organizationSlug: $organizationSlug) {
				id
				name
				humanId
				lastSnapshotAt
				databases {
					id
					datname
					displayName
				}
			}
		}
	`

	variables := make(map[string]any)
	if v := p.orgSlug(args); v != "" {
		variables["organizationSlug"] = v
	}

	data, err := p.gql(ctx, query, variables)
	if err != nil {
		return errResult(err)
	}

	var resp struct {
		GetServers json.RawMessage `json:"getServers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errResult(err)
	}
	return rawResult(resp.GetServers)
}
