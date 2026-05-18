package vercel

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("vercel", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type vercel struct {
	token    string
	teamID   string
	teamSlug string
	client   *http.Client
	baseURL  string
}

var (
	_ mcp.FieldCompactionIntegration = (*vercel)(nil)
	_ mcp.PlainTextCredentials       = (*vercel)(nil)
	_ mcp.PlaceholderHints           = (*vercel)(nil)
	_ mcp.OptionalCredentials        = (*vercel)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*vercel)(nil)
)

func (v *vercel) PlainTextKeys() []string {
	return []string{"team_id", "team_slug", "base_url"}
}

func (v *vercel) Placeholders() map[string]string {
	return map[string]string{
		"api_token": "Vercel personal access token",
		"team_id":   "Default team ID (team_...) for scoped requests",
		"team_slug": "Default team slug if team_id is not set",
		"base_url":  "https://api.vercel.com",
	}
}

func (v *vercel) OptionalKeys() []string {
	return []string{"team_id", "team_slug", "base_url"}
}

const maxResponseSize = 10 * 1024 * 1024

func New() mcp.Integration {
	return &vercel{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.vercel.com",
	}
}

func (v *vercel) Name() string { return "vercel" }

func (v *vercel) Configure(_ context.Context, creds mcp.Credentials) error {
	v.token = creds["api_token"]
	if v.token == "" {
		return fmt.Errorf("vercel: api_token is required")
	}
	v.teamID = creds["team_id"]
	v.teamSlug = creds["team_slug"]
	if val := creds["base_url"]; val != "" {
		v.baseURL = strings.TrimRight(val, "/")
	}
	return nil
}

func (v *vercel) Healthy(ctx context.Context) bool {
	if v.client == nil || v.token == "" {
		return false
	}
	_, err := v.get(ctx, "/v2/teams%s", queryEncode(v.scopedQuery(map[string]string{"limit": "1"})))
	return err == nil
}

func (v *vercel) Tools() []mcp.ToolDefinition {
	return tools
}

func (v *vercel) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, v, args)
}

func (v *vercel) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (v *vercel) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (v *vercel) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, v.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+v.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("vercel API error (%d): %s", resp.StatusCode, vercelErrorMessage(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("vercel API error (%d): %s", resp.StatusCode, vercelErrorMessage(data))
	}
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusAccepted || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (v *vercel) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return v.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (v *vercel) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return v.doRequest(ctx, http.MethodPost, path, body)
}

func (v *vercel) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return v.doRequest(ctx, http.MethodPatch, path, body)
}

func (v *vercel) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return v.doRequest(ctx, http.MethodDelete, fmt.Sprintf(pathFmt, args...), nil)
}

type handlerFunc func(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error)

func (v *vercel) scopedQuery(params map[string]string) map[string]string {
	out := make(map[string]string, len(params)+2)
	for k, val := range params {
		if val != "" {
			out[k] = val
		}
	}
	if out["teamId"] == "" && out["slug"] == "" {
		if v.teamID != "" {
			out["teamId"] = v.teamID
		} else if v.teamSlug != "" {
			out["slug"] = v.teamSlug
		}
	}
	return out
}

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, val := range params {
		if val != "" {
			vals.Set(k, val)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

func vercelErrorMessage(data []byte) string {
	var payload struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err == nil && payload.Error != nil {
		if msg, ok := payload.Error.(map[string]any); ok {
			code, _ := msg["code"].(string)
			message, _ := msg["message"].(string)
			if code != "" && message != "" {
				return code + ": " + message
			}
			if message != "" {
				return message
			}
		}
	}
	return string(data)
}

func required(value, name string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func optionalScopeArgs(r *mcp.Args) (teamID, teamSlug string) {
	return r.Str("team_id"), r.Str("team_slug")
}

func paginationArgs(r *mcp.Args, defaultLimit int) map[string]string {
	params := map[string]string{
		"limit": fmt.Sprintf("%d", r.OptInt("limit", defaultLimit)),
	}
	if next := r.Str("next"); next != "" {
		params["next"] = next
	}
	if until := r.Str("until"); until != "" {
		params["until"] = until
	}
	if since := r.Str("since"); since != "" {
		params["since"] = since
	}
	return params
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("vercel_list_teams"):              listTeams,
	mcp.ToolName("vercel_get_team"):                getTeam,
	mcp.ToolName("vercel_list_team_members"):       listTeamMembers,
	mcp.ToolName("vercel_list_user_events"):        listUserEvents,
	mcp.ToolName("vercel_list_projects"):           listProjects,
	mcp.ToolName("vercel_get_project"):             getProject,
	mcp.ToolName("vercel_create_project"):          createProject,
	mcp.ToolName("vercel_update_project"):          updateProject,
	mcp.ToolName("vercel_delete_project"):          deleteProject,
	mcp.ToolName("vercel_list_deployments"):        listDeployments,
	mcp.ToolName("vercel_get_deployment"):          getDeployment,
	mcp.ToolName("vercel_create_deployment"):       createDeployment,
	mcp.ToolName("vercel_cancel_deployment"):       cancelDeployment,
	mcp.ToolName("vercel_delete_deployment"):       deleteDeployment,
	mcp.ToolName("vercel_list_deployment_events"):  listDeploymentEvents,
	mcp.ToolName("vercel_list_runtime_logs"):       listRuntimeLogs,
	mcp.ToolName("vercel_list_project_env_vars"):   listProjectEnvVars,
	mcp.ToolName("vercel_create_project_env_vars"): createProjectEnvVars,
	mcp.ToolName("vercel_update_project_env_var"):  updateProjectEnvVar,
	mcp.ToolName("vercel_delete_project_env_var"):  deleteProjectEnvVar,
	mcp.ToolName("vercel_add_project_domain"):      addProjectDomain,
	mcp.ToolName("vercel_remove_project_domain"):   removeProjectDomain,
	mcp.ToolName("vercel_get_domain_config"):       getDomainConfig,
	mcp.ToolName("vercel_list_deployment_aliases"): listDeploymentAliases,
	mcp.ToolName("vercel_assign_deployment_alias"): assignDeploymentAlias,
	mcp.ToolName("vercel_delete_deployment_alias"): deleteDeploymentAlias,
}
