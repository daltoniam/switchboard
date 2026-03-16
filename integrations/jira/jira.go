package jira

import (
	"bytes"
	"context"
	"encoding/base64"
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

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*jira)(nil)
	_ mcp.FieldCompactionIntegration = (*jira)(nil)
	_ mcp.PlainTextCredentials       = (*jira)(nil)
)

type jira struct {
	email    string
	apiToken string
	domain   string
	client   *http.Client
	baseURL  string // https://{domain}.atlassian.net/rest/api/3
	agileURL string // https://{domain}.atlassian.net/rest/agile/1.0
}

func New() mcp.Integration {
	return &jira{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (j *jira) Name() string { return "jira" }

func (j *jira) PlainTextKeys() []string { return []string{"email", "domain"} }

func (j *jira) Configure(_ context.Context, creds mcp.Credentials) error {
	j.email = creds["email"]
	j.apiToken = creds["api_token"]
	j.domain = creds["domain"]
	if j.email == "" {
		return fmt.Errorf("jira: email is required")
	}
	if j.apiToken == "" {
		return fmt.Errorf("jira: api_token is required")
	}
	if j.domain == "" {
		return fmt.Errorf("jira: domain is required")
	}
	j.baseURL = fmt.Sprintf("https://%s.atlassian.net/rest/api/3", j.domain)
	j.agileURL = fmt.Sprintf("https://%s.atlassian.net/rest/agile/1.0", j.domain)
	return nil
}

func (j *jira) Healthy(ctx context.Context) bool {
	_, err := j.get(ctx, "/myself")
	return err == nil
}

func (j *jira) Tools() []mcp.ToolDefinition {
	return tools
}

func (j *jira) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (j *jira) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, j, args)
}

// --- HTTP helpers ---

func (j *jira) authHeader() string {
	creds := j.email + ":" + j.apiToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (j *jira) doRequest(ctx context.Context, method, fullURL string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", j.authHeader())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	const maxResponseSize = 10 * 1024 * 1024 // 10 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("jira API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jira API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// REST API v3 helpers
func (j *jira) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return j.doRequest(ctx, "GET", j.baseURL+fmt.Sprintf(pathFmt, args...), nil)
}

func (j *jira) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return j.doRequest(ctx, "POST", j.baseURL+path, body)
}

func (j *jira) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return j.doRequest(ctx, "PUT", j.baseURL+path, body)
}

func (j *jira) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return j.doRequest(ctx, "DELETE", j.baseURL+fmt.Sprintf(pathFmt, args...), nil)
}

// Agile API v1.0 helpers
func (j *jira) agileGet(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return j.doRequest(ctx, "GET", j.agileURL+fmt.Sprintf(pathFmt, args...), nil)
}

func (j *jira) agilePost(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return j.doRequest(ctx, "POST", j.agileURL+path, body)
}

func (j *jira) agilePut(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return j.doRequest(ctx, "PUT", j.agileURL+path, body)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	if mcp.IsRetryable(err) {
		return nil, err
	}
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
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

// textToADF wraps plain text in a minimal Atlassian Document Format structure.
func textToADF(text string) map[string]any {
	paragraphs := []any{}
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			paragraphs = append(paragraphs, map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": " "},
				},
			})
		} else {
			paragraphs = append(paragraphs, map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": line},
				},
			})
		}
	}
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": paragraphs,
	}
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Issues
	"jira_search_issues":     searchIssues,
	"jira_get_issue":         getIssue,
	"jira_create_issue":      createIssue,
	"jira_update_issue":      updateIssue,
	"jira_delete_issue":      deleteIssue,
	"jira_transition_issue":  transitionIssue,
	"jira_get_transitions":   getTransitions,
	"jira_assign_issue":      assignIssue,
	"jira_list_comments":     listComments,
	"jira_add_comment":       addComment,
	"jira_update_comment":    updateComment,
	"jira_delete_comment":    deleteComment,
	"jira_list_issue_links":  listIssueLinks,
	"jira_create_issue_link": createIssueLink,
	"jira_delete_issue_link": deleteIssueLink,

	// Projects
	"jira_list_projects":           listProjects,
	"jira_get_project":             getProject,
	"jira_list_project_components": listProjectComponents,
	"jira_list_project_versions":   listProjectVersions,
	"jira_list_project_statuses":   listProjectStatuses,

	// Boards & Sprints
	"jira_list_boards":           listBoards,
	"jira_get_board":             getBoard,
	"jira_list_sprints":          listSprints,
	"jira_get_sprint":            getSprint,
	"jira_create_sprint":         createSprint,
	"jira_update_sprint":         updateSprint,
	"jira_get_sprint_issues":     getSprintIssues,
	"jira_move_issues_to_sprint": moveIssuesToSprint,
	"jira_list_board_backlog":    listBoardBacklog,
	"jira_get_board_config":      getBoardConfig,

	// Users
	"jira_get_myself":   getMyself,
	"jira_search_users": searchUsers,
	"jira_get_user":     getUser,

	// Metadata
	"jira_list_issue_types": listIssueTypes,
	"jira_list_priorities":  listPriorities,
	"jira_list_statuses":    listStatuses,
	"jira_list_labels":      listLabels,
	"jira_list_fields":      listFields,
	"jira_list_filters":     listFilters,
	"jira_get_filter":       getFilter,

	// Worklogs & Info
	"jira_list_worklogs":   listWorklogs,
	"jira_add_worklog":     addWorklog,
	"jira_get_server_info": getServerInfo,
}
