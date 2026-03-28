package readarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

var (
	_ mcp.Integration                = (*readarr)(nil)
	_ mcp.FieldCompactionIntegration = (*readarr)(nil)
	_ mcp.PlainTextCredentials       = (*readarr)(nil)
)

type readarr struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &readarr{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *readarr) Name() string { return "readarr" }

func (r *readarr) PlainTextKeys() []string {
	return []string{"base_url"}
}

func (r *readarr) Configure(_ context.Context, creds mcp.Credentials) error {
	r.apiKey = creds["api_key"]
	if r.apiKey == "" {
		return fmt.Errorf("readarr: api_key is required")
	}
	r.baseURL = creds["base_url"]
	if r.baseURL == "" {
		return fmt.Errorf("readarr: base_url is required")
	}
	r.baseURL = strings.TrimRight(r.baseURL, "/")
	return nil
}

func (r *readarr) Healthy(ctx context.Context) bool {
	if r.apiKey == "" || r.baseURL == "" {
		return false
	}
	_, err := r.get(ctx, "/api/v1/system/status")
	return err == nil
}

func (r *readarr) Tools() []mcp.ToolDefinition {
	return tools
}

func (r *readarr) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, r, args)
}

func (r *readarr) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- HTTP helpers ---

func (r *readarr) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", r.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("readarr API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("readarr API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (r *readarr) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return r.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (r *readarr) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return r.doRequest(ctx, "POST", path, body)
}

func (r *readarr) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return r.doRequest(ctx, "PUT", path, body)
}

func (r *readarr) delWithQuery(ctx context.Context, path string) (json.RawMessage, error) {
	return r.doRequest(ctx, "DELETE", path, nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error)

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Books
	"readarr_list_books":    listBooks,
	"readarr_get_book":      getBook,
	"readarr_search_books":  searchBooks,
	"readarr_monitor_books": monitorBooks,

	// Authors
	"readarr_list_authors": listAuthors,
	"readarr_get_author":   getAuthor,

	// Calendar
	"readarr_get_calendar": getCalendar,

	// Wanted / Missing
	"readarr_get_missing": getMissing,
	"readarr_get_cutoff":  getCutoff,

	// Queue
	"readarr_get_queue":         getQueue,
	"readarr_delete_queue_item": deleteQueueItem,
	"readarr_delete_queue_bulk": deleteQueueBulk,
	"readarr_grab_queue_item":   grabQueueItem,

	// History
	"readarr_get_history":        getHistory,
	"readarr_get_history_author": getHistoryAuthor,
	"readarr_get_history_since":  getHistorySince,

	// Commands
	"readarr_list_commands": listCommands,
	"readarr_run_command":   runCommand,
	"readarr_get_command":   getCommand,

	// System
	"readarr_get_system_status": getSystemStatus,

	// Root Folders
	"readarr_list_root_folders": listRootFolders,

	// Quality Profiles
	"readarr_list_quality_profiles": listQualityProfiles,

	// Metadata Profiles
	"readarr_list_metadata_profiles": listMetadataProfiles,

	// Tags
	"readarr_list_tags": listTags,
}
