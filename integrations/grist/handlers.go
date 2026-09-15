package grist

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func pathEscape(id string) string {
	return url.PathEscape(id)
}

func anyJSONArg(args map[string]any, key string) (any, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	switch v := v.(type) {
	case string:
		if v == "" {
			return nil, fmt.Errorf("%s is required", key)
		}
		var out any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON for %s: %w", key, err)
		}
		return out, nil
	default:
		return v, nil
	}
}

func optionalJSONArg(args map[string]any, key string) (any, bool, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, false, nil
	}
	if s, isStr := v.(string); isStr && s == "" {
		return nil, false, nil
	}
	out, err := anyJSONArg(args, key)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func listLimit(args map[string]any) (int, error) {
	r := mcp.NewArgs(args)
	limit := r.Int("limit")
	unlimited := r.Bool("unlimited")
	if err := r.Err(); err != nil {
		return 0, err
	}
	if _, ok := args["limit"]; !ok {
		return defaultLimit, nil
	}
	if limit == 0 {
		if !unlimited {
			return 0, fmt.Errorf("limit 0 fetches the whole table; pass unlimited=true to confirm")
		}
		return 0, nil
	}
	if limit > maxLimit && !unlimited {
		return maxLimit, nil
	}
	return limit, nil
}

func parseIntIDs(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid record id %q", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("record_ids is required")
	}
	return out, nil
}

func filterQuery(args map[string]any) (string, error) {
	v, ok, err := optionalJSONArg(args, "filter")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	if s, isStr := v.(string); isStr {
		return s, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("invalid JSON for filter: %w", err)
	}
	return string(data), nil
}

func intFromAny(v any) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int32:
		return int(n), nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, err
		}
		return int(i), nil
	case string:
		return strconv.Atoi(n)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func recordIDsArg(args map[string]any) ([]int, error) {
	v, ok := args["record_ids"]
	if !ok || v == nil {
		return nil, fmt.Errorf("record_ids is required")
	}
	switch v := v.(type) {
	case string:
		return parseIntIDs(v)
	case []any:
		out := make([]int, 0, len(v))
		for i, item := range v {
			n, err := intFromAny(item)
			if err != nil {
				return nil, fmt.Errorf("record_ids[%d]: %w", i, err)
			}
			out = append(out, n)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("record_ids is required")
		}
		return out, nil
	case []int:
		if len(v) == 0 {
			return nil, fmt.Errorf("record_ids is required")
		}
		return v, nil
	default:
		parsed, err := anyJSONArg(args, "record_ids")
		if err != nil {
			return nil, err
		}
		if s, isStr := parsed.(string); isStr {
			return parseIntIDs(s)
		}
		items, ok := parsed.([]any)
		if !ok {
			return nil, fmt.Errorf("record_ids must be a CSV string or array of ids")
		}
		out := make([]int, 0, len(items))
		for i, item := range items {
			n, err := intFromAny(item)
			if err != nil {
				return nil, fmt.Errorf("record_ids[%d]: %w", i, err)
			}
			out = append(out, n)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("record_ids is required")
		}
		return out, nil
	}
}

