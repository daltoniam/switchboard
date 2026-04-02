package snowflake

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

func executeQuery(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	db := r.Str("database")
	schema := r.Str("schema")
	warehouse := r.Str("warehouse")
	role := r.Str("role")
	timeout := r.Int("timeout")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if query == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}

	if timeout <= 0 {
		timeout = 60
	}

	upper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "WITH") {
		if !strings.Contains(upper, "LIMIT") {
			query += " LIMIT 10000"
		}
	}

	resp, err := s.submitStatement(ctx, query, &statementRequest{
		Database:  db,
		Schema:    schema,
		Warehouse: warehouse,
		Role:      role,
		Timeout:   timeout,
	})
	if err != nil {
		return mcp.ErrResult(err)
	}

	if resp.StatementHandle != "" && resp.Data == nil {
		resp, err = s.pollUntilComplete(ctx, resp.StatementHandle, time.Duration(timeout)*time.Second)
		if err != nil {
			return mcp.ErrResult(err)
		}
	}

	data, err := formatResults(resp)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getQueryStatus(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	handle := r.Str("statement_handle")
	partition := r.Int("partition")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if handle == "" {
		return mcp.ErrResult(fmt.Errorf("statement_handle is required"))
	}

	resp, err := s.getStatementStatus(ctx, handle, partition)
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := formatResults(resp)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cancelQuery(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	handle := r.Str("statement_handle")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if handle == "" {
		return mcp.ErrResult(fmt.Errorf("statement_handle is required"))
	}

	if err := s.cancelStatement(ctx, handle); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult([]byte(`{"status":"cancelled"}`))
}

// pollUntilComplete polls the statement status until the query finishes or times out.
func (s *snowflake) pollUntilComplete(ctx context.Context, handle string, timeout time.Duration) (*statementResponse, error) {
	deadline := time.Now().Add(timeout)
	backoff := 500 * time.Millisecond

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("snowflake: query timed out after %s", timeout)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		resp, err := s.getStatementStatus(ctx, handle, 0)
		if err != nil {
			return nil, err
		}

		if resp.Data != nil || resp.Code == "090001" {
			return resp, nil
		}

		if backoff < 4*time.Second {
			backoff *= 2
		}
	}
}
