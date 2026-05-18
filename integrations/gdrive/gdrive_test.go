package gdrive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "gdrive", i.Name())
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
	g := &gdrive{client: &http.Client{}, baseURL: "https://www.googleapis.com/drive/v3"}
	err := g.Configure(context.Background(), mcp.Credentials{
		"access_token": "ya29.test",
		"base_url":     "https://custom.example.com/",
		"upload_url":   "https://upload.example.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com", g.baseURL)
	assert.Equal(t, "https://upload.example.com", g.uploadURL)
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

func TestTools_AllHaveGdrivePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "gdrive_", "tool %s missing gdrive_ prefix", tool.Name)
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
	g := &gdrive{accessToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdrive_nonexistent", nil)
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

func TestIsTextType(t *testing.T) {
	cases := map[string]bool{
		"text/plain":               true,
		"text/csv; charset=utf-8":  true,
		"application/json":         true,
		"application/javascript":   true,
		"application/x-yaml":       true,
		"application/octet-stream": false,
		"image/png":                false,
		"application/pdf":          false,
	}
	for ct, want := range cases {
		assert.Equal(t, want, isTextType(ct), "isTextType(%q)", ct)
	}
}

func TestAddSupportsAllDrives_DefaultsTrue(t *testing.T) {
	params := map[string]string{}
	addSupportsAllDrives(params, mcp.NewArgs(map[string]any{}))
	assert.Equal(t, "true", params["supportsAllDrives"])
}

func TestAddSupportsAllDrives_RespectsExplicit(t *testing.T) {
	params := map[string]string{}
	addSupportsAllDrives(params, mcp.NewArgs(map[string]any{"supports_all_drives": "false"}))
	assert.Equal(t, "false", params["supportsAllDrives"])
}

func TestGenerateRequestID(t *testing.T) {
	a := generateRequestID()
	b := generateRequestID()
	assert.NotEqual(t, a, b)
	assert.True(t, strings.HasPrefix(a, "switchboard-"))
	assert.Len(t, a, len("switchboard-")+16)
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"name":"My File"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := g.get(context.Background(), "/files/abc")
	require.NoError(t, err)
	assert.Contains(t, string(data), "My File")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"message":"Forbidden"}}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "bad", client: ts.Client(), baseURL: ts.URL}
	_, err := g.get(context.Background(), "/files/abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gdrive API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, err := g.del(context.Background(), "/files/abc")
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRaw_ReturnsBytesAndContentType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	data, ct, err := g.doRaw(context.Background(), "GET", "/files/abc?alt=media")
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
	assert.Equal(t, "text/plain; charset=utf-8", ct)
}

func TestDoRaw_429ReturnsRetryableError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, _, err := g.doRaw(context.Background(), "GET", "/files/abc?alt=media")
	require.Error(t, err)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
}

func TestDoRaw_503ReturnsRetryableError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	_, _, err := g.doRaw(context.Background(), "GET", "/files/abc?alt=media")
	require.Error(t, err)
	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 503, re.StatusCode)
}

// --- handler integration tests ---

func TestListFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/files")
		assert.Equal(t, "name contains 'foo'", r.URL.Query().Get("q"))
		assert.Equal(t, "true", r.URL.Query().Get("supportsAllDrives"))
		_, _ = w.Write([]byte(`{"files":[{"id":"f1","name":"foo.txt","mimeType":"text/plain"}]}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_list_files", map[string]any{
		"q": "name contains 'foo'",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "f1")
}

func TestGetFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/files/f1")
		assert.Equal(t, "*", r.URL.Query().Get("fields"))
		_, _ = w.Write([]byte(`{"id":"f1","name":"foo.txt"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_get_file", map[string]any{
		"file_id": "f1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "f1")
}

func TestDownloadFile_Text(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "media", r.URL.Query().Get("alt"))
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_download_file", map[string]any{
		"file_id": "f1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"content":"hello"`)
	assert.Contains(t, result.Data, `"content_type":"text/plain"`)
}

func TestDownloadFile_Binary(t *testing.T) {
	binary := []byte{0x00, 0x01, 0x02, 0xff}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(binary)
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_download_file", map[string]any{
		"file_id": "f1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	expected := base64.StdEncoding.EncodeToString(binary)
	assert.Contains(t, result.Data, `"content_base64":"`+expected+`"`)
}

func TestDownloadFile_Truncated(t *testing.T) {
	body := strings.Repeat("a", 100)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_download_file", map[string]any{
		"file_id":   "f1",
		"max_bytes": 10,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"truncated":true`)
	assert.Contains(t, result.Data, `"content":"aaaaaaaaaa"`)
}

func TestExportFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/files/doc1/export")
		assert.Equal(t, "text/plain", r.URL.Query().Get("mimeType"))
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("exported doc body"))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_export_file", map[string]any{
		"file_id":   "doc1",
		"mime_type": "text/plain",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "exported doc body")
}

func TestCreateFile_MetadataOnly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/files")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "new.txt", body["name"])
		assert.Equal(t, "text/plain", body["mimeType"])
		_, _ = w.Write([]byte(`{"id":"new_id","name":"new.txt"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL, uploadURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_file", map[string]any{
		"name":      "new.txt",
		"mime_type": "text/plain",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new_id")
}

func TestCreateFile_WithContent_Multipart(t *testing.T) {
	// The upload endpoint receives multipart/related: metadata JSON +
	// content body. Verify the boundary structure and that both parts
	// are present in the request.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "multipart", r.URL.Query().Get("uploadType"))
		ct := r.Header.Get("Content-Type")
		assert.Contains(t, ct, "multipart/related")
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		assert.Contains(t, body, `"name":"upload.txt"`)
		assert.Contains(t, body, "hello multipart")
		_, _ = w.Write([]byte(`{"id":"uploaded_id"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL, uploadURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_file", map[string]any{
		"name":      "upload.txt",
		"mime_type": "text/plain",
		"content":   "hello multipart",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "uploaded_id")
}

func TestCreateFile_RawBodyOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "from-body.txt", body["name"])
		// convenience args ignored when body is supplied
		assert.NotContains(t, body, "description")
		_, _ = w.Write([]byte(`{"id":"raw_id"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_file", map[string]any{
		"name":        "ignored",
		"description": "also ignored",
		"body":        `{"name":"from-body.txt"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateFile_InvalidBodyJSON(t *testing.T) {
	g := &gdrive{accessToken: "tok", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := g.Execute(context.Background(), "gdrive_create_file", map[string]any{
		"body": "{bad json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for body")
}

func TestUpdateFile_Metadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/files/f1")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "renamed.txt", body["name"])
		_, _ = w.Write([]byte(`{"id":"f1","name":"renamed.txt"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL, uploadURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_update_file", map[string]any{
		"file_id": "f1",
		"name":    "renamed.txt",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "renamed.txt")
}

func TestTrashFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, true, body["trashed"])
		_, _ = w.Write([]byte(`{"id":"f1","trashed":true}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_trash_file", map[string]any{
		"file_id": "f1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateFolder(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Engineering", body["name"])
		assert.Equal(t, "application/vnd.google-apps.folder", body["mimeType"])
		parents, _ := body["parents"].([]any)
		assert.Len(t, parents, 1)
		_, _ = w.Write([]byte(`{"id":"folder_id"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_folder", map[string]any{
		"name":    "Engineering",
		"parents": "parent_id",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "folder_id")
}

func TestListPermissions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/files/f1/permissions")
		_, _ = w.Write([]byte(`{"permissions":[{"id":"p1","role":"writer","emailAddress":"alice@example.com"}]}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_list_permissions", map[string]any{
		"file_id": "f1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "writer")
}

func TestCreatePermission(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "writer", body["role"])
		assert.Equal(t, "user", body["type"])
		assert.Equal(t, "alice@example.com", body["emailAddress"])
		_, _ = w.Write([]byte(`{"id":"p1"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_permission", map[string]any{
		"file_id":       "f1",
		"role":          "writer",
		"type":          "user",
		"email_address": "alice@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListDrives(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/drives")
		_, _ = w.Write([]byte(`{"drives":[{"id":"d1","name":"Eng"}]}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_list_drives", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Eng")
}

func TestCreateDrive_AutoRequestID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.URL.Query().Get("requestId")
		assert.True(t, strings.HasPrefix(rid, "switchboard-"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Engineering", body["name"])
		_, _ = w.Write([]byte(`{"id":"d1","name":"Engineering"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_drive", map[string]any{
		"name": "Engineering",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetAbout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/about")
		assert.Equal(t, "*", r.URL.Query().Get("fields"))
		_, _ = w.Write([]byte(`{"user":{"emailAddress":"me@example.com"},"storageQuota":{"usage":"100"}}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_get_about", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "me@example.com")
}

func TestCreateComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/files/f1/comments")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "looks good", body["content"])
		_, _ = w.Write([]byte(`{"id":"c1","content":"looks good"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_comment", map[string]any{
		"file_id": "f1",
		"content": "looks good",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateReply(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/files/f1/comments/c1/replies")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "thanks", body["content"])
		assert.Equal(t, "resolve", body["action"])
		_, _ = w.Write([]byte(`{"id":"r1"}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_create_reply", map[string]any{
		"file_id":    "f1",
		"comment_id": "c1",
		"content":    "thanks",
		"action":     "resolve",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateRevision_KeepForever(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/files/f1/revisions/r1")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, true, body["keepForever"])
		_, _ = w.Write([]byte(`{"id":"r1","keepForever":true}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_update_revision", map[string]any{
		"file_id":      "f1",
		"revision_id":  "r1",
		"keep_forever": "true",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGenerateIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/files/generateIds")
		assert.Equal(t, "5", r.URL.Query().Get("count"))
		_, _ = w.Write([]byte(`{"ids":["a","b","c","d","e"]}`))
	}))
	defer ts.Close()

	g := &gdrive{accessToken: "tok", client: ts.Client(), baseURL: ts.URL}
	result, err := g.Execute(context.Background(), "gdrive_generate_ids", map[string]any{
		"count": "5",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"a"`)
}

// --- result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"id":"f1"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"id":"f1"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}
