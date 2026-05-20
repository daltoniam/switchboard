package gtasks

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
	assert.Equal(t, "gtasks", i.Name())
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
	g := &gtasks{client: &http.Client{}, baseURL: "https://tasks.googleapis.com/tasks/v1"}
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

func TestTools_AllHaveGtasksPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gtasks_", "tool %s missing gtasks_ prefix", tool.Name)
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
		if tool.Name == mcp.ToolName("gtasks_list_tasklists") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gtasks_list_tasklists must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gtasks_list_tasklists tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gtasks{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gtasks_nonexistent", nil)
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
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/users/@me/lists")
	require.NoError(t, err)
	assert.Contains(t, string(data), "items")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/users/@me/lists/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gtasks API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.delete(context.Background(), "/users/@me/lists/x")
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

// ── Handler: listTasklists ─────────────────────────────────────────

func TestListTasklists(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/users/@me/lists", r.URL.Path)
		assert.Equal(t, "50", r.URL.Query().Get("maxResults"))
		assert.Equal(t, "tok-2", r.URL.Query().Get("pageToken"))
		_, _ = w.Write([]byte(`{"items":[{"id":"tl-1","title":"My Tasks"}]}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_list_tasklists", map[string]any{
		"max_results": 50,
		"page_token":  "tok-2",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Tasks")
}

// ── Handler: createTasklist ────────────────────────────────────────

func TestCreateTasklist(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/users/@me/lists", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Groceries", body["title"])
		_, _ = w.Write([]byte(`{"id":"tl-new","title":"Groceries"}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_create_tasklist", map[string]any{
		"title": "Groceries",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "tl-new")
}

func TestCreateTasklist_MissingTitle(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gtasks_create_tasklist", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "title is required")
}

// ── Handler: deleteTasklist ────────────────────────────────────────

func TestDeleteTasklist(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/users/@me/lists/tl-1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_delete_tasklist", map[string]any{
		"tasklist_id": "tl-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteTasklist_MissingID(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gtasks_delete_tasklist", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "tasklist_id is required")
}

// ── Handler: listTasks ─────────────────────────────────────────────

func TestListTasks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/lists/tl-1/tasks", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "25", q.Get("maxResults"))
		assert.Equal(t, "page-2", q.Get("pageToken"))
		assert.Equal(t, "true", q.Get("showCompleted"))
		assert.Equal(t, "true", q.Get("showHidden"))
		assert.Equal(t, "true", q.Get("showDeleted"))
		assert.Equal(t, "2024-01-01T00:00:00Z", q.Get("dueMin"))
		assert.Equal(t, "2024-12-31T23:59:59Z", q.Get("dueMax"))
		assert.Equal(t, "2024-06-01T00:00:00Z", q.Get("completedMin"))
		assert.Equal(t, "2024-06-30T23:59:59Z", q.Get("completedMax"))
		assert.Equal(t, "2024-05-01T00:00:00Z", q.Get("updatedMin"))
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_list_tasks", map[string]any{
		"tasklist_id":    "tl-1",
		"max_results":    25,
		"page_token":     "page-2",
		"show_completed": true,
		"show_hidden":    true,
		"show_deleted":   true,
		"due_min":        "2024-01-01T00:00:00Z",
		"due_max":        "2024-12-31T23:59:59Z",
		"completed_min":  "2024-06-01T00:00:00Z",
		"completed_max":  "2024-06-30T23:59:59Z",
		"updated_min":    "2024-05-01T00:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListTasks_MissingID(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gtasks_list_tasks", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "tasklist_id is required")
}

// ── Handler: getTask ───────────────────────────────────────────────

func TestGetTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/lists/tl-1/tasks/t-1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"t-1","title":"Buy milk"}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_get_task", map[string]any{
		"tasklist_id": "tl-1",
		"task_id":     "t-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Buy milk")
}

