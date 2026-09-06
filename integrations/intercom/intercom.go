package intercom

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

var compactResult = compact.MustLoadWithOverlay("intercom", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type intercom struct {
	accessToken string
	client      *http.Client
	baseURL     string
}

const (
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
	apiVersion      = "2.16"
	defaultBaseURL  = "https://api.intercom.io"
)

var (
	_ mcp.Integration                = (*intercom)(nil)
	_ mcp.FieldCompactionIntegration = (*intercom)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*intercom)(nil)
	_ mcp.MarkdownIntegration        = (*intercom)(nil)
	_ mcp.PlainTextCredentials       = (*intercom)(nil)
	_ mcp.PlaceholderHints           = (*intercom)(nil)
	_ mcp.OptionalCredentials        = (*intercom)(nil)
)

func (c *intercom) PlainTextKeys() []string { return []string{"base_url"} }

func (c *intercom) Placeholders() map[string]string {
	return map[string]string{
		"access_token": "Intercom access token from Developer Hub",
		"base_url":     "https://api.intercom.io (default), https://api.eu.intercom.io, or https://api.au.intercom.io",
	}
}

func (c *intercom) OptionalKeys() []string { return []string{"base_url"} }

func New() mcp.Integration {
	return &intercom{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultBaseURL,
	}
}

func (c *intercom) Name() string { return "intercom" }

func (c *intercom) Configure(_ context.Context, creds mcp.Credentials) error {
	c.accessToken = creds["access_token"]
	if c.accessToken == "" {
		return fmt.Errorf("intercom: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		c.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (c *intercom) Healthy(ctx context.Context) bool {
	_, err := c.get(ctx, "/me")
	return err == nil
}

func (c *intercom) Tools() []mcp.ToolDefinition {
	return tools
}

func (c *intercom) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (c *intercom) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (c *intercom) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, c, args)
}

func (c *intercom) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Intercom-Version", apiVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("intercom API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("intercom API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (c *intercom) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return c.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (c *intercom) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

func (c *intercom) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doRequest(ctx, http.MethodPut, path, body)
}

type handlerFunc func(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error)

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

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("intercom_search_conversations"): searchConversations,
	mcp.ToolName("intercom_list_conversations"):   listConversations,
	mcp.ToolName("intercom_get_conversation"):     getConversation,
	mcp.ToolName("intercom_reply_conversation"):   replyConversation,
	mcp.ToolName("intercom_close_conversation"):   closeConversation,
	mcp.ToolName("intercom_reopen_conversation"):  reopenConversation,
	mcp.ToolName("intercom_snooze_conversation"):  snoozeConversation,
	mcp.ToolName("intercom_assign_conversation"):  assignConversation,
	mcp.ToolName("intercom_tag_conversation"):     tagConversation,
	mcp.ToolName("intercom_search_contacts"):      searchContacts,
	mcp.ToolName("intercom_list_contacts"):        listContacts,
	mcp.ToolName("intercom_get_contact"):          getContact,
	mcp.ToolName("intercom_create_contact"):       createContact,
	mcp.ToolName("intercom_update_contact"):       updateContact,
	mcp.ToolName("intercom_list_companies"):       listCompanies,
	mcp.ToolName("intercom_get_company"):          getCompany,
	mcp.ToolName("intercom_search_tickets"):       searchTickets,
	mcp.ToolName("intercom_get_ticket"):           getTicket,
	mcp.ToolName("intercom_create_ticket"):        createTicket,
	mcp.ToolName("intercom_update_ticket"):        updateTicket,
	mcp.ToolName("intercom_reply_ticket"):         replyTicket,
	mcp.ToolName("intercom_search_articles"):      searchArticles,
	mcp.ToolName("intercom_list_articles"):        listArticles,
	mcp.ToolName("intercom_get_article"):          getArticle,
	mcp.ToolName("intercom_list_admins"):          listAdmins,
	mcp.ToolName("intercom_get_me"):               getMe,
	mcp.ToolName("intercom_list_teams"):           listTeams,
	mcp.ToolName("intercom_get_team"):             getTeam,
	mcp.ToolName("intercom_list_tags"):            listTags,
	mcp.ToolName("intercom_list_ticket_types"):    listTicketTypes,
}
