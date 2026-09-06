package paperless

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
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
	assert.Equal(t, paperlessHTTPTimeout, i.(*paperless).client.Timeout)

	for _, loopbackURL := range []string{"http://localhost:8000", "http://127.0.0.1:8000", "http://[::1]:8000"} {
		t.Run("allows loopback "+loopbackURL, func(t *testing.T) {
			assert.NoError(t, New().Configure(context.Background(), mcp.Credentials{"token": "paperless-token", "url": loopbackURL}))
		})
	}

	for name, tt := range map[string]struct {
		creds mcp.Credentials
		want  string
	}{
		"missing token":        {creds: mcp.Credentials{"url": "https://paperless.example.com"}, want: "token is required"},
		"missing url":          {creds: mcp.Credentials{"token": "paperless-token"}, want: "url is required"},
		"invalid url":          {creds: mcp.Credentials{"token": "paperless-token", "url": "not a URL"}, want: "invalid url"},
		"insecure remote url":  {creds: mcp.Credentials{"token": "paperless-token", "url": "http://paperless.example.com"}, want: "https"},
		"insecure private url": {creds: mcp.Credentials{"token": "paperless-token", "url": "http://192.168.1.5"}, want: "https"},
	} {
		t.Run(name, func(t *testing.T) {
			err := New().Configure(context.Background(), tt.creds)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range New().Tools() {
		assert.NotEmpty(t, tool.Name)
		assert.Contains(t, string(tool.Name), "paperless_")
		assert.NotEmpty(t, tool.Description)
		assert.False(t, seen[tool.Name], "duplicate tool %s", tool.Name)
		seen[tool.Name] = true
		_, handled := dispatch[tool.Name]
		assert.True(t, handled, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	toolsByName := make(map[mcp.ToolName]struct{})
	for _, tool := range New().Tools() {
		toolsByName[tool.Name] = struct{}{}
	}
	for name := range dispatch {
		_, defined := toolsByName[name]
		assert.True(t, defined, "dispatch handler %s has no tool definition", name)
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

func TestDocumentReadsExcludeOCRContent(t *testing.T) {
	const ocrText = "sensitive OCR text"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/documents/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":7,"title":"Invoice","content":"` + ocrText + `"}]}`))
		case "/api/documents/7/":
			_, _ = w.Write([]byte(`{"id":7,"title":"Invoice","content":"` + ocrText + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "paperless-token", baseURL: ts.URL, client: ts.Client()}
	for _, tt := range []struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{
		{name: "list", tool: "paperless_list_documents"},
		{name: "search", tool: "paperless_search_documents", args: map[string]any{"query": "invoice"}},
		{name: "get", tool: "paperless_get_document", args: map[string]any{"document_id": 7}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Execute(context.Background(), tt.tool, tt.args)
			require.NoError(t, err)
			assert.False(t, result.IsError)
			assert.NotContains(t, result.Data, ocrText)
			assert.NotContains(t, result.Data, `"content"`)
		})
	}
}

func TestGetDocumentOCRTextReturnsPaginatedContent(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/documents/7/", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":7,"title":"Invoice","content":"abcdef","unrelated":"ignored"}`))
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "paperless-token", baseURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "paperless_get_document_ocr_text", map[string]any{"document_id": 7, "offset": 2, "limit": 3})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.JSONEq(t, `{"document_id":7,"offset":2,"limit":3,"total_chars":6,"content":"cde","has_more":true,"next_offset":5}`, result.Data)
	assert.Equal(t, 1, requests)
}

func TestGetDocumentOCRTextArgumentValidation(t *testing.T) {
	p := &paperless{}
	for _, tt := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "negative offset", args: map[string]any{"document_id": 1, "offset": -1}, want: "offset must be non-negative"},
		{name: "zero limit", args: map[string]any{"document_id": 1, "limit": 0}, want: "limit must be positive"},
		{name: "limit over maximum", args: map[string]any{"document_id": 1, "limit": 20001}, want: "limit must not exceed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Execute(context.Background(), "paperless_get_document_ocr_text", tt.args)
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
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte{0x00, 0xff, 0x10})
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
		{"download", "paperless_download_document", map[string]any{"document_id": 7}, base64.StdEncoding.EncodeToString([]byte{0x00, 0xff, 0x10})},
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

func TestDownloadDocumentReturnsBinaryEnvelope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf; charset=binary")
		_, _ = w.Write([]byte{0x00, 0xff, 0x10})
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "paperless_download_document", map[string]any{"document_id": 1})
	require.NoError(t, err)
	assert.JSONEq(t, `{"content_type":"application/pdf; charset=binary","bytes":3,"content_base64":"AP8Q"}`, result.Data)
}

func TestGetDocumentThumbnailReturnsNativeImage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/documents/7/thumb/", r.URL.Path)
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("image"))
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "paperless_get_document_thumbnail", map[string]any{"document_id": 7})
	require.NoError(t, err)
	assert.JSONEq(t, `{"document_id":7,"content_type":"image/webp","bytes":5}`, result.Data)
	require.Len(t, result.Media, 1)
	assert.Equal(t, []byte("image"), result.Media[0].Data)
	assert.Equal(t, "image/webp", result.Media[0].MIMEType)
	assert.Equal(t, "paperless-document-7.webp", result.Media[0].Name)
}

func TestGetDocumentPreviewReturnsImageOrPDF(t *testing.T) {
	contentType := "image/png"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/documents/7/preview/", r.URL.Path)
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte("preview"))
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "paperless_get_document_preview", map[string]any{"document_id": 7})
	require.NoError(t, err)
	require.Len(t, result.Media, 1)
	assert.Equal(t, "image/png", result.Media[0].MIMEType)

	contentType = "application/pdf"
	result, err = p.Execute(context.Background(), "paperless_get_document_preview", map[string]any{"document_id": 7})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	require.Len(t, result.Media, 1)
	assert.Equal(t, "application/pdf", result.Media[0].MIMEType)
	assert.Equal(t, "paperless-document-7.pdf", result.Media[0].Name)
}

func TestGetDocumentPreviewUsesPredictableJPEGExtension(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpeg"))
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "paperless_get_document_preview", map[string]any{"document_id": 7})
	require.NoError(t, err)
	require.Len(t, result.Media, 1)
	assert.Equal(t, "paperless-document-7.jpg", result.Media[0].Name)
}

func TestDocumentVisionResponseLimits(t *testing.T) {
	p := New().(*paperless)
	for _, tool := range []mcp.ToolName{"paperless_get_document_thumbnail", "paperless_get_document_preview"} {
		limit, ok := p.MaxResponseBytesForTool(tool)
		assert.True(t, ok)
		assert.Equal(t, paperlessImageSizeLimit, limit)
	}
	_, ok := p.MaxResponseBytesForTool("paperless_get_document")
	assert.False(t, ok)
}

func TestDocumentVisionToolDescriptions(t *testing.T) {
	definitions := make(map[mcp.ToolName]mcp.ToolDefinition)
	for _, tool := range New().Tools() {
		definitions[tool.Name] = tool
	}
	assert.Contains(t, definitions["paperless_get_document_thumbnail"].Description, "Start here")
	assert.Contains(t, definitions["paperless_get_document_thumbnail"].Description, "PDF")
	assert.Contains(t, definitions["paperless_get_document_preview"].Description, "image")
	assert.Contains(t, definitions["paperless_get_document_ocr_text"].Description, "thumbnail")
}

func TestPaperlessResponseSizeLimits(t *testing.T) {
	body := strings.Repeat("x", 2*1024*1024+1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
	_, err := p.doRequest(context.Background(), http.MethodGet, "/json", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response exceeds")

	_, err = p.doMultipartRequest(context.Background(), "/multipart", strings.NewReader("body"), mime.TypeByExtension(".txt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response exceeds")
}

func TestPaperlessDownloadSizeLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", 1024*1024+1))
	}))
	t.Cleanup(ts.Close)

	p := &paperless{token: "token", baseURL: ts.URL, client: ts.Client()}
	result, err := p.Execute(context.Background(), "paperless_download_document", map[string]any{"document_id": 1})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "response exceeds")
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
