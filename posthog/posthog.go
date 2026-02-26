package posthog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type posthog struct {
	apiKey    string
	projectID string
	client    *http.Client
	baseURL   string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &posthog{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://us.posthog.com",
	}
}

func (p *posthog) Name() string { return "posthog" }

func (p *posthog) Configure(creds mcp.Credentials) error {
	p.apiKey = creds["api_key"]
	p.projectID = creds["project_id"]
	if p.apiKey == "" {
		return fmt.Errorf("posthog: api_key is required")
	}
	if p.projectID == "" {
		return fmt.Errorf("posthog: project_id is required")
	}
	if v := creds["base_url"]; v != "" {
		p.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (p *posthog) Healthy(ctx context.Context) bool {
	_, err := p.get(ctx, "/api/projects/%s/", p.projectID)
	return err == nil
}

func (p *posthog) Tools() []mcp.ToolDefinition {
	return tools
}

func (p *posthog) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, p, args)
}

// --- HTTP helpers ---

func (p *posthog) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("posthog API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (p *posthog) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return p.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (p *posthog) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return p.doRequest(ctx, "POST", path, body)
}

func (p *posthog) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return p.doRequest(ctx, "PATCH", path, body)
}

func (p *posthog) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return p.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

// parseJSON unmarshals a JSON string arg, returning an error result if invalid.
func parseJSON(args map[string]any, key string) (any, error) {
	v := argStr(args, key)
	if v == "" {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON for %s: %w", key, err)
	}
	return out, nil
}

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

// proj returns the project ID from args, falling back to the configured default.
func (p *posthog) proj(args map[string]any) string {
	if v := argStr(args, "project_id"); v != "" {
		return v
	}
	return p.projectID
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Projects
	"posthog_list_projects":  listProjects,
	"posthog_get_project":    getProject,
	"posthog_update_project": updateProject,
	"posthog_create_project": createProject,
	"posthog_delete_project": deleteProject,

	// Feature Flags
	"posthog_list_feature_flags":    listFeatureFlags,
	"posthog_get_feature_flag":      getFeatureFlag,
	"posthog_create_feature_flag":   createFeatureFlag,
	"posthog_update_feature_flag":   updateFeatureFlag,
	"posthog_delete_feature_flag":   deleteFeatureFlag,
	"posthog_feature_flag_activity": featureFlagActivity,

	// Cohorts
	"posthog_list_cohorts":        listCohorts,
	"posthog_get_cohort":          getCohort,
	"posthog_create_cohort":       createCohort,
	"posthog_update_cohort":       updateCohort,
	"posthog_delete_cohort":       deleteCohort,
	"posthog_list_cohort_persons": listCohortPersons,

	// Insights
	"posthog_list_insights":  listInsights,
	"posthog_get_insight":    getInsight,
	"posthog_create_insight": createInsight,
	"posthog_update_insight": updateInsight,
	"posthog_delete_insight": deleteInsight,

	// Persons
	"posthog_list_persons":          listPersons,
	"posthog_get_person":            getPerson,
	"posthog_delete_person":         deletePerson,
	"posthog_update_person_property": updatePersonProperty,
	"posthog_delete_person_property": deletePersonProperty,

	// Groups
	"posthog_list_groups":   listGroups,
	"posthog_find_group":    findGroup,

	// Annotations
	"posthog_list_annotations":  listAnnotations,
	"posthog_get_annotation":    getAnnotation,
	"posthog_create_annotation": createAnnotation,
	"posthog_update_annotation": updateAnnotation,
	"posthog_delete_annotation": deleteAnnotation,

	// Dashboards
	"posthog_list_dashboards":  listDashboards,
	"posthog_get_dashboard":    getDashboard,
	"posthog_create_dashboard": createDashboard,
	"posthog_update_dashboard": updateDashboard,
	"posthog_delete_dashboard": deleteDashboard,

	// Actions
	"posthog_list_actions":  listActions,
	"posthog_get_action":    getAction,
	"posthog_create_action": createAction,
	"posthog_update_action": updateAction,
	"posthog_delete_action": deleteAction,

	// Events
	"posthog_list_events": listEvents,
	"posthog_get_event":   getEvent,

	// Experiments
	"posthog_list_experiments":  listExperiments,
	"posthog_get_experiment":    getExperiment,
	"posthog_create_experiment": createExperiment,
	"posthog_update_experiment": updateExperiment,
	"posthog_delete_experiment": deleteExperiment,

	// Surveys
	"posthog_list_surveys":  listSurveys,
	"posthog_get_survey":    getSurvey,
	"posthog_create_survey": createSurvey,
	"posthog_update_survey": updateSurvey,
	"posthog_delete_survey": deleteSurvey,
}