func listOrgs(ctx context.Context, g *grist, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/api/orgs")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getOrg(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	orgID := r.Str("org_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if orgID == "" {
		return mcp.ErrResult(fmt.Errorf("org_id is required"))
	}
	data, err := g.get(ctx, "/api/orgs/%s", pathEscape(orgID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listWorkspaces(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	orgID := r.Str("org_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if orgID == "" {
		return mcp.ErrResult(fmt.Errorf("org_id is required"))
	}
	data, err := g.get(ctx, "/api/orgs/%s/workspaces", pathEscape(orgID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getWorkspace(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	workspaceID := r.Int("workspace_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if workspaceID == 0 {
		return mcp.ErrResult(fmt.Errorf("workspace_id is required"))
	}
	data, err := g.get(ctx, "/api/workspaces/%d", workspaceID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createWorkspace(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	orgID := r.Str("org_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if orgID == "" {
		return mcp.ErrResult(fmt.Errorf("org_id is required"))
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	data, err := g.post(ctx, "/api/orgs/"+pathEscape(orgID)+"/workspaces", map[string]any{"name": name})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteWorkspace(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	workspaceID := r.Int("workspace_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if workspaceID == 0 {
		return mcp.ErrResult(fmt.Errorf("workspace_id is required"))
	}
	data, err := g.del(ctx, "/api/workspaces/%d", workspaceID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDoc(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	data, err := g.get(ctx, "/api/docs/%s", pathEscape(docID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDoc(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	workspaceID := r.Int("workspace_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if workspaceID == 0 {
		return mcp.ErrResult(fmt.Errorf("workspace_id is required"))
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	data, err := g.post(ctx, fmt.Sprintf("/api/workspaces/%d/docs", workspaceID), map[string]any{"name": name})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDoc(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	data, err := g.patch(ctx, "/api/docs/"+pathEscape(docID), map[string]any{"name": name})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func moveDoc(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	workspaceID := r.Int("workspace_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if workspaceID == 0 {
		return mcp.ErrResult(fmt.Errorf("workspace_id is required"))
	}
	data, err := g.patch(ctx, "/api/docs/"+pathEscape(docID)+"/move", map[string]any{"workspace": workspaceID})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func copyDoc(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	workspaceID := r.Int("workspace_id")
	name := r.Str("name")
	asTemplate := r.Bool("as_template")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if workspaceID == 0 {
		return mcp.ErrResult(fmt.Errorf("workspace_id is required"))
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	body := map[string]any{"workspaceId": workspaceID, "documentName": name}
	if asTemplate {
		body["asTemplate"] = true
	}
	data, err := g.post(ctx, "/api/docs/"+pathEscape(docID)+"/copy", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDoc(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	data, err := g.del(ctx, "/api/docs/%s", pathEscape(docID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTables(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	expand := r.Bool("expand")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	params := map[string]string{}
	if expand {
		params["expand"] = "columns"
	}
	data, err := g.get(ctx, "/api/docs/%s/tables%s", pathEscape(docID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapKey(data, "tables"))
}

func createTables(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	tables, err := anyJSONArg(args, "tables")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, "/api/docs/"+pathEscape(docID)+"/tables", map[string]any{"tables": tables})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTables(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	tables, err := anyJSONArg(args, "tables")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, "/api/docs/"+pathEscape(docID)+"/tables", map[string]any{"tables": tables})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listColumns(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	hidden := r.Bool("hidden")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	params := map[string]string{}
	if hidden {
		params["hidden"] = "true"
	}
	data, err := g.get(ctx, "/api/docs/%s/tables/%s/columns%s", pathEscape(docID), pathEscape(tableID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapKey(data, "columns"))
}

func addColumns(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	columns, err := anyJSONArg(args, "columns")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/api/docs/%s/tables/%s/columns", pathEscape(docID), pathEscape(tableID)), map[string]any{"columns": columns})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateColumns(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	columns, err := anyJSONArg(args, "columns")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, fmt.Sprintf("/api/docs/%s/tables/%s/columns", pathEscape(docID), pathEscape(tableID)), map[string]any{"columns": columns})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteColumn(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	colID := r.Str("col_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	if colID == "" {
		return mcp.ErrResult(fmt.Errorf("col_id is required"))
	}
	data, err := g.del(ctx, "/api/docs/%s/tables/%s/columns/%s", pathEscape(docID), pathEscape(tableID), pathEscape(colID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRecords(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	sort := r.Str("sort")
	cellFormat := r.Str("cell_format")
	hidden := r.Bool("hidden")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	limit, err := listLimit(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	filter, err := filterQuery(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if filter != "" {
		params["filter"] = filter
	}
	if sort != "" {
		params["sort"] = sort
	}
	if cellFormat != "" {
		params["cellFormat"] = cellFormat
	}
	if hidden {
		params["hidden"] = "true"
	}
	params["limit"] = strconv.Itoa(limit)
	data, err := g.get(ctx, "/api/docs/%s/tables/%s/records%s", pathEscape(docID), pathEscape(tableID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapKey(data, "records"))
}

func addRecords(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	noparse := r.Bool("noparse")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	records, err := anyJSONArg(args, "records")
	if err != nil {
		return mcp.ErrResult(err)
	}
	q := ""
	if noparse {
		q = "?noparse=true"
	}
	data, err := g.post(ctx, fmt.Sprintf("/api/docs/%s/tables/%s/records%s", pathEscape(docID), pathEscape(tableID), q), map[string]any{"records": records})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateRecords(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	noparse := r.Bool("noparse")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	records, err := anyJSONArg(args, "records")
	if err != nil {
		return mcp.ErrResult(err)
	}
	q := ""
	if noparse {
		q = "?noparse=true"
	}
	data, err := g.patch(ctx, fmt.Sprintf("/api/docs/%s/tables/%s/records%s", pathEscape(docID), pathEscape(tableID), q), map[string]any{"records": records})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func upsertRecords(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	onmany := r.Str("onmany")
	noadd := r.Bool("noadd")
	noupdate := r.Bool("noupdate")
	allowEmpty := r.Bool("allow_empty_require")
	noparse := r.Bool("noparse")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	records, err := anyJSONArg(args, "records")
	if err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if onmany != "" {
		params["onmany"] = onmany
	}
	if noadd {
		params["noadd"] = "true"
	}
	if noupdate {
		params["noupdate"] = "true"
	}
	if allowEmpty {
		params["allow_empty_require"] = "true"
	}
	if noparse {
		params["noparse"] = "true"
	}
	data, err := g.put(ctx, fmt.Sprintf("/api/docs/%s/tables/%s/records%s", pathEscape(docID), pathEscape(tableID), queryEncode(params)), map[string]any{"records": records})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteRecords(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	tableID := r.Str("table_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if tableID == "" {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	ids, err := recordIDsArg(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/api/docs/%s/tables/%s/records/delete", pathEscape(docID), pathEscape(tableID)), ids)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func querySQL(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	sql := r.Str("sql")
	timeout := r.Int("timeout")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if sql == "" {
		return mcp.ErrResult(fmt.Errorf("sql is required"))
	}
	body := map[string]any{"sql": sql}
	sqlArgs, ok, err := optionalJSONArg(args, "args")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if ok {
		body["args"] = sqlArgs
	}
	if timeout > 0 {
		body["timeout"] = timeout
	}
	data, err := g.post(ctx, "/api/docs/"+pathEscape(docID)+"/sql", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapKey(data, "records"))
}

func listWebhooks(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	data, err := g.get(ctx, "/api/docs/%s/webhooks", pathEscape(docID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapKey(data, "webhooks"))
}

func createWebhooks(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	webhooks, err := anyJSONArg(args, "webhooks")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, "/api/docs/"+pathEscape(docID)+"/webhooks", map[string]any{"webhooks": webhooks})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteWebhook(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	webhookID := r.Str("webhook_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if webhookID == "" {
		return mcp.ErrResult(fmt.Errorf("webhook_id is required"))
	}
	data, err := g.del(ctx, "/api/docs/%s/webhooks/%s", pathEscape(docID), pathEscape(webhookID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAttachments(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	sort := r.Str("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	limit, err := listLimit(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	filter, err := filterQuery(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{"limit": strconv.Itoa(limit)}
	if filter != "" {
		params["filter"] = filter
	}
	if sort != "" {
		params["sort"] = sort
	}
	data, err := g.get(ctx, "/api/docs/%s/attachments%s", pathEscape(docID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapKey(data, "records"))
}

func getAttachment(ctx context.Context, g *grist, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("doc_id")
	attachmentID := r.Int("attachment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("doc_id is required"))
	}
	if attachmentID == 0 {
		return mcp.ErrResult(fmt.Errorf("attachment_id is required"))
	}
	data, err := g.get(ctx, "/api/docs/%s/attachments/%d", pathEscape(docID), attachmentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
