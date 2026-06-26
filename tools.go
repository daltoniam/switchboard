package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// toolsDoc is the top-level envelope. Tools is kept as a raw yaml.Node so we
// can walk its Content (key/value alternating pairs) manually — this preserves
// declaration order and enables duplicate-key detection, neither of which is
// possible when decoding into a map[string]T.
type toolsDoc struct {
	Version int       `yaml:"version"`
	Tools   yaml.Node `yaml:"tools"`
}

// LoadToolsYAML parses an embedded tools YAML document into []ToolDefinition.
//
// Schema:
//
//	version: 1
//	tools:
//	  <tool_name>:
//	    description: "<string>"   # required
//	    parameters:               # optional; omit or use ~ for no parameters
//	      <param_name>:
//	        description: "<string>"
//	        required: true        # only `true` is permitted; absence means optional
//
// Strict-key behaviour: unknown keys at every level (top-level, tool entry,
// parameter entry) are rejected with an error naming the offending key. This
// catches typos like `descripton:` or `requird: true` at load time.
//
// Declaration order: tools and parameters are returned in the order they
// appear in the YAML document. Map-based decoding would randomise this order;
// the yaml.Node walk here preserves it.
//
// required semantics: absence of `required:` is equivalent to optional.
// Only `required: true` is meaningful; explicitly writing `required: false`
// is rejected as a parse error (absence already conveys optional).
//
// MustLoadToolsYAML wraps this function and panics on error. It is intended
// for use with embedded FS data at program init — not safe for user-supplied
// input.
func LoadToolsYAML(data []byte) ([]ToolDefinition, error) {
	if len(data) == 0 {
		return nil, errors.New("tools.yaml: empty input")
	}

	var raw toolsDoc
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("tools.yaml: parse: %w", err)
	}

	// Guard against multi-document YAML: a second Decode must return io.EOF.
	// Use `any` as the sentinel type so a well-formed second document parses
	// cleanly (err == nil) — with a struct{} target plus KnownFields(true) any
	// content errors, masking the multi-doc case behind a parse error.
	var rest any
	switch err := dec.Decode(&rest); {
	case err == nil:
		return nil, errors.New("tools.yaml: multi-document yaml not supported")
	case !errors.Is(err, io.EOF):
		return nil, fmt.Errorf("tools.yaml: trailing content after first document: %w", err)
	}

	if raw.Version != 1 {
		return nil, fmt.Errorf("tools.yaml: unsupported version %d (want 1)", raw.Version)
	}

	// tools: missing → Kind==0; tools: ~ → ScalarNode with tag !!null; tools: {} → empty MappingNode.
	toolsNode := raw.Tools
	isNull := toolsNode.Kind == yaml.ScalarNode && toolsNode.Tag == "!!null"
	isEmpty := toolsNode.Kind == yaml.MappingNode && len(toolsNode.Content) == 0
	if toolsNode.Kind == 0 || isNull || isEmpty {
		return nil, errors.New("tools.yaml: tools must be a non-empty mapping")
	}
	if toolsNode.Kind != yaml.MappingNode {
		return nil, errors.New("tools.yaml: tools must be a mapping")
	}

	seen := make(map[string]struct{}, len(toolsNode.Content)/2)
	out := make([]ToolDefinition, 0, len(toolsNode.Content)/2)

	for i := 0; i < len(toolsNode.Content); i += 2 {
		nameNode := toolsNode.Content[i]
		entryNode := toolsNode.Content[i+1]
		toolName := nameNode.Value

		if _, dup := seen[toolName]; dup {
			return nil, fmt.Errorf("tools.yaml: duplicate tool name %q", toolName)
		}
		seen[toolName] = struct{}{}

		desc, params, err := walkToolEntryNode(entryNode)
		if err != nil {
			return nil, fmt.Errorf("tools.yaml: tool %q: %w", toolName, err)
		}
		out = append(out, ToolDefinition{
			Name:        ToolName(toolName),
			Description: desc,
			Parameters:  params,
		})
	}
	return out, nil
}

// walkToolEntryNode walks a tool's value node (a mapping with keys
// "description" and optionally "parameters"). Any other key is an error.
func walkToolEntryNode(node *yaml.Node) (desc string, params []Parameter, err error) {
	if node.Kind != yaml.MappingNode {
		return "", nil, errors.New("expected mapping")
	}
	var paramsNode *yaml.Node
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "description":
			desc = val.Value
		case "parameters":
			paramsNode = val
		default:
			return "", nil, fmt.Errorf("unknown key %q", key)
		}
	}
	if desc == "" {
		return "", nil, errors.New("description must be non-empty")
	}
	if paramsNode == nil {
		return desc, nil, nil
	}
	params, err = walkParameterNode(paramsNode)
	if err != nil {
		return "", nil, err
	}
	return desc, params, nil
}

// walkParameterNode iterates a yaml.MappingNode and preserves declaration
// order. yaml.v3 populates MappingNode.Content as key, value, key, value, ...
// A missing key (Kind == 0) or null scalar (`parameters: ~`) is accepted and
// returns nil, nil.
func walkParameterNode(node *yaml.Node) ([]Parameter, error) {
	if node.Kind == 0 || (node.Kind == yaml.ScalarNode && node.Tag == "!!null") {
		return nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, errors.New("parameters: expected mapping")
	}
	out := make([]Parameter, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		var pe struct {
			description string
			required    bool
		}
		// Walk the parameter's value node strictly — no unknown keys permitted.
		if valNode.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("parameter %q: expected mapping", keyNode.Value)
		}
		for j := 0; j < len(valNode.Content); j += 2 {
			k := valNode.Content[j].Value
			v := valNode.Content[j+1]
			switch k {
			case "description":
				pe.description = v.Value
			case "required":
				if err := v.Decode(&pe.required); err != nil {
					return nil, fmt.Errorf("parameter %q: required: %w", keyNode.Value, err)
				}
				if !pe.required {
					return nil, fmt.Errorf("parameter %q: required: false is not allowed (omit the key — absence already conveys optional)", keyNode.Value)
				}
			default:
				return nil, fmt.Errorf("parameter %q: unknown key %q", keyNode.Value, k)
			}
		}
		if pe.description == "" {
			return nil, fmt.Errorf("parameter %q: description must be non-empty", keyNode.Value)
		}
		out = append(out, Parameter{
			Name:        ParamName(keyNode.Value),
			Description: pe.description,
			Required:    pe.required,
		})
	}
	return out, nil
}

// MustLoadToolsYAML panics at init on malformed YAML. Programmer errors
// (malformed embedded YAML at startup) fail loud; production never sees them.
func MustLoadToolsYAML(data []byte) []ToolDefinition {
	tools, err := LoadToolsYAML(data)
	if err != nil {
		panic(err)
	}
	return tools
}
