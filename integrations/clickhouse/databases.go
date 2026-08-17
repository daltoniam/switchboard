package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	mcp "github.com/daltoniam/switchboard"
)

func listConnections(_ context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	type connInfo struct {
		Alias      string `json:"alias"`
		Host       string `json:"host"`
		Port       string `json:"port"`
		Username   string `json:"username"`
		Database   string `json:"database"`
		Secure     bool   `json:"secure"`
		SkipVerify bool   `json:"skip_verify"`
		Default    bool   `json:"is_default"`
	}

	aliases := make([]string, 0, len(c.conns))
	for alias := range c.conns {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	results := make([]connInfo, 0, len(c.conns))
	for _, alias := range aliases {
		conn := c.conns[alias]
		results = append(results, connInfo{
			Alias:      alias,
			Host:       conn.host,
			Port:       conn.port,
			Username:   conn.username,
			Database:   conn.database,
			Secure:     conn.secure,
			SkipVerify: conn.skipVerify,
			Default:    alias == c.defaultAlias,
		})
	}

	data, err := json.Marshal(results)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("marshal error: %w", err))
	}
	return mcp.RawResult(json.RawMessage(data))
}

func listDatabases(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	conn, err := c.getConnForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.query(ctx, conn, "SELECT name, engine, comment FROM system.databases ORDER BY name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTables(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	conn, err := c.getConnForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	db := r.Str("database")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if db == "" {
		q := `SELECT database, name, engine, total_rows, total_bytes, comment
			FROM system.tables WHERE database = currentDatabase() ORDER BY name`
		data, err := c.query(ctx, conn, q)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	q := `SELECT database, name, engine, total_rows, total_bytes, comment
		FROM system.tables WHERE database = ? ORDER BY name`
	data, err := c.query(ctx, conn, q, db)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func describeTable(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	conn, err := c.getConnForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
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

	data, err := c.query(ctx, conn, "DESCRIBE TABLE "+fqn) // #nosec G201 -- identifiers escaped via escapeIdentifier
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listColumns(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	conn, err := c.getConnForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
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
		data, err := c.query(ctx, conn, q, db, table)
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
	data, err := c.query(ctx, conn, q, table)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func showCreateTable(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	conn, err := c.getConnForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
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

	data, err := c.query(ctx, conn, "SHOW CREATE TABLE "+fqn) // #nosec G201 -- identifiers escaped via escapeIdentifier
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
