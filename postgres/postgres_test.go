package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	assert.NoError(t, err)
	assert.Equal(t, "host=localhost port=5432 user=test dbname=testdb sslmode=disable", p.connStr)
	assert.NotNil(t, p.db)
	_ = p.db.Close()
}

func TestConfigure_IndividualCredentials(t *testing.T) {
	p := &postgres{}
	err := p.Configure(mcp.Credentials{
		"host":     "myhost",
		"port":     "5433",
		"user":     "myuser",
		"password": "mypass",
		"database": "mydb",
		"sslmode":  "require",
	})
	assert.NoError(t, err)
	assert.Contains(t, p.connStr, "host=myhost")
	assert.Contains(t, p.connStr, "port=5433")
	assert.Contains(t, p.connStr, "user=myuser")
	assert.Contains(t, p.connStr, "password=mypass")
	assert.Contains(t, p.connStr, "dbname=mydb")
	assert.Contains(t, p.connStr, "sslmode=require")
	assert.NotNil(t, p.db)
	_ = p.db.Close()
}

func TestConfigure_Defaults(t *testing.T) {
	p := &postgres{}
	err := p.Configure(mcp.Credentials{"user": "test"})
	assert.NoError(t, err)
	assert.Contains(t, p.connStr, "host=localhost")
	assert.Contains(t, p.connStr, "port=5432")
	assert.Contains(t, p.connStr, "sslmode=prefer")
	_ = p.db.Close()
}

func TestConfigure_MissingUser(t *testing.T) {
	p := &postgres{}
	err := p.Configure(mcp.Credentials{"host": "localhost"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
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
	p := &postgres{db: &sql.DB{}}
	result, err := executeTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
}

func TestExplainTool_RequiresSQL(t *testing.T) {
	p := &postgres{db: &sql.DB{}}
	result, err := explainTool(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "sql is required")
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
