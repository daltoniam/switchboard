package snowflake

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listDatabases(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return s.execSimpleQuery(ctx, "SHOW DATABASES", role)
}

func listSchemas(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	query := "SHOW SCHEMAS"
	if db != "" {
		query += " IN DATABASE " + quoteIdentifier(db)
	}
	return s.execSimpleQuery(ctx, query, role)
}

func listTables(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	query := "SHOW TABLES"
	if db != "" && schema != "" {
		query += " IN " + quoteIdentifier(db) + "." + quoteIdentifier(schema)
	} else if db != "" {
		query += " IN DATABASE " + quoteIdentifier(db)
	}
	return s.execSimpleQuery(ctx, query, role)
}

func listViews(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	query := "SHOW VIEWS"
	if db != "" && schema != "" {
		query += " IN " + quoteIdentifier(db) + "." + quoteIdentifier(schema)
	} else if db != "" {
		query += " IN DATABASE " + quoteIdentifier(db)
	}
	return s.execSimpleQuery(ctx, query, role)
}

func describeTable(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		return mcp.ErrResult(fmt.Errorf("table is required"))
	}

	fqn := qualifyTable(db, schema, table)
	return s.execSimpleQuery(ctx, "DESCRIBE TABLE "+fqn, role)
}

func showCreateTable(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		return mcp.ErrResult(fmt.Errorf("table is required"))
	}

	fqn := qualifyTable(db, schema, table)
	return s.execSimpleQuery(ctx, "SELECT GET_DDL('TABLE', '"+fqn+"')", role) // #nosec G201 -- identifiers quoted via quoteIdentifier
}

func listWarehouses(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return s.execSimpleQuery(ctx, "SHOW WAREHOUSES", role)
}

func listRunningQueries(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	limit := r.Int("limit")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`SELECT query_id, query_text, user_name, warehouse_name, 
		execution_status, start_time, end_time, total_elapsed_time, rows_produced 
		FROM TABLE(information_schema.query_history_by_warehouse()) 
		ORDER BY start_time DESC LIMIT %d`, limit)
	return s.execSimpleQuery(ctx, query, role)
}

func currentSession(ctx context.Context, s *snowflake, _ map[string]any) (*mcp.ToolResult, error) {
	query := `SELECT CURRENT_USER() AS current_user, CURRENT_ROLE() AS current_role, 
		CURRENT_WAREHOUSE() AS current_warehouse, CURRENT_DATABASE() AS current_database, 
		CURRENT_SCHEMA() AS current_schema, CURRENT_SESSION() AS session_id, 
		CURRENT_VERSION() AS snowflake_version`
	return s.execSimpleQuery(ctx, query, "")
}

func listUsers(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return s.execSimpleQuery(ctx, "SHOW USERS", role)
}

func listRoles(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return s.execSimpleQuery(ctx, "SHOW ROLES", role)
}

func listStages(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	query := "SHOW STAGES"
	if db != "" && schema != "" {
		query += " IN " + quoteIdentifier(db) + "." + quoteIdentifier(schema)
	} else if db != "" {
		query += " IN DATABASE " + quoteIdentifier(db)
	}
	return s.execSimpleQuery(ctx, query, role)
}

func listTasks(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	query := "SHOW TASKS"
	if db != "" && schema != "" {
		query += " IN " + quoteIdentifier(db) + "." + quoteIdentifier(schema)
	} else if db != "" {
		query += " IN DATABASE " + quoteIdentifier(db)
	}
	return s.execSimpleQuery(ctx, query, role)
}

func listPipes(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	query := "SHOW PIPES"
	if db != "" && schema != "" {
		query += " IN " + quoteIdentifier(db) + "." + quoteIdentifier(schema)
	} else if db != "" {
		query += " IN DATABASE " + quoteIdentifier(db)
	}
	return s.execSimpleQuery(ctx, query, role)
}

func listStreams(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	schema := r.Str("schema")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	query := "SHOW STREAMS"
	if db != "" && schema != "" {
		query += " IN " + quoteIdentifier(db) + "." + quoteIdentifier(schema)
	} else if db != "" {
		query += " IN DATABASE " + quoteIdentifier(db)
	}
	return s.execSimpleQuery(ctx, query, role)
}

// --- Helpers ---

// execSimpleQuery runs a SQL statement with an optional role override and returns formatted results.
func (s *snowflake) execSimpleQuery(ctx context.Context, sql, role string) (*mcp.ToolResult, error) {
	var opts *statementRequest
	if role != "" {
		opts = &statementRequest{Role: role}
	}

	resp, err := s.submitStatement(ctx, sql, opts)
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := formatResults(resp)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// quoteIdentifier wraps a Snowflake identifier in double quotes, escaping internal quotes.
func quoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// qualifyTable builds a fully-qualified table reference.
func qualifyTable(db, schema, table string) string {
	var parts []string
	if db != "" {
		parts = append(parts, quoteIdentifier(db))
	}
	if schema != "" {
		parts = append(parts, quoteIdentifier(schema))
	}
	parts = append(parts, quoteIdentifier(table))
	return strings.Join(parts, ".")
}
