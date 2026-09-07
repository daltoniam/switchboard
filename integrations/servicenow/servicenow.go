package servicenow

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("servicenow", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type servicenow struct {
	instanceURL string
	username    string
	password    string
	accessToken string
	client      *http.Client
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var tableNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

var (
	_ mcp.Integration                = (*servicenow)(nil)
	_ mcp.FieldCompactionIntegration = (*servicenow)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*servicenow)(nil)
	_ mcp.MarkdownIntegration        = (*servicenow)(nil)
	_ mcp.PlainTextCredentials       = (*servicenow)(nil)
	_ mcp.PlaceholderHints           = (*servicenow)(nil)
	_ mcp.OptionalCredentials        = (*servicenow)(nil)
)

func (s *servicenow) PlainTextKeys() []string { return []string{"instance_url", "username"} }

func (s *servicenow) Placeholders() map[string]string {
	return map[string]string{
		"instance_url": "https://dev12345.service-now.com",
		"username":     "Integration user name (basic auth)",
		"password":     "Integration user password (basic auth)",
		"access_token": "OAuth bearer token (alternative to username/password)",
	}
}

func (s *servicenow) OptionalKeys() []string {
	return []string{"username", "password", "access_token"}
}

func New() mcp.Integration {
	return &servicenow{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *servicenow) Name() string { return "servicenow" }

func (s *servicenow) Configure(_ context.Context, creds mcp.Credentials) error {
	s.instanceURL = normalizeInstance(creds["instance_url"])
	s.username = creds["username"]
	s.password = creds["password"]
	s.accessToken = creds["access_token"]
	if s.instanceURL == "" {
		return fmt.Errorf("servicenow: instance_url is required")
	}
	hasBasic := s.username != "" && s.password != ""
	hasToken := s.accessToken != ""
	if !hasBasic && !hasToken {
		return fmt.Errorf("servicenow: either access_token or username and password are required")
	}
	return nil
}

func (s *servicenow) Healthy(ctx context.Context) bool {
	_, err := s.get(ctx, "/api/now/table/sys_user?sysparm_limit=1")
	return err == nil
}

func (s *servicenow) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *servicenow) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *servicenow) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (s *servicenow) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func normalizeInstance(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimRight(v, "/")
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return v
	}
	if strings.Contains(v, ".") {
		return "https://" + v
	}
	return "https://" + v + ".service-now.com"
}

func validTable(table string) error {
	if !tableNamePattern.MatchString(table) {
		return fmt.Errorf("invalid table %q: must be alphanumeric/underscore", table)
	}
	return nil
}

func (s *servicenow) setAuth(req *http.Request) {
	if s.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.accessToken)
		return
	}
	token := base64.StdEncoding.EncodeToString([]byte(s.username + ":" + s.password))
	req.Header.Set("Authorization", "Basic "+token)
}

func (s *servicenow) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.instanceURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	s.setAuth(req)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("servicenow API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("servicenow API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (s *servicenow) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (s *servicenow) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, http.MethodPost, path, body)
}

func (s *servicenow) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, http.MethodPatch, path, body)
}

type handlerFunc func(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error)

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

func listQueryParams(args map[string]any, defaultFields string) (map[string]string, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	fields := r.Str("fields")
	displayValue := r.Str("display_value")
	limit := r.OptInt("limit", 25)
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return nil, err
	}

	if fields == "" {
		fields = defaultFields
	}
	if displayValue == "" {
		displayValue = "all"
	}
	params := map[string]string{
		"sysparm_query":                  query,
		"sysparm_fields":                 fields,
		"sysparm_display_value":          displayValue,
		"sysparm_exclude_reference_link": "true",
		"sysparm_limit":                  fmt.Sprintf("%d", limit),
	}
	if offset > 0 {
		params["sysparm_offset"] = fmt.Sprintf("%d", offset)
	}
	return params, nil
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("servicenow_list_incidents"):          listIncidents,
	mcp.ToolName("servicenow_get_incident"):            getIncident,
	mcp.ToolName("servicenow_create_incident"):         createIncident,
	mcp.ToolName("servicenow_update_incident"):         updateIncident,
	mcp.ToolName("servicenow_list_problems"):           listProblems,
	mcp.ToolName("servicenow_get_problem"):             getProblem,
	mcp.ToolName("servicenow_list_change_requests"):    listChangeRequests,
	mcp.ToolName("servicenow_get_change_request"):      getChangeRequest,
	mcp.ToolName("servicenow_list_catalog_requests"):   listCatalogRequests,
	mcp.ToolName("servicenow_get_catalog_request"):     getCatalogRequest,
	mcp.ToolName("servicenow_list_knowledge_articles"): listKnowledgeArticles,
	mcp.ToolName("servicenow_get_knowledge_article"):   getKnowledgeArticle,
	mcp.ToolName("servicenow_list_users"):              listUsers,
	mcp.ToolName("servicenow_get_user"):                getUser,
	mcp.ToolName("servicenow_list_groups"):             listGroups,
	mcp.ToolName("servicenow_list_cis"):                listCIs,
	mcp.ToolName("servicenow_get_ci"):                  getCI,
	mcp.ToolName("servicenow_list_records"):            listRecords,
	mcp.ToolName("servicenow_get_record"):              getRecord,
	mcp.ToolName("servicenow_create_record"):           createRecord,
	mcp.ToolName("servicenow_update_record"):           updateRecord,
	mcp.ToolName("servicenow_aggregate"):               aggregate,
	mcp.ToolName("servicenow_list_comments"):           listComments,
	mcp.ToolName("servicenow_add_comment"):             addComment,
	mcp.ToolName("servicenow_list_attachments"):        listAttachments,
}
