package mcp

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"maps"
	"path"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/ohler55/ojg/gen"
	"github.com/shopspring/decimal"
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

type columnarResult[T any] struct {
	columns   []string
	rows      [][]T
	constants map[string]T
}

type JSONOptions struct {
	Fields   []CompactField
	Columnar bool
}

type jsonValue struct {
	gen.Node
	columnar bool
}

var jsonNodeUnmarshalers = jsonv2.UnmarshalFromFunc(func(dec *jsontext.Decoder, value *gen.Node) error {
	node, err := readJSONNode(dec)
	*value = node
	return err
})

// CompactJSON applies field compaction to JSON bytes.
// Parses once and encodes the projected value. See CompactAny for field semantics.
func CompactJSON(data []byte, fields []CompactField) ([]byte, error) {
	if len(data) == 0 || len(fields) == 0 {
		return data, nil
	}
	parsed, err := ParseJSONValue(data)
	if err != nil {
		return nil, fmt.Errorf("compactJSON: %w", err)
	}
	return EncodeJSONValue(parsed, JSONOptions{Fields: fields})
}

// ColumnarizeJSON converts arrays of objects in JSON to columnar format.
// Parses once and encodes the columnar value. See ColumnarizeAny for semantics.
// Returns data unchanged if parsing or reshaping fails — columnarization is best-effort.
func ColumnarizeJSON(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	parsed, err := ParseJSONValue(data)
	if err != nil {
		return data, nil
	}
	result, err := EncodeJSONValue(parsed, JSONOptions{Columnar: true})
	if err != nil {
		return data, nil
	}
	return result, nil
}

func EncodeJSONValue(value gen.Node, options JSONOptions) ([]byte, error) {
	projected := CompactValue(value, options.Fields)
	return jsonv2.Marshal(jsonValue{Node: projected, columnar: options.Columnar}, json.DefaultOptionsV1())
}

func CompactValue(value gen.Node, fields []CompactField) gen.Node {
	unchanged := value == nil || len(fields) == 0
	if unchanged {
		return value
	}
	plan := buildFieldPlan(fields)
	switch data := value.(type) {
	case gen.Object:
		return compactTypedObject(data, fields, plan)
	case gen.Array:
		return compactTypedArray(data, fields, plan)
	default:
		return value
	}
}

