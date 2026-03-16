package pganalyze

import (
	"context"
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
)

func getQueryStats(ctx context.Context, p *pganalyze, args map[string]any) (*mcp.ToolResult, error) {
	query := `
		query GetQueryStats($databaseId: ID!, $startTs: Int, $endTs: Int, $limit: Int) {
			getQueryStats(databaseId: $databaseId, startTs: $startTs, endTs: $endTs, limit: $limit) {
				id
				queryId
				queryUrl
				normalizedQuery
				truncatedQuery
				queryComment
				statementTypes
				totalCalls
				avgTime
				avgIoTime
				bufferHitRatio
				pctOfTotal
				callsPerMinute
			}
		}
	`

	variables := map[string]any{
		"databaseId": argStr(args, "database_id"),
	}
	if v := argInt(args, "start_ts"); v > 0 {
		variables["startTs"] = v
	}
	if v := argInt(args, "end_ts"); v > 0 {
		variables["endTs"] = v
	}
	limit := argInt(args, "limit")
	if limit <= 0 {
		limit = 20
	}
	variables["limit"] = limit

	data, err := p.gql(ctx, query, variables)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var resp struct {
		GetQueryStats json.RawMessage `json:"getQueryStats"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(resp.GetQueryStats)
}
