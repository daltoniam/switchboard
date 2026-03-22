package clickhouse

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDatabases(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, "SELECT name, engine, comment FROM system.databases ORDER BY name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTables(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	db := r.Str("database")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if db == "" {
		q := `SELECT database, name, engine, total_rows, total_bytes, comment
			FROM system.tables WHERE database = currentDatabase() ORDER BY name`
		data, err := c.query(ctx, q)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	q := `SELECT database, name, engine, total_rows, total_bytes, comment
		FROM system.tables WHERE database = ? ORDER BY name`
	data, err := c.query(ctx, q, db)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func describeTable(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	db := r.Str("database")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		return mcp.ErrResult(fmt.Errorf("table is required"))
	}
	var fqn string
	if db != "" {
		fqn = escapeIdentifier(db) + "." + escapeIdentifier(table)
	} else {
		fqn = escapeIdentifier(table)
	}

	data, err := c.query(ctx, "DESCRIBE TABLE "+fqn) // #nosec G201 -- identifiers escaped via escapeIdentifier
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listColumns(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	db := r.Str("database")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		return mcp.ErrResult(fmt.Errorf("table is required"))
	}

	if db != "" {
		q := `SELECT name, type, default_kind, default_expression, comment,
			is_in_partition_key, is_in_sorting_key, is_in_primary_key
			FROM system.columns
			WHERE database = ? AND table = ?
			ORDER BY position`
		data, err := c.query(ctx, q, db, table)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	q := `SELECT name, type, default_kind, default_expression, comment,
		is_in_partition_key, is_in_sorting_key, is_in_primary_key
		FROM system.columns
		WHERE database = currentDatabase() AND table = ?
		ORDER BY position`
	data, err := c.query(ctx, q, table)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func showCreateTable(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	db := r.Str("database")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		return mcp.ErrResult(fmt.Errorf("table is required"))
	}
	var fqn string
	if db != "" {
		fqn = escapeIdentifier(db) + "." + escapeIdentifier(table)
	} else {
		fqn = escapeIdentifier(table)
	}

	data, err := c.query(ctx, "SHOW CREATE TABLE "+fqn) // #nosec G201 -- identifiers escaped via escapeIdentifier
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
