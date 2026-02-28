package postgres

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listSchemas(ctx context.Context, p *postgres, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := p.query(ctx, `
		SELECT schema_name, 
		       schema_owner,
		       CASE WHEN schema_name IN ('pg_catalog', 'information_schema', 'pg_toast') THEN true ELSE false END AS is_system
		FROM information_schema.schemata
		ORDER BY schema_name`)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listTables(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT c.relname AS table_name,
		       pg_size_pretty(pg_total_relation_size(c.oid)) AS total_size,
		       pg_size_pretty(pg_relation_size(c.oid)) AS data_size,
		       pg_size_pretty(pg_indexes_size(c.oid)) AS index_size,
		       COALESCE(s.n_live_tup, 0) AS estimated_rows,
		       obj_description(c.oid) AS comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_stat_user_tables s ON s.relid = c.oid
		WHERE n.nspname = $1
		  AND c.relkind = 'r'
		ORDER BY c.relname`, schema)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func describeTable(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT c.column_name,
		       c.data_type,
		       c.character_maximum_length,
		       c.numeric_precision,
		       c.is_nullable,
		       c.column_default,
		       col_description(
		         (SELECT oid FROM pg_class WHERE relname = c.table_name AND relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = c.table_schema)),
		         c.ordinal_position
		       ) AS comment,
		       CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END AS is_primary_key
		FROM information_schema.columns c
		LEFT JOIN (
		  SELECT ku.column_name, ku.table_name, ku.table_schema
		  FROM information_schema.table_constraints tc
		  JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name AND tc.table_schema = ku.table_schema
		  WHERE tc.constraint_type = 'PRIMARY KEY'
		) pk ON pk.column_name = c.column_name AND pk.table_name = c.table_name AND pk.table_schema = c.table_schema
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`, schema, table)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listColumns(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT column_name, data_type, is_nullable, column_default, ordinal_position
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listIndexes(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT i.relname AS index_name,
		       ix.indisunique AS is_unique,
		       ix.indisprimary AS is_primary,
		       pg_get_indexdef(ix.indexrelid) AS definition,
		       pg_size_pretty(pg_relation_size(i.oid)) AS size
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1 AND t.relname = $2
		ORDER BY i.relname`, schema, table)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listConstraints(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT tc.constraint_name,
		       tc.constraint_type,
		       tc.table_name,
		       kcu.column_name,
		       ccu.table_name AS references_table,
		       ccu.column_name AS references_column,
		       cc.check_clause
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		LEFT JOIN information_schema.constraint_column_usage ccu
		  ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		  AND tc.constraint_type = 'FOREIGN KEY'
		LEFT JOIN information_schema.check_constraints cc
		  ON tc.constraint_name = cc.constraint_name AND tc.constraint_schema = cc.constraint_schema
		WHERE tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY tc.constraint_type, tc.constraint_name`, schema, table)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listForeignKeys(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT tc.constraint_name,
		       kcu.column_name AS from_column,
		       ccu.table_schema AS to_schema,
		       ccu.table_name AS to_table,
		       ccu.column_name AS to_column,
		       rc.update_rule,
		       rc.delete_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		JOIN information_schema.referential_constraints rc
		  ON tc.constraint_name = rc.constraint_name AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		  AND tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY tc.constraint_name`, schema, table)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listViews(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT table_name AS view_name,
		       view_definition,
		       is_updatable,
		       is_insertable_into
		FROM information_schema.views
		WHERE table_schema = $1
		ORDER BY table_name`, schema)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listFunctions(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT p.proname AS function_name,
		       pg_get_function_arguments(p.oid) AS arguments,
		       pg_get_function_result(p.oid) AS return_type,
		       CASE p.prokind WHEN 'f' THEN 'function' WHEN 'p' THEN 'procedure' WHEN 'a' THEN 'aggregate' WHEN 'w' THEN 'window' END AS kind,
		       p.provolatile AS volatility,
		       obj_description(p.oid) AS comment
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1
		ORDER BY p.proname`, schema)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listTriggers(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}
	table := argStr(args, "table")

	q := `
		SELECT trigger_name,
		       event_manipulation AS event,
		       event_object_table AS table_name,
		       action_timing AS timing,
		       action_orientation AS orientation,
		       action_statement
		FROM information_schema.triggers
		WHERE trigger_schema = $1`

	if table != "" {
		q += ` AND event_object_table = $2 ORDER BY trigger_name`
		data, err := p.query(ctx, q, schema, table)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	q += ` ORDER BY event_object_table, trigger_name`
	data, err := p.query(ctx, q, schema)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listEnums(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	data, err := p.query(ctx, `
		SELECT t.typname AS enum_name,
		       array_agg(e.enumlabel ORDER BY e.enumsortorder) AS values
		FROM pg_type t
		JOIN pg_enum e ON t.oid = e.enumtypid
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = $1
		GROUP BY t.typname
		ORDER BY t.typname`, schema)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
