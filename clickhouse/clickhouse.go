package clickhouse

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	mcp "github.com/daltoniam/switchboard"
)

type clickhouseInt struct {
	conn driver.Conn
}

func New() mcp.Integration {
	return &clickhouseInt{}
}

func (c *clickhouseInt) Name() string { return "clickhouse" }

func (c *clickhouseInt) Configure(creds mcp.Credentials) error {
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
			InsecureSkipVerify: creds["skip_verify"] == "true",
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

func (c *clickhouseInt) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, c, args)
}

type handlerFunc func(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error)

func (c *clickhouseInt) query(ctx context.Context, query string) (json.RawMessage, error) {
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := rows.ColumnTypes()
	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = col.Name()
	}

	var results []map[string]any
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

func (c *clickhouseInt) exec(ctx context.Context, query string) error {
	return c.conn.Exec(ctx, query)
}

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

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

func escapeIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
