package prometheus

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type handlerFunc func(context.Context, *prometheus, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"prometheus_query":             query,
	"prometheus_query_range":       queryRange,
	"prometheus_list_alerts":       listAlerts,
	"prometheus_list_rules":        listRules,
	"prometheus_list_targets":      listTargets,
	"prometheus_search_metrics":    searchMetrics,
	"prometheus_list_label_values": listLabelValues,
	"prometheus_list_series":       listSeries,
	"prometheus_server_info":       serverInfo,
}

var labelPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func timestamp(value string, now time.Time) (float64, error) {
	if value == "now" {
		return float64(now.UnixNano()) / 1e9, nil
	}
	if strings.HasPrefix(value, "now-") {
		seconds, err := durationSeconds(strings.TrimPrefix(value, "now-"))
		if err != nil {
			return 0, err
		}
		result := float64(now.UnixNano())/1e9 - seconds
		if !math.IsInf(result, 0) && !math.IsNaN(result) {
			return result, nil
		}
		return 0, fmt.Errorf("prometheus: invalid relative timestamp")
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil && !math.IsNaN(number) && !math.IsInf(number, 0) {
		return number, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return 0, fmt.Errorf("prometheus: invalid timestamp; use RFC3339, Unix seconds or now-1h")
	}
	return float64(parsed.UnixNano()) / 1e9, nil
}

func durationSeconds(value string) (float64, error) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		if strings.HasSuffix(value, "d") || strings.HasSuffix(value, "w") {
			multiplier := float64(86400)
			if strings.HasSuffix(value, "w") {
				multiplier *= 7
			}
			number, err = strconv.ParseFloat(value[:len(value)-1], 64)
			number *= multiplier
		} else {
			duration, parseErr := time.ParseDuration(value)
			number, err = duration.Seconds(), parseErr
		}
	}
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 {
		return 0, fmt.Errorf("prometheus: duration or step must be positive finite seconds or a duration")
	}
	return number, nil
}

func readParams(args map[string]any) (url.Values, int, error) {
	reader := mcp.NewArgs(args)
	limit := reader.Int("limit")
	if _, present := args["limit"]; !present {
		limit = 20
	}
	start, end := reader.Str("start"), reader.Str("end")
	if err := reader.Err(); err != nil {
		return nil, 0, err
	}
	if limit < 1 || limit > 100 {
		return nil, 0, fmt.Errorf("prometheus: limit must be 1..100")
	}
	params := url.Values{"limit": []string{strconv.Itoa(limit)}}
	now := time.Now()
	for key, value := range map[string]string{"start": start, "end": end} {
		if value != "" {
			number, err := timestamp(value, now)
			if err != nil {
				return nil, 0, err
			}
			params.Set(key, strconv.FormatFloat(number, 'f', -1, 64))
		}
	}
	if start != "" && end != "" {
		first, _ := strconv.ParseFloat(params.Get("start"), 64)
		last, _ := strconv.ParseFloat(params.Get("end"), 64)
		if last < first {
			return nil, 0, fmt.Errorf("prometheus: end must not precede start")
		}
	}
	if raw, ok := args["match"]; ok && raw != nil {
		var selectors []string
		switch values := raw.(type) {
		case []string:
			selectors = values
		case []any:
			for _, value := range values {
				text, ok := value.(string)
				if !ok {
					return nil, 0, fmt.Errorf("prometheus: match must be an array of strings")
				}
				selectors = append(selectors, text)
			}
		default:
			return nil, 0, fmt.Errorf("prometheus: match must be an array of strings")
		}
		if len(selectors) > 20 {
			return nil, 0, fmt.Errorf("prometheus: at most 20 selectors are allowed")
		}
		for _, selector := range selectors {
			if strings.TrimSpace(selector) == "" || len(selector) > 4096 {
				return nil, 0, fmt.Errorf("prometheus: selectors must be nonempty and at most 4096 bytes")
			}
			params.Add("match[]", selector)
		}
	}
	return params, limit, nil
}

func query(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	return expression(ctx, p, args, false)
}
func queryRange(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	return expression(ctx, p, args, true)
}

