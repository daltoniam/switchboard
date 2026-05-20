package gslides

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Presentation lifecycle ──────────────────────────────────────
	{
		Name: mcp.ToolName("gslides_get_presentation"), Description: "Retrieve (get) a Google Slides presentation by ID, including its slides (pages), layouts, masters, and page elements (shapes, text, tables, images). Start here when you need to discover slide IDs for batchUpdate, inspect existing content, or read the textual content of a deck. The response can be very large for content-rich decks — use gslides_get_page for a single slide if you only need one. For thumbnails, use gslides_get_page_thumbnail.",
		Parameters: map[string]string{
			"presentation_id": "The Google Slides presentation ID (the long string in the URL between /d/ and /edit)",
			"fields":          "Optional partial-response field mask (e.g. 'slides.objectId,title') to trim the response",
		},
		Required: []string{"presentation_id"},
	},
	{
		Name: mcp.ToolName("gslides_create_presentation"), Description: "Create a new Google Slides presentation. Returns the new presentation including its ID and an initial blank slide. Pass the returned presentation_id to gslides_batch_update to add slides and content. To save the presentation into a specific Drive folder, use gdrive_update_file afterward to set parents.",
		Parameters: map[string]string{
			"title": "Presentation title (defaults to 'Untitled presentation')",
		},
	},

	// ── Page (slide) access ─────────────────────────────────────────
	{
		Name: mcp.ToolName("gslides_get_page"), Description: "Retrieve a single page (slide) from a Google Slides presentation by its object ID. Much lighter than gslides_get_presentation when you only need one slide's content or page elements. The page_object_id comes from the slides[].objectId field of a presentation response.",
		Parameters: map[string]string{
			"presentation_id": "The presentation ID",
			"page_object_id":  "The object ID of the slide page (from slides[].objectId)",
		},
		Required: []string{"presentation_id", "page_object_id"},
	},
	{
		Name: mcp.ToolName("gslides_get_page_thumbnail"), Description: "Generate a thumbnail image URL for a single Google Slides page (slide). Returns a short-lived signed URL pointing to a PNG. Use thumbnail_size to control resolution (LARGE/MEDIUM/SMALL or unspecified). The mime_type defaults to PNG.",
		Parameters: map[string]string{
			"presentation_id": "The presentation ID",
			"page_object_id":  "The object ID of the slide page (from slides[].objectId)",
			"thumbnail_size":  "LARGE / MEDIUM / SMALL (default unspecified = medium)",
			"mime_type":       "PNG (default) — currently the only supported value",
		},
		Required: []string{"presentation_id", "page_object_id"},
	},

	// ── Presentation-level batchUpdate (the workhorse) ──────────────
	{
		Name: mcp.ToolName("gslides_batch_update"), Description: "Apply a batch of edit requests to a Google Slides presentation — the primary mutation API. Use for creating/deleting slides, inserting text, replacing text, formatting, inserting shapes/tables/images, moving page elements, applying themes, and so on. Pass 'requests' as a JSON array of Slides API Request objects (e.g. [{\"createSlide\":{}}, {\"insertText\":{\"objectId\":\"...\",\"text\":\"hello\"}}]). For pure read access, use gslides_get_presentation or gslides_get_page instead.",
		Parameters: map[string]string{
			"presentation_id":        "The presentation ID",
			"requests":               "JSON array of Slides API Request objects",
			"write_control_revision": "Optional required revision ID for optimistic concurrency control (will fail if the presentation has been edited since this revision)",
		},
		Required: []string{"presentation_id", "requests"},
	},
}
