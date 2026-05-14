package compact

import (
	mcp "github.com/daltoniam/switchboard"
)

// ViewSet is one tool's complete view configuration, resolved at load time.
//
// Default names the (view, format) combo that applies when args omit them.
// Views holds the parsed config for each named view, keyed by ViewName.
// Renderers is the load-time PDV output: every (view, format) combo declared
// in the YAML is resolved to a callable renderer here. Absence in this map
// means the combo was never declared and must return an error envelope at
// runtime — there is no fallback.
type ViewSet struct {
	Default   ViewSelection
	Views     map[ViewName]ParsedView
	Renderers map[ViewName]map[Format]Renderer
}

// ParsedView mirrors ViewConfig with the spec parsed into []mcp.CompactField.
// Adapters and the server pipeline read these typed fields directly, never
// re-parsing at runtime.
type ParsedView struct {
	Spec     []mcp.CompactField
	Hint     string
	Formats  []Format
	MaxBytes int
}

// ViewSelection is the parsed result of "what did the LLM ask for?" — the
// typed handoff between the args boundary and the pipeline. The pipeline
// trusts this value; it does not re-validate.
type ViewSelection struct {
	View   ViewName
	Format Format
}

// RenderKey identifies one (tool, view, format) triple in adapter-provided
// renderer registries. Adapters list these in Options.Renderers when they
// want to override a framework default formatter for a specific combo.
type RenderKey struct {
	Tool   mcp.ToolName
	View   ViewName
	Format Format
}

// Renderer turns a projected Go value (whatever survived the view's spec)
// into bytes in the target format. Returning an error fails the dispatch
// and bubbles up as an error envelope to the LLM.
type Renderer func(projected any) ([]byte, error)

// ToolViewsIntegration is the opt-in interface adapters implement when a
// tool exposes multiple views. The server pipeline calls Views() with the
// tool name and dispatches the (view, format) selection against the
// returned ViewSet. Adapters whose tools use only the flat (spec-only)
// form don't implement this interface — the server stays on the existing
// compaction path.
//
// This interface lives in the compact package (rather than mcp alongside
// other port interfaces) because ViewSet is intrinsic to the loader, and
// pulling ViewSet up to mcp would introduce mutual import dependencies.
// Adapters already import compact for MustLoadWithOverlay; satisfying
// this interface alongside is incremental.
type ToolViewsIntegration interface {
	Views(toolName mcp.ToolName) (ViewSet, bool)
}

// Argument names ViewSet dispatch reads from the args map. These are the
// single source of truth: parseViewSelection reads args[ArgView] and
// args[ArgFormat], and the MCP arg validator skips these keys when the
// tool implements ToolViewsIntegration. Drift between the two callers
// would either reject valid input or silently miss a reserved arg.
const (
	ArgView   = "view"
	ArgFormat = "format"
)

// ReservedArgs returns the argument names consumed by ViewSet dispatch.
// The MCP arg validator queries this when a tool implements
// ToolViewsIntegration so view/format pass through to parseViewSelection
// instead of failing as unknown parameters. A future field (e.g. an arg
// that selects a composition slot) would be added to this list, and the
// validator inherits the new tolerance without further changes.
func ReservedArgs() []string {
	return []string{ArgView, ArgFormat}
}
