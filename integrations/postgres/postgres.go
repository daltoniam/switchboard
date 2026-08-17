package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("postgres", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*postgres)(nil)
	_ mcp.FieldCompactionIntegration = (*postgres)(nil)
	_ mcp.PlainTextCredentials       = (*postgres)(nil)
	_ mcp.PlaceholderHints           = (*postgres)(nil)
	_ mcp.OptionalCredentials        = (*postgres)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*postgres)(nil)
)

// pgConn holds a single Postgres connection and its metadata. Statement
// execution goes through runner, which is either a direct *sql.DB (sqlRunner)
// or a hosted tunnel relay (tunnelRunner) when mode=agent.
type pgConn struct {
	runner   runner
	connStr  string
	readOnly bool
	alias    string
	host     string
	dbName   string
}

// connConfig represents a named connection parsed from the "connections" JSON credential.
type connConfig struct {
	Alias            string `json:"alias"`
	ConnectionString string `json:"connection_string"`
	Host             string `json:"host"`
	Port             string `json:"port"`
	User             string `json:"user"`
	Password         string `json:"password"`
	Database         string `json:"database"`
	SSLMode          string `json:"sslmode"`
	ReadOnly         string `json:"read_only"`
}

type postgres struct {
	mu           sync.RWMutex
	conns        map[string]*pgConn
	defaultAlias string
}

func New() mcp.Integration {
	return &postgres{
		conns: make(map[string]*pgConn),
	}
}

func (p *postgres) Name() string { return "postgres" }

func (p *postgres) PlainTextKeys() []string {
	return []string{"host", "port", "user", "database", "sslmode", "read_only", "connections", "mode", "tunnel_url", "org"}
}

func (p *postgres) Placeholders() map[string]string {
	return map[string]string{
		"connections": `[{"alias":"analytics","connection_string":"postgres://..."}]`,
	}
}

func (p *postgres) OptionalKeys() []string {
	return []string{"connections", "mode", "tunnel_url", "org", "tunnel_auth_token"}
}

func (p *postgres) Configure(ctx context.Context, creds mcp.Credentials) error {
	// Phase 1: Parse and validate all connection configs before opening any.
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
			return fmt.Errorf("postgres: invalid connections JSON: %w", err)
		}
		for _, ec := range extras {
			if ec.Alias == "" {
				return fmt.Errorf("postgres: each additional connection must have an alias")
			}
			if ec.Alias == "default" {
				return fmt.Errorf("postgres: alias \"default\" is reserved for the primary connection")
			}
			if seenAliases[ec.Alias] {
				return fmt.Errorf("postgres: duplicate connection alias %q", ec.Alias)
			}
			seenAliases[ec.Alias] = true
			pending = append(pending, pendingConn{
				alias: ec.Alias,
				creds: mcp.Credentials{
					"connection_string": ec.ConnectionString,
					"host":              ec.Host,
					"port":              ec.Port,
					"user":              ec.User,
					"password":          ec.Password,
					"database":          ec.Database,
					"sslmode":           ec.SSLMode,
					"read_only":         ec.ReadOnly,
				},
			})
		}
	}

	// Phase 2: Open all connections.
	newConns := make(map[string]*pgConn, len(pending))
	for _, pc := range pending {
		c, err := openConn(ctx, pc.alias, pc.creds)
		if err != nil {
			for _, prev := range newConns {
				_ = prev.runner.close()
			}
			if pc.alias != "default" {
				return fmt.Errorf("postgres: connection %q: %w", pc.alias, err)
			}
			return err
		}
		newConns[pc.alias] = c
	}

	p.mu.Lock()
	old := p.conns
	p.conns = newConns
	p.defaultAlias = "default"
	p.mu.Unlock()

	for _, c := range old {
		_ = c.runner.close()
	}

	return nil
}

func openConn(ctx context.Context, alias string, creds mcp.Credentials) (*pgConn, error) {
	if strings.EqualFold(strings.TrimSpace(creds["mode"]), "agent") {
		return openAgentConn(ctx, alias, creds)
	}

	readOnly := creds["read_only"] != "false"
	connStr, host, dbName, err := buildConnStr(creds)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to open connection: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: failed to ping database: %w", err)
	}

	return &pgConn{
		runner:   &sqlRunner{db: db},
		connStr:  connStr,
		readOnly: readOnly,
		alias:    alias,
		host:     host,
		dbName:   dbName,
	}, nil
}

