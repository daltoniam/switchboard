package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type handlerFunc func(ctx context.Context, g *grafana, args map[string]any) (*mcp.ToolResult, error)

const (
	defaultSearchLimit   = 50
	maxSearchLimit       = 5000
	defaultFolderLimit   = 100
	maxFolderLimit       = 1000
	defaultMaxDataPoints = 300
	maxMaxDataPoints     = 2000
)

// readJSON GETs a read-only endpoint and returns its body.
func readJSON(ctx context.Context, g *grafana, path string, q url.Values) (*mcp.ToolResult, error) {
	data, err := g.doRequest(ctx, http.MethodGet, path, q, nil, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(data)
}

// objectArg reads an argument that may arrive as a JSON object or as a
// string holding one. Missing and empty values return nil.
func objectArg(args map[string]any, key string) (map[string]any, error) {
	switch v := args[key].(type) {
	case nil:
		return nil, nil
	case map[string]any:
		return v, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		dec := json.NewDecoder(strings.NewReader(v))
		dec.UseNumber()
		var obj map[string]any
		if err := dec.Decode(&obj); err != nil || obj == nil {
			return nil, fmt.Errorf("grafana: %s must be a JSON object", key)
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("grafana: %s must be a JSON object", key)
	}
}

func searchDashboards(ctx context.Context, g *grafana, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	tags := r.StrSlice("tags")
	folderUIDs := r.StrSlice("folder_uids")
	starred := r.Bool("starred")
	limit := r.Int("limit")
	page := r.Int("page")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit == 0 {
		limit = defaultSearchLimit
	}
	if limit < 1 || limit > maxSearchLimit {
		return mcp.ErrResult(fmt.Errorf("grafana: limit must be between 1 and %d", maxSearchLimit))
	}
	q := url.Values{"type": {"dash-db"}, "limit": {strconv.Itoa(limit)}}
	if query != "" {
		q.Set("query", query)
	}
	for _, tag := range tags {
		q.Add("tag", tag)
	}
	for _, uid := range folderUIDs {
		q.Add("folderUIDs", uid)
	}
	if starred {
		q.Set("starred", "true")
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	return readJSON(ctx, g, "/api/search", q)
}

func getDashboard(ctx context.Context, g *grafana, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	uid := strings.TrimSpace(r.Str("uid"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if uid == "" {
		return mcp.ErrResult(fmt.Errorf("grafana: uid is required"))
	}
	return readJSON(ctx, g, "/api/dashboards/uid/"+url.PathEscape(uid), nil)
}

func saveDashboard(ctx context.Context, g *grafana, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	folderUID := strings.TrimSpace(r.Str("folder_uid"))
	overwrite := r.Bool("overwrite")
	message := r.Str("message")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	dash, err := objectArg(args, "dashboard")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if dash == nil {
		return mcp.ErrResult(fmt.Errorf("grafana: dashboard is required"))
	}
	// Accept the {dashboard, meta} result of grafana_get_dashboard as is.
	if inner, ok := dash["dashboard"].(map[string]any); ok && dash["title"] == nil {
		dash = inner
	}
	if title, _ := dash["title"].(string); strings.TrimSpace(title) == "" {
		return mcp.ErrResult(fmt.Errorf("grafana: dashboard needs a title"))
	}
	inputs, err := objectArg(args, "inputs")
	if err != nil {
		return mcp.ErrResult(err)
	}
	importInputs, err := resolveInputs(dash, inputs)
	if err != nil {
		return mcp.ErrResult(err)
	}

	dash = maps.Clone(dash)
	isImport := importInputs != nil
	// An import with a folder needs nothing from the stored dashboard.
	if uid, _ := dash["uid"].(string); uid != "" && (!isImport || folderUID == "") {
		existing, err := findDashboard(ctx, g, uid)
		if err != nil {
			return mcp.ErrResult(err)
		}
		switch {
		case existing == nil || isImport:
			// The import API matches by uid. For a new dashboard, an id copied
			// from another Grafana, or from an export, would point at a
			// different dashboard or none.
			delete(dash, "id")
		default:
			// An update. Grafana needs the stored id to check the version
			// instead of reporting a uid clash.
			dash["id"] = existing.id
		}
		// Saving without a folder would move an existing dashboard to the root.
		if existing != nil && folderUID == "" {
			folderUID = existing.folderUID
		}
	}

	body := map[string]any{"dashboard": dash, "overwrite": overwrite}
	if folderUID != "" {
		body["folderUid"] = folderUID
	}
	path := "/api/dashboards/db"
	if importInputs != nil {
		path = "/api/dashboards/import"
		body["inputs"] = importInputs
	} else if message != "" {
		body["message"] = message
	}
	data, err := g.doRequest(ctx, http.MethodPost, path, nil, body, false)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(data)
}

// storedDashboard is what a save needs to know about the dashboard a uid
// already names.
type storedDashboard struct {
	id        any
	folderUID string
}

// findDashboard looks up the dashboard a uid names. It returns nil when there
// is none.
func findDashboard(ctx context.Context, g *grafana, uid string) (*storedDashboard, error) {
	data, err := g.doRequest(ctx, http.MethodGet, "/api/dashboards/uid/"+url.PathEscape(uid), nil, nil, true)
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	found, _ := data.(map[string]any)
	dash, _ := found["dashboard"].(map[string]any)
	meta, _ := found["meta"].(map[string]any)
	stored := &storedDashboard{id: dash["id"]}
	stored.folderUID, _ = meta["folderUid"].(string)
	return stored, nil
}

// resolveInputs pairs an exported dashboard's __inputs with the values the
// caller gave. It returns nil when the dashboard has no __inputs, meaning a
// plain save rather than an import.
func resolveInputs(dash, given map[string]any) ([]map[string]any, error) {
	declared, _ := dash["__inputs"].([]any)
	known := map[string]map[string]any{}
	var order []string
	for _, item := range declared {
		in, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := in["name"].(string); name != "" {
			known[name] = in
			order = append(order, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(given)) {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("grafana: input %s is not one of the dashboard's __inputs", name)
		}
		if _, ok := given[name].(string); !ok {
			return nil, fmt.Errorf("grafana: input %s must be a string", name)
		}
	}
	if len(order) == 0 {
		return nil, nil
	}
	var missing []string
	for _, name := range order {
		if v, _ := given[name].(string); v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("grafana: this is an exported dashboard; pass inputs with a value for %s (for a data source, its uid from grafana_list_datasources)", strings.Join(missing, ", "))
	}
	out := make([]map[string]any, 0, len(order))
	for _, name := range order {
		out = append(out, map[string]any{
			"name":     name,
			"type":     known[name]["type"],
			"pluginId": known[name]["pluginId"],
			"value":    given[name],
		})
	}
	return out, nil
}

func listFolders(ctx context.Context, g *grafana, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parentUID := strings.TrimSpace(r.Str("parent_uid"))
	limit := r.Int("limit")
	page := r.Int("page")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit == 0 {
		limit = defaultFolderLimit
	}
	if limit < 1 || limit > maxFolderLimit {
		return mcp.ErrResult(fmt.Errorf("grafana: limit must be between 1 and %d", maxFolderLimit))
	}
	q := url.Values{"limit": {strconv.Itoa(limit)}}
	if parentUID != "" {
		q.Set("parentUid", parentUID)
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	return readJSON(ctx, g, "/api/folders", q)
}

func listDatasources(ctx context.Context, g *grafana, _ map[string]any) (*mcp.ToolResult, error) {
	return readJSON(ctx, g, "/api/datasources", nil)
}

func queryDatasource(ctx context.Context, g *grafana, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	uid := strings.TrimSpace(r.Str("datasource_uid"))
	expr := r.Str("expr")
	from := strings.TrimSpace(r.Str("from"))
	to := strings.TrimSpace(r.Str("to"))
	instant := r.Bool("instant")
	maxDataPoints := r.Int("max_data_points")
	intervalMs := r.Int("interval_ms")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	extra, err := objectArg(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if uid == "" {
		return mcp.ErrResult(fmt.Errorf("grafana: datasource_uid is required"))
	}
	if strings.TrimSpace(expr) == "" && extra == nil {
		return mcp.ErrResult(fmt.Errorf("grafana: expr or query is required"))
	}
	if maxDataPoints == 0 {
		maxDataPoints = defaultMaxDataPoints
	}
	if maxDataPoints < 1 || maxDataPoints > maxMaxDataPoints {
		return mcp.ErrResult(fmt.Errorf("grafana: max_data_points must be between 1 and %d", maxMaxDataPoints))
	}
	if from == "" {
		from = "now-1h"
	}
	if to == "" {
		to = "now"
	}

	if intervalMs == 0 && !instant {
		intervalMs = queryInterval(from, to, maxDataPoints, time.Now())
	}
	query := map[string]any{"instant": instant, "range": !instant, "maxDataPoints": maxDataPoints}
	if expr != "" {
		query["expr"] = expr
	}
	if intervalMs > 0 {
		query["intervalMs"] = intervalMs
	}
	maps.Copy(query, extra)
	query["refId"] = "A"
	query["datasource"] = map[string]any{"uid": uid}

	body := map[string]any{"from": from, "to": to, "queries": []any{query}}
	data, err := g.doRequest(ctx, http.MethodPost, "/api/ds/query", nil, body, true)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) {
			if detail := queryErrors(apiErr.body); detail != "" {
				return mcp.ErrResult(fmt.Errorf("grafana: query failed (%d): %s", apiErr.status, detail))
			}
		}
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resultList(data))
}

var relativeTime = regexp.MustCompile(`^now-(\d+)([smhdw])$`)

var relativeUnits = map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour}

// parseTime reads the time forms queryInterval understands: now, now-<n><unit>,
// epoch milliseconds and RFC3339. Anything else, such as now/d, is left to
// Grafana.
func parseTime(value string, now time.Time) (time.Time, bool) {
	if value == "now" {
		return now, true
	}
	if m := relativeTime.FindStringSubmatch(value); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return time.Time{}, false
		}
		return now.Add(-time.Duration(n) * relativeUnits[m[2]]), true
	}
	if ms, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.UnixMilli(ms), true
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// queryInterval spreads points over from..to, the step Grafana's own
// frontend sends; /api/ds/query does not derive it from maxDataPoints. It
// returns 0, leaving the step to Grafana, when the range cannot be read.
func queryInterval(from, to string, points int, now time.Time) int {
	start, ok := parseTime(from, now)
	if !ok {
		return 0
	}
	end, ok := parseTime(to, now)
	if !ok || !end.After(start) || points < 1 {
		return 0
	}
	span := end.Sub(start).Milliseconds()
	step := int((span + int64(points) - 1) / int64(points))
	return max(step, 1000)
}

// resultList turns the {"results": {"A": {...}}} map into a list ordered by
// ref_id, so compaction can address each result.
func resultList(data any) []map[string]any {
	obj, _ := data.(map[string]any)
	results, _ := obj["results"].(map[string]any)
	out := make([]map[string]any, 0, len(results))
	for _, ref := range slices.Sorted(maps.Keys(results)) {
		entry := map[string]any{"ref_id": ref}
		if res, ok := results[ref].(map[string]any); ok {
			maps.Copy(entry, res)
		}
		out = append(out, entry)
	}
	return out
}

// queryErrors joins the per-query error messages of a failed /api/ds/query.
func queryErrors(body any) string {
	var parts []string
	for _, res := range resultList(body) {
		if msg, _ := res["error"].(string); msg != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", res["ref_id"], msg))
		}
	}
	return strings.Join(parts, "; ")
}

func getHealth(ctx context.Context, g *grafana, _ map[string]any) (*mcp.ToolResult, error) {
	health, err := g.doRequest(ctx, http.MethodGet, "/api/health", nil, nil, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	org, err := g.doRequest(ctx, http.MethodGet, "/api/org", nil, nil, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"grafana": health, "organization": org})
}
