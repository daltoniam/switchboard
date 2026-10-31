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
	OpenAPI string                     `json:"openapi"`
	Info    openAPIInfo                `json:"info"`
	Servers []openAPIServer            `json:"servers"`
	Paths   map[string]openAPIPathItem `json:"paths"`
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
	Type json.RawMessage `json:"type"`
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
// else mutates and must be gated by the policy layer.
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
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base != "" {
		if _, err := url.ParseRequestURI(base); err != nil {
			return nil, fmt.Errorf("specimport: invalid server url %q: %w", base, err)
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
			op, err := buildOpenAPIOperation(name, rawPath, me, item.mergedParameters(me.op), seen)
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
func buildOpenAPIOperation(integration, rawPath string, me methodEntry, pathParams []openAPIParam, seen map[string]bool) (operation, error) {
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
	for _, pr := range mergeOpenAPIParams(pathParams, me.op.Parameters) {
		if pr.Ref != "" || pr.Name == "" {
			continue
		}
		arg := openAPIToolArg(pr)
		desc := pr.Description
		if desc == "" {
			desc = fmt.Sprintf("%s parameter", pr.In)
		}
		params[arg] = desc
		if pr.Required {
			required = append(required, arg)
		}
		switch strings.ToLower(pr.In) {
		case "path":
			pathBindings = append(pathBindings, openAPIParamBinding{arg: arg, wire: pr.Name})
		case "query":
			queryBindings = append(queryBindings, openAPIParamBinding{arg: arg, wire: pr.Name})
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
