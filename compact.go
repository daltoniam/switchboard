package mcp

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
)

// CompactField is a parsed field compaction spec — parse once via ParseCompactSpecs,
// then pass to CompactJSON on each request.
type CompactField struct {
	path        []string // e.g. ["user", "login"] or ["labels[]", "name"]
	outputKey   string   // top-level key in the compacted output
	arrayIdx    int      // index of the "[]" segment in path, -1 if none
	arrayKey    string   // path[arrayIdx] without "[]", empty if arrayIdx == -1
	childPath   []string // path[arrayIdx+1:], nil if arrayIdx == -1
	objectRoot  string   // first path segment for non-array multi-segment specs, empty otherwise
	exclude     bool     // true for "-field" exclusion specs
	wildcard    bool     // true for "parent.*" wildcard specs
	globPattern string   // non-empty for glob exclusion specs like "-*_url"
}

// fieldPlan holds pre-computed groupings for a set of CompactField specs.
// Built once by ParseCompactSpecs; reused on every object in an array.
type fieldPlan struct {
	scalars      []CompactField            // simple fields + single-member object groups
	arrayGroups  map[string][]CompactField // outputKey → array field group
	objectGroups map[string][]CompactField // objectRoot → nested object group (2+ members only)
	childPlans   map[string][]CompactField // objectRoot → pre-parsed child CompactFields
	excludes     map[string]bool           // top-level keys to exclude (exact match)
	globExcludes []string                  // glob patterns to exclude (e.g. "*_url")
	hasIncludes  bool                      // true if any non-exclude specs exist
}

// buildFieldPlan pre-computes groupings from a slice of CompactFields.
func buildFieldPlan(fields []CompactField) *fieldPlan {
	plan := &fieldPlan{
		arrayGroups:  make(map[string][]CompactField),
		objectGroups: make(map[string][]CompactField),
		childPlans:   make(map[string][]CompactField),
		excludes:     make(map[string]bool),
	}

	// Separate excludes from includes.
	var includes []CompactField
	for _, f := range fields {
		if !f.exclude {
			includes = append(includes, f)
			continue
		}
		if f.globPattern != "" {
			plan.globExcludes = append(plan.globExcludes, f.globPattern)
			continue
		}
		if len(f.path) == 1 {
			plan.excludes[f.path[0]] = true
		} else {
			plan.excludes[f.objectRoot] = true
		}
	}
	plan.hasIncludes = len(includes) > 0

	// Group include fields.
	rawObjectGroups := map[string][]CompactField{}
	for _, f := range includes {
		if f.wildcard {
			plan.scalars = append(plan.scalars, f)
			continue
		}
		if f.arrayIdx >= 0 {
			plan.arrayGroups[f.outputKey] = append(plan.arrayGroups[f.outputKey], f)
			continue
		}
		if f.objectRoot != "" {
			rawObjectGroups[f.objectRoot] = append(rawObjectGroups[f.objectRoot], f)
			continue
		}
		plan.scalars = append(plan.scalars, f)
	}

	// Collect objectRoots used by nested array groups (arrayIdx > 0).
	// Object specs sharing these roots must always nest (even single-member)
	// so they merge with the nested array under the same parent.
	nestedArrayRoots := map[string]bool{}
	for _, group := range plan.arrayGroups {
		if root := group[0].objectRoot; root != "" {
			nestedArrayRoots[root] = true
		}
	}

	// Split object groups: single-member → scalars (unless sharing a root with
	// a nested array group), multi-member → objectGroups with pre-parsed children.
	for root, group := range rawObjectGroups {
		if len(group) == 1 && !nestedArrayRoots[root] {
			plan.scalars = append(plan.scalars, group[0])
			continue
		}
		plan.objectGroups[root] = group
		childFields := make([]CompactField, 0, len(group))
		for _, f := range group {
			cf, err := parseCompactSpec(strings.Join(f.path[1:], "."))
			if err != nil {
				continue
			}
			childFields = append(childFields, cf)
		}
		plan.childPlans[root] = childFields
	}

	return plan
}

