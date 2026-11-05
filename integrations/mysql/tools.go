package mysql

import mcp "github.com/daltoniam/switchboard"

func (m *mysql) Tools() []mcp.ToolDefinition { return tools }

var tools = buildTools()

func buildTools() []mcp.ToolDefinition {
	definitions := []struct {
		name        mcp.ToolName
		description string
		table       bool
		database    bool
		query       bool
	}{
		{"mysql_list_databases", "List accessible MySQL databases and schemas. Start here for MySQL schema discovery; connection selects a configured alias (default if omitted).", false, false, false},
		{"mysql_list_tables", "List MySQL tables with storage engine, estimated row counts and data/index size. Use after mysql_list_databases.", false, true, false},
		{"mysql_describe_table", "Describe a MySQL table: column types, indexes, foreign keys and SHOW CREATE TABLE DDL. Use after mysql_list_tables.", true, true, false},
		{"mysql_list_columns", "List MySQL table columns, data types, nullability, defaults and ordinal positions. Use after mysql_list_tables.", true, true, false},
		{"mysql_list_indexes", "List MySQL table indexes, indexed columns, uniqueness and cardinality. Use after mysql_list_tables.", true, true, false},
		{"mysql_list_foreign_keys", "List outgoing MySQL foreign key relationships, referenced tables and update/delete rules. Use after mysql_list_tables.", true, true, false},
		{"mysql_list_views", "List MySQL database views and SQL definitions. Requires SHOW VIEW permission; use after mysql_list_databases.", false, true, false},
		{"mysql_table_stats", "Get MySQL table storage statistics, row estimate, engine, collation and auto increment. Use after mysql_list_tables.", true, true, false},
		{"mysql_running_queries", "List MySQL active queries from performance_schema.processlist. PROCESS is optional for broader visibility; no kill operation.", false, false, false},
		{"mysql_server_info", "Get MySQL server version, uptime, max connections and thread counts for database diagnostics.", false, false, false},
		{"mysql_query", "Query MySQL 8.4 with a single bounded SELECT, supported SHOW or EXPLAIN SELECT. Read-only transaction, 10s timeout, 1 MiB response, 100 rows default (1000 max). Only allowlisted built-in functions; no comments, semicolons, arbitrary routines or file operations. Requires least-privilege credentials; not a SQL sandbox.", false, false, true},
		{"mysql_explain", "Explain a MySQL SELECT query using EXPLAIN FORMAT=JSON without executing ANALYZE. Use after mysql_describe_table to inspect query plans and index usage.", false, false, true},
	}
	result := make([]mcp.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		parameters := map[string]string{"connection": "Configured connection alias (default: default)", "limit": "Maximum result rows, default 100, range 1-1000; truncated signals omitted rows"}
		var required []string
		if definition.database {
			parameters["database"] = "MySQL database/schema name from mysql_list_databases (not a connection alias)"
			required = append(required, "database")
		}
		if definition.table {
			parameters["table"] = "Table name from mysql_list_tables"
			required = append(required, "table")
		}
		if definition.query {
			parameters["query"] = "Single read-only SQL statement; mysql_explain accepts SELECT only"
			parameters["params"] = "Optional JSON array of positional values for ? placeholders"
			required = append(required, "query")
		}
		result = append(result, mcp.ToolDefinition{Name: definition.name, Description: definition.description, Parameters: parameters, Required: required})
	}
	return result
}
