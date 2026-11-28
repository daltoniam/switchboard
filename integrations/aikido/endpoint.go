package aikido

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

type location int

const (
	inAuto location = iota
	inPath
	inQuery
	inBody
)

type paramKind int

const (
	kindString paramKind = iota
	kindInt
	kindBool
	kindStrings
	kindInts
	kindObject
	// kindAny passes a JSON value through unchanged, for arrays of objects
	// such as autofix packages.
	kindAny
)

// param is one tool argument. Its name is also the Aikido field name.
type param struct {
	name     string
	in       location
	kind     paramKind
	desc     string
	required bool
	enum     []string
	// intBool sends a boolean as 1 or 0, for query flags Aikido declares as
	// integers.
	intBool bool
}

func (p param) req() param   { p.required = true; return p }
func (p param) query() param { p.in = inQuery; return p }
func (p param) oneOf(values ...string) param {
	p.enum = values
	p.desc += " One of: " + strings.Join(values, ", ") + "."
	return p
}
func (p param) asInt() param { p.intBool = true; return p }

func str(name, desc string) param     { return param{name: name, kind: kindString, desc: desc} }
func integer(name, desc string) param { return param{name: name, kind: kindInt, desc: desc} }
func boolean(name, desc string) param { return param{name: name, kind: kindBool, desc: desc} }
func strs(name, desc string) param    { return param{name: name, kind: kindStrings, desc: desc} }
func ints(name, desc string) param    { return param{name: name, kind: kindInts, desc: desc} }
func object(name, desc string) param  { return param{name: name, kind: kindObject, desc: desc} }
func value(name, desc string) param   { return param{name: name, kind: kindAny, desc: desc} }

func ps(items ...param) []param { return items }

// paging describes Aikido's zero-based page/per_page pagination.
type paging struct {
	min, max, def int
}

// endpoint is one tool: an Aikido public API operation and its arguments.
type endpoint struct {
	tool   mcp.ToolName
	method string
	path   string
	scope  string
	access mcp.ToolAccess
	desc   string
	params []param
	// page adds page/per_page arguments and wraps the response as
	// {"results", "page", "per_page", "has_next_page"}.
	page *paging
	// unwrap names the object key holding a paginated endpoint's records, so
	// results is always the record array.
	unwrap string
	// fixed query values always sent, such as a JSON export format.
	fixed url.Values
	// redact removes secrets from a parsed response before it is returned.
	redact func(any)
}

func pages(min, max, def int) *paging { return &paging{min: min, max: max, def: def} }

var pathParam = regexp.MustCompile(`\{([a-z_]+)\}`)

// resolved fills in parameter locations: names in the path template are path
// parameters, the rest go in the query for GET and DELETE and in the JSON
// body otherwise.
func (e endpoint) resolved() endpoint {
	inPathNames := map[string]bool{}
	for _, m := range pathParam.FindAllStringSubmatch(e.path, -1) {
		inPathNames[m[1]] = true
	}
	out := e
	out.params = slices.Clone(e.params)
	for i, p := range out.params {
		switch {
		case inPathNames[p.name]:
			p.in = inPath
			p.required = true
		case p.in != inAuto:
		case e.method == http.MethodGet || e.method == http.MethodDelete:
			p.in = inQuery
		default:
			p.in = inBody
		}
		out.params[i] = p
	}
	if e.page != nil {
		out.params = append(out.params,
			param{name: "page", in: inQuery, kind: kindInt, desc: "Zero-based page number (default 0); use has_next_page from the previous page."},
			param{name: "per_page", in: inQuery, kind: kindInt, desc: fmt.Sprintf("Results per page, %d to %d (default %d).", e.page.min, e.page.max, e.page.def)},
		)
	}
	return out
}

func (e endpoint) definition() mcp.ToolDefinition {
	parameters := make(map[string]string, len(e.params))
	var required []string
	for _, p := range e.params {
		parameters[p.name] = p.desc
		if p.required {
			required = append(required, p.name)
		}
	}
	return mcp.ToolDefinition{Name: e.tool, Description: e.desc, Parameters: parameters, Required: required, Access: e.access}
}

