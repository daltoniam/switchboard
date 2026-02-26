package posthog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// ── Annotations ─────────────────────────────────────────────────────

func listAnnotations(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"search": argStr(args, "search"),
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/annotations/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/annotations/%s/", p.proj(args), argStr(args, "annotation_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"content":     argStr(args, "content"),
		"date_marker": argStr(args, "date_marker"),
	}
	if v := argStr(args, "scope"); v != "" {
		body["scope"] = v
	}
	path := fmt.Sprintf("/api/projects/%s/annotations/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "content"); v != "" {
		body["content"] = v
	}
	if v := argStr(args, "date_marker"); v != "" {
		body["date_marker"] = v
	}
	if v := argStr(args, "scope"); v != "" {
		body["scope"] = v
	}
	path := fmt.Sprintf("/api/projects/%s/annotations/%s/", p.proj(args), argStr(args, "annotation_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.del(ctx, "/api/projects/%s/annotations/%s/", p.proj(args), argStr(args, "annotation_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Dashboards ──────────────────────────────────────────────────────

func listDashboards(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/dashboards/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/dashboards/%s/", p.proj(args), argStr(args, "dashboard_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"name": argStr(args, "name")}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if _, ok := args["pinned"]; ok {
		body["pinned"] = argBool(args, "pinned")
	}
	if v := argStr(args, "tags"); v != "" {
		body["tags"] = strings.Split(v, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/dashboards/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if _, ok := args["pinned"]; ok {
		body["pinned"] = argBool(args, "pinned")
	}
	if v := argStr(args, "tags"); v != "" {
		body["tags"] = strings.Split(v, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/dashboards/%s/", p.proj(args), argStr(args, "dashboard_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/api/projects/%s/dashboards/%s/", p.proj(args), argStr(args, "dashboard_id"))
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Actions ─────────────────────────────────────────────────────────

func listActions(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/actions/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/actions/%s/", p.proj(args), argStr(args, "action_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"name": argStr(args, "name")}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "steps"); v != "" {
		var steps any
		if err := json.Unmarshal([]byte(v), &steps); err == nil {
			body["steps"] = steps
		}
	}
	if v := argStr(args, "tags"); v != "" {
		body["tags"] = strings.Split(v, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/actions/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "steps"); v != "" {
		var steps any
		if err := json.Unmarshal([]byte(v), &steps); err == nil {
			body["steps"] = steps
		}
	}
	if v := argStr(args, "tags"); v != "" {
		body["tags"] = strings.Split(v, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/actions/%s/", p.proj(args), argStr(args, "action_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/api/projects/%s/actions/%s/", p.proj(args), argStr(args, "action_id"))
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Events ──────────────────────────────────────────────────────────

func listEvents(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"event":       argStr(args, "event"),
		"person_id":   argStr(args, "person_id"),
		"distinct_id": argStr(args, "distinct_id"),
		"properties":  argStr(args, "properties"),
		"before":      argStr(args, "before"),
		"after":       argStr(args, "after"),
		"limit":       argStr(args, "limit"),
		"offset":      argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/events/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getEvent(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/events/%s/", p.proj(args), argStr(args, "event_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Experiments ─────────────────────────────────────────────────────

func listExperiments(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/experiments/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/experiments/%s/", p.proj(args), argStr(args, "experiment_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"name":             argStr(args, "name"),
		"feature_flag_key": argStr(args, "feature_flag_key"),
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "start_date"); v != "" {
		body["start_date"] = v
	}
	if v := argStr(args, "end_date"); v != "" {
		body["end_date"] = v
	}
	if v := argStr(args, "filters"); v != "" {
		var filters any
		if err := json.Unmarshal([]byte(v), &filters); err == nil {
			body["filters"] = filters
		}
	}
	path := fmt.Sprintf("/api/projects/%s/experiments/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "start_date"); v != "" {
		body["start_date"] = v
	}
	if v := argStr(args, "end_date"); v != "" {
		body["end_date"] = v
	}
	path := fmt.Sprintf("/api/projects/%s/experiments/%s/", p.proj(args), argStr(args, "experiment_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/api/projects/%s/experiments/%s/", p.proj(args), argStr(args, "experiment_id"))
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Surveys ─────────────────────────────────────────────────────────

func listSurveys(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/surveys/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/surveys/%s/", p.proj(args), argStr(args, "survey_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"name": argStr(args, "name")}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "type"); v != "" {
		body["type"] = v
	}
	if v := argStr(args, "questions"); v != "" {
		var questions any
		if err := json.Unmarshal([]byte(v), &questions); err == nil {
			body["questions"] = questions
		}
	}
	if v := argStr(args, "targeting_flag_filters"); v != "" {
		var filters any
		if err := json.Unmarshal([]byte(v), &filters); err == nil {
			body["targeting_flag_filters"] = filters
		}
	}
	path := fmt.Sprintf("/api/projects/%s/surveys/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "questions"); v != "" {
		var questions any
		if err := json.Unmarshal([]byte(v), &questions); err == nil {
			body["questions"] = questions
		}
	}
	path := fmt.Sprintf("/api/projects/%s/surveys/%s/", p.proj(args), argStr(args, "survey_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/api/projects/%s/surveys/%s/", p.proj(args), argStr(args, "survey_id"))
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
