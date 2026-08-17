package compact

// RawSpec is a single unparsed compaction spec line in dot-notation
// (e.g. "issues.nodes[].id", "-*_url", "field:alias"). Parsing produces
// an mcp.CompactField. The named type signals "do not pass downstream
// without parsing through mcp.ParseCompactSpecs".
type RawSpec string

// ViewName identifies one projection of a tool's output (e.g. "toc", "full").
// Together with Format, it picks one entry from the renderer registry.
type ViewName string

// Format identifies one serialization of a projected tool response
// (e.g. "json", "markdown", "text"). Reserved in the namespace even when
// not yet implemented — see Hyrum's law on stable name surfaces.
type Format string

// Known framework-default formats. Adapters may declare additional names
// in YAML only if they register a custom renderer for them via Options.
const (
	FormatJSON     Format = "json"
	FormatMarkdown Format = "markdown"
	FormatText     Format = "text"
)

// SpecFile is the top-level structure of a compact.yaml file.
type SpecFile struct {
	Version int                   `yaml:"version"`
	Tools   map[string]ToolConfig `yaml:"tools"`
}

// ToolConfig describes one tool's compaction configuration.
//
// Two shapes are valid:
//
//  1. Flat form: set Spec (and optionally MaxBytes). Single projection,
//     JSON output. Today's tools use this shape.
//
//  2. Multi-view form: set Views (one entry per projection) and Default
//     (which view + format applies when args omit them). Each view declares
//     its own spec, optional hint text, supported formats, and optional cap.
//
// Spec and Views are mutually exclusive — the loader validates this at parse.
// Designed to grow: future fields (Script, Compose, FetchFrom) can be added
// without breaking existing YAML or pipeline code.
type ToolConfig struct {
	// Flat form
	Spec     []RawSpec `yaml:"spec,omitempty"`
	MaxBytes int       `yaml:"max_bytes,omitempty"`

	// Multi-view form
	Views   map[string]ViewConfig `yaml:"views,omitempty"`
	Default *DefaultSelection     `yaml:"default,omitempty"`
}

// ViewConfig is one projection of a tool's output.
//
// Spec selects which fields survive compaction. Formats lists which
// serializations are available — the loader resolves each entry to a
// concrete renderer at load time; absence at runtime means the combo was
// never declared and returns an error envelope.
//
// Hint is the per-view help text surfaced in the response's _more
// envelope when the LLM has alternates available. Authors should write
// it as if the LLM is reading "I just got the default; what else could
// I get?" — be specific about size and use case.
type ViewConfig struct {
	Spec     []RawSpec `yaml:"spec"`
	Hint     string    `yaml:"hint,omitempty"`
	Formats  []Format  `yaml:"formats,omitempty"`
	MaxBytes int       `yaml:"max_bytes,omitempty"`
}

// DefaultSelection names the (view, format) combo that applies when args
// omit one or both. Required for multi-view tools; the loader validates
// that View exists in Views and Format is declared in that view's Formats.
type DefaultSelection struct {
	View   ViewName `yaml:"view"`
	Format Format   `yaml:"format"`
}
