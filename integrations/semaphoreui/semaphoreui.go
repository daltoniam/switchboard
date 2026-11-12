package semaphoreui

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var fieldCompactionSpecs = compact.MustLoadWithOverlay("semaphoreui", compactYAML, compact.Options{Strict: false}).Specs

const maxResponseBytes = 2 * 1024 * 1024

var (
	_ mcp.Integration                = (*semaphoreui)(nil)
	_ mcp.FieldCompactionIntegration = (*semaphoreui)(nil)
	_ mcp.PlainTextCredentials       = (*semaphoreui)(nil)
	_ mcp.OptionalCredentials        = (*semaphoreui)(nil)
	_ mcp.PlaceholderHints           = (*semaphoreui)(nil)
	_ io.Closer                      = (*semaphoreui)(nil)
)

type semaphoreui struct {
	mu        sync.RWMutex
	client    *http.Client
	baseURL   string
	token     string
	projectID int
}

func New() mcp.Integration                     { return &semaphoreui{} }
func (s *semaphoreui) Name() string            { return "semaphoreui" }
func (s *semaphoreui) PlainTextKeys() []string { return []string{"base_url", "project_id", "ca_cert"} }
func (s *semaphoreui) OptionalKeys() []string  { return []string{"ca_cert"} }
func (s *semaphoreui) Placeholders() map[string]string {
	return map[string]string{
		"base_url":   "Trusted HTTPS Semaphore UI installation URL (v2.19.12)",
		"api_token":  "Dedicated non-admin guest API token, without Bearer prefix",
		"project_id": "One approved project ID, guest role with permissions=0",
		"ca_cert":    `Optional PEM CA certificate, literal \n accepted; TLS verification remains enabled`,
	}
}

func (s *semaphoreui) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	return nil
}

func (s *semaphoreui) clear() {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	s.client, s.baseURL, s.token, s.projectID = nil, "", "", 0
}

func positiveID(value string) (int, error) {
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return 0, fmt.Errorf("semaphoreui: ID must be a positive decimal integer")
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("semaphoreui: ID must be in 1..2147483647")
	}
	return int(parsed), nil
}

