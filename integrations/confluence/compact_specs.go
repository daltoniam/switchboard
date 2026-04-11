package confluence

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Spaces ──────────────────────────────────────────────────────
	mcp.ToolName("confluence_list_spaces"): {
		"results[].id", "results[].key", "results[].name", "results[].type", "results[].status",
	},
	mcp.ToolName("confluence_get_space"): {
		"id", "key", "name", "type", "status", "description", "homepageId",
	},
	mcp.ToolName("confluence_search"): {
		"results[].content.id", "results[].content.type", "results[].content.title",
		"results[].content.status", "results[].content._links.webui",
		"results[].excerpt", "results[].url",
	},

	// ── Pages ───────────────────────────────────────────────────────
	mcp.ToolName("confluence_list_pages"): {
		"results[].id", "results[].title", "results[].status", "results[].spaceId",
		"results[].authorId", "results[].createdAt", "results[].version.number",
	},
	// confluence_get_page and confluence_get_blog_post are rendered as Markdown
	// by MarkdownIntegration.RenderMarkdown — these specs serve as fallback.
	mcp.ToolName("confluence_get_page"): {
		"id", "title", "status", "spaceId", "authorId", "createdAt",
		"version.number", "version.createdAt", "body.storage.value",
		"parentId", "parentType", "_links.webui",
	},
	mcp.ToolName("confluence_get_page_children"): {
		"results[].id", "results[].title", "results[].status", "results[].spaceId",
		"results[].authorId", "results[].createdAt", "results[].version.number",
	},

	// ── Blog Posts ──────────────────────────────────────────────────
	mcp.ToolName("confluence_list_blog_posts"): {
		"results[].id", "results[].title", "results[].status", "results[].spaceId",
		"results[].authorId", "results[].createdAt", "results[].version.number",
	},
	mcp.ToolName("confluence_get_blog_post"): {
		"id", "title", "status", "spaceId", "authorId", "createdAt",
		"version.number", "version.createdAt", "body.storage.value",
		"_links.webui",
	},

	// ── Comments ────────────────────────────────────────────────────
	// confluence_list_comments rendered as Markdown — spec serves as fallback.
	mcp.ToolName("confluence_list_comments"): {
		"results[].id", "results[].status", "results[].authorId",
		"results[].createdAt", "results[].version.number",
		"results[].body.storage.value",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("confluence: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
