package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "github", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"token": "ghp_test123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}

func TestConfigure_EmptyCredentials(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	// Verify all tools have names and descriptions.
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveGitHubPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "github_", "tool %s missing github_ prefix", tool.Name)
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
	g := &integration{token: "test", client: nil}
	result, err := g.Execute(context.Background(), "github_nonexistent", nil)
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

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
	assert.Contains(t, result.Data, `"value"`)
}

func TestJsonResult_MarshalError(t *testing.T) {
	result, err := mcp.JSONResult(make(chan int))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// newTestGitHub creates a *integration backed by a test HTTP server.
func newTestGitHub(handler http.Handler) (*integration, *httptest.Server) {
	ts := httptest.NewServer(handler)
	client := gh.NewClient(nil).WithAuthToken("test-token")
	url, _ := client.BaseURL.Parse(ts.URL + "/")
	client.BaseURL = url
	return &integration{token: "test-token", client: client}, ts
}

func TestCreateHook_ActiveBool(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		wantActive bool
	}{
		{name: "missing defaults to true", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com"}, wantActive: true},
		{name: "bool true", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com", "active": true}, wantActive: true},
		{name: "bool false", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com", "active": false}, wantActive: false},
		{name: "string false", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com", "active": "false"}, wantActive: false},
		{name: "string 0", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com", "active": "0"}, wantActive: false},
		{name: "string f", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com", "active": "f"}, wantActive: false},
		{name: "string FALSE", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com", "active": "FALSE"}, wantActive: false},
		{name: "string true", args: map[string]any{"owner": "o", "repo": "r", "url": "http://example.com", "active": "true"}, wantActive: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedActive *bool
			g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var hook gh.Hook
				json.NewDecoder(r.Body).Decode(&hook)
				capturedActive = hook.Active
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(&hook)
			}))
			defer ts.Close()

			result, err := createHook(context.Background(), g, tt.args)
			require.NoError(t, err)
			assert.False(t, result.IsError, "unexpected error: %s", result.Data)
			require.NotNil(t, capturedActive)
			assert.Equal(t, tt.wantActive, *capturedActive)
		})
	}
}

func TestCreateHook_ActiveInvalidType(t *testing.T) {
	g := &integration{token: "test"}
	result, err := createHook(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "url": "http://example.com",
		"active": 42, // int is not a valid bool type
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parameter")
}

// ── PR Review Comment CRUD Tests ─────────────────────────────────

func TestGetPRComment(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/repos/o/r/pulls/comments/42")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gh.PullRequestComment{
			ID:   gh.Ptr(int64(42)),
			Body: gh.Ptr("looks good"),
		})
	}))
	defer ts.Close()

	result, err := getPRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "comment_id": float64(42),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "unexpected error: %s", result.Data)
	assert.Contains(t, result.Data, `"looks good"`)
}

func TestReplyToPRComment(t *testing.T) {
	var capturedBody struct {
		Body      string `json:"body"`
		InReplyTo int64  `json:"in_reply_to"`
	}
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/repos/o/r/pulls/7/comments")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gh.PullRequestComment{
			ID:   gh.Ptr(int64(99)),
			Body: gh.Ptr(capturedBody.Body),
		})
	}))
	defer ts.Close()

	result, err := replyToPRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "pull_number": float64(7),
		"body": "thanks!", "comment_id": float64(42),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "unexpected error: %s", result.Data)
	assert.Equal(t, int64(42), capturedBody.InReplyTo)
	assert.Equal(t, "thanks!", capturedBody.Body)
}

func TestUpdatePRComment(t *testing.T) {
	var capturedBody gh.PullRequestComment
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Contains(t, r.URL.Path, "/repos/o/r/pulls/comments/42")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gh.PullRequestComment{
			ID:   gh.Ptr(int64(42)),
			Body: gh.Ptr("updated body"),
		})
	}))
	defer ts.Close()

	result, err := updatePRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "comment_id": float64(42), "body": "updated body",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "unexpected error: %s", result.Data)
	assert.Equal(t, "updated body", capturedBody.GetBody())
	assert.Contains(t, result.Data, `"updated body"`)
}

func TestDeletePRComment(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Contains(t, r.URL.Path, "/repos/o/r/pulls/comments/42")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	result, err := deletePRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "comment_id": float64(42),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "unexpected error: %s", result.Data)
	assert.Contains(t, result.Data, `"deleted"`)
}

func TestGetPRComment_APIError(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer ts.Close()

	result, err := getPRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "comment_id": float64(999),
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "Not Found")
}

func TestGetPRComment_ServerError(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"Internal Server Error"}`)
	}))
	defer ts.Close()

	_, err := getPRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "comment_id": float64(42),
	})
	require.Error(t, err, "500 should propagate as retryable Go error")
	var re *mcp.RetryableError
	assert.ErrorAs(t, err, &re)
	assert.Equal(t, 500, re.StatusCode)
}

func TestReplyToPRComment_APIError(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"message":"Validation Failed"}`)
	}))
	defer ts.Close()

	result, err := replyToPRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "pull_number": float64(7),
		"body": "reply", "comment_id": float64(999),
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "Validation Failed")
}

func TestUpdatePRComment_APIError(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer ts.Close()

	result, err := updatePRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "comment_id": float64(999), "body": "new body",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "Not Found")
}

func TestDeletePRComment_APIError(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer ts.Close()

	result, err := deletePRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "comment_id": float64(999),
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "Not Found")
}

func TestReplyToPRComment_ServerError(t *testing.T) {
	g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"Internal Server Error"}`)
	}))
	defer ts.Close()

	_, err := replyToPRComment(context.Background(), g, map[string]any{
		"owner": "o", "repo": "r", "pull_number": float64(7),
		"body": "reply", "comment_id": float64(42),
	})
	require.Error(t, err, "500 should propagate as retryable Go error")
	var re *mcp.RetryableError
	assert.ErrorAs(t, err, &re)
	assert.Equal(t, 500, re.StatusCode)
}

func TestListIssues_MilestonePassthrough(t *testing.T) {
	tests := []struct {
		name          string
		milestone     any
		wantMilestone string
	}{
		{name: "star", milestone: "*", wantMilestone: "%2A"},
		{name: "none", milestone: "none", wantMilestone: "none"},
		{name: "numeric string", milestone: "3", wantMilestone: "3"},
		{name: "missing", milestone: nil, wantMilestone: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]any{})
			}))
			defer ts.Close()

			args := map[string]any{"owner": "o", "repo": "r"}
			if tt.milestone != nil {
				args["milestone"] = tt.milestone
			}
			result, err := listIssues(context.Background(), g, args)
			require.NoError(t, err)
			assert.False(t, result.IsError, "unexpected error: %s", result.Data)

			if tt.wantMilestone != "" {
				assert.Contains(t, capturedPath, "milestone="+tt.wantMilestone)
			}
		})
	}
}
