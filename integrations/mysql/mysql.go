package mysql

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	driver "github.com/go-sql-driver/mysql"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("mysql", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs

var (
	_ mcp.Integration                = (*mysql)(nil)
	_ mcp.FieldCompactionIntegration = (*mysql)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*mysql)(nil)
	_ mcp.PlainTextCredentials       = (*mysql)(nil)
	_ mcp.PlaceholderHints           = (*mysql)(nil)
	_ mcp.OptionalCredentials        = (*mysql)(nil)
)

const (
	queryTimeout   = 10 * time.Second
	maxResultBytes = 1 << 20
	maxRows        = 1000
)

type mysql struct {
	mu    sync.RWMutex
	conns map[string]*sql.DB
	open  func(*driver.Config) (*sql.DB, error)
}

func New() mcp.Integration {
	return &mysql{conns: make(map[string]*sql.DB), open: openDatabase}
}

func (m *mysql) Name() string { return "mysql" }

func (m *mysql) PlainTextKeys() []string {
	return []string{"host", "port", "user", "database", "tls"}
}

func (m *mysql) OptionalKeys() []string {
	return []string{"connection_string", "host", "port", "user", "password", "database", "tls", "connections"}
}

func (m *mysql) Placeholders() map[string]string {
	return map[string]string{
		"host":              "127.0.0.1 or database hostname (required without DSN)",
		"port":              "3306",
		"user":              "Least-privilege SELECT / SHOW VIEW account (required without DSN)",
		"password":          "Database password",
		"database":          "Optional default database",
		"tls":               "true (default); false allowed only for literal loopback / localhost",
		"connection_string": "reader:password@tcp(db.example:3306)/app?tls=true",
		"connections":       `[{"alias":"analytics","host":"db.example","user":"reader","password":"...","database":"analytics","tls":"true"}]`,
	}
}

func buildConfig(creds mcp.Credentials) (*driver.Config, error) {
	if creds["mode"] != "" && creds["mode"] != "direct" || creds["read_only"] != "" && creds["read_only"] != "true" {
		return nil, fmt.Errorf("mysql: only direct TCP read-only access is supported")
	}
	config := driver.NewConfig()
	if dsn := creds["connection_string"]; dsn != "" {
		parsed, err := driver.ParseDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("mysql: invalid connection_string")
		}
		config = parsed
		if config.Net != "tcp" || config.MultiStatements || config.AllowAllFiles || config.AllowFallbackToPlaintext || config.AllowCleartextPasswords || config.AllowOldPasswords || config.InterpolateParams || len(config.Params) != 0 {
			return nil, fmt.Errorf("mysql: DSN must use TCP without unsafe driver options or session variables")
		}
	} else {
		host := strings.TrimSpace(creds["host"])
		if host == "" {
			return nil, fmt.Errorf("mysql: host is required without connection_string")
		}
		port := creds["port"]
		if port == "" {
			port = "3306"
		}
		config.Net = "tcp"
		config.Addr = net.JoinHostPort(host, port)
		config.User, config.Passwd, config.DBName = creds["user"], creds["password"], creds["database"]
		config.TLSConfig = creds["tls"]
	}
	if strings.TrimSpace(config.User) == "" {
		return nil, fmt.Errorf("mysql: user is required without connection_string")
	}
	host, port, err := net.SplitHostPort(config.Addr)
	if err != nil || strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("mysql: invalid TCP address")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("mysql: port must be between 1 and 65535")
	}
	tlsMode := config.TLSConfig
	if creds["connection_string"] != "" && creds["tls"] != "" && creds["tls"] != tlsMode {
		return nil, fmt.Errorf("mysql: tls conflicts with connection_string")
	}
	if tlsMode == "" {
		tlsMode = "true"
	}
	if tlsMode != "true" && tlsMode != "false" {
		return nil, fmt.Errorf("mysql: tls must be true (verified) or false (loopback only)")
	}
	address := net.ParseIP(host)
	if tlsMode == "false" && host != "localhost" && (address == nil || !address.IsLoopback()) {
		return nil, fmt.Errorf("mysql: verified TLS is required for remote TCP connections")
	}
	config.TLSConfig = tlsMode
	config.TLS = nil
	config.Timeout = 5 * time.Second
	config.ReadTimeout, config.WriteTimeout = queryTimeout, queryTimeout
	config.MaxAllowedPacket = maxResultBytes
	config.ParseTime = true
	return config, nil
}

