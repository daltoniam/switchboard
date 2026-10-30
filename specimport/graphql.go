package specimport

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// GraphQL introspection produces a deeply nested schema. We decode the query
// and mutation root field lists plus every object/interface type so we can
// expose each top-level field as a callable tool AND synthesize a default
// selection set for it. Object-, interface-, and union-typed fields are
// invalid in a real query without a selection set, so for those we emit the
// type's scalar/enum leaf fields (recursing a bounded depth for nested
// objects). Fields whose return type is already a scalar or enum get no
// selection set. This keeps the import deterministic while producing queries
// that actually validate against a spec-compliant server.

type gqlIntrospection struct {
	Data struct {
		Schema gqlSchema `json:"__schema"`
	} `json:"data"`
	// Some endpoints return the schema at the top level (no "data" wrapper).
	Schema *gqlSchema `json:"__schema"`
}

type gqlSchema struct {
	QueryType    *gqlNamedType `json:"queryType"`
	MutationType *gqlNamedType `json:"mutationType"`
	Types        []gqlType     `json:"types"`
}

type gqlNamedType struct {
	Name string `json:"name"`
}

type gqlType struct {
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Fields        []gqlField     `json:"fields"`
	PossibleTypes []gqlNamedType `json:"possibleTypes"`
}

type gqlField struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Args        []gqlArg   `json:"args"`
	Type        gqlTypeRef `json:"type"`
}

type gqlArg struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	DefaultValue *string    `json:"defaultValue"`
	Type         gqlTypeRef `json:"type"`
}

func (a gqlArg) hasDefault() bool {
	if a.DefaultValue == nil {
		return false
	}
	s := strings.TrimSpace(*a.DefaultValue)
	return s != "" && s != "null"
}

// gqlTypeRef is the recursive type wrapper. We only need to know whether the
// outermost wrapper is NON_NULL (required) and the underlying named type for
// the variable declaration.
type gqlTypeRef struct {
	Kind   string      `json:"kind"`
	Name   string      `json:"name"`
	OfType *gqlTypeRef `json:"ofType"`
}

// required reports whether the outermost wrapper makes this arg non-null.
func (t gqlTypeRef) required() bool { return t.Kind == "NON_NULL" }

// typeName renders the GraphQL type for a variable declaration, e.g.
// "[String!]!" — walking the ofType chain. Falls back to "String" when the
// chain is malformed so we always emit a syntactically valid declaration.
func (t gqlTypeRef) typeName() string {
	switch t.Kind {
	case "NON_NULL":
		if t.OfType != nil {
			return t.OfType.typeName() + "!"
		}
	case "LIST":
		if t.OfType != nil {
			return "[" + t.OfType.typeName() + "]"
		}
	default:
		if t.Name != "" {
			return t.Name
		}
	}
	return "String"
}

// named unwraps NON_NULL/LIST wrappers and returns the underlying named
// type. The returned name is "" if the chain bottoms out without a name.
func (t gqlTypeRef) named() (kind, name string) {
	cur := &t
	for cur != nil {
		if cur.Name != "" {
			return cur.Kind, cur.Name
		}
		cur = cur.OfType
	}
	return "", ""
}

// isList reports whether the type is wrapped in one or more LIST layers.
func (t gqlTypeRef) isList() bool {
	for cur := &t; cur != nil; cur = cur.OfType {
		if cur.Kind == "LIST" {
			return true
		}
	}
	return false
}

// isListOfComposite reports whether the outermost LIST element is an object,
// interface, or union (as opposed to a scalar/enum list).
func (t gqlTypeRef) isListOfComposite() bool {
	for cur := &t; cur != nil; cur = cur.OfType {
		if cur.Kind == "LIST" && cur.OfType != nil {
			kind, _ := cur.OfType.named()
			return isComposite(kind)
		}
	}
	return false
}

// listElementNamed returns the kind and name of the element type when t is a list.
func (t gqlTypeRef) listElementNamed() (kind, name string) {
	for cur := &t; cur != nil; cur = cur.OfType {
		if cur.Kind == "LIST" && cur.OfType != nil {
			return cur.OfType.named()
		}
	}
	return "", ""
}

