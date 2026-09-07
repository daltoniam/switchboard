package microsoft365

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
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("microsoft365", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type m365 struct {
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	tenantID     string
	client       *http.Client
	baseURL      string
	configSvc    mcp.ConfigService
	mu           sync.Mutex
}

const (
	defaultBaseURL    = "https://graph.microsoft.com/v1.0"
	defaultTenant     = "common"
	maxResponseSize   = 10 * 1024 * 1024
	defaultPageSize   = 25
	maxDownloadBytes  = 10_000_000
	defaultDownloadBy = 5_000_000
	maxSimpleUpload   = 4 * 1024 * 1024
)

var (
	_ mcp.Integration                = (*m365)(nil)
	_ mcp.FieldCompactionIntegration = (*m365)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*m365)(nil)
	_ mcp.MarkdownIntegration        = (*m365)(nil)
	_ mcp.PlainTextCredentials       = (*m365)(nil)
	_ mcp.PlaceholderHints           = (*m365)(nil)
	_ mcp.OptionalCredentials        = (*m365)(nil)
)

func (m *m365) PlainTextKeys() []string {
	return []string{"base_url", "tenant_id", mcp.CredKeyClientID, mcp.CredKeyTokenSource}
}

func (m *m365) Placeholders() map[string]string {
	return map[string]string{
		"access_token":          "Microsoft Graph OAuth access token",
		"refresh_token":         "OAuth refresh token (optional; enables auto-refresh)",
		mcp.CredKeyClientID:     "Azure AD application (client) ID",
		mcp.CredKeyClientSecret: "Azure AD client secret",
		"tenant_id":             "Directory tenant ID, domain, or 'common' (default)",
		"base_url":              "https://graph.microsoft.com/v1.0 (default)",
	}
}

func (m *m365) OptionalKeys() []string {
	return []string{"refresh_token", mcp.CredKeyClientID, mcp.CredKeyClientSecret, "tenant_id", "base_url", mcp.CredKeyTokenSource}
}

func New() mcp.Integration {
	return &m365{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  defaultBaseURL,
		tenantID: defaultTenant,
	}
}

func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if m, ok := i.(*m365); ok {
		m.mu.Lock()
		m.configSvc = svc
		m.mu.Unlock()
	}
}

func (m *m365) Name() string { return "microsoft365" }

func (m *m365) Configure(_ context.Context, creds mcp.Credentials) error {
	m.accessToken = creds["access_token"]
	m.refreshToken = creds["refresh_token"]
	m.clientID = creds[mcp.CredKeyClientID]
	m.clientSecret = creds[mcp.CredKeyClientSecret]
	m.tenantID = creds["tenant_id"]
	if m.tenantID == "" {
		m.tenantID = defaultTenant
	}
	if m.accessToken == "" {
		return fmt.Errorf("microsoft365: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		m.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (m *m365) Healthy(ctx context.Context) bool {
	_, err := m.get(ctx, "/me")
	return err == nil
}

func (m *m365) Tools() []mcp.ToolDefinition {
	return tools
}

func (m *m365) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (m *m365) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (m *m365) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, m, args)
}

func (m *m365) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	data, _, err := m.doRequestInner(ctx, method, path, body, nil, true)
	return data, err
}

