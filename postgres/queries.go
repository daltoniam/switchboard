package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

var validExplainFormats = map[string]bool{
	"text": true,
	"json": true,
	"yaml": true,
	"xml":  true,
}

func queryTool(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	sqlStr := argStr(args, "sql")
	if sqlStr == "" {
		return errResult(fmt.Errorf("sql is required"))
	}

	limit := argInt(args, "limit")
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	wrapped := fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d", strings.TrimRight(strings.TrimSpace(sqlStr), ";"), limit) // #nosec G201 -- intentional: this tool executes user-provided SQL in a read-only transaction

	tx, err := p.db.BeginTx(ctx, &readOnlyTx)
	if err != nil {
		return errResult(fmt.Errorf("begin transaction: %w", err))
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, wrapped)
	if err != nil {
		return errResult(fmt.Errorf("query error: %w", err))
	}
	defer func() { _ = rows.Close() }()

	data, err := scanRows(rows)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func executeTool(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	if p.readOnly {
		return errResult(fmt.Errorf("execute is disabled: set read_only=false in postgres credentials to enable"))
	}

	sqlStr := argStr(args, "sql")
	if sqlStr == "" {
		return errResult(fmt.Errorf("sql is required"))
	}

	upper := strings.ToUpper(strings.TrimSpace(sqlStr))
	for _, prefix := range []string{"DROP DATABASE", "TRUNCATE"} {
		if strings.HasPrefix(upper, prefix) {
			return errResult(fmt.Errorf("statement rejected: %s is not allowed", prefix))
		}
	}

	data, err := p.exec(ctx, sqlStr)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func explainTool(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	sqlStr := argStr(args, "sql")
	if sqlStr == "" {
		return errResult(fmt.Errorf("sql is required"))
	}

	format := strings.ToLower(argStr(args, "format"))
	if format == "" {
		format = "text"
	}
	if !validExplainFormats[format] {
		return errResult(fmt.Errorf("invalid format %q: must be one of text, json, yaml, xml", format))
	}
	analyze := argBool(args, "analyze")

	var explain string
	if analyze {
		explain = fmt.Sprintf("EXPLAIN (ANALYZE, FORMAT %s) %s", format, sqlStr)
	} else {
		explain = fmt.Sprintf("EXPLAIN (FORMAT %s) %s", format, sqlStr)
	}

	tx, err := p.db.BeginTx(ctx, &readOnlyTx)
	if err != nil {
		return errResult(fmt.Errorf("begin transaction: %w", err))
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, explain)
	if err != nil {
		return errResult(fmt.Errorf("explain error: %w", err))
	}
	defer func() { _ = rows.Close() }()

	data, err := scanRows(rows)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func selectTool(ctx context.Context, p *postgres, args map[string]any) (*mcp.ToolResult, error) {
	table := argStr(args, "table")
	if table == "" {
		return errResult(fmt.Errorf("table is required"))
	}
	schema := argStr(args, "schema")
	if schema == "" {
		schema = "public"
	}

	safeSchema, err := sanitizeIdentifier(schema)
	if err != nil {
		return errResult(err)
	}
	safeTable, err := sanitizeIdentifier(table)
	if err != nil {
		return errResult(err)
	}

	columns := argStr(args, "columns")
	if columns == "" {
		columns = "*"
	} else if err := validateSQLFragment(columns); err != nil {
		return errResult(fmt.Errorf("columns: %w", err))
	}

	q := fmt.Sprintf("SELECT %s FROM %s.%s", columns, safeSchema, safeTable) // #nosec G201 -- identifiers are sanitized via sanitizeIdentifier

	if where := argStr(args, "where"); where != "" {
		if err := validateSQLFragment(where); err != nil {
			return errResult(fmt.Errorf("where: %w", err))
		}
		q += " WHERE " + where
	}
	if orderBy := argStr(args, "order_by"); orderBy != "" {
		if err := validateSQLFragment(orderBy); err != nil {
			return errResult(fmt.Errorf("order_by: %w", err))
		}
		q += " ORDER BY " + orderBy
	}

	limit := argInt(args, "limit")
	if limit <= 0 {
		limit = 100
	}
	q += fmt.Sprintf(" LIMIT %d", limit)

	if offset := argInt(args, "offset"); offset > 0 {
		q += fmt.Sprintf(" OFFSET %d", offset)
	}

	tx, err := p.db.BeginTx(ctx, &readOnlyTx)
	if err != nil {
		return errResult(fmt.Errorf("begin transaction: %w", err))
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return errResult(fmt.Errorf("query error: %w", err))
	}
	defer func() { _ = rows.Close() }()

	data, err := scanRows(rows)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- helpers ---

var readOnlyTx = sql.TxOptions{ReadOnly: true}

func validateSQLFragment(s string) error {
	if strings.Contains(s, ";") {
		return fmt.Errorf("semicolons are not allowed")
	}
	if strings.Contains(s, "--") {
		return fmt.Errorf("line comments (--) are not allowed")
	}
	if strings.Contains(s, "/*") {
		return fmt.Errorf("block comments (/*) are not allowed")
	}
	return nil
}

func scanRows(rows *sql.Rows) (json.RawMessage, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns error: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		row := make(map[string]any, len(columns))
		for i, col := range columns {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if results == nil {
		results = []map[string]any{}
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}
	return json.RawMessage(data), nil
}
