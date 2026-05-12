package compactyaml

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	"gopkg.in/yaml.v3"
)

// Options controls loader behavior.
type Options struct{ Strict bool }

// Result holds the parsed compaction specs and per-tool size caps.
type Result struct {
	Specs    map[mcp.ToolName][]mcp.CompactField
	MaxBytes map[mcp.ToolName]int
	Warnings []error
}

// Load parses a compact.yaml byte slice into a Result.
// In strict mode any invalid entry returns an error immediately.
// In lenient mode invalid entries are skipped and appended to Warnings.
func Load(data []byte, opts Options) (Result, error) {
	var sf SpecFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return Result{}, fmt.Errorf("compactyaml: parse: %w", err)
	}
	if sf.Version != 1 {
		return Result{}, fmt.Errorf("compactyaml: unsupported version %d (want 1)", sf.Version)
	}
	res := Result{
		Specs:    make(map[mcp.ToolName][]mcp.CompactField, len(sf.Tools)),
		MaxBytes: make(map[mcp.ToolName]int),
	}
	for name, cfg := range sf.Tools {
		fields, err := parseToolConfig(name, cfg)
		if err != nil && opts.Strict {
			return Result{}, err
		}
		if err != nil {
			res.Warnings = append(res.Warnings, err)
			continue
		}
		res.Specs[mcp.ToolName(name)] = fields
		if cfg.MaxBytes > 0 {
			res.MaxBytes[mcp.ToolName(name)] = cfg.MaxBytes
		}
	}
	return res, nil
}

// parseToolConfig validates one tool's config and returns the parsed spec fields.
func parseToolConfig(name string, cfg ToolConfig) ([]mcp.CompactField, error) {
	if cfg.MaxBytes < 0 {
		return nil, fmt.Errorf("compactyaml: tool %q: max_bytes must be >= 0, got %d", name, cfg.MaxBytes)
	}
	raw := make([]string, len(cfg.Spec))
	for i, s := range cfg.Spec {
		raw[i] = string(s)
	}
	fields, err := mcp.ParseCompactSpecs(raw)
	if err != nil {
		return nil, fmt.Errorf("compactyaml: tool %q: %w", name, err)
	}
	return fields, nil
}
