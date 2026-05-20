package gsheets

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
	assert.Equal(t, "gsheets", i.Name())
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
	g := &gsheets{client: &http.Client{}, baseURL: "https://sheets.googleapis.com/v4"}
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

func TestTools_AllHaveGsheetsPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gsheets_", "tool %s missing gsheets_ prefix", tool.Name)
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
		if tool.Name == mcp.ToolName("gsheets_get_spreadsheet") {
			assert.Contains(t, tool.Description, "Start here",
				"entry-point tool gsheets_get_spreadsheet must include 'Start here' for wayfinding")
			return
		}
	}
	t.Fatal("gsheets_get_spreadsheet tool not found")
}

// ── Dispatch parity ─────────────────────────────────────────────────

func TestExecute_UnknownTool(t *testing.T) {
	g := &gsheets{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gsheets_nonexistent", nil)
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
		_, _ = w.Write([]byte(`{"spreadsheetId":"s-1","properties":{"title":"Test"}}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/spreadsheets/s-1")
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/spreadsheets/missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gsheets API error (404)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.post(context.Background(), "/spreadsheets/x:batchUpdate", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

// ── Handler: getSpreadsheet ────────────────────────────────────────

func TestGetSpreadsheet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spreadsheets/s-1", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("includeGridData"))
		assert.ElementsMatch(t, []string{"Sheet1!A1:C", "Sheet2!A:A"}, r.URL.Query()["ranges"])
		_, _ = w.Write([]byte(`{"spreadsheetId":"s-1","properties":{"title":"Hello"}}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_get_spreadsheet", map[string]any{
		"spreadsheet_id":    "s-1",
		"ranges":            "Sheet1!A1:C, Sheet2!A:A",
		"include_grid_data": "true",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello")
}

func TestGetSpreadsheet_MissingID(t *testing.T) {
	g := &gsheets{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gsheets_get_spreadsheet", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "spreadsheet_id is required")
}

// ── Handler: createSpreadsheet ─────────────────────────────────────

func TestCreateSpreadsheet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/spreadsheets", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		props, _ := body["properties"].(map[string]any)
		assert.Equal(t, "My Sheet", props["title"])
		sheets, _ := body["sheets"].([]any)
		assert.Len(t, sheets, 2)
		_, _ = w.Write([]byte(`{"spreadsheetId":"new-1"}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_create_spreadsheet", map[string]any{
		"title":        "My Sheet",
		"sheet_titles": "Summary, Data",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-1")
}

func TestCreateSpreadsheet_NoTitle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, hasProps := body["properties"]
		assert.False(t, hasProps, "no properties should be sent when title is empty")
		_, _ = w.Write([]byte(`{"spreadsheetId":"new-2"}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_create_spreadsheet", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: getValues ─────────────────────────────────────────────

func TestGetValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spreadsheets/s-1/values/Sheet1%21A1:C10", r.URL.EscapedPath())
		assert.Equal(t, "UNFORMATTED_VALUE", r.URL.Query().Get("valueRenderOption"))
		assert.Equal(t, "COLUMNS", r.URL.Query().Get("majorDimension"))
		_, _ = w.Write([]byte(`{"range":"Sheet1!A1:C10","majorDimension":"COLUMNS","values":[["a"],["b"]]}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_get_values", map[string]any{
		"spreadsheet_id":      "s-1",
		"range":               "Sheet1!A1:C10",
		"value_render_option": "UNFORMATTED_VALUE",
		"major_dimension":     "COLUMNS",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetValues_MissingRange(t *testing.T) {
	g := &gsheets{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gsheets_get_values", map[string]any{
		"spreadsheet_id": "s-1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "range is required")
}

// ── Handler: batchGetValues ────────────────────────────────────────

func TestBatchGetValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/spreadsheets/s-1/values:batchGet", r.URL.Path)
		assert.ElementsMatch(t, []string{"Sheet1!A1:B5", "Sheet2!C:C"}, r.URL.Query()["ranges"])
		_, _ = w.Write([]byte(`{"spreadsheetId":"s-1","valueRanges":[]}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_batch_get_values", map[string]any{
		"spreadsheet_id": "s-1",
		"ranges":         "Sheet1!A1:B5,Sheet2!C:C",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: updateValues ──────────────────────────────────────────

func TestUpdateValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "USER_ENTERED", r.URL.Query().Get("valueInputOption"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Sheet1!A1:B1", body["range"])
		values, _ := body["values"].([]any)
		assert.Len(t, values, 1)
		_, _ = w.Write([]byte(`{"spreadsheetId":"s-1","updatedRange":"Sheet1!A1:B1"}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_update_values", map[string]any{
		"spreadsheet_id": "s-1",
		"range":          "Sheet1!A1:B1",
		"values":         `[["a","b"]]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateValues_BadJSON(t *testing.T) {
	g := &gsheets{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gsheets_update_values", map[string]any{
		"spreadsheet_id": "s-1",
		"range":          "Sheet1!A1",
		"values":         "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "JSON 2-D array")
}

func TestUpdateValues_RAWOption(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "RAW", r.URL.Query().Get("valueInputOption"))
		_, _ = w.Write([]byte(`{"updatedRange":"x"}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gsheets_update_values", map[string]any{
		"spreadsheet_id":     "s-1",
		"range":              "Sheet1!A1",
		"values":             `[["x"]]`,
		"value_input_option": "RAW",
	})
	require.NoError(t, err)
}

// ── Handler: appendValues ──────────────────────────────────────────

func TestAppendValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, ":append")
		assert.Equal(t, "USER_ENTERED", r.URL.Query().Get("valueInputOption"))
		assert.Equal(t, "INSERT_ROWS", r.URL.Query().Get("insertDataOption"))
		_, _ = w.Write([]byte(`{"updates":{"updatedRange":"Sheet1!A5:B5"}}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_append_values", map[string]any{
		"spreadsheet_id":     "s-1",
		"range":              "Sheet1",
		"values":             `[["new","row"]]`,
		"insert_data_option": "INSERT_ROWS",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: clearValues ───────────────────────────────────────────

func TestClearValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, ":clear")
		_, _ = w.Write([]byte(`{"clearedRange":"Sheet1!A:Z"}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_clear_values", map[string]any{
		"spreadsheet_id": "s-1",
		"range":          "Sheet1!A:Z",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── Handler: batchUpdateValues ─────────────────────────────────────

func TestBatchUpdateValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/spreadsheets/s-1/values:batchUpdate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "USER_ENTERED", body["valueInputOption"])
		data, _ := body["data"].([]any)
		assert.Len(t, data, 2)
		_, _ = w.Write([]byte(`{"totalUpdatedCells":4}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_batch_update_values", map[string]any{
		"spreadsheet_id": "s-1",
		"data":           `[{"range":"Sheet1!A1","values":[["x"]]},{"range":"Sheet2!A1","values":[["y","z","w"]]}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestBatchUpdateValues_BadJSON(t *testing.T) {
	g := &gsheets{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gsheets_batch_update_values", map[string]any{
		"spreadsheet_id": "s-1",
		"data":           "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "JSON array")
}

// ── Handler: batchUpdate ───────────────────────────────────────────

func TestBatchUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/spreadsheets/s-1:batchUpdate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		reqs, _ := body["requests"].([]any)
		assert.Len(t, reqs, 1)
		assert.Equal(t, true, body["includeSpreadsheetInResponse"])
		_, _ = w.Write([]byte(`{"replies":[{}]}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gsheets_batch_update", map[string]any{
		"spreadsheet_id":                  "s-1",
		"requests":                        `[{"addSheet":{"properties":{"title":"NewTab"}}}]`,
		"include_spreadsheet_in_response": "true",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestBatchUpdate_BadJSON(t *testing.T) {
	g := &gsheets{accessToken: "tok", client: &http.Client{}, baseURL: "http://x"}
	result, err := g.Execute(context.Background(), "gsheets_batch_update", map[string]any{
		"spreadsheet_id": "s-1",
		"requests":       "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "JSON array")
}

// ── Healthy ────────────────────────────────────────────────────────

func TestHealthy_TrueOn404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, g.Healthy(context.Background()))
}

func TestHealthy_FalseOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	assert.False(t, g.Healthy(context.Background()))
}

// TestHealthy_TrueAfterRefresh verifies that an expired access token does
// not flip the health badge red so long as refresh credentials are
// configured: the 401 from the API triggers a transparent refresh
// through g.get() and the retried sentinel probe still allows a 404.
func TestHealthy_TrueAfterRefresh(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		assert.Equal(t, "refresh_token", vals.Get("grant_type"))
		_, _ = w.Write([]byte(`{"access_token":"new-token","expires_in":3600}`))
	}))
	defer tokenSrv.Close()
	googleoauth.SetTokenURLForTest(t, tokenSrv.URL)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(401)
			return
		}
		// Sentinel-probe path: 404 means auth worked, probe doc absent.
		w.WriteHeader(404)
	}))
	defer api.Close()

	g := &gsheets{
		accessToken:  "expired",
		refreshToken: "rtok",
		clientID:     "cid",
		clientSecret: "csec",
		client:       api.Client(),
		baseURL:      api.URL,
	}
	assert.True(t, g.Healthy(context.Background()))
	assert.Equal(t, "new-token", g.accessToken)
}

// ── Path escaping ───────────────────────────────────────────────────

func TestSpreadsheetIDIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"spreadsheetId":"foo bar"}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gsheets_get_spreadsheet", map[string]any{
		"spreadsheet_id": "foo bar/baz",
	})
	require.NoError(t, err)
	assert.True(t, strings.Contains(seenPath, "foo%20bar") || strings.Contains(seenPath, "foo+bar"),
		"spreadsheet id with space should be URL-escaped; got %s", seenPath)
}

func TestRangeIsURLEscaped(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"range":"Sheet 1!A1","values":[]}`))
	}))
	defer ts.Close()

	g := &gsheets{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, err := g.Execute(context.Background(), "gsheets_get_values", map[string]any{
		"spreadsheet_id": "s-1",
		"range":          "Sheet 1!A1",
	})
	require.NoError(t, err)
	// Sheet name has a space; ensure it's escaped.
	assert.True(t, strings.Contains(seenPath, "Sheet%201"),
		"range with space should be URL-escaped; got %s", seenPath)
}