// parseGraphQL decodes a GraphQL introspection result and builds an Imported
// where each query/mutation root field becomes a tool. endpoint is the URL
// requests will be POSTed to and is required (introspection does not carry
// the endpoint URL).
func parseGraphQL(name string, doc []byte, endpoint string) (*Imported, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("specimport: graphql import requires an endpoint url")
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, fmt.Errorf("specimport: invalid graphql endpoint %q: %w", endpoint, err)
	}

	var intro gqlIntrospection
	if err := json.Unmarshal(doc, &intro); err != nil {
		return nil, fmt.Errorf("specimport: parse graphql introspection json: %w", err)
	}
	schema := intro.Data.Schema
	if schema.QueryType == nil && schema.MutationType == nil && intro.Schema != nil {
		schema = *intro.Schema
	}

	typesByName := map[string]gqlType{}
	for _, t := range schema.Types {
		typesByName[t.Name] = t
	}

	im := &Imported{Name: name, Kind: KindGraphQL, BaseURL: endpoint}
	seen := map[string]bool{}
	sel := &selectionBuilder{types: typesByName}

	addRoot := func(root *gqlNamedType, eff effect, gqlOp string) {
		if root == nil {
			return
		}
		t, ok := typesByName[root.Name]
		if !ok {
			return
		}
		fields := append([]gqlField(nil), t.Fields...)
		sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
		for _, f := range fields {
			im.operations = append(im.operations, buildGraphQLOperation(name, f, eff, gqlOp, seen, sel))
		}
	}
	addRoot(schema.QueryType, effectRead, "query")
	addRoot(schema.MutationType, effectWrite, "mutation")

	if len(im.operations) == 0 {
		return nil, ErrNoOperations
	}
	return im, nil
}

// buildGraphQLOperation turns a single root field into an operation. It
// synthesizes a GraphQL document that declares the field's args as variables
// and passes them through, and — for fields returning an object, interface,
// or union — appends a default selection set so the query validates. The
// executor only has to fill in variables from the tool args.
func buildGraphQLOperation(integration string, f gqlField, eff effect, gqlOp string, seen map[string]bool, sel *selectionBuilder) operation {
	fullName := uniqueName(integration+"_"+sanitizeName(f.Name), seen)

	params := map[string]string{}
	var required, varNames []string
	var varDecls, callArgs []string
	for _, a := range f.Args {
		desc := a.Description
		if desc == "" {
			desc = a.Type.typeName() + " argument"
		}
		params[a.Name] = desc
		if a.Type.required() && !a.hasDefault() {
			required = append(required, a.Name)
		}
		varNames = append(varNames, a.Name)
		decl := fmt.Sprintf("$%s: %s", a.Name, a.Type.typeName())
		if a.hasDefault() {
			decl += " = " + strings.TrimSpace(*a.DefaultValue)
		}
		varDecls = append(varDecls, decl)
		callArgs = append(callArgs, fmt.Sprintf("%s: $%s", a.Name, a.Name))
	}

	var sb strings.Builder
	sb.WriteString(gqlOp)
	if len(varDecls) > 0 {
		sb.WriteString("(" + strings.Join(varDecls, ", ") + ")")
	}
	sb.WriteString(" { " + f.Name)
	if len(callArgs) > 0 {
		sb.WriteString("(" + strings.Join(callArgs, ", ") + ")")
	}
	if selSet := sel.build(f.Type); selSet != "" {
		sb.WriteString(" " + selSet)
	}
	sb.WriteString(" }")

	desc := f.Description
	if desc == "" {
		desc = fmt.Sprintf("GraphQL %s %s", gqlOp, f.Name)
	}
	if eff == effectWrite {
		desc = "[write] " + desc
	}

	return operation{
		tool: mcp.ToolDefinition{
			Name:        mcp.ToolName(fullName),
			Description: desc,
			Parameters:  params,
			Required:    required,
		},
		effect:       eff,
		gqlDocument:  sb.String(),
		gqlVariables: varNames,
	}
}

// maxSelectionDepth bounds how deeply build recurses into nested object
// types when synthesizing a default selection set for non-list return types.
// Two levels selects scalars on the return type plus one nested object level
// (scalars only beneath that). List-returning root fields use depth 1 (scalars
// only on each element) so large list responses stay under response size limits.
const maxSelectionDepth = 2

// maxSelectionFields caps how many leaf fields we select at a single level so
// a type with hundreds of scalar fields does not explode the query size.
const maxSelectionFields = 50

// selectionBuilder synthesizes a default selection set for object/interface/
// union return types from the introspected schema. Scalar and enum types
// need no selection set; object-like types require one to form a valid query.
type selectionBuilder struct {
	types map[string]gqlType
}

