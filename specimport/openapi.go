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

// openAPIPathItem holds the operations keyed by HTTP method. We decode the
// well-known verbs explicitly rather than as a map so JSON tags document the
// supported set.
type openAPIPathItem struct {
	Get    *openAPIOperation `json:"get"`
	Post   *openAPIOperation `json:"post"`
	Put    *openAPIOperation `json:"put"`
	Patch  *openAPIOperation `json:"patch"`
	Delete *openAPIOperation `json:"delete"`
	Head   *openAPIOperation `json:"head"`
}

type openAPIOperation struct {
	OperationID string          `json:"operationId"`
	Summary     string          `json:"summary"`
	Description string          `json:"description"`
	Parameters  []openAPIParam  `json:"parameters"`
	RequestBody *openAPIReqBody `json:"requestBody"`
}

type openAPIParam struct {
	Name        string         `json:"name"`
	In          string         `json:"in"` // path, query, header, cookie
	Description string         `json:"description"`
	Required    bool           `json:"required"`
	Schema      *openAPISchema `json:"schema"`
}

type openAPIReqBody struct {
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type openAPISchema struct {
	Type string `json:"type"`
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

	im := &Imported{Name: name, Kind: KindOpenAPI, BaseURL: base}
	seen := map[string]bool{}
	for _, rawPath := range paths {
		item := d.Paths[rawPath]
		for _, me := range item.entries() {
			op := buildOpenAPIOperation(name, rawPath, me, seen)
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
func buildOpenAPIOperation(integration, rawPath string, me methodEntry, seen map[string]bool) operation {
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
	var pathParams, queryParams []string
	for _, pr := range me.op.Parameters {
		desc := pr.Description
		if desc == "" {
			desc = fmt.Sprintf("%s parameter", pr.In)
		}
		params[pr.Name] = desc
		if pr.Required {
			required = append(required, pr.Name)
		}
		switch strings.ToLower(pr.In) {
		case "path":
			pathParams = append(pathParams, pr.Name)
		case "query":
			queryParams = append(queryParams, pr.Name)
		}
	}
	eff := effectForMethod(me.method)
	if eff == effectWrite && me.op.RequestBody != nil {
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
		effect:       eff,
		httpMethod:   me.method,
		pathTemplate: rawPath,
		pathParams:   pathParams,
		queryParams:  queryParams,
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
