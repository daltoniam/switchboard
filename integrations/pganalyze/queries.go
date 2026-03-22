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

	r := mcp.NewArgs(args)
	databaseID := r.Str("database_id")
	startTs := r.Int("start_ts")
	endTs := r.Int("end_ts")
	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	variables := map[string]any{
		"databaseId": databaseID,
	}
	if startTs > 0 {
		variables["startTs"] = startTs
	}
	if endTs > 0 {
		variables["endTs"] = endTs
	}
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
