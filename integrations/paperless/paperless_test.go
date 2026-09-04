package paperless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAndConfigure(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "paperless", i.Name())
	assert.NoError(t, i.Configure(context.Background(), mcp.Credentials{
		"token": "paperless-token",
		"url":   "https://paperless.example.com/",
	}))

	for name, creds := range map[string]mcp.Credentials{
		"missing token": {"url": "https://paperless.example.com"},
		"missing url":   {"token": "paperless-token"},
	} {
		t.Run(name, func(t *testing.T) {
			err := New().Configure(context.Background(), creds)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "required")
		})
	}
}

func TestToolsAndDispatchParity(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.NotEmpty(t, tool.Name)
		assert.Contains(t, string(tool.Name), "paperless_")
		assert.NotEmpty(t, tool.Description)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		_, handled := dispatch[tool.Name]
		assert.True(t, handled, "tool %s has no dispatch handler", tool.Name)
	}
	for name := range dispatch {
		assert.True(t, seen[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestCreateDocument(t *testing.T) {
	var gotTitle, gotContent, gotFilename string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/documents/post_document/", r.URL.Path)
		assert.Equal(t, "Token paperless-token", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1<<20))
		gotTitle = r.FormValue("title")
		file, header, err := r.FormFile("document")
		require.NoError(t, err)
		defer func() { _ = file.Close() }()
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		gotContent = string(content)
		gotFilename = header.Filename
		_, _ = w.Write([]byte(`{"task_id":"123"}`))
	}))
	defer ts.Close()

	p := &paperless{token: "paperless-token", baseURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "paperless_create_document", map[string]any{
		"title":   "Live test document",
		"content": "Safe test content",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "123")
	assert.Equal(t, "Live test document", gotTitle)
	assert.Equal(t, "Safe test content", gotContent)
	assert.Equal(t, "document.txt", gotFilename)
}

func TestCreateDocumentArgumentErrors(t *testing.T) {
	p := &paperless{}
	for _, tt := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing title", args: map[string]any{"content": "Safe test content"}, want: "title is required"},
		{name: "missing content", args: map[string]any{"title": "Live test document"}, want: "content is required"},
		{name: "invalid title type", args: map[string]any{"title": []string{"invalid"}, "content": "Safe test content"}, want: `parameter "title"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Execute(context.Background(), "paperless_create_document", tt.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
}

func TestExecuteDocumentOperations(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Token paperless-token", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/documents/":
			gotQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":7,"title":"Invoice"}]}`))
		case "/api/documents/7/":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(`{"id":7,"title":"Invoice"}`))
			case http.MethodPatch:
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "Paid invoice", body["title"])
				_, _ = w.Write([]byte(`{"id":7,"title":"Paid invoice"}`))
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			}
		case "/api/documents/7/download/":
			_, _ = w.Write([]byte("PDF bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	p := &paperless{token: "paperless-token", baseURL: ts.URL, client: ts.Client()}
	for _, tt := range []struct {
		name string
		tool mcp.ToolName
		args map[string]any
		want string
	}{
		{"list", "paperless_list_documents", map[string]any{"page": 2, "page_size": 25}, "Invoice"},
		{"search", "paperless_search_documents", map[string]any{"query": "invoice"}, "Invoice"},
		{"get", "paperless_get_document", map[string]any{"document_id": 7}, "Invoice"},
		{"update", "paperless_update_document", map[string]any{"document_id": 7, "title": "Paid invoice"}, "Paid invoice"},
		{"delete", "paperless_delete_document", map[string]any{"document_id": 7}, "success"},
		{"download", "paperless_download_document", map[string]any{"document_id": 7}, "PDF bytes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Contains(t, result.Data, tt.want)
		})
	}
	assert.Contains(t, gotQuery, "query=invoice")
}

func TestMetadataAndHTTPFailures(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags/", "/api/correspondents/", "/api/document_types/", "/api/storage_paths/", "/api/custom_fields/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"name":"Example"}]}`))
		case "/api/users/me/":
			_, _ = w.Write([]byte(`{"username":"admin"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"Invalid token."}`))
		}
	}))
	defer ts.Close()

	p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
	for _, tool := range []mcp.ToolName{"paperless_list_tags", "paperless_list_correspondents", "paperless_list_document_types", "paperless_list_storage_paths", "paperless_list_custom_fields"} {
		result, err := p.Execute(context.Background(), tool, nil)
		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Data, "Example")
	}
	assert.True(t, p.Healthy(context.Background()))

	_, err := p.get(context.Background(), "/api/missing/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paperless API error (401)")
}

func TestExecuteUnknownTool(t *testing.T) {
	result, err := New().Execute(context.Background(), "paperless_unknown", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}