// openAgentConn builds a tunnel-backed connection. The real DSN lives in the
// customer-side agent; hosted only needs the tunnel endpoint and org ID. Agent
// mode is always read-only because tunneld enforces it server-side.
func openAgentConn(ctx context.Context, alias string, creds mcp.Credentials) (*pgConn, error) {
	r, err := newTunnelRunner(creds["tunnel_url"], creds["org"], creds["tunnel_auth_token"])
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := r.ping(pingCtx); err != nil {
		return nil, fmt.Errorf("postgres: failed to reach agent tunnel: %w", err)
	}

	return &pgConn{
		runner:   r,
		readOnly: true,
		alias:    alias,
		host:     "agent:" + strings.TrimSpace(creds["org"]),
		dbName:   creds["database"],
	}, nil
}

func buildConnStr(creds mcp.Credentials) (connStr, host, dbName string, err error) {
	connStr = creds["connection_string"]
	host = creds["host"]
	dbName = creds["database"]

	if connStr != "" {
		return extractHostFromConnStr(connStr, host, dbName)
	}

	port := creds["port"]
	user := creds["user"]
	password := creds["password"]
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
		return "", "", "", fmt.Errorf("postgres: user is required (set connection_string or user credential)")
	}

	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     host + ":" + port,
		Path:     dbName,
		RawQuery: "sslmode=" + url.QueryEscape(sslmode),
	}
	return u.String(), host, dbName, nil
}

func extractHostFromConnStr(connStr, host, dbName string) (string, string, string, error) {
	if host != "" {
		return connStr, host, dbName, nil
	}
	u, err := url.Parse(connStr)
	if err != nil {
		return connStr, host, dbName, nil
	}
	host = u.Hostname()
	if dbName == "" {
		dbName = strings.TrimPrefix(u.Path, "/")
	}
	return connStr, host, dbName, nil
}

func (p *postgres) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for _, c := range p.conns {
		if err := c.runner.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.conns = make(map[string]*pgConn)
	return firstErr
}

func (p *postgres) Healthy(ctx context.Context) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.conns) == 0 {
		return false
	}
	c, ok := p.conns[p.defaultAlias]
	if !ok {
		return false
	}
	return c.runner.ping(ctx) == nil
}

func (p *postgres) Tools() []mcp.ToolDefinition {
	return tools
}

func (p *postgres) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (p *postgres) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (p *postgres) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	p.mu.RLock()
	hasConns := len(p.conns) > 0
	p.mu.RUnlock()
	if !hasConns {
		return &mcp.ToolResult{Data: "postgres: not configured (connection failed)", IsError: true}, nil
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, p, args)
}

// getConnForArgs resolves the connection to use based on the optional "database" arg.
func (p *postgres) getConnForArgs(args map[string]any) (*pgConn, error) {
	alias, _ := mcp.ArgStr(args, "database")

	p.mu.RLock()
	defer p.mu.RUnlock()

	if alias == "" {
		alias = p.defaultAlias
	}
	c, ok := p.conns[alias]
	if !ok {
		available := make([]string, 0, len(p.conns))
		for k := range p.conns {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("unknown database %q (available: %s)", alias, strings.Join(available, ", "))
	}
	return c, nil
}

// --- Query helpers ---

func (p *postgres) query(ctx context.Context, conn *pgConn, q string, args ...any) (json.RawMessage, error) {
	return conn.runner.queryRows(ctx, q, args...)
}

func (p *postgres) exec(ctx context.Context, conn *pgConn, query string, args ...any) (json.RawMessage, error) {
	return conn.runner.exec(ctx, query, args...)
}

func (p *postgres) queryRow(ctx context.Context, conn *pgConn, query string, args ...any) (json.RawMessage, error) {
	return conn.runner.queryRow(ctx, query, args...)
}

// scanFirstRow returns the first row of a result set as a JSON object, or {} if
// there are no rows. It mirrors the previous direct queryRow behavior.
func scanFirstRow(rows *sql.Rows) (json.RawMessage, error) {
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

// --- Argument helpers (use shared mcp.Arg* / mcp.Args) ---

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
