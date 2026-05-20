package gcal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/googleoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "gcal", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "ya29.test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	g := &gcal{client: &http.Client{}, baseURL: "https://www.googleapis.com/calendar/v3"}
	err := g.Configure(context.Background(), mcp.Credentials{
		"access_token": "ya29.test",
		"base_url":     "https://custom.example.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com", g.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGcalPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gcal_", "tool %s missing gcal_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	g := &gcal{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gcal_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- helper tests ---

func TestCalendarID_Default(t *testing.T) {
	assert.Equal(t, "primary", calendarID(mcp.NewArgs(map[string]any{})))
	assert.Equal(t, "secondary@group.calendar.google.com", calendarID(mcp.NewArgs(map[string]any{"calendar_id": "secondary@group.calendar.google.com"})))
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty=")
		assert.True(t, result[0] == '?')
	})
	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestPathEscape(t *testing.T) {
	assert.Equal(t, "primary", pathEscape("primary"))
	assert.Equal(t, "alice%40example.com", pathEscape("alice@example.com"))
	assert.Equal(t, "en.usa%23holiday%40group.v.calendar.google.com", pathEscape("en.usa#holiday@group.v.calendar.google.com"))
}

func TestEventTime(t *testing.T) {
	t.Run("all-day", func(t *testing.T) {
		got := eventTime("2024-03-15", "America/Los_Angeles")
		assert.Equal(t, map[string]any{"date": "2024-03-15"}, got)
	})
	t.Run("datetime with tz", func(t *testing.T) {
		got := eventTime("2024-03-15T14:00:00-07:00", "America/Los_Angeles")
		assert.Equal(t, "2024-03-15T14:00:00-07:00", got["dateTime"])
		assert.Equal(t, "America/Los_Angeles", got["timeZone"])
	})
	t.Run("datetime no tz", func(t *testing.T) {
		got := eventTime("2024-03-15T14:00:00Z", "")
		assert.Equal(t, "2024-03-15T14:00:00Z", got["dateTime"])
		_, hasTZ := got["timeZone"]
		assert.False(t, hasTZ)
	})
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"summary":"My Calendar"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "My Calendar")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"message":"Forbidden"}}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "bad-token", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gcal API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

// --- handler integration tests ---