// request builds the path, query and body from declared arguments only;
// undeclared arguments, such as session defaults for other integrations,
// are ignored.
func (e endpoint) request(args map[string]any) (string, url.Values, map[string]any, error) {
	r := mcp.NewArgs(args)
	path := e.path
	query := url.Values{}
	for key, values := range e.fixed {
		query[key] = slices.Clone(values)
	}
	var body map[string]any
	if e.method != http.MethodGet && e.method != http.MethodDelete {
		body = map[string]any{}
	}
	var missing, problems []string
	for _, p := range e.params {
		raw, present := args[p.name]
		if !present || raw == nil || raw == "" {
			if p.required {
				missing = append(missing, p.name)
			}
			continue
		}
		var v any
		switch p.kind {
		case kindString:
			text := strings.TrimSpace(r.Str(p.name))
			if text == "" {
				if p.required {
					missing = append(missing, p.name)
				}
				continue
			}
			v = text
		case kindInt:
			v = r.Int(p.name)
		case kindBool:
			v = r.Bool(p.name)
		case kindStrings:
			items := r.StrSlice(p.name)
			for i := range items {
				items[i] = strings.TrimSpace(items[i])
			}
			v = items
		case kindInts:
			list, err := intList(raw)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", p.name, err))
				continue
			}
			v = list
		case kindObject:
			v = r.Map(p.name)
		case kindAny:
			v = raw
		}
		if len(p.enum) > 0 {
			if bad := outsideEnum(v, p.enum); bad != "" {
				problems = append(problems, fmt.Sprintf("%s must be one of %s (got %q)", p.name, strings.Join(p.enum, ", "), bad))
				continue
			}
		}
		switch p.in {
		case inPath:
			path = strings.Replace(path, "{"+p.name+"}", url.PathEscape(scalarText(v, false)), 1)
		case inQuery:
			query.Set(p.name, scalarText(v, p.intBool))
		case inBody:
			body[p.name] = v
		}
	}
	if err := r.Err(); err != nil {
		return "", nil, nil, err
	}
	if len(missing) > 0 {
		return "", nil, nil, fmt.Errorf("missing required argument(s): %s", strings.Join(missing, ", "))
	}
	if len(problems) > 0 {
		return "", nil, nil, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	if e.page != nil {
		if err := e.page.apply(query); err != nil {
			return "", nil, nil, err
		}
	}
	return path, query, body, nil
}

// apply validates per_page and sets the defaults, so list calls never fall
// back to an endpoint's "everything at once" behavior.
func (p *paging) apply(query url.Values) error {
	if query.Get("page") == "" {
		query.Set("page", "0")
	} else if n, _ := strconv.Atoi(query.Get("page")); n < 0 {
		return fmt.Errorf("page must be 0 or greater")
	}
	if query.Get("per_page") == "" {
		query.Set("per_page", strconv.Itoa(p.def))
		return nil
	}
	n, _ := strconv.Atoi(query.Get("per_page"))
	if n < p.min || n > p.max {
		return fmt.Errorf("per_page must be between %d and %d", p.min, p.max)
	}
	return nil
}

// intList accepts a JSON array of numbers or numeric strings, or a
// comma-separated string, since models send IDs in either form.
func intList(raw any) ([]int, error) {
	var items []any
	switch t := raw.(type) {
	case []any:
		items = t
	case string:
		for _, part := range strings.Split(t, ",") {
			items = append(items, part)
		}
	default:
		items = []any{t}
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		switch n := item.(type) {
		case float64:
			if n != float64(int(n)) {
				return nil, fmt.Errorf("%v is not a whole number", n)
			}
			out = append(out, int(n))
		case int:
			out = append(out, n)
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(n))
			if err != nil {
				return nil, fmt.Errorf("%q is not a number", n)
			}
			out = append(out, parsed)
		default:
			return nil, fmt.Errorf("%v is not a number", item)
		}
	}
	return out, nil
}

func outsideEnum(v any, allowed []string) string {
	values := []string{}
	switch t := v.(type) {
	case string:
		values = append(values, t)
	case []string:
		values = t
	}
	for _, item := range values {
		if !slices.Contains(allowed, item) {
			return item
		}
	}
	return ""
}

