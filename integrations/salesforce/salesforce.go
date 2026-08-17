package salesforce

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("salesforce", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type salesforce struct {
	accessToken string
	instanceURL string
	apiVersion  string
	client      *http.Client
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var (
	_ mcp.Integration                = (*salesforce)(nil)
	_ mcp.FieldCompactionIntegration = (*salesforce)(nil)
	_ mcp.PlainTextCredentials       = (*salesforce)(nil)
	_ mcp.PlaceholderHints           = (*salesforce)(nil)
	_ mcp.OptionalCredentials        = (*salesforce)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*salesforce)(nil)
)

func (s *salesforce) PlainTextKeys() []string { return []string{"instance_url", "api_version"} }

func (s *salesforce) Placeholders() map[string]string {
	return map[string]string{
		"access_token": "Salesforce OAuth access token",
		"instance_url": "https://yourorg.my.salesforce.com",
		"api_version":  "v62.0 (default)",
	}
}

func (s *salesforce) OptionalKeys() []string { return []string{"api_version"} }

func New() mcp.Integration {
	return &salesforce{
		client:     &http.Client{Timeout: 30 * time.Second},
		apiVersion: "v62.0",
	}
}

func (s *salesforce) Name() string { return "salesforce" }

func (s *salesforce) Configure(_ context.Context, creds mcp.Credentials) error {
	s.accessToken = creds["access_token"]
	s.instanceURL = strings.TrimRight(creds["instance_url"], "/")
	if v := creds["api_version"]; v != "" {
		s.apiVersion = v
	}
	if s.accessToken == "" {
		return fmt.Errorf("salesforce: access_token is required")
	}
	if s.instanceURL == "" {
		return fmt.Errorf("salesforce: instance_url is required")
	}
	return nil
}

func (s *salesforce) Healthy(ctx context.Context) bool {
	_, err := s.get(ctx, "%s/limits", s.ver())
	return err == nil
}

func (s *salesforce) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *salesforce) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *salesforce) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (s *salesforce) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

// --- HTTP helpers ---

func (s *salesforce) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.instanceURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("salesforce API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("salesforce API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (s *salesforce) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (s *salesforce) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "POST", path, body)
}

func (s *salesforce) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "PATCH", path, body)
}

func (s *salesforce) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, s *salesforce, args map[string]any) (*mcp.ToolResult, error)

// --- Argument helpers ---

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

// ver returns the REST API base path for the configured API version.
func (s *salesforce) ver() string {
	return "/services/data/" + s.apiVersion
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// SObject CRUD
	mcp.ToolName("salesforce_describe_global"):           describeGlobal,
	mcp.ToolName("salesforce_describe_sobject"):          describeSObject,
	mcp.ToolName("salesforce_get_record"):                getRecord,
	mcp.ToolName("salesforce_create_record"):             createRecord,
	mcp.ToolName("salesforce_update_record"):             updateRecord,
	mcp.ToolName("salesforce_delete_record"):             deleteRecord,
	mcp.ToolName("salesforce_get_record_by_external_id"): getRecordByExternalID,
	mcp.ToolName("salesforce_upsert_by_external_id"):     upsertByExternalID,

	// Queries
	mcp.ToolName("salesforce_query"):      query,
	mcp.ToolName("salesforce_query_more"): queryMore,
	mcp.ToolName("salesforce_search"):     search,

	// Metadata & Org
	mcp.ToolName("salesforce_list_api_versions"):    listAPIVersions,
	mcp.ToolName("salesforce_get_limits"):           getLimits,
	mcp.ToolName("salesforce_list_recently_viewed"): listRecentlyViewed,

	// Composite
	mcp.ToolName("salesforce_composite_batch"):     compositeBatch,
	mcp.ToolName("salesforce_sobject_collections"): sObjectCollections,
}
