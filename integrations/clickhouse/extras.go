package clickhouse

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func serverInfo(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, `SELECT
		version() AS version,
		uptime() AS uptime_seconds,
		currentDatabase() AS current_database,
		currentUser() AS current_user,
		hostName() AS hostname,
		OSVersion() AS os_version,
		totalMemory() AS total_memory_bytes`)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProcesses(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, `SELECT
		query_id,
		user,
		address,
		elapsed,
		read_rows,
		read_bytes,
		memory_usage,
		query
	FROM system.processes
	ORDER BY elapsed DESC`)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func killQuery(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queryID := r.Str("query_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if queryID == "" {
		return mcp.ErrResult(fmt.Errorf("query_id is required"))
	}

	data, err := c.query(ctx, "KILL QUERY WHERE query_id = ?", queryID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listSettings(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pattern := r.Str("pattern")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pattern != "" {
		data, err := c.query(ctx,
			"SELECT name, value, changed, description, type FROM system.settings WHERE name LIKE ? ORDER BY name LIMIT 200",
			pattern)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	data, err := c.query(ctx,
		"SELECT name, value, changed, description, type FROM system.settings ORDER BY name LIMIT 200")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listMerges(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, `SELECT
		database,
		table,
		elapsed,
		progress,
		num_parts,
		total_size_bytes_compressed,
		rows_read,
		rows_written
	FROM system.merges
	ORDER BY elapsed DESC`)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listReplicas(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, `SELECT
		database,
		table,
		is_leader,
		is_readonly,
		absolute_delay,
		queue_size,
		inserts_in_queue,
		merges_in_queue,
		total_replicas,
		active_replicas
	FROM system.replicas
	ORDER BY database, table`)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func diskUsage(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, `SELECT
		database,
		count() AS table_count,
		sum(total_rows) AS total_rows,
		sum(total_bytes) AS total_bytes,
		formatReadableSize(sum(total_bytes)) AS readable_size
	FROM system.tables
	WHERE database NOT IN ('system', 'INFORMATION_SCHEMA', 'information_schema')
	GROUP BY database
	ORDER BY total_bytes DESC`)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listParts(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	db := r.Str("database")
	activeOnly := r.Str("active") != "false"
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		return mcp.ErrResult(fmt.Errorf("table is required"))
	}

	q := `SELECT partition, name, active, rows, bytes_on_disk,
		formatReadableSize(bytes_on_disk) AS readable_size, modification_time
		FROM system.parts WHERE `
	var conds []string
	var qargs []any
	if db != "" {
		conds = append(conds, "database = ?")
		qargs = append(qargs, db)
	} else {
		conds = append(conds, "database = currentDatabase()")
	}
	conds = append(conds, "table = ?")
	qargs = append(qargs, table)
	if activeOnly {
		conds = append(conds, "active = 1")
	}
	q += strings.Join(conds, " AND ") + " ORDER BY modification_time DESC LIMIT 100"
	data, err := c.query(ctx, q, qargs...)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listDictionaries(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, `SELECT
		database,
		name,
		status,
		origin,
		type,
		element_count,
		bytes_allocated,
		loading_duration
	FROM system.dictionaries
	ORDER BY database, name`)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUsers(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, "SHOW USERS")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRoles(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, "SHOW ROLES")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryLog(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	limit := r.Int("limit")
	pattern := r.Str("query_pattern")
	qtype := r.Str("query_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = 50
	}

	baseQuery := `SELECT type, event_time, query_id, query_duration_ms, read_rows, read_bytes,
		result_rows, result_bytes, memory_usage, exception, query
		FROM system.query_log`

	var queryArgs []any

	switch {
	case pattern != "" && qtype != "":
		baseQuery += " WHERE query LIKE ? AND type = ?"
		queryArgs = append(queryArgs, pattern, qtype)
	case pattern != "":
		baseQuery += " WHERE query LIKE ?"
		queryArgs = append(queryArgs, pattern)
	case qtype != "":
		baseQuery += " WHERE type = ?"
		queryArgs = append(queryArgs, qtype)
	}

	baseQuery += fmt.Sprintf(" ORDER BY event_time DESC LIMIT %d", limit)

	data, err := c.query(ctx, baseQuery, queryArgs...)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
