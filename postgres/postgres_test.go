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
	p := &postgres{}
	err := p.Configure(mcp.Credentials{"connection_string": "host=localhost port=5432 user=test dbname=testdb sslmode=disable"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping")
	assert.True(t, p.readOnly)
}

func TestConfigure_IndividualCredentials(t *testing.T) {
	p := &postgres{}
	err := p.Configure(mcp.Credentials{
		"host":     "myhost",
		"port":     "5433",
		"user":     "myuser",
		"password": "my pass",
		"database": "mydb",
		"sslmode":  "require",
	})
	assert.Error(t, err)
	assert.Contains(t, p.connStr, "myhost")
	assert.Contains(t, p.connStr, "5433")
	assert.Contains(t, p.connStr, "myuser")
	assert.Contains(t, p.connStr, "my%20pass")
	assert.Contains(t, p.connStr, "mydb")
	assert.Contains(t, p.connStr, "sslmode=require")
}

func TestConfigure_Defaults(t *testing.T) {
	p := &postgres{}
	err := p.Configure(mcp.Credentials{"user": "test"})
	assert.Error(t, err)
	assert.Contains(t, p.connStr, "localhost")
	assert.Contains(t, p.connStr, "5432")
	assert.Contains(t, p.connStr, "sslmode=prefer")
}

func TestConfigure_MissingUser(t *testing.T) {
	p := &postgres{}
	err := p.Configure(mcp.Credentials{"host": "localhost"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
}

func TestConfigure_ReadOnlyDefault(t *testing.T) {
	p := &postgres{}
	_ = p.Configure(mcp.Credentials{"connection_string": "host=localhost"})
	assert.True(t, p.readOnly)
}

func TestConfigure_ReadOnlyExplicitFalse(t *testing.T) {
	p := &postgres{}
	_ = p.Configure(mcp.Credentials{"connection_string": "host=localhost", "read_only": "false"})
	assert.False(t, p.readOnly)
}

func TestClose_NilDB(t *testing.T) {
	p := &postgres{}
	assert.NoError(t, p.Close())
}

func TestHealthy_NilDB(t *testing.T) {
	p := &postgres{}
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
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	p := &postgres{connStr: "host=localhost", db: &sql.DB{}}
	result, err := p.Execute(context.Background(), "postgres_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
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
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
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

func TestQueryTool_RequiresSQL(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := queryTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
}

func TestExecuteTool_RequiresSQL(t *testing.T) {
	p := &postgres{db: &sql.DB{}, readOnly: false}
	result, err := executeTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
}

func TestExecuteTool_ReadOnlyBlocks(t *testing.T) {
	p := &postgres{db: &sql.DB{}, readOnly: true}
	result, err := executeTool(context.Background(), p, map[string]any{"sql": "INSERT INTO t VALUES (1)"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "execute is disabled")
}

func TestExecuteTool_DenyList(t *testing.T) {
	p := &postgres{db: &sql.DB{}, readOnly: false}
	for _, sql := range []string{"DROP DATABASE mydb", "drop database mydb", "TRUNCATE users", "truncate users"} {
		result, err := executeTool(context.Background(), p, map[string]any{"sql": sql})
		require.NoError(t, err)
		assert.True(t, result.IsError, "expected error for: %s", sql)
		assert.Contains(t, result.Data, "not allowed", "expected deny for: %s", sql)
	}
}

func TestExplainTool_RequiresSQL(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := explainTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
}

func TestExplainTool_InvalidFormat(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
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
	p := &postgres{db: &sql.DB{}}

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
	p := &postgres{db: &sql.DB{}}
	result, err := selectTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestDescribeTable_RequiresTable(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := describeTable(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListColumns_RequiresTable(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := listColumns(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListIndexes_RequiresTable(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := listIndexes(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListConstraints_RequiresTable(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := listConstraints(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestListForeignKeys_RequiresTable(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := listForeignKeys(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestTableStats_RequiresTable(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := tableStats(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "table is required")
}

func TestSelectTool_InvalidIdentifier(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
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
