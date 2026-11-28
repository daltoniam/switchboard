package panther

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
)

type handler func(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error)

// spec is one native tool: its definition and the handler that runs it.
type spec struct {
	name   mcp.ToolName
	desc   string
	access mcp.ToolAccess
	params []param
	run    handler
}

type paramKind int

const (
	kindString paramKind = iota
	kindStrings
	kindInt
	kindBool
)

// param is one tool argument. query names the REST query parameter it maps
// to for generic list tools; handlers read other arguments themselves.
type param struct {
	name     string
	desc     string
	kind     paramKind
	query    string
	required bool
}

func (p param) req() param { p.required = true; return p }

func str(name, query, desc string) param { return param{name: name, query: query, desc: desc} }
func strs(name, query, desc string) param {
	return param{name: name, query: query, desc: desc, kind: kindStrings}
}
func integer(name, query, desc string) param {
	return param{name: name, query: query, desc: desc, kind: kindInt}
}
func boolean(name, query, desc string) param {
	return param{name: name, query: query, desc: desc, kind: kindBool}
}

func pageParams() []param {
	return []param{
		str("cursor", "cursor", "Opaque next cursor from the previous page; omit for the first page."),
		integer("limit", "limit", fmt.Sprintf("Page size, 1 to %d (default %d).", maxPageSize, defaultPageSize)),
	}
}

func (s spec) definition() mcp.ToolDefinition {
	parameters := make(map[string]string, len(s.params))
	var required []string
	for _, p := range s.params {
		parameters[p.name] = p.desc
		if p.required {
			required = append(required, p.name)
		}
	}
	return mcp.ToolDefinition{Name: s.name, Description: s.desc, Parameters: parameters, Required: required, Access: s.access}
}

// queryValues maps the declared query arguments onto REST query parameters.
// Lists repeat the key, which is how Panther's REST API takes arrays.
func queryValues(params []param, args map[string]any) (url.Values, error) {
	r := mcp.NewArgs(args)
	q := url.Values{}
	for _, p := range params {
		if p.query == "" {
			continue
		}
		switch p.kind {
		case kindString:
			if v := strings.TrimSpace(r.Str(p.name)); v != "" {
				q.Set(p.query, v)
			}
		case kindStrings:
			for _, v := range r.StrSlice(p.name) {
				if v = strings.TrimSpace(v); v != "" {
					q.Add(p.query, v)
				}
			}
		case kindInt:
			if _, ok := args[p.name]; ok {
				q.Set(p.query, strconv.Itoa(r.Int(p.name)))
			}
		case kindBool:
			if _, ok := args[p.name]; ok {
				q.Set(p.query, strconv.FormatBool(r.Bool(p.name)))
			}
		}
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return q, nil
}

// pageLimit applies the default page size and refuses out-of-range values.
func pageLimit(args map[string]any) (int, error) {
	limit, err := mcp.ArgInt(args, "limit")
	if err != nil {
		return 0, err
	}
	if _, set := args["limit"]; !set || args["limit"] == nil {
		return defaultPageSize, nil
	}
	if limit < 1 || limit > maxPageSize {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxPageSize)
	}
	return limit, nil
}

// restList is a paginated REST GET that returns Panther's {results, next}
// body unchanged, so compaction and cursors work on the API's own shape.
func restList(name mcp.ToolName, desc, path string, filters ...param) spec {
	params := slices.Concat(filters, pageParams())
	return spec{name: name, desc: desc, access: mcp.AccessRead, params: params, run: func(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
		return listREST(ctx, c, path, params, args, nil)
	}}
}

func listREST(ctx context.Context, c *client, path string, params []param, args map[string]any, defaults func(url.Values)) (*mcp.ToolResult, error) {
	q, err := queryValues(params, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	limit, err := pageLimit(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	q.Set("limit", strconv.Itoa(limit))
	if defaults != nil {
		defaults(q)
	}
	return getList(ctx, c, path, q)
}

// getList reads a REST list and always returns {results, next}, with next
// null on the last page, so callers and compaction see one shape whether or
// not Panther sent the key. Records stay raw.
func getList(ctx context.Context, c *client, path string, q url.Values) (*mcp.ToolResult, error) {
	data, err := c.rest(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var page struct {
		Results json.RawMessage `json:"results"`
		Next    *string         `json:"next"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return mcp.ErrResult(fmt.Errorf("panther GET %s: unexpected response: %w", path, err))
	}
	if len(page.Results) == 0 || string(page.Results) == "null" {
		page.Results = json.RawMessage("[]")
	}
	if page.Next != nil && *page.Next == "" {
		page.Next = nil
	}
	return mcp.JSONResult(map[string]any{"results": page.Results, "next": page.Next})
}

// restGet fetches one record by id from path/<id>.
func restGet(name mcp.ToolName, desc, path, idArg, idDesc string) spec {
	return spec{name: name, desc: desc, access: mcp.AccessRead, params: []param{str(idArg, "", idDesc).req()}, run: func(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
		id, err := mcp.ArgStr(args, idArg)
		if err != nil {
			return mcp.ErrResult(err)
		}
		escaped, err := pathID(idArg, id)
		if err != nil {
			return mcp.ErrResult(err)
		}
		data, err := c.rest(ctx, http.MethodGet, path+"/"+escaped, nil, nil)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}}
}

var specs = slices.Concat(
	alertSpecs,
	detectionSpecs,
	dataLakeSpecs,
	platformSpecs,
)

var dispatch = func() map[mcp.ToolName]handler {
	m := make(map[mcp.ToolName]handler, len(specs))
	for _, s := range specs {
		m[s.name] = s.run
	}
	return m
}()

var nativeTools = func() []mcp.ToolDefinition {
	out := make([]mcp.ToolDefinition, len(specs))
	for i, s := range specs {
		out[i] = s.definition()
	}
	return out
}()

func cloneTools(tools []mcp.ToolDefinition) []mcp.ToolDefinition {
	out := slices.Clone(tools)
	for i := range out {
		out[i].Parameters = maps.Clone(out[i].Parameters)
		out[i].Required = slices.Clone(out[i].Required)
	}
	return out
}

func (c *client) execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	run, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	return run(ctx, c, args)
}
