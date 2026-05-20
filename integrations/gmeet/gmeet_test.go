package gmeet

import (
	"context"
	"encoding/json"
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

// ── Constructor / config ────────────────────────────────────────────

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "gmeet", i.Name())
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
	g := &gmeet{client: &http.Client{}, baseURL: "https://meet.googleapis.com/v2"}
	err := g.Configure(context.Background(), mcp.Credentials{
		"access_token": "ya29.test",
		"base_url":     "https://custom.example.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com", g.baseURL)
}

// ── Tools metadata ──────────────────────────────────────────────────

func TestTools(t *testing.T) {
	i := New()
	defs := i.Tools()
	assert.NotEmpty(t, defs)
	for _, tool := range defs {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGmeetPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gmeet_", "tool %s missing gmeet_ prefix", tool.Name)
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

func TestTools_EntryPointHasStartHere(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		if tool.Name == mcp.ToolName("gmeet_create_space") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gmeet_create_space must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gmeet_create_space tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gmeet{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gmeet_nonexistent", nil)
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

// ── Compaction parity ──────────────────────────────────────────────

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", name)
	}
}

// ── HTTP helpers ────────────────────────────────────────────────────

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"name":"spaces/abc"}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/spaces/abc")
	require.NoError(t, err)
	assert.Contains(t, string(data), "spaces/abc")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/spaces/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gmeet API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.post(context.Background(), "/spaces/abc:endActiveConference", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_5xxIsRetryable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":{"message":"Service Unavailable"}}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/conferenceRecords")
	require.Error(t, err)
	re, ok := err.(*mcp.RetryableError)
	require.True(t, ok, "expected mcp.RetryableError")
	assert.Equal(t, 503, re.StatusCode)
}

// ── Handler: createSpace ────────────────────────────────────────────

