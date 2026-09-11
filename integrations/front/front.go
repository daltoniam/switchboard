package front

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

var compactResult = compact.MustLoadWithOverlay("front", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type front struct {
	accessToken string
	client      *http.Client
	baseURL     string
}

const (
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
	defaultBaseURL  = "https://api2.frontapp.com"
)

var (
	_ mcp.Integration                = (*front)(nil)
	_ mcp.FieldCompactionIntegration = (*front)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*front)(nil)
	_ mcp.MarkdownIntegration        = (*front)(nil)
	_ mcp.PlainTextCredentials       = (*front)(nil)
	_ mcp.PlaceholderHints           = (*front)(nil)
	_ mcp.OptionalCredentials        = (*front)(nil)
)

func (f *front) PlainTextKeys() []string { return []string{"base_url"} }

func (f *front) Placeholders() map[string]string {
	return map[string]string{
		"access_token": "Front API token or OAuth access token",
		"base_url":     "https://api2.frontapp.com (default) or https://{company}.api.frontapp.com",
	}
}

func (f *front) OptionalKeys() []string { return []string{"base_url"} }

func New() mcp.Integration {
	return &front{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultBaseURL,
	}
}

func (f *front) Name() string { return "front" }

func (f *front) Configure(_ context.Context, creds mcp.Credentials) error {
	f.accessToken = creds["access_token"]
	if f.accessToken == "" {
		return fmt.Errorf("front: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		f.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (f *front) Healthy(ctx context.Context) bool {
	_, err := f.get(ctx, "/teammates?limit=1")
	return err == nil
}

func (f *front) Tools() []mcp.ToolDefinition {
	return tools
}

func (f *front) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (f *front) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (f *front) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, f, args)
}

func (f *front) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, f.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.accessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("front API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("front API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (f *front) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return f.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (f *front) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return f.doRequest(ctx, http.MethodPost, path, body)
}

func (f *front) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return f.doRequest(ctx, http.MethodPatch, path, body)
}

type handlerFunc func(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error)

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
	mcp.ToolName("front_search_conversations"):       searchConversations,
	mcp.ToolName("front_list_conversations"):         listConversations,
	mcp.ToolName("front_get_conversation"):           getConversation,
	mcp.ToolName("front_list_conversation_messages"): listConversationMessages,
	mcp.ToolName("front_list_conversation_comments"): listConversationComments,
	mcp.ToolName("front_add_comment"):                addComment,
	mcp.ToolName("front_update_conversation"):        updateConversation,
	mcp.ToolName("front_assign_conversation"):        assignConversation,
	mcp.ToolName("front_list_inboxes"):               listInboxes,
	mcp.ToolName("front_list_teammates"):             listTeammates,
	mcp.ToolName("front_list_tags"):                  listTags,
	mcp.ToolName("front_list_channels"):              listChannels,
	mcp.ToolName("front_list_contacts"):              listContacts,
	mcp.ToolName("front_search_contacts"):            searchContacts,
	mcp.ToolName("front_get_contact"):                getContact,
	mcp.ToolName("front_list_accounts"):              listAccounts,
	mcp.ToolName("front_get_account"):                getAccount,
	mcp.ToolName("front_create_message"):             createMessage,
	mcp.ToolName("front_reply_conversation"):         replyConversation,
	mcp.ToolName("front_create_draft"):               createDraft,
}
