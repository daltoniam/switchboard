package gitlab

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay(integrationName, compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.FieldCompactionIntegration         = (*gitlab)(nil)
	_ mcp.ToolMaxBytesIntegration            = (*gitlab)(nil)
	_ mcp.PerToolMaxResponseBytesIntegration = (*gitlab)(nil)
)

func (g *gitlab) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (g *gitlab) MaxBytes(name mcp.ToolName) (int, bool) {
	limit, ok := maxBytesByTool[name]
	return limit, ok
}
