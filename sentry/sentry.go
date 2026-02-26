package sentry

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

	mcp "github.com/daltoniam/switchboard"
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

func (s *sentry) Configure(creds mcp.Credentials) error {
	s.authToken = creds["auth_token"]
	s.organization = creds["organization"]
	if s.authToken == "" {
		return fmt.Errorf("sentry: auth_token is required")
	}
	if s.organization == "" {
		return fmt.Errorf("sentry: organization is required")
	}
	if v := creds["base_url"]; v != "" {
		s.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (s *sentry) Healthy(ctx context.Context) bool {
	_, err := s.get(ctx, "/organizations/%s/", s.organization)
	return err == nil
}

func (s *sentry) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *sentry) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
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
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
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

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errResult(err)
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

// --- Argument helpers ---

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

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return nil
}

func optInt(args map[string]any, key string, def int) int {
	if v := argInt(args, key); v > 0 {
		return v
	}
	return def
}

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
	if v := argStr(args, "organization"); v != "" {
		return v
	}
	return s.organization
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Organizations
	"sentry_get_organization":    getOrganization,
	"sentry_list_org_projects":   listOrgProjects,
	"sentry_list_org_teams":      listOrgTeams,
	"sentry_list_org_members":    listOrgMembers,
	"sentry_get_org_member":      getOrgMember,
	"sentry_list_org_repos":      listOrgRepos,
	"sentry_resolve_short_id":    resolveShortID,

	// Projects
	"sentry_list_projects":        listProjects,
	"sentry_get_project":          getProject,
	"sentry_update_project":       updateProject,
	"sentry_delete_project":       deleteProject,
	"sentry_create_project":       createProject,
	"sentry_list_project_keys":    listProjectKeys,
	"sentry_list_project_envs":    listProjectEnvironments,
	"sentry_list_project_tags":    listProjectTags,
	"sentry_get_project_stats":    getProjectStats,
	"sentry_list_project_hooks":   listProjectHooks,

	// Teams
	"sentry_get_team":             getTeam,
	"sentry_create_team":          createTeam,
	"sentry_delete_team":          deleteTeam,
	"sentry_list_team_projects":   listTeamProjects,

	// Issues & Events
	"sentry_list_issues":          listIssues,
	"sentry_get_issue":            getIssue,
	"sentry_update_issue":         updateIssue,
	"sentry_delete_issue":         deleteIssue,
	"sentry_list_issue_events":    listIssueEvents,
	"sentry_list_issue_hashes":    listIssueHashes,
	"sentry_get_issue_tag_values": getIssueTagValues,
	"sentry_list_project_events":  listProjectEvents,
	"sentry_get_event":            getEvent,
	"sentry_list_org_issues":      listOrgIssues,

	// Releases
	"sentry_list_releases":        listReleases,
	"sentry_get_release":          getRelease,
	"sentry_create_release":       createRelease,
	"sentry_delete_release":       deleteRelease,
	"sentry_list_release_commits": listReleaseCommits,
	"sentry_list_release_deploys": listReleaseDeploys,
	"sentry_create_deploy":        createDeploy,
	"sentry_list_release_files":   listReleaseFiles,

	// Alerts
	"sentry_list_metric_alerts":       listMetricAlerts,
	"sentry_get_metric_alert":         getMetricAlert,
	"sentry_delete_metric_alert":      deleteMetricAlert,
	"sentry_list_issue_alerts":        listIssueAlerts,
	"sentry_get_issue_alert":          getIssueAlert,
	"sentry_delete_issue_alert":       deleteIssueAlert,

	// Monitors (Cron)
	"sentry_list_monitors":         listMonitors,
	"sentry_get_monitor":           getMonitor,
	"sentry_delete_monitor":        deleteMonitor,

	// Discover
	"sentry_list_saved_queries":  listSavedQueries,
	"sentry_get_saved_query":     getSavedQuery,
	"sentry_delete_saved_query":  deleteSavedQuery,

	// Replays
	"sentry_list_replays":  listReplays,
	"sentry_get_replay":    getReplay,
	"sentry_delete_replay": deleteReplay,
}
