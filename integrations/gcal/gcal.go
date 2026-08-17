// Package gcal is the Switchboard adapter for Google Calendar v3.
//
// It mirrors the structure of the gmail adapter: raw HTTP against
// calendar.googleapis.com using a Bearer token, with OAuth refresh handled
// transparently on 401 responses. The OAuth flow itself lives in the shared
// googleoauth package.
package gcal

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

var compactResult = compact.MustLoadWithOverlay("gcal", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes
var viewSets = compactResult.Views

type gcal struct {
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
	_ mcp.FieldCompactionIntegration = (*gcal)(nil)
	_ mcp.PlainTextCredentials       = (*gcal)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gcal)(nil)
	_ compact.ToolViewsIntegration   = (*gcal)(nil)
)

func (g *gcal) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gcal integration.
func New() mcp.Integration {
	return &gcal{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://www.googleapis.com/calendar/v3",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gcal); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gcal) Name() string { return "gcal" }

func (g *gcal) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gcal: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (g *gcal) Healthy(ctx context.Context) bool {
	_, err := g.get(ctx, "/users/me/calendarList?maxResults=1")
	return err == nil
}

func (g *gcal) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gcal) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gcal) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gcal) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *gcal) Views(toolName mcp.ToolName) (compact.ViewSet, bool) {
	vs, ok := viewSets[toolName]
	return vs, ok
}

// --- HTTP helpers ---

func (g *gcal) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, path, body, true)
}

func (g *gcal) doRequestInner(ctx context.Context, method, path string, body any, canRetry bool) (json.RawMessage, error) {
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

		newToken, rerr := RefreshAccessToken(ctx, g.clientID, g.clientSecret, g.refreshToken)
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gcal API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gcal API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gcal) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gcal) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gcal) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PATCH", path, body)
}

func (g *gcal) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PUT", path, body)
}

func (g *gcal) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gcal) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gcal")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gcal", ic)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error)

// --- Argument helpers ---

// calendarID returns the calendar_id arg, falling back to "primary" — the
// alias for the authenticated user's primary calendar. Most LLM-driven
// workflows want the primary calendar by default, so this default keeps
// the common case zero-arg.
func calendarID(r *mcp.Args) string {
	if v := r.Str("calendar_id"); v != "" {
		return v
	}
	return "primary"
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

// pathEscape percent-encodes a path segment for use as a calendar ID or
// rule ID. Calendar IDs can be email addresses ("alice@example.com") or
// contain '#' (group calendars like "en.usa#holiday@group.v.calendar.google.com").
// Go's url.PathEscape leaves '@' alone (RFC 3986 pchar), but the Calendar
// API expects it percent-encoded, so we encode it explicitly here.
func pathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "@", "%40")
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Events
	mcp.ToolName("gcal_list_events"):     listEvents,
	mcp.ToolName("gcal_get_event"):       getEvent,
	mcp.ToolName("gcal_create_event"):    createEvent,
	mcp.ToolName("gcal_update_event"):    updateEvent,
	mcp.ToolName("gcal_patch_event"):     patchEvent,
	mcp.ToolName("gcal_delete_event"):    deleteEvent,
	mcp.ToolName("gcal_move_event"):      moveEvent,
	mcp.ToolName("gcal_list_instances"):  listEventInstances,
	mcp.ToolName("gcal_quick_add_event"): quickAddEvent,
	mcp.ToolName("gcal_import_event"):    importEvent,

	// CalendarList (the user's subscribed calendars)
	mcp.ToolName("gcal_list_calendars"):               listCalendarList,
	mcp.ToolName("gcal_get_calendar_subscription"):    getCalendarListEntry,
	mcp.ToolName("gcal_subscribe_calendar"):           insertCalendarList,
	mcp.ToolName("gcal_update_calendar_subscription"): updateCalendarList,
	mcp.ToolName("gcal_unsubscribe_calendar"):         deleteCalendarList,

	// Calendars (the metadata of the underlying calendar resource)
	mcp.ToolName("gcal_get_calendar"):    getCalendar,
	mcp.ToolName("gcal_create_calendar"): createCalendar,
	mcp.ToolName("gcal_update_calendar"): updateCalendar,
	mcp.ToolName("gcal_delete_calendar"): deleteCalendar,
	mcp.ToolName("gcal_clear_calendar"):  clearCalendar,

	// ACL (sharing rules)
	mcp.ToolName("gcal_list_acl"):   listACL,
	mcp.ToolName("gcal_get_acl"):    getACL,
	mcp.ToolName("gcal_create_acl"): createACL,
	mcp.ToolName("gcal_update_acl"): updateACL,
	mcp.ToolName("gcal_delete_acl"): deleteACL,

	// Freebusy
	mcp.ToolName("gcal_query_freebusy"): queryFreebusy,

	// Settings + colors
	mcp.ToolName("gcal_list_settings"): listSettings,
	mcp.ToolName("gcal_get_setting"):   getSetting,
	mcp.ToolName("gcal_get_colors"):    getColors,
}
