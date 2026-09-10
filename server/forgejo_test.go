package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/forgejo"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newForgejoExecuteTestServer(t *testing.T, handler http.HandlerFunc) *Server {
	t.Helper()
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	reg := newMockRegistry()
	require.NoError(t, reg.Register(forgejo.New()))
	s := New(&mcp.Services{
		Config: newMockConfigService(map[string]*mcp.IntegrationConfig{
			"forgejo": {
				Enabled: true,
				Credentials: mcp.Credentials{
					"base_url": upstream.URL,
					"token":    "test-token",
				},
			},
		}),
		Registry: reg,
	})
	s.retryBackoff = 0
	return s
}

func TestHandleSearch_ForgejoDiscovery(t *testing.T) {
	var requests atomic.Int64
	s := newForgejoExecuteTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "search must not contact Forgejo", http.StatusInternalServerError)
	})
	t.Cleanup(func() { assert.Zero(t, requests.Load(), "discovery must not make upstream HTTP requests") })

	result, err := s.handleSearch(t.Context(), searchRequest(map[string]any{
		"integration": "forgejo", "limit": 50,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)
	catalog := parseSearchResponse(t, result)
	require.Equal(t, 30, catalog.Total)
	assert.Equal(t, 30, searchToolCount(t, catalog))
	assert.False(t, catalog.HasMore)
	var toolNames []string
	for _, tool := range forgejo.New().Tools() {
		toolNames = append(toolNames, string(tool.Name))
	}
	assert.ElementsMatch(t, toolNames, searchToolNames(t, catalog))

	tests := []struct {
		name        string
		query       string
		integration string
		top         int
		wantAny     []string
	}{
		{
			name: "repositories", query: "Forgejo repo", top: 3,
			wantAny: []string{"forgejo_list_user_repos", "forgejo_search_repos", "forgejo_get_repo"},
		},
		{
			name: "repository search", query: "Forgejo search repos", top: 3,
			wantAny: []string{"forgejo_search_repos"},
		},
		{
			name: "issues", query: "Forgejo issue", top: 3,
			wantAny: []string{"forgejo_list_issues"},
		},
		{
			name: "pull review workflow", query: "Forgejo pull review", top: 5,
			wantAny: []string{"forgejo_list_pulls", "forgejo_get_pull_diff", "forgejo_create_pull_review"},
		},
		{
			name: "file content", query: "Forgejo file content", top: 3,
			wantAny: []string{"forgejo_get_file_contents"},
		},
		{
			name: "list existing pull reviews", query: "list pull reviews", integration: "forgejo", top: 3,
			wantAny: []string{"forgejo_list_pull_reviews"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{"query": tt.query, "limit": tt.top}
			if tt.integration != "" {
				args["integration"] = tt.integration
			}
			var first []string
			for attempt := range 3 {
				s.RefreshSearchIndex()
				result, err := s.handleSearch(t.Context(), searchRequest(args))
				require.NoError(t, err)
				require.False(t, result.IsError)
				resp := parseSearchResponse(t, result)
				names := searchToolNames(t, resp)
				require.Len(t, names, tt.top)
				var hits []string
				for _, want := range tt.wantAny {
					if slices.Contains(names, want) {
						hits = append(hits, want)
					}
				}
				assert.NotEmpty(t, hits, "%q should surface one of %v in the top %d; got %v", tt.query, tt.wantAny, tt.top, names)
				for _, integration := range searchToolIntegrations(t, resp) {
					assert.Equal(t, "forgejo", integration)
				}
				if attempt == 0 {
					first = names
					t.Logf("%q (integration=%q) top %d: %v", tt.query, tt.integration, tt.top, names)
				} else {
					assert.Equal(t, first, names, "rebuilding the index must preserve discovery order")
				}
			}
		})
	}
}

func TestHandleSearch_ForgejoVisibility(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		discoverAll bool
		wantTotal   int
	}{
		{name: "enabled", enabled: true, wantTotal: 30},
		{name: "disabled"},
		{name: "enabled discover all", enabled: true, discoverAll: true, wantTotal: 30},
		{name: "disabled discover all", discoverAll: true, wantTotal: 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int64
			s := newForgejoExecuteTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				http.Error(w, "search must not contact Forgejo", http.StatusInternalServerError)
			})
			t.Cleanup(func() { assert.Zero(t, requests.Load(), "discovery must not make upstream HTTP requests") })
			ic, ok := s.services.Config.GetIntegration("forgejo")
			require.True(t, ok)
			ic.Enabled = tt.enabled
			require.NoError(t, s.services.Config.SetIntegration("forgejo", ic))
			s = New(s.services, WithDiscoverAll(tt.discoverAll))

			for _, query := range []string{"", "Forgejo"} {
				result, err := s.handleSearch(t.Context(), searchRequest(map[string]any{
					"query": query, "integration": "forgejo", "limit": 3,
				}))
				require.NoError(t, err)
				require.False(t, result.IsError)
				resp := parseSearchResponse(t, result)
				assert.Equal(t, tt.wantTotal, resp.Total, "query=%q", query)
				assert.Equal(t, min(3, tt.wantTotal), searchToolCount(t, resp))
				assert.Equal(t, tt.wantTotal > 3, resp.HasMore)
				if tt.wantTotal == 0 {
					assert.NotContains(t, resp.Integrations, "forgejo")
					continue
				}
				assert.Contains(t, resp.Integrations, "forgejo")
				var tools []struct {
					Configured *bool `json:"configured"`
				}
				require.NoError(t, json.Unmarshal(resp.Tools, &tools))
				for _, tool := range tools {
					if !tt.enabled {
						require.NotNil(t, tool.Configured)
						assert.False(t, *tool.Configured)
					} else if tool.Configured != nil {
						assert.True(t, *tool.Configured)
					}
				}
			}
		})
	}
}

