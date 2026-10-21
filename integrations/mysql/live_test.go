package mysql

import (
	"os"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestMySQL84Live(t *testing.T) {
	dsn := os.Getenv("SWITCHBOARD_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set SWITCHBOARD_MYSQL_TEST_DSN for an isolated MySQL 8.4 fixture")
	}
	integration := New().(*mysql)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"connection_string": dsn}))
	t.Cleanup(func() { require.NoError(t, integration.Close()) })
	for _, tool := range integration.Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			args := map[string]any{"database": "app", "table": "users", "query": "SELECT id, name FROM users WHERE id = ?", "params": []any{1}}
			result, err := integration.Execute(t.Context(), tool.Name, args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.NotEqual(t, "{}", result.Data)
			t.Logf("%s: %s", tool.Name, result.Data)
		})
	}
}