func projectObjects(obj, out gen.Object, plan *fieldPlan) {
	for root := range plan.objectGroups {
		parent, ok := obj[root].(gen.Object)
		if !ok {
			continue
		}
		children := plan.childPlans[root]
		sub := compactTypedObject(parent, children, buildFieldPlan(children))
		if len(sub) > 0 {
			out[root] = sub
		}
	}
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

func writeJSONArrayLayout(enc *jsontext.Encoder, array gen.Array, columnar bool) error {
	if columnar {
		if objects, ok := columnarObjects(array); ok {
			result := columnarObject(buildColumnar(objects, sameJSONValue))
			const nestedColumnar = false
			return writeJSONObject(enc, result, nestedColumnar)
		}
	}
	return writeJSONArray(enc, array)
}

// buildColumnar converts a slice of objects into columnar format:
// {"columns": [sorted keys], "rows": [[val, ...], ...], "constants": {key: val, ...}}.
// Missing keys produce nil (JSON null) in the corresponding cell.
// Columns whose values compare equal across every row are lifted
// into a "constants" map and removed from columns/rows.
func buildColumnar[T any](objects []map[string]T, equal func(T, T) bool) columnarResult[T] {
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
	var zero T
	for _, o := range objects {
		for _, c := range allCols {
			if !equal(o[c], zero) {
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
	fullRows := make([][]T, len(objects))
	for i, o := range objects {
		row := make([]T, len(allCols))
		for j, c := range allCols {
			row[j] = o[c]
		}
		fullRows[i] = row
	}

	// Detect constant columns: every row has the same value.
	constants := map[string]T{}
	constantIdx := make(map[int]bool)
	for j, col := range allCols {
		first := fullRows[0][j]
		uniform := true
		for i := 1; i < len(fullRows); i++ {
			if !equal(first, fullRows[i][j]) {
				uniform = false
				break
			}
		}
		if uniform {
			constants[col] = first
			constantIdx[j] = true
		}
	}

	if len(constants) == 0 {
		return columnarResult[T]{columns: allCols, rows: fullRows}
	}

	// Build final columns/rows without constants.
	cols := make([]string, 0, len(allCols)-len(constants))
	for j, c := range allCols {
		if !constantIdx[j] {
			cols = append(cols, c)
		}
	}

	rows := make([][]T, len(objects))
	for i, fullRow := range fullRows {
		row := make([]T, 0, len(cols))
		for j, val := range fullRow {
			if !constantIdx[j] {
				row = append(row, val)
			}
		}
		rows[i] = row
	}

	return columnarResult[T]{columns: cols, rows: rows, constants: constants}
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
			obj[key] = buildColumnar(objects, reflect.DeepEqual).asMap()
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
		return buildColumnar(objects, reflect.DeepEqual).asMap()
	default:
		return v
	}
}

func (r columnarResult[T]) asMap() map[string]any {
	result := map[string]any{"columns": r.columns, "rows": r.rows}
	if len(r.constants) > 0 {
		result["constants"] = r.constants
	}
	return result
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

func ParseJSONValue(data []byte) (gen.Node, error) {
	var value jsonValue
	err := jsonv2.Unmarshal(data, &value, json.DefaultOptionsV1())
	return value.Node, err
}

func UnmarshalJSON[T any](data []byte, target *T) error {
	return jsonv2.Unmarshal(data, target, json.DefaultOptionsV1(), jsonv2.WithUnmarshalers(jsonNodeUnmarshalers))
}

func (v *jsonValue) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	node, err := readJSONNode(dec)
	v.Node = node
	return err
}

func readJSONNode(dec *jsontext.Decoder) (gen.Node, error) {
	token, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}
	switch token.Kind() {
	case '{':
		return readJSONObject(dec)
	case '[':
		return readJSONArray(dec)
	default:
		return readJSONScalar(token)
	}
}

func readJSONScalar(token jsontext.Token) (gen.Node, error) {
	switch token.Kind() {
	case 'n':
		return nil, nil
	case 't', 'f':
		return gen.Bool(token.Bool()), nil
	case '"':
		return gen.String(token.String()), nil
	case '0':
		if _, err := token.Float(); err != nil {
			return nil, err
		}
		return gen.Big(token.String()), nil
	default:
		return nil, fmt.Errorf("unexpected JSON token %s", token.Kind())
	}
}

func readJSONObject(dec *jsontext.Decoder) (gen.Object, error) {
	object := make(gen.Object)
	for dec.PeekKind() != '}' {
		name, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}
		key := name.String()
		value, err := readJSONNode(dec)
		if err != nil {
			return nil, err
		}
		object[key] = value
	}
	_, err := dec.ReadToken()
	return object, err
}

func readJSONArray(dec *jsontext.Decoder) (gen.Array, error) {
	array := make(gen.Array, 0)
	for dec.PeekKind() != ']' {
		value, err := readJSONNode(dec)
		if err != nil {
			return nil, err
		}
		array = append(array, value)
	}
	_, err := dec.ReadToken()
	return array, err
}

func MarshalJSONValue(value gen.Node) ([]byte, error) {
	return jsonv2.Marshal(jsonValue{Node: value}, json.DefaultOptionsV1())
}

func (v jsonValue) MarshalJSONTo(enc *jsontext.Encoder) error {
	return writeJSONNode(enc, v.Node, v.columnar)
}

func writeJSONNode(enc *jsontext.Encoder, node gen.Node, columnar bool) error {
	switch value := node.(type) {
	case gen.Object:
		return writeJSONObject(enc, value, columnar)
	case gen.Array:
		return writeJSONArrayLayout(enc, value, columnar)
	case gen.Big:
		return enc.WriteValue(jsontext.Value(value))
	case gen.Float:
		return jsonv2.MarshalEncode(enc, float64(value))
	default:
		token, err := jsonScalarToken(node)
		if err != nil {
			return err
		}
		return enc.WriteToken(token)
	}
}

func jsonScalarToken(node gen.Node) (jsontext.Token, error) {
	switch value := node.(type) {
	case nil:
		return jsontext.Null, nil
	case gen.String:
		return jsontext.String(string(value)), nil
	case gen.Bool:
		return jsontext.Bool(bool(value)), nil
	case gen.Int:
		return jsontext.Int(int64(value)), nil
	default:
		return jsontext.Token{}, fmt.Errorf("unsupported JSON node %T", node)
	}
}

func writeJSONObject(enc *jsontext.Encoder, object gen.Object, columnar bool) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}
	keys := slices.Sorted(maps.Keys(object))
	for _, key := range keys {
		if err := enc.WriteToken(jsontext.String(key)); err != nil {
			return err
		}
		if err := writeJSONNode(enc, object[key], columnar); err != nil {
			return err
		}
	}
	return enc.WriteToken(jsontext.EndObject)
}

