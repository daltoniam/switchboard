// Package paperless integrates Paperless-ngx document management instances.
package paperless

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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

type paperless struct {
	token   string
	baseURL string
	client  *http.Client
}

// New creates a Paperless-ngx integration.
func New() mcp.Integration {
	return &paperless{client: &http.Client{}}
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
	return nil
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
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create Paperless request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+p.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("make Paperless request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Paperless response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		retryErr := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("paperless API error (%d): %s", resp.StatusCode, data)}
		retryErr.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, retryErr
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("paperless API error (%d): %s", resp.StatusCode, data)
	}
	if resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return []byte(`{"status":"success"}`), nil
	}
	return data, nil
}

func (p *paperless) get(ctx context.Context, path string) ([]byte, error) {
	return p.doRequest(ctx, http.MethodGet, path, nil)
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
	return mcp.RawResult(data)
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
	return mcp.RawResult(data)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create Paperless request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+p.token)
	req.Header.Set("Content-Type", contentType)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("make Paperless request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Paperless response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		retryErr := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("paperless API error (%d): %s", resp.StatusCode, data)}
		retryErr.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, retryErr
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("paperless API error (%d): %s", resp.StatusCode, data)
	}
	if resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return []byte(`{"status":"success"}`), nil
	}
	return data, nil
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
	return mcp.RawResult(data)
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
	data, err := p.get(ctx, fmt.Sprintf("/api/documents/%d/download/", id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(data)}, nil
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
