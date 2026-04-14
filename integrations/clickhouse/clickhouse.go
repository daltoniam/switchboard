package clickhouse

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*clickhouseInt)(nil)
	_ mcp.FieldCompactionIntegration = (*clickhouseInt)(nil)
	_ mcp.PlainTextCredentials       = (*clickhouseInt)(nil)
)

func (c *clickhouseInt) PlainTextKeys() []string {
	return []string{"host", "port", "username", "database", "secure", "skip_verify"}
}

type clickhouseInt struct {
	conn driver.Conn
}

func New() mcp.Integration {
	return &clickhouseInt{}
}

func (c *clickhouseInt) Name() string { return "clickhouse" }

func (c *clickhouseInt) Configure(_ context.Context, creds mcp.Credentials) error {
	if c.conn != nil {
		_ = c.conn.Close()
	}

	host := creds["host"]
	if host == "" {
		host = "localhost"
	}
	port := creds["port"]
	if port == "" {
		port = "9000"
	}
	username := creds["username"]
	if username == "" {
		username = "default"
	}
	password := creds["password"]
	database := creds["database"]
	if database == "" {
		database = "default"
	}
	secure := creds["secure"] == "true"

	addr := fmt.Sprintf("%s:%s", host, port)

	opts := &ch.Options{
		Addr: []string{addr},
		Auth: ch.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
	}

	if secure {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: creds["skip_verify"] == "true", // #nosec G402 -- user-configured TLS setting
		}
	}

	conn, err := ch.Open(opts)
	if err != nil {
		return fmt.Errorf("clickhouse: failed to open connection: %w", err)
	}

	c.conn = conn
	return nil
}

func (c *clickhouseInt) Healthy(ctx context.Context) bool {
	if c.conn == nil {
		return false
	}
	return c.conn.Ping(ctx) == nil
}

func (c *clickhouseInt) Tools() []mcp.ToolDefinition {
	return tools
}

func (c *clickhouseInt) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (c *clickhouseInt) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	if c.conn == nil {
		return &mcp.ToolResult{Data: "clickhouse: not configured (connection failed)", IsError: true}, nil
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, c, args)
}

type handlerFunc func(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error)

func (c *clickhouseInt) query(ctx context.Context, q string, args ...any) (json.RawMessage, error) {
	rows, err := c.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	columns := rows.ColumnTypes()
	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = col.Name()
	}

	results := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(columns))
		for i, col := range columns {
			vals[i] = reflect.New(col.ScanType()).Interface()
		}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for i, name := range colNames {
			row[name] = reflect.ValueOf(vals[i]).Elem().Interface()
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *clickhouseInt) exec(ctx context.Context, q string) error {
	return c.conn.Exec(ctx, q)
}

func escapeIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
