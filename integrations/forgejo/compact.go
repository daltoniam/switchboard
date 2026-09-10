package forgejo

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("forgejo", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.FieldCompactionIntegration = (*forgejo)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*forgejo)(nil)
	_ mcp.MarkdownIntegration        = (*forgejo)(nil)
)

func (f *forgejo) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (f *forgejo) MaxBytes(name mcp.ToolName) (int, bool) {
	limit, ok := maxBytesByTool[name]
	return limit, ok
}
