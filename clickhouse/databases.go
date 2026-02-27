package clickhouse

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDatabases(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, "SELECT name, engine, comment FROM system.databases ORDER BY name")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listTables(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	db := argStr(args, "database")
	q := `SELECT
		database,
		name,
		engine,
		total_rows,
		total_bytes,
		comment
	FROM system.tables
	WHERE database = currentDatabase()
	ORDER BY name`

	if db != "" {
		q = fmt.Sprintf(`SELECT
			database,
			name,
			engine,
			total_rows,
			total_bytes,
			comment
		FROM system.tables
		WHERE database = '%s'
		ORDER BY name`, escapeSingleQuote(db))
	}

	data, err := c.query(ctx, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func describeTable(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}

	db := argStr(args, "database")
	var fqn string
	if db != "" {
		fqn = escapeIdentifier(db) + "." + escapeIdentifier(table)
	} else {
		fqn = escapeIdentifier(table)
	}

	data, err := c.query(ctx, "DESCRIBE TABLE "+fqn)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listColumns(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}

	db := argStr(args, "database")
	dbFilter := "database = currentDatabase()"
	if db != "" {
		dbFilter = fmt.Sprintf("database = '%s'", escapeSingleQuote(db))
	}

	q := fmt.Sprintf(`SELECT
		name,
		type,
		default_kind,
		default_expression,
		comment,
		is_in_partition_key,
		is_in_sorting_key,
		is_in_primary_key
	FROM system.columns
	WHERE %s AND table = '%s'
	ORDER BY position`, dbFilter, escapeSingleQuote(table))

	data, err := c.query(ctx, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func showCreateTable(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}

	db := argStr(args, "database")
	var fqn string
	if db != "" {
		fqn = escapeIdentifier(db) + "." + escapeIdentifier(table)
	} else {
		fqn = escapeIdentifier(table)
	}

	data, err := c.query(ctx, "SHOW CREATE TABLE "+fqn)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func escapeSingleQuote(s string) string {
	return replaceAll(s, "'", "\\'")
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}
