package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// runner abstracts how the adapter talks to a database. The direct
// implementation (sqlRunner) opens a *sql.DB and runs statements locally.
// The agent implementation (tunnelRunner) forwards read-only queries to a
// hosted tunneld over HTTP, which relays them to a customer-side agent that
// holds the real DSN. Both return the same JSON shapes the tools already
// expect, so every tool works identically regardless of mode.
type runner interface {
	// queryRows runs a read query and returns a JSON array of row objects,
	// matching scanRows output.
	queryRows(ctx context.Context, query string, args ...any) (json.RawMessage, error)
	// queryReadOnly runs a query inside a read-only transaction. Direct mode
	// wraps it in BEGIN ... READ ONLY; agent mode relies on tunneld enforcing
	// read-only server-side.
	queryReadOnly(ctx context.Context, query string, args ...any) (json.RawMessage, error)
	// queryRow runs a query and returns the first row as a JSON object (or {}).
	queryRow(ctx context.Context, query string, args ...any) (json.RawMessage, error)
	// exec runs a write statement. Agent mode is read-only and returns an error.
	exec(ctx context.Context, query string, args ...any) (json.RawMessage, error)
	// ping checks connectivity.
	ping(ctx context.Context) error
	// close releases resources.
	close() error
}

// --- direct (*sql.DB) runner ---

type sqlRunner struct {
	db *sql.DB
}

func (r *sqlRunner) queryRows(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRows(rows)
}

func (r *sqlRunner) queryReadOnly(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	tx, err := r.db.BeginTx(ctx, &readOnlyTx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRows(rows)
}

func (r *sqlRunner) queryRow(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanFirstRow(rows)
}

func (r *sqlRunner) exec(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec error: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	data, _ := json.Marshal(map[string]any{
		"status":        "success",
		"rows_affected": rowsAffected,
	})
	return json.RawMessage(data), nil
}

func (r *sqlRunner) ping(ctx context.Context) error { return r.db.PingContext(ctx) }
func (r *sqlRunner) close() error                   { return r.db.Close() }

// --- agent (tunneld HTTP) runner ---

const (
	tunnelDefaultMaxRows   = 1000
	tunnelDefaultTimeoutMs = 30000
)

// tunnelRunner forwards queries to a hosted tunneld /internal/query endpoint.
// tunneld relays to the org's agent, which executes against the real database
// in read-only mode. Writes are not possible over this path.
type tunnelRunner struct {
	client    *http.Client
	endpoint  string // fully-qualified tunneld /internal/query URL
	orgID     string
	authToken string
	maxRows   int
	timeoutMs int
}

func newTunnelRunner(tunnelURL, orgID, authToken string) (*tunnelRunner, error) {
	tunnelURL = strings.TrimSpace(tunnelURL)
	if tunnelURL == "" {
		return nil, fmt.Errorf("postgres: tunnel_url is required for mode=agent")
	}
	if strings.TrimSpace(orgID) == "" {
		return nil, fmt.Errorf("postgres: org is required for mode=agent")
	}
	endpoint := strings.TrimRight(tunnelURL, "/")
	if !strings.HasSuffix(endpoint, "/internal/query") {
		endpoint += "/internal/query"
	}
	return &tunnelRunner{
		client:    &http.Client{Timeout: 45 * time.Second},
		endpoint:  endpoint,
		orgID:     strings.TrimSpace(orgID),
		authToken: strings.TrimSpace(authToken),
		maxRows:   tunnelDefaultMaxRows,
		timeoutMs: tunnelDefaultTimeoutMs,
	}, nil
}

// tunnelQueryRequest mirrors the JSON the tunneld /internal/query endpoint
// accepts. read_only is always enforced server-side, so it is not sent.
type tunnelQueryRequest struct {
	OrgID     string `json:"org_id"`
	SQL       string `json:"sql"`
	Args      []any  `json:"args,omitempty"`
	MaxRows   int    `json:"max_rows,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

// tunnelQueryResponse mirrors tunnel.QueryResult plus the error envelope the
// endpoint returns on failure.
type tunnelQueryResponse struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
	Error     string   `json:"error"`
}

func (r *tunnelRunner) do(ctx context.Context, query string, args []any) (*tunnelQueryResponse, error) {
	reqBody, err := json.Marshal(tunnelQueryRequest{
		OrgID:     r.orgID,
		SQL:       query,
		Args:      args,
		MaxRows:   r.maxRows,
		TimeoutMs: r.timeoutMs,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.authToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tunnel request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read tunnel response: %w", err)
	}

	var out tunnelQueryResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, fmt.Errorf("decode tunnel response (status %d): %w", resp.StatusCode, err)
		}
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != "" {
			return nil, fmt.Errorf("tunnel query failed: %s", out.Error)
		}
		return nil, fmt.Errorf("tunnel query failed: status %d", resp.StatusCode)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("tunnel query failed: %s", out.Error)
	}
	return &out, nil
}

// rowObjects converts the column/row-array tunnel response into the JSON array
// of row objects that scanRows produces, so tool output is identical to direct
// mode.
func (resp *tunnelQueryResponse) rowObjects() (json.RawMessage, error) {
	results := make([]map[string]any, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		obj := make(map[string]any, len(resp.Columns))
		for i, col := range resp.Columns {
			if i < len(row) {
				obj[col] = normalizeTunnelValue(row[i])
			} else {
				obj[col] = nil
			}
		}
		results = append(results, obj)
	}
	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}
	return json.RawMessage(data), nil
}

// normalizeTunnelValue mirrors scanRows' []byte -> string handling. JSON
// decoding already yields strings/float64/bool/nil, but base64-encoded byte
// payloads can arrive as strings; we leave them as-is to match the direct path.
func normalizeTunnelValue(v any) any { return v }

func (r *tunnelRunner) queryRows(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	resp, err := r.do(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return resp.rowObjects()
}

func (r *tunnelRunner) queryReadOnly(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	// tunneld enforces read-only server-side, so no transaction wrapping needed.
	return r.queryRows(ctx, query, args...)
}

func (r *tunnelRunner) queryRow(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	resp, err := r.do(ctx, query, args)
	if err != nil {
		return nil, err
	}
	if len(resp.Rows) == 0 {
		return json.RawMessage(`{}`), nil
	}
	obj := make(map[string]any, len(resp.Columns))
	for i, col := range resp.Columns {
		if i < len(resp.Rows[0]) {
			obj[col] = normalizeTunnelValue(resp.Rows[0][i])
		} else {
			obj[col] = nil
		}
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}
	return json.RawMessage(data), nil
}

func (r *tunnelRunner) exec(_ context.Context, _ string, _ ...any) (json.RawMessage, error) {
	return nil, fmt.Errorf("postgres: write statements are not supported over the agent tunnel (read-only)")
}

func (r *tunnelRunner) ping(ctx context.Context) error {
	_, err := r.do(ctx, "SELECT 1", nil)
	return err
}

func (r *tunnelRunner) close() error { return nil }
