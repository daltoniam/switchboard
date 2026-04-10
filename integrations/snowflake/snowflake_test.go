package snowflake

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPEM returns a PEM-encoded PKCS#1 RSA private key for testing.
func testPEM(t *testing.T) string {
	t.Helper()
	return pemEncodePKCS1(testKey(t))
}

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "snowflake", i.Name())
}

func TestConfigure_MissingAccount(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{"token": "tok"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account is required")
}

func TestConfigure_MissingAuth(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{"account": "acct"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token or private_key is required")
}

func TestConfigure_KeyPairAuth(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{
		"account":     "xy12345.us-east-1",
		"user":        "myuser",
		"private_key": testPEM(t),
	})
	require.NoError(t, err)
	assert.NotNil(t, s.privateKey)
	assert.Equal(t, "MYUSER", s.user)
	assert.Equal(t, "xy12345.us-east-1", s.account)
	assert.Empty(t, s.token, "token should be empty when using key-pair auth")
}

func TestConfigure_KeyPairAuth_MissingUser(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{
		"account":     "acct",
		"private_key": testPEM(t),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
}

func TestConfigure_KeyPairAuth_InvalidKey(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{
		"account":     "acct",
		"user":        "myuser",
		"private_key": "not-a-pem",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid private_key")
}

func TestConfigure_KeyPairPrecedence(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{
		"account":     "acct",
		"token":       "should-be-ignored",
		"user":        "myuser",
		"private_key": testPEM(t),
	})
	require.NoError(t, err)
	assert.NotNil(t, s.privateKey, "key-pair auth should take precedence")
	assert.Empty(t, s.token, "token should be empty when private_key is provided")
}

func TestConfigure_DefaultURL(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{
		"account": "xy12345.us-east-1",
		"token":   "test-token",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://xy12345.us-east-1.snowflakecomputing.com", s.baseURL)
}

func TestConfigure_CustomURL(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{
		"account":     "acct",
		"token":       "test-token",
		"account_url": "https://custom.snowflake.example.com/",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://custom.snowflake.example.com", s.baseURL)
}

func TestConfigure_OptionalFields(t *testing.T) {
	s := &snowflake{}
	err := s.Configure(context.Background(), mcp.Credentials{
		"account":   "acct",
		"token":     "tok",
		"warehouse": "WH",
		"database":  "DB",
		"schema":    "SCH",
		"role":      "ROLE",
	})
	require.NoError(t, err)
	assert.Equal(t, "WH", s.warehouse)
	assert.Equal(t, "DB", s.database)
	assert.Equal(t, "SCH", s.schema)
	assert.Equal(t, "ROLE", s.role)
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

func TestTools_AllHaveSnowflakePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "snowflake_", "tool %s missing snowflake_ prefix", tool.Name)
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

func TestExecute_NilClient(t *testing.T) {
	s := &snowflake{}
	result, err := s.Execute(context.Background(), "snowflake_execute_query", map[string]any{"query": "SELECT 1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not configured")
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &snowflake{client: &http.Client{}}
	result, err := s.Execute(context.Background(), "snowflake_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestHealthy_NilClient(t *testing.T) {
	s := &snowflake{}
	assert.False(t, s.Healthy(context.Background()))
}

func TestPlainTextKeys(t *testing.T) {
	s := &snowflake{}
	keys := s.PlainTextKeys()
	assert.Contains(t, keys, "account")
	assert.Contains(t, keys, "user")
	assert.Contains(t, keys, "warehouse")
	assert.Contains(t, keys, "semantic_view")
	assert.NotContains(t, keys, "token")
	assert.NotContains(t, keys, "private_key")
}

func TestOptionalKeys(t *testing.T) {
	s := &snowflake{}
	keys := s.OptionalKeys()
	assert.Contains(t, keys, "warehouse")
	assert.Contains(t, keys, "database")
	assert.Contains(t, keys, "token")
	assert.Contains(t, keys, "user")
	assert.Contains(t, keys, "private_key")
	assert.Contains(t, keys, "semantic_view")
	assert.NotContains(t, keys, "account")
}

func TestPlaceholders(t *testing.T) {
	s := &snowflake{}
	ph := s.Placeholders()
	assert.NotEmpty(t, ph["account"])
	assert.NotEmpty(t, ph["token"])
	assert.NotEmpty(t, ph["user"])
	assert.NotEmpty(t, ph["private_key"])
	assert.NotEmpty(t, ph["semantic_view"])
}

func TestQuoteIdentifier(t *testing.T) {
	assert.Equal(t, `"my_table"`, quoteIdentifier("my_table"))
	assert.Equal(t, `"my""table"`, quoteIdentifier(`my"table`))
}

func TestQualifyTable(t *testing.T) {
	tests := []struct {
		db, schema, table, want string
	}{
		{"", "", "t", `"t"`},
		{"db", "", "t", `"db"."t"`},
		{"db", "sch", "t", `"db"."sch"."t"`},
		{"", "sch", "t", `"sch"."t"`},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, qualifyTable(tt.db, tt.schema, tt.table))
		})
	}
}

func TestFormatResults(t *testing.T) {
	meta := resultSetMetaData{
		NumRows: 2,
		RowType: []rowType{
			{Name: "id", Type: "FIXED"},
			{Name: "name", Type: "TEXT"},
		},
	}
	metaJSON, _ := json.Marshal(meta)
	dataJSON, _ := json.Marshal([][]any{{"1", "alice"}, {"2", "bob"}})

	resp := &statementResponse{
		ResultSetMetaData: metaJSON,
		Data:              dataJSON,
	}

	result, err := formatResults(resp)
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal(result, &rows))
	assert.Len(t, rows, 2)
	assert.Equal(t, "alice", rows[0]["name"])
	assert.Equal(t, "bob", rows[1]["name"])
}

func TestFormatResults_NilData(t *testing.T) {
	resp := &statementResponse{}
	result, err := formatResults(resp)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(result))
}

func TestFormatResults_MultiPartition(t *testing.T) {
	meta := resultSetMetaData{
		NumRows: 100,
		RowType: []rowType{{Name: "id", Type: "FIXED"}},
		PartitionInfo: []struct {
			RowCount int `json:"rowCount"`
		}{{RowCount: 50}, {RowCount: 50}},
	}
	metaJSON, _ := json.Marshal(meta)
	dataJSON, _ := json.Marshal([][]any{{"1"}, {"2"}})

	resp := &statementResponse{
		StatementHandle:   "handle-123",
		ResultSetMetaData: metaJSON,
		Data:              dataJSON,
	}

	result, err := formatResults(resp)
	require.NoError(t, err)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(result, &envelope))
	assert.Equal(t, float64(2), envelope["partitions_total"])
	assert.Equal(t, float64(1), envelope["partitions_fetched"])
	assert.Equal(t, "handle-123", envelope["statement_handle"])
	assert.NotNil(t, envelope["rows"])
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *snowflake {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &snowflake{
		client:  ts.Client(),
		baseURL: ts.URL,
		token:   "test-token",
	}
}

func TestExecuteQuery_Success(t *testing.T) {
	meta := resultSetMetaData{
		NumRows: 1,
		RowType: []rowType{{Name: "result", Type: "FIXED"}},
	}
	metaJSON, _ := json.Marshal(meta)
	dataJSON, _ := json.Marshal([][]any{{"1"}})

	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "/api/v2/statements", r.URL.Path)

		resp := statementResponse{
			Code:              "090001",
			Message:           "ok",
			ResultSetMetaData: metaJSON,
			Data:              dataJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	result, err := executeQuery(context.Background(), s, map[string]any{"query": "SELECT 1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &rows))
	assert.Len(t, rows, 1)
}

func TestExecuteQuery_EmptyQuery(t *testing.T) {
	s := &snowflake{client: &http.Client{}}
	result, err := executeQuery(context.Background(), s, map[string]any{"query": ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "query is required")
}

func TestCancelQuery_Success(t *testing.T) {
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/cancel")
		resp := statementResponse{Code: "000000", Message: "cancelled"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	result, err := cancelQuery(context.Background(), s, map[string]any{
		"statement_handle": "test-handle-123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "cancelled")
}

func TestCancelQuery_EmptyHandle(t *testing.T) {
	s := &snowflake{client: &http.Client{}}
	result, err := cancelQuery(context.Background(), s, map[string]any{"statement_handle": ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "statement_handle is required")
}

func TestGetQueryStatus_Success(t *testing.T) {
	meta := resultSetMetaData{
		NumRows: 1,
		RowType: []rowType{{Name: "col1", Type: "TEXT"}},
	}
	metaJSON, _ := json.Marshal(meta)
	dataJSON, _ := json.Marshal([][]any{{"value1"}})

	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v2/statements/")
		resp := statementResponse{
			Code:              "090001",
			ResultSetMetaData: metaJSON,
			Data:              dataJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	result, err := getQueryStatus(context.Background(), s, map[string]any{
		"statement_handle": "handle-456",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDoStatementRequest_RateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "tok"}
	_, err := s.doStatementRequest(context.Background(), http.MethodPost, "/api/v2/statements", nil)
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestDoStatementRequest_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal"}`)) //nolint:errcheck
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "tok"}
	_, err := s.doStatementRequest(context.Background(), http.MethodPost, "/api/v2/statements", nil)
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestDoStatementRequest_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "tok"}
	_, err := s.doStatementRequest(context.Background(), http.MethodPost, "/api/v2/statements", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
	assert.False(t, mcp.IsRetryable(err))
}

func TestDoStatementRequest_422(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(statementResponse{Message: "SQL compilation error"}) //nolint:errcheck
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "tok"}
	_, err := s.doStatementRequest(context.Background(), http.MethodPost, "/api/v2/statements", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQL compilation error")
}

func TestListDatabases_Integration(t *testing.T) {
	meta := resultSetMetaData{
		NumRows: 1,
		RowType: []rowType{{Name: "name", Type: "TEXT"}},
	}
	metaJSON, _ := json.Marshal(meta)
	dataJSON, _ := json.Marshal([][]any{{"MY_DB"}})

	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req statementRequest
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		assert.Equal(t, "SHOW DATABASES", req.Statement)

		resp := statementResponse{
			Code:              "090001",
			ResultSetMetaData: metaJSON,
			Data:              dataJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	result, err := listDatabases(context.Background(), s, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "MY_DB")
}

func TestShowCreateTable_SQLConstruction(t *testing.T) {
	tests := []struct {
		name         string
		db, schema   string
		table        string
		wantContains string
	}{
		{"simple", "", "", "my_table", `GET_DDL('TABLE', '"my_table"')`},
		{"fully qualified", "db", "sch", "t", `GET_DDL('TABLE', '"db"."sch"."t"')`},
		{"single quote in name", "", "", "it's", `GET_DDL('TABLE', '"it''s"')`},
		{"dot in name", "", "", "my.table", `GET_DDL('TABLE', '"my.table"')`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured string
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				var req statementRequest
				json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
				captured = req.Statement

				meta := resultSetMetaData{NumRows: 0, RowType: []rowType{{Name: "ddl", Type: "TEXT"}}}
				metaJSON, _ := json.Marshal(meta)
				resp := statementResponse{Code: "090001", ResultSetMetaData: metaJSON, Data: json.RawMessage("[]")}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck
			})

			args := map[string]any{"table": tt.table}
			if tt.db != "" {
				args["database"] = tt.db
			}
			if tt.schema != "" {
				args["schema"] = tt.schema
			}

			result, err := showCreateTable(context.Background(), s, args)
			require.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Contains(t, captured, tt.wantContains, "SQL: %s", captured)
		})
	}
}