// isExcluded reports whether a key should be excluded by exact match or glob pattern.
func (p *fieldPlan) isExcluded(key string) bool {
	if p.excludes[key] {
		return true
	}
	for _, pattern := range p.globExcludes {
		if matched, _ := path.Match(pattern, key); matched {
			return true
		}
	}
	return false
}

// ParseCompactSpecs parses dot-notation spec strings into CompactFields.
// Call once at init; pass the result to CompactJSON on each request.
//
//   - "field"          → scalar value, output key = field name
//   - "parent.child"   → nested value, output key = dot-path ("user.login")
//   - "parent[].child" → array element extraction, output key = array base ("labels")
//   - "-field"         → exclusion spec, removes top-level field from output
//   - "-parent.child"  → exclusion spec, removes entire parent object from output
//   - "field:alias"    → field renaming, output key = alias
//   - "parent.*"       → wildcard, keeps entire sub-object under parent key
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

	// Handle exclusion specs.
	var exclude bool
	if strings.HasPrefix(spec, "-") {
		exclude = true
		spec = spec[1:]
		if spec == "" {
			return CompactField{}, fmt.Errorf("compact spec: empty exclusion")
		}
	}

	// Handle field renaming (source:alias).
	var alias string
	if idx := strings.LastIndex(spec, ":"); idx >= 0 {
		if exclude {
			return CompactField{}, fmt.Errorf("compact spec: exclusion cannot use rename syntax %q", "-"+spec)
		}
		alias = spec[idx+1:]
		if alias == "" {
			return CompactField{}, fmt.Errorf("compact spec: empty alias in %q", spec)
		}
		spec = spec[:idx]
	}

	if strings.HasPrefix(spec, ".") || strings.HasSuffix(spec, ".") {
		return CompactField{}, fmt.Errorf("compact spec: invalid %q", spec)
	}
	if strings.Contains(spec, "..") {
		return CompactField{}, fmt.Errorf("compact spec: invalid %q", spec)
	}

	parts := strings.Split(spec, ".")

	// Detect glob patterns (contain * but aren't bare "*").
	hasGlob := strings.Contains(spec, "*")

	// Glob exclusions: "-*_url", "-node_*" — only single-segment, exclude-only.
	if hasGlob && exclude && len(parts) == 1 && parts[0] != "*" {
		if _, err := path.Match(parts[0], ""); err != nil {
			return CompactField{}, fmt.Errorf("compact spec: invalid glob pattern %q: %w", spec, err)
		}
		return CompactField{
			path:        parts,
			outputKey:   parts[0],
			arrayIdx:    -1,
			exclude:     true,
			globPattern: parts[0],
		}, nil
	}

	// Non-glob exclusions with bracket metacharacters are also invalid globs.
	if exclude && len(parts) == 1 && strings.ContainsAny(parts[0], "[?") {
		return CompactField{}, fmt.Errorf("compact spec: invalid glob pattern %q", spec)
	}

	// Reject glob in include specs (not a valid wildcard like parent.*).
	if hasGlob && !exclude && (len(parts) != 2 || parts[1] != "*") {
		return CompactField{}, fmt.Errorf("compact spec: glob patterns only allowed in exclusion specs, got %q", spec)
	}

	// Handle wildcard specs (parent.*).
	var isWildcard bool
	if parts[len(parts)-1] == "*" {
		if len(parts) == 1 {
			return CompactField{}, fmt.Errorf("compact spec: bare wildcard not allowed")
		}
		if len(parts) > 2 {
			return CompactField{}, fmt.Errorf("compact spec: wildcard only supports one level (parent.*), got %q", spec)
		}
		isWildcard = true
	}
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "*" {
			return CompactField{}, fmt.Errorf("compact spec: wildcard must be terminal in %q", spec)
		}
	}

	arrayIdx := -1
	for i, p := range parts {
		if !strings.HasSuffix(p, "[]") {
			continue
		}
		if exclude {
			return CompactField{}, fmt.Errorf("compact spec: exclusion cannot use array syntax %q", "-"+spec)
		}
		if i == len(parts)-1 {
			return CompactField{}, fmt.Errorf("compact spec: %q ends with [] (need a child field)", spec)
		}
		if arrayIdx >= 0 {
			continue // nested [] handled at extraction time
		}
		arrayIdx = i
	}

	// "title" → "title", "user.login" → "user.login", "labels[].name" → "labels",
	// "issues.nodes[].id" → "issues.nodes" (preserves parent prefix for nested arrays)
	var outputKey, arrayKey string
	var childPath []string
	switch {
	case arrayIdx >= 0:
		arrayKey = strings.TrimSuffix(parts[arrayIdx], "[]")
		if arrayIdx > 0 {
			outputKey = strings.Join(parts[:arrayIdx], ".") + "." + arrayKey
		} else {
			outputKey = arrayKey
		}
		childPath = parts[arrayIdx+1:]
	case isWildcard:
		outputKey = parts[0]
	case len(parts) == 1:
		outputKey = parts[0]
	default:
		outputKey = spec
	}

	// Apply alias override if provided.
	if alias != "" {
		outputKey = alias
	}

	// objectRoot: first path segment for multi-segment specs (both non-array and
	// nested array specs with arrayIdx > 0). Enables object grouping in
	// compactObject() when 2+ specs share a root. Flat array specs (arrayIdx == 0)
	// have no parent to nest under, so objectRoot stays empty.
	var objectRoot string
	if len(parts) > 1 && (arrayIdx == -1 || arrayIdx > 0) {
		objectRoot = parts[0]
	}

	return CompactField{
		path:       parts,
		outputKey:  outputKey,
		arrayIdx:   arrayIdx,
		arrayKey:   arrayKey,
		childPath:  childPath,
		objectRoot: objectRoot,
		exclude:    exclude,
		wildcard:   isWildcard,
	}, nil
}