func (s *semaphoreui) Configure(ctx context.Context, creds mcp.Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	raw := creds["base_url"]
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, unicode.IsSpace) {
		return fmt.Errorf("semaphoreui: base_url must be a trusted HTTPS installation URL without credentials, query, fragment or encoded paths")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("semaphoreui: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			return fmt.Errorf("semaphoreui: invalid base_url port")
		}
	}
	for _, segment := range strings.Split(endpoint.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("semaphoreui: invalid installation prefix")
		}
	}
	projectID, err := positiveID(creds["project_id"])
	if err != nil {
		return err
	}
	token := creds["api_token"]
	if token == "" || strings.ContainsFunc(token, func(value rune) bool { return unicode.IsSpace(value) || unicode.IsControl(value) }) {
		return fmt.Errorf("semaphoreui: api_token is required without whitespace or Bearer prefix")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := creds["ca_cert"]; cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(cert, `\n`, "\n"))) {
			return fmt.Errorf("semaphoreui: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	s.client = &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	s.baseURL, s.token, s.projectID = strings.TrimRight(endpoint.String(), "/"), token, projectID
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := s.authorize(ctx); err != nil {
		s.clear()
		return err
	}
	return nil
}

func (s *semaphoreui) Healthy(ctx context.Context) bool {
	result, err := s.Execute(ctx, "semaphoreui_get_project", nil)
	return err == nil && result != nil && !result.IsError
}

var tools = []mcp.ToolDefinition{
	{Name: "semaphoreui_get_project", Access: mcp.AccessRead, Description: "Get the configured Semaphore UI project identity. Start here for self-hosted Ansible and OpenTofu automation task status, not Semaphore CI SaaS. Guest-only metadata reads, no infrastructure execution, logs or configuration."},
	{Name: "semaphoreui_list_templates", Access: mcp.AccessRead, Description: "List up to 100 Semaphore UI automation template summaries in the configured project, with safe last-task status when available. Use after semaphoreui_get_project; truncated explicitly signals an incomplete list. No playbooks, arguments, inventories or variables."},
	{Name: "semaphoreui_get_template", Access: mcp.AccessRead, Description: "Get one Semaphore UI template identity, app, type and safe last-task status. Use template_id from semaphoreui_list_templates. No executable configuration, secrets or run actions.", Parameters: map[string]string{"template_id": "Positive template ID from semaphoreui_list_templates"}, Required: []string{"template_id"}},
	{Name: "semaphoreui_list_tasks", Access: mcp.AccessRead, Description: "List a single newest-first page of Semaphore UI automation tasks with status and timestamps. Use after semaphoreui_list_templates. Status and template filters apply only to this page; keep next_before even for an empty filtered page. pagination_known=false means completeness is unknown. No logs or execution.", Parameters: map[string]string{"count": "Page size, default 50, 1-100", "before": "Positive next_before task ID from the previous page", "template_id": "Optional local template ID filter, not a server filter", "status": "Optional local task status filter: waiting, starting, running, stopping, stopped, success, error, confirmed, rejected, waiting_confirmation"}},
	{Name: "semaphoreui_get_task", Access: mcp.AccessRead, Description: "Get one exact Semaphore UI automation task status and timestamps in the configured project. Use task_id from semaphoreui_list_tasks. Deliberately excludes output, logs, messages, environment and secret-bearing configuration.", Parameters: map[string]string{"task_id": "Positive task ID from semaphoreui_list_tasks"}, Required: []string{"task_id"}},
}

func (s *semaphoreui) Tools() []mcp.ToolDefinition { return tools }
func (s *semaphoreui) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handler func(context.Context, *semaphoreui, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handler{
	"semaphoreui_get_project":    getProject,
	"semaphoreui_list_templates": listTemplates,
	"semaphoreui_get_template":   getTemplate,
	"semaphoreui_list_tasks":     listTasks,
	"semaphoreui_get_task":       getTask,
}

func (s *semaphoreui) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("semaphoreui: unknown tool: %s", name))
	}
	for _, tool := range tools {
		if tool.Name != name {
			continue
		}
		for key := range args {
			if _, ok := tool.Parameters[key]; !ok {
				return mcp.ErrResult(fmt.Errorf("semaphoreui: unsupported argument"))
			}
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.client == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := s.authorize(ctx); err != nil {
		return mcp.ErrResult(err)
	}
	return fn(ctx, s, args)
}

func (s *semaphoreui) projectPath() string { return "/project/" + strconv.Itoa(s.projectID) }

func (s *semaphoreui) authorize(ctx context.Context) error {
	var user struct {
		Admin *bool `json:"admin"`
	}
	if _, err := s.get(ctx, "/user", nil, &user); err != nil {
		return err
	}
	if user.Admin == nil || *user.Admin {
		return fmt.Errorf("semaphoreui: dedicated non-admin user required")
	}
	var info struct {
		Version string `json:"version"`
	}
	if _, err := s.get(ctx, "/info", nil, &info); err != nil {
		return err
	}
	if info.Version != "v2.19.12" && !strings.HasPrefix(info.Version, "v2.19.12-") {
		return fmt.Errorf("semaphoreui: only released v2.19.12 is verified")
	}
	var role struct {
		Role        string `json:"role"`
		Permissions *int64 `json:"permissions"`
	}
	if _, err := s.get(ctx, s.projectPath()+"/role", nil, &role); err != nil {
		return err
	}
	if role.Role != "guest" || role.Permissions == nil || *role.Permissions != 0 {
		return fmt.Errorf("semaphoreui: project guest role with permissions=0 required; remove run grants")
	}
	return nil
}

func (s *semaphoreui) get(ctx context.Context, path string, query url.Values, target any) (http.Header, error) {
	endpoint := s.baseURL + "/api" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("semaphoreui: cannot construct request")
		}
		request.Header.Set("Authorization", "Bearer "+s.token)
		request.Header.Set("Accept", "application/json")
		response, err := s.client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("semaphoreui: request failed; check connectivity, TLS trust and deadline")
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			_ = response.Body.Close()
			wait := mcp.ParseRetryAfter(response.Header.Get("Retry-After"))
			if parsed, err := http.ParseTime(response.Header.Get("Retry-After")); err == nil {
				wait = time.Until(parsed)
			}
			if wait <= 0 {
				wait = time.Second
			}
			if wait > 15*time.Second {
				return nil, fmt.Errorf("semaphoreui: rate limited; retry later")
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, fmt.Errorf("semaphoreui: request deadline exceeded")
			case <-timer.C:
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return nil, fmt.Errorf("semaphoreui: API HTTP %d; verify guest access for 401/403", response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		_ = response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("semaphoreui: response read failed")
		}
		if len(data) > maxResponseBytes {
			return nil, fmt.Errorf("semaphoreui: response exceeds 2 MiB; incomplete result rejected")
		}
		if strings.TrimSpace(string(data)) == "null" || json.Unmarshal(data, target) != nil {
			return nil, fmt.Errorf("semaphoreui: malformed or missing JSON response")
		}
		return response.Header, nil
	}
	return nil, fmt.Errorf("semaphoreui: rate limited; retry later")
}

type projectSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type taskSummary struct {
	ID         int     `json:"id"`
	ProjectID  *int    `json:"project_id,omitempty"`
	TemplateID int     `json:"template_id"`
	Status     *string `json:"status,omitempty"`
	Created    *string `json:"created,omitempty"`
	Start      *string `json:"start,omitempty"`
	End        *string `json:"end,omitempty"`
}
type lastTaskSummary struct {
	ID     int     `json:"id"`
	Status *string `json:"status,omitempty"`
}
type templateSummary struct {
	ID        int              `json:"id"`
	ProjectID *int             `json:"project_id,omitempty"`
	Name      string           `json:"name"`
	App       *string          `json:"app,omitempty"`
	Type      *string          `json:"type,omitempty"`
	LastTask  *lastTaskSummary `json:"last_task,omitempty"`
}

