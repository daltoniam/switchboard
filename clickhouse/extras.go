package clickhouse

import (
	"context"
	"fmt"

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
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
}

func killQuery(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	queryID := argStr(args, "query_id")
	if queryID == "" {
		return errResult(fmt.Errorf("query_id is required"))
	}

	data, err := c.query(ctx, "KILL QUERY WHERE query_id = ?", queryID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listSettings(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	pattern := argStr(args, "pattern")
	if pattern != "" {
		data, err := c.query(ctx,
			"SELECT name, value, changed, description, type FROM system.settings WHERE name LIKE ? ORDER BY name LIMIT 200",
			pattern)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	data, err := c.query(ctx,
		"SELECT name, value, changed, description, type FROM system.settings ORDER BY name LIMIT 200")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
}

func listParts(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}

	db := argStr(args, "database")
	activeOnly := argStr(args, "active") != "false"

	if db != "" {
		if activeOnly {
			data, err := c.query(ctx, `SELECT partition, name, active, rows, bytes_on_disk,
				formatReadableSize(bytes_on_disk) AS readable_size, modification_time
				FROM system.parts
				WHERE database = ? AND table = ? AND active = 1
				ORDER BY modification_time DESC LIMIT 100`, db, table)
			if err != nil {
				return errResult(err)
			}
			return rawResult(data)
		}
		data, err := c.query(ctx, `SELECT partition, name, active, rows, bytes_on_disk,
			formatReadableSize(bytes_on_disk) AS readable_size, modification_time
			FROM system.parts
			WHERE database = ? AND table = ?
			ORDER BY modification_time DESC LIMIT 100`, db, table)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	if activeOnly {
		data, err := c.query(ctx, `SELECT partition, name, active, rows, bytes_on_disk,
			formatReadableSize(bytes_on_disk) AS readable_size, modification_time
			FROM system.parts
			WHERE database = currentDatabase() AND table = ? AND active = 1
			ORDER BY modification_time DESC LIMIT 100`, table)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}
	data, err := c.query(ctx, `SELECT partition, name, active, rows, bytes_on_disk,
		formatReadableSize(bytes_on_disk) AS readable_size, modification_time
		FROM system.parts
		WHERE database = currentDatabase() AND table = ?
		ORDER BY modification_time DESC LIMIT 100`, table)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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
		return errResult(err)
	}
	return rawResult(data)
}

func listUsers(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, "SHOW USERS")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listRoles(ctx context.Context, c *clickhouseInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.query(ctx, "SHOW ROLES")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func queryLog(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	limit := argInt(args, "limit")
	if limit <= 0 {
		limit = 50
	}

	pattern := argStr(args, "query_pattern")
	qtype := argStr(args, "query_type")

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
		return errResult(err)
	}
	return rawResult(data)
}
