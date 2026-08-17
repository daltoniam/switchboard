package gcal

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Events ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcal_list_events"), Description: "List events on a calendar with filters for time range, search query, recurrence, and updates. Start here for browsing the schedule, finding meetings, or checking what's on the calendar.",
		Parameters: map[string]string{
			"calendar_id":               "Calendar ID (defaults to 'primary' for the user's primary calendar)",
			"q":                         "Free-text search across summary, description, location, attendees",
			"time_min":                  "Lower bound (inclusive) RFC3339 timestamp, e.g. '2024-03-15T00:00:00Z'",
			"time_max":                  "Upper bound (exclusive) RFC3339 timestamp",
			"single_events":             "Expand recurring events into individual instances (true/false, default false)",
			"order_by":                  "Order: 'startTime' (requires single_events=true) or 'updated'",
			"max_results":               "Max results per page (default 250, max 2500)",
			"page_token":                "Token for next page",
			"show_deleted":              "Include cancelled events (true/false)",
			"updated_min":               "Only events updated at/after this RFC3339 timestamp",
			"ical_uid":                  "Filter to a specific iCalendar UID",
			"private_extended_property": "Repeated propertyName=value pairs, comma-separated, for private extended-property filters",
			"shared_extended_property":  "Repeated propertyName=value pairs, comma-separated, for shared extended-property filters",
			"time_zone":                 "Time zone used in response (default: calendar's time zone)",
		},
	},
	{
		Name: mcp.ToolName("gcal_get_event"), Description: "Get a single calendar event by ID. Read the full meeting details including attendees, location, attachments, and conference data.",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID (defaults to 'primary')",
			"event_id":    "Event ID",
			"time_zone":   "Time zone used in response (default: calendar's time zone)",
		},
		Required: []string{"event_id"},
	},
	{
		Name: mcp.ToolName("gcal_create_event"), Description: "Create a calendar event. Schedule a meeting or appointment with attendees, time, location, and notifications.",
		Parameters: map[string]string{
			"calendar_id":             "Calendar ID (defaults to 'primary')",
			"summary":                 "Event title",
			"description":             "Event description (plain text or HTML)",
			"location":                "Event location",
			"start":                   "Start time RFC3339 (e.g. '2024-03-15T14:00:00-07:00') OR YYYY-MM-DD for all-day",
			"end":                     "End time RFC3339 OR YYYY-MM-DD for all-day (exclusive)",
			"time_zone":               "Time zone for start/end (e.g. 'America/Los_Angeles')",
			"attendees":               "Comma-separated attendee emails",
			"recurrence":              "Comma-separated RRULE/EXRULE/RDATE/EXDATE lines (e.g. 'RRULE:FREQ=WEEKLY;COUNT=10')",
			"send_updates":            "Send notifications: all, externalOnly, none (default none)",
			"conference_data_version": "Set to '1' to enable conferenceData (e.g. auto-create Google Meet link via 'create_meet=true')",
			"create_meet":             "If true, attaches a Google Meet conference (requires conference_data_version=1)",
			"reminders_use_default":   "Use the calendar's default reminders (true/false)",
			"visibility":              "Visibility: default, public, private, confidential",
			"transparency":            "Show as: opaque (busy) or transparent (free)",
			"color_id":                "Color ID from gcal_get_colors event palette",
			"body":                    "Raw JSON body — overrides all other fields. Use for advanced cases (attachments, custom reminders, extendedProperties)",
		},
	},
	{
		Name: mcp.ToolName("gcal_update_event"), Description: "Replace a calendar event with PUT semantics — all fields must be provided. Prefer gcal_patch_event for partial updates.",
		Parameters: map[string]string{
			"calendar_id":  "Calendar ID (defaults to 'primary')",
			"event_id":     "Event ID",
			"body":         "Full JSON event resource (replaces the entire event)",
			"send_updates": "Send notifications: all, externalOnly, none (default none)",
		},
		Required: []string{"event_id", "body"},
	},
	{
		Name: mcp.ToolName("gcal_patch_event"), Description: "Partially update a calendar event. Edit a meeting's title, time, attendees, or other fields without replacing the entire event.",
		Parameters: map[string]string{
			"calendar_id":  "Calendar ID (defaults to 'primary')",
			"event_id":     "Event ID",
			"summary":      "New title (optional)",
			"description":  "New description (optional)",
			"location":     "New location (optional)",
			"start":        "New start time RFC3339 or YYYY-MM-DD (optional)",
			"end":          "New end time RFC3339 or YYYY-MM-DD (optional)",
			"time_zone":    "Time zone for start/end",
			"attendees":    "Replace attendee list with comma-separated emails",
			"recurrence":   "Replace recurrence rules (comma-separated lines)",
			"send_updates": "Send notifications: all, externalOnly, none (default none)",
			"visibility":   "Visibility: default, public, private, confidential",
			"transparency": "Show as: opaque or transparent",
			"color_id":     "Color ID",
			"body":         "Raw JSON patch body — overrides other fields",
		},
		Required: []string{"event_id"},
	},
	{
		Name: mcp.ToolName("gcal_delete_event"), Description: "Delete a calendar event. Cancel a meeting; optionally notify attendees.",
		Parameters: map[string]string{
			"calendar_id":  "Calendar ID (defaults to 'primary')",
			"event_id":     "Event ID",
			"send_updates": "Send cancellation notifications: all, externalOnly, none (default none)",
		},
		Required: []string{"event_id"},
	},
	{
		Name: mcp.ToolName("gcal_move_event"), Description: "Move an event to a different calendar (re-home a meeting from personal to shared, for example).",
		Parameters: map[string]string{
			"calendar_id":  "Source calendar ID (defaults to 'primary')",
			"event_id":     "Event ID",
			"destination":  "Destination calendar ID",
			"send_updates": "Send notifications: all, externalOnly, none (default none)",
		},
		Required: []string{"event_id", "destination"},
	},
	{
		Name: mcp.ToolName("gcal_list_instances"), Description: "List the individual instances of a recurring event. Expand a weekly meeting into the actual occurrences.",
		Parameters: map[string]string{
			"calendar_id":    "Calendar ID (defaults to 'primary')",
			"event_id":       "Recurring event ID",
			"time_min":       "Lower bound RFC3339",
			"time_max":       "Upper bound RFC3339",
			"max_results":    "Max results per page",
			"page_token":     "Token for next page",
			"show_deleted":   "Include cancelled instances (true/false)",
			"original_start": "Original start time of a specific instance (RFC3339)",
			"time_zone":      "Time zone used in response",
		},
		Required: []string{"event_id"},
	},
	{
		Name: mcp.ToolName("gcal_quick_add_event"), Description: "Create an event from a natural-language string (e.g. 'Lunch with Sarah Friday 1pm'). Quick way to schedule when you have a free-form text description.",
		Parameters: map[string]string{
			"calendar_id":  "Calendar ID (defaults to 'primary')",
			"text":         "Natural-language event description",
			"send_updates": "Send notifications: all, externalOnly, none (default none)",
		},
		Required: []string{"text"},
	},
	{
		Name: mcp.ToolName("gcal_import_event"), Description: "Import an event from an external calendar (preserves the iCalUID, organizer, and original creation time). Use for syncing events; use create_event for new events.",
		Parameters: map[string]string{
			"calendar_id":             "Calendar ID (defaults to 'primary')",
			"body":                    "Full JSON event resource including iCalUID and organizer",
			"conference_data_version": "Set to '1' to preserve conferenceData",
			"supports_attachments":    "Whether the API client supports event attachments (true/false)",
		},
		Required: []string{"body"},
	},

	// ── CalendarList (the user's subscribed calendars) ──────────────
	{
		Name: mcp.ToolName("gcal_list_calendars"), Description: "List the calendars in the user's calendar list. Discover which calendars are available to query (primary, secondary, shared, subscribed).",
		Parameters: map[string]string{
			"max_results":     "Max results per page (default 100, max 250)",
			"page_token":      "Token for next page",
			"show_deleted":    "Include deleted entries (true/false)",
			"show_hidden":     "Include hidden entries (true/false)",
			"min_access_role": "Filter by minimum access role: freeBusyReader, reader, writer, owner",
		},
	},
	{
		Name: mcp.ToolName("gcal_get_calendar_subscription"), Description: "Get a single calendar-list entry (the user-specific view of a subscribed calendar: color, hidden state, default reminders).",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID",
		},
		Required: []string{"calendar_id"},
	},
	{
		Name: mcp.ToolName("gcal_subscribe_calendar"), Description: "Subscribe to an existing calendar by inserting it into the user's calendar list.",
		Parameters: map[string]string{
			"calendar_id":      "Calendar ID to subscribe to",
			"color_rgb_format": "Set to 'true' to use the color_id/background_color/foreground_color values",
			"background_color": "Background color hex (e.g. '#ffffff')",
			"foreground_color": "Foreground color hex",
			"color_id":         "Color ID from gcal_get_colors calendar palette",
			"hidden":           "Hide from UI (true/false)",
			"selected":         "Show in calendar view (true/false)",
			"summary_override": "Custom display name for this user",
		},
		Required: []string{"calendar_id"},
	},
	{
		Name: mcp.ToolName("gcal_update_calendar_subscription"), Description: "Update the user-specific properties of a calendar subscription (color, hidden state, summary override).",
		Parameters: map[string]string{
			"calendar_id":      "Calendar ID",
			"color_rgb_format": "Set to 'true' to use the color hex values",
			"background_color": "Background color hex",
			"foreground_color": "Foreground color hex",
			"color_id":         "Color ID",
			"hidden":           "Hide from UI (true/false)",
			"selected":         "Show in calendar view (true/false)",
			"summary_override": "Custom display name for this user",
			"body":             "Raw JSON body — overrides other fields",
		},
		Required: []string{"calendar_id"},
	},
	{
		Name: mcp.ToolName("gcal_unsubscribe_calendar"), Description: "Remove a calendar from the user's calendar list (unsubscribe — does not delete the underlying calendar).",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID",
		},
		Required: []string{"calendar_id"},
	},

	// ── Calendars (the underlying calendar resource) ────────────────
	{
		Name: mcp.ToolName("gcal_get_calendar"), Description: "Get the metadata of a calendar resource (summary, description, time zone, location).",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID (defaults to 'primary')",
		},
	},
	{
		Name: mcp.ToolName("gcal_create_calendar"), Description: "Create a new secondary calendar owned by the user.",
		Parameters: map[string]string{
			"summary":     "Calendar title",
			"description": "Calendar description",
			"location":    "Geographic location of the calendar",
			"time_zone":   "Time zone (e.g. 'America/Los_Angeles')",
			"body":        "Raw JSON body — overrides other fields",
		},
		Required: []string{"summary"},
	},
	{
		Name: mcp.ToolName("gcal_update_calendar"), Description: "Update a calendar's metadata (summary, description, time zone).",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID",
			"summary":     "New calendar title",
			"description": "New description",
			"location":    "New location",
			"time_zone":   "New time zone",
			"body":        "Raw JSON patch body — overrides other fields",
		},
		Required: []string{"calendar_id"},
	},
	{
		Name: mcp.ToolName("gcal_delete_calendar"), Description: "Permanently delete a secondary calendar (cannot delete primary calendars; use gcal_clear_calendar for primary).",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID",
		},
		Required: []string{"calendar_id"},
	},
	{
		Name: mcp.ToolName("gcal_clear_calendar"), Description: "Clear all events from the user's primary calendar. Use only for the primary calendar; secondaries should be deleted instead.",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID (defaults to 'primary')",
		},
	},

	// ── ACL (sharing rules) ─────────────────────────────────────────
	{
		Name: mcp.ToolName("gcal_list_acl"), Description: "List the access control rules (sharing) for a calendar.",
		Parameters: map[string]string{
			"calendar_id":  "Calendar ID (defaults to 'primary')",
			"max_results":  "Max results per page",
			"page_token":   "Token for next page",
			"show_deleted": "Include deleted ACL entries (true/false)",
		},
	},
	{
		Name: mcp.ToolName("gcal_get_acl"), Description: "Get a single access control rule by ID.",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID (defaults to 'primary')",
			"rule_id":     "ACL rule ID",
		},
		Required: []string{"rule_id"},
	},
	{
		Name: mcp.ToolName("gcal_create_acl"), Description: "Grant access to a calendar. Share with a specific user, group, or domain.",
		Parameters: map[string]string{
			"calendar_id":        "Calendar ID (defaults to 'primary')",
			"role":               "Role: none, freeBusyReader, reader, writer, owner",
			"scope_type":         "Scope type: default, user, group, domain",
			"scope_value":        "Scope value (email for user/group, domain name for domain; omit for 'default')",
			"send_notifications": "Send notification email (true/false)",
			"body":               "Raw JSON body — overrides other fields",
		},
		Required: []string{"role", "scope_type"},
	},
	{
		Name: mcp.ToolName("gcal_update_acl"), Description: "Update a calendar access control rule (change the granted role).",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID (defaults to 'primary')",
			"rule_id":     "ACL rule ID",
			"role":        "New role: none, freeBusyReader, reader, writer, owner",
			"scope_type":  "Scope type (must match existing): default, user, group, domain",
			"scope_value": "Scope value",
			"body":        "Raw JSON body — overrides other fields",
		},
		Required: []string{"rule_id"},
	},
	{
		Name: mcp.ToolName("gcal_delete_acl"), Description: "Revoke a calendar access control rule (unshare).",
		Parameters: map[string]string{
			"calendar_id": "Calendar ID (defaults to 'primary')",
			"rule_id":     "ACL rule ID",
		},
		Required: []string{"rule_id"},
	},

	// ── Freebusy ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gcal_query_freebusy"), Description: "Check availability across calendars in a time range. Returns busy intervals — use to find open time slots for scheduling meetings.",
		Parameters: map[string]string{
			"time_min":               "Lower bound RFC3339",
			"time_max":               "Upper bound RFC3339",
			"items":                  "Comma-separated calendar IDs (or group IDs) to query",
			"time_zone":              "Time zone for the response",
			"calendar_expansion_max": "Max number of calendars to expand from groups (default 50)",
			"group_expansion_max":    "Max members per group expansion (default 100)",
			"body":                   "Raw JSON body — overrides other fields",
		},
		Required: []string{"time_min", "time_max", "items"},
	},

	// ── Settings + colors ───────────────────────────────────────────
	{
		Name: mcp.ToolName("gcal_list_settings"), Description: "List the user's Calendar settings (week start, timezone, default event length, working hours).",
		Parameters: map[string]string{
			"max_results": "Max results per page",
			"page_token":  "Token for next page",
		},
	},
	{
		Name: mcp.ToolName("gcal_get_setting"), Description: "Get a single user Calendar setting by ID.",
		Parameters: map[string]string{
			"setting_id": "Setting ID (e.g. 'timezone', 'weekStart', 'defaultEventLength')",
		},
		Required: []string{"setting_id"},
	},
	{
		Name: mcp.ToolName("gcal_get_colors"), Description: "Get the color palettes available for calendars and events. Returns the mapping of color IDs to hex values.",
		Parameters: map[string]string{},
	},
}