// CompactAny applies field compaction to already-parsed JSON data.
// v must be map[string]any (object) or []any (array).
// Returns the compacted value without any JSON serialization.
// Exclusion specs (prefixed with "-") preserve all unlisted fields.
// Null values, empty objects, and empty strings are omitted from output.
func CompactAny(v any, fields []CompactField) any {
	if len(fields) == 0 || v == nil {
		return v
	}
	plan := buildFieldPlan(fields)
	switch val := v.(type) {
	case map[string]any:
		return compactObject(val, fields, plan)
	case []any:
		result := make([]any, 0, len(val))
		for _, elem := range val {
			obj, ok := elem.(map[string]any)
			if !ok {
				result = append(result, elem)
				continue
			}
			result = append(result, compactObject(obj, fields, plan))
		}
		return result
	default:
		return v
	}
}

// CompactJSON applies field compaction to JSON bytes.
// Thin wrapper: unmarshal → CompactAny → marshal. See CompactAny for field semantics.
func CompactJSON(data []byte, fields []CompactField) ([]byte, error) {
	if len(data) == 0 || len(fields) == 0 {
		return data, nil
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("compactJSON: %w", err)
	}
	return json.Marshal(CompactAny(parsed, fields))
}

// compactObject keeps only the specified fields from an unmarshalled object.
// Uses pre-computed fieldPlan for groupings, avoiding per-object re-computation.
// When only exclusion specs exist, copies all keys except excluded ones.
// Omits null values, empty arrays, and empty objects from output.
func compactObject(obj map[string]any, fields []CompactField, plan *fieldPlan) map[string]any {
	// Exclusion-only mode: copy all fields except excluded ones.
	if !plan.hasIncludes && (len(plan.excludes) > 0 || len(plan.globExcludes) > 0) {
		out := make(map[string]any, len(obj))
		for k, v := range obj {
			if plan.isExcluded(k) {
				continue
			}
			if isEmptyValue(v) {
				continue
			}
			out[k] = v
		}
		return out
	}

	out := make(map[string]any, len(plan.scalars)+len(plan.arrayGroups)+len(plan.objectGroups))

	// Scalar fields (simple + single-member object groups).
	for _, f := range plan.scalars {
		val, ok := extractField(obj, f)
		if !ok || isEmptyValue(val) {
			continue
		}
		out[f.outputKey] = val
	}

	// Object groups: 2+ specs sharing a root → nested sub-object.
	for root, group := range plan.objectGroups {
		childFields := plan.childPlans[root]
		if sub := compactSubObject(obj, root, group, childFields); len(sub) > 0 {
			out[root] = sub
		}
	}

	// Array groups.
	for key, group := range plan.arrayGroups {
		var val any
		var ok bool
		if len(group) == 1 {
			val, ok = extractArrayField(obj, group[0])
		} else {
			val, ok = extractArrayFieldGroup(obj, group)
		}
		if !ok {
			continue
		}

		// Nested array groups (arrayIdx > 0, e.g. "issues.nodes") nest under
		// their parent object alongside sibling object specs. Flat array groups
		// (arrayIdx == 0, e.g. "labels") go directly into out.
		root := group[0].objectRoot
		if root != "" {
			parent, _ := out[root].(map[string]any)
			if parent == nil {
				parent = make(map[string]any, 2)
			}
			arrayKey := group[0].arrayKey
			if arr, isArr := val.([]any); isArr {
				parent[arrayKey] = arr
			} else if !isEmptyValue(val) {
				parent[arrayKey] = val
			}
			if len(parent) > 0 {
				out[root] = parent
			}
			continue
		}

		// Preserve empty arrays for spec-targeted array groups: a spec
		// targeting "matches[].text" means the author chose to include this
		// field — [] means "0 results", not noise.
		if arr, isArr := val.([]any); isArr {
			out[key] = arr
		} else if !isEmptyValue(val) {
			out[key] = val
		}
	}

	// Apply excludes to mixed mode (includes + excludes).
	for k := range out {
		if plan.isExcluded(k) {
			delete(out, k)
		}
	}

	return out
}