func (s *semaphoreui) validateIdentity(id int, projectID *int, expected int) error {
	if id < 1 || (expected > 0 && id != expected) || (projectID != nil && *projectID != s.projectID) {
		return fmt.Errorf("semaphoreui: response identity outside configured project or requested record")
	}
	return nil
}

func getProject(ctx context.Context, s *semaphoreui, _ map[string]any) (*mcp.ToolResult, error) {
	var project projectSummary
	if _, err := s.get(ctx, s.projectPath(), nil, &project); err != nil {
		return mcp.ErrResult(err)
	}
	if project.ID != s.projectID {
		return mcp.ErrResult(fmt.Errorf("semaphoreui: response project mismatch"))
	}
	return mcp.JSONResult(project)
}

func listTemplates(ctx context.Context, s *semaphoreui, _ map[string]any) (*mcp.ToolResult, error) {
	var templates []templateSummary
	if _, err := s.get(ctx, s.projectPath()+"/templates", url.Values{"sort": {"name"}, "order": {"asc"}}, &templates); err != nil {
		return mcp.ErrResult(err)
	}
	for _, template := range templates {
		if err := s.validateIdentity(template.ID, template.ProjectID, 0); err != nil {
			return mcp.ErrResult(err)
		}
	}
	truncated := len(templates) > 100
	if truncated {
		templates = templates[:100]
	}
	if templates == nil {
		templates = []templateSummary{}
	}
	return mcp.JSONResult(struct {
		Templates []templateSummary `json:"templates"`
		Truncated bool              `json:"truncated"`
	}{templates, truncated})
}

func getTemplate(ctx context.Context, s *semaphoreui, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	value := reader.Str("template_id")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id, err := positiveID(value)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var template templateSummary
	if _, err := s.get(ctx, s.projectPath()+"/templates/"+strconv.Itoa(id), nil, &template); err != nil {
		return mcp.ErrResult(err)
	}
	if err := s.validateIdentity(template.ID, template.ProjectID, id); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(template)
}

func getTask(ctx context.Context, s *semaphoreui, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	value := reader.Str("task_id")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id, err := positiveID(value)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var task taskSummary
	if _, err := s.get(ctx, s.projectPath()+"/tasks/"+strconv.Itoa(id), nil, &task); err != nil {
		return mcp.ErrResult(err)
	}
	if err := s.validateIdentity(task.ID, task.ProjectID, id); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(task)
}

func validStatus(status string) bool {
	switch status {
	case "waiting", "starting", "running", "stopping", "stopped", "success", "error", "confirmed", "rejected", "waiting_confirmation":
		return true
	default:
		return false
	}
}

func listTasks(ctx context.Context, s *semaphoreui, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	countValue := reader.Str("count")
	beforeValue, templateValue, status := reader.Str("before"), reader.Str("template_id"), reader.Str("status")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	count := 50
	if _, present := args["count"]; present {
		var err error
		count, err = positiveID(countValue)
		if err != nil {
			return mcp.ErrResult(err)
		}
	}
	if count < 1 || count > 100 || (status != "" && !validStatus(status)) {
		return mcp.ErrResult(fmt.Errorf("semaphoreui: invalid count or status"))
	}
	query := url.Values{"count": {strconv.Itoa(count)}}
	if _, present := args["before"]; present {
		before, err := positiveID(beforeValue)
		if err != nil {
			return mcp.ErrResult(err)
		}
		query.Set("before", strconv.Itoa(before))
	}
	templateID := 0
	if _, present := args["template_id"]; present {
		var err error
		templateID, err = positiveID(templateValue)
		if err != nil {
			return mcp.ErrResult(err)
		}
	}
	var tasks []taskSummary
	headers, err := s.get(ctx, s.projectPath()+"/tasks/last", query, &tasks)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(tasks) > count {
		return mcp.ErrResult(fmt.Errorf("semaphoreui: provider ignored page bound; incomplete result rejected"))
	}
	page := struct {
		Tasks           []taskSummary `json:"tasks"`
		HasNext         *bool         `json:"has_next,omitempty"`
		NextBefore      int           `json:"next_before,omitempty"`
		PaginationKnown bool          `json:"pagination_known"`
	}{Tasks: []taskSummary{}}
	minimum := 0
	for _, task := range tasks {
		if err := s.validateIdentity(task.ID, task.ProjectID, 0); err != nil {
			return mcp.ErrResult(err)
		}
		if minimum == 0 || task.ID < minimum {
			minimum = task.ID
		}
		if templateID > 0 && task.TemplateID != templateID {
			continue
		}
		if status != "" && (task.Status == nil || *task.Status != status) {
			continue
		}
		page.Tasks = append(page.Tasks, task)
	}
	if header := headers.Get("X-Has-Next"); header == "true" || header == "false" {
		hasNext := header == "true"
		if hasNext && minimum == 0 {
			return mcp.ErrResult(fmt.Errorf("semaphoreui: next page advertised without a cursor"))
		}
		page.HasNext, page.PaginationKnown = &hasNext, true
		if hasNext {
			page.NextBefore = minimum
		}
	}
	return mcp.JSONResult(page)
}
