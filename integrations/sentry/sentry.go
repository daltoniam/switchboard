package sentry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*sentry)(nil)
	_ mcp.FieldCompactionIntegration = (*sentry)(nil)
)

type sentry struct {
	authToken    string
	organization string
	client       *http.Client
	baseURL      string
}

func New() mcp.Integration {
	return &sentry{
		client:  &http.Client{},
		baseURL: "https://sentry.io/api/0",
	}
}

func (s *sentry) Name() string { return "sentry" }

func (s *sentry) Configure(_ context.Context, creds mcp.Credentials) error {
	s.authToken = creds["auth_token"]
	s.organization = creds["organization"]
	if s.authToken == "" {
		return fmt.Errorf("sentry: auth_token is required")
	}
	if v := creds["base_url"]; v != "" {
		s.baseURL = strings.TrimRight(v, "/")
	}
	if s.organization == "" {
		org, err := s.fetchOrganization()
		if err != nil {
			return fmt.Errorf("sentry: organization is required (auto-detect failed: %v)", err)
		}
		s.organization = org
	}
	return nil
}

// fetchOrganization calls GET /organizations/ to auto-detect the user's organization slug.
func (s *sentry) fetchOrganization() (string, error) {
	req, err := http.NewRequest("GET", s.baseURL+"/organizations/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	var orgs []struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(data, &orgs); err != nil {
		return "", fmt.Errorf("parse organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations found for this token")
	}
	return orgs[0].Slug, nil
}

func (s *sentry) Healthy(ctx context.Context) bool {
	_, err := s.get(ctx, "/organizations/%s/", s.organization)
	return err == nil
}

func (s *sentry) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *sentry) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *sentry) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

// --- HTTP helpers ---

func (s *sentry) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.authToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("sentry API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("sentry API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (s *sentry) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (s *sentry) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "POST", path, body)
}

func (s *sentry) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "PUT", path, body)
}

func (s *sentry) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error)

// queryEncode builds a query string from non-empty key/value pairs.
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

// org returns the organization slug from args, falling back to the configured default.
func (s *sentry) org(args map[string]any) string {
	v, _ := mcp.ArgStr(args, "organization")
	if v != "" {
		return v
	}
	return s.organization
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Organizations
	mcp.ToolName("sentry_get_organization"):  getOrganization,
	mcp.ToolName("sentry_list_org_projects"): listOrgProjects,
	mcp.ToolName("sentry_list_org_teams"):    listOrgTeams,
	mcp.ToolName("sentry_list_org_members"):  listOrgMembers,
	mcp.ToolName("sentry_get_org_member"):    getOrgMember,
	mcp.ToolName("sentry_list_org_repos"):    listOrgRepos,
	mcp.ToolName("sentry_resolve_short_id"):  resolveShortID,

	// Projects
	mcp.ToolName("sentry_list_projects"):      listProjects,
	mcp.ToolName("sentry_get_project"):        getProject,
	mcp.ToolName("sentry_update_project"):     updateProject,
	mcp.ToolName("sentry_delete_project"):     deleteProject,
	mcp.ToolName("sentry_create_project"):     createProject,
	mcp.ToolName("sentry_list_project_keys"):  listProjectKeys,
	mcp.ToolName("sentry_list_project_envs"):  listProjectEnvironments,
	mcp.ToolName("sentry_list_project_tags"):  listProjectTags,
	mcp.ToolName("sentry_get_project_stats"):  getProjectStats,
	mcp.ToolName("sentry_list_project_hooks"): listProjectHooks,

	// Teams
	mcp.ToolName("sentry_get_team"):           getTeam,
	mcp.ToolName("sentry_create_team"):        createTeam,
	mcp.ToolName("sentry_delete_team"):        deleteTeam,
	mcp.ToolName("sentry_list_team_projects"): listTeamProjects,

	// Issues & Events
	mcp.ToolName("sentry_list_issues"):          listIssues,
	mcp.ToolName("sentry_get_issue"):            getIssue,
	mcp.ToolName("sentry_update_issue"):         updateIssue,
	mcp.ToolName("sentry_delete_issue"):         deleteIssue,
	mcp.ToolName("sentry_list_issue_events"):    listIssueEvents,
	mcp.ToolName("sentry_list_issue_hashes"):    listIssueHashes,
	mcp.ToolName("sentry_get_issue_tag_values"): getIssueTagValues,
	mcp.ToolName("sentry_list_project_events"):  listProjectEvents,
	mcp.ToolName("sentry_get_event"):            getEvent,
	mcp.ToolName("sentry_list_org_issues"):      listOrgIssues,

	// Releases
	mcp.ToolName("sentry_list_releases"):        listReleases,
	mcp.ToolName("sentry_get_release"):          getRelease,
	mcp.ToolName("sentry_create_release"):       createRelease,
	mcp.ToolName("sentry_delete_release"):       deleteRelease,
	mcp.ToolName("sentry_list_release_commits"): listReleaseCommits,
	mcp.ToolName("sentry_list_release_deploys"): listReleaseDeploys,
	mcp.ToolName("sentry_create_deploy"):        createDeploy,
	mcp.ToolName("sentry_list_release_files"):   listReleaseFiles,

	// Alerts
	mcp.ToolName("sentry_list_metric_alerts"):  listMetricAlerts,
	mcp.ToolName("sentry_get_metric_alert"):    getMetricAlert,
	mcp.ToolName("sentry_delete_metric_alert"): deleteMetricAlert,
	mcp.ToolName("sentry_list_issue_alerts"):   listIssueAlerts,
	mcp.ToolName("sentry_get_issue_alert"):     getIssueAlert,
	mcp.ToolName("sentry_delete_issue_alert"):  deleteIssueAlert,

	// Monitors (Cron)
	mcp.ToolName("sentry_list_monitors"):  listMonitors,
	mcp.ToolName("sentry_get_monitor"):    getMonitor,
	mcp.ToolName("sentry_delete_monitor"): deleteMonitor,

	// Discover
	mcp.ToolName("sentry_list_saved_queries"): listSavedQueries,
	mcp.ToolName("sentry_get_saved_query"):    getSavedQuery,
	mcp.ToolName("sentry_delete_saved_query"): deleteSavedQuery,

	// Replays
	mcp.ToolName("sentry_list_replays"):  listReplays,
	mcp.ToolName("sentry_get_replay"):    getReplay,
	mcp.ToolName("sentry_delete_replay"): deleteReplay,
}
