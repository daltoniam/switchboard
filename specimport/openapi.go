package specimport

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// openAPIDoc captures the subset of OpenAPI 3.x we need to generate tools.
// We deliberately decode only what we use; unknown fields are ignored so a
// fuller spec still parses.
type openAPIDoc struct {
	OpenAPI    string                     `json:"openapi"`
	Info       openAPIInfo                `json:"info"`
	Servers    []openAPIServer            `json:"servers"`
	Paths      map[string]openAPIPathItem `json:"paths"`
	Components openAPIComponents          `json:"components"`
}

type openAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type openAPIServer struct {
	URL string `json:"url"`
}

// openAPIPathItem holds path-level parameters and operations keyed by HTTP method.
type openAPIPathItem struct {
	Parameters []openAPIParam    `json:"parameters"`
	Servers    []openAPIServer   `json:"servers"`
	Get        *openAPIOperation `json:"get"`
	Post       *openAPIOperation `json:"post"`
	Put        *openAPIOperation `json:"put"`
	Patch      *openAPIOperation `json:"patch"`
	Delete     *openAPIOperation `json:"delete"`
	Head       *openAPIOperation `json:"head"`
}

func (p openAPIPathItem) mergedParameters(op *openAPIOperation) []openAPIParam {
	return openAPIOperationParameters(p, op)
}

type openAPIOperation struct {
	OperationID string          `json:"operationId"`
	Summary     string          `json:"summary"`
	Description string          `json:"description"`
	Servers     []openAPIServer `json:"servers"`
	Parameters  []openAPIParam  `json:"parameters"`
	RequestBody *openAPIReqBody `json:"requestBody"`
}

type openAPIParam struct {
	Ref         string         `json:"$ref"`
	Name        string         `json:"name"`
	In          string         `json:"in"` // path, query, header, cookie
	Description string         `json:"description"`
	Required    bool           `json:"required"`
	Schema      *openAPISchema `json:"schema"`
	Style       string         `json:"style"`
	Explode     *bool          `json:"explode"`
}

// isArray reports whether the parameter's (resolved) schema type is, or
// includes in OpenAPI 3.1, "array".
func (p openAPIParam) isArray() bool {
	if p.Schema == nil || len(p.Schema.Type) == 0 {
		return false
	}
	var many []string
	if json.Unmarshal(p.Schema.Type, &many) == nil {
		for _, t := range many {
			if t == "array" {
				return true
			}
		}
		return false
	}
	return p.Schema.schemaType() == "array"
}

// queryExplode reports how an array query parameter is sent. Form style (the
// query default) explodes unless explode is false.
func (p openAPIParam) queryExplode() bool {
	if p.Explode != nil {
		return *p.Explode
	}
	return p.Style == "" || p.Style == "form"
}

type openAPIReqBody struct {
	Ref         string                      `json:"$ref"`
	Description string                      `json:"description"`
	Required    bool                        `json:"required"`
	Content     map[string]openAPIMediaType `json:"content"`
}

type openAPIMediaType struct {
	Schema *openAPISchema `json:"schema"`
}

type openAPISchema struct {
	Ref  string          `json:"$ref"`
	Type json.RawMessage `json:"type"`
}

type openAPIComponents struct {
	Schemas map[string]openAPISchema `json:"schemas"`
}

// maxSchemaRefDepth bounds how many $ref hops are followed for one schema.
const maxSchemaRefDepth = 10

// resolveSchema follows local "#/components/schemas/..." references. External,
// missing or cyclic references are an error, so the parameter is never
// silently treated as a plain value.
func (c openAPIComponents) resolveSchema(s *openAPISchema) (*openAPISchema, error) {
	const prefix = "#/components/schemas/"
	for hops := 0; s != nil && s.Ref != ""; hops++ {
		if hops >= maxSchemaRefDepth {
			return nil, fmt.Errorf("schema $ref chain too long or cyclic at %q", s.Ref)
		}
		if !strings.HasPrefix(s.Ref, prefix) {
			return nil, fmt.Errorf("unsupported schema $ref %q", s.Ref)
		}
		next, ok := c.Schemas[strings.TrimPrefix(s.Ref, prefix)]
		if !ok {
			return nil, fmt.Errorf("unresolved schema $ref %q", s.Ref)
		}
		s = &next
	}
	return s, nil
}

