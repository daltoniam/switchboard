package snowflake

import (
	"bytes"
	"context"
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
	client    *http.Client
	accountID string
	token     string
	baseURL   string
	warehouse string
	database  string
	schema    string
	role      string
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
	token := creds["token"]
	if token == "" {
		return fmt.Errorf("snowflake: token (JWT or OAuth) is required")
	}

	s.accountID = account
	s.token = token
	s.warehouse = creds["warehouse"]
	s.database = creds["database"]
	s.schema = creds["schema"]
	s.role = creds["role"]

	if url := creds["account_url"]; url != "" {
		s.baseURL = strings.TrimRight(url, "/")
	} else {
		s.baseURL = fmt.Sprintf("https://%s.snowflakecomputing.com", account)
	}

	s.client = &http.Client{Timeout: 120 * time.Second}
	return nil
}

func (s *snowflake) Healthy(ctx context.Context) bool {
	if s.client == nil || s.token == "" {
		return false
	}
	_, err := s.submitStatement(ctx, "SELECT 1", nil)
	return err == nil
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
	return []string{"account", "warehouse", "database", "schema", "role", "account_url"}
}

func (s *snowflake) Placeholders() map[string]string {
	return map[string]string{
		"account":     "xy12345.us-east-1",
		"token":       "JWT or OAuth token",
		"warehouse":   "COMPUTE_WH",
		"database":    "MY_DB",
		"schema":      "PUBLIC",
		"role":        "SYSADMIN",
		"account_url": "https://xy12345.us-east-1.snowflakecomputing.com",
	}
}

func (s *snowflake) OptionalKeys() []string {
	return []string{"warehouse", "database", "schema", "role", "account_url"}
}

type handlerFunc func(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error)

// --- HTTP helpers ---

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

func (s *snowflake) doStatementRequest(ctx context.Context, method, path string, body any) (*statementResponse, error) {
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

	httpReq.Header.Set("Authorization", "Bearer "+s.token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Switchboard/1.0")
	httpReq.Header.Set("X-Snowflake-Authorization-Token-Type", "KEYPAIR_JWT")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("snowflake: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("snowflake: read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var result statementResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("snowflake: parse response: %w", err)
		}
		return &result, nil
	case http.StatusAccepted:
		var result statementResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("snowflake: parse 202 response: %w", err)
		}
		return &result, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("snowflake: unauthorized (401) — check token")
	case http.StatusTooManyRequests:
		retryAfter := mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("snowflake: rate limited (429)"),
			RetryAfter: retryAfter,
		}
	case http.StatusUnprocessableEntity:
		var result statementResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("snowflake: execution error (422): %s", string(respBody))
		}
		return nil, fmt.Errorf("snowflake: execution error (422): %s", result.Message)
	default:
		if resp.StatusCode >= 500 {
			return nil, &mcp.RetryableError{
				StatusCode: resp.StatusCode,
				Err:        fmt.Errorf("snowflake: server error (%d): %s", resp.StatusCode, string(respBody)),
			}
		}
		return nil, fmt.Errorf("snowflake: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
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

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("marshal results: %w", err)
	}
	return json.RawMessage(data), nil
}
