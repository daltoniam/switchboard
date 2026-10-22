package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type rowResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
}

func readRows(ctx context.Context, db *sql.DB, query string, params []any, limit int) (*rowResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("mysql: cannot acquire connection")
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "SET SESSION max_execution_time = 10000"); err != nil {
		return nil, fmt.Errorf("mysql: cannot enforce execution timeout")
	}
	if _, err := conn.ExecContext(ctx, "SET SESSION sql_select_limit = 1001"); err != nil {
		return nil, fmt.Errorf("mysql: cannot enforce row limit")
	}
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("mysql: cannot begin read-only transaction")
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("mysql: read failed (check SQL syntax and SELECT / SHOW VIEW permissions)")
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("mysql: cannot read columns")
	}
	result := &rowResult{Columns: columns, Rows: make([][]any, 0)}
	header, err := json.Marshal(result)
	if err != nil || len(header) > maxResultBytes-128 {
		return nil, fmt.Errorf("mysql: result metadata exceeds byte limit")
	}
	used := len(header) + 128
	for rows.Next() {
		if len(result.Rows) == limit {
			result.Truncated = true
			break
		}
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for index := range values {
			targets[index] = &values[index]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("mysql: cannot scan result row")
		}
		for index, value := range values {
			if bytes, ok := value.([]byte); ok {
				values[index] = string(bytes)
			}
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return nil, fmt.Errorf("mysql: cannot encode result row")
		}
		if used+len(encoded)+1 > maxResultBytes {
			result.Truncated = true
			break
		}
		used += len(encoded) + 1
		result.Rows = append(result.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: result stream failed or timed out")
	}
	if result.Truncated {
		cancel()
	}
	return result, nil
}
