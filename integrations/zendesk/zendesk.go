package zendesk

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
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("zendesk", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type zendesk struct {
	subdomain   string
	email       string
	apiToken    string
	accessToken string
	client      *http.Client
	baseURL     string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var (
	_ mcp.Integration                = (*zendesk)(nil)
	_ mcp.FieldCompactionIntegration = (*zendesk)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*zendesk)(nil)
	_ mcp.MarkdownIntegration        = (*zendesk)(nil)
	_ mcp.PlainTextCredentials       = (*zendesk)(nil)
	_ mcp.PlaceholderHints           = (*zendesk)(nil)
	_ mcp.OptionalCredentials        = (*zendesk)(nil)
)

func (z *zendesk) PlainTextKeys() []string { return []string{"subdomain", "email", "base_url"} }

func (z *zendesk) Placeholders() map[string]string {
	return map[string]string{
		"subdomain":    "Zendesk subdomain (acme from acme.zendesk.com)",
		"email":        "Agent email for API token auth",
		"api_token":    "Zendesk API token (Admin Center > Apps and integrations > Zendesk API)",
		"access_token": "OAuth access token (alternative to email + api_token)",
		"base_url":     "Optional override (default https://{subdomain}.zendesk.com/api/v2)",
	}
}

func (z *zendesk) OptionalKeys() []string {
	return []string{"email", "api_token", "access_token", "base_url"}
}

func New() mcp.Integration {
	return &zendesk{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (z *zendesk) Name() string { return "zendesk" }

func (z *zendesk) Configure(_ context.Context, creds mcp.Credentials) error {
	z.subdomain = normalizeSubdomain(creds["subdomain"])
	z.email = creds["email"]
	z.apiToken = creds["api_token"]
	z.accessToken = creds["access_token"]
	if z.accessToken == "" && (z.email == "" || z.apiToken == "") {
		return fmt.Errorf("zendesk: access_token or email + api_token is required")
	}
	if v := creds["base_url"]; v != "" {
		z.baseURL = strings.TrimRight(v, "/")
		return nil
	}
	if z.subdomain == "" {
		return fmt.Errorf("zendesk: subdomain is required")
	}
	z.baseURL = fmt.Sprintf("https://%s.zendesk.com/api/v2", z.subdomain)
	return nil
}

func (z *zendesk) Healthy(ctx context.Context) bool {
	_, err := z.get(ctx, "/users/me.json")
	return err == nil
}

func (z *zendesk) Tools() []mcp.ToolDefinition {
	return tools
}

func (z *zendesk) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (z *zendesk) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (z *zendesk) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, z, args)
}

func (z *zendesk) authHeader() string {
	if z.accessToken != "" {
		return "Bearer " + z.accessToken
	}
	creds := z.email + "/token:" + z.apiToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (z *zendesk) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, z.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", z.authHeader())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("zendesk API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("zendesk API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (z *zendesk) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return z.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (z *zendesk) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return z.doRequest(ctx, http.MethodPost, path, body)
}

func (z *zendesk) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return z.doRequest(ctx, http.MethodPut, path, body)
}

func (z *zendesk) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return z.doRequest(ctx, http.MethodDelete, fmt.Sprintf(pathFmt, args...), nil)
}

type handlerFunc func(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error)

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

func pageParams(args map[string]any) map[string]string {
	r := mcp.NewArgs(args)
	size := r.OptInt("page_size", 25)
	after := r.Str("page_after")
	before := r.Str("page_before")
	_ = r.Err()
	params := map[string]string{"page[size]": strconv.Itoa(size)}
	if after != "" {
		params["page[after]"] = after
	}
	if before != "" {
		params["page[before]"] = before
	}
	return params
}

func normalizeSubdomain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".zendesk.com")
	return s
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("zendesk_search_tickets"):            searchTickets,
	mcp.ToolName("zendesk_list_tickets"):              listTickets,
	mcp.ToolName("zendesk_get_ticket"):                getTicket,
	mcp.ToolName("zendesk_create_ticket"):             createTicket,
	mcp.ToolName("zendesk_update_ticket"):             updateTicket,
	mcp.ToolName("zendesk_delete_ticket"):             deleteTicket,
	mcp.ToolName("zendesk_list_ticket_comments"):      listTicketComments,
	mcp.ToolName("zendesk_add_ticket_comment"):        addTicketComment,
	mcp.ToolName("zendesk_list_ticket_audits"):        listTicketAudits,
	mcp.ToolName("zendesk_search"):                    search,
	mcp.ToolName("zendesk_list_users"):                listUsers,
	mcp.ToolName("zendesk_get_user"):                  getUser,
	mcp.ToolName("zendesk_get_current_user"):          getCurrentUser,
	mcp.ToolName("zendesk_create_user"):               createUser,
	mcp.ToolName("zendesk_update_user"):               updateUser,
	mcp.ToolName("zendesk_list_organizations"):        listOrganizations,
	mcp.ToolName("zendesk_get_organization"):          getOrganization,
	mcp.ToolName("zendesk_list_groups"):               listGroups,
	mcp.ToolName("zendesk_get_group"):                 getGroup,
	mcp.ToolName("zendesk_list_views"):                listViews,
	mcp.ToolName("zendesk_list_view_tickets"):         listViewTickets,
	mcp.ToolName("zendesk_list_macros"):               listMacros,
	mcp.ToolName("zendesk_search_articles"):           searchArticles,
	mcp.ToolName("zendesk_get_article"):               getArticle,
	mcp.ToolName("zendesk_list_tags"):                 listTags,
	mcp.ToolName("zendesk_list_satisfaction_ratings"): listSatisfactionRatings,
}
