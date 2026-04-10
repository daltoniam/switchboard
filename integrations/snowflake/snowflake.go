package snowflake

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

var (
	_ mcp.Integration                = (*snowflake)(nil)
	_ mcp.FieldCompactionIntegration = (*snowflake)(nil)
	_ mcp.PlainTextCredentials       = (*snowflake)(nil)
	_ mcp.PlaceholderHints           = (*snowflake)(nil)
	_ mcp.OptionalCredentials        = (*snowflake)(nil)
)

type snowflake struct {
	client       *http.Client
	token        string
	baseURL      string
	warehouse    string
	database     string
	schema       string
	role         string
	account      string
	user         string
	semanticView string
	privateKey   *rsa.PrivateKey
	jwtCache     jwtCache
}

func New() mcp.Integration {
	return &snowflake{}
}

func (s *snowflake) Name() string { return "snowflake" }

func (s *snowflake) Configure(_ context.Context, creds mcp.Credentials) error {
	account := creds["account"]
	if account == "" {
		return fmt.Errorf("snowflake: account is required")
	}

	s.account = account
	s.warehouse = creds["warehouse"]
	s.database = creds["database"]
	s.schema = creds["schema"]
	s.role = creds["role"]
	s.semanticView = creds["semantic_view"]

	// Key-pair auth takes precedence over a static token.
	if pk := creds["private_key"]; pk != "" {
		user := creds["user"]
		if user == "" {
			return fmt.Errorf("snowflake: user is required when using private_key authentication")
		}
		key, err := parsePrivateKey(pk)
		if err != nil {
			return fmt.Errorf("snowflake: invalid private_key: %w", err)
		}
		s.privateKey = key
		s.user = strings.ToUpper(user)
	} else {
		token := creds["token"]
		if token == "" {
			return fmt.Errorf("snowflake: token or private_key is required")
		}
		s.token = token
	}

	if url := creds["account_url"]; url != "" {
		s.baseURL = strings.TrimRight(url, "/")
	} else {
		s.baseURL = fmt.Sprintf("https://%s.snowflakecomputing.com", account)
	}

	s.client = &http.Client{Timeout: 120 * time.Second}
	return nil
}

func (s *snowflake) Healthy(ctx context.Context) bool {
	if s.client == nil || (s.token == "" && s.privateKey == nil) {
		return false
	}
	_, err := s.submitStatement(ctx, "SELECT 1", nil)
	return err == nil
}

// getToken returns the bearer token for the current request. For key-pair auth
// this generates (or returns a cached) JWT; for static token auth it returns
// the configured token directly.
func (s *snowflake) getToken() (string, error) {
	if s.privateKey != nil {
		return s.jwtCache.getOrGenerate(s.privateKey, s.account, s.user)
	}
	return s.token, nil
}

func (s *snowflake) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *snowflake) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *snowflake) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	if s.client == nil {
		return &mcp.ToolResult{Data: "snowflake: not configured", IsError: true}, nil
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *snowflake) PlainTextKeys() []string {
	return []string{"account", "user", "warehouse", "database", "schema", "role", "semantic_view", "account_url"}
}

func (s *snowflake) Placeholders() map[string]string {
	return map[string]string{
		"account":       "abc1234-xy56789",
		"token":         "JWT or OAuth token (if not using key-pair auth)",
		"user":          "Username (required with private_key)",
		"private_key":   "PEM-encoded RSA private key (alternative to token)",
		"warehouse":     "COMPUTE_WH",
		"database":      "MY_DB",
		"schema":        "PUBLIC",
		"role":          "SYSADMIN",
		"semantic_view": "MY_DB.MY_SCHEMA.MY_SEMANTIC_VIEW",
		"account_url":   "https://abc1234-xy56789.snowflakecomputing.com",
	}
}

func (s *snowflake) OptionalKeys() []string {
	return []string{"token", "user", "private_key", "warehouse", "database", "schema", "role", "semantic_view", "account_url"}
}

type handlerFunc func(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error)

// --- HTTP helpers ---

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