func TestCreateSpace_NoConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/spaces", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// No config -> empty body.
		_, hasConfig := body["config"]
		assert.False(t, hasConfig)
		_, _ = w.Write([]byte(`{"name":"spaces/abc","meetingUri":"https://meet.google.com/abc-defg-hij","meetingCode":"abc-defg-hij"}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_create_space", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "spaces/abc")
}

func TestCreateSpace_WithConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		cfg, ok := body["config"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "TRUSTED", cfg["accessType"])
		_, _ = w.Write([]byte(`{"name":"spaces/xyz"}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_create_space", map[string]any{
		"config": `{"accessType":"TRUSTED","moderation":"ON"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateSpace_InvalidConfig(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gmeet_create_space", map[string]any{
		"config": "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON")
}

// ── Handler: getSpace ───────────────────────────────────────────────

func TestGetSpace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spaces/abc", r.URL.Path)
		_, _ = w.Write([]byte(`{"name":"spaces/abc"}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_get_space", map[string]any{
		"name": "abc",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetSpace_AcceptsResourceName(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = w.Write([]byte(`{"name":"spaces/abc"}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gmeet_get_space", map[string]any{
		"name": "spaces/abc",
	})
	require.NoError(t, err)
	assert.Equal(t, "/spaces/abc", seenPath)
}

func TestGetSpace_AcceptsMeetingCode(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gmeet_get_space", map[string]any{
		"name": "abc-defg-hij",
	})
	require.NoError(t, err)
	// Meeting code is passed through as part of the spaces/{name} path.
	assert.Equal(t, "/spaces/abc-defg-hij", seenPath)
}

func TestGetSpace_MissingName(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gmeet_get_space", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "name is required")
}

// ── Handler: updateSpace ────────────────────────────────────────────

func TestUpdateSpace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/spaces/abc", r.URL.Path)
		assert.Equal(t, "config.accessType,config.moderation", r.URL.Query().Get("updateMask"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		cfg, ok := body["config"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "RESTRICTED", cfg["accessType"])
		_, _ = w.Write([]byte(`{"name":"spaces/abc"}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_update_space", map[string]any{
		"name":        "spaces/abc",
		"update_mask": "config.accessType,config.moderation",
		"space": map[string]any{
			"config": map[string]any{"accessType": "RESTRICTED", "moderation": "ON"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateSpace_MissingArgs(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}

	r1, _ := g.Execute(context.Background(), "gmeet_update_space", map[string]any{
		"update_mask": "config.accessType",
		"space":       map[string]any{"config": map[string]any{}},
	})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "name is required")

	r2, _ := g.Execute(context.Background(), "gmeet_update_space", map[string]any{
		"name":  "spaces/abc",
		"space": map[string]any{"config": map[string]any{}},
	})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "update_mask is required")

	r3, _ := g.Execute(context.Background(), "gmeet_update_space", map[string]any{
		"name":        "spaces/abc",
		"update_mask": "config.accessType",
	})
	assert.True(t, r3.IsError)
	assert.Contains(t, r3.Data, "space is required")
}

// ── Handler: endActiveConference ────────────────────────────────────

func TestEndActiveConference(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/spaces/abc:endActiveConference", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_end_active_conference", map[string]any{
		"name": "abc",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestEndActiveConference_MissingName(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gmeet_end_active_conference", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "name is required")
}

// ── Handler: listConferenceRecords ──────────────────────────────────

func TestListConferenceRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/conferenceRecords", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, `space.meeting_code = "abc-defg-hij"`, q.Get("filter"))
		assert.Equal(t, "20", q.Get("pageSize"))
		assert.Equal(t, "tok-1", q.Get("pageToken"))
		_, _ = w.Write([]byte(`{"conferenceRecords":[]}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_list_conference_records", map[string]any{
		"filter":     `space.meeting_code = "abc-defg-hij"`,
		"page_size":  20,
		"page_token": "tok-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListConferenceRecords_NoFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/conferenceRecords", r.URL.Path)
		assert.Empty(t, r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"conferenceRecords":[]}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gmeet_list_conference_records", map[string]any{})
	require.NoError(t, err)
}

// ── Handler: getConferenceRecord ────────────────────────────────────

func TestGetConferenceRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/conferenceRecords/cr-1", r.URL.Path)
		_, _ = w.Write([]byte(`{"name":"conferenceRecords/cr-1"}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_get_conference_record", map[string]any{
		"name": "cr-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetConferenceRecord_MissingName(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gmeet_get_conference_record", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "name is required")
}

// ── Handler: listParticipants ───────────────────────────────────────

func TestListParticipants(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/conferenceRecords/cr-1/participants", r.URL.Path)
		_, _ = w.Write([]byte(`{"participants":[]}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_list_participants", map[string]any{
		"conference_record": "cr-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListParticipants_MissingConferenceRecord(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gmeet_list_participants", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "conference_record is required")
}

// ── Handler: listRecordings ─────────────────────────────────────────

func TestListRecordings(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/conferenceRecords/cr-1/recordings", r.URL.Path)
		_, _ = w.Write([]byte(`{"recordings":[]}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_list_recordings", map[string]any{
		"conference_record": "cr-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListRecordings_MissingConferenceRecord(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gmeet_list_recordings", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ── Handler: listTranscripts ────────────────────────────────────────

func TestListTranscripts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/conferenceRecords/cr-1/transcripts", r.URL.Path)
		_, _ = w.Write([]byte(`{"transcripts":[]}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_list_transcripts", map[string]any{
		"conference_record": "cr-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: listTranscriptEntries ──────────────────────────────────

func TestListTranscriptEntries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/conferenceRecords/cr-1/transcripts/t-1/entries", r.URL.Path)
		_, _ = w.Write([]byte(`{"transcriptEntries":[]}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gmeet_list_transcript_entries", map[string]any{
		"transcript": "conferenceRecords/cr-1/transcripts/t-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListTranscriptEntries_MissingTranscript(t *testing.T) {
	g := &gmeet{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gmeet_list_transcript_entries", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "transcript is required")
}

// ── Healthy ────────────────────────────────────────────────────────

func TestHealthy_TrueOn200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"conferenceRecords":[]}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
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
		_, _ = w.Write([]byte(`{"conferenceRecords":[]}`))
	}))
	defer api.Close()

	g := &gmeet{
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

// ── Path escaping ───────────────────────────────────────────────────

func TestSpaceNameIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gmeet{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gmeet_get_space", map[string]any{
		"name": "id with space",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "id%20with%20space") || strings.Contains(seenPath, "id+with+space"),
		"space name with space should be URL-escaped; got %s", seenPath)
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestNormalizeSpaceName(t *testing.T) {
	assert.Equal(t, "spaces/abc", normalizeSpaceName("abc"))
	assert.Equal(t, "spaces/abc", normalizeSpaceName("spaces/abc"))
	assert.Equal(t, "spaces/abc-defg-hij", normalizeSpaceName("abc-defg-hij"))
	assert.Equal(t, "", normalizeSpaceName(""))
	assert.Equal(t, "spaces/abc", normalizeSpaceName("  abc  "))
}

func TestNormalizeConferenceRecord(t *testing.T) {
	assert.Equal(t, "conferenceRecords/cr-1", normalizeConferenceRecord("cr-1"))
	assert.Equal(t, "conferenceRecords/cr-1", normalizeConferenceRecord("conferenceRecords/cr-1"))
	assert.Equal(t, "", normalizeConferenceRecord(""))
}

func TestParseObject_String(t *testing.T) {
	got, err := parseObject(`{"accessType":"OPEN"}`, "config")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "OPEN", got["accessType"])
}

func TestParseObject_Invalid(t *testing.T) {
	_, err := parseObject("not-json", "config")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestParseObject_EmptyString(t *testing.T) {
	got, err := parseObject("", "config")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseObject_Nil(t *testing.T) {
	got, err := parseObject(nil, "config")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseObject_AlreadyParsed(t *testing.T) {
	in := map[string]any{"accessType": "TRUSTED"}
	got, err := parseObject(in, "config")
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

func TestParseObject_EmptyMap(t *testing.T) {
	got, err := parseObject(map[string]any{}, "config")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseObject_WrongType(t *testing.T) {
	_, err := parseObject(123, "config")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected JSON object or string")
}

// ── PlainTextKeys ──────────────────────────────────────────────────

func TestPlainTextKeys(t *testing.T) {
	g := &gmeet{}
	keys := g.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
}
