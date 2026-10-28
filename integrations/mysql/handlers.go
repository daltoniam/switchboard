package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

type handlerFunc func(context.Context, *sql.DB, map[string]any, int) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"mysql_list_databases":    catalogHandler("SELECT SCHEMA_NAME AS database_name, DEFAULT_CHARACTER_SET_NAME AS character_set, DEFAULT_COLLATION_NAME AS collation FROM information_schema.SCHEMATA ORDER BY SCHEMA_NAME", false, false),
	"mysql_list_tables":       catalogHandler("SELECT TABLE_NAME AS table_name, TABLE_TYPE AS table_type, ENGINE AS engine, TABLE_ROWS AS estimated_rows, DATA_LENGTH AS data_bytes, INDEX_LENGTH AS index_bytes FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME", true, false),
	"mysql_list_columns":      catalogHandler(columnsSQL, true, true),
	"mysql_list_indexes":      catalogHandler(indexesSQL, true, true),
	"mysql_list_foreign_keys": catalogHandler(foreignKeysSQL, true, true),
	"mysql_list_views":        catalogHandler("SELECT TABLE_NAME AS view_name, VIEW_DEFINITION AS definition, CHECK_OPTION AS check_option, IS_UPDATABLE AS is_updatable FROM information_schema.VIEWS WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME", true, false),
	"mysql_table_stats":       catalogHandler("SELECT TABLE_NAME AS table_name, ENGINE AS engine, TABLE_ROWS AS estimated_rows, DATA_LENGTH AS data_bytes, INDEX_LENGTH AS index_bytes, DATA_FREE AS free_bytes, AUTO_INCREMENT AS auto_increment, TABLE_COLLATION AS collation FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", true, true),
	"mysql_running_queries":   catalogHandler("SELECT ID AS id, USER AS user, HOST AS host, DB AS database_name, COMMAND AS command, TIME AS seconds, STATE AS state, LEFT(INFO, 4096) AS query FROM performance_schema.processlist WHERE COMMAND <> 'Sleep' ORDER BY TIME DESC, ID", false, false),
	"mysql_server_info":       catalogHandler("SELECT VERSION() AS version, @@version_comment AS variant, @@max_connections AS max_connections, (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Uptime') AS uptime_seconds, (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Threads_connected') AS threads_connected, (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Threads_running') AS threads_running", false, false),
	"mysql_describe_table":    describeTable,
	"mysql_query":             executeQuery,
	"mysql_explain":           explainQuery,
}

