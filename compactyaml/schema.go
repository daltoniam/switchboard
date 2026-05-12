package compactyaml

// RawSpec is a single unparsed compaction spec line in dot-notation
// (e.g. "issues.nodes[].id", "-*_url", "field:alias"). Parsing produces
// an mcp.CompactField. The named type signals "do not pass downstream
// without parsing through mcp.ParseCompactSpecs".
type RawSpec string

// SpecFile is the top-level structure of a compact.yaml file.
type SpecFile struct {
	Version int                   `yaml:"version"`
	Tools   map[string]ToolConfig `yaml:"tools"`
}

// ToolConfig holds the spec strings and optional response size cap for one tool.
type ToolConfig struct {
	Spec     []RawSpec `yaml:"spec"`
	MaxBytes int       `yaml:"max_bytes,omitempty"`
}