func (m *m365) doRequestInner(ctx context.Context, method, fullURL string, body any, headers map[string]string, canRetry bool) (json.RawMessage, string, error) {
	var bodyReader io.Reader
	if body != nil {
		switch b := body.(type) {
		case []byte:
			bodyReader = bytes.NewReader(b)
		case string:
			bodyReader = strings.NewReader(b)
		default:
			data, err := json.Marshal(body)
			if err != nil {
				return nil, "", err
			}
			bodyReader = bytes.NewReader(data)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, "", err
	}
	m.mu.Lock()
	token := m.accessToken
	m.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		if _, ok := req.Header["Content-Type"]; !ok {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, "", err
	}
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode == 401 && canRetry {
		if refreshed := m.tryRefresh(ctx); refreshed {
			return m.doRequestInner(ctx, method, fullURL, body, headers, false)
		}
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("microsoft365 API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, contentType, re
	}
	if resp.StatusCode >= 400 {
		return nil, contentType, fmt.Errorf("microsoft365 API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 202 || resp.StatusCode == 204 {
		return json.RawMessage(`{"status":"success"}`), contentType, nil
	}
	if len(data) == 0 && strings.Contains(strings.ToLower(contentType), "json") {
		return json.RawMessage(`{"status":"success"}`), contentType, nil
	}
	return json.RawMessage(data), contentType, nil
}

func (m *m365) tryRefresh(ctx context.Context) bool {
	m.mu.Lock()
	refreshToken := m.refreshToken
	clientID := m.clientID
	clientSecret := m.clientSecret
	tenant := m.tenantID
	currentToken := m.accessToken
	m.mu.Unlock()
	if refreshToken == "" || clientID == "" {
		return false
	}
	newAccess, newRefresh, err := RefreshAccessToken(ctx, clientID, clientSecret, refreshToken, tenant)
	if err != nil {
		return false
	}
	m.mu.Lock()
	if m.accessToken == currentToken {
		m.accessToken = newAccess
		if newRefresh != "" {
			m.refreshToken = newRefresh
		}
		m.persistTokens(newAccess, newRefresh)
	}
	m.mu.Unlock()
	return true
}

func (m *m365) persistTokens(access, refresh string) {
	if m.configSvc == nil {
		return
	}
	ic, ok := m.configSvc.GetIntegration("microsoft365")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = access
	if refresh != "" {
		ic.Credentials["refresh_token"] = refresh
	}
	_ = m.configSvc.SetIntegration("microsoft365", ic)
}

func (m *m365) absURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return m.baseURL + path
}

func (m *m365) get(ctx context.Context, path string) (json.RawMessage, error) {
	return m.doRequest(ctx, http.MethodGet, m.absURL(path), nil)
}

func (m *m365) getWithHeaders(ctx context.Context, path string, headers map[string]string) (json.RawMessage, error) {
	data, _, err := m.doRequestInner(ctx, http.MethodGet, m.absURL(path), nil, headers, true)
	return data, err
}

func (m *m365) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return m.doRequest(ctx, http.MethodPost, m.absURL(path), body)
}

func (m *m365) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return m.doRequest(ctx, http.MethodPatch, m.absURL(path), body)
}

func (m *m365) put(ctx context.Context, path string, body any, contentType string) (json.RawMessage, error) {
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	data, _, err := m.doRequestInner(ctx, http.MethodPut, m.absURL(path), body, headers, true)
	return data, err
}

func (m *m365) del(ctx context.Context, path string) (json.RawMessage, error) {
	return m.doRequest(ctx, http.MethodDelete, m.absURL(path), nil)
}

func (m *m365) getRaw(ctx context.Context, path string) ([]byte, string, error) {
	data, ct, err := m.doRequestInner(ctx, http.MethodGet, m.absURL(path), nil, map[string]string{"Accept": "*/*"}, true)
	if err != nil {
		return nil, ct, err
	}
	return []byte(data), ct, nil
}

type handlerFunc func(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error)

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

func userPath(userID string) string {
	if userID == "" || userID == "me" {
		return "/me"
	}
	return "/users/" + url.PathEscape(userID)
}

func quoteGraphSearch(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	if len(q) >= 2 && q[0] == '"' && q[len(q)-1] == '"' {
		return q
	}
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return `"` + q + `"`
}