func rangeParams(params url.Values, step string) error {
	if params.Get("start") == "" || params.Get("end") == "" {
		return fmt.Errorf("prometheus: start and end are required")
	}
	start, _ := strconv.ParseFloat(params.Get("start"), 64)
	end, _ := strconv.ParseFloat(params.Get("end"), 64)
	seconds := math.Max(1, math.Ceil((end-start)/999))
	if step != "" {
		var err error
		seconds, err = durationSeconds(step)
		if err != nil {
			return err
		}
	}
	if math.Floor((end-start)/seconds)+1 > 1000 {
		return fmt.Errorf("prometheus: range exceeds 1000 evaluation points; increase step")
	}
	params.Set("step", strconv.FormatFloat(seconds, 'f', -1, 64))
	return nil
}

func expression(ctx context.Context, p *prometheus, args map[string]any, isRange bool) (*mcp.ToolResult, error) {
	params, limit, err := readParams(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	reader := mcp.NewArgs(args)
	promql, at, step := reader.Str("query"), reader.Str("time"), reader.Str("step")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if strings.TrimSpace(promql) == "" || len(promql) > 16384 {
		return mcp.ErrResult(fmt.Errorf("prometheus: query is required and must not exceed 16384 bytes"))
	}
	params.Set("query", promql)
	params.Set("timeout", "10s")
	path := "/query"
	if isRange {
		if err := rangeParams(params, step); err != nil {
			return mcp.ErrResult(err)
		}
		path = "/query_range"
	} else if at != "" {
		value, err := timestamp(at, time.Now())
		if err != nil {
			return mcp.ErrResult(err)
		}
		params.Set("time", strconv.FormatFloat(value, 'f', -1, 64))
	}
	result, err := p.get(ctx, path, params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid expression result"))
	}
	kind, _ := data["resultType"].(string)
	if kind == "scalar" || kind == "string" {
		pair, ok := data["result"].([]any)
		if !ok || len(pair) != 2 {
			return mcp.ErrResult(fmt.Errorf("prometheus: invalid scalar result"))
		}
		result.Data = map[string]any{"resultType": kind, "result": pair}
		return mcp.JSONResult(result)
	}
	if kind != "matrix" && kind != "vector" {
		return mcp.ErrResult(fmt.Errorf("prometheus: unsupported expression result type"))
	}
	series, ok := data["result"].([]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid series result"))
	}
	if len(series) > limit {
		result.Truncated = true
		series = series[:limit]
	}
	totalSamples := 0
	output := make([]any, 0, len(series))
	for _, raw := range series {
		record, ok := raw.(map[string]any)
		if !ok {
			return mcp.ErrResult(fmt.Errorf("prometheus: invalid series record"))
		}
		projected := pick(record, "metric", "value", "histogram", "histograms")
		if values, ok := record["values"].([]any); ok {
			totalSamples += len(values)
			if totalSamples > 5000 {
				return mcp.ErrResult(fmt.Errorf("prometheus: response exceeds 5000 sample budget; narrow query"))
			}
			times, samples := make([]any, 0, len(values)), make([]any, 0, len(values))
			for _, rawPair := range values {
				pair, ok := rawPair.([]any)
				if !ok || len(pair) != 2 {
					return mcp.ErrResult(fmt.Errorf("prometheus: invalid sample pair"))
				}
				times, samples = append(times, pair[0]), append(samples, pair[1])
			}
			projected["timestamps"], projected["samples"] = times, samples
		}
		if _, ok := record["value"]; ok {
			totalSamples++
		}
		if _, ok := record["histogram"]; ok {
			totalSamples++
		}
		if histograms, ok := record["histograms"].([]any); ok {
			totalSamples += len(histograms)
		}
		if totalSamples > 5000 {
			return mcp.ErrResult(fmt.Errorf("prometheus: response exceeds 5000 sample budget; narrow query"))
		}
		output = append(output, projected)
	}
	result.Data = map[string]any{"resultType": kind, "result": output}
	return mcp.JSONResult(result)
}

func pick(record map[string]any, keys ...string) map[string]any {
	output := map[string]any{}
	for _, key := range keys {
		if value, ok := record[key]; ok {
			output[key] = value
		}
	}
	return output
}

func boundedRecords(raw any, limit int, keys []string) ([]any, bool, error) {
	records, ok := raw.([]any)
	if !ok {
		return nil, false, fmt.Errorf("prometheus: invalid records response")
	}
	truncated := len(records) > limit
	if truncated {
		records = records[:limit]
	}
	output := make([]any, 0, len(records))
	for _, raw := range records {
		record, ok := raw.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("prometheus: invalid record")
		}
		output = append(output, pick(record, keys...))
	}
	return output, truncated, nil
}

