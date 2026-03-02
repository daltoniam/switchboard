package mcp

import (
	"bytes"
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
			continue // nested [] handled at extraction time
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

	trimmed := bytes.TrimLeft(data, " \t\n\r")
	if len(trimmed) == 0 {
		return data, nil
	}

	switch trimmed[0] {
	case '[':
		return compactArray(data, fields)
	case '{':
		return compactObjectJSON(data, fields)
	default:
		return nil, fmt.Errorf("compactJSON: expected JSON object or array, got %q", trimmed[0])
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
// Groups array fields by outputKey so multiple child specs on the same parent
// (e.g. steps[].name + steps[].conclusion) produce sub-objects, not overwrites.
func compactObject(obj map[string]any, fields []CompactField) map[string]any {
	out := make(map[string]any, len(fields))

	// Group array fields by outputKey to detect multi-field specs.
	arrayGroups := map[string][]CompactField{}
	for _, f := range fields {
		if f.arrayIdx >= 0 {
			arrayGroups[f.outputKey] = append(arrayGroups[f.outputKey], f)
			continue
		}
		val, ok := extractField(obj, f)
		if !ok {
			continue
		}
		out[f.outputKey] = val
	}

	for key, group := range arrayGroups {
		var val any
		var ok bool
		if len(group) == 1 {
			val, ok = extractArrayField(obj, group[0])
		} else {
			val, ok = extractArrayFieldGroup(obj, group)
		}
		if ok {
			out[key] = val
		}
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

// navigateToArray walks path segments before arrayIdx to reach the object
// containing the array, then returns the array. Handles both flat (labels[])
// and nested (repo.labels[]) array parents.
func navigateToArray(obj map[string]any, f CompactField) ([]any, bool) {
	current := obj
	for i := 0; i < f.arrayIdx; i++ {
		next, ok := current[f.path[i]]
		if !ok {
			return nil, false
		}
		nested, ok := next.(map[string]any)
		if !ok {
			return nil, false
		}
		current = nested
	}

	raw, ok := current[f.arrayKey]
	if !ok {
		return nil, false
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	return arr, true
}

// extractArrayField handles "labels[].name" specs — plucks a single field from each array element.
// For nested arrays (labels[].name where childPath has no []), produces flat scalars.
// For childPaths containing [], delegates to group extraction.
func extractArrayField(obj map[string]any, f CompactField) (any, bool) {
	if hasNestedArray(f.childPath) {
		return extractArrayFieldGroup(obj, []CompactField{f})
	}

	arr, ok := navigateToArray(obj, f)
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

// extractArrayFieldGroup handles multiple specs on the same array parent
// (e.g. steps[].name + steps[].conclusion) — produces sub-objects per element.
// Also handles nested array specs (e.g. items[].labels[].name).
func extractArrayFieldGroup(obj map[string]any, fields []CompactField) (any, bool) {
	arr, ok := navigateToArray(obj, fields[0])
	if !ok {
		return nil, false
	}

	result := make([]any, 0, len(arr))
	for _, elem := range arr {
		elemObj, ok := elem.(map[string]any)
		if !ok {
			// Already a sub-object from a previous pass — preserve for idempotence.
			result = append(result, elem)
			continue
		}
		sub := make(map[string]any, len(fields))
		for _, f := range fields {
			key, val, ok := extractChildValue(elemObj, f.childPath)
			if ok {
				sub[key] = val
			}
		}
		if len(sub) > 0 {
			result = append(result, sub)
		}
	}

	return result, true
}

// extractChildValue extracts a value from an object following a child path.
// Handles simple paths ("name"), nested paths ("user.login"), and nested
// array paths ("labels[].name") within the child path.
func extractChildValue(obj map[string]any, childPath []string) (string, any, bool) {
	// Find nested [] in childPath.
	for i, p := range childPath {
		if !strings.HasSuffix(p, "[]") {
			continue
		}

		// Navigate to the nested array parent.
		current := obj
		for j := 0; j < i; j++ {
			next, ok := current[childPath[j]]
			if !ok {
				return "", nil, false
			}
			nested, ok := next.(map[string]any)
			if !ok {
				return "", nil, false
			}
			current = nested
		}

		arrayKey := strings.TrimSuffix(p, "[]")
		raw, ok := current[arrayKey]
		if !ok {
			return "", nil, false
		}
		arr, ok := raw.([]any)
		if !ok {
			return "", nil, false
		}

		// Extract from each element using the remaining path.
		remaining := childPath[i+1:]
		extracted := make([]any, 0, len(arr))
		for _, elem := range arr {
			elemObj, ok := elem.(map[string]any)
			if !ok {
				extracted = append(extracted, elem) // idempotence
				continue
			}
			val, ok := navigateToLeaf(elemObj, remaining)
			if !ok {
				continue
			}
			extracted = append(extracted, val)
		}
		return arrayKey, extracted, true
	}

	// No nested array — simple leaf navigation.
	val, ok := navigateToLeaf(obj, childPath)
	if !ok {
		return "", nil, false
	}
	return childPath[len(childPath)-1], val, ok
}

// hasNestedArray reports whether a child path contains a [] segment.
func hasNestedArray(childPath []string) bool {
	for _, p := range childPath {
		if strings.HasSuffix(p, "[]") {
			return true
		}
	}
	return false
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
