package posthog

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// -- Annotations --

func listAnnotations(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"search": r.Str("search"),
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/annotations/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	annotationID, err := mcp.ArgStr(args, "annotation_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/annotations/%s/", p.proj(args), annotationID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	content := r.Str("content")
	dateMarker := r.Str("date_marker")
	scope := r.Str("scope")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"content":     content,
		"date_marker": dateMarker,
	}
	if scope != "" {
		body["scope"] = scope
	}
	path := fmt.Sprintf("/api/projects/%s/annotations/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	annotationID := r.Str("annotation_id")
	content := r.Str("content")
	dateMarker := r.Str("date_marker")
	scope := r.Str("scope")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if content != "" {
		body["content"] = content
	}
	if dateMarker != "" {
		body["date_marker"] = dateMarker
	}
	if scope != "" {
		body["scope"] = scope
	}
	path := fmt.Sprintf("/api/projects/%s/annotations/%s/", p.proj(args), annotationID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAnnotation(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	annotationID, err := mcp.ArgStr(args, "annotation_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.del(ctx, "/api/projects/%s/annotations/%s/", p.proj(args), annotationID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// -- Dashboards --

func listDashboards(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/dashboards/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	dashboardID, err := mcp.ArgStr(args, "dashboard_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/dashboards/%s/", p.proj(args), dashboardID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	pinned := r.Bool("pinned")
	tags := r.Str("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"name": name}
	if description != "" {
		body["description"] = description
	}
	if _, ok := args["pinned"]; ok {
		body["pinned"] = pinned
	}
	if tags != "" {
		body["tags"] = strings.Split(tags, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/dashboards/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	dashboardID := r.Str("dashboard_id")
	name := r.Str("name")
	description := r.Str("description")
	pinned := r.Bool("pinned")
	tags := r.Str("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if _, ok := args["pinned"]; ok {
		body["pinned"] = pinned
	}
	if tags != "" {
		body["tags"] = strings.Split(tags, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/dashboards/%s/", p.proj(args), dashboardID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDashboard(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	dashboardID, err := mcp.ArgStr(args, "dashboard_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/dashboards/%s/", p.proj(args), dashboardID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// -- Actions --

func listActions(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/actions/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	actionID, err := mcp.ArgStr(args, "action_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/actions/%s/", p.proj(args), actionID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	tags := r.Str("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"name": name}
	if description != "" {
		body["description"] = description
	}
	if steps, err := parseJSON(args, "steps"); err != nil {
		return mcp.ErrResult(err)
	} else if steps != nil {
		body["steps"] = steps
	}
	if tags != "" {
		body["tags"] = strings.Split(tags, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/actions/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	actionID := r.Str("action_id")
	name := r.Str("name")
	description := r.Str("description")
	tags := r.Str("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if steps, err := parseJSON(args, "steps"); err != nil {
		return mcp.ErrResult(err)
	} else if steps != nil {
		body["steps"] = steps
	}
	if tags != "" {
		body["tags"] = strings.Split(tags, ",")
	}
	path := fmt.Sprintf("/api/projects/%s/actions/%s/", p.proj(args), actionID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAction(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	actionID, err := mcp.ArgStr(args, "action_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/actions/%s/", p.proj(args), actionID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// -- Events --

func listEvents(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"event":       r.Str("event"),
		"person_id":   r.Str("person_id"),
		"distinct_id": r.Str("distinct_id"),
		"properties":  r.Str("properties"),
		"before":      r.Str("before"),
		"after":       r.Str("after"),
		"limit":       r.Str("limit"),
		"offset":      r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/events/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getEvent(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	eventID, err := mcp.ArgStr(args, "event_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/events/%s/", p.proj(args), eventID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// -- Experiments --

func listExperiments(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/experiments/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	experimentID, err := mcp.ArgStr(args, "experiment_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/experiments/%s/", p.proj(args), experimentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	featureFlagKey := r.Str("feature_flag_key")
	description := r.Str("description")
	startDate := r.Str("start_date")
	endDate := r.Str("end_date")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"name":             name,
		"feature_flag_key": featureFlagKey,
	}
	if description != "" {
		body["description"] = description
	}
	if startDate != "" {
		body["start_date"] = startDate
	}
	if endDate != "" {
		body["end_date"] = endDate
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	path := fmt.Sprintf("/api/projects/%s/experiments/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	experimentID := r.Str("experiment_id")
	name := r.Str("name")
	description := r.Str("description")
	startDate := r.Str("start_date")
	endDate := r.Str("end_date")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if startDate != "" {
		body["start_date"] = startDate
	}
	if endDate != "" {
		body["end_date"] = endDate
	}
	path := fmt.Sprintf("/api/projects/%s/experiments/%s/", p.proj(args), experimentID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteExperiment(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	experimentID, err := mcp.ArgStr(args, "experiment_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/experiments/%s/", p.proj(args), experimentID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// -- Surveys --

func listSurveys(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/surveys/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	surveyID, err := mcp.ArgStr(args, "survey_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/surveys/%s/", p.proj(args), surveyID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	surveyType := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"name": name}
	if description != "" {
		body["description"] = description
	}
	if surveyType != "" {
		body["type"] = surveyType
	}
	if questions, err := parseJSON(args, "questions"); err != nil {
		return mcp.ErrResult(err)
	} else if questions != nil {
		body["questions"] = questions
	}
	if filters, err := parseJSON(args, "targeting_flag_filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["targeting_flag_filters"] = filters
	}
	path := fmt.Sprintf("/api/projects/%s/surveys/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	surveyID := r.Str("survey_id")
	name := r.Str("name")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if questions, err := parseJSON(args, "questions"); err != nil {
		return mcp.ErrResult(err)
	} else if questions != nil {
		body["questions"] = questions
	}
	path := fmt.Sprintf("/api/projects/%s/surveys/%s/", p.proj(args), surveyID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteSurvey(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	surveyID, err := mcp.ArgStr(args, "survey_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/surveys/%s/", p.proj(args), surveyID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