// isEmptyValue reports whether a value should be omitted from compacted output.
// Null, empty arrays, and empty objects are considered empty.
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch tv := v.(type) {
	case []any:
		return len(tv) == 0
	case map[string]any:
		return len(tv) == 0
	}
	return false
}

// compactSubObject builds a compacted nested object from specs sharing a root.
// Uses pre-parsed child fields instead of re-parsing on every call.
func compactSubObject(obj map[string]any, root string, _ []CompactField, childFields []CompactField) map[string]any {
	parentObj, ok := obj[root].(map[string]any)
	if !ok {
		return nil
	}
	childPlan := buildFieldPlan(childFields)
	return compactObject(parentObj, childFields, childPlan)
}

// columnarMinItems is the minimum array length for columnar format.
// Below 8 items, per-record JSON is more readable and the byte savings (<25%) are marginal.
// At 8+ items, columnar saves 28%+ even on heterogeneous data, and payloads are large enough
// that eliminating key repetition genuinely helps LLM scanning.
const columnarMinItems = 8

// buildColumnar converts a slice of objects into columnar format:
// {"columns": [sorted keys], "rows": [[val, ...], ...], "constants": {key: val, ...}}.
// Missing keys produce nil (JSON null) in the corresponding cell.
// Columns where every row has the same value (by reflect.DeepEqual) are lifted
// into a "constants" map and removed from columns/rows.
func buildColumnar(objects []map[string]any) map[string]any {
	seen := make(map[string]bool)
	for _, o := range objects {
		for k := range o {
			seen[k] = true
		}
	}
	allCols := make([]string, 0, len(seen))
	for k := range seen {
		allCols = append(allCols, k)
	}

	// Count non-null values per column for density ordering.
	density := make(map[string]int, len(allCols))
	for _, o := range objects {
		for _, c := range allCols {
			if o[c] != nil {
				density[c]++
			}
		}
	}

	// Sort by density descending, then alphabetically within same density.
	sort.Slice(allCols, func(i, j int) bool {
		di, dj := density[allCols[i]], density[allCols[j]]
		if di != dj {
			return di > dj
		}
		return allCols[i] < allCols[j]
	})

	// Build full rows first, then scan for constant columns.
	fullRows := make([][]any, len(objects))
	for i, o := range objects {
		row := make([]any, len(allCols))
		for j, c := range allCols {
			row[j] = o[c]
		}
		fullRows[i] = row
	}

	// Detect constant columns: every row has the same value.
	constants := map[string]any{}
	constantIdx := make(map[int]bool)
	for j, col := range allCols {
		first := fullRows[0][j]
		uniform := true
		for i := 1; i < len(fullRows); i++ {
			if !reflect.DeepEqual(first, fullRows[i][j]) {
				uniform = false
				break
			}
		}
		if uniform {
			constants[col] = first
			constantIdx[j] = true
		}
	}

	// Build final columns/rows without constants.
	cols := make([]string, 0, len(allCols)-len(constants))
	for j, c := range allCols {
		if !constantIdx[j] {
			cols = append(cols, c)
		}
	}

	rows := make([][]any, len(objects))
	for i, fullRow := range fullRows {
		row := make([]any, 0, len(cols))
		for j, val := range fullRow {
			if !constantIdx[j] {
				row = append(row, val)
			}
		}
		rows[i] = row
	}

	result := map[string]any{"columns": cols, "rows": rows}
	if len(constants) > 0 {
		result["constants"] = constants
	}
	return result
}

