package pganalyze

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

// staticTools defines the known pganalyze MCP tool definitions.
// These are used for search indexing when the proxy has not yet connected.
// When the proxy connects successfully, tools are dynamically refreshed from the MCP server.
var staticTools = mcp.MustLoadToolsYAML(toolsYAML)