func writeJSONArray(enc *jsontext.Encoder, array gen.Array) error {
	if err := enc.WriteToken(jsontext.BeginArray); err != nil {
		return err
	}
	const columnar = false
	for _, value := range array {
		if err := writeJSONNode(enc, value, columnar); err != nil {
			return err
		}
	}
	return enc.WriteToken(jsontext.EndArray)
}

func columnarObjects(array gen.Array) ([]map[string]gen.Node, bool) {
	if len(array) < columnarMinItems {
		return nil, false
	}
	objects := make([]map[string]gen.Node, 0, len(array))
	for _, value := range array {
		object, ok := value.(gen.Object)
		if !ok {
			return nil, false
		}
		objects = append(objects, object)
	}
	return objects, true
}

func columnarObject(result columnarResult[gen.Node]) gen.Object {
	columns := make(gen.Array, len(result.columns))
	for i, key := range result.columns {
		columns[i] = gen.String(key)
	}
	rows := make(gen.Array, len(result.rows))
	for i, row := range result.rows {
		rows[i] = gen.Array(row)
	}
	object := gen.Object{"columns": columns, "rows": rows}
	if len(result.constants) > 0 {
		object["constants"] = gen.Object(result.constants)
	}
	return object
}

func sameJSONValue(a, b gen.Node) bool {
	switch value := a.(type) {
	case gen.Object:
		other, ok := b.(gen.Object)
		return ok && maps.EqualFunc(value, other, sameJSONValue)
	case gen.Array:
		other, ok := b.(gen.Array)
		return ok && slices.EqualFunc(value, other, sameJSONValue)
	case gen.Big, gen.Int, gen.Float:
		if !jsonNumberNode(b) {
			return false
		}
		return sameJSONNumber(a.String(), b.String())
	default:
		return reflect.DeepEqual(a, b)
	}
}

func jsonNumberNode(node gen.Node) bool {
	switch node.(type) {
	case gen.Big, gen.Int, gen.Float:
		return true
	default:
		return false
	}
}

func sameJSONNumber(a, b string) bool {
	if a == b {
		return true
	}
	left, leftErr := decimal.NewFromString(a)
	right, rightErr := decimal.NewFromString(b)
	valid := leftErr == nil && rightErr == nil
	if !valid {
		return false
	}
	return equalJSONDecimals(left, right)
}

func equalJSONDecimals(a, b decimal.Decimal) bool {
	zero := a.IsZero() || b.IsZero()
	if zero {
		bothZero := a.IsZero() && b.IsZero()
		return bothZero
	}
	leftMagnitude := int64(a.NumDigits()) + int64(a.Exponent())
	rightMagnitude := int64(b.NumDigits()) + int64(b.Exponent())
	if leftMagnitude != rightMagnitude {
		return false
	}
	return a.Equal(b)
}

func compactTypedArray(array gen.Array, fields []CompactField, plan *fieldPlan) gen.Array {
	result := make(gen.Array, len(array))
	for i, value := range array {
		object, ok := value.(gen.Object)
		if !ok {
			result[i] = value
			continue
		}
		result[i] = compactTypedObject(object, fields, plan)
	}
	return result
}

func compactTypedObject(obj gen.Object, fields []CompactField, plan *fieldPlan) gen.Object {
	if !plan.hasIncludes {
		return excludedObject(obj, plan)
	}
	out := make(gen.Object, len(fields))
	projectScalars(obj, out, plan)
	projectObjects(obj, out, plan)
	projectArrays(obj, out, plan)
	for key := range out {
		if plan.isExcluded(key) {
			delete(out, key)
		}
	}
	return out
}

func excludedObject(obj gen.Object, plan *fieldPlan) gen.Object {
	out := make(gen.Object, len(obj))
	for key, value := range obj {
		keep := !plan.isExcluded(key) && !emptyTypedValue(value)
		if keep {
			out[key] = value
		}
	}
	return out
}