func TestListEvents(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/calendars/primary/events")
		assert.Equal(t, "standup", r.URL.Query().Get("q"))
		_, _ = w.Write([]byte(`{"items":[{"id":"ev1","summary":"Daily standup"}]}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_list_events", map[string]any{
		"q": "standup",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ev1")
}

func TestListEvents_NonPrimaryCalendar(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// alice@example.com should be percent-encoded in the request line
		assert.Contains(t, r.RequestURI, "alice%40example.com/events")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_list_events", map[string]any{
		"calendar_id": "alice@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/calendars/primary/events/ev123")
		_, _ = w.Write([]byte(`{"id":"ev123","summary":"Meeting"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_get_event", map[string]any{
		"event_id": "ev123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ev123")
}

func TestCreateEvent_SimpleFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/calendars/primary/events")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Standup", body["summary"])
		start, _ := body["start"].(map[string]any)
		require.NotNil(t, start)
		assert.Equal(t, "2024-03-15T14:00:00Z", start["dateTime"])
		attendees, _ := body["attendees"].([]any)
		assert.Len(t, attendees, 2)
		_, _ = w.Write([]byte(`{"id":"ev_new"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_create_event", map[string]any{
		"summary":   "Standup",
		"start":     "2024-03-15T14:00:00Z",
		"end":       "2024-03-15T14:30:00Z",
		"attendees": "alice@example.com,bob@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ev_new")
}

func TestCreateEvent_AllDay(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		start, _ := body["start"].(map[string]any)
		require.NotNil(t, start)
		assert.Equal(t, "2024-03-15", start["date"])
		_, hasDT := start["dateTime"]
		assert.False(t, hasDT)
		_, _ = w.Write([]byte(`{"id":"ev_allday"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_create_event", map[string]any{
		"summary": "Holiday",
		"start":   "2024-03-15",
		"end":     "2024-03-16",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// TestCreateEvent_MeetRequestIDUnique covers a Google Calendar gotcha:
// `conferenceData.createRequest.requestId` is an idempotency key — two
// events sharing the same id silently reuse the first event's Meet link
// instead of provisioning a new room. Each create_meet=true call must
// produce a fresh id.
func TestCreateEvent_MeetRequestIDUnique(t *testing.T) {
	var seenIDs []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		cd, _ := body["conferenceData"].(map[string]any)
		require.NotNil(t, cd, "conferenceData must be set when create_meet=true")
		req, _ := cd["createRequest"].(map[string]any)
		require.NotNil(t, req)
		id, _ := req["requestId"].(string)
		seenIDs = append(seenIDs, id)
		_, _ = w.Write([]byte(`{"id":"ev"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	for range 3 {
		_, err := g.Execute(context.Background(), "gcal_create_event", map[string]any{
			"summary":     "Standup", // same summary across calls — must NOT collide
			"start":       "2024-03-15T14:00:00Z",
			"end":         "2024-03-15T14:30:00Z",
			"create_meet": "true",
		})
		require.NoError(t, err)
	}
	require.Len(t, seenIDs, 3)
	for _, id := range seenIDs {
		assert.True(t, strings.HasPrefix(id, "switchboard-"), "requestId should be prefixed: %q", id)
		assert.Greater(t, len(id), len("switchboard-"), "requestId should have a unique suffix: %q", id)
	}
	uniq := map[string]struct{}{}
	for _, id := range seenIDs {
		uniq[id] = struct{}{}
	}
	assert.Equal(t, 3, len(uniq), "every requestId must be unique across calls: %v", seenIDs)
}

func TestCreateEvent_RawBodyOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// summary from body field, not the convenience arg
		assert.Equal(t, "From Body", body["summary"])
		assert.NotContains(t, body, "description")
		_, _ = w.Write([]byte(`{"id":"ev_raw"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_create_event", map[string]any{
		"summary":     "Ignored",
		"description": "Also ignored",
		"body":        `{"summary":"From Body"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestPatchEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/calendars/primary/events/ev1")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Renamed", body["summary"])
		_, _ = w.Write([]byte(`{"id":"ev1","summary":"Renamed"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_patch_event", map[string]any{
		"event_id": "ev1",
		"summary":  "Renamed",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Renamed")
}

func TestDeleteEvent_WithSendUpdates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "all", r.URL.Query().Get("sendUpdates"))
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_delete_event", map[string]any{
		"event_id":     "ev1",
		"send_updates": "all",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestMoveEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/calendars/primary/events/ev1/move")
		assert.Equal(t, "dest@example.com", r.URL.Query().Get("destination"))
		_, _ = w.Write([]byte(`{"id":"ev1"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_move_event", map[string]any{
		"event_id":    "ev1",
		"destination": "dest@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQuickAddEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/calendars/primary/events/quickAdd")
		assert.Equal(t, "Lunch Friday 1pm", r.URL.Query().Get("text"))
		_, _ = w.Write([]byte(`{"id":"ev_quick"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_quick_add_event", map[string]any{
		"text": "Lunch Friday 1pm",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListCalendars(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me/calendarList")
		_, _ = w.Write([]byte(`{"items":[{"id":"primary"}]}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_list_calendars", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "primary")
}

func TestCreateCalendar(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/calendars")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Engineering", body["summary"])
		assert.Equal(t, "America/Los_Angeles", body["timeZone"])
		_, _ = w.Write([]byte(`{"id":"newcal@group.calendar.google.com"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_create_calendar", map[string]any{
		"summary":   "Engineering",
		"time_zone": "America/Los_Angeles",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListACL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/calendars/primary/acl")
		_, _ = w.Write([]byte(`{"items":[{"id":"user:alice@example.com","role":"writer"}]}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_list_acl", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "writer")
}

func TestCreateACL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "writer", body["role"])
		scope, _ := body["scope"].(map[string]any)
		require.NotNil(t, scope)
		assert.Equal(t, "user", scope["type"])
		assert.Equal(t, "alice@example.com", scope["value"])
		_, _ = w.Write([]byte(`{"id":"user:alice@example.com"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_create_acl", map[string]any{
		"role":        "writer",
		"scope_type":  "user",
		"scope_value": "alice@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryFreebusy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/freeBusy")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "2024-03-15T00:00:00Z", body["timeMin"])
		items, _ := body["items"].([]any)
		assert.Len(t, items, 2)
		_, _ = w.Write([]byte(`{"calendars":{"primary":{"busy":[]}}}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_query_freebusy", map[string]any{
		"time_min": "2024-03-15T00:00:00Z",
		"time_max": "2024-03-16T00:00:00Z",
		"items":    "primary,alice@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetColors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/colors")
		_, _ = w.Write([]byte(`{"calendar":{},"event":{}}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_get_colors", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestSubscribeCalendar(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me/calendarList")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "shared@group.calendar.google.com", body["id"])
		_, _ = w.Write([]byte(`{"id":"shared@group.calendar.google.com"}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gcal_subscribe_calendar", map[string]any{
		"calendar_id": "shared@group.calendar.google.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestImportEvent_InvalidJSON(t *testing.T) {
	g := &gcal{accessToken: "token", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gcal_import_event", map[string]any{
		"body": "{bad json}",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for body")
}

func TestUpdateEvent_RequiresBody(t *testing.T) {
	g := &gcal{accessToken: "token", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gcal_update_event", map[string]any{
		"event_id": "ev1",
	})
	require.NoError(t, err)
	// Empty body string fails JSON unmarshal
	assert.True(t, result.IsError)
}

// --- result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"id":"ev1"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"id":"ev1"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// ── Healthy() ───────────────────────────────────────────────────────

func TestHealthy_TrueOn200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"kind":"calendar#calendarList","items":[]}`))
	}))
	defer ts.Close()

	g := &gcal{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gcal{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, g.Healthy(context.Background()))
}

// TestHealthy_TrueAfterRefresh verifies that an expired access token does
// not flip the health badge red so long as refresh credentials are
// configured: the 401 from the API should trigger a transparent refresh
// through g.get() and the retried call should succeed.
func TestHealthy_TrueAfterRefresh(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		assert.Equal(t, "refresh_token", vals.Get("grant_type"))
		assert.Equal(t, "rtok", vals.Get("refresh_token"))
		_, _ = w.Write([]byte(`{"access_token":"new-token","expires_in":3600}`))
	}))
	defer tokenSrv.Close()
	googleoauth.SetTokenURLForTest(t, tokenSrv.URL)

	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"kind":"calendar#calendarList","items":[]}`))
	}))
	defer api.Close()

	g := &gcal{
		accessToken:  "expired",
		refreshToken: "rtok",
		clientID:     "cid",
		clientSecret: "csec",
		client:       api.Client(),
		baseURL:      api.URL,
	}
	assert.True(t, g.Healthy(context.Background()))
	assert.Equal(t, "new-token", g.accessToken)
	assert.Equal(t, 2, calls, "expected initial 401 + retried 200")
}
