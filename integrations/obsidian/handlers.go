package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

const (
	acceptNote        = "application/vnd.olrapi.note+json"
	acceptDocumentMap = "application/vnd.olrapi.document-map+json"
	contentMarkdown   = "text/markdown"
	contentJSONLogic  = "application/vnd.olrapi.jsonlogic+json"

	defaultSearchLimit = 20
	defaultQueryLimit  = 50
)

// cleanPath normalizes a vault-relative path and rejects anything that could
// leave the vault. A trailing slash (a folder) is preserved.
func cleanPath(raw string) (string, error) {
	p := strings.TrimLeft(strings.TrimSpace(raw), "/")
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("obsidian: path must not contain '..' segments")
		}
	}
	return p, nil
}

// notePath returns the cleaned path of a single note or file.
func notePath(args map[string]any) (string, error) {
	raw, err := mcp.ArgStr(args, "path")
	if err != nil {
		return "", err
	}
	p, err := cleanPath(raw)
	if err != nil {
		return "", err
	}
	if p == "" {
		return "", fmt.Errorf("obsidian: path is required (vault-relative, e.g. Projects/Plan.md)")
	}
	if strings.HasSuffix(p, "/") {
		return "", fmt.Errorf("obsidian: %q is a folder; use obsidian_list_files for folders", p)
	}
	return p, nil
}

// escapePath percent-encodes each segment so spaces and reserved characters
// in note names survive the URL.
func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

func vaultURL(p string, target ...string) string {
	out := "/vault/" + escapePath(p)
	for _, t := range target {
		out += "/" + url.PathEscape(t)
	}
	return out
}

// headingPath accepts a heading as a single string or as an array of
// headings from the top level down. A string is never split, so a heading
// may contain commas or slashes.
func headingPath(args map[string]any, key string) ([]string, error) {
	switch v := args[key].(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return []string{v}, nil
	default:
		return mcp.ArgStrSlice(args, key)
	}
}

func blockID(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "^") }

