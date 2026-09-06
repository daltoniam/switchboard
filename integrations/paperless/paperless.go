// Package paperless integrates Paperless-ngx document management instances.
package paperless

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("paperless", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

var (
	_ mcp.Integration                = (*paperless)(nil)
	_ mcp.FieldCompactionIntegration = (*paperless)(nil)
	_ mcp.PlainTextCredentials       = (*paperless)(nil)
	_ mcp.PlaceholderHints           = (*paperless)(nil)
)

const (
	paperlessHTTPTimeout       = 30 * time.Second
	paperlessResponseSizeLimit = 2 * 1024 * 1024
	paperlessDownloadSizeLimit = 1024 * 1024
	paperlessImageSizeLimit    = 5 * 1024 * 1024
	defaultOCRTextLimit        = 10_000
	maxOCRTextLimit            = 20_000
)

type paperless struct {
	token   string
	baseURL string
	client  *http.Client
}

// New creates a Paperless-ngx integration.
func New() mcp.Integration {
	return &paperless{client: &http.Client{Timeout: paperlessHTTPTimeout}}
}

func (p *paperless) Name() string { return "paperless" }

func (p *paperless) PlainTextKeys() []string { return []string{"url"} }

func (p *paperless) Placeholders() map[string]string {
	return map[string]string{"url": "https://paperless.example.com"}
}

func (p *paperless) Configure(_ context.Context, creds mcp.Credentials) error {
	p.token = creds["token"]
	p.baseURL = strings.TrimRight(creds["url"], "/")
	if p.token == "" {
		return fmt.Errorf("paperless: token is required")
	}
	if p.baseURL == "" {
		return fmt.Errorf("paperless: url is required")
	}
	parsed, err := url.ParseRequestURI(p.baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("paperless: invalid url")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("paperless: url must use https unless the host is loopback")
	}
	if p.client == nil {
		p.client = &http.Client{Timeout: paperlessHTTPTimeout}
	} else if p.client.Timeout == 0 {
		p.client.Timeout = paperlessHTTPTimeout
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (p *paperless) Healthy(ctx context.Context) bool {
	if p.client == nil || p.token == "" || p.baseURL == "" {
		return false
	}
	_, err := p.get(ctx, "/api/users/me/")
	return err == nil
}

func (p *paperless) Tools() []mcp.ToolDefinition { return tools }

func (p *paperless) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (p *paperless) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	return fn(ctx, p, args)
}

func (p *paperless) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal Paperless request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	data, _, err := p.doRequestWithLimit(ctx, method, path, reader, "application/json", paperlessResponseSizeLimit)
	return data, err
}

func (p *paperless) get(ctx context.Context, path string) ([]byte, error) {
	return p.doRequest(ctx, http.MethodGet, path, nil)
}

func (p *paperless) doRequestWithLimit(ctx context.Context, method, path string, body io.Reader, contentType string, limit int) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, body)
	if err != nil {
		return nil, nil, fmt.Errorf("create Paperless request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+p.token)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("make Paperless request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read Paperless response: %w", err)
	}
	if len(data) > limit {
		return nil, nil, fmt.Errorf("paperless response exceeds %d bytes", limit)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		retryErr := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("paperless API error (%d): %s", resp.StatusCode, data)}
		retryErr.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, nil, retryErr
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, nil, fmt.Errorf("paperless API error (%d): %s", resp.StatusCode, data)
	}
	if resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return []byte(`{"status":"success"}`), resp.Header, nil
	}
	return data, resp.Header, nil
}