// resolveParams returns params with their schemas resolved.
func (c openAPIComponents) resolveParams(params []openAPIParam) ([]openAPIParam, error) {
	out := make([]openAPIParam, len(params))
	for i, p := range params {
		resolved, err := c.resolveSchema(p.Schema)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", p.Name, err)
		}
		p.Schema = resolved
		out[i] = p
	}
	return out, nil
}

// schemaType returns the schema's type, the first non-null one for an
// OpenAPI 3.1 type array.
func (s *openAPISchema) schemaType() string {
	if s == nil || len(s.Type) == 0 {
		return ""
	}
	var one string
	if json.Unmarshal(s.Type, &one) == nil {
		return one
	}
	var many []string
	if json.Unmarshal(s.Type, &many) == nil {
		for _, t := range many {
			if t != "null" {
				return t
			}
		}
	}
	return ""
}

// methodEntry pairs an HTTP verb with its parsed operation for iteration.
type methodEntry struct {
	method string
	op     *openAPIOperation
}

func (p openAPIPathItem) entries() []methodEntry {
	out := make([]methodEntry, 0, 6)
	if p.Get != nil {
		out = append(out, methodEntry{"GET", p.Get})
	}
	if p.Head != nil {
		out = append(out, methodEntry{"HEAD", p.Head})
	}
	if p.Post != nil {
		out = append(out, methodEntry{"POST", p.Post})
	}
	if p.Put != nil {
		out = append(out, methodEntry{"PUT", p.Put})
	}
	if p.Patch != nil {
		out = append(out, methodEntry{"PATCH", p.Patch})
	}
	if p.Delete != nil {
		out = append(out, methodEntry{"DELETE", p.Delete})
	}
	return out
}

// effectForMethod classifies an HTTP verb. GET/HEAD are reads; everything
// else may change data and is declared as a write.
func effectForMethod(method string) effect {
	switch strings.ToUpper(method) {
	case "GET", "HEAD":
		return effectRead
	default:
		return effectWrite
	}
}

// parseOpenAPI decodes an OpenAPI 3.x JSON document and builds an Imported.
// endpointOverride, when non-empty, replaces the servers[0].url so callers
// can repoint a spec at a staging/prod host without editing the document.
func parseOpenAPI(name string, doc []byte, endpointOverride string) (*Imported, error) {
	var d openAPIDoc
	if err := json.Unmarshal(doc, &d); err != nil {
		return nil, fmt.Errorf("specimport: parse openapi json: %w", err)
	}
	base := endpointOverride
	if base == "" && len(d.Servers) > 0 {
		base = d.Servers[0].URL
	}
	base = strings.TrimSpace(base)
	if base != "" {
		base = trimURLPathOnly(base)
		if _, err := url.ParseRequestURI(base); err != nil {
			return nil, fmt.Errorf("specimport: invalid server url %q: %w", base, err)
		}
		if err := validateAbsoluteHTTPURL(base); err != nil {
			return nil, err
		}
	}

	// Sort paths for deterministic tool ordering — important so repeated
	// imports of the same spec produce identical tool lists (and so tests
	// are stable).
	paths := make([]string, 0, len(d.Paths))
	for p := range d.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	if err := validateOpenAPIParams(d); err != nil {
		return nil, err
	}

	im := &Imported{Name: name, Kind: KindOpenAPI, BaseURL: base}
	seen := map[string]bool{}
	for _, rawPath := range paths {
		item := d.Paths[rawPath]
		if len(item.Servers) > 0 {
			return nil, fmt.Errorf("specimport: path-level servers are not supported on %s", rawPath)
		}
		for _, me := range item.entries() {
			if len(me.op.Servers) > 0 {
				return nil, fmt.Errorf("specimport: operation-level servers are not supported on %s %s", me.method, rawPath)
			}
			params, err := d.Components.resolveParams(item.mergedParameters(me.op))
			if err != nil {
				continue
			}
			op, err := buildOpenAPIOperation(name, rawPath, me, params, seen)
			if err != nil {
				continue
			}
			im.operations = append(im.operations, op)
		}
	}
	if len(im.operations) == 0 {
		return nil, ErrNoOperations
	}
	return im, nil
}