func openDatabase(config *driver.Config) (*sql.DB, error) {
	connector, err := driver.NewConnector(config)
	if err != nil {
		return nil, fmt.Errorf("mysql: invalid connection configuration")
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(3 * time.Minute)
	return db, nil
}

func connectionConfigs(creds mcp.Credentials) (map[string]*driver.Config, error) {
	primary, err := buildConfig(creds)
	if err != nil {
		return nil, err
	}
	configs := map[string]*driver.Config{"default": primary}
	if raw := creds["connections"]; raw != "" {
		var extras []map[string]string
		if err := json.Unmarshal([]byte(raw), &extras); err != nil {
			return nil, fmt.Errorf("mysql: invalid connections JSON")
		}
		if len(extras) > 8 {
			return nil, fmt.Errorf("mysql: at most eight additional connections are supported")
		}
		for _, extra := range extras {
			alias := strings.TrimSpace(extra["alias"])
			if alias == "" || configs[alias] != nil {
				return nil, fmt.Errorf("mysql: additional connection aliases must be nonempty and unique; default is reserved")
			}
			config, err := buildConfig(mcp.Credentials(extra))
			if err != nil {
				return nil, fmt.Errorf("mysql: invalid additional connection: %w", err)
			}
			configs[alias] = config
		}
	}
	return configs, nil
}

func (m *mysql) Configure(ctx context.Context, creds mcp.Credentials) error {
	configs, err := connectionConfigs(creds)
	if err != nil {
		return err
	}
	opened := make(map[string]*sql.DB, len(configs))
	defer func() {
		for _, db := range opened {
			_ = db.Close()
		}
	}()
	for alias, config := range configs {
		db, err := m.open(config)
		if err != nil {
			return fmt.Errorf("mysql: cannot open database connection")
		}
		opened[alias] = db
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = verifyServer(pingCtx, db)
		cancel()
		if err != nil {
			return err
		}
	}
	m.mu.Lock()
	old := m.conns
	m.conns = opened
	opened = old
	m.mu.Unlock()
	return nil
}

func verifyServer(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("mysql: connection failed; check address, credentials and TLS")
	}
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return fmt.Errorf("mysql: cannot verify server version")
	}
	if !strings.HasPrefix(version, "8.4.") || strings.Contains(strings.ToLower(version), "tidb") || strings.Contains(strings.ToLower(version), "mariadb") {
		return fmt.Errorf("mysql: only MySQL 8.4 LTS is supported")
	}
	return nil
}

func (m *mysql) Healthy(ctx context.Context) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.conns) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, db := range m.conns {
		if err := db.PingContext(ctx); err != nil {
			return false
		}
	}
	return true
}

func (m *mysql) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var first error
	for _, db := range m.conns {
		if err := db.Close(); err != nil && first == nil {
			first = err
		}
	}
	m.conns = make(map[string]*sql.DB)
	return first
}

func (m *mysql) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (m *mysql) MaxBytes(name mcp.ToolName) (int, bool) {
	limit, ok := compactResult.MaxBytes[name]
	return limit, ok
}

func (m *mysql) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("mysql: unknown tool %s", name))
	}
	reader := mcp.NewArgs(args)
	alias := reader.Str("connection")
	limit := reader.OptInt("limit", 100)
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit < 1 || limit > maxRows {
		return mcp.ErrResult(fmt.Errorf("mysql: limit must be between 1 and 1000"))
	}
	if alias == "" {
		alias = "default"
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	db := m.conns[alias]
	if db == nil {
		return mcp.ErrResult(fmt.Errorf("mysql: connection is not configured"))
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return handler(ctx, db, args, limit)
}
