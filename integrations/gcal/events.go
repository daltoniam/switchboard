package gcal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/google/uuid"
)

// ── Events ──────────────────────────────────────────────────────────

func listEvents(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"q":                       r.Str("q"),
		"timeMin":                 r.Str("time_min"),
		"timeMax":                 r.Str("time_max"),
		"singleEvents":            r.Str("single_events"),
		"orderBy":                 r.Str("order_by"),
		"maxResults":              r.Str("max_results"),
		"pageToken":               r.Str("page_token"),
		"showDeleted":             r.Str("show_deleted"),
		"updatedMin":              r.Str("updated_min"),
		"iCalUID":                 r.Str("ical_uid"),
		"timeZone":                r.Str("time_zone"),
		"privateExtendedProperty": r.Str("private_extended_property"),
		"sharedExtendedProperty":  r.Str("shared_extended_property"),
	}
	cid := calendarID(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/calendars/%s/events%s", pathEscape(cid), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	eid := r.Str("event_id")
	tz := r.Str("time_zone")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"timeZone": tz})
	data, err := g.get(ctx, "/calendars/%s/events/%s%s", pathEscape(cid), pathEscape(eid), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// buildEventBody assembles an Event resource from convenience args. If the
// caller passes a `body` arg, it overrides everything (escape hatch for
// fields we don't expose: attachments, custom reminders, extendedProperties).
func buildEventBody(r *mcp.Args) (map[string]any, error) {
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
	if v := r.Str("visibility"); v != "" {
		body["visibility"] = v
	}
	if v := r.Str("transparency"); v != "" {
		body["transparency"] = v
	}
	if v := r.Str("color_id"); v != "" {
		body["colorId"] = v
	}
	tz := r.Str("time_zone")
	if v := r.Str("start"); v != "" {
		body["start"] = eventTime(v, tz)
	}
	if v := r.Str("end"); v != "" {
		body["end"] = eventTime(v, tz)
	}
	if emails := r.StrSlice("attendees"); len(emails) > 0 {
		attendees := make([]map[string]any, 0, len(emails))
		for _, e := range emails {
			attendees = append(attendees, map[string]any{"email": e})
		}
		body["attendees"] = attendees
	}
	if rules := r.StrSlice("recurrence"); len(rules) > 0 {
		body["recurrence"] = rules
	}
	if v := r.Str("reminders_use_default"); v != "" {
		body["reminders"] = map[string]any{"useDefault": v == "true"}
	}
	if v := r.Str("create_meet"); v == "true" {
		body["conferenceData"] = map[string]any{
			"createRequest": map[string]any{
				"requestId":             "switchboard-" + uuid.New().String(),
				"conferenceSolutionKey": map[string]any{"type": "hangoutsMeet"},
			},
		}
	}
	return body, nil
}

// eventTime returns the Event.start/end shape:
//   - bare YYYY-MM-DD → {"date": "..."} (all-day event)
//   - anything else  → {"dateTime": "...", "timeZone": tz?}
func eventTime(v, tz string) map[string]any {
	if len(v) == 10 && strings.Count(v, "-") == 2 {
		return map[string]any{"date": v}
	}
	out := map[string]any{"dateTime": v}
	if tz != "" {
		out["timeZone"] = tz
	}
	return out
}

func createEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	sendUpdates := r.Str("send_updates")
	confVer := r.Str("conference_data_version")
	body, berr := buildEventBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"sendUpdates":           sendUpdates,
		"conferenceDataVersion": confVer,
	})
	path := fmt.Sprintf("/calendars/%s/events%s", pathEscape(cid), q)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	eid := r.Str("event_id")
	sendUpdates := r.Str("send_updates")
	raw := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
	}
	q := queryEncode(map[string]string{"sendUpdates": sendUpdates})
	path := fmt.Sprintf("/calendars/%s/events/%s%s", pathEscape(cid), pathEscape(eid), q)
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func patchEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	eid := r.Str("event_id")
	sendUpdates := r.Str("send_updates")
	body, berr := buildEventBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"sendUpdates": sendUpdates})
	path := fmt.Sprintf("/calendars/%s/events/%s%s", pathEscape(cid), pathEscape(eid), q)
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	eid := r.Str("event_id")
	sendUpdates := r.Str("send_updates")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"sendUpdates": sendUpdates})
	data, err := g.del(ctx, "/calendars/%s/events/%s%s", pathEscape(cid), pathEscape(eid), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func moveEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	eid := r.Str("event_id")
	dest := r.Str("destination")
	sendUpdates := r.Str("send_updates")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"destination": dest,
		"sendUpdates": sendUpdates,
	})
	path := fmt.Sprintf("/calendars/%s/events/%s/move%s", pathEscape(cid), pathEscape(eid), q)
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listEventInstances(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	eid := r.Str("event_id")
	params := map[string]string{
		"timeMin":       r.Str("time_min"),
		"timeMax":       r.Str("time_max"),
		"maxResults":    r.Str("max_results"),
		"pageToken":     r.Str("page_token"),
		"showDeleted":   r.Str("show_deleted"),
		"originalStart": r.Str("original_start"),
		"timeZone":      r.Str("time_zone"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/calendars/%s/events/%s/instances%s", pathEscape(cid), pathEscape(eid), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func quickAddEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	text := r.Str("text")
	sendUpdates := r.Str("send_updates")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"text":        text,
		"sendUpdates": sendUpdates,
	})
	path := fmt.Sprintf("/calendars/%s/events/quickAdd%s", pathEscape(cid), q)
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func importEvent(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	raw := r.Str("body")
	confVer := r.Str("conference_data_version")
	supportsAtt := r.Str("supports_attachments")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
	}
	q := queryEncode(map[string]string{
		"conferenceDataVersion": confVer,
		"supportsAttachments":   supportsAtt,
	})
	path := fmt.Sprintf("/calendars/%s/events/import%s", pathEscape(cid), q)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