// buildOpenAPIOperation converts a single (path, method, operation) into the
// protocol-neutral operation form, deriving a stable, prefixed tool name and
// splitting parameters into path vs query buckets.
// effective is the operation's parameter list: path-item parameters merged
// with operation overrides, with schema references resolved.
func buildOpenAPIOperation(integration, rawPath string, me methodEntry, effective []openAPIParam, seen map[string]bool) (operation, error) {
	toolBase := me.op.OperationID
	if toolBase == "" {
		// Synthesize from method + path when operationId is absent, e.g.
		// GET /users/{id} -> get_users_id.
		toolBase = strings.ToLower(me.method) + "_" + sanitizeName(rawPath)
	}
	toolBase = sanitizeName(toolBase)
	fullName := uniqueName(integration+"_"+toolBase, seen)

	params := map[string]string{}
	var required []string
	var pathBindings, queryBindings, headerBindings, cookieBindings []openAPIParamBinding
	usedArgs := map[string]struct{}{}
	for _, pr := range effective {
		if pr.Ref != "" || pr.Name == "" {
			continue
		}
		arg := openAPIToolArg(pr)
		if _, dup := usedArgs[arg]; dup {
			return operation{}, fmt.Errorf("specimport: duplicate tool argument %q for %s %s", arg, me.method, rawPath)
		}
		usedArgs[arg] = struct{}{}
		desc := pr.Description
		if desc == "" {
			desc = fmt.Sprintf("%s parameter", pr.In)
		}
		params[arg] = desc
		if pr.Required {
			required = append(required, arg)
		}
		if style := strings.ToLower(pr.Style); style != "" && style != defaultParamStyle(pr.In) && strings.ToLower(pr.In) != "query" {
			return operation{}, fmt.Errorf("specimport: unsupported %s parameter style %q for %q on %s %s", pr.In, pr.Style, pr.Name, me.method, rawPath)
		}
		switch strings.ToLower(pr.In) {
		case "path":
			pathBindings = append(pathBindings, openAPIParamBinding{arg: arg, wire: pr.Name})
		case "query":
			// Only form style is sent; objects (form or deepObject) are not.
			if pr.Style != "" && pr.Style != "form" {
				return operation{}, fmt.Errorf("specimport: unsupported query parameter style %q for %q on %s %s", pr.Style, pr.Name, me.method, rawPath)
			}
			if pr.Schema.schemaType() == "object" {
				return operation{}, fmt.Errorf("specimport: unsupported object query parameter %q on %s %s", pr.Name, me.method, rawPath)
			}
			queryBindings = append(queryBindings, openAPIParamBinding{
				arg: arg, wire: pr.Name, array: pr.isArray(), explode: pr.queryExplode(),
			})
		case "header":
			headerBindings = append(headerBindings, openAPIParamBinding{arg: arg, wire: pr.Name})
		case "cookie":
			cookieBindings = append(cookieBindings, openAPIParamBinding{arg: arg, wire: pr.Name})
		}
	}
	eff := effectForMethod(me.method)
	if eff == effectWrite && me.op.RequestBody != nil && strings.TrimSpace(me.op.RequestBody.Ref) != "" {
		return operation{}, fmt.Errorf("specimport: unsupported requestBody $ref on %s %s", me.method, rawPath)
	}
	if eff == effectWrite && me.op.RequestBody != nil {
		if len(me.op.RequestBody.Content) > 0 {
			if _, ok := me.op.RequestBody.Content["application/json"]; !ok {
				return operation{}, fmt.Errorf("specimport: unsupported requestBody media type on %s %s (application/json required)", me.method, rawPath)
			}
		}
		params["body"] = "JSON request body (object)"
		if me.op.RequestBody.Required {
			required = append(required, "body")
		}
	}

	desc := firstNonEmpty(me.op.Summary, me.op.Description, fmt.Sprintf("%s %s", me.method, rawPath))
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
		effect:         eff,
		httpMethod:     me.method,
		pathTemplate:   rawPath,
		pathBindings:   pathBindings,
		queryBindings:  queryBindings,
		headerBindings: headerBindings,
		cookieBindings: cookieBindings,
	}, nil
}