// build returns a selection set (including the surrounding braces) for the
// given return type, or "" when the type is a scalar/enum that needs none.
func (s *selectionBuilder) build(ret gqlTypeRef) string {
	kind, name := ret.named()
	if !isComposite(kind) {
		return ""
	}
	depth := maxSelectionDepth
	scalarsOnly := false
	if ret.isList() {
		depth = 1
		scalarsOnly = true
	}
	body := s.fields(name, depth, map[string]bool{}, scalarsOnly)
	if body == "" {
		// A composite type we cannot expand (unknown, empty, or purely
		// cyclic) still needs a non-empty selection set to be valid.
		body = "__typename"
	}
	return "{ " + body + " }"
}

// fields renders the space-separated selection body for the named composite
// type. It selects scalar/enum leaves directly and recurses into nested
// composite fields until depth is exhausted, at which point nested composites
// are reduced to their __typename so the selection set stays valid. visited
// guards against cyclic type references along the current path.
func (s *selectionBuilder) fields(typeName string, depth int, visited map[string]bool, scalarsOnly bool) string {
	t, ok := s.types[typeName]
	if !ok {
		return ""
	}
	if t.Kind == "UNION" {
		return s.unionSelection(t, depth, visited, scalarsOnly)
	}
	if len(t.Fields) == 0 {
		return ""
	}
	if visited[typeName] {
		return ""
	}
	visited[typeName] = true
	defer delete(visited, typeName)

	fields := append([]gqlField(nil), t.Fields...)
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })

	var parts []string
	for _, f := range fields {
		if len(parts) >= maxSelectionFields {
			break
		}
		// Skip fields that require arguments — we have no values to supply.
		if hasRequiredArg(f) {
			continue
		}
		if sel := s.selectionForField(f, depth, visited, scalarsOnly); sel != "" {
			parts = append(parts, sel)
		}
	}
	return strings.Join(parts, " ")
}

func (s *selectionBuilder) selectionForField(f gqlField, depth int, visited map[string]bool, scalarsOnly bool) string {
	if f.Type.isListOfComposite() {
		return s.selectionForListComposite(f, depth, visited, scalarsOnly)
	}
	kind, name := f.Type.named()
	if !isComposite(kind) {
		return f.Name
	}
	if scalarsOnly {
		return ""
	}
	if depth <= 1 {
		return f.Name + " { __typename }"
	}
	nested := s.fields(name, depth-1, visited, false)
	if nested == "" {
		nested = "__typename"
	}
	return f.Name + " { " + nested + " }"
}

func (s *selectionBuilder) selectionForListComposite(f gqlField, depth int, visited map[string]bool, scalarsOnly bool) string {
	elemKind, elemName := f.Type.listElementNamed()
	if scalarsOnly {
		if !isComposite(elemKind) {
			return ""
		}
		if leaf := s.firstScalarField(elemName); leaf != "" {
			return f.Name + " { " + leaf + " }"
		}
		return f.Name + " { __typename }"
	}
	if depth <= 1 {
		return f.Name + " { __typename }"
	}
	if !isComposite(elemKind) {
		return ""
	}
	nested := s.fields(elemName, depth-1, visited, false)
	if nested == "" {
		nested = "__typename"
	}
	return f.Name + " { " + nested + " }"
}

func (s *selectionBuilder) unionSelection(t gqlType, depth int, visited map[string]bool, scalarsOnly bool) string {
	if len(t.PossibleTypes) == 0 {
		return ""
	}
	var parts []string
	for _, pt := range t.PossibleTypes {
		if len(parts) >= maxSelectionFields {
			break
		}
		member := s.fields(pt.Name, depth, visited, scalarsOnly)
		if member == "" {
			member = "__typename"
		}
		parts = append(parts, fmt.Sprintf("... on %s { %s }", pt.Name, member))
	}
	return strings.Join(parts, " ")
}

// firstScalarField returns the name of the first selectable scalar/enum field on
// a composite type, used for bounded list-of-object selections.
func (s *selectionBuilder) firstScalarField(typeName string) string {
	t, ok := s.types[typeName]
	if !ok {
		return ""
	}
	fields := append([]gqlField(nil), t.Fields...)
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	for _, f := range fields {
		if hasRequiredArg(f) {
			continue
		}
		kind, _ := f.Type.named()
		if !isComposite(kind) {
			return f.Name
		}
	}
	return ""
}

// hasRequiredArg reports whether any of a field's arguments is non-null,
// meaning we cannot select it without supplying a value.
func hasRequiredArg(f gqlField) bool {
	for _, a := range f.Args {
		if a.Type.required() && !a.hasDefault() {
			return true
		}
	}
	return false
}

// isComposite reports whether a type kind needs a selection set (object,
// interface, or union) as opposed to a scalar/enum leaf.
func isComposite(kind string) bool {
	switch kind {
	case "OBJECT", "INTERFACE", "UNION":
		return true
	default:
		return false
	}
}