func listFiles(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	raw, err := mcp.ArgStr(args, "path")
	if err != nil {
		return mcp.ErrResult(err)
	}
	dir, err := cleanPath(raw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if dir != "" && !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	data, err := o.do(ctx, request{method: http.MethodGet, path: "/vault/" + escapePath(dir)})
	if err != nil {
		return mcp.ErrResult(err)
	}
	var listing struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(data, &listing); err != nil {
		return mcp.ErrResult(fmt.Errorf("obsidian: decode file list: %w", err))
	}
	files := make([]string, len(listing.Files))
	for i, f := range listing.Files {
		files[i] = dir + f
	}
	return mcp.JSONResult(map[string]any{"path": dir, "files": files, "count": len(files)})
}

func readNote(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	p, err := notePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	block := blockID(r.Str("block"))
	field := strings.TrimSpace(r.Str("frontmatter_field"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	heading, err := headingPath(args, "heading")
	if err != nil {
		return mcp.ErrResult(err)
	}

	var segs []string
	var label string
	targets := 0
	if len(heading) > 0 {
		targets++
		segs = append([]string{"heading"}, heading...)
		label = "heading: " + strings.Join(heading, " > ")
	}
	if block != "" {
		targets++
		segs = []string{"block", block}
		label = "block: " + block
	}
	if field != "" {
		targets++
		segs = []string{"frontmatter", field}
		label = "frontmatter: " + field
	}
	if targets > 1 {
		return mcp.ErrResult(fmt.Errorf("obsidian: set only one of heading, block, or frontmatter_field"))
	}

	if targets == 0 {
		data, err := o.do(ctx, request{method: http.MethodGet, path: vaultURL(p), accept: acceptNote})
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
	// Heading and block reads return markdown; a frontmatter field always
	// comes back as its JSON value (e.g. "draft" with quotes), kept as text.
	data, err := o.do(ctx, request{method: http.MethodGet, path: vaultURL(p, segs...), accept: contentMarkdown})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"path": p, "target": label, "content": string(data)})
}

func getDocumentMap(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	p, err := notePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.do(ctx, request{method: http.MethodGet, path: vaultURL(p), accept: acceptDocumentMap})
	if err != nil {
		return mcp.ErrResult(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return mcp.ErrResult(fmt.Errorf("obsidian: decode document map: %w", err))
	}
	out["path"] = p
	return mcp.JSONResult(out)
}

func search(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := strings.TrimSpace(r.Str("query"))
	contextLength := r.Int("context_length")
	limit := r.OptInt("limit", defaultSearchLimit)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if q == "" {
		return mcp.ErrResult(fmt.Errorf("obsidian: query is required"))
	}
	params := url.Values{"query": {q}}
	if contextLength > 0 {
		params.Set("contextLength", strconv.Itoa(contextLength))
	}
	data, err := o.do(ctx, request{method: http.MethodPost, path: "/search/simple/", query: params})
	if err != nil {
		return mcp.ErrResult(err)
	}
	var results []any
	if err := json.Unmarshal(data, &results); err != nil {
		return mcp.ErrResult(fmt.Errorf("obsidian: decode search results: %w", err))
	}
	out := limited(results, limit)
	out["query"] = q
	return mcp.JSONResult(out)
}

func query(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	limit := r.OptInt("limit", defaultQueryLimit)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var logic map[string]any
	switch v := args["jsonlogic"].(type) {
	case nil:
		return mcp.ErrResult(fmt.Errorf("obsidian: jsonlogic is required"))
	case map[string]any:
		logic = v
	case string:
		if strings.TrimSpace(v) == "" {
			return mcp.ErrResult(fmt.Errorf("obsidian: jsonlogic is required"))
		}
		if err := json.Unmarshal([]byte(v), &logic); err != nil {
			return mcp.ErrResult(fmt.Errorf("obsidian: jsonlogic must be a JSON object: %w", err))
		}
	default:
		return mcp.ErrResult(fmt.Errorf("obsidian: jsonlogic must be a JSON object, got %T", v))
	}
	body, err := json.Marshal(logic)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.do(ctx, request{method: http.MethodPost, path: "/search/", contentType: contentJSONLogic, body: body})
	if err != nil {
		return mcp.ErrResult(err)
	}
	var results []any
	if err := json.Unmarshal(data, &results); err != nil {
		return mcp.ErrResult(fmt.Errorf("obsidian: decode query results: %w", err))
	}
	return mcp.JSONResult(limited(results, limit))
}

func limited(results []any, limit int) map[string]any {
	total := len(results)
	if results == nil {
		results = []any{}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return map[string]any{"total": total, "returned": len(results), "results": results}
}

func listTags(ctx context.Context, o *obsidian, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.do(ctx, request{method: http.MethodGet, path: "/tags/"})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCommands(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	filter, err := mcp.ArgStr(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	data, err := o.do(ctx, request{method: http.MethodGet, path: "/commands/"})
	if err != nil {
		return mcp.ErrResult(err)
	}
	var listing struct {
		Commands []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(data, &listing); err != nil {
		return mcp.ErrResult(fmt.Errorf("obsidian: decode commands: %w", err))
	}
	commands := make([]map[string]string, 0, len(listing.Commands))
	for _, c := range listing.Commands {
		if filter == "" || strings.Contains(strings.ToLower(c.ID), filter) || strings.Contains(strings.ToLower(c.Name), filter) {
			commands = append(commands, map[string]string{"id": c.ID, "name": c.Name})
		}
	}
	return mcp.JSONResult(map[string]any{"total": len(commands), "commands": commands})
}

func getActiveNote(ctx context.Context, o *obsidian, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.do(ctx, request{method: http.MethodGet, path: "/active/", accept: acceptNote})
	if isStatus(err, http.StatusNotFound) {
		return mcp.ErrResult(fmt.Errorf("obsidian: no note is open in the Obsidian editor"))
	}
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func writeNote(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	p, err := notePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, ok := args["content"]; !ok {
		return mcp.ErrResult(fmt.Errorf("obsidian: content is required (pass an empty string for an empty note)"))
	}
	content, err := mcp.ArgStr(args, "content")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := o.do(ctx, request{method: http.MethodPut, path: vaultURL(p), contentType: contentMarkdown, body: []byte(content)}); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"path": p, "status": "written"})
}

func appendNote(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	p, err := notePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	content, err := mcp.ArgStr(args, "content")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if content == "" {
		return mcp.ErrResult(fmt.Errorf("obsidian: content is required"))
	}
	if _, err := o.do(ctx, request{method: http.MethodPost, path: vaultURL(p), contentType: contentMarkdown, body: []byte(content)}); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"path": p, "status": "appended"})
}

var (
	patchOperations  = map[string]bool{"append": true, "prepend": true, "replace": true, "delete": true}
	patchTargetTypes = map[string]bool{"heading": true, "block": true, "frontmatter": true}
)

// patchNote sends one structured markdown-patch (v2) instruction as JSON.
func patchNote(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	p, err := notePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	op := strings.TrimSpace(r.Str("operation"))
	targetType := strings.TrimSpace(r.Str("target_type"))
	scope := strings.TrimSpace(r.Str("scope"))
	ifMatch := strings.TrimSpace(r.Str("if_match"))
	createMissing := r.Bool("create_target_if_missing")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if !patchOperations[op] {
		return mcp.ErrResult(fmt.Errorf("obsidian: operation must be append, prepend, replace, or delete"))
	}
	if !patchTargetTypes[targetType] {
		return mcp.ErrResult(fmt.Errorf("obsidian: target_type must be heading, block, or frontmatter"))
	}

	instruction := map[string]any{"operation": op, "targetType": targetType}
	if targetType == "heading" {
		heading, err := headingPath(args, "target")
		if err != nil {
			return mcp.ErrResult(err)
		}
		if len(heading) == 0 {
			return mcp.ErrResult(fmt.Errorf("obsidian: target is required (the heading, or an array of headings from the top level down)"))
		}
		instruction["target"] = heading
	} else {
		target, err := mcp.ArgStr(args, "target")
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("obsidian: target must be a string for %s targets: %w", targetType, err))
		}
		if targetType == "block" {
			target = blockID(target)
		}
		if strings.TrimSpace(target) == "" {
			return mcp.ErrResult(fmt.Errorf("obsidian: target is required"))
		}
		instruction["target"] = target
	}

	_, hasContent := args["content"]
	value, hasValue := args["value"]
	if hasContent && hasValue {
		return mcp.ErrResult(fmt.Errorf("obsidian: provide content or value, not both"))
	}
	if op != "delete" && !hasContent && !hasValue {
		return mcp.ErrResult(fmt.Errorf("obsidian: %s needs content or value", op))
	}
	if hasContent {
		s, err := mcp.ArgStr(args, "content")
		if err != nil {
			return mcp.ErrResult(err)
		}
		instruction["content"] = s
	}
	if hasValue {
		instruction["value"] = value
	}
	if scope != "" {
		instruction["scope"] = scope
	}
	if ifMatch != "" {
		instruction["ifMatch"] = ifMatch
	}
	if createMissing {
		instruction["createTargetIfMissing"] = true
	}
	body, err := json.Marshal(instruction)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := o.do(ctx, request{
		method:      http.MethodPatch,
		path:        vaultURL(p),
		contentType: "application/json",
		headers:     map[string]string{"Markdown-Patch-Version": "2"},
		body:        body,
	}); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"path": p, "status": "patched"})
}

func deleteNote(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	p, err := notePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := o.do(ctx, request{method: http.MethodDelete, path: vaultURL(p)}); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"path": p, "status": "deleted"})
}

func executeCommand(ctx context.Context, o *obsidian, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "command_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("obsidian: command_id is required (see obsidian_list_commands)"))
	}
	if _, err := o.do(ctx, request{method: http.MethodPost, path: "/commands/" + url.PathEscape(id) + "/"}); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"command_id": id, "status": "executed"})
}
