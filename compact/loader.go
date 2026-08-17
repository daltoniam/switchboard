package compact

import (
	"fmt"
	"slices"

	mcp "github.com/daltoniam/switchboard"
	"gopkg.in/yaml.v3"
)

// Options controls loader behavior.
//
// Strict toggles fail-fast on invalid entries: true at test time (CI gate)
// to catch broken YAML; false at runtime so a single bad entry doesn't
// disable the whole adapter.
//
// Renderers supplies adapter-specific renderers keyed by (tool, view,
// format). Anything not in this map falls back to the framework defaults
// (jsonRenderer, markdownRenderer). A declared format with neither an
// adapter renderer nor a framework default fails loading.
type Options struct {
	Strict    bool
	Renderers map[RenderKey]Renderer
}

// Result holds the parsed compaction config.
//
// Specs and MaxBytes carry the back-compat lookup: for both flat-form
// tools and multi-view tools, these map to the default-view spec/cap so
// existing adapter code paths (CompactSpec, MaxBytes) keep working.
//
// Views holds the multi-view detail, populated only for tools that use
// the `views:` form. Pipeline code with view dispatch reads this map.
type Result struct {
	Specs    map[mcp.ToolName][]mcp.CompactField
	MaxBytes map[mcp.ToolName]int
	Views    map[mcp.ToolName]ViewSet
	Warnings []error
}

// Load parses a compact.yaml byte slice into a Result.
// In strict mode any invalid entry returns an error immediately.
// In lenient mode invalid entries are skipped and appended to Warnings.
func Load(data []byte, opts Options) (Result, error) {
	var sf SpecFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return Result{}, fmt.Errorf("compact: parse: %w", err)
	}
	if sf.Version != 1 {
		return Result{}, fmt.Errorf("compact: unsupported version %d (want 1)", sf.Version)
	}
	res := Result{
		Specs:    make(map[mcp.ToolName][]mcp.CompactField, len(sf.Tools)),
		MaxBytes: make(map[mcp.ToolName]int),
		Views:    make(map[mcp.ToolName]ViewSet),
	}
	for name, cfg := range sf.Tools {
		toolName := mcp.ToolName(name)
		if err := loadOneTool(&res, toolName, cfg, opts); err != nil {
			if opts.Strict {
				return Result{}, err
			}
			res.Warnings = append(res.Warnings, err)
		}
	}
	return res, nil
}

// loadOneTool dispatches between the flat-form and multi-view-form paths.
// On error it leaves res unchanged for that tool (caller decides
// strict-vs-lenient).
func loadOneTool(res *Result, name mcp.ToolName, cfg ToolConfig, opts Options) error {
	hasSpec := len(cfg.Spec) > 0
	hasViews := len(cfg.Views) > 0
	switch {
	case hasSpec && hasViews:
		return fmt.Errorf("compact: tool %q: cannot set both `spec` and `views`; pick one", name)
	case hasViews:
		return loadMultiViewTool(res, name, cfg, opts)
	default:
		return loadFlatTool(res, name, cfg)
	}
}

// loadFlatTool handles the today-form: one spec, optional max_bytes.
func loadFlatTool(res *Result, name mcp.ToolName, cfg ToolConfig) error {
	fields, err := parseToolConfig(string(name), cfg)
	if err != nil {
		return err
	}
	res.Specs[name] = fields
	if cfg.MaxBytes > 0 {
		res.MaxBytes[name] = cfg.MaxBytes
	}
	return nil
}

