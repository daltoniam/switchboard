package microsoft365

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func calendarBase(userID, calendarID string) string {
	base := userPath(userID)
	if calendarID == "" {
		return base + "/events"
	}
	return base + "/calendars/" + url.PathEscape(calendarID) + "/events"
}

func calendarViewBase(userID, calendarID string) string {
	base := userPath(userID)
	if calendarID == "" {
		return base + "/calendarView"
	}
	return base + "/calendars/" + url.PathEscape(calendarID) + "/calendarView"
}

func listCalendars(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		userID := r.Str("user_id")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return userPath(userID) + "/calendars" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func listEvents(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		userID := r.Str("user_id")
		calendarID := r.Str("calendar_id")
		start := r.Str("start")
		end := r.Str("end")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		if start != "" && end != "" {
			params["startDateTime"] = start
			params["endDateTime"] = end
			return calendarViewBase(userID, calendarID) + queryEncode(params), nil, nil
		}
		return calendarBase(userID, calendarID) + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func getEvent(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	calendarID := r.Str("calendar_id")
	eventID := r.Str("event_id")
	sel := r.Str("select")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.get(ctx, calendarBase(userID, calendarID)+"/"+url.PathEscape(eventID)+queryEncode(map[string]string{"$select": sel}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func eventDateTime(value, tz string) map[string]any {
	if tz == "" {
		tz = "UTC"
	}
	return map[string]any{"dateTime": value, "timeZone": tz}
}

func eventAttendees(emails string) []map[string]any {
	var out []map[string]any
	for _, addr := range splitCSV(emails) {
		out = append(out, map[string]any{
			"emailAddress": map[string]any{"address": addr},
			"type":         "required",
		})
	}
	return out
}

func eventPatchBody(r *mcp.Args) map[string]any {
	body := map[string]any{}
	if v := r.Str("subject"); v != "" {
		body["subject"] = v
	}
	if v := r.Str("body"); v != "" {
		bodyType := r.Str("body_type")
		if bodyType == "" {
			bodyType = "Text"
		}
		body["body"] = map[string]any{"contentType": bodyType, "content": v}
	}
	tz := r.Str("time_zone")
	if v := r.Str("start"); v != "" {
		body["start"] = eventDateTime(v, tz)
	}
	if v := r.Str("end"); v != "" {
		body["end"] = eventDateTime(v, tz)
	}
	if v := r.Str("location"); v != "" {
		body["location"] = map[string]any{"displayName": v}
	}
	if v := r.Str("attendees"); v != "" {
		body["attendees"] = eventAttendees(v)
	}
	if v := r.Str("is_online"); v == "true" {
		body["isOnlineMeeting"] = true
		body["onlineMeetingProvider"] = "teamsForBusiness"
	}
	return body
}

func createEvent(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	calendarID := r.Str("calendar_id")
	body := eventPatchBody(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.post(ctx, calendarBase(userID, calendarID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func updateEvent(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	calendarID := r.Str("calendar_id")
	eventID := r.Str("event_id")
	body := eventPatchBody(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.patch(ctx, calendarBase(userID, calendarID)+"/"+url.PathEscape(eventID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func deleteEvent(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	calendarID := r.Str("calendar_id")
	eventID := r.Str("event_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.del(ctx, calendarBase(userID, calendarID)+"/"+url.PathEscape(eventID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}
