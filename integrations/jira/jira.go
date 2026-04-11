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
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*jira)(nil)
	_ mcp.FieldCompactionIntegration = (*jira)(nil)
	_ mcp.MarkdownIntegration        = (*jira)(nil)
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

func (j *jira) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (j *jira) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
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

type handlerFunc func(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error)

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

var dispatch = map[mcp.ToolName]handlerFunc{
	// Issues
	mcp.ToolName("jira_search_issues"):     searchIssues,
	mcp.ToolName("jira_get_issue"):         getIssue,
	mcp.ToolName("jira_create_issue"):      createIssue,
	mcp.ToolName("jira_update_issue"):      updateIssue,
	mcp.ToolName("jira_delete_issue"):      deleteIssue,
	mcp.ToolName("jira_transition_issue"):  transitionIssue,
	mcp.ToolName("jira_get_transitions"):   getTransitions,
	mcp.ToolName("jira_assign_issue"):      assignIssue,
	mcp.ToolName("jira_list_comments"):     listComments,
	mcp.ToolName("jira_add_comment"):       addComment,
	mcp.ToolName("jira_update_comment"):    updateComment,
	mcp.ToolName("jira_delete_comment"):    deleteComment,
	mcp.ToolName("jira_list_issue_links"):  listIssueLinks,
	mcp.ToolName("jira_create_issue_link"): createIssueLink,
	mcp.ToolName("jira_delete_issue_link"): deleteIssueLink,

	// Projects
	mcp.ToolName("jira_list_projects"):           listProjects,
	mcp.ToolName("jira_get_project"):             getProject,
	mcp.ToolName("jira_list_project_components"): listProjectComponents,
	mcp.ToolName("jira_list_project_versions"):   listProjectVersions,
	mcp.ToolName("jira_list_project_statuses"):   listProjectStatuses,

	// Boards & Sprints
	mcp.ToolName("jira_list_boards"):           listBoards,
	mcp.ToolName("jira_get_board"):             getBoard,
	mcp.ToolName("jira_list_sprints"):          listSprints,
	mcp.ToolName("jira_get_sprint"):            getSprint,
	mcp.ToolName("jira_create_sprint"):         createSprint,
	mcp.ToolName("jira_update_sprint"):         updateSprint,
	mcp.ToolName("jira_get_sprint_issues"):     getSprintIssues,
	mcp.ToolName("jira_move_issues_to_sprint"): moveIssuesToSprint,
	mcp.ToolName("jira_list_board_backlog"):    listBoardBacklog,
	mcp.ToolName("jira_get_board_config"):      getBoardConfig,

	// Users
	mcp.ToolName("jira_get_myself"):   getMyself,
	mcp.ToolName("jira_search_users"): searchUsers,
	mcp.ToolName("jira_get_user"):     getUser,

	// Metadata
	mcp.ToolName("jira_list_issue_types"): listIssueTypes,
	mcp.ToolName("jira_list_priorities"):  listPriorities,
	mcp.ToolName("jira_list_statuses"):    listStatuses,
	mcp.ToolName("jira_list_labels"):      listLabels,
	mcp.ToolName("jira_list_fields"):      listFields,
	mcp.ToolName("jira_list_filters"):     listFilters,
	mcp.ToolName("jira_get_filter"):       getFilter,

	// Worklogs & Info
	mcp.ToolName("jira_list_worklogs"):   listWorklogs,
	mcp.ToolName("jira_add_worklog"):     addWorklog,
	mcp.ToolName("jira_get_server_info"): getServerInfo,
}
