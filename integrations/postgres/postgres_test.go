package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "postgres", i.Name())
}

func TestConfigure_ConnectionString(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{"connection_string": "host=localhost port=5432 user=test dbname=testdb sslmode=disable"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping")
	conn, ok := p.conns["default"]
	if ok {
		assert.True(t, conn.readOnly)
	}
}

func TestConfigure_IndividualCredentials(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{
		"host":     "myhost",
		"port":     "5433",
		"user":     "myuser",
		"password": "my pass",
		"database": "mydb",
		"sslmode":  "require",
	})
	assert.Error(t, err)
}

func TestConfigure_Defaults(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{"user": "test"})
	assert.Error(t, err)
}

func TestConfigure_MissingUser(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{"host": "localhost"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
}

func TestConfigure_ReadOnlyDefault(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	_ = p.Configure(context.Background(), mcp.Credentials{"connection_string": "host=localhost"})
}

func TestConfigure_ReadOnlyExplicitFalse(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	_ = p.Configure(context.Background(), mcp.Credentials{"connection_string": "host=localhost", "read_only": "false"})
}

func TestConfigure_InvalidConnectionsJSON(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{
		"connection_string": "host=localhost",
		"connections":       "not-json",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid connections JSON")
}

func TestConfigure_ConnectionsMissingAlias(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{
		"connection_string": "host=localhost",
		"connections":       `[{"connection_string":"host=localhost2"}]`,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must have an alias")
}

func TestConfigure_ConnectionsReservedAlias(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{
		"connection_string": "host=localhost",
		"connections":       `[{"alias":"default","connection_string":"host=localhost2"}]`,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestConfigure_ConnectionsDuplicateAlias(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{
		"connection_string": "host=localhost",
		"connections":       `[{"alias":"prod","connection_string":"host=a"},{"alias":"prod","connection_string":"host=b"}]`,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestClose_EmptyConns(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	assert.NoError(t, p.Close())
}

func TestHealthy_EmptyConns(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	assert.False(t, p.Healthy(context.Background()))
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHavePostgresPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "postgres_", "tool %s missing postgres_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	p := &postgres{
		conns:        map[string]*pgConn{"default": {db: &sql.DB{}, readOnly: true, alias: "default"}},
		defaultAlias: "default",
	}
	result, err := p.Execute(context.Background(), "postgres_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecute_NoConns(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	result, err := p.Execute(context.Background(), "postgres_query", map[string]any{"sql": "SELECT 1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- getConnForArgs tests ---

func TestGetConnForArgs_Default(t *testing.T) {
	p := &postgres{
		conns: map[string]*pgConn{
			"default": {db: &sql.DB{}, alias: "default"},
			"prod":    {db: &sql.DB{}, alias: "prod"},
		},
		defaultAlias: "default",
	}
	conn, err := p.getConnForArgs(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "default", conn.alias)
}

func TestGetConnForArgs_ExplicitAlias(t *testing.T) {
	p := &postgres{
		conns: map[string]*pgConn{
			"default": {db: &sql.DB{}, alias: "default"},
			"prod":    {db: &sql.DB{}, alias: "prod"},
		},
		defaultAlias: "default",
	}
	conn, err := p.getConnForArgs(map[string]any{"database": "prod"})
	require.NoError(t, err)
	assert.Equal(t, "prod", conn.alias)
}

func TestGetConnForArgs_UnknownAlias(t *testing.T) {
	p := &postgres{
		conns: map[string]*pgConn{
			"default": {db: &sql.DB{}, alias: "default"},
		},
		defaultAlias: "default",
	}
	_, err := p.getConnForArgs(map[string]any{"database": "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown database")
	assert.Contains(t, err.Error(), "available")
}

// --- listDatabases test ---

func TestListDatabases(t *testing.T) {
	p := &postgres{
		conns: map[string]*pgConn{
			"default":   {db: &sql.DB{}, alias: "default", host: "localhost", dbName: "mydb", readOnly: true},
			"analytics": {db: &sql.DB{}, alias: "analytics", host: "analytics.example.com", dbName: "analytics", readOnly: false},
		},
		defaultAlias: "default",
	}

	result, err := listDatabases(context.Background(), p, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var dbs []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &dbs))
	assert.Len(t, dbs, 2)

	var defaultDB map[string]any
	for _, db := range dbs {
		if db["alias"] == "default" {
			defaultDB = db
		}
	}
	require.NotNil(t, defaultDB)
	assert.Equal(t, true, defaultDB["is_default"])
	assert.Equal(t, "localhost", defaultDB["host"])
}

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Sanitize identifier tests ---

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"users", `"users"`, false},
		{"my_table", `"my_table"`, false},
		{"public", `"public"`, false},
		{`table"name`, `"table""name"`, false},
		{"", "", true},
		{"bad;name", "", true},
		{"bad\x00name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := sanitizeIdentifier(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// --- Handler validation tests (no real DB) ---

func newTestPostgres() *postgres {
	return &postgres{
		conns: map[string]*pgConn{
			"default": {db: &sql.DB{}, readOnly: true, alias: "default"},
		},
		defaultAlias: "default",
	}
}

func newTestPostgresWritable() *postgres {
	return &postgres{
		conns: map[string]*pgConn{
			"default": {db: &sql.DB{}, readOnly: false, alias: "default"},
		},
		defaultAlias: "default",
	}
}

func TestQueryTool_RequiresSQL(t *testing.T) {
	p := newTestPostgres()
	result, err := queryTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
}

func TestExecuteTool_RequiresSQL(t *testing.T) {
	p := newTestPostgresWritable()
	result, err := executeTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
}

func TestExecuteTool_ReadOnlyBlocks(t *testing.T) {
	p := newTestPostgres()
	result, err := executeTool(context.Background(), p, map[string]any{"sql": "INSERT INTO t VALUES (1)"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "execute is disabled")
}

func TestExecuteTool_DenyList(t *testing.T) {
	p := newTestPostgresWritable()
	for _, sql := range []string{"DROP DATABASE mydb", "drop database mydb", "TRUNCATE users", "truncate users"} {
		result, err := executeTool(context.Background(), p, map[string]any{"sql": sql})
		require.NoError(t, err)
		assert.True(t, result.IsError, "expected error for: %s", sql)
		assert.Contains(t, result.Data, "not allowed", "expected deny for: %s", sql)
	}
}

func TestExplainTool_RequiresSQL(t *testing.T) {
	p := newTestPostgres()
	result, err := explainTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
}

func TestExplainTool_InvalidFormat(t *testing.T) {
	p := newTestPostgres()
	result, err := explainTool(context.Background(), p, map[string]any{"sql": "SELECT 1", "format": "evil"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid format")
}

func TestExplainTool_ValidFormats(t *testing.T) {
	for _, f := range []string{"text", "json", "yaml", "xml", "TEXT", "Json"} {
		assert.True(t, validExplainFormats[strings.ToLower(f)], "expected valid format: %s", f)
	}
}

func TestValidateSQLFragment(t *testing.T) {
	assert.NoError(t, validateSQLFragment("id = 1"))
	assert.NoError(t, validateSQLFragment("name ASC"))
	assert.Error(t, validateSQLFragment("1; DROP TABLE users"))
	assert.Error(t, validateSQLFragment("id -- comment"))
	assert.Error(t, validateSQLFragment("id /* block */"))
}

func TestSelectTool_RejectsMaliciousFragments(t *testing.T) {
	p := newTestPostgres()

	result, err := selectTool(context.Background(), p, map[string]any{"table": "users", "columns": "*; DROP TABLE users"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "semicolons")

	result, err = selectTool(context.Background(), p, map[string]any{"table": "users", "where": "1=1 -- always true"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "comments")

	result, err = selectTool(context.Background(), p, map[string]any{"table": "users", "order_by": "id /* evil */"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "comments")
}

func TestSelectTool_RequiresTable(t *testing.T) {
	p := newTestPostgres()
	result, err := selectTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestDescribeTable_RequiresTable(t *testing.T) {
	p := newTestPostgres()
	result, err := describeTable(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListColumns_RequiresTable(t *testing.T) {
	p := newTestPostgres()
	result, err := listColumns(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListIndexes_RequiresTable(t *testing.T) {
	p := newTestPostgres()
	result, err := listIndexes(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListConstraints_RequiresTable(t *testing.T) {
	p := newTestPostgres()
	result, err := listConstraints(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListForeignKeys_RequiresTable(t *testing.T) {
	p := newTestPostgres()
	result, err := listForeignKeys(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestTableStats_RequiresTable(t *testing.T) {
	p := newTestPostgres()
	result, err := tableStats(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestSelectTool_InvalidIdentifier(t *testing.T) {
	p := newTestPostgres()
	result, err := selectTool(context.Background(), p, map[string]any{"table": "bad;table"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid identifier")
}

func TestTools_RequiredFieldsAreValid(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		for _, req := range tool.Required {
			_, exists := tool.Parameters[req]
			assert.True(t, exists, "tool %s: required param %s not in parameters", tool.Name, req)
		}
	}
}

func TestTools_AllHaveDatabaseParam(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		if tool.Name == "postgres_list_databases" {
			continue
		}
		_, exists := tool.Parameters["database"]
		assert.True(t, exists, "tool %s missing database parameter", tool.Name)
	}
}

func TestPlainTextKeys(t *testing.T) {
	p := &postgres{}
	keys := p.PlainTextKeys()
	assert.Contains(t, keys, "connections")
	assert.Contains(t, keys, "host")
}

func TestOptionalKeys(t *testing.T) {
	p := &postgres{}
	keys := p.OptionalKeys()
	assert.Contains(t, keys, "connections")
}

func TestPlaceholders(t *testing.T) {
	p := &postgres{}
	ph := p.Placeholders()
	assert.Contains(t, ph, "connections")
}
