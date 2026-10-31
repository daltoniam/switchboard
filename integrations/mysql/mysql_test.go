package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mcp "github.com/daltoniam/switchboard"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	integration := New()
	require.Equal(t, "mysql", integration.Name())
	require.False(t, integration.Healthy(t.Context()))
	result, err := integration.Execute(t.Context(), "mysql_query", map[string]any{"query": "SELECT 1"})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestBuildConfig(t *testing.T) {
	cases := []struct {
		name  string
		creds mcp.Credentials
		valid bool
	}{
		{"local", mcp.Credentials{"host": "127.0.0.1", "user": "reader", "tls": "false"}, true},
		{"remote verified", mcp.Credentials{"host": "db.example.test", "user": "reader"}, true},
		{"dsn", mcp.Credentials{"connection_string": "reader:secret@tcp(db.example.test:3306)/app?tls=true&parseTime=true"}, true},
		{"missing user", mcp.Credentials{}, false},
		{"missing host", mcp.Credentials{"user": "reader"}, false},
		{"remote plaintext", mcp.Credentials{"host": "db.example.test", "user": "reader", "tls": "false"}, false},
		{"skip verification", mcp.Credentials{"host": "127.0.0.1", "user": "reader", "tls": "skip-verify"}, false},
		{"opportunistic", mcp.Credentials{"host": "db.example.test", "user": "reader", "tls": "preferred"}, false},
		{"invalid port", mcp.Credentials{"host": "127.0.0.1", "user": "reader", "port": "0"}, false},
		{"bad dsn", mcp.Credentials{"connection_string": "secret-invalid-dsn"}, false},
		{"multi statements", mcp.Credentials{"connection_string": "reader:secret@tcp(localhost:3306)/app?multiStatements=true"}, false},
		{"files", mcp.Credentials{"connection_string": "reader:secret@tcp(localhost:3306)/app?allowAllFiles=true"}, false},
		{"session variable", mcp.Credentials{"connection_string": "reader:secret@tcp(localhost:3306)/app?init_connect=secret"}, false},
		{"unix", mcp.Credentials{"connection_string": "reader:secret@unix(/tmp/mysql.sock)/app"}, false},
		{"agent", mcp.Credentials{"host": "localhost", "user": "reader", "mode": "agent"}, false},
		{"write mode", mcp.Credentials{"host": "localhost", "user": "reader", "read_only": "false"}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			config, err := buildConfig(test.creds)
			if !test.valid {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "secret")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "tcp", config.Net)
			require.False(t, config.MultiStatements)
			require.False(t, config.AllowAllFiles)
			require.False(t, config.AllowFallbackToPlaintext)
			require.LessOrEqual(t, config.Timeout, 5*time.Second)
			require.LessOrEqual(t, config.ReadTimeout, queryTimeout)
		})
	}
}

func mockIntegration(t *testing.T) (*mysql, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	integration := New().(*mysql)
	integration.open = func(*driver.Config) (*sql.DB, error) { return db, nil }
	t.Cleanup(func() { _ = db.Close() })
	return integration, mock
}

func expectConfigure(mock sqlmock.Sqlmock) {
	mock.ExpectPing()
	mock.ExpectQuery("SELECT VERSION").WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("8.4.8"))
}

