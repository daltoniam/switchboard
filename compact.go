package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CompactField is a parsed field compaction spec — parse once via ParseCompactSpecs,
// then pass to CompactJSON on each request.
type CompactField struct {
	path      []string // e.g. ["user", "login"] or ["labels[]", "name"]
	outputKey string   // top-level key in the compacted output
	arrayIdx  int      // index of the "[]" segment in path, -1 if none
	arrayKey  string   // path[arrayIdx] without "[]", empty if arrayIdx == -1
	childPath []string // path[arrayIdx+1:], nil if arrayIdx == -1
}

// ParseCompactSpecs parses dot-notation spec strings into CompactFields.
// Call once at init; pass the result to CompactJSON on each request.
//
//   - "field"          → scalar value, output key = field name
//   - "parent.child"   → nested value, output key = dot-path ("user.login")
//   - "parent[].child" → array element extraction, output key = array base ("labels")
func ParseCompactSpecs(specs []string) ([]CompactField, error) {
	fields := make([]CompactField, 0, len(specs))
	for _, s := range specs {
		f, err := parseCompactSpec(s)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, nil
}

func parseCompactSpec(spec string) (CompactField, error) {
	if spec == "" {
		return CompactField{}, fmt.Errorf("compact spec: empty string")
	}
	if strings.HasPrefix(spec, ".") || strings.HasSuffix(spec, ".") {
		return CompactField{}, fmt.Errorf("compact spec: invalid %q", spec)
	}
	if strings.Contains(spec, "..") {
		return CompactField{}, fmt.Errorf("compact spec: invalid %q", spec)
	}

	parts := strings.Split(spec, ".")
	arrayIdx := -1
	for i, p := range parts {
		if !strings.HasSuffix(p, "[]") {
			continue
		}
		if i == len(parts)-1 {
			return CompactField{}, fmt.Errorf("compact spec: %q ends with [] (need a child field)", spec)
		}
		if arrayIdx >= 0 {
			return CompactField{}, fmt.Errorf("compact spec: %q has multiple [] (nested array traversal not supported)", spec)
		}
		arrayIdx = i
	}

	// "title" → "title", "user.login" → "user.login", "labels[].name" → "labels"
	var outputKey, arrayKey string
	var childPath []string
	switch {
	case arrayIdx >= 0:
		outputKey = strings.TrimSuffix(parts[arrayIdx], "[]")
		arrayKey = outputKey
		childPath = parts[arrayIdx+1:]
	case len(parts) == 1:
		outputKey = parts[0]
	default:
		outputKey = spec
	}

	return CompactField{
		path:      parts,
		outputKey: outputKey,
		arrayIdx:  arrayIdx,
		arrayKey:  arrayKey,
		childPath: childPath,
	}, nil
}

// CompactJSON applies field compaction to JSON data, keeping only the specified fields.
// Nil or empty fields returns data unchanged. Handles both objects and arrays.
func CompactJSON(data []byte, fields []CompactField) ([]byte, error) {
	if len(data) == 0 || len(fields) == 0 {
		return data, nil
	}

	switch data[0] {
	case '[':
		return compactArray(data, fields)
	case '{':
		return compactObjectJSON(data, fields)
	default:
		return nil, fmt.Errorf("compactJSON: expected JSON object or array, got %q", data[0])
	}
}

// compactObjectJSON unmarshals a single JSON object, applies field compaction, and re-marshals.
func compactObjectJSON(data []byte, fields []CompactField) ([]byte, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("compactJSON: %w", err)
	}
	return json.Marshal(compactObject(obj, fields))
}

// compactObject keeps only the specified fields from an unmarshalled object.
func compactObject(obj map[string]any, fields []CompactField) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		val, ok := extractField(obj, f)
		if !ok {
			continue
		}
		out[f.outputKey] = val
	}
	return out
}

// compactArray applies field compaction to each element in a JSON array.
// Unmarshals once into []any to avoid per-element unmarshal overhead.
func compactArray(data []byte, fields []CompactField) ([]byte, error) {
	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, fmt.Errorf("compactJSON: %w", err)
	}

	result := make([]any, 0, len(arr))
	for _, elem := range arr {
		obj, ok := elem.(map[string]any)
		if !ok {
			result = append(result, elem) // preserve non-object elements unchanged
			continue
		}
		result = append(result, compactObject(obj, fields))
	}

	return json.Marshal(result)
}

// extractField dispatches to the right extraction strategy based on the spec shape.
func extractField(obj map[string]any, f CompactField) (any, bool) {
	if len(f.path) == 1 {
		val, ok := obj[f.path[0]]
		return val, ok
	}

	if f.arrayIdx >= 0 {
		return extractArrayField(obj, f)
	}

	// Idempotence: after a first pass, "user.login" becomes a flat key and
	// the nested "user" object no longer exists. Look up the flat key directly.
	// Array specs skip this — their output key ("labels") collides with the source key.
	if val, ok := obj[f.outputKey]; ok {
		return val, true
	}

	return navigateToLeaf(obj, f.path)
}

// extractArrayField handles "labels[].name" specs — plucks a field from each array element.
// Uses precomputed arrayKey and childPath to avoid per-call string ops.
func extractArrayField(obj map[string]any, f CompactField) (any, bool) {
	raw, ok := obj[f.arrayKey]
	if !ok {
		return nil, false
	}

	arr, ok := raw.([]any)
	if !ok {
		return nil, false
	}

	result := make([]any, 0, len(arr))
	for _, elem := range arr {
		elemObj, ok := elem.(map[string]any)
		if !ok {
			// Already a scalar from a previous pass — preserve for idempotence.
			result = append(result, elem)
			continue
		}
		val, ok := navigateToLeaf(elemObj, f.childPath)
		if !ok {
			continue
		}
		result = append(result, val)
	}

	return result, true
}

// navigateToLeaf walks a dot-path through nested JSON objects to the leaf value.
func navigateToLeaf(obj map[string]any, path []string) (any, bool) {
	current := obj
	for i := 0; i < len(path)-1; i++ {
		next, ok := current[path[i]]
		if !ok {
			return nil, false
		}
		nested, ok := next.(map[string]any)
		if !ok {
			return nil, false
		}
		current = nested
	}

	val, ok := current[path[len(path)-1]]
	return val, ok
}
