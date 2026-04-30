package posthog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*posthog)(nil)
	_ mcp.FieldCompactionIntegration = (*posthog)(nil)
	_ mcp.PlainTextCredentials       = (*posthog)(nil)
	_ mcp.PlaceholderHints           = (*posthog)(nil)
	_ mcp.OptionalCredentials        = (*posthog)(nil)
)

func (p *posthog) PlainTextKeys() []string { return []string{"project_id", "base_url"} }

func (p *posthog) Placeholders() map[string]string {
	return map[string]string{
		"project_id": "Default project ID (leave blank to specify per-request)",
		"base_url":   "https://us.posthog.com (default) or https://eu.posthog.com",
	}
}

func (p *posthog) OptionalKeys() []string { return []string{"project_id", "base_url"} }

func New() mcp.Integration {
	return &posthog{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://us.posthog.com",
	}
}

func (p *posthog) Name() string { return "posthog" }

func (p *posthog) Configure(_ context.Context, creds mcp.Credentials) error {
	p.apiKey = creds["api_key"]
	p.projectID = creds["project_id"]
	if p.apiKey == "" {
		return fmt.Errorf("posthog: api_key is required")
	}
	if v := creds["base_url"]; v != "" {
		p.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (p *posthog) Healthy(ctx context.Context) bool {
	if p.projectID != "" {
		_, err := p.get(ctx, "/api/projects/%s/", p.projectID)
		return err == nil
	}
	_, err := p.get(ctx, "/api/projects/")
	return err == nil
}

func (p *posthog) Tools() []mcp.ToolDefinition {
	return tools
}

func (p *posthog) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (p *posthog) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("posthog API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
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

// --- Argument helpers ---

// parseJSON unmarshals a JSON string arg, returning an error result if invalid.
func parseJSON(args map[string]any, key string) (any, error) {
	v, err := mcp.ArgStr(args, key)
	if err != nil {
		return nil, err
	}
	if v == "" {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON for %s: %w", key, err)
	}
	return out, nil
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
// Returns an error if no project ID is available from either source.
func (p *posthog) proj(args map[string]any) (string, error) {
	if v, _ := mcp.ArgStr(args, "project_id"); v != "" {
		return v, nil
	}
	if p.projectID != "" {
		return p.projectID, nil
	}
	return "", fmt.Errorf("project_id is required (no default configured)")
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Projects
	mcp.ToolName("posthog_list_projects"):  listProjects,
	mcp.ToolName("posthog_get_project"):    getProject,
	mcp.ToolName("posthog_update_project"): updateProject,
	mcp.ToolName("posthog_create_project"): createProject,
	mcp.ToolName("posthog_delete_project"): deleteProject,

	// Feature Flags
	mcp.ToolName("posthog_list_feature_flags"):    listFeatureFlags,
	mcp.ToolName("posthog_get_feature_flag"):      getFeatureFlag,
	mcp.ToolName("posthog_create_feature_flag"):   createFeatureFlag,
	mcp.ToolName("posthog_update_feature_flag"):   updateFeatureFlag,
	mcp.ToolName("posthog_delete_feature_flag"):   deleteFeatureFlag,
	mcp.ToolName("posthog_feature_flag_activity"): featureFlagActivity,

	// Cohorts
	mcp.ToolName("posthog_list_cohorts"):        listCohorts,
	mcp.ToolName("posthog_get_cohort"):          getCohort,
	mcp.ToolName("posthog_create_cohort"):       createCohort,
	mcp.ToolName("posthog_update_cohort"):       updateCohort,
	mcp.ToolName("posthog_delete_cohort"):       deleteCohort,
	mcp.ToolName("posthog_list_cohort_persons"): listCohortPersons,

	// Insights
	mcp.ToolName("posthog_list_insights"):  listInsights,
	mcp.ToolName("posthog_get_insight"):    getInsight,
	mcp.ToolName("posthog_create_insight"): createInsight,
	mcp.ToolName("posthog_update_insight"): updateInsight,
	mcp.ToolName("posthog_delete_insight"): deleteInsight,
	mcp.ToolName("posthog_query"):          runQuery,

	// Persons
	mcp.ToolName("posthog_list_persons"):           listPersons,
	mcp.ToolName("posthog_get_person"):             getPerson,
	mcp.ToolName("posthog_delete_person"):          deletePerson,
	mcp.ToolName("posthog_update_person_property"): updatePersonProperty,
	mcp.ToolName("posthog_delete_person_property"): deletePersonProperty,

	// Groups
	mcp.ToolName("posthog_list_groups"): listGroups,
	mcp.ToolName("posthog_find_group"):  findGroup,

	// Annotations
	mcp.ToolName("posthog_list_annotations"):  listAnnotations,
	mcp.ToolName("posthog_get_annotation"):    getAnnotation,
	mcp.ToolName("posthog_create_annotation"): createAnnotation,
	mcp.ToolName("posthog_update_annotation"): updateAnnotation,
	mcp.ToolName("posthog_delete_annotation"): deleteAnnotation,

	// Dashboards
	mcp.ToolName("posthog_list_dashboards"):  listDashboards,
	mcp.ToolName("posthog_get_dashboard"):    getDashboard,
	mcp.ToolName("posthog_create_dashboard"): createDashboard,
	mcp.ToolName("posthog_update_dashboard"): updateDashboard,
	mcp.ToolName("posthog_delete_dashboard"): deleteDashboard,

	// Actions
	mcp.ToolName("posthog_list_actions"):  listActions,
	mcp.ToolName("posthog_get_action"):    getAction,
	mcp.ToolName("posthog_create_action"): createAction,
	mcp.ToolName("posthog_update_action"): updateAction,
	mcp.ToolName("posthog_delete_action"): deleteAction,

	// Events
	mcp.ToolName("posthog_list_events"): listEvents,
	mcp.ToolName("posthog_get_event"):   getEvent,

	// Experiments
	mcp.ToolName("posthog_list_experiments"):  listExperiments,
	mcp.ToolName("posthog_get_experiment"):    getExperiment,
	mcp.ToolName("posthog_create_experiment"): createExperiment,
	mcp.ToolName("posthog_update_experiment"): updateExperiment,
	mcp.ToolName("posthog_delete_experiment"): deleteExperiment,

	// Surveys
	mcp.ToolName("posthog_list_surveys"):  listSurveys,
	mcp.ToolName("posthog_get_survey"):    getSurvey,
	mcp.ToolName("posthog_create_survey"): createSurvey,
	mcp.ToolName("posthog_update_survey"): updateSurvey,
	mcp.ToolName("posthog_delete_survey"): deleteSurvey,
}