// loadMultiViewTool handles the new form: multiple views with declared
// formats. Parses every view's spec, resolves every declared (view,
// format) to a renderer at load time, populates Result.Views.
//
// Back-compat: the default view's spec also populates Result.Specs and
// Result.MaxBytes so adapter code reading those maps (CompactSpec,
// MaxBytes methods) keeps working without view-awareness.
func loadMultiViewTool(res *Result, name mcp.ToolName, cfg ToolConfig, opts Options) error {
	if cfg.Default == nil {
		return fmt.Errorf("compact: tool %q: `views` requires a `default` (view + format)", name)
	}
	if _, ok := cfg.Views[string(cfg.Default.View)]; !ok {
		return fmt.Errorf("compact: tool %q: default.view %q does not exist in views", name, cfg.Default.View)
	}

	parsedViews := make(map[ViewName]ParsedView, len(cfg.Views))
	renderers := make(map[ViewName]map[Format]Renderer, len(cfg.Views))

	for viewKey, vc := range cfg.Views {
		view := ViewName(viewKey)
		parsed, err := parseOneView(vc)
		if err != nil {
			return fmt.Errorf("compact: tool %q: view %q: %w", name, viewKey, err)
		}
		parsedViews[view] = parsed

		viewRenderers, err := resolveViewRenderers(name, view, parsed.Formats, opts)
		if err != nil {
			return fmt.Errorf("compact: tool %q: view %q: %w", name, viewKey, err)
		}
		renderers[view] = viewRenderers
	}

	defaultView := cfg.Default.View
	defaultFormat := cfg.Default.Format
	if !containsFormat(parsedViews[defaultView].Formats, defaultFormat) {
		return fmt.Errorf("compact: tool %q: default.format %q is not declared in default.view %q's formats", name, defaultFormat, defaultView)
	}

	res.Views[name] = ViewSet{
		Default:   ViewSelection{View: defaultView, Format: defaultFormat},
		Views:     parsedViews,
		Renderers: renderers,
	}

	// Back-compat: mirror the default view's spec/cap into the flat maps.
	res.Specs[name] = parsedViews[defaultView].Spec
	if cap := parsedViews[defaultView].MaxBytes; cap > 0 {
		res.MaxBytes[name] = cap
	}
	return nil
}

// parseOneView validates one view's config and parses its spec into
// CompactField values. Returns a ParsedView ready to drop into the
// per-tool ViewSet.
func parseOneView(vc ViewConfig) (ParsedView, error) {
	if vc.MaxBytes < 0 {
		return ParsedView{}, fmt.Errorf("max_bytes must be >= 0, got %d", vc.MaxBytes)
	}
	raw := make([]string, len(vc.Spec))
	for i, s := range vc.Spec {
		raw[i] = string(s)
	}
	fields, err := mcp.ParseCompactSpecs(raw)
	if err != nil {
		return ParsedView{}, err
	}
	return ParsedView{
		Spec:     fields,
		Hint:     vc.Hint,
		Formats:  vc.Formats,
		MaxBytes: vc.MaxBytes,
	}, nil
}

// resolveViewRenderers resolves every (tool, view, format) combo in
// formats to a renderer at load time. The returned map IS the typed
// proof: lookup misses at runtime are unsupported combos.
func resolveViewRenderers(tool mcp.ToolName, view ViewName, formats []Format, opts Options) (map[Format]Renderer, error) {
	out := make(map[Format]Renderer, len(formats))
	for _, f := range formats {
		r, err := resolveRenderer(RenderKey{Tool: tool, View: view, Format: f}, opts)
		if err != nil {
			return nil, fmt.Errorf("format %q: %w", f, err)
		}
		out[f] = r
	}
	return out, nil
}

func containsFormat(formats []Format, target Format) bool {
	return slices.Contains(formats, target)
}

// parseToolConfig validates one tool's config and returns the parsed spec fields.
func parseToolConfig(name string, cfg ToolConfig) ([]mcp.CompactField, error) {
	if cfg.MaxBytes < 0 {
		return nil, fmt.Errorf("compact: tool %q: max_bytes must be >= 0, got %d", name, cfg.MaxBytes)
	}
	raw := make([]string, len(cfg.Spec))
	for i, s := range cfg.Spec {
		raw[i] = string(s)
	}
	fields, err := mcp.ParseCompactSpecs(raw)
	if err != nil {
		return nil, fmt.Errorf("compact: tool %q: %w", name, err)
	}
	return fields, nil
}
