package posthog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecordingRef(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantSession string
		wantProject string
		wantErr     bool
	}{
		{name: "bare id", in: "01a08ba9-0bbb-78d5-af50-c44b748a366e", wantSession: "01a08ba9-0bbb-78d5-af50-c44b748a366e"},
		{name: "replay url", in: "https://us.posthog.com/project/26958/replay/01a08ba9-0bbb-78d5-af50-c44b748a366e", wantSession: "01a08ba9-0bbb-78d5-af50-c44b748a366e", wantProject: "26958"},
		{name: "replay url with query", in: "https://us.posthog.com/project/26958/replay/01a08ba9-0bbb?t=42", wantSession: "01a08ba9-0bbb", wantProject: "26958"},
		{name: "shared replay query param", in: "https://us.posthog.com/project/26958/replay?sessionRecordingId=abc-123", wantSession: "abc-123", wantProject: "26958"},
		{name: "injection attempt", in: "abc' OR 1=1 --", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, project, err := parseRecordingRef(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSession, session)
			assert.Equal(t, tt.wantProject, project)
		})
	}
}

func TestGetSessionConsoleLogs_BuildsSafeHogQL(t *testing.T) {
	var gotQuery, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body struct {
			Query struct {
				Kind  string `json:"kind"`
				Query string `json:"query"`
			} `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "HogQLQuery", body.Query.Kind)
		gotQuery = body.Query.Query
		_, _ = w.Write([]byte(`{"columns":["timestamp","level","message"],"results":[["2026-09-10T14:14:46Z","error","[Apollo Client — GraphQL error] Path: bookDelivery"]]}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_get_session_console_logs", map[string]any{
		"session_id": "https://us.posthog.com/project/26958/replay/01a08ba9-0bbb-78d5-af50-c44b748a366e",
		"level":      "error, warn",
		"search":     "Apollo's",
		"after":      "2026-09-10T09:14:30-05:00",
		"before":     "2026-09-10T14:14:30.9Z",
		"limit":      "50",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "/api/projects/26958/query/", gotPath, "project comes from the replay URL")
	assert.Contains(t, result.Data, "bookDelivery")
	for _, want := range []string{
		"FROM console_logs_log_entries",
		"log_source = 'session_replay'",
		"log_source_id = '01a08ba9-0bbb-78d5-af50-c44b748a366e'",
		"level IN ('error', 'warn')",
		`positionCaseInsensitive(message, 'Apollo\'s') > 0`,
		"timestamp >= toDateTime64('2026-09-10 14:14:30.000000', 6, 'UTC')",
		"timestamp <= toDateTime64('2026-09-10 14:14:30.900000', 6, 'UTC')",
		"ORDER BY timestamp ASC",
		"LIMIT 50",
	} {
		assert.Contains(t, gotQuery, want)
	}
}

func TestGetSessionConsoleLogs_RejectsBadInput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to %s", r.URL.Path)
	}))
	defer ts.Close()
	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing session", args: map[string]any{}, want: "session_id"},
		{name: "bad level", args: map[string]any{"session_id": "abc", "level": "error') OR 1=1"}, want: "level"},
		{name: "bad timestamp", args: map[string]any{"session_id": "abc", "after": "yesterday"}, want: "after"},
		{name: "bad limit", args: map[string]any{"session_id": "abc", "limit": "5000"}, want: "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Execute(context.Background(), "posthog_get_session_console_logs", tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
}

func TestGetSessionConsoleLogs_DefaultsToAllLevelsAndLimit(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query struct {
				Query string `json:"query"`
			} `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotQuery = body.Query.Query
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()
	p := &posthog{apiKey: "token", projectID: "7", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_get_session_console_logs", map[string]any{"session_id": "abc-123"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.NotContains(t, gotQuery, "level IN")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(gotQuery), "LIMIT 200"), gotQuery)
}

func TestListSessionRecordings(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/projects/1/session_recordings/", r.URL.Path)
		assert.Equal(t, "person-uuid", r.URL.Query().Get("person_uuid"))
		assert.Equal(t, "2026-09-10T00:00:00Z", r.URL.Query().Get("date_from"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"results":[{"id":"abc","console_error_count":2,"start_url":"https://app.curri.com/book"}],"has_next":false}`))
	}))
	defer ts.Close()
	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_list_session_recordings", map[string]any{
		"person_id": "person-uuid",
		"date_from": "2026-09-10T00:00:00Z",
		"limit":     "10",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, "console_error_count")
}

func TestGetSessionRecording(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/projects/26958/session_recordings/01a08ba9-0bbb/", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"01a08ba9-0bbb","console_error_count":4}`))
	}))
	defer ts.Close()
	p := &posthog{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_get_session_recording", map[string]any{
		"session_id": "https://us.posthog.com/project/26958/replay/01a08ba9-0bbb",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, "console_error_count")
}
