package gmeet

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Spaces (meeting rooms) ──────────────────────────────────────
	{
		Name: mcp.ToolName("gmeet_create_space"), Description: "Create a new Google Meet meeting space (meeting room). Start here for create meet, new meeting, schedule call, video call, conference room, meeting URL. Returns a Space with meetingUri (the join URL to share), meetingCode (the short code like 'abc-defg-hij'), and a stable resource name 'spaces/{id}'. The space is persistent — anyone with the join URL can start a conference at any time. Optionally pass 'config' to control who can join and moderation settings.",
		Parameters: map[string]string{
			"config": "Optional SpaceConfig (JSON object or string) controlling join behavior. Common fields: accessType ('OPEN' = anyone with link, 'TRUSTED' = signed-in only, 'RESTRICTED' = invited only — default OPEN), entryPointAccess ('ALL' or 'CREATOR_APP_ONLY'), moderation ('ON'/'OFF'), moderationRestrictions (object). Example: {\"accessType\":\"TRUSTED\",\"moderation\":\"ON\"}",
		},
		Required: []string{},
	},
	{
		Name: mcp.ToolName("gmeet_get_space"), Description: "Retrieve a Google Meet space by resource name or meeting code. Accepts the full 'spaces/{id}', a bare id, or a meeting code like 'abc-defg-hij'. Returns the Space including meetingUri, meetingCode, config, and (if a conference is currently in progress) activeConference.conferenceRecord — the conference record id you can use to look up live participants.",
		Parameters: map[string]string{
			"name": "Space resource name ('spaces/{id}'), bare id, or meeting code ('abc-defg-hij'). Meeting codes are case-insensitive and hyphens are optional.",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("gmeet_update_space"), Description: "Update a Meet space's configuration (PATCH). Only fields named in update_mask are replaced. Use to change accessType, moderation, moderationRestrictions, or entryPointAccess on an existing space. Requires the meetings.space.settings OAuth scope.",
		Parameters: map[string]string{
			"name":        "Space resource name ('spaces/{id}') or bare id",
			"space":       "Space object (JSON object or string) with the new field values. Only the fields listed in update_mask are applied. Example: {\"config\":{\"accessType\":\"RESTRICTED\",\"moderation\":\"ON\"}}",
			"update_mask": "Required FieldMask of paths to update (e.g. 'config.accessType,config.moderation'). Valid: config.accessType, config.entryPointAccess, config.moderation, config.moderationRestrictions.chatRestriction, config.moderationRestrictions.reactionRestriction, config.moderationRestrictions.presentRestriction, config.moderationRestrictions.defaultJoinAsViewerType.",
		},
		Required: []string{"name", "space", "update_mask"},
	},
	{
		Name: mcp.ToolName("gmeet_end_active_conference"), Description: "End the conference currently in progress in a Meet space. All participants are disconnected. The space itself persists — anyone with the join link can start a new conference later. No-op (succeeds with empty body) if no conference is active.",
		Parameters: map[string]string{
			"name": "Space resource name ('spaces/{id}') or bare id",
		},
		Required: []string{"name"},
	},

	// ── Conference records (past meetings) ──────────────────────────
	{
		Name: mcp.ToolName("gmeet_list_conference_records"), Description: "List past Google Meet conferences (conference records) the user attended or owns. Use for queries like 'recent meetings', 'past calls', 'meetings I joined last week'. Returns each conference record with name, the space it was held in, startTime, and endTime. Supports an EBNF filter for date ranges, space, etc.",
		Parameters: map[string]string{
			"filter":     "Optional EBNF filter expression. Examples: 'space.meeting_code = \"abc-mnop-xyz\"', 'space.name = \"spaces/{spaceId}\"', 'start_time>=\"2024-01-01T00:00:00Z\" AND end_time<=\"2024-02-01T00:00:00Z\"'.",
			"page_size":  "Optional page size (1-50, default 10)",
			"page_token": "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{},
	},
	{
		Name: mcp.ToolName("gmeet_get_conference_record"), Description: "Retrieve a single past conference (conference record) by name. Use when you already have a conference record id from a previous list call or from a space's activeConference.",
		Parameters: map[string]string{
			"name": "Conference record resource name ('conferenceRecords/{id}') or bare id",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("gmeet_list_participants"), Description: "List participants who attended a past conference. Returns each participant with their identity (signedinUser.displayName + user.email when available; anonymous attendees are reported separately), earliestStartTime, and latestEndTime. Useful for 'who was in the meeting' / 'attendee list' queries.",
		Parameters: map[string]string{
			"conference_record": "Conference record resource name ('conferenceRecords/{id}') or bare id",
			"filter":            "Optional filter (e.g. 'earliest_start_time>=\"2024-01-01T00:00:00Z\"')",
			"page_size":         "Optional page size (1-100, default 100)",
			"page_token":        "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{"conference_record"},
	},
	{
		Name: mcp.ToolName("gmeet_list_recordings"), Description: "List video recordings for a past conference. Each recording links to a Google Drive file (driveDestination.file) and has a state (FILE_GENERATED = ready to view, STARTED = still recording, ENDED = stopped). Most conferences have at most one recording.",
		Parameters: map[string]string{
			"conference_record": "Conference record resource name ('conferenceRecords/{id}') or bare id",
			"page_size":         "Optional page size (1-10, default 10)",
			"page_token":        "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{"conference_record"},
	},
	{
		Name: mcp.ToolName("gmeet_list_transcripts"), Description: "List captions transcripts for a past conference. Each transcript is delivered as a Google Doc (docsDestination.document). Use gmeet_list_transcript_entries to retrieve individual spoken segments from a transcript without downloading the Doc.",
		Parameters: map[string]string{
			"conference_record": "Conference record resource name ('conferenceRecords/{id}') or bare id",
			"page_size":         "Optional page size (1-10, default 10)",
			"page_token":        "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{"conference_record"},
	},
	{
		Name: mcp.ToolName("gmeet_list_transcript_entries"), Description: "List individual transcript entries (spoken segments) from a conference transcript. Each entry has the participant who spoke, the text they said, languageCode, startTime, and endTime — useful for searching what was said in a meeting without parsing the Doc. Order is chronological.",
		Parameters: map[string]string{
			"transcript": "Transcript resource name ('conferenceRecords/{cid}/transcripts/{tid}'). Get it from gmeet_list_transcripts.",
			"page_size":  "Optional page size (1-1000, default 100)",
			"page_token": "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{"transcript"},
	},
}
