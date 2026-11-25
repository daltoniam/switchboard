package ashby

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureAPIKey = "fixture-api-key"

type apiCall struct {
	path   string
	body   map[string]any
	accept string
}

// apiFixture is a fake Ashby REST API. Like the real one it answers POST
// /<endpoint> with HTTP Basic auth (the key as username, empty password):
// 401 without credentials, 403 for any other key.
type apiFixture struct {
	server    *httptest.Server
	mu        sync.Mutex
	calls     []apiCall
	responses map[string]string
}

func newAPIFixture(t *testing.T, responses map[string]string) *apiFixture {
	t.Helper()
	f := &apiFixture{responses: responses}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user == "" {
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}
		if user != fixtureAPIKey || password != "" {
			http.Error(w, "Invalid or deactivated API key", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var body map[string]any
		data, err := io.ReadAll(r.Body)
		if err != nil || json.Unmarshal(data, &body) != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		f.mu.Lock()
		f.calls = append(f.calls, apiCall{path: path, body: body, accept: r.Header.Get("Accept")})
		response, found := f.responses[path]
		f.mu.Unlock()
		if !found {
			response = `{"success":true,"results":{"endpoint":"` + path + `"}}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *apiFixture) snapshot() []apiCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]apiCall(nil), f.calls...)
}

func configuredAPI(t *testing.T, baseURL, key string) mcp.Integration {
	t.Helper()
	integration := newIntegration(t)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"api_key": key, "api_base_url": baseURL}))
	return integration
}

func TestConfigure_APIKeyMode(t *testing.T) {
	tests := []struct {
		name    string
		creds   mcp.Credentials
		wantErr string
		wantURL string
	}{
		{name: "default url", creds: mcp.Credentials{"api_key": "k"}, wantURL: defaultAPIBaseURL},
		{name: "trims key and url", creds: mcp.Credentials{"api_key": " k ", "api_base_url": " https://example.test/ "}, wantURL: "https://example.test"},
		{name: "bad scheme", creds: mcp.Credentials{"api_key": "k", "api_base_url": "ftp://api.ashbyhq.com"}, wantErr: "invalid api_base_url"},
		{name: "no host", creds: mcp.Credentials{"api_key": "k", "api_base_url": "https://"}, wantErr: "invalid api_base_url"},
		{name: "api key wins over a saved sign-in", creds: mcp.Credentials{"api_key": "k", "mcp_access_token": "t"}, wantURL: defaultAPIBaseURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := newIntegration(t)
			err := integration.Configure(t.Context(), tt.creds)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			a := integration.(*ashby)
			require.NotNil(t, a.native)
			assert.Equal(t, tt.wantURL, a.native.baseURL)
			assert.Equal(t, "k", a.native.apiKey)
			assert.Nil(t, a.remote, "API-key mode must not hold a hosted MCP session")
			assert.Len(t, integration.Tools(), len(nativeEndpoints))
		})
	}
}

func TestConfigure_SwitchesBetweenModes(t *testing.T) {
	hosted := newHostedFixture(t, nil)
	integration := newIntegration(t)

	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"api_key": "k"}))
	assert.Len(t, integration.Tools(), len(nativeEndpoints))

	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": hosted.server.URL, "mcp_access_token": fixtureToken}))
	a := integration.(*ashby)
	assert.Nil(t, a.native, "signing in replaces API-key mode")
	assert.Len(t, integration.Tools(), len(dispatch))
	assert.True(t, integration.Healthy(t.Context()))

	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"api_key": "k"}))
	assert.Nil(t, a.remote, "API-key mode closes the hosted session")
	assert.Len(t, integration.Tools(), len(nativeEndpoints))
}

func TestConfigure_RequiresACredential(t *testing.T) {
	err := newIntegration(t).Configure(t.Context(), mcp.Credentials{"api_key": "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
	assert.Contains(t, err.Error(), "mcp_access_token")
}

func TestNative_BasicAuthAndHeaders(t *testing.T) {
	fixture := newAPIFixture(t, map[string]string{
		"candidate.search": `{"success":true,"results":[{"id":"cand-1","name":"Jordan Lee"}],"moreDataAvailable":false}`,
	})
	integration := configuredAPI(t, fixture.server.URL, fixtureAPIKey)
	result, err := integration.Execute(t.Context(), "ashby_search_candidates", map[string]any{"name": "Jordan"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.JSONEq(t, `{"results":[{"id":"cand-1","name":"Jordan Lee"}],"moreDataAvailable":false,"nextCursor":null,"syncToken":null}`, result.Data)
	calls := fixture.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "candidate.search", calls[0].path)
	assert.Equal(t, map[string]any{"name": "Jordan"}, calls[0].body)
	assert.Equal(t, "application/json; version=1", calls[0].accept)
}

func TestNative_AuthFailures(t *testing.T) {
	fixture := newAPIFixture(t, nil)
	tests := []struct {
		name string
		key  string
		want []string
	}{
		{name: "wrong or deactivated key", key: "wrong-key", want: []string{"403", "Invalid or deactivated API key", "Admin"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := configuredAPI(t, fixture.server.URL, tt.key)
			assert.False(t, integration.Healthy(t.Context()))
			result, err := integration.Execute(t.Context(), "ashby_list_jobs", nil)
			require.NoError(t, err)
			require.True(t, result.IsError)
			for _, want := range tt.want {
				assert.Contains(t, result.Data, want)
			}
			assert.NotContains(t, result.Data, tt.key, "errors must not echo the API key")
		})
	}
}

func TestNative_MissingKeyIs401(t *testing.T) {
	fixture := newAPIFixture(t, nil)
	client := newNativeClient("", fixture.server.URL)
	_, err := client.call(t.Context(), "job.list", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "Missing API key")
}

func TestNative_HealthyCallsAPIKeyInfo(t *testing.T) {
	fixture := newAPIFixture(t, map[string]string{"apiKey.info": `{"success":true,"results":{"title":"Switchboard","scopes":[]}}`})
	integration := configuredAPI(t, fixture.server.URL, fixtureAPIKey)
	assert.True(t, integration.Healthy(t.Context()))
	calls := fixture.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "apiKey.info", calls[0].path)
	assert.Empty(t, calls[0].body)
}

func TestNative_EnvelopeErrors(t *testing.T) {
	tests := []struct {
		name     string
		response string
		status   int
		want     []string
	}{
		{name: "errorInfo on HTTP 200", response: `{"success":false,"errorInfo":{"code":"application_not_found","message":"Application not found","requestId":"req-1"}}`, want: []string{"application_not_found", "Application not found", "req-1"}},
		{name: "errors array on HTTP 200", response: `{"success":false,"errors":["invalid_input","jobId is required"]}`, want: []string{"invalid_input", "jobId is required"}},
		{name: "missing endpoint permission", status: http.StatusForbidden, response: `{"success":false,"errorInfo":{"code":"missing_endpoint_permission","message":"The API key lacks permission"}}`, want: []string{"403", "missing_endpoint_permission", "application.list", "Admin"}},
		{name: "success false without detail", response: `{"success":false}`, want: []string{"application.list", "success=false"}},
		{name: "not json", response: `<html>oops</html>`, want: []string{"invalid JSON"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.status != 0 {
					w.WriteHeader(tt.status)
				}
				_, _ = io.WriteString(w, tt.response)
			}))
			t.Cleanup(server.Close)
			integration := configuredAPI(t, server.URL, fixtureAPIKey)
			result, err := integration.Execute(t.Context(), "ashby_list_applications", nil)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			for _, want := range tt.want {
				assert.Contains(t, result.Data, want)
			}
		})
	}
}

func TestNative_RetryableStatuses(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(status)
			}))
			t.Cleanup(server.Close)
			integration := configuredAPI(t, server.URL, fixtureAPIKey)
			_, err := integration.Execute(t.Context(), "ashby_list_jobs", nil)
			require.Error(t, err)
			assert.True(t, mcp.IsRetryable(err))
		})
	}
}

func TestNative_RequestBodies(t *testing.T) {
	tests := []struct {
		name string
		tool mcp.ToolName
		args map[string]any
		path string
		body map[string]any
	}{
		{
			name: "list applies the default page size and maps snake_case args",
			tool: "ashby_list_applications",
			args: map[string]any{"job_id": "job-1", "status": "Active", "cursor": "c1", "sync_token": "s1", "expand": []any{"openings"}},
			path: "application.list",
			body: map[string]any{"jobId": "job-1", "status": "Active", "cursor": "c1", "syncToken": "s1", "limit": float64(defaultPageLimit), "expand": []any{"openings"}},
		},
		{
			name: "explicit limit and integer coercion",
			tool: "ashby_list_candidates",
			args: map[string]any{"limit": "10", "created_after": float64(1700000000000)},
			path: "candidate.list",
			body: map[string]any{"limit": float64(10), "createdAfter": float64(1700000000000)},
		},
		{
			name: "id argument renamed to the endpoint field",
			tool: "ashby_get_candidate",
			args: map[string]any{"candidate_id": "cand-1"},
			path: "candidate.info",
			body: map[string]any{"id": "cand-1"},
		},
		{
			name: "booleans and objects pass through",
			tool: "ashby_change_application_stage",
			args: map[string]any{"application_id": "app-1", "interview_stage_id": "stage-9", "archive_reason_id": "r-1", "archive_email": map[string]any{"communicationTemplateId": "t-1"}},
			path: "application.changeStage",
			body: map[string]any{"applicationId": "app-1", "interviewStageId": "stage-9", "archiveReasonId": "r-1", "archiveEmail": map[string]any{"communicationTemplateId": "t-1"}},
		},
		{
			name: "note may be a string",
			tool: "ashby_create_candidate_note",
			args: map[string]any{"candidate_id": "cand-1", "note": "Strong systems design", "is_private": true},
			path: "candidate.createNote",
			body: map[string]any{"candidateId": "cand-1", "note": "Strong systems design", "isPrivate": true},
		},
		{
			name: "default sent when omitted",
			tool: "ashby_list_hiring_team_roles",
			args: nil,
			path: "hiringTeamRole.list",
			body: map[string]any{"namesOnly": false},
		},
		{
			name: "unknown arguments are ignored",
			tool: "ashby_get_job",
			args: map[string]any{"job_id": "job-1", "owner": "session-default"},
			path: "job.info",
			body: map[string]any{"id": "job-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newAPIFixture(t, nil)
			integration := configuredAPI(t, fixture.server.URL, fixtureAPIKey)
			result, err := integration.Execute(t.Context(), tt.tool, tt.args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			calls := fixture.snapshot()
			require.Len(t, calls, 1)
			assert.Equal(t, tt.path, calls[0].path)
			assert.Equal(t, tt.body, calls[0].body)
		})
	}
}

func TestNative_ArgumentErrors(t *testing.T) {
	tests := []struct {
		name string
		tool mcp.ToolName
		args map[string]any
		want string
	}{
		{name: "missing required", tool: "ashby_list_application_history", args: nil, want: "application_id"},
		{name: "blank required string", tool: "ashby_get_job", args: map[string]any{"job_id": " "}, want: "job_id"},
		{name: "wrong type", tool: "ashby_list_candidates", args: map[string]any{"limit": "lots"}, want: "limit"},
		{name: "object expected", tool: "ashby_submit_application_feedback", args: map[string]any{"application_id": "a", "form_definition_id": "f", "feedback_form": "nope"}, want: "feedback_form"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newAPIFixture(t, nil)
			integration := configuredAPI(t, fixture.server.URL, fixtureAPIKey)
			result, err := integration.Execute(t.Context(), tt.tool, tt.args)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
			assert.Empty(t, fixture.snapshot(), "invalid arguments must not reach Ashby")
		})
	}
}

func TestNative_UnknownTool(t *testing.T) {
	fixture := newAPIFixture(t, nil)
	integration := configuredAPI(t, fixture.server.URL, fixtureAPIKey)
	for _, name := range []mcp.ToolName{"ashby_nope", "ashby_get_pending_tasks", "ashby_search_records_by_name"} {
		t.Run(string(name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), name, nil)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "unknown tool")
		})
	}
	assert.Empty(t, fixture.snapshot())
}

func TestNative_Tools(t *testing.T) {
	integration := configuredAPI(t, "https://api.example.test", fixtureAPIKey)
	tools := integration.Tools()
	require.Len(t, tools, len(nativeEndpoints))
	seen := map[mcp.ToolName]bool{}
	var starts []mcp.ToolName
	for _, tool := range tools {
		assert.True(t, strings.HasPrefix(string(tool.Name), "ashby_"), "%s lacks prefix", tool.Name)
		assert.NotEmpty(t, tool.Description, "%s has no description", tool.Name)
		assert.NotContains(t, tool.Description, "\u2014", "%s uses an em dash", tool.Name)
		assert.True(t, tool.Access.Declared(), "%s does not declare Access", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		assert.NotNil(t, tool.Parameters, "%s has nil parameters", tool.Name)
		for _, required := range tool.Required {
			assert.Contains(t, tool.Parameters, required, "%s requires undeclared %s", tool.Name, required)
		}
		for name, desc := range tool.Parameters {
			assert.NotEmpty(t, desc, "%s.%s has no description", tool.Name, name)
			assert.Equal(t, strings.ToLower(name), name, "%s.%s must be snake_case", tool.Name, name)
		}
		if strings.Contains(tool.Description, "Start here") {
			starts = append(starts, tool.Name)
		}
		seen[tool.Name] = true
	}
	assert.Contains(t, starts, mcp.ToolName("ashby_search_candidates"))
	assert.Contains(t, starts, mcp.ToolName("ashby_list_jobs"))

	tools[0].Parameters["injected"] = "x"
	_, injected := integration.Tools()[0].Parameters["injected"]
	assert.False(t, injected, "Tools must return a copy")
}

func TestNative_Access(t *testing.T) {
	destructive := map[mcp.ToolName]bool{
		"ashby_set_job_status":            true,
		"ashby_set_opening_archived":      true,
		"ashby_change_application_stage":  true,
		"ashby_delete_application":        true,
		"ashby_update_interview_schedule": true,
		"ashby_cancel_interview_schedule": true,
		"ashby_archive_candidate_tag":     true,
	}
	for _, ep := range nativeEndpoints {
		t.Run(string(ep.tool), func(t *testing.T) {
			switch {
			case destructive[ep.tool]:
				assert.Equal(t, mcp.AccessDestructive, ep.access)
			case readPrefix(ep.tool):
				assert.Equal(t, mcp.AccessRead, ep.access)
			case strings.Contains(string(ep.tool), "_archive_") || strings.Contains(string(ep.tool), "_delete_") || strings.Contains(string(ep.tool), "_cancel_"):
				t.Errorf("%s archives, deletes, or cancels but is not listed as destructive", ep.tool)
			default:
				assert.Equal(t, mcp.AccessWrite, ep.access)
			}
		})
	}
}

// readPrefix reports whether a tool name starts with a read verb. Report
// generation is a read in Ashby's permission model (Reports module, read).
func readPrefix(name mcp.ToolName) bool {
	for _, prefix := range []string{"ashby_list_", "ashby_get_", "ashby_search_", "ashby_generate_report"} {
		if strings.HasPrefix(string(name), prefix) {
			return true
		}
	}
	return false
}

func TestNativeDispatch_AllToolsCovered(t *testing.T) {
	for _, tool := range configuredAPI(t, "https://api.example.test", fixtureAPIKey).Tools() {
		_, ok := nativeDispatch[tool.Name]
		assert.True(t, ok, "tool %s has no native dispatch entry", tool.Name)
	}
}

func TestNativeDispatch_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range configuredAPI(t, "https://api.example.test", fixtureAPIKey).Tools() {
		names[tool.Name] = true
	}
	require.Len(t, nativeDispatch, len(nativeEndpoints), "duplicate tool names collapse in the dispatch map")
	for name, ep := range nativeDispatch {
		assert.True(t, names[name], "native dispatch entry %s has no tool definition", name)
		assert.Regexp(t, `^[a-zA-Z]+\.[a-zA-Z]+$`, ep.path, "%s has a malformed endpoint", name)
	}
}

// Hosted and native catalogs may share names (for example
// ashby_get_candidate); each mode dispatches only its own catalog, so a
// shared name must never route a native call to the hosted server or back.
func TestDispatch_SharedNamesStayInTheirMode(t *testing.T) {
	var shared []mcp.ToolName
	for name := range dispatch {
		if _, ok := nativeDispatch[name]; ok {
			shared = append(shared, name)
		}
	}
	require.NotEmpty(t, shared, "the test needs at least one shared name to mean anything")

	api := newAPIFixture(t, nil)
	native := configuredAPI(t, api.server.URL, fixtureAPIKey)
	hosted := newHostedFixture(t, nil)
	remote := configured(t, hosted.server.URL, fixtureToken)
	for _, name := range shared {
		t.Run(string(name), func(t *testing.T) {
			_, err := native.Execute(t.Context(), name, map[string]any{"candidate_id": "c", "name": "n", "application_id": "a", "interview_stage_id": "s"})
			require.NoError(t, err)
			_, err = remote.Execute(t.Context(), name, map[string]any{"query": "q"})
			require.NoError(t, err)
		})
	}
	for _, call := range api.snapshot() {
		assert.Equal(t, nativeDispatch[toolForPath(t, call.path)].path, call.path)
	}
	assert.Len(t, api.snapshot(), len(shared))
	assert.Len(t, hosted.snapshot(), len(shared))
}

func toolForPath(t *testing.T, path string) mcp.ToolName {
	t.Helper()
	for name, ep := range nativeDispatch {
		if ep.path == path {
			return name
		}
	}
	t.Fatalf("no native tool calls %s", path)
	return ""
}

func TestNative_CompactionOnlyInAPIKeyMode(t *testing.T) {
	native := configuredAPI(t, "https://api.example.test", fixtureAPIKey).(mcp.FieldCompactionIntegration)
	_, ok := native.CompactSpec("ashby_get_candidate")
	assert.True(t, ok)

	hosted := newHostedFixture(t, nil)
	remote := configured(t, hosted.server.URL, fixtureToken).(mcp.FieldCompactionIntegration)
	_, ok = remote.CompactSpec("ashby_get_candidate")
	assert.False(t, ok, "hosted responses are shaped by Ashby's MCP server, never by the REST specs")
	_, ok = New().(mcp.FieldCompactionIntegration).CompactSpec("ashby_get_candidate")
	assert.False(t, ok)
}

func TestNative_ResponseCapOnlyInHostedMode(t *testing.T) {
	native := configuredAPI(t, "https://api.example.test", fixtureAPIKey).(mcp.PerToolMaxResponseBytesIntegration)
	_, ok := native.MaxResponseBytesForTool("ashby_get_candidate")
	assert.False(t, ok)
}

func TestNative_CloseIsIdempotent(t *testing.T) {
	integration := configuredAPI(t, "https://api.example.test", fixtureAPIKey)
	closer := integration.(io.Closer)
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	result, err := integration.Execute(t.Context(), "ashby_list_jobs", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
}
