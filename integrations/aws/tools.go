package aws

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)
