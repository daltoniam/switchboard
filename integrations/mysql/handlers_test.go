package mysql

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mcp "github.com/daltoniam/switchboard"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestHandlers(t *testing.T) {
	for _, tool := range New().Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			integration, mock := mockIntegration(t)
			db, err := integration.open(nil)
			require.NoError(t, err)
			integration.conns["default"] = db
			args := map[string]any{"database": "app", "table": "users", "query": "SELECT id FROM users", "limit": 2}
			if tool.Name == "mysql_describe_table" {
				for _, query := range []string{columnsSQL, indexesSQL, foreignKeysSQL, "SHOW CREATE TABLE `app`.`users`"} {
					expectRead(mock, regexp.QuoteMeta(query), sqlmock.NewRows([]string{"id"}).AddRow(1))
				}
			} else {
				expectRead(mock, ".*", sqlmock.NewRows([]string{"id"}).AddRow(1))
			}
			result, err := integration.Execute(t.Context(), tool.Name, args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, "truncated")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestConfiguredValidation(t *testing.T) {
	integration, mock := mockIntegration(t)
	db, err := integration.open(nil)
	require.NoError(t, err)
	integration.conns["default"] = db
	cases := []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"mysql_query", map[string]any{"query": "SELECT evil()"}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "limit": 0}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "limit": 1001}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "params": map[string]any{}}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "params": []any{map[string]any{}}}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "params": "bad-json"}},
		{"mysql_query", map[string]any{"query": 123}},
		{"mysql_explain", map[string]any{"query": "SHOW DATABASES"}},
		{"mysql_describe_table", map[string]any{"database": "app", "table": "x`; DROP TABLE users"}},
		{"mysql_list_tables", map[string]any{"database": ""}},
	}
	for _, test := range cases {
		t.Run(string(test.name), func(t *testing.T) {
			result, err := integration.Execute(t.Context(), test.name, test.args)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryParameters(t *testing.T) {
	for _, params := range []any{[]any{"value", nil, true, 1.5}, `["value",null,true,1.5]`} {
		t.Run("array", func(t *testing.T) {
			integration, mock := mockIntegration(t)
			db, err := integration.open(nil)
			require.NoError(t, err)
			integration.conns["default"] = db
			mock.ExpectExec("SET SESSION max_execution_time").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("SET SESSION sql_select_limit").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT ?, ?, ?, ?")).WithArgs("value", nil, true, 1.5).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectRollback()
			result, err := integration.Execute(t.Context(), "mysql_query", map[string]any{"query": "SELECT ?, ?, ?, ?", "params": params})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestConfigureFailurePreservesPrevious(t *testing.T) {
	integration, mock := mockIntegration(t)
	expectConfigure(mock)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"host": "localhost", "user": "reader"}))
	previous := integration.conns["default"]
	db, failing, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	integration.open = func(*driver.Config) (*sql.DB, error) { return db, nil }
	failing.ExpectPing().WillReturnError(errors.New("credential-secret"))
	failing.ExpectClose()
	err = integration.Configure(t.Context(), mcp.Credentials{"host": "localhost", "user": "reader"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "credential-secret")
	require.Same(t, previous, integration.conns["default"])
	require.NoError(t, failing.ExpectationsWereMet())
}

func TestConfigureAliases(t *testing.T) {
	integration := New().(*mysql)
	mocks := map[string]sqlmock.Sqlmock{}
	integration.open = func(config *driver.Config) (*sql.DB, error) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			return nil, err
		}
		expectConfigure(mock)
		mocks[config.DBName] = mock
		return db, nil
	}
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"host": "localhost", "user": "reader", "database": "primary", "connections": `[{"alias":"analytics","host":"localhost","user":"reader","database":"analytics"}]`}))
	expectRead(mocks["analytics"], "SELECT", sqlmock.NewRows([]string{"id"}).AddRow(2))
	result, err := integration.Execute(t.Context(), "mysql_query", map[string]any{"query": "SELECT 2", "connection": "analytics"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	for _, mock := range mocks {
		mock.ExpectClose()
	}
	require.NoError(t, integration.Close())
	for _, mock := range mocks {
		require.NoError(t, mock.ExpectationsWereMet())
	}
	require.False(t, integration.Healthy(t.Context()))
}

func TestReadCancellation(t *testing.T) {
	integration, mock := mockIntegration(t)
	db, err := integration.open(nil)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	mock.ExpectExec("SET SESSION max_execution_time").WillDelayFor(time.Second).WillReturnResult(sqlmock.NewResult(0, 0))
	start := time.Now()
	_, err = readRows(ctx, db, "SELECT 1", nil, 100)
	require.Error(t, err)
	require.Less(t, time.Since(start), time.Second)
	require.NoError(t, mock.ExpectationsWereMet())
}