func directorySearch(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	upper := strings.ToUpper(q)
	if strings.Contains(upper, " OR ") || strings.Contains(upper, " AND ") {
		return q
	}
	if strings.Contains(q, ":") {
		return quoteGraphSearch(q)
	}
	escaped := strings.ReplaceAll(strings.ReplaceAll(q, `\`, `\\`), `"`, `\"`)
	return `"displayName:` + escaped + `" OR "mail:` + escaped + `"`
}

func graphListParams(r *mcp.Args) map[string]string {
	top := fmt.Sprintf("%d", r.OptInt("top", defaultPageSize))
	params := map[string]string{
		"$filter":  r.Str("filter"),
		"$select":  r.Str("select"),
		"$search":  quoteGraphSearch(r.Str("search")),
		"$orderby": r.Str("orderby"),
		"$top":     top,
	}
	return params
}

func nextOrGet(ctx context.Context, m *m365, args map[string]any, buildPath func() (string, map[string]string, error)) (json.RawMessage, error) {
	r := mcp.NewArgs(args)
	next := r.Str("next_link")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if next != "" {
		if !allowedNextLink(next, m.baseURL) {
			return nil, fmt.Errorf("next_link must be a Microsoft Graph URL")
		}
		if needsEventualConsistency(next) {
			return m.getWithHeaders(ctx, next, map[string]string{"ConsistencyLevel": "eventual"})
		}
		return m.get(ctx, next)
	}
	path, headers, err := buildPath()
	if err != nil {
		return nil, err
	}
	if len(headers) > 0 {
		return m.getWithHeaders(ctx, path, headers)
	}
	return m.get(ctx, path)
}

func needsEventualConsistency(next string) bool {
	u, err := url.Parse(next)
	if err != nil {
		return false
	}
	q := u.Query()
	return q.Get("$search") != "" || q.Get("$count") != ""
}

func allowedNextLink(next, baseURL string) bool {
	u, err := url.Parse(next)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme == "https" {
		switch host {
		case "graph.microsoft.com", "graph.microsoft.us", "graph.microsoft.de", "microsoftgraph.chinacloudapi.cn":
			return true
		}
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Hostname() == "" {
		return false
	}
	if !strings.EqualFold(base.Hostname(), host) {
		return false
	}
	return u.Scheme == "https" || u.Scheme == base.Scheme
}

func unwrapGraph(data json.RawMessage) json.RawMessage {
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return data
	}
	changed := false
	if next, ok := env["@odata.nextLink"]; ok {
		env["next_link"] = next
		delete(env, "@odata.nextLink")
		changed = true
	}
	if count, ok := env["@odata.count"]; ok {
		env["count"] = count
		delete(env, "@odata.count")
		changed = true
	}
	delete(env, "@odata.context")
	if !changed {
		return data
	}
	out, err := json.Marshal(env)
	if err != nil {
		return data
	}
	return out
}

func graphResult(data json.RawMessage, err error) (*mcp.ToolResult, error) {
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapGraph(data))
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("microsoft365_get_me"):                getMe,
	mcp.ToolName("microsoft365_list_users"):            listUsers,
	mcp.ToolName("microsoft365_search_people"):         searchPeople,
	mcp.ToolName("microsoft365_list_messages"):         listMessages,
	mcp.ToolName("microsoft365_get_message"):           getMessage,
	mcp.ToolName("microsoft365_send_mail"):             sendMail,
	mcp.ToolName("microsoft365_create_draft"):          createDraft,
	mcp.ToolName("microsoft365_reply_message"):         replyMessage,
	mcp.ToolName("microsoft365_delete_message"):        deleteMessage,
	mcp.ToolName("microsoft365_list_mail_folders"):     listMailFolders,
	mcp.ToolName("microsoft365_list_calendars"):        listCalendars,
	mcp.ToolName("microsoft365_list_events"):           listEvents,
	mcp.ToolName("microsoft365_get_event"):             getEvent,
	mcp.ToolName("microsoft365_create_event"):          createEvent,
	mcp.ToolName("microsoft365_update_event"):          updateEvent,
	mcp.ToolName("microsoft365_delete_event"):          deleteEvent,
	mcp.ToolName("microsoft365_list_drive_items"):      listDriveItems,
	mcp.ToolName("microsoft365_get_drive_item"):        getDriveItem,
	mcp.ToolName("microsoft365_search_drive"):          searchDrive,
	mcp.ToolName("microsoft365_download_drive_item"):   downloadDriveItem,
	mcp.ToolName("microsoft365_create_folder"):         createFolder,
	mcp.ToolName("microsoft365_upload_drive_item"):     uploadDriveItem,
	mcp.ToolName("microsoft365_delete_drive_item"):     deleteDriveItem,
	mcp.ToolName("microsoft365_list_teams"):            listTeams,
	mcp.ToolName("microsoft365_list_channels"):         listChannels,
	mcp.ToolName("microsoft365_list_channel_messages"): listChannelMessages,
	mcp.ToolName("microsoft365_send_channel_message"):  sendChannelMessage,
	mcp.ToolName("microsoft365_list_chats"):            listChats,
	mcp.ToolName("microsoft365_list_chat_messages"):    listChatMessages,
	mcp.ToolName("microsoft365_send_chat_message"):     sendChatMessage,
	mcp.ToolName("microsoft365_list_todo_lists"):       listTodoLists,
	mcp.ToolName("microsoft365_list_todo_tasks"):       listTodoTasks,
	mcp.ToolName("microsoft365_create_todo_task"):      createTodoTask,
	mcp.ToolName("microsoft365_update_todo_task"):      updateTodoTask,
	mcp.ToolName("microsoft365_delete_todo_task"):      deleteTodoTask,
}
