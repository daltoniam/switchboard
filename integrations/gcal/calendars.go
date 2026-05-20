package gcal

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// ── CalendarList (the user's subscribed calendars) ──────────────────

func listCalendarList(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"maxResults":    r.Str("max_results"),
		"pageToken":     r.Str("page_token"),
		"showDeleted":   r.Str("show_deleted"),
		"showHidden":    r.Str("show_hidden"),
		"minAccessRole": r.Str("min_access_role"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/users/me/calendarList%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCalendarListEntry(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := r.Str("calendar_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/users/me/calendarList/%s", pathEscape(cid))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func buildCalendarListBody(r *mcp.Args) (map[string]any, error) {
	if raw := r.Str("body"); raw != "" {
		var out map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON for body: %w", err)
		}
		return out, nil
	}
	body := map[string]any{}
	if v := r.Str("background_color"); v != "" {
		body["backgroundColor"] = v
	}
	if v := r.Str("foreground_color"); v != "" {
		body["foregroundColor"] = v
	}
	if v := r.Str("color_id"); v != "" {
		body["colorId"] = v
	}
	if v := r.Str("hidden"); v != "" {
		body["hidden"] = v == "true"
	}
	if v := r.Str("selected"); v != "" {
		body["selected"] = v == "true"
	}
	if v := r.Str("summary_override"); v != "" {
		body["summaryOverride"] = v
	}
	return body, nil
}

func insertCalendarList(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := r.Str("calendar_id")
	colorFmt := r.Str("color_rgb_format")
	body, berr := buildCalendarListBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body["id"] = cid
	q := queryEncode(map[string]string{"colorRgbFormat": colorFmt})
	path := fmt.Sprintf("/users/me/calendarList%s", q)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCalendarList(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := r.Str("calendar_id")
	colorFmt := r.Str("color_rgb_format")
	body, berr := buildCalendarListBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"colorRgbFormat": colorFmt})
	path := fmt.Sprintf("/users/me/calendarList/%s%s", pathEscape(cid), q)
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteCalendarList(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := r.Str("calendar_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/users/me/calendarList/%s", pathEscape(cid))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Calendars (the underlying calendar resource) ────────────────────

func getCalendar(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/calendars/%s", pathEscape(cid))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func buildCalendarBody(r *mcp.Args) (map[string]any, error) {
	if raw := r.Str("body"); raw != "" {
		var out map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON for body: %w", err)
		}
		return out, nil
	}
	body := map[string]any{}
	if v := r.Str("summary"); v != "" {
		body["summary"] = v
	}
	if v := r.Str("description"); v != "" {
		body["description"] = v
	}
	if v := r.Str("location"); v != "" {
		body["location"] = v
	}
	if v := r.Str("time_zone"); v != "" {
		body["timeZone"] = v
	}
	return body, nil
}

func createCalendar(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body, berr := buildCalendarBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, "/calendars", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCalendar(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := r.Str("calendar_id")
	body, berr := buildCalendarBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/calendars/%s", pathEscape(cid))
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteCalendar(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := r.Str("calendar_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/calendars/%s", pathEscape(cid))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func clearCalendar(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/calendars/%s/clear", pathEscape(cid))
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