func scalarText(v any, intBool bool) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case bool:
		if intBool {
			if t {
				return "1"
			}
			return "0"
		}
		return strconv.FormatBool(t)
	case []string:
		return strings.Join(t, ",")
	case []int:
		parts := make([]string, len(t))
		for i, n := range t {
			parts[i] = strconv.Itoa(n)
		}
		return strings.Join(parts, ",")
	}
	data, _ := json.Marshal(v)
	return string(data)
}

// pageResult is the wrapped response of a paginated endpoint. Results stay
// raw so records are parsed once, by the response pipeline.
type pageResult struct {
	Results     json.RawMessage `json:"results"`
	Page        int             `json:"page"`
	PerPage     int             `json:"per_page"`
	HasNextPage bool            `json:"has_next_page"`
}

func (e endpoint) run(ctx context.Context, a *aikido, args map[string]any) (*mcp.ToolResult, error) {
	path, query, body, err := e.request(args)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("%s: %w", e.tool, err))
	}
	var payload any
	if body != nil {
		payload = body
	}
	resp, err := a.do(ctx, e.method, path, query, payload, e.scope)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data := resp.body
	if len(strings.TrimSpace(string(data))) == 0 {
		return mcp.JSONResult(map[string]any{"ok": true, "status": resp.status})
	}
	if !json.Valid(data) {
		if e.redact != nil {
			return mcp.ErrResult(fmt.Errorf("aikido: %s returned a response that is not valid JSON, so it cannot be checked for secrets", e.tool))
		}
		return mcp.RawResult(data)
	}
	if e.redact != nil {
		var parsed any
		if err := json.Unmarshal(data, &parsed); err != nil {
			return mcp.ErrResult(fmt.Errorf("aikido: decode response: %w", err))
		}
		e.redact(parsed)
		if e.page == nil {
			return mcp.JSONResult(parsed)
		}
		if data, err = json.Marshal(parsed); err != nil {
			return mcp.ErrResult(fmt.Errorf("aikido: encode response: %w", err))
		}
	}
	if e.page == nil {
		return mcp.RawResult(data)
	}
	if e.unwrap != "" {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(data, &wrapper); err != nil {
			return mcp.ErrResult(fmt.Errorf("aikido: decode response: %w", err))
		}
		inner, ok := wrapper[e.unwrap]
		if !ok {
			inner = json.RawMessage("[]")
		}
		data = inner
	}
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))
	return mcp.JSONResult(pageResult{Results: data, Page: page, PerPage: perPage, HasNextPage: hasNextPage(resp.hasNext, data, perPage)})
}

// hasNextPage trusts Aikido's X-Has-Next-Page header and otherwise treats
// a full page as a sign that more results may follow.
func hasNextPage(header string, data []byte, perPage int) bool {
	switch strings.ToLower(strings.TrimSpace(header)) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	var items []json.RawMessage
	if json.Unmarshal(data, &items) != nil {
		return false
	}
	return len(items) >= perPage
}

// redactFields replaces the named fields anywhere in a parsed response.
func redactFields(names ...string) func(any) {
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for key, child := range t {
				if slices.Contains(names, key) {
					if child != nil && child != "" {
						t[key] = "[redacted]"
					}
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	return walk
}

var endpoints = func() []endpoint {
	all := slices.Concat(issueEndpoints, codeEndpoints, containerEndpoints, cloudEndpoints, domainEndpoints,
		orgEndpoints, reportEndpoints, researchEndpoints, zenEndpoints, remediationEndpoints)
	for i := range all {
		all[i] = all[i].resolved()
	}
	return all
}()

var dispatch = func() map[mcp.ToolName]endpoint {
	m := make(map[mcp.ToolName]endpoint, len(endpoints))
	for _, ep := range endpoints {
		m[ep.tool] = ep
	}
	return m
}()

var tools = func() []mcp.ToolDefinition {
	out := make([]mcp.ToolDefinition, len(endpoints))
	for i, ep := range endpoints {
		out[i] = ep.definition()
	}
	return out
}()
