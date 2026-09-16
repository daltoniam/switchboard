package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	mcp "github.com/daltoniam/switchboard"
)

const (
	integrationName    = "gitlab"
	defaultInstanceURL = "https://gitlab.com"
	apiV4Path          = "/api/v4"
)

type gitlab struct {
	mu         sync.RWMutex
	baseURL    string // instance root, no /api/v4
	token      string
	httpClient *http.Client
}

// New creates a GitLab REST API integration (GitLab.com or self-managed).
func New() mcp.Integration { return &gitlab{} }

func (g *gitlab) Name() string { return integrationName }

func (g *gitlab) PlainTextKeys() []string { return []string{"base_url"} }

func (g *gitlab) OptionalKeys() []string { return []string{"base_url"} }

func (g *gitlab) Configure(_ context.Context, creds mcp.Credentials) error {
	baseURL, err := normalizeInstanceURL(creds["base_url"])
	if err != nil {
		return err
	}
	token := strings.TrimSpace(creds["token"])
	if token == "" || strings.ContainsFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return fmt.Errorf("gitlab: token is required and must not contain whitespace or control characters")
	}
	hc := &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	g.mu.Lock()
	g.baseURL, g.token, g.httpClient = baseURL, token, hc
	g.mu.Unlock()
	return nil
}

func (g *gitlab) Healthy(ctx context.Context) bool {
	_, status, err := g.doJSON(ctx, http.MethodGet, "/user", nil, nil)
	return err == nil && status == http.StatusOK
}

func (g *gitlab) Tools() []mcp.ToolDefinition { return tools }

func (g *gitlab) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("gitlab: unknown tool: %s", name))
	}
	if err := validateArgs(name, args); err != nil {
		return mcp.ErrResult(err)
	}
	return handler(ctx, g, args)
}

func (g *gitlab) apiURL(path string, query url.Values) string {
	g.mu.RLock()
	base := strings.TrimRight(g.baseURL, "/")
	g.mu.RUnlock()
	inst, err := url.Parse(base)
	if err != nil || inst.Scheme == "" || inst.Host == "" {
		s := base + apiV4Path + path
		if len(query) > 0 {
			s += "?" + query.Encode()
		}
		return s
	}
	prefix := strings.TrimSuffix(inst.Path, "/")
	apiPath := prefix + apiV4Path + path
	inst.Path = strings.ReplaceAll(apiPath, "%2F", "/")
	if strings.Contains(apiPath, "%2F") {
		inst.RawPath = apiPath
	}
	inst.RawQuery = query.Encode()
	inst.Fragment = ""
	return inst.String()
}

func (g *gitlab) doJSON(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = strings.NewReader(string(data))
	}
	g.mu.RLock()
	token := g.token
	hc := g.httpClient
	g.mu.RUnlock()
	if token == "" || hc == nil {
		return nil, 0, mcp.ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, method, g.apiURL(path, query), bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if err := httpStatusError(resp.StatusCode, data); err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return json.RawMessage("null"), resp.StatusCode, nil
	}
	return json.RawMessage(data), resp.StatusCode, nil
}

func (g *gitlab) doText(ctx context.Context, path string, query url.Values, maxBytes int64) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiURL(path, query), nil)
	if err != nil {
		return "", 0, err
	}
	g.mu.RLock()
	token := g.token
	hc := g.httpClient
	g.mu.RUnlock()
	if token == "" || hc == nil {
		return "", 0, mcp.ErrNotConfigured
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	resp, err := hc.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return "", resp.StatusCode, err
	}
	if err := httpStatusError(resp.StatusCode, data); err != nil {
		return "", resp.StatusCode, err
	}
	return string(data), resp.StatusCode, nil
}

func httpStatusError(status int, data []byte) error {
	if status < 300 {
		return nil
	}
	msg := strings.TrimSpace(string(data))
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("gitlab: HTTP %d: %s", status, msg)
}

func jsonResult(raw json.RawMessage, err error) (*mcp.ToolResult, error) {
	if err != nil {
		return mcp.ErrResult(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return mcp.RawResult(raw)
	}
	return mcp.JSONResult(v)
}

func encodeProjectID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return id
	}
	if strings.Contains(id, "%") {
		unescaped, err := url.PathUnescape(id)
		if err != nil {
			return url.PathEscape(id)
		}
		id = unescaped
	}
	return url.PathEscape(id)
}

// NormalizeInstanceURL validates and canonicalizes a GitLab instance root URL.
func NormalizeInstanceURL(raw string) (string, error) {
	return normalizeInstanceURL(raw)
}

func pagination(r *mcp.Args) (page, perPage int) {
	return r.OptInt("page", 1), r.OptInt("per_page", 20)
}

func paginationQuery(r *mcp.Args) url.Values {
	page, perPage := pagination(r)
	q := url.Values{}
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	return q
}

func normalizeInstanceURL(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return defaultInstanceURL, nil
	}
	u, err := url.Parse(v)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("gitlab: base_url must be an HTTP(S) GitLab instance URL without credentials")
	}
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), apiV4Path)
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

type handlerFunc func(context.Context, *gitlab, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"gitlab_get_current_user":          handleGetCurrentUser,
	"gitlab_list_projects":             handleListProjects,
	"gitlab_get_project":               handleGetProject,
	"gitlab_list_merge_requests":       handleListMergeRequests,
	"gitlab_get_merge_request":         handleGetMergeRequest,
	"gitlab_create_merge_request_note": handleCreateMergeRequestNote,
	"gitlab_list_issues":               handleListIssues,
	"gitlab_get_issue":                 handleGetIssue,
	"gitlab_list_pipelines":            handleListPipelines,
	"gitlab_list_pipeline_jobs":        handleListPipelineJobs,
	"gitlab_get_job_trace":             handleGetJobTrace,
}