func TestConfigure(t *testing.T) {
	integration, mock := mockIntegration(t)
	expectConfigure(mock)
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"host": "localhost", "user": "reader"}))
	mock.ExpectPing()
	require.True(t, integration.Healthy(t.Context()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigureRejectsUnsupportedServers(t *testing.T) {
	for _, version := range []string{"8.0.46", "9.0.0", "10.11.1-MariaDB", "5.7.0", "8.4.0-TiDB"} {
		t.Run(version, func(t *testing.T) {
			integration, mock := mockIntegration(t)
			mock.ExpectPing()
			mock.ExpectQuery("SELECT VERSION").WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(version))
			mock.ExpectClose()
			err := integration.Configure(t.Context(), mcp.Credentials{"host": "localhost", "user": "reader"})
			require.ErrorContains(t, err, "MySQL 8.4")
			require.False(t, integration.Healthy(t.Context()))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestConfigureAliasesValidateBeforeOpening(t *testing.T) {
	for _, connections := range []string{`not json`, `[{"alias":""}]`, `[{"alias":"default"}]`, `[{"alias":"a"},{"alias":"a"}]`, `[{"alias":"a","host":"remote","user":"reader","tls":"false"}]`} {
		t.Run(connections, func(t *testing.T) {
			integration := New().(*mysql)
			integration.open = func(*driver.Config) (*sql.DB, error) {
				t.Fatal("must validate all configs before opening")
				return nil, nil
			}
			require.Error(t, integration.Configure(t.Context(), mcp.Credentials{"host": "localhost", "user": "reader", "connections": connections}))
		})
	}
}

func expectRead(mock sqlmock.Sqlmock, query string, rows *sqlmock.Rows) {
	mock.ExpectExec("SET SESSION max_execution_time = 10000").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET SESSION sql_select_limit = 1001").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery(query).WillReturnRows(rows)
	mock.ExpectRollback()
}

func TestReadBounds(t *testing.T) {
	cases := []struct {
		name      string
		rows      *sqlmock.Rows
		limit     int
		wantRows  int
		truncated bool
	}{
		{"values", sqlmock.NewRows([]string{"id", "name", "nullable"}).AddRow(1, []byte("hello"), nil), 100, 1, false},
		{"empty", sqlmock.NewRows([]string{"id"}), 100, 0, false},
		{"row cap", sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2), 1, 1, true},
		{"byte cap", sqlmock.NewRows([]string{"text"}).AddRow(strings.Repeat("x", maxResultBytes+1)), 100, 0, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			integration, mock := mockIntegration(t)
			db, err := integration.open(nil)
			require.NoError(t, err)
			expectRead(mock, "SELECT", test.rows)
			result, err := readRows(t.Context(), db, "SELECT * FROM users", nil, test.limit)
			require.NoError(t, err)
			require.Len(t, result.Rows, test.wantRows)
			require.Equal(t, test.truncated, result.Truncated)
			encoded, err := json.Marshal(result)
			require.NoError(t, err)
			require.LessOrEqual(t, len(encoded), maxResultBytes)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestReadFailures(t *testing.T) {
	cases := []string{"session", "limit", "begin", "query", "scan"}
	for _, stage := range cases {
		t.Run(stage, func(t *testing.T) {
			integration, mock := mockIntegration(t)
			db, err := integration.open(nil)
			require.NoError(t, err)
			failure := errors.New("fixture failure")
			for _, setting := range []struct{ name, query string }{{"session", "SET SESSION max_execution_time"}, {"limit", "SET SESSION sql_select_limit"}} {
				expectation := mock.ExpectExec(setting.query)
				if stage == setting.name {
					expectation.WillReturnError(failure)
					break
				}
				expectation.WillReturnResult(sqlmock.NewResult(0, 0))
			}
			if stage == "begin" {
				mock.ExpectBegin().WillReturnError(failure)
			}
			if stage == "query" || stage == "scan" {
				mock.ExpectBegin()
				query := mock.ExpectQuery("SELECT")
				if stage == "query" {
					query.WillReturnError(failure)
				} else {
					query.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).RowError(0, failure))
				}
				mock.ExpectRollback()
			}
			_, err = readRows(t.Context(), db, "SELECT 1", nil, 100)
			require.Error(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestExecuteValidation(t *testing.T) {
	integration := New().(*mysql)
	cases := []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"mysql_missing", nil},
		{"mysql_query", map[string]any{"query": "DELETE FROM users"}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "limit": 1001}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "limit": "bad"}},
		{"mysql_query", map[string]any{"query": "SELECT 1", "connection": "missing"}},
		{"mysql_describe_table", map[string]any{"database": "app", "table": "x`; DROP TABLE users"}},
		{"mysql_explain", map[string]any{"query": "DELETE FROM users"}},
	}
	for _, test := range cases {
		t.Run(string(test.tool), func(t *testing.T) {
			result, err := integration.Execute(context.Background(), test.tool, test.args)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	seen := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		require.NotEmpty(t, tool.Description)
		require.True(t, strings.HasPrefix(string(tool.Name), "mysql_"))
		require.False(t, seen[tool.Name])
		seen[tool.Name] = true
		require.Contains(t, dispatch, tool.Name)
	}
	require.Len(t, seen, 12)
	require.Contains(t, New().Tools()[0].Description, "Start here")
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	tools := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		tools[tool.Name] = true
	}
	for name := range dispatch {
		require.Contains(t, tools, name)
	}
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
	for name := range fieldCompactionSpecs {
		require.Contains(t, dispatch, name)
	}
}

func TestFieldCompactionShape(t *testing.T) {
	for _, tool := range New().Tools() {
		t.Run(string(tool.Name), func(t *testing.T) {
			fields, ok := New().(mcp.FieldCompactionIntegration).CompactSpec(tool.Name)
			require.True(t, ok)
			value := map[string]any{"columns": []string{"id"}, "rows": [][]any{{1}}, "truncated": true, "noise": "remove"}
			if tool.Name == "mysql_describe_table" {
				value = map[string]any{"columns": value, "indexes": value, "foreign_keys": value, "ddl": value, "noise": "remove"}
			}
			result := mcp.CompactAny(value, fields)
			require.NotEmpty(t, result)
			encoded, err := json.Marshal(result)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "noise")
			require.Contains(t, string(encoded), "truncated")
		})
	}
}
