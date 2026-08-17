package compact

import (
	"fmt"

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

// ViewSelection is the resolved selection: ViewSet defaults filled in,
// validated against the registered renderers. The pipeline trusts this
// value; it does not re-validate.
type ViewSelection struct {
	View   ViewName
	Format Format
}

// ViewArgs is the LLM-requested view/format, parsed from raw args at the
// MCP boundary. Empty fields mean "not specified" — they're filled in
// from ViewSet.Default downstream by ResolveSelection.
//
// This is the typed input to the response-processing pipeline. Lifting
// the args boundary to this struct turns "caller forgot to thread args
// through" from a runtime nil-map check into a compile-time non-issue:
// processResult takes ViewArgs (struct, no nil possible), so the leak
// the old map[string]any signature allowed is structurally unreachable.
//
// The err field captures type errors discovered while parsing (e.g. the
// LLM passed view=123 instead of a string). Carrying the error inside
// the value keeps callsites to a single line — downstream code consults
// Err() before dispatching.
type ViewArgs struct {
	View   ViewName
	Format Format
	err    error
}

// Err returns the parse error, if any. nil means the args were
// well-typed; non-nil means downstream dispatch should surface this
// as a view_dispatch_failed envelope instead of attempting to render.
func (v ViewArgs) Err() error { return v.err }

// ParseViewArgs extracts the view/format selection from the raw args
// map. Missing keys produce empty fields (legitimate — "LLM didn't
// specify"). Wrong-typed values produce an embedded error reachable
// via Err(); the partial struct is still returned so callers can
// continue threading it through the pipeline.
//
// Nil args is fine — it produces an empty ViewArgs with no error.
// This is by design: ViewArgs{} and ParseViewArgs(nil) are equivalent
// and both mean "use ViewSet defaults". The old nil-vs-empty-map
// ambiguity is gone because nil is no longer a valid input *type* —
// processResult and friends now take ViewArgs, not map[string]any.
func ParseViewArgs(args map[string]any) ViewArgs {
	var v ViewArgs
	if raw, ok := args[ArgView]; ok {
		s, ok := raw.(string)
		if !ok {
			v.err = fmt.Errorf("arg %q must be string, got %T", ArgView, raw)
			return v
		}
		v.View = ViewName(s)
	}
	if raw, ok := args[ArgFormat]; ok {
		s, ok := raw.(string)
		if !ok {
			v.err = fmt.Errorf("arg %q must be string, got %T", ArgFormat, raw)
			return v
		}
		v.Format = Format(s)
	}
	return v
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
