package gforms

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Form lifecycle ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("gforms_get_form"), Description: "Retrieve (get) a Google Forms form by ID, including all questions (items), info (title/description), settings, and the linked responder URL. Start here for surveys, quizzes, polls, questionnaires, feedback forms — to discover question item IDs for batchUpdate, inspect existing form structure, or read the form's response-collection settings. Use gforms_list_responses to fetch submitted answers.",
		Parameters: map[string]string{
			"form_id": "The Google Forms form ID (the long string in the URL between /d/ and /edit)",
		},
		Required: []string{"form_id"},
	},
	{
		Name: mcp.ToolName("gforms_create_form"), Description: "Create a new Google Forms form (survey, quiz, poll, questionnaire). Only the title can be set at creation time per the Forms API; add questions via gforms_batch_update afterward. Returns the new form including its formId and the linked responderUri (public URL for submitters).",
		Parameters: map[string]string{
			"title":          "Form title (shown to respondents)",
			"document_title": "Optional document title (shown in Drive). Defaults to the form title.",
		},
		Required: []string{"title"},
	},

	// ── Form structure mutation ─────────────────────────────────────
	{
		Name: mcp.ToolName("gforms_batch_update"), Description: "Apply a batch of edit requests to a Google Forms form — the primary mutation API. Use for adding/deleting questions (items), updating titles or descriptions, changing settings (quiz mode, response collection), and moving items. Pass 'requests' as a JSON array of Forms API Request objects (e.g. [{\"createItem\":{\"item\":{\"title\":\"Q1\",\"questionItem\":{\"question\":{\"textQuestion\":{}}}},\"location\":{\"index\":0}}}]). For pure read access, use gforms_get_form instead.",
		Parameters: map[string]string{
			"form_id":                       "The form ID",
			"requests":                      "JSON array of Forms API Request objects",
			"include_form_in_response":      "Optional boolean — if true, the response includes the updated form",
			"write_control_revision":        "Optional required revision ID for optimistic concurrency (will fail if the form has been edited since this revision)",
			"write_control_target_revision": "Optional target revision ID — overrides required_revision_id when present",
		},
		Required: []string{"form_id", "requests"},
	},

	// ── Response (submission) access ────────────────────────────────
	{
		Name: mcp.ToolName("gforms_list_responses"), Description: "List submitted responses (answers, submissions) for a Google Forms form. Returns paginated FormResponse objects with respondent answers keyed by question item ID. Use to analyze survey results, quiz submissions, feedback, or any collected form data. Combine with gforms_get_form to map question IDs to question text.",
		Parameters: map[string]string{
			"form_id":    "The form ID",
			"page_size":  "Optional max responses per page (1-5000, default 5000)",
			"page_token": "Optional pagination token from a previous response's nextPageToken",
			"filter":     "Optional filter (e.g. 'timestamp > 2024-01-01T00:00:00Z') — see Forms API filter syntax",
		},
		Required: []string{"form_id"},
	},
	{
		Name: mcp.ToolName("gforms_get_response"), Description: "Retrieve a single submitted response (answer, submission) from a Google Forms form by its response ID. Lighter than gforms_list_responses when you already have the responseId.",
		Parameters: map[string]string{
			"form_id":     "The form ID",
			"response_id": "The response ID (from FormResponse.responseId)",
		},
		Required: []string{"form_id", "response_id"},
	},
}