// columnarizeNestedArrays walks a compacted object and converts any nested
// []any where all elements are map[string]any into {"columns":[...],"rows":[[...],...]}.
// Flat scalar arrays (like ["bug","P1"] from labels[].name) are left untouched.
// Recurses into nested map[string]any values to handle deep envelopes (e.g., issues.nodes[]).
func columnarizeNestedArrays(obj map[string]any) {
	for key, val := range obj {
		switch v := val.(type) {
		case map[string]any:
			columnarizeNestedArrays(v)
		case []any:
			if len(v) < columnarMinItems {
				continue
			}
			objects := make([]map[string]any, 0, len(v))
			for _, elem := range v {
				m, ok := elem.(map[string]any)
				if !ok {
					objects = nil
					break
				}
				objects = append(objects, m)
			}
			if objects == nil {
				continue
			}
			obj[key] = buildColumnar(objects)
		}
	}
}

// ColumnarizeAny converts arrays of 8+ objects to columnar format.
// Operates on already-parsed data — no JSON serialization.
// Note: map[string]any inputs are modified in-place (nested arrays columnarized);
// []any inputs return a new columnar map without mutating the original slice.
func ColumnarizeAny(v any) any {
	switch val := v.(type) {
	case map[string]any:
		columnarizeNestedArrays(val)
		return val
	case []any:
		if len(val) < columnarMinItems {
			return val
		}
		objects := make([]map[string]any, 0, len(val))
		for _, elem := range val {
			m, ok := elem.(map[string]any)
			if !ok {
				return val
			}
			objects = append(objects, m)
		}
		return buildColumnar(objects)
	default:
		return v
	}
}

// ColumnarizeJSON converts arrays of objects in JSON to columnar format.
// Thin wrapper: unmarshal → ColumnarizeAny → marshal. See ColumnarizeAny for semantics.
// Returns data unchanged if parsing or reshaping fails — columnarization is best-effort.
func ColumnarizeJSON(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return data, nil
	}
	result, err := json.Marshal(ColumnarizeAny(parsed))
	if err != nil {
		return data, nil
	}
	return result, nil
}

// extractField dispatches to the right extraction strategy based on the spec shape.
func extractField(obj map[string]any, f CompactField) (any, bool) {
	// Wildcard: "user.*" → return obj["user"] as-is.
	if f.wildcard {
		val, ok := obj[f.path[0]]
		return val, ok
	}

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
			if ok && !isEmptyValue(val) {
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
