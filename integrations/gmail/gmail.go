package gmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type gmail struct {
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	client       *http.Client
	baseURL      string
	configSvc    mcp.ConfigService
	mu           sync.Mutex
}

var (
	_ mcp.FieldCompactionIntegration = (*gmail)(nil)
	_ mcp.MarkdownIntegration        = (*gmail)(nil)
)

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &gmail{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://gmail.googleapis.com",
	}
}

func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gmail); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gmail) Name() string { return "gmail" }

func (g *gmail) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds["client_id"]
	g.clientSecret = creds["client_secret"]
	if g.accessToken == "" {
		return fmt.Errorf("gmail: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (g *gmail) Healthy(ctx context.Context) bool {
	_, err := g.get(ctx, "/gmail/v1/users/me/profile")
	return err == nil
}

func (g *gmail) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gmail) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gmail) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- HTTP helpers ---

func (g *gmail) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, path, body, true)
}

func (g *gmail) doRequestInner(ctx context.Context, method, path string, body any, canRetry bool) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 && canRetry && g.refreshToken != "" && g.clientID != "" && g.clientSecret != "" {
		g.mu.Lock()
		currentToken := g.accessToken
		g.mu.Unlock()

		newToken, rerr := RefreshAccessToken(g.clientID, g.clientSecret, g.refreshToken)
		if rerr == nil {
			g.mu.Lock()
			if g.accessToken == currentToken {
				g.accessToken = newToken
				g.persistToken(newToken)
			}
			g.mu.Unlock()
			return g.doRequestInner(ctx, method, path, body, false)
		}
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gmail API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gmail API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gmail) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gmail) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gmail) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PATCH", path, body)
}

func (g *gmail) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PUT", path, body)
}

func (g *gmail) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gmail) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gmail")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gmail", ic)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error)

// --- Argument helpers ---

func parseJSON(args map[string]any, key string) (any, error) {
	v, err := mcp.ArgStr(args, key)
	if err != nil {
		return nil, err
	}
	if v == "" {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON for %s: %w", key, err)
	}
	return out, nil
}

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

func queryEncodeMulti(params map[string]string, multi map[string][]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	for k, vs := range multi {
		for _, v := range vs {
			vals.Add(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

// user returns the userId from args, falling back to "me".
func user(r *mcp.Args) string {
	if v := r.Str("user_id"); v != "" {
		return v
	}
	return "me"
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Profile
	mcp.ToolName("gmail_get_profile"): getProfile,

	// Messages
	mcp.ToolName("gmail_list_messages"):   listMessages,
	mcp.ToolName("gmail_get_message"):     getMessage,
	mcp.ToolName("gmail_send_message"):    sendMessage,
	mcp.ToolName("gmail_delete_message"):  deleteMessage,
	mcp.ToolName("gmail_trash_message"):   trashMessage,
	mcp.ToolName("gmail_untrash_message"): untrashMessage,
	mcp.ToolName("gmail_modify_message"):  modifyMessage,
	mcp.ToolName("gmail_batch_modify"):    batchModifyMessages,
	mcp.ToolName("gmail_batch_delete"):    batchDeleteMessages,
	mcp.ToolName("gmail_get_attachment"):  getAttachment,

	// Threads
	mcp.ToolName("gmail_list_threads"):   listThreads,
	mcp.ToolName("gmail_get_thread"):     getThread,
	mcp.ToolName("gmail_delete_thread"):  deleteThread,
	mcp.ToolName("gmail_trash_thread"):   trashThread,
	mcp.ToolName("gmail_untrash_thread"): untrashThread,
	mcp.ToolName("gmail_modify_thread"):  modifyThread,

	// Labels
	mcp.ToolName("gmail_list_labels"):  listLabels,
	mcp.ToolName("gmail_get_label"):    getLabel,
	mcp.ToolName("gmail_create_label"): createLabel,
	mcp.ToolName("gmail_update_label"): updateLabel,
	mcp.ToolName("gmail_delete_label"): deleteLabel,

	// Drafts
	mcp.ToolName("gmail_list_drafts"):  listDrafts,
	mcp.ToolName("gmail_get_draft"):    getDraft,
	mcp.ToolName("gmail_create_draft"): createDraft,
	mcp.ToolName("gmail_update_draft"): updateDraft,
	mcp.ToolName("gmail_delete_draft"): deleteDraft,
	mcp.ToolName("gmail_send_draft"):   sendDraft,

	// History
	mcp.ToolName("gmail_list_history"): listHistory,

	// Settings
	mcp.ToolName("gmail_get_vacation"):           getVacation,
	mcp.ToolName("gmail_update_vacation"):        updateVacation,
	mcp.ToolName("gmail_get_auto_forwarding"):    getAutoForwarding,
	mcp.ToolName("gmail_update_auto_forwarding"): updateAutoForwarding,
	mcp.ToolName("gmail_get_imap"):               getImap,
	mcp.ToolName("gmail_update_imap"):            updateImap,
	mcp.ToolName("gmail_get_pop"):                getPop,
	mcp.ToolName("gmail_update_pop"):             updatePop,
	mcp.ToolName("gmail_get_language"):           getLanguage,
	mcp.ToolName("gmail_update_language"):        updateLanguage,

	// Filters
	mcp.ToolName("gmail_list_filters"):  listFilters,
	mcp.ToolName("gmail_get_filter"):    getFilter,
	mcp.ToolName("gmail_create_filter"): createFilter,
	mcp.ToolName("gmail_delete_filter"): deleteFilter,

	// Forwarding Addresses
	mcp.ToolName("gmail_list_forwarding_addresses"): listForwardingAddresses,
	mcp.ToolName("gmail_get_forwarding_address"):    getForwardingAddress,
	mcp.ToolName("gmail_create_forwarding_address"): createForwardingAddress,
	mcp.ToolName("gmail_delete_forwarding_address"): deleteForwardingAddress,

	// Send As
	mcp.ToolName("gmail_list_send_as"):   listSendAs,
	mcp.ToolName("gmail_get_send_as"):    getSendAs,
	mcp.ToolName("gmail_create_send_as"): createSendAs,
	mcp.ToolName("gmail_update_send_as"): updateSendAs,
	mcp.ToolName("gmail_delete_send_as"): deleteSendAs,
	mcp.ToolName("gmail_verify_send_as"): verifySendAs,

	// Delegates
	mcp.ToolName("gmail_list_delegates"):  listDelegates,
	mcp.ToolName("gmail_get_delegate"):    getDelegate,
	mcp.ToolName("gmail_create_delegate"): createDelegate,
	mcp.ToolName("gmail_delete_delegate"): deleteDelegate,
}