func listAlerts(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	_, limit, err := readParams(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	reader := mcp.NewArgs(args)
	state := reader.Str("state")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if state != "" && state != "firing" && state != "pending" {
		return mcp.ErrResult(fmt.Errorf("prometheus: state must be firing or pending"))
	}
	result, err := p.get(ctx, "/alerts", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid alerts response"))
	}
	alerts, ok := data["alerts"].([]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid alerts list"))
	}
	filtered := make([]any, 0, len(alerts))
	for _, raw := range alerts {
		record, ok := raw.(map[string]any)
		if !ok {
			return mcp.ErrResult(fmt.Errorf("prometheus: invalid alert"))
		}
		if state == "" || record["state"] == state {
			filtered = append(filtered, record)
		}
	}
	output, truncated, err := boundedRecords(filtered, limit, []string{"labels", "annotations", "state", "activeAt", "value"})
	if err != nil {
		return mcp.ErrResult(err)
	}
	result.Data, result.Truncated = map[string]any{"alerts": output, "matching": len(filtered)}, truncated
	return mcp.JSONResult(result)
}

func listRules(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	_, limit, err := readParams(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	reader := mcp.NewArgs(args)
	kind, group, token := reader.Str("type"), reader.Str("group"), reader.Str("group_next_token")
	names := reader.StrSlice("rule_name")
	groupLimit := reader.Int("group_limit")
	if _, present := args["group_limit"]; !present {
		groupLimit = 20
	}
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if kind != "" && kind != "alert" && kind != "record" {
		return mcp.ErrResult(fmt.Errorf("prometheus: type must be alert or record"))
	}
	if groupLimit < 1 || groupLimit > 100 {
		return mcp.ErrResult(fmt.Errorf("prometheus: group_limit must be 1..100"))
	}
	params := url.Values{"group_limit": []string{strconv.Itoa(groupLimit)}, "exclude_alerts": []string{"true"}}
	if kind != "" {
		params.Set("type", kind)
	}
	if group != "" {
		params.Add("rule_group[]", group)
	}
	if token != "" {
		params.Set("group_next_token", token)
	}
	for _, name := range names {
		params.Add("rule_name[]", name)
	}
	result, err := p.get(ctx, "/rules", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid rules response"))
	}
	groups, truncated, err := boundedRecords(data["groups"], limit, []string{"name", "interval", "rules"})
	if err != nil {
		return mcp.ErrResult(err)
	}
	for _, raw := range groups {
		group := raw.(map[string]any)
		rules, cut, err := boundedRecords(group["rules"], limit, []string{"name", "query", "type", "health", "lastError", "duration", "labels", "annotations", "state", "evaluationTime", "lastEvaluation"})
		if err != nil {
			return mcp.ErrResult(err)
		}
		group["rules"] = rules
		truncated = truncated || cut
	}
	result.Data = map[string]any{"groups": groups}
	if token, ok := data["groupNextToken"]; ok {
		result.Data.(map[string]any)["groupNextToken"] = token
	}
	result.Truncated = truncated
	return mcp.JSONResult(result)
}

func listTargets(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	_, limit, err := readParams(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	reader := mcp.NewArgs(args)
	state, pool := reader.Str("state"), reader.Str("scrape_pool")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if state == "" {
		state = "active"
	}
	if state != "active" && state != "dropped" && state != "any" {
		return mcp.ErrResult(fmt.Errorf("prometheus: state must be active, dropped or any"))
	}
	params := url.Values{"state": []string{state}}
	if pool != "" {
		params.Set("scrapePool", pool)
	}
	result, err := p.get(ctx, "/targets", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid targets response"))
	}
	output := map[string]any{}
	for _, key := range []string{"activeTargets", "droppedTargets"} {
		if raw, ok := data[key]; ok {
			records, cut, err := boundedRecords(raw, limit, []string{"labels", "scrapePool", "health", "lastError", "lastScrape", "lastScrapeDuration", "scrapeInterval", "scrapeTimeout"})
			if err != nil {
				return mcp.ErrResult(err)
			}
			output[key] = records
			result.Truncated = result.Truncated || cut
		}
	}
	result.Data = output
	return mcp.JSONResult(result)
}

func labelValues(ctx context.Context, p *prometheus, args map[string]any, label string) (*envelope, error) {
	params, limit, err := readParams(args)
	if err != nil {
		return nil, err
	}
	if !labelPattern.MatchString(label) || len(label) > 256 {
		return nil, fmt.Errorf("prometheus: label must be a valid ASCII label name")
	}
	result, err := p.get(ctx, "/label/"+label+"/values", params)
	if err != nil {
		return nil, err
	}
	values, ok := result.Data.([]any)
	if !ok {
		return nil, fmt.Errorf("prometheus: invalid label values response")
	}
	if len(values) > limit {
		values = values[:limit]
		result.Truncated = true
	}
	for _, value := range values {
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("prometheus: invalid label value")
		}
	}
	result.Data = values
	return result, nil
}

func listLabelValues(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	label := reader.Str("label")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	result, err := labelValues(ctx, p, args, label)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(result)
}

func listSeries(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	params, limit, err := readParams(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(params["match[]"]) == 0 {
		return mcp.ErrResult(fmt.Errorf("prometheus: at least one match selector is required"))
	}
	result, err := p.get(ctx, "/series", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	records, ok := result.Data.([]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid series response"))
	}
	if len(records) > limit {
		result.Truncated = true
		records = records[:limit]
	}
	result.Data = records
	return mcp.JSONResult(result)
}

func searchMetrics(ctx context.Context, p *prometheus, args map[string]any) (*mcp.ToolResult, error) {
	result, err := labelValues(ctx, p, args, "__name__")
	if err != nil {
		return mcp.ErrResult(err)
	}
	_, limit, err := readParams(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	metadata, err := p.get(ctx, "/metadata", url.Values{"limit": []string{strconv.Itoa(limit)}, "limit_per_metric": []string{"1"}})
	if err != nil {
		return mcp.ErrResult(err)
	}
	records, ok := metadata.Data.(map[string]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid metadata response"))
	}
	output := make([]any, 0)
	for _, raw := range result.Data.([]any) {
		name := raw.(string)
		record := map[string]any{"name": name}
		if entries, ok := records[name].([]any); ok && len(entries) > 0 {
			if entry, ok := entries[0].(map[string]any); ok {
				for key, value := range pick(entry, "type", "help", "unit") {
					record[key] = value
				}
			}
		}
		output = append(output, record)
	}
	result.Data = map[string]any{"metrics": output, "metadata_complete": false}
	result.Warnings = append(result.Warnings, metadata.Warnings...)
	result.Infos = append(result.Infos, metadata.Infos...)
	return mcp.JSONResult(result)
}

func serverInfo(ctx context.Context, p *prometheus, _ map[string]any) (*mcp.ToolResult, error) {
	build, err := p.get(ctx, "/status/buildinfo", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	runtime, err := p.get(ctx, "/status/runtimeinfo", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	buildData, ok := build.Data.(map[string]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid build info"))
	}
	runtimeData, ok := runtime.Data.(map[string]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("prometheus: invalid runtime info"))
	}
	build.Data = map[string]any{"build": pick(buildData, "version", "revision", "branch", "buildDate", "goVersion"), "runtime": pick(runtimeData, "startTime", "serverTime", "reloadConfigSuccess", "lastConfigTime", "timeSeriesCount", "corruptionCount", "goroutineCount", "storageRetention")}
	build.Warnings = append(build.Warnings, runtime.Warnings...)
	build.Infos = append(build.Infos, runtime.Infos...)
	return mcp.JSONResult(build)
}