func TestGetTask_MissingArgs(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gtasks_get_task", map[string]any{"task_id": "x"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "tasklist_id is required")
	r2, _ := g.Execute(context.Background(), "gtasks_get_task", map[string]any{"tasklist_id": "tl"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "task_id is required")
}

// ── Handler: createTask ────────────────────────────────────────────

func TestCreateTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/lists/tl-1/tasks", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "p-1", q.Get("parent"))
		assert.Equal(t, "prev-1", q.Get("previous"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Walk dog", body["title"])
		assert.Equal(t, "Around the block", body["notes"])
		assert.Equal(t, "2024-07-04T00:00:00Z", body["due"])
		_, _ = w.Write([]byte(`{"id":"t-new","title":"Walk dog"}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_create_task", map[string]any{
		"tasklist_id": "tl-1",
		"title":       "Walk dog",
		"notes":       "Around the block",
		"due":         "2024-07-04T00:00:00Z",
		"parent_id":   "p-1",
		"previous_id": "prev-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "t-new")
}

func TestCreateTask_MissingArgs(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gtasks_create_task", map[string]any{"title": "x"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "tasklist_id is required")
	r2, _ := g.Execute(context.Background(), "gtasks_create_task", map[string]any{"tasklist_id": "tl"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "title is required")
}

// ── Handler: updateTask ────────────────────────────────────────────

func TestUpdateTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/lists/tl-1/tasks/t-1", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Done!", body["title"])
		assert.Equal(t, "completed", body["status"])
		_, _ = w.Write([]byte(`{"id":"t-1","status":"completed"}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_update_task", map[string]any{
		"tasklist_id": "tl-1",
		"task_id":     "t-1",
		"title":       "Done!",
		"status":      "completed",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "completed")
}

func TestUpdateTask_ClearNotes(t *testing.T) {
	// Passing notes="" should send {"notes":""} to clear the field.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		v, ok := body["notes"]
		assert.True(t, ok, "notes key should be present in PATCH body")
		assert.Equal(t, "", v)
		_, _ = w.Write([]byte(`{"id":"t-1"}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_update_task", map[string]any{
		"tasklist_id": "tl-1",
		"task_id":     "t-1",
		"notes":       "",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateTask_NoFields(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gtasks_update_task", map[string]any{
		"tasklist_id": "tl-1",
		"task_id":     "t-1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "no fields to update")
}

func TestUpdateTask_MissingArgs(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gtasks_update_task", map[string]any{"task_id": "x"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "tasklist_id is required")
	r2, _ := g.Execute(context.Background(), "gtasks_update_task", map[string]any{"tasklist_id": "tl"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "task_id is required")
}

// ── Handler: deleteTask ────────────────────────────────────────────

func TestDeleteTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/lists/tl-1/tasks/t-1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_delete_task", map[string]any{
		"tasklist_id": "tl-1",
		"task_id":     "t-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: moveTask ──────────────────────────────────────────────

func TestMoveTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/lists/tl-1/tasks/t-1/move", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "p-1", q.Get("parent"))
		assert.Equal(t, "prev-1", q.Get("previous"))
		_, _ = w.Write([]byte(`{"id":"t-1","parent":"p-1"}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_move_task", map[string]any{
		"tasklist_id": "tl-1",
		"task_id":     "t-1",
		"parent_id":   "p-1",
		"previous_id": "prev-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestMoveTask_MissingArgs(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	r1, _ := g.Execute(context.Background(), "gtasks_move_task", map[string]any{"task_id": "x"})
	assert.True(t, r1.IsError)
	assert.Contains(t, r1.Data, "tasklist_id is required")
	r2, _ := g.Execute(context.Background(), "gtasks_move_task", map[string]any{"tasklist_id": "tl"})
	assert.True(t, r2.IsError)
	assert.Contains(t, r2.Data, "task_id is required")
}

// ── Handler: clearCompleted ────────────────────────────────────────

func TestClearCompleted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/lists/tl-1/clear", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gtasks_clear_completed", map[string]any{
		"tasklist_id": "tl-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestClearCompleted_MissingID(t *testing.T) {
	g := &gtasks{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gtasks_clear_completed", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "tasklist_id is required")
}

// ── Healthy ────────────────────────────────────────────────────────

func TestHealthy_TrueOn200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
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
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer api.Close()

	g := &gtasks{
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

func TestTasklistIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gtasks_delete_tasklist", map[string]any{
		"tasklist_id": "tl with space/x",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "tl%20with%20space") || strings.Contains(seenPath, "tl+with+space"),
		"tasklist id with space should be URL-escaped; got %s", seenPath)
}

func TestTaskIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	g := &gtasks{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gtasks_get_task", map[string]any{
		"tasklist_id": "tl",
		"task_id":     "task with space",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "task%20with%20space"),
		"task id with space should be URL-escaped; got %s", seenPath)
}
