package obsidian

import mcp "github.com/daltoniam/switchboard"

const pathParam = "Vault-relative path to the note, e.g. Projects/Plan.md (see obsidian_list_files)"

var tools = []mcp.ToolDefinition{
	// ── Discovery ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("obsidian_list_files"),
		Description: "List notes, files, and folders in an Obsidian vault directory (markdown knowledge base, second brain, personal notes). Start here to browse the vault and find note paths for read, search, and edit tools.",
		Parameters: map[string]string{
			"path": "Folder relative to the vault root, e.g. Daily or Projects/2026. Omit for the vault root. Entries ending in / are subfolders",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("obsidian_search"),
		Description: "Full-text search across all Obsidian notes in the vault. Returns matching note paths, scores, and surrounding text snippets. Use to find notes by keyword before obsidian_read_note.",
		Parameters: map[string]string{
			"query":          "Text to search for",
			"context_length": "Characters of surrounding context per match (default 100)",
			"limit":          "Maximum notes to return (default 20)",
		},
		Required: []string{"query"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("obsidian_query"),
		Description: "Structured query over Obsidian note metadata (frontmatter fields, tags, paths) using JsonLogic, with glob and regexp operators. Use to filter notes by tag, frontmatter value, or folder, e.g. {\"in\":[\"work\",{\"var\":\"tags\"}]} or {\"glob\":[\"Daily/*.md\",{\"var\":\"path\"}]}.",
		Parameters: map[string]string{
			"jsonlogic": "JsonLogic expression (object or JSON string) evaluated against each note's JSON (path, content, frontmatter, tags, stat). Notes with a truthy result are returned",
			"limit":     "Maximum notes to return (default 50)",
		},
		Required: []string{"jsonlogic"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("obsidian_list_tags"),
		Description: "List all tags used across Obsidian notes in the vault, with usage counts (inline #tags and frontmatter tags). Use with obsidian_query to find notes by tag.",
		Parameters:  map[string]string{},
		Access:      mcp.AccessRead,
	},

	// ── Reading ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("obsidian_read_note"),
		Description: "Read (get) an Obsidian note's markdown content, frontmatter, tags, links, and modified time. Optionally read just one heading section, block reference, or frontmatter field. Use after obsidian_list_files or obsidian_search.",
		Parameters: map[string]string{
			"path":              pathParam,
			"heading":           "Read only the section under this heading. A string for a top-level heading, or an array of headings from the top level down for a nested one, e.g. [\"Plan\", \"TODO\"]",
			"block":             "Read only the block with this reference id (with or without the leading ^)",
			"frontmatter_field": "Read only this frontmatter field's value",
		},
		Required: []string{"path"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("obsidian_get_document_map"),
		Description: "Get the outline of an Obsidian note: its heading tree, block reference ids, frontmatter field names, and a version token. Use before obsidian_patch_note to find valid targets, or before obsidian_read_note to read one section of a long note.",
		Parameters: map[string]string{
			"path": pathParam,
		},
		Required: []string{"path"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("obsidian_get_active_note"),
		Description: "Get the Obsidian note currently open (active file) in the editor, with content, frontmatter, and tags.",
		Parameters:  map[string]string{},
		Access:      mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("obsidian_list_commands"),
		Description: "List Obsidian commands (command palette actions from core and community plugins) that obsidian_execute_command can run.",
		Parameters: map[string]string{
			"query": "Case-insensitive filter on command id or name, e.g. daily or graph",
		},
		Access: mcp.AccessRead,
	},

	// ── Writing ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("obsidian_append_note"),
		Description: "Append markdown content to the end of an Obsidian note, creating the note if it does not exist. Use for journaling, daily notes, logs, and inbox capture.",
		Parameters: map[string]string{
			"path":    pathParam,
			"content": "Markdown to append",
		},
		Required: []string{"path", "content"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        mcp.ToolName("obsidian_patch_note"),
		Description: "Edit part of an Obsidian note in place: append, prepend, replace, or delete content under a heading, at a block reference, or in a frontmatter field (e.g. add a tag, set status). Use after obsidian_get_document_map.",
		Parameters: map[string]string{
			"path":                     pathParam,
			"operation":                "append, prepend, replace, or delete",
			"target_type":              "heading, block, or frontmatter",
			"target":                   "For heading: a heading string or an array of headings from the top level down. For block: the reference id. For frontmatter: the field name",
			"content":                  "Markdown text to insert. Provide content or value (neither for delete)",
			"value":                    "Typed JSON value: a frontmatter value (list, object, number, string) or table rows for a block that is a table. Provide content or value",
			"scope":                    "Part of the target to act on: content (default), marker (the heading text, block id, or field name; replace renames it), or markerAndContent (the whole section)",
			"create_target_if_missing": "true to create the heading or frontmatter field when it does not exist",
			"if_match":                 "Version token from obsidian_get_document_map; the edit fails with 412 if the note changed since",
		},
		Required: []string{"path", "operation", "target_type", "target"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        mcp.ToolName("obsidian_write_note"),
		Description: "Create or overwrite an Obsidian note with the given markdown content. Replaces the entire note; prefer obsidian_append_note or obsidian_patch_note to change part of an existing note.",
		Parameters: map[string]string{
			"path":    pathParam,
			"content": "Full markdown content of the note (may be empty)",
		},
		Required: []string{"path", "content"},
		Access:   mcp.AccessDestructive,
	},
	{
		Name:        mcp.ToolName("obsidian_delete_note"),
		Description: "Delete (remove) a note or file from the Obsidian vault. It moves to the trash according to the user's Obsidian deleted-files setting.",
		Parameters: map[string]string{
			"path": pathParam,
		},
		Required: []string{"path"},
		Access:   mcp.AccessDestructive,
	},
	{
		Name:        mcp.ToolName("obsidian_execute_command"),
		Description: "Run an Obsidian command by id (command palette action), such as opening today's daily note. Commands can change notes or app state. Use after obsidian_list_commands.",
		Parameters: map[string]string{
			"command_id": "Command id from obsidian_list_commands, e.g. daily-notes",
		},
		Required: []string{"command_id"},
		Access:   mcp.AccessDestructive,
	},
}
