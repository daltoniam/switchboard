package gcal

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func queryFreebusy(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if raw := r.Str("body"); raw != "" {
		if err := r.Err(); err != nil {
			return mcp.ErrResult(err)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
		}
		data, err := g.post(ctx, "/freeBusy", body)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	timeMin := r.Str("time_min")
	timeMax := r.Str("time_max")
	itemIDs := r.StrSlice("items")
	tz := r.Str("time_zone")
	calExp := r.Str("calendar_expansion_max")
	groupExp := r.Str("group_expansion_max")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	items := make([]map[string]any, 0, len(itemIDs))
	for _, id := range itemIDs {
		items = append(items, map[string]any{"id": id})
	}
	body := map[string]any{
		"timeMin": timeMin,
		"timeMax": timeMax,
		"items":   items,
	}
	if tz != "" {
		body["timeZone"] = tz
	}
	if calExp != "" {
		body["calendarExpansionMax"] = calExp
	}
	if groupExp != "" {
		body["groupExpansionMax"] = groupExp
	}
	data, err := g.post(ctx, "/freeBusy", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
