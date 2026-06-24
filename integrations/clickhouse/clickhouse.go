package clickhouse

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("clickhouse", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*clickhouseInt)(nil)
	_ mcp.FieldCompactionIntegration = (*clickhouseInt)(nil)
	_ mcp.PlainTextCredentials       = (*clickhouseInt)(nil)
	_ mcp.PlaceholderHints           = (*clickhouseInt)(nil)
	_ mcp.OptionalCredentials        = (*clickhouseInt)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*clickhouseInt)(nil)
)

func (c *clickhouseInt) PlainTextKeys() []string {
	return []string{"host", "port", "username", "database", "secure", "skip_verify", "connections"}
}

func (c *clickhouseInt) Placeholders() map[string]string {
	return map[string]string{
		"connections": `[{"alias":"analytics","host":"abc.clickhouse.cloud","port":"9440","username":"default","password":"...","database":"default","secure":"true"}]`,
	}
}

func (c *clickhouseInt) OptionalKeys() []string {
	return []string{"connections"}
}

type chConn struct {
	conn       driver.Conn
	alias      string
	host       string
	port       string
	username   string
	database   string
	secure     bool
	skipVerify bool
}

type connConfig struct {
	Alias      string `json:"alias"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Database   string `json:"database"`
	Secure     string `json:"secure"`
	SkipVerify string `json:"skip_verify"`
}

type clickhouseInt struct {
	mu           sync.RWMutex
	conns        map[string]*chConn
	defaultAlias string
}

func New() mcp.Integration {
	return &clickhouseInt{
		conns: make(map[string]*chConn),
	}
}

func (c *clickhouseInt) Name() string { return "clickhouse" }

func (c *clickhouseInt) Configure(ctx context.Context, creds mcp.Credentials) error {
	type pendingConn struct {
		alias string
		creds mcp.Credentials
	}
	pending := []pendingConn{
		{alias: "default", creds: creds},
	}
	seenAliases := map[string]bool{"default": true}

	if raw := creds["connections"]; raw != "" {
		var extras []connConfig
		if err := json.Unmarshal([]byte(raw), &extras); err != nil {
			return fmt.Errorf("clickhouse: invalid connections JSON: %w", err)
		}
		for _, ec := range extras {
			if ec.Alias == "" {
				return fmt.Errorf("clickhouse: each additional connection must have an alias")
			}
			if ec.Alias == "default" {
				return fmt.Errorf("clickhouse: alias \"default\" is reserved for the primary connection")
			}
			if seenAliases[ec.Alias] {
				return fmt.Errorf("clickhouse: duplicate connection alias %q", ec.Alias)
			}
			seenAliases[ec.Alias] = true
			pending = append(pending, pendingConn{
				alias: ec.Alias,
				creds: mcp.Credentials{
					"host":        ec.Host,
					"port":        ec.Port,
					"username":    ec.Username,
					"password":    ec.Password,
					"database":    ec.Database,
					"secure":      ec.Secure,
					"skip_verify": ec.SkipVerify,
				},
			})
		}
	}

	newConns := make(map[string]*chConn, len(pending))
	for _, pc := range pending {
		conn, err := openConn(ctx, pc.alias, pc.creds)
		if err != nil {
			for _, prev := range newConns {
				_ = prev.conn.Close()
			}
			if pc.alias != "default" {
				return fmt.Errorf("clickhouse: connection %q: %w", pc.alias, err)
			}
			return err
		}
		newConns[pc.alias] = conn
	}

	c.mu.Lock()
	old := c.conns
	c.conns = newConns
	c.defaultAlias = "default"
	c.mu.Unlock()

	for _, conn := range old {
		_ = conn.conn.Close()
	}

	return nil
}

func openConn(_ context.Context, alias string, creds mcp.Credentials) (*chConn, error) {
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
	skipVerify := creds["skip_verify"] == "true"

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
			InsecureSkipVerify: skipVerify, // #nosec G402 -- user-configured TLS setting
		}
	}

	conn, err := ch.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: failed to open connection: %w", err)
	}

	return &chConn{
		conn:       conn,
		alias:      alias,
		host:       host,
		port:       port,
		username:   username,
		database:   database,
		secure:     secure,
		skipVerify: skipVerify,
	}, nil
}

func (c *clickhouseInt) Healthy(ctx context.Context) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.conns) == 0 {
		return false
	}
	conn, ok := c.conns[c.defaultAlias]
	if !ok {
		return false
	}
	return conn.conn.Ping(ctx) == nil
}

func (c *clickhouseInt) Tools() []mcp.ToolDefinition {
	return tools
}

func (c *clickhouseInt) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (c *clickhouseInt) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (c *clickhouseInt) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	c.mu.RLock()
	hasConns := len(c.conns) > 0
	c.mu.RUnlock()
	if !hasConns {
		return &mcp.ToolResult{Data: "clickhouse: not configured (connection failed)", IsError: true}, nil
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, c, args)
}

func (c *clickhouseInt) getConnForArgs(args map[string]any) (*chConn, error) {
	alias, _ := mcp.ArgStr(args, "connection")

	c.mu.RLock()
	defer c.mu.RUnlock()

	if alias == "" {
		alias = c.defaultAlias
	}
	conn, ok := c.conns[alias]
	if !ok {
		available := make([]string, 0, len(c.conns))
		for k := range c.conns {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("unknown connection %q (available: %s)", alias, strings.Join(available, ", "))
	}
	return conn, nil
}

type handlerFunc func(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error)

func (c *clickhouseInt) query(ctx context.Context, conn *chConn, q string, args ...any) (json.RawMessage, error) {
	rows, err := conn.conn.Query(ctx, q, args...)
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

func (c *clickhouseInt) exec(ctx context.Context, conn *chConn, q string) error {
	return conn.conn.Exec(ctx, q)
}

func escapeIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
