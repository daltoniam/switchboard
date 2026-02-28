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

	if db == "" {
		q := `SELECT database, name, engine, total_rows, total_bytes, comment
			FROM system.tables WHERE database = currentDatabase() ORDER BY name`
		data, err := c.query(ctx, q)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	q := `SELECT database, name, engine, total_rows, total_bytes, comment
		FROM system.tables WHERE database = ? ORDER BY name`
	data, err := c.query(ctx, q, db)
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

	data, err := c.query(ctx, "DESCRIBE TABLE "+fqn) // #nosec G201 -- identifiers escaped via escapeIdentifier
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

	if db != "" {
		q := `SELECT name, type, default_kind, default_expression, comment,
			is_in_partition_key, is_in_sorting_key, is_in_primary_key
			FROM system.columns
			WHERE database = ? AND table = ?
			ORDER BY position`
		data, err := c.query(ctx, q, db, table)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	q := `SELECT name, type, default_kind, default_expression, comment,
		is_in_partition_key, is_in_sorting_key, is_in_primary_key
		FROM system.columns
		WHERE database = currentDatabase() AND table = ?
		ORDER BY position`
	data, err := c.query(ctx, q, table)
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

	data, err := c.query(ctx, "SHOW CREATE TABLE "+fqn) // #nosec G201 -- identifiers escaped via escapeIdentifier
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