func projectScalars(obj, out gen.Object, plan *fieldPlan) {
	for _, field := range plan.scalars {
		value, ok := extractTypedField(obj, field)
		keep := ok && !emptyTypedValue(value)
		if keep {
			out[field.outputKey] = value
		}
	}
}

func projectArrays(obj, out gen.Object, plan *fieldPlan) {
	for key, group := range plan.arrayGroups {
		value, ok := extractTypedArrayGroup(obj, group)
		if !ok {
			continue
		}
		root := group[0].objectRoot
		if root == "" {
			out[key] = value
			continue
		}
		parent, _ := out[root].(gen.Object)
		if parent == nil {
			parent = make(gen.Object)
		}
		parent[group[0].arrayKey] = value
		out[root] = parent
	}
}

func extractTypedField(obj gen.Object, field CompactField) (gen.Node, bool) {
	direct := field.wildcard || len(field.path) == 1
	if direct {
		value, ok := obj[field.path[0]]
		return value, ok
	}
	if field.arrayIdx >= 0 {
		return extractTypedArrayGroup(obj, []CompactField{field})
	}
	if value, ok := obj[field.outputKey]; ok {
		return value, true
	}
	return navigateTypedLeaf(obj, field.path)
}

func extractTypedArrayGroup(obj gen.Object, fields []CompactField) (gen.Node, bool) {
	field := fields[0]
	path := append([]string(nil), field.path[:field.arrayIdx]...)
	path = append(path, field.arrayKey)
	value, ok := navigateTypedLeaf(obj, path)
	items, array := value.(gen.Array)
	arrayFound := ok && array
	if !arrayFound {
		return nil, false
	}
	pluck := len(fields) == 1 && !hasNestedArray(field.childPath)
	if pluck {
		return pluckTypedArray(items, field.childPath), true
	}
	return projectArrayItems(items, fields), true
}

func projectArrayItems(items gen.Array, fields []CompactField) gen.Node {
	result := make(gen.Array, 0, len(items))
	for _, item := range items {
		object, ok := item.(gen.Object)
		if !ok {
			result = append(result, item)
			continue
		}
		sub := projectArrayObject(object, fields)
		if len(sub) > 0 {
			result = append(result, sub)
		}
	}
	return result
}

func projectArrayObject(object gen.Object, fields []CompactField) gen.Object {
	sub := make(gen.Object, len(fields))
	for _, child := range fields {
		key, value, ok := extractTypedChild(object, child.childPath)
		keep := ok && !emptyTypedValue(value)
		if keep {
			sub[key] = value
		}
	}
	return sub
}

func extractTypedChild(obj gen.Object, path []string) (string, gen.Node, bool) {
	for i, key := range path {
		if !strings.HasSuffix(key, "[]") {
			continue
		}
		key = strings.TrimSuffix(key, "[]")
		parentPath := append([]string(nil), path[:i]...)
		parentPath = append(parentPath, key)
		value, ok := navigateTypedLeaf(obj, parentPath)
		items, array := value.(gen.Array)
		arrayFound := ok && array
		if !arrayFound {
			return "", nil, false
		}
		return key, pluckTypedArray(items, path[i+1:]), true
	}
	value, ok := navigateTypedLeaf(obj, path)
	return path[len(path)-1], value, ok
}

func emptyTypedValue(value gen.Node) bool {
	switch data := value.(type) {
	case nil:
		return true
	case gen.Object:
		return len(data) == 0
	case gen.Array:
		return len(data) == 0
	default:
		return false
	}
}

func pluckTypedArray(items gen.Array, path []string) gen.Node {
	result := make(gen.Array, 0, len(items))
	for _, item := range items {
		object, ok := item.(gen.Object)
		if !ok {
			result = append(result, item)
			continue
		}
		value, ok := navigateTypedLeaf(object, path)
		if ok {
			result = append(result, value)
		}
	}
	return result
}

func navigateTypedLeaf(obj gen.Object, path []string) (gen.Node, bool) {
	for _, key := range path[:len(path)-1] {
		value, ok := obj[key].(gen.Object)
		if !ok {
			return nil, false
		}
		obj = value
	}
	value, ok := obj[path[len(path)-1]]
	return value, ok
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