func listDocuments(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	page := r.OptInt("page", 1)
	pageSize := r.OptInt("page_size", 25)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	data, err := p.get(ctx, "/api/documents/?"+query.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return withoutOCRContentFromDocumentList(data)
}

func searchDocuments(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queryText := r.Str("query")
	if queryText == "" && r.Err() == nil {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}
	page := r.OptInt("page", 1)
	pageSize := r.OptInt("page_size", 25)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	query := url.Values{"query": {queryText}, "page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	data, err := p.get(ctx, "/api/documents/?"+query.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return withoutOCRContentFromDocumentList(data)
}

func createDocument(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	content := r.Str("content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if title == "" {
		return mcp.ErrResult(fmt.Errorf("title is required"))
	}
	if content == "" {
		return mcp.ErrResult(fmt.Errorf("content is required"))
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("title", title); err != nil {
		return mcp.ErrResult(fmt.Errorf("write Paperless title field: %w", err))
	}
	file, err := writer.CreateFormFile("document", "document.txt")
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("create Paperless document field: %w", err))
	}
	if _, err := io.WriteString(file, content); err != nil {
		return mcp.ErrResult(fmt.Errorf("write Paperless document content: %w", err))
	}
	if err := writer.Close(); err != nil {
		return mcp.ErrResult(fmt.Errorf("close Paperless multipart request: %w", err))
	}

	data, err := p.doMultipartRequest(ctx, "/api/documents/post_document/", &body, writer.FormDataContentType())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func (p *paperless) doMultipartRequest(ctx context.Context, path string, body io.Reader, contentType string) ([]byte, error) {
	data, _, err := p.doRequestWithLimit(ctx, http.MethodPost, path, body, contentType, paperlessResponseSizeLimit)
	return data, err
}

func getDocument(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	id, err := documentID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, fmt.Sprintf("/api/documents/%d/", id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return withoutOCRContent(data)
}

func getDocumentThumbnail(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	return getDocumentImage(ctx, p, args, "thumb")
}

func getDocumentPreview(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	return getDocumentImage(ctx, p, args, "preview")
}

func getDocumentImage(ctx context.Context, p *paperless, args map[string]any, variant string) (*mcp.ToolResult, error) {
	id, err := documentID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, header, err := p.doRequestWithLimit(ctx, http.MethodGet, fmt.Sprintf("/api/documents/%d/%s/", id, variant), nil, "", paperlessImageSizeLimit)
	if err != nil {
		return mcp.ErrResult(err)
	}
	contentType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil || (!strings.HasPrefix(contentType, "image/") && contentType != "application/pdf") {
		return mcp.ErrResult(fmt.Errorf("paperless %s returned unsupported content type %q", variant, header.Get("Content-Type")))
	}
	if variant == "thumb" && !strings.HasPrefix(contentType, "image/") {
		return mcp.ErrResult(fmt.Errorf("paperless thumbnail returned %q, not an image", header.Get("Content-Type")))
	}
	metadata, err := json.Marshal(map[string]any{
		"document_id":  id,
		"content_type": contentType,
		"bytes":        len(data),
	})
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("encode Paperless image metadata: %w", err))
	}
	nameExtension := ".pdf"
	if contentType != "application/pdf" {
		extensions, _ := mime.ExtensionsByType(contentType)
		nameExtension = ""
		if len(extensions) > 0 {
			nameExtension = extensions[0]
		}
	}
	return mcp.MediaResult(string(metadata), data, contentType, fmt.Sprintf("paperless-document-%d%s", id, nameExtension))
}

func getDocumentOCRText(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	id, err := documentID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	offset, limit, err := ocrTextPageArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := p.get(ctx, fmt.Sprintf("/api/documents/%d/", id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	var document struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return mcp.ErrResult(fmt.Errorf("decode Paperless document content: %w", err))
	}

	totalChars := len([]rune(document.Content))
	if offset > totalChars {
		offset = totalChars
	}
	content := []rune(document.Content)
	end := offset + min(limit, totalChars-offset)
	result := map[string]any{
		"document_id": id,
		"offset":      offset,
		"limit":       limit,
		"total_chars": totalChars,
		"content":     string(content[offset:end]),
		"has_more":    end < totalChars,
	}
	if end < totalChars {
		result["next_offset"] = end
	}
	return mcp.JSONResult(result)
}

func withoutOCRContent(data []byte) (*mcp.ToolResult, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return mcp.ErrResult(fmt.Errorf("decode Paperless document: %w", err))
	}
	delete(document, "content")
	return mcp.JSONResult(document)
}

func withoutOCRContentFromDocumentList(data []byte) (*mcp.ToolResult, error) {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(data, &response); err != nil {
		return mcp.ErrResult(fmt.Errorf("decode Paperless document list: %w", err))
	}
	var documents []map[string]json.RawMessage
	if err := json.Unmarshal(response["results"], &documents); err != nil {
		return mcp.ErrResult(fmt.Errorf("decode Paperless document results: %w", err))
	}
	for _, document := range documents {
		delete(document, "content")
	}
	results, err := json.Marshal(documents)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("encode Paperless document results: %w", err))
	}
	response["results"] = results
	return mcp.JSONResult(response)
}

func ocrTextPageArgs(args map[string]any) (offset, limit int, err error) {
	if value, ok := args["offset"]; ok {
		offset, err = mcp.ArgInt(map[string]any{"offset": value}, "offset")
		if err != nil {
			return 0, 0, err
		}
		if offset < 0 {
			return 0, 0, fmt.Errorf("offset must be non-negative")
		}
	}
	limit = defaultOCRTextLimit
	if value, ok := args["limit"]; ok {
		limit, err = mcp.ArgInt(map[string]any{"limit": value}, "limit")
		if err != nil {
			return 0, 0, err
		}
		if limit <= 0 {
			return 0, 0, fmt.Errorf("limit must be positive")
		}
	}
	if limit > maxOCRTextLimit {
		return 0, 0, fmt.Errorf("limit must not exceed %d", maxOCRTextLimit)
	}
	return offset, limit, nil
}

func updateDocument(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	id, err := documentID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := make(map[string]any)
	for _, key := range []string{"title", "correspondent", "document_type", "storage_path", "archive_serial_number", "created", "added", "tags", "custom_fields"} {
		if value, ok := args[key]; ok {
			body[key] = value
		}
	}
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("at least one document field is required"))
	}
	data, err := p.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/api/documents/%d/", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDocument(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	id, err := documentID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/documents/%d/", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func downloadDocument(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	id, err := documentID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, header, err := p.doRequestWithLimit(ctx, http.MethodGet, fmt.Sprintf("/api/documents/%d/download/", id), nil, "", paperlessDownloadSizeLimit)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"content_type":   header.Get("Content-Type"),
		"bytes":          len(data),
		"content_base64": base64.StdEncoding.EncodeToString(data),
	})
}

func listMetadata(path string) handlerFunc {
	return func(ctx context.Context, p *paperless, _ map[string]any) (*mcp.ToolResult, error) {
		data, err := p.get(ctx, path)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func documentID(args map[string]any) (int, error) {
	r := mcp.NewArgs(args)
	id := r.Int("document_id")
	if err := r.Err(); err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("document_id is required")
	}
	return id, nil
}

type handlerFunc func(context.Context, *paperless, map[string]any) (*mcp.ToolResult, error)
