package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	mcp "github.com/daltoniam/switchboard"
)

type postgres struct {
	connStr  string
	db       *sql.DB
	readOnly bool
}

func New() mcp.Integration {
	return &postgres{}
}

func (p *postgres) Name() string { return "postgres" }

func (p *postgres) Configure(creds mcp.Credentials) error {
	p.readOnly = creds["read_only"] != "false"

	connStr := creds["connection_string"]
	if connStr == "" {
		host := creds["host"]
		port := creds["port"]
		user := creds["user"]
		password := creds["password"]
		dbname := creds["database"]
		sslmode := creds["sslmode"]

		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = "5432"
		}
		if sslmode == "" {
			sslmode = "prefer"
		}
		if user == "" {
			return fmt.Errorf("postgres: user is required (set connection_string or user credential)")
		}

		u := &url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(user, password),
			Host:     host + ":" + port,
			Path:     dbname,
			RawQuery: "sslmode=" + url.QueryEscape(sslmode),
		}
		connStr = u.String()
	}

	p.connStr = connStr

	db, err := sql.Open("postgres", p.connStr)
	if err != nil {
		return fmt.Errorf("postgres: failed to open connection: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("postgres: failed to ping database: %w", err)
	}

	p.db = db
	return nil
}

func (p *postgres) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *postgres) Healthy(ctx context.Context) bool {
	if p.db == nil {
		return false
	}
	return p.db.PingContext(ctx) == nil
}

func (p *postgres) Tools() []mcp.ToolDefinition {
	return tools
}

func (p *postgres) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, p, args)
}

// --- Query helpers ---

func (p *postgres) query(ctx context.Context, q string, args ...any) (json.RawMessage, error) {
	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanRows(rows)
}

func (p *postgres) exec(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	result, err := p.db.ExecContext(ctx, query, args...)
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

func (p *postgres) queryRow(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns error: %w", err)
	}

	if !rows.Next() {
		return json.RawMessage(`{}`), nil
	}

	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := rows.Scan(pointers...); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	row := make(map[string]any, len(columns))
	for i, col := range columns {
		val := values[i]
		switch v := val.(type) {
		case []byte:
			row[col] = string(v)
		default:
			row[col] = v
		}
	}

	data, err := json.Marshal(row)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}
	return json.RawMessage(data), nil
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

// sanitizeIdentifier validates and quotes a SQL identifier to prevent injection.
func sanitizeIdentifier(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("identifier cannot be empty")
	}
	if strings.ContainsAny(name, ";\x00") {
		return "", fmt.Errorf("invalid identifier: %s", name)
	}
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`, nil
}
