package gmail

import (
	"bytes"
	"context"
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

type gmail struct {
	accessToken string
	client      *http.Client
	baseURL     string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &gmail{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://gmail.googleapis.com",
	}
}

func (g *gmail) Name() string { return "gmail" }

func (g *gmail) Configure(creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
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

func (g *gmail) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

// --- HTTP helpers ---

func (g *gmail) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
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

// --- Result helpers ---

type handlerFunc func(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func parseJSON(args map[string]any, key string) (any, error) {
	v := argStr(args, key)
	if v == "" {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON for %s: %w", key, err)
	}
	return out, nil
}

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

func argStrSlice(args map[string]any, key string) []string {
	v := argStr(args, key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
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

// user returns the userId from args, falling back to "me".
func user(args map[string]any) string {
	if v := argStr(args, "user_id"); v != "" {
		return v
	}
	return "me"
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Profile
	"gmail_get_profile": getProfile,

	// Messages
	"gmail_list_messages":       listMessages,
	"gmail_get_message":         getMessage,
	"gmail_send_message":        sendMessage,
	"gmail_delete_message":      deleteMessage,
	"gmail_trash_message":       trashMessage,
	"gmail_untrash_message":     untrashMessage,
	"gmail_modify_message":      modifyMessage,
	"gmail_batch_modify":        batchModifyMessages,
	"gmail_batch_delete":        batchDeleteMessages,
	"gmail_get_attachment":      getAttachment,

	// Threads
	"gmail_list_threads":    listThreads,
	"gmail_get_thread":      getThread,
	"gmail_delete_thread":   deleteThread,
	"gmail_trash_thread":    trashThread,
	"gmail_untrash_thread":  untrashThread,
	"gmail_modify_thread":   modifyThread,

	// Labels
	"gmail_list_labels":  listLabels,
	"gmail_get_label":    getLabel,
	"gmail_create_label": createLabel,
	"gmail_update_label": updateLabel,
	"gmail_delete_label": deleteLabel,

	// Drafts
	"gmail_list_drafts":  listDrafts,
	"gmail_get_draft":    getDraft,
	"gmail_create_draft": createDraft,
	"gmail_update_draft": updateDraft,
	"gmail_delete_draft": deleteDraft,
	"gmail_send_draft":   sendDraft,

	// History
	"gmail_list_history": listHistory,

	// Settings
	"gmail_get_vacation":    getVacation,
	"gmail_update_vacation": updateVacation,
	"gmail_get_auto_forwarding":    getAutoForwarding,
	"gmail_update_auto_forwarding": updateAutoForwarding,
	"gmail_get_imap":       getImap,
	"gmail_update_imap":    updateImap,
	"gmail_get_pop":        getPop,
	"gmail_update_pop":     updatePop,
	"gmail_get_language":   getLanguage,
	"gmail_update_language": updateLanguage,

	// Filters
	"gmail_list_filters":  listFilters,
	"gmail_get_filter":    getFilter,
	"gmail_create_filter": createFilter,
	"gmail_delete_filter": deleteFilter,

	// Forwarding Addresses
	"gmail_list_forwarding_addresses":  listForwardingAddresses,
	"gmail_get_forwarding_address":     getForwardingAddress,
	"gmail_create_forwarding_address":  createForwardingAddress,
	"gmail_delete_forwarding_address":  deleteForwardingAddress,

	// Send As
	"gmail_list_send_as":  listSendAs,
	"gmail_get_send_as":   getSendAs,
	"gmail_create_send_as": createSendAs,
	"gmail_update_send_as": updateSendAs,
	"gmail_delete_send_as": deleteSendAs,
	"gmail_verify_send_as": verifySendAs,

	// Delegates
	"gmail_list_delegates":  listDelegates,
	"gmail_get_delegate":    getDelegate,
	"gmail_create_delegate": createDelegate,
	"gmail_delete_delegate": deleteDelegate,
}