func TestHandleExecute_ForgejoWriteFailuresAreNotRetried(t *testing.T) {
	const mentionFailure = "CreateIssueComment: comment persisted, but mention processing failure: UpdateIssueMentions could not resolve @missing-user because the user lookup failed after the comment transaction was committed"
	const rateLimitFailure = "Forgejo request rate limit exceeded for issue comment creation"
	tests := []struct {
		name       string
		status     int
		failure    string
		scriptMode bool
	}{
		{name: "persisted_comment_500", status: http.StatusInternalServerError, failure: mentionFailure},
		{name: "rate_limited_comment_429", status: http.StatusTooManyRequests, failure: rateLimitFailure},
		{name: "script_persisted_comment_500", status: http.StatusInternalServerError, failure: mentionFailure, scriptMode: true},
		{name: "script_rate_limited_comment_429", status: http.StatusTooManyRequests, failure: rateLimitFailure, scriptMode: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var posts, persistedComments atomic.Int64
			s := newForgejoExecuteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/repos/owner/repo/issues/7/comments", r.URL.Path)
				assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
				if !assert.Equal(t, http.MethodPost, r.Method) {
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
					return
				}
				posts.Add(1)
				var body struct {
					Body string `json:"body"`
				}
				if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
					http.Error(w, "invalid comment body", http.StatusBadRequest)
					return
				}
				assert.Equal(t, "Hello @missing-user", body.Body)
				if tt.status == http.StatusInternalServerError {
					persistedComments.Add(1)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"message": tt.failure}))
			})

			req := executeRequest("forgejo_create_issue_comment", map[string]any{
				"owner": "owner", "repo": "repo", "number": 7, "body": "Hello @missing-user",
			})
			if tt.scriptMode {
				req = &mcpsdk.CallToolRequest{Params: &mcpsdk.CallToolParamsRaw{
					Name:      "execute",
					Arguments: json.RawMessage(`{"script":"api.call('forgejo_create_issue_comment', {owner: 'owner', repo: 'repo', number: 7, body: 'Hello @missing-user'});"}`),
				}}
			}
			result, err := s.handleExecute(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, int64(1), posts.Load(), "failed writes must not be automatically retried")
			if tt.status == http.StatusInternalServerError {
				assert.Equal(t, int64(1), persistedComments.Load(), "a failure after commit must not create duplicate comments")
			} else {
				assert.Zero(t, persistedComments.Load())
			}
			require.NotNil(t, result)
			assert.True(t, result.IsError)
			require.Len(t, result.Content, 1)
			text, ok := result.Content[0].(*mcpsdk.TextContent)
			require.True(t, ok)
			assert.Contains(t, text.Text, strconv.Itoa(tt.status))
			assert.Contains(t, text.Text, tt.failure, "the full upstream failure must reach the MCP client")
			if tt.status == http.StatusInternalServerError {
				assert.Regexp(t, `(?i)(uncertain|unknown|may have (succeeded|completed|been))`, text.Text)
				assert.Regexp(t, `(?i)verif.*before.*retr`, text.Text)
			}
		})
	}
}

func TestHandleExecute_ForgejoSafeReadsRetryTransientFailures(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var gets atomic.Int64
			s := newForgejoExecuteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/repos/owner/repo/issues/7", r.URL.Path)
				if !assert.Equal(t, http.MethodGet, r.Method) {
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if gets.Add(1) == 1 {
					w.WriteHeader(status)
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"message": "temporary Forgejo read failure"}))
					return
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"id": 17, "number": 7, "title": "Recovered issue", "state": "open",
				}))
			})

			result, err := s.handleExecute(t.Context(), executeRequest("forgejo_get_issue", map[string]any{
				"owner": "owner", "repo": "repo", "number": 7,
			}))
			require.NoError(t, err)
			assert.Equal(t, int64(2), gets.Load(), "a safe read should retry once and stop after success")
			require.NotNil(t, result)
			assert.False(t, result.IsError)
			require.NotEmpty(t, result.Content)
			text, ok := result.Content[0].(*mcpsdk.TextContent)
			require.True(t, ok)
			assert.Contains(t, text.Text, "Recovered issue")
		})
	}
}