const columnsSQL = "SELECT COLUMN_NAME AS column_name, ORDINAL_POSITION AS position, COLUMN_TYPE AS column_type, IS_NULLABLE AS nullable, COLUMN_DEFAULT AS default_value, COLUMN_KEY AS column_key, EXTRA AS extra, COLUMN_COMMENT AS comment FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION"
const indexesSQL = "SELECT INDEX_NAME AS index_name, NON_UNIQUE AS non_unique, SEQ_IN_INDEX AS position, COLUMN_NAME AS column_name, COLLATION AS collation, CARDINALITY AS cardinality, SUB_PART AS prefix_length, INDEX_TYPE AS index_type, IS_VISIBLE AS visible, EXPRESSION AS expression FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY INDEX_NAME, SEQ_IN_INDEX"
const foreignKeysSQL = "SELECT k.CONSTRAINT_NAME AS constraint_name, k.COLUMN_NAME AS column_name, k.ORDINAL_POSITION AS position, k.REFERENCED_TABLE_SCHEMA AS referenced_database, k.REFERENCED_TABLE_NAME AS referenced_table, k.REFERENCED_COLUMN_NAME AS referenced_column, r.UPDATE_RULE AS update_rule, r.DELETE_RULE AS delete_rule FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME AND r.TABLE_NAME = k.TABLE_NAME WHERE k.TABLE_SCHEMA = ? AND k.TABLE_NAME = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION"

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_$-]{1,64}$`)

func catalogArgs(args map[string]any, databaseRequired, tableRequired bool) ([]any, error) {
	reader := mcp.NewArgs(args)
	database, table := reader.Str("database"), reader.Str("table")
	if err := reader.Err(); err != nil {
		return nil, err
	}
	var params []any
	if databaseRequired {
		if !identifierPattern.MatchString(database) {
			return nil, fmt.Errorf("mysql: database must be a nonempty identifier (letters, numbers, _, $, -; max 64 characters)")
		}
		params = append(params, database)
	}
	if tableRequired {
		if !identifierPattern.MatchString(table) {
			return nil, fmt.Errorf("mysql: table must be a nonempty identifier (letters, numbers, _, $, -; max 64 characters)")
		}
		params = append(params, table)
	}
	return params, nil
}

func catalogHandler(query string, databaseRequired, tableRequired bool) handlerFunc {
	return func(ctx context.Context, db *sql.DB, args map[string]any, limit int) (*mcp.ToolResult, error) {
		params, err := catalogArgs(args, databaseRequired, tableRequired)
		if err != nil {
			return mcp.ErrResult(err)
		}
		result, err := readRows(ctx, db, query+fmt.Sprintf(" LIMIT %d", limit+1), params, limit)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.JSONResult(result)
	}
}

func queryArgs(args map[string]any) (string, []any, error) {
	reader := mcp.NewArgs(args)
	query := reader.Str("query")
	if err := reader.Err(); err != nil {
		return "", nil, err
	}
	if err := validateQuery(query); err != nil {
		return "", nil, err
	}
	var params []any
	if raw, ok := args["params"]; ok {
		switch value := raw.(type) {
		case []any:
			params = value
		case string:
			decoded, err := decodeParamsJSON(value)
			if err != nil {
				return "", nil, fmt.Errorf("mysql: params must be a JSON array")
			}
			params = decoded
		default:
			return "", nil, fmt.Errorf("mysql: params must be a JSON array")
		}
	}
	for index, value := range params {
		switch value.(type) {
		case nil, string, bool, float64, int, int64, json.Number:
		default:
			return "", nil, fmt.Errorf("mysql: params must contain only scalar values")
		}
		normalized, err := normalizeQueryParam(value)
		if err != nil {
			return "", nil, err
		}
		params[index] = normalized
	}
	return query, params, nil
}

func decodeParamsJSON(raw string) ([]any, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	var params []any
	if err := dec.Decode(&params); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("mysql: params must be a JSON array")
	}
	return params, nil
}

func normalizeQueryParam(value any) (any, error) {
	number, ok := value.(json.Number)
	if !ok {
		return value, nil
	}
	if integer, err := number.Int64(); err == nil {
		return integer, nil
	}
	decimal, err := number.Float64()
	if err != nil {
		return nil, fmt.Errorf("mysql: params must contain only scalar values")
	}
	return decimal, nil
}

func executeQuery(ctx context.Context, db *sql.DB, args map[string]any, limit int) (*mcp.ToolResult, error) {
	query, params, err := queryArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	result, err := readRows(ctx, db, query, params, limit)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(result)
}

func explainQuery(ctx context.Context, db *sql.DB, args map[string]any, limit int) (*mcp.ToolResult, error) {
	query, params, err := queryArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
		return mcp.ErrResult(fmt.Errorf("mysql: explain requires a SELECT query"))
	}
	result, err := readRows(ctx, db, "EXPLAIN FORMAT=JSON "+query, params, limit)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(result)
}

func describeTable(ctx context.Context, db *sql.DB, args map[string]any, limit int) (*mcp.ToolResult, error) {
	params, err := catalogArgs(args, true, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	result := make(map[string]*rowResult, 4)
	for _, part := range []struct{ name, query string }{{"columns", columnsSQL}, {"indexes", indexesSQL}, {"foreign_keys", foreignKeysSQL}} {
		rows, err := readRows(ctx, db, part.query+fmt.Sprintf(" LIMIT %d", limit+1), params, limit)
		if err != nil {
			return mcp.ErrResult(err)
		}
		result[part.name] = rows
	}
	ddl, err := readRows(ctx, db, fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", params[0], params[1]), nil, 1)
	if err != nil {
		return mcp.ErrResult(err)
	}
	result["ddl"] = ddl
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > maxResultBytes {
		return mcp.ErrResult(fmt.Errorf("mysql: table description exceeds byte limit; use individual catalog tools"))
	}
	return mcp.RawResult(encoded)
}
