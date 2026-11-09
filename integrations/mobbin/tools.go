package mobbin

import mcp "github.com/daltoniam/switchboard"

// Parameter descriptions shared by several upstream tools.
const (
	paramPlatform          = "Platform: ios or web. Do not put the platform in query."
	paramImageFormat       = "Preview image format: webp (default) or jpg when the client cannot read webp."
	paramTaskIntent        = "One short English sentence summarizing the user's overall task; keep it identical across calls for the same task. No personal data or verbatim messages."
	paramOutputDestination = "What the results are for: code, design_tool, doc, reference_library (only when saving is the ask), chat, or other. Keep it identical across calls for the same task."
	paramOutputTool        = "Product the results go into next, such as Figma or Notion. Omit when unknown."
	paramCollectionID      = "Collection UUID from mobbin_list_collections, or all-saved for everything the user saved."
	paramContentType       = "Kind of item: screens, flows, or sections."
)

// tools mirrors the hosted Mobbin MCP catalog (https://api.mobbin.com/mcp).
// Names keep the upstream tool name behind the mobbin_ prefix so Execute can
// forward them unchanged. Results carry inline low-resolution preview images;
// cite each item's mobbin_url and download image_url for high resolution.
var tools = []mcp.ToolDefinition{
	{
		Name:        "mobbin_search_screens",
		Description: "Search Mobbin's library of real app UI screens and screenshots with natural language (paywall, settings screen, checkout page, login). Start here for UI design inspiration, UX patterns, and mobile app or web app screen references. Returns screens with inline preview images, app names, mobbin_url links, and high-resolution image_url.",
		Parameters: map[string]string{
			"query":              "One screen in plain language, with the kind of app when known, e.g. \"banking app onboarding welcome screen\" or \"Spotify now-playing screen\". One intent per search; avoid vague style words.",
			"platform":           paramPlatform,
			"mode":               "deep (default, AI-ranked relevance for nuanced queries) or standard (lower latency).",
			"limit":              "Maximum screens to return, 1-30 (default 20). More screens use more context.",
			"exclude_screen_ids": "Screen UUIDs to leave out, for fetching more results after a first search.",
			"image_format":       paramImageFormat,
			"task_intent":        paramTaskIntent,
			"output_destination": paramOutputDestination,
			"output_tool":        paramOutputTool,
		},
		Required: []string{"platform", "query"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "mobbin_search_flows",
		Description: "Search Mobbin for multi-step user flows and UX journeys in real apps (onboarding, checkout, password reset, signup). Use for flow design references and step-by-step screen sequences. Returns flows with evenly spaced preview images and per-screen metadata.",
		Parameters: map[string]string{
			"query":              "One user journey in plain language, with the kind of app when known, e.g. \"fitness app onboarding\" or \"Duolingo onboarding\".",
			"platform":           paramPlatform,
			"limit":              "Maximum flows to return, 1-10 (default 5).",
			"page":               "Page number, 1-20 (default 1).",
			"image_format":       paramImageFormat,
			"task_intent":        paramTaskIntent,
			"output_destination": paramOutputDestination,
			"output_tool":        paramOutputTool,
		},
		Required: []string{"query", "platform"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "mobbin_search_sections",
		Description: "Search Mobbin for website sections and landing page design references (hero, pricing, about, footer, testimonials). Returns website sections with inline preview images and metadata.",
		Parameters: map[string]string{
			"query":              "One website section in plain language, with the kind of product when known, e.g. \"AI tools pricing section\" or \"hero section with signup form\".",
			"limit":              "Maximum sections to return, 1-30 (default 20).",
			"page":               "Page number (default 1).",
			"image_format":       paramImageFormat,
			"task_intent":        paramTaskIntent,
			"output_destination": paramOutputDestination,
			"output_tool":        paramOutputTool,
		},
		Required: []string{"query"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "mobbin_list_collections",
		Description: "List the user's saved Mobbin collections and All saved, most recently changed first, with counts of saved iOS and web screens, flows, and website sections. Use before mobbin_get_collection_contents, mobbin_save_content, or mobbin_create_collection. Collection names are not unique.",
		Parameters: map[string]string{
			"task_intent":        paramTaskIntent,
			"output_destination": paramOutputDestination,
			"output_tool":        paramOutputTool,
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "mobbin_get_collection_contents",
		Description: "Get one page of the screens, flows, or website sections saved in a Mobbin collection or in all-saved, with preview images and team comments. Use after mobbin_list_collections. One content_type per call; items come newest save first.",
		Parameters: map[string]string{
			"id":                 paramCollectionID,
			"content_type":       paramContentType,
			"platform":           "Required for screens and flows: ios or web. Ignored for sections.",
			"page":               "1-based page, at most 20. has_next_page says whether another follows.",
			"limit":              "Items per page, 1-30 (default 20 for screens and sections, 5 for flows; flows allow at most 10). Keep it constant while paging.",
			"image_format":       paramImageFormat,
			"task_intent":        paramTaskIntent,
			"output_destination": paramOutputDestination,
			"output_tool":        paramOutputTool,
		},
		Required: []string{"id", "content_type"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "mobbin_create_collection",
		Description: "Create a new private Mobbin collection and return its id and link. It starts empty; add items with mobbin_save_content. Check mobbin_list_collections first because collection names are not unique.",
		Parameters: map[string]string{
			"name":        "Collection title on Mobbin.",
			"description": "What the collection is for, shown under its name.",
			"task_intent": paramTaskIntent,
		},
		Required: []string{"name"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "mobbin_save_content",
		Description: "Save screens, flows, or website sections to a Mobbin collection or to all-saved. Use after a Mobbin search or mobbin_get_collection_contents, only when the user asks to save items. All or nothing; repeating the same request is safe.",
		Parameters: map[string]string{
			"collection_id": paramCollectionID + " Saving to a collection also saves to all-saved.",
			"contents":      "Items to save: array of {id, content_type} where id is a screen, flow, or section UUID from a Mobbin result and content_type is screens, flows, or sections. Never invent ids.",
			"task_intent":   paramTaskIntent,
		},
		Required: []string{"collection_id", "contents"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "mobbin_add_comment",
		Description: "Add a comment or note to a Mobbin screen, flow, or website section that the user's team can see. Use after a Mobbin search or mobbin_get_collection_contents, only when the user asks. Each call posts a new comment; unsaved items are saved to all-saved first.",
		Parameters: map[string]string{
			"content_id":   "Screen, flow, or section UUID from a Mobbin result. Never invent ids.",
			"content_type": paramContentType,
			"comment":      "Plain text; line breaks are kept and @mentions notify no one.",
			"task_intent":  paramTaskIntent,
		},
		Required: []string{"content_id", "content_type", "comment"},
		Access:   mcp.AccessWrite,
	},
}
