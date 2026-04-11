package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const maxResponseSize = 512 << 10 // 512 KB — largest real response ~230KB, caps worst-case at ~125K tokens

var (
	_ mcp.Integration                = (*notion)(nil)
	_ mcp.FieldCompactionIntegration = (*notion)(nil)
	_ mcp.MarkdownIntegration        = (*notion)(nil)
)

type notion struct {
	tokenV2 string
	spaceID string
	userID  string
	baseURL string
	client  *http.Client
}

func New() mcp.Integration {
	return &notion{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxConnsPerHost: 10,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL: "https://www.notion.so",
	}
}

func (n *notion) Name() string { return "notion" }

func (n *notion) Configure(ctx context.Context, creds mcp.Credentials) error {
	n.tokenV2 = creds["token_v2"]
	if n.tokenV2 == "" {
		return fmt.Errorf("notion: token_v2 is required")
	}
	if v := creds["base_url"]; v != "" {
		n.baseURL = strings.TrimRight(v, "/")
	}

	spaceID, userID, err := n.resolveSpaceAndUser(ctx)
	if err != nil {
		return fmt.Errorf("notion: failed to resolve workspace: %w", err)
	}
	n.spaceID = spaceID
	n.userID = userID
	return nil
}

// resolveSpaceAndUser calls getSpaces to discover the first space ID and user ID.
func (n *notion) resolveSpaceAndUser(ctx context.Context) (string, string, error) {
	data, err := n.doRequest(ctx, "/api/v3/getSpaces", map[string]any{})
	if err != nil {
		return "", "", err
	}

	// getSpaces returns { "<user_id>": { "space": { "<space_id>": { "value": {...} } }, "notion_user": {...} } }
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return "", "", fmt.Errorf("parse getSpaces: %w", err)
	}

	// Sort user IDs for deterministic selection (Go map iteration is random)
	userIDs := make([]string, 0, len(top))
	for uid := range top {
		userIDs = append(userIDs, uid)
	}
	sort.Strings(userIDs)

	for _, userID := range userIDs {
		var tables struct {
			Space map[string]json.RawMessage `json:"space"`
		}
		if err := json.Unmarshal(top[userID], &tables); err != nil {
			continue
		}
		spaceIDs := make([]string, 0, len(tables.Space))
		for sid := range tables.Space {
			spaceIDs = append(spaceIDs, sid)
		}
		sort.Strings(spaceIDs)
		if len(spaceIDs) > 0 {
			return spaceIDs[0], userID, nil
		}
	}
	return "", "", fmt.Errorf("no spaces found")
}

func (n *notion) Healthy(ctx context.Context) bool {
	if n.client == nil || n.tokenV2 == "" {
		return false
	}
	_, err := n.doRequest(ctx, "/api/v3/getSpaces", map[string]any{})
	return err == nil
}

func (n *notion) Tools() []mcp.ToolDefinition {
	return tools
}

func (n *notion) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (n *notion) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, n, args)
}

// --- HTTP helpers ---

// doRequest sends a POST request to a v3 API endpoint with cookie auth.
// All v3 endpoints are POST — there are no GET/PATCH/DELETE methods.
func (n *notion) doRequest(ctx context.Context, path string, body any) (json.RawMessage, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "token_v2="+n.tokenV2)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: formatAPIError(resp.StatusCode, respData)}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, formatAPIError(resp.StatusCode, respData)
	}
	if resp.StatusCode == 204 || len(respData) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(respData), nil
}

const maxErrorLen = 500

// formatAPIError extracts a clean error from a Notion v3 error response.
// Prefers the structured {name, message} fields; falls back to truncated raw body.
func formatAPIError(statusCode int, body []byte) error {
	var errResp struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Name != "" {
		return fmt.Errorf("notion API error (%d): %s: %s", statusCode, errResp.Name, errResp.Message)
	}
	s := string(body)
	if len(s) > maxErrorLen {
		s = s[:maxErrorLen] + "... (truncated)"
	}
	return fmt.Errorf("notion API error (%d): %s", statusCode, s)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error)

// unmarshalJSON is a convenience wrapper that provides clearer error messages.
func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// notionPageURL builds the canonical Notion URL for a page ID.
func notionPageURL(id string) string {
	return "https://www.notion.so/" + strings.ReplaceAll(id, "-", "")
}

// --- Write helpers ---

// resolveParent extracts parent ID and table from a parent arg object.
func resolveParent(parent map[string]any) (string, string, error) {
	pid, err := mcp.ArgStr(parent, "page_id")
	if err != nil {
		return "", "", err
	}
	if pid != "" {
		return pid, "block", nil
	}
	dbid, err := mcp.ArgStr(parent, "database_id")
	if err != nil {
		return "", "", err
	}
	if dbid != "" {
		return dbid, "collection", nil
	}
	return "", "", nil
}

// currentTimeMillis returns the current time in milliseconds (Notion v3 format).
func currentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

// buildChildBlockOps creates operations for a single child block under a parent.
func buildChildBlockOps(n *notion, parentID string, child map[string]any, now int64) []op {
	childID := newBlockID()
	childType, _ := child["type"].(string)
	if childType == "" {
		childType = "text"
	}

	blockData := map[string]any{
		"id":                   childID,
		"type":                 childType,
		"parent_id":            parentID,
		"parent_table":         "block",
		"space_id":             n.spaceID,
		"created_by_id":        n.userID,
		"created_by_table":     "notion_user",
		"last_edited_by_id":    n.userID,
		"last_edited_by_table": "notion_user",
		"alive":                true,
		"created_time":         now,
		"last_edited_time":     now,
	}

	if props, ok := child["properties"].(map[string]any); ok {
		blockData["properties"] = props
	}
	if format, ok := child["format"].(map[string]any); ok {
		blockData["format"] = format
	}

	return []op{
		buildSetOp("block", childID, []string{}, blockData),
		buildUpdateOp("block", childID, []string{}, map[string]any{
			"last_edited_time": now,
		}),
		buildListAfterOp("block", parentID, []string{"content"}, map[string]any{
			"id": childID,
		}),
	}
}
