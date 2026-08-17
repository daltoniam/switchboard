package gdocs

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Document lifecycle ──────────────────────────────────────────
	{
		Name: mcp.ToolName("gdocs_get_document"), Description: "Retrieve (get) a Google Docs document by ID, including its full structured content (paragraphs, tables, lists, headings, images, footnotes). Start here for reading document text, getting word counts, finding sections, or fetching raw structure for analysis. Returns the document body as markdown via the renderer.",
		Parameters: map[string]string{
			"document_id":           "The Google Docs document ID (the long string in the document URL)",
			"suggestions_view_mode": "How suggested edits are surfaced: DEFAULT_FOR_CURRENT_ACCESS, SUGGESTIONS_INLINE, PREVIEW_SUGGESTIONS_ACCEPTED, PREVIEW_WITHOUT_SUGGESTIONS",
			"include_tabs_content":  "true/false — include content of all tabs (default false; the API returns the active tab only otherwise)",
		},
		Required: []string{"document_id"},
	},
	{
		Name: mcp.ToolName("gdocs_create_document"), Description: "Create a new, empty Google Docs document. Returns the new document including its ID, which you can then pass to gdocs_batch_update, gdocs_insert_text, or gdocs_append_text to add content. To save the document into a specific Drive folder, use gdrive_update_file afterward to set parents.",
		Parameters: map[string]string{
			"title": "Document title (defaults to 'Untitled document')",
		},
	},

	// ── Edits: power + convenience ──────────────────────────────────
	{
		Name: mcp.ToolName("gdocs_batch_update"), Description: "Apply a batch of edit requests to a Google Docs document — the full power of the Docs API. Use after gdocs_get_document to see existing indices. Each request is one of: insertText, deleteContentRange, replaceAllText, insertTableRow, insertPageBreak, updateTextStyle, updateParagraphStyle, createParagraphBullets, deleteParagraphBullets, insertInlineImage, and many more. Prefer the convenience tools (gdocs_insert_text, gdocs_append_text, gdocs_replace_text, gdocs_delete_content) for simple edits; use this for compound or formatting changes.",
		Parameters: map[string]string{
			"document_id":            "The document ID to edit",
			"requests":               "JSON array of Docs API Request objects (e.g. [{\"insertText\":{\"location\":{\"index\":1},\"text\":\"Hello\"}}])",
			"write_control_revision": "Optional required_revision_id — only apply if the document is at this revision",
			"write_control_target":   "Optional target_revision_id — only apply if the document is at or below this revision (use revision_id from gdocs_get_document)",
		},
		Required: []string{"document_id", "requests"},
	},
	{
		Name: mcp.ToolName("gdocs_insert_text"), Description: "Insert plain text at a specific character index in a Google Docs document. Convenience wrapper over gdocs_batch_update for the most common edit. Use gdocs_get_document first to find the index; index 1 is the start of the document body (index 0 is reserved).",
		Parameters: map[string]string{
			"document_id": "The document ID",
			"text":        "The text to insert (may contain newlines for paragraph breaks)",
			"index":       "Insertion index (1-based; 1 = start of body). Use gdocs_append_text to insert at end.",
		},
		Required: []string{"document_id", "text", "index"},
	},
	{
		Name: mcp.ToolName("gdocs_append_text"), Description: "Append text to the end of a Google Docs document. Convenience wrapper that fetches the doc, computes the end index, and inserts. The simplest way to add content without needing to track indices.",
		Parameters: map[string]string{
			"document_id":     "The document ID",
			"text":            "The text to append",
			"leading_newline": "true/false — prepend a newline before the appended text (default true for clean separation)",
		},
		Required: []string{"document_id", "text"},
	},
	{
		Name: mcp.ToolName("gdocs_replace_text"), Description: "Find and replace all occurrences of a string in a Google Docs document. Convenience wrapper over the Docs API's replaceAllText request. Use match_case=true for case-sensitive replacement.",
		Parameters: map[string]string{
			"document_id": "The document ID",
			"find":        "Text to find",
			"replace":     "Replacement text (use empty string to delete all matches)",
			"match_case":  "true/false — case-sensitive matching (default false)",
		},
		Required: []string{"document_id", "find", "replace"},
	},
	{
		Name: mcp.ToolName("gdocs_delete_content"), Description: "Delete a range of content (text, images, tables) between two character indices in a Google Docs document. Convenience wrapper over the Docs API's deleteContentRange request. Use gdocs_get_document to find the indices first.",
		Parameters: map[string]string{
			"document_id": "The document ID",
			"start_index": "Start of range to delete (inclusive)",
			"end_index":   "End of range to delete (exclusive)",
		},
		Required: []string{"document_id", "start_index", "end_index"},
	},
}
