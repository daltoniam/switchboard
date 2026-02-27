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

	data, err := c.query(ctx, fmt.Sprintf("KILL QUERY WHERE query_id = '%s'", escapeSingleQuote(queryID)))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listSettings(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	pattern := argStr(args, "pattern")
	q := `SELECT name, value, changed, description, type FROM system.settings`
	if pattern != "" {
		q += fmt.Sprintf(" WHERE name LIKE '%s'", escapeSingleQuote(pattern))
	}
	q += " ORDER BY name LIMIT 200"

	data, err := c.query(ctx, q)
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
	dbFilter := "database = currentDatabase()"
	if db != "" {
		dbFilter = fmt.Sprintf("database = '%s'", escapeSingleQuote(db))
	}

	activeFilter := "AND active = 1"
	if argStr(args, "active") == "false" {
		activeFilter = ""
	}

	q := fmt.Sprintf(`SELECT
		partition,
		name,
		active,
		rows,
		bytes_on_disk,
		formatReadableSize(bytes_on_disk) AS readable_size,
		modification_time
	FROM system.parts
	WHERE %s AND table = '%s' %s
	ORDER BY modification_time DESC
	LIMIT 100`, dbFilter, escapeSingleQuote(table), activeFilter)

	data, err := c.query(ctx, q)
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

	conditions := []string{}
	if pattern := argStr(args, "query_pattern"); pattern != "" {
		conditions = append(conditions, fmt.Sprintf("query LIKE '%s'", escapeSingleQuote(pattern)))
	}
	if qtype := argStr(args, "query_type"); qtype != "" {
		conditions = append(conditions, fmt.Sprintf("type = '%s'", escapeSingleQuote(qtype)))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + conditions[0]
		for _, cond := range conditions[1:] {
			where += " AND " + cond
		}
	}

	q := fmt.Sprintf(`SELECT
		type,
		event_time,
		query_id,
		query_duration_ms,
		read_rows,
		read_bytes,
		result_rows,
		result_bytes,
		memory_usage,
		exception,
		query
	FROM system.query_log
	%s
	ORDER BY event_time DESC
	LIMIT %d`, where, limit)

	data, err := c.query(ctx, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