// defaultParamStyle is the OpenAPI default serialization style for a
// parameter location: the only style the importer sends for path, header and
// cookie parameters.
func defaultParamStyle(in string) string {
	switch strings.ToLower(in) {
	case "query", "cookie":
		return "form"
	default:
		return "simple"
	}
}

func openAPIToolArg(p openAPIParam) string {
	switch strings.ToLower(p.In) {
	case "path":
		return "path_" + p.Name
	case "query":
		return "query_" + p.Name
	case "header":
		return "header_" + sanitizeName(p.Name)
	case "cookie":
		return "cookie_" + sanitizeName(p.Name)
	default:
		return sanitizeName(p.In) + "_" + sanitizeName(p.Name)
	}
}

// uniqueName guarantees a tool name is not reused within one import by
// appending _2, _3, ... on collision. Collisions happen when a spec lacks
// operationIds and two synthesized names coincide.
func uniqueName(name string, seen map[string]bool) string {
	if !seen[name] {
		seen[name] = true
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", name, i)
		if !seen[candidate] {
			seen[candidate] = true
			return candidate
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func validateOpenAPIParams(doc openAPIDoc) error {
	for path, item := range doc.Paths {
		if err := validateOpenAPIParamList(item.Parameters); err != nil {
			return fmt.Errorf("specimport: path %q: %w", path, err)
		}
		for _, me := range item.entries() {
			if me.op == nil {
				continue
			}
			if err := validateOpenAPIParamList(me.op.Parameters); err != nil {
				return fmt.Errorf("specimport: path %q %s: %w", path, me.method, err)
			}
		}
	}
	return nil
}

func validateOpenAPIParamList(ps []openAPIParam) error {
	for _, p := range ps {
		if p.Ref != "" {
			return fmt.Errorf("unsupported parameter $ref %q", p.Ref)
		}
	}
	return nil
}

// openAPIOperationParameters merges path-item parameters with operation overrides.
func openAPIOperationParameters(item openAPIPathItem, op *openAPIOperation) []openAPIParam {
	if op == nil {
		return mergeOpenAPIParams(item.Parameters, nil)
	}
	return mergeOpenAPIParams(item.Parameters, op.Parameters)
}

func mergeOpenAPIParams(pathLevel, opLevel []openAPIParam) []openAPIParam {
	type paramKey struct {
		name string
		in   string
	}
	merged := map[paramKey]openAPIParam{}
	var order []paramKey
	add := func(ps []openAPIParam) {
		for _, p := range ps {
			if p.Ref != "" || p.Name == "" {
				continue
			}
			k := paramKey{name: p.Name, in: strings.ToLower(p.In)}
			if _, ok := merged[k]; !ok {
				order = append(order, k)
			}
			merged[k] = p
		}
	}
	add(pathLevel)
	add(opLevel)
	out := make([]openAPIParam, 0, len(order))
	for _, k := range order {
		out = append(out, merged[k])
	}
	return out
}
