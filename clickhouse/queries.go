package clickhouse

import (
	"context"
	"fmt"
	"strings"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	mcp "github.com/daltoniam/switchboard"
)

func executeQuery(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	query := argStr(args, "query")
	if query == "" {
		return errResult(fmt.Errorf("query is required"))
	}

	if db := argStr(args, "database"); db != "" {
		ctx = ch.Context(ctx, ch.WithSettings(ch.Settings{
			"database": db,
		}))
	}

	upper := strings.ToUpper(strings.TrimSpace(query))
	if !strings.Contains(upper, "LIMIT") {
		query += " LIMIT 10000"
	}

	if strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "DESCRIBE") ||
		strings.HasPrefix(upper, "DESC") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "EXISTS") ||
		strings.HasPrefix(upper, "WITH") {
		data, err := c.query(ctx, query)
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	if err := c.exec(ctx, query); err != nil {
		return errResult(err)
	}
	return rawResult([]byte(`{"status":"ok"}`))
}

func explainQuery(ctx context.Context, c *clickhouseInt, args map[string]any) (*mcp.ToolResult, error) {
	query := argStr(args, "query")
	if query == "" {
		return errResult(fmt.Errorf("query is required"))
	}

	explainType := strings.ToUpper(argStr(args, "type"))
	switch explainType {
	case "PIPELINE", "SYNTAX", "AST", "ESTIMATE":
	case "PLAN", "":
		explainType = "PLAN"
	default:
		return errResult(fmt.Errorf("invalid explain type: %s (valid: PLAN, PIPELINE, SYNTAX, AST, ESTIMATE)", explainType))
	}

	sql := "EXPLAIN " + explainType + " " + query // #nosec G201 -- explainType is validated against allowlist above; query is user-provided SQL (the tool's purpose)
	data, err := c.query(ctx, sql)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
