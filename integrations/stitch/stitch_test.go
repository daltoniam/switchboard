package stitch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	i := New()
	assert.Equal(t, "stitch", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"access_token": "test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	s := &stitch{client: &http.Client{}}
	err := s.Configure(context.Background(), mcp.Credentials{
		"access_token": "tok",
		"base_url":     "https://custom.stitch.dev/v1/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://custom.stitch.dev/v1", s.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tl := i.Tools()
	assert.Len(t, tl, 12)

	for _, tool := range tl {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveStitchPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "stitch_", "tool %s missing stitch_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &stitch{token: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "stitch_nonexistent", nil)
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
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- Health check tests ---

func TestHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.String(), "/projects")
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"projects":[]}`))
	}))
	defer ts.Close()

	s := &stitch{token: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, s.Healthy(context.Background()))
}

func TestHealthy_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	s := &stitch{token: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, s.Healthy(context.Background()))
}

// --- HTTP helper tests ---

func TestDoRequest_BearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	s := &stitch{token: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := s.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer ts.Close()

	s := &stitch{token: "bad", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stitch API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &stitch{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := s.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	s := &stitch{token: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := s.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	s := &stitch{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "429 should produce RetryableError")

	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
	assert.Equal(t, 30*time.Second, re.RetryAfter)
}

func TestDoRequest_RetryableOn5xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`service unavailable`))
	}))
	defer ts.Close()

	s := &stitch{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "503 should produce RetryableError")
}

func TestDoRequest_NonRetryableOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`not found`))
	}))
	defer ts.Close()

	s := &stitch{token: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/test")
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "404 should NOT be retryable")
}

// --- test helpers ---

func newTestStitch(t *testing.T, handler http.HandlerFunc) *stitch {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &stitch{token: "test-token", client: ts.Client(), baseURL: ts.URL}
}

// --- tool roundtrip tests ---

func TestListProjects(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/projects")
		_, _ = w.Write([]byte(`{"projects":[{"name":"projects/123","title":"My Project"}]}`))
	})
	result, err := s.Execute(context.Background(), "stitch_list_projects", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Project")
}

func TestListProjects_WithFilter(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "view=shared", r.URL.Query().Get("filter"))
		_, _ = w.Write([]byte(`{"projects":[]}`))
	})
	result, err := s.Execute(context.Background(), "stitch_list_projects", map[string]any{"filter": "view=shared"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateProject(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/projects", r.URL.Path)
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "New App", body["title"])
		_, _ = w.Write([]byte(`{"name":"projects/456","title":"New App"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_create_project", map[string]any{"title": "New App"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "New App")
}

func TestGetProject(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/projects/123", r.URL.Path)
		_, _ = w.Write([]byte(`{"name":"projects/123","title":"My Project"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_get_project", map[string]any{"name": "projects/123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "projects/123")
}

func TestGetProject_MissingName(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server")
	})
	result, err := s.Execute(context.Background(), "stitch_get_project", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "name parameter is required")
}

func TestListScreens(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/projects/123/screens", r.URL.Path)
		_, _ = w.Write([]byte(`{"screens":[{"name":"projects/123/screens/abc"}]}`))
	})
	result, err := s.Execute(context.Background(), "stitch_list_screens", map[string]any{"project_id": "123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "abc")
}

func TestListScreens_MissingProjectID(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server")
	})
	result, err := s.Execute(context.Background(), "stitch_list_screens", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "project_id parameter is required")
}

func TestGetScreen(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/projects/123/screens/abc", r.URL.Path)
		_, _ = w.Write([]byte(`{"name":"projects/123/screens/abc"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_get_screen", map[string]any{
		"name": "projects/123/screens/abc",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGenerateScreenFromText(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/projects/123/screens:generateFromText")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "123", body["projectId"])
		assert.Equal(t, "a login page", body["prompt"])
		assert.Equal(t, "MOBILE", body["deviceType"])
		_, _ = w.Write([]byte(`{"name":"projects/123/screens/new1"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_generate_screen_from_text", map[string]any{
		"project_id":  "123",
		"prompt":      "a login page",
		"device_type": "MOBILE",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new1")
}

func TestEditScreens(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/projects/123/screens:edit")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "123", body["projectId"])
		assert.Equal(t, "make it blue", body["prompt"])
		ids := body["selectedScreenIds"].([]any)
		assert.Len(t, ids, 2)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_edit_screens", map[string]any{
		"project_id":          "123",
		"selected_screen_ids": []any{"s1", "s2"},
		"prompt":              "make it blue",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestEditScreens_MissingScreenIDs(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server")
	})
	result, err := s.Execute(context.Background(), "stitch_edit_screens", map[string]any{
		"project_id": "123",
		"prompt":     "make it blue",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "selected_screen_ids parameter is required")
}

func TestGenerateVariants(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/projects/123/screens:generateVariants")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "123", body["projectId"])
		assert.Equal(t, "more colorful", body["prompt"])
		opts := body["variantOptions"].(map[string]any)
		assert.Equal(t, "REIMAGINE", opts["creativeRange"])
		assert.Equal(t, float64(5), opts["variantCount"])
		_, _ = w.Write([]byte(`{"variants":[]}`))
	})
	result, err := s.Execute(context.Background(), "stitch_generate_variants", map[string]any{
		"project_id":          "123",
		"selected_screen_ids": []any{"s1"},
		"prompt":              "more colorful",
		"creative_range":      "REIMAGINE",
		"variant_count":       float64(5),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListDesignSystems(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/projects/123/designSystems", r.URL.Path)
		_, _ = w.Write([]byte(`{"designSystems":[{"name":"assets/789"}]}`))
	})
	result, err := s.Execute(context.Background(), "stitch_list_design_systems", map[string]any{"project_id": "123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "789")
}

func TestListDesignSystems_Global(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/designSystems", r.URL.Path)
		_, _ = w.Write([]byte(`{"designSystems":[]}`))
	})
	result, err := s.Execute(context.Background(), "stitch_list_design_systems", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateDesignSystem(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/designSystems", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		ds := body["designSystem"].(map[string]any)
		assert.Equal(t, "Material You", ds["displayName"])
		theme := ds["theme"].(map[string]any)
		assert.Equal(t, "LIGHT", theme["colorMode"])
		assert.Equal(t, "#6750A4", theme["customColor"])
		assert.Equal(t, "123", body["projectId"])
		_, _ = w.Write([]byte(`{"name":"assets/new1"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_create_design_system", map[string]any{
		"project_id":    "123",
		"display_name":  "Material You",
		"color_mode":    "LIGHT",
		"headline_font": "INTER",
		"body_font":     "INTER",
		"roundness":     "ROUND_TWELVE",
		"custom_color":  "#6750A4",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateDesignSystem(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/assets/789", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "assets/789", body["name"])
		assert.Equal(t, "123", body["projectId"])
		_, _ = w.Write([]byte(`{"name":"assets/789"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_update_design_system", map[string]any{
		"name":          "assets/789",
		"project_id":    "123",
		"display_name":  "Updated DS",
		"color_mode":    "DARK",
		"headline_font": "DM_SANS",
		"body_font":     "DM_SANS",
		"roundness":     "ROUND_FULL",
		"custom_color":  "#FF0000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestApplyDesignSystem(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/projects/123/screens:applyDesignSystem")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "123", body["projectId"])
		assert.Equal(t, "789", body["assetId"])
		instances := body["selectedScreenInstances"].([]any)
		assert.Len(t, instances, 1)
		inst := instances[0].(map[string]any)
		assert.Equal(t, "inst1", inst["id"])
		assert.Equal(t, "projects/123/screens/s1", inst["sourceScreen"])
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})
	result, err := s.Execute(context.Background(), "stitch_apply_design_system", map[string]any{
		"project_id": "123",
		"asset_id":   "789",
		"selected_screen_instances": []any{
			map[string]any{"id": "inst1", "source_screen": "projects/123/screens/s1"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestApplyDesignSystem_MissingInstances(t *testing.T) {
	s := newTestStitch(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach server")
	})
	result, err := s.Execute(context.Background(), "stitch_apply_design_system", map[string]any{
		"project_id": "123",
		"asset_id":   "789",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "selected_screen_instances parameter is required")
}

// --- PlainTextKeys test ---

func TestPlainTextKeys(t *testing.T) {
	s := &stitch{}
	keys := s.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
}

// --- queryEncode tests ---

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

// --- ErrResult helper tests ---

func TestErrResult_PropagatesRetryableError(t *testing.T) {
	retryErr := &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
	result, err := mcp.ErrResult(retryErr)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestErrResult_WrapsNonRetryableError(t *testing.T) {
	plainErr := fmt.Errorf("bad request")
	result, err := mcp.ErrResult(plainErr)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "bad request", result.Data)
}