type statementRequest struct {
	Statement  string            `json:"statement"`
	Timeout    int               `json:"timeout,omitempty"`
	Database   string            `json:"database,omitempty"`
	Schema     string            `json:"schema,omitempty"`
	Warehouse  string            `json:"warehouse,omitempty"`
	Role       string            `json:"role,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

type statementResponse struct {
	Code               string          `json:"code"`
	Message            string          `json:"message"`
	StatementHandle    string          `json:"statementHandle"`
	StatementStatusURL string          `json:"statementStatusUrl"`
	SQLState           string          `json:"sqlState"`
	CreatedOn          int64           `json:"createdOn"`
	ResultSetMetaData  json.RawMessage `json:"resultSetMetaData"`
	Data               json.RawMessage `json:"data"`
}

type resultSetMetaData struct {
	NumRows       int       `json:"numRows"`
	Format        string    `json:"format"`
	RowType       []rowType `json:"rowType"`
	PartitionInfo []struct {
		RowCount int `json:"rowCount"`
	} `json:"partitionInfo"`
}

type rowType struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

func (s *snowflake) submitStatement(ctx context.Context, sql string, opts *statementRequest) (*statementResponse, error) {
	req := &statementRequest{Statement: sql}
	if opts != nil {
		req.Timeout = opts.Timeout
		req.Database = opts.Database
		req.Schema = opts.Schema
		req.Warehouse = opts.Warehouse
		req.Role = opts.Role
		req.Parameters = opts.Parameters
	}

	if req.Database == "" {
		req.Database = s.database
	}
	if req.Schema == "" {
		req.Schema = s.schema
	}
	if req.Warehouse == "" {
		req.Warehouse = s.warehouse
	}
	if req.Role == "" {
		req.Role = s.role
	}

	return s.doStatementRequest(ctx, http.MethodPost, "/api/v2/statements", req)
}

func (s *snowflake) getStatementStatus(ctx context.Context, handle string, partition int) (*statementResponse, error) {
	url := fmt.Sprintf("/api/v2/statements/%s", handle)
	if partition > 0 {
		url += fmt.Sprintf("?partition=%d", partition)
	}
	return s.doStatementRequest(ctx, http.MethodGet, url, nil)
}

func (s *snowflake) cancelStatement(ctx context.Context, handle string) error {
	url := fmt.Sprintf("/api/v2/statements/%s/cancel", handle)
	_, err := s.doStatementRequest(ctx, http.MethodPost, url, nil)
	return err
}

// doJSON performs a JSON API request with common Snowflake auth headers and shared
// error handling (401, 429, 5xx). On 200 OK the response is unmarshalled into T.
// The optional extraStatus callback handles endpoint-specific status codes (e.g.
// 202, 422 for the SQL Statements API); return (nil, nil) to fall through to the
// shared error handler.
func doJSON[T any](ctx context.Context, s *snowflake, method, path string, body any, extraStatus func(int, []byte) (*T, error)) (*T, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("snowflake: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("snowflake: create request: %w", err)
	}

	token, err := s.getToken()
	if err != nil {
		return nil, fmt.Errorf("snowflake: get auth token: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Switchboard/1.0")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("snowflake: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("snowflake: read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result T
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("snowflake: parse response: %w", err)
		}
		return &result, nil
	}

	if extraStatus != nil {
		if result, err := extraStatus(resp.StatusCode, respBody); result != nil || err != nil {
			return result, err
		}
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("snowflake: unauthorized (401) — check token")
	case resp.StatusCode == http.StatusTooManyRequests:
		retryAfter := mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("snowflake: rate limited (429)"),
			RetryAfter: retryAfter,
		}
	case resp.StatusCode >= 500:
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("snowflake: server error (%d): %s", resp.StatusCode, string(respBody)),
		}
	default:
		return nil, fmt.Errorf("snowflake: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
}

func (s *snowflake) doStatementRequest(ctx context.Context, method, path string, body any) (*statementResponse, error) {
	return doJSON[statementResponse](ctx, s, method, path, body, func(status int, respBody []byte) (*statementResponse, error) {
		switch status {
		case http.StatusAccepted:
			var result statementResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return nil, fmt.Errorf("snowflake: parse 202 response: %w", err)
			}
			return &result, nil
		case http.StatusUnprocessableEntity:
			var result statementResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return nil, fmt.Errorf("snowflake: execution error (422): %s", string(respBody))
			}
			return nil, fmt.Errorf("snowflake: execution error (422): %s", result.Message)
		}
		return nil, nil
	})
}

// formatResults converts the Snowflake columnar response (array of arrays + metadata)
// into a slice of maps for consistent JSON output.
func formatResults(resp *statementResponse) (json.RawMessage, error) {
	if resp.Data == nil || resp.ResultSetMetaData == nil {
		return json.RawMessage("[]"), nil
	}

	var meta resultSetMetaData
	if err := json.Unmarshal(resp.ResultSetMetaData, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	var rows [][]any
	if err := json.Unmarshal(resp.Data, &rows); err != nil {
		return nil, fmt.Errorf("parse data: %w", err)
	}

	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]any, len(meta.RowType))
		for i, col := range meta.RowType {
			if i < len(row) {
				m[col.Name] = row[i]
			}
		}
		results = append(results, m)
	}

	if len(meta.PartitionInfo) > 1 {
		envelope := map[string]any{
			"rows":               results,
			"partitions_total":   len(meta.PartitionInfo),
			"partitions_fetched": 1,
			"statement_handle":   resp.StatementHandle,
		}
		data, err := json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("marshal results: %w", err)
		}
		return json.RawMessage(data), nil
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("marshal results: %w", err)
	}
	return json.RawMessage(data), nil
}
