package webfetch

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "web_fetch",
		Description: "Fetches the content of a URL and returns it as readable text. " +
			"Use this to read documentation, API references, README files, changelogs, " +
			"GitHub raw content, package docs, or any web page whose content you need to " +
			"reason about. Returns extracted readable text, not raw HTML. " +
			"Start here for web browsing, URL reading, and online documentation lookup.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("url"), Description: "The full URL to fetch (https only).", Required: true}, {Name: mcp.ParamName("timeout"), Description: "Request timeout in seconds (default 10, max 30)."}},
	},
}
