package postgres

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func databaseInfo(ctx context.Context, p *postgres, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := p.queryRow(ctx, `
		SELECT version() AS version,
		       current_database() AS database,
		       current_user AS user,
		       inet_server_addr() AS server_address,
		       inet_server_port() AS server_port,
		       pg_postmaster_start_time() AS server_start_time,
		       current_setting('server_encoding') AS encoding,
		       current_setting('TimeZone') AS timezone,
		       current_setting('max_connections') AS max_connections`)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func databaseSize(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	limit := argInt(args, "limit")
	if limit <= 0 {
		limit = 20
	}

	data, err := p.query(ctx, `
		SELECT current_database() AS database,
		       pg_size_pretty(pg_database_size(current_database())) AS database_size,
		       schemaname AS schema,
		       relname AS table_name,
		       pg_size_pretty(pg_total_relation_size(relid)) AS total_size,
		       pg_size_pretty(pg_relation_size(relid)) AS data_size,
		       pg_size_pretty(pg_indexes_size(relid)) AS index_size,
		       n_live_tup AS estimated_rows
		FROM pg_stat_user_tables
		ORDER BY pg_total_relation_size(relid) DESC
		LIMIT $1`, limit)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func tableStats(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.queryRow(ctx, `
		SELECT s.relname AS table_name,
		       s.schemaname AS schema,
		       s.n_live_tup AS live_rows,
		       s.n_dead_tup AS dead_rows,
		       s.n_tup_ins AS total_inserts,
		       s.n_tup_upd AS total_updates,
		       s.n_tup_del AS total_deletes,
		       s.last_vacuum,
		       s.last_autovacuum,
		       s.last_analyze,
		       s.last_autoanalyze,
		       s.vacuum_count,
		       s.autovacuum_count,
		       pg_size_pretty(pg_total_relation_size(s.relid)) AS total_size,
		       pg_size_pretty(pg_relation_size(s.relid)) AS data_size,
		       pg_size_pretty(pg_indexes_size(s.relid)) AS index_size,
		       (SELECT count(*) FROM pg_index WHERE indrelid = s.relid) AS index_count
		FROM pg_stat_user_tables s
		WHERE s.schemaname = $1 AND s.relname = $2`, schema, table)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listRoles(ctx context.Context, p *postgres, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := p.query(ctx, `
		SELECT rolname AS role_name,
		       rolsuper AS is_superuser,
		       rolcreatedb AS can_create_db,
		       rolcreaterole AS can_create_role,
		       rolcanlogin AS can_login,
		       rolreplication AS is_replication,
		       rolconnlimit AS connection_limit,
		       rolvaliduntil AS valid_until
		FROM pg_roles
		ORDER BY rolname`)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listGrants(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}
	table := argStr(args, "table")

	if table != "" {
		data, err := p.query(ctx, `
			SELECT grantee, privilege_type, is_grantable, table_schema, table_name
			FROM information_schema.table_privileges
			WHERE table_schema = $1 AND table_name = $2
			ORDER BY grantee, privilege_type`, schema, table)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	data, err := p.query(ctx, `
		SELECT grantee, privilege_type, is_grantable, table_schema, table_name
		FROM information_schema.table_privileges
		WHERE table_schema = $1
		ORDER BY table_name, grantee, privilege_type`, schema)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listExtensions(ctx context.Context, p *postgres, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := p.query(ctx, `
		SELECT extname AS name,
		       extversion AS version,
		       n.nspname AS schema,
		       obj_description(e.oid) AS comment
		FROM pg_extension e
		JOIN pg_namespace n ON n.oid = e.extnamespace
		ORDER BY extname`)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listActiveConnections(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	state := argStr(args, "state")

	if state != "" {
		data, err := p.query(ctx, `
			SELECT pid, usename AS user, datname AS database, client_addr, client_port,
			       state, backend_start, query_start, state_change,
			       EXTRACT(EPOCH FROM now() - query_start)::int AS duration_seconds,
			       LEFT(query, 200) AS query
			FROM pg_stat_activity
			WHERE datname = current_database() AND state = $1
			ORDER BY query_start`, state)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	data, err := p.query(ctx, `
		SELECT pid, usename AS user, datname AS database, client_addr, client_port,
		       state, backend_start, query_start, state_change,
		       EXTRACT(EPOCH FROM now() - query_start)::int AS duration_seconds,
		       LEFT(query, 200) AS query
		FROM pg_stat_activity
		WHERE datname = current_database()
		ORDER BY query_start`)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listLocks(ctx context.Context, p *postgres, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := p.query(ctx, `
		SELECT blocked_locks.pid AS blocked_pid,
		       blocked_activity.usename AS blocked_user,
		       LEFT(blocked_activity.query, 200) AS blocked_query,
		       blocking_locks.pid AS blocking_pid,
		       blocking_activity.usename AS blocking_user,
		       LEFT(blocking_activity.query, 200) AS blocking_query,
		       blocked_locks.locktype,
		       blocked_locks.mode AS blocked_mode,
		       blocking_locks.mode AS blocking_mode
		FROM pg_catalog.pg_locks blocked_locks
		JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
		JOIN pg_catalog.pg_locks blocking_locks
		  ON blocking_locks.locktype = blocked_locks.locktype
		  AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
		  AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
		  AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
		  AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
		  AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
		  AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
		  AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
		  AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
		  AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
		  AND blocking_locks.pid != blocked_locks.pid
		JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
		WHERE NOT blocked_locks.granted
		ORDER BY blocked_activity.query_start`)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func runningQueries(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	minDuration := argStr(args, "min_duration")

	if minDuration != "" {
		data, err := p.query(ctx, `
			SELECT pid, usename AS user, datname AS database,
			       state, query_start,
			       EXTRACT(EPOCH FROM now() - query_start)::int AS duration_seconds,
			       wait_event_type, wait_event,
			       LEFT(query, 500) AS query
			FROM pg_stat_activity
			WHERE datname = current_database()
			  AND state = 'active'
			  AND pid != pg_backend_pid()
			  AND EXTRACT(EPOCH FROM now() - query_start) > $1::int
			ORDER BY query_start`, minDuration)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	data, err := p.query(ctx, `
		SELECT pid, usename AS user, datname AS database,
		       state, query_start,
		       EXTRACT(EPOCH FROM now() - query_start)::int AS duration_seconds,
		       wait_event_type, wait_event,
		       LEFT(query, 500) AS query
		FROM pg_stat_activity
		WHERE datname = current_database()
		  AND state = 'active'
		  AND pid != pg_backend_pid()
		ORDER BY query_start`)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
