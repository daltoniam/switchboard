package panther

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultQueryTimeout = 30 * time.Second
	maxQueryTimeout     = 5 * time.Minute
	defaultResultRows   = 100
	maxResultRows       = 999
)

var dataLakeSpecs = []spec{
	{
		name:   "panther_list_databases",
		desc:   "List Panther security data lake databases (panther_logs, panther_rule_matches, panther_signals, panther_cloudsecurity, panther_views). Start here for log search and threat hunting with SQL, before panther_list_database_tables.",
		access: mcp.AccessRead,
		run:    listDatabases,
	},
	{
		name:   "panther_list_database_tables",
		desc:   "List tables in a Panther data lake database; each table holds one log type, such as aws_cloudtrail. Use after panther_list_databases, before panther_get_table_schema.",
		access: mcp.AccessRead,
		params: append([]param{str("database_name", "", "Database name from panther_list_databases, such as panther_logs.").req()}, pageParams()...),
		run:    listDatabaseTables,
	},
	{
		name:   "panther_get_table_schema",
		desc:   "Get the columns (field names, types, descriptions) of a Panther data lake table, to write correct SQL for log search. Use after panther_list_database_tables.",
		access: mcp.AccessRead,
		params: []param{
			str("database_name", "", "Database name, such as panther_logs.").req(),
			str("table_name", "", "Table name, such as aws_cloudtrail.").req(),
		},
		run: getTableSchema,
	},
	{
		name: "panther_query_data_lake",
		desc: "Run a SQL query on the Panther security data lake (Snowflake or Databricks SQL) for log search, threat hunting, and incident investigation, and wait for the first page of rows. " +
			"Always filter on time with p_event_time or a Panther macro such as p_occurs_since('1 d'), name tables in full (panther_logs.public.aws_cloudtrail on Snowflake), and use p_any_ip_addresses, p_any_usernames, or p_any_emails to correlate indicators. " +
			"Panther runs data lake queries as a read-only database user. If the query is still running at timeout_seconds, the result has status running and a queryId for panther_get_data_lake_query. Use panther_get_table_schema first for column names.",
		access: mcp.AccessRead,
		params: []param{
			str("sql", "", "SQL to run, unchanged. Include a time filter and a LIMIT.").req(),
			str("database_name", "", "Default database for unqualified table names (Panther's default is panther_logs)."),
			integer("timeout_seconds", "", "How long to wait for results, 1 to 300 (default 30). The query keeps running in Panther after this."),
			integer("page_size", "", fmt.Sprintf("Rows per page, 1 to %d (default %d).", maxResultRows, defaultResultRows)),
		},
		run: queryDataLake,
	},
	{
		name:   "panther_get_data_lake_query",
		desc:   "Get the status and a page of result rows of a Panther data lake SQL query by id: poll a running query or read the next page of results. Use after panther_query_data_lake or panther_list_data_lake_queries.",
		access: mcp.AccessRead,
		params: []param{
			str("query_id", "", "Query id from panther_query_data_lake.").req(),
			str("cursor", "", "next cursor from the previous page of rows; omit for the first page."),
			integer("page_size", "", fmt.Sprintf("Rows per page, 1 to %d (default %d).", maxResultRows, defaultResultRows)),
		},
		run: getDataLakeQuery,
	},
	{
		name:   "panther_list_data_lake_queries",
		desc:   "List previously run Panther data lake SQL queries (search history), including scheduled and running queries, with SQL, status, and who issued them.",
		access: mcp.AccessRead,
		params: append([]param{
			str("contains", "", "Only queries whose name or SQL contains this text."),
			strs("status", "", "Statuses to include: running, succeeded, failed, cancelled."),
			boolean("is_scheduled", "", "true for scheduled queries only, false for ad-hoc queries only."),
			str("started_after", "", "Only queries started after this ISO-8601 time."),
			str("started_before", "", "Only queries started before this ISO-8601 time."),
		}, pageParams()...),
		run: listDataLakeQueries,
	},
	{
		name:   "panther_cancel_data_lake_query",
		desc:   "Cancel (stop) a running Panther data lake SQL query. Use after panther_query_data_lake times out or panther_list_data_lake_queries shows a runaway query.",
		access: mcp.AccessWrite,
		params: []param{str("query_id", "", "Id of the running query.").req()},
		run:    cancelDataLakeQuery,
	},
}

// connection is a GraphQL connection whose nodes stay raw, so records are
// parsed once, by the response pipeline.
type connection struct {
	Edges []struct {
		Node json.RawMessage `json:"node"`
	} `json:"edges"`
	PageInfo struct {
		HasNextPage bool    `json:"hasNextPage"`
		EndCursor   *string `json:"endCursor"`
	} `json:"pageInfo"`
}

// listResult reshapes a connection like a REST list: {results, next}.
func (c connection) listResult() map[string]any {
	results := make([]json.RawMessage, 0, len(c.Edges))
	for _, edge := range c.Edges {
		results = append(results, edge.Node)
	}
	out := map[string]any{"results": results, "next": nil}
	if c.PageInfo.HasNextPage && c.PageInfo.EndCursor != nil {
		out["next"] = *c.PageInfo.EndCursor
	}
	return out
}

func listDatabases(ctx context.Context, c *client, _ map[string]any) (*mcp.ToolResult, error) {
	var data struct {
		Databases []json.RawMessage `json:"dataLakeDatabases"`
	}
	if err := c.graphql(ctx, "ListDatabases", `query ListDatabases { dataLakeDatabases { name description } }`, nil, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"results": data.Databases})
}

func requiredStr(args map[string]any, name string) (string, error) {
	v, err := mcp.ArgStr(args, name)
	if err != nil {
		return "", err
	}
	if v = strings.TrimSpace(v); v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

func optionalCursor(args map[string]any) (any, error) {
	cursor, err := mcp.ArgStr(args, "cursor")
	if err != nil {
		return nil, err
	}
	if cursor = strings.TrimSpace(cursor); cursor == "" {
		return nil, nil
	}
	return cursor, nil
}

func listDatabaseTables(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	database, err := requiredStr(args, "database_name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	cursor, err := optionalCursor(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	limit, err := pageLimit(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var data struct {
		Tables connection `json:"dataLakeDatabaseTables"`
	}
	const query = `query ListTables($input: DataLakeDatabaseTablesInput!) {
  dataLakeDatabaseTables(input: $input) { edges { node { name displayName description logType } } pageInfo { hasNextPage endCursor } }
}`
	input := map[string]any{"databaseName": database, "pageSize": limit, "cursor": cursor}
	if err := c.graphql(ctx, "ListTables", query, map[string]any{"input": input}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(data.Tables.listResult())
}

func getTableSchema(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	database, err := requiredStr(args, "database_name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	table, err := requiredStr(args, "table_name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	var data struct {
		Table json.RawMessage `json:"dataLakeDatabaseTable"`
	}
	const query = `query GetTable($input: DataLakeDatabaseTableInput!) {
  dataLakeDatabaseTable(input: $input) { name displayName description logType columns { name type description } }
}`
	if err := c.graphql(ctx, "GetTable", query, map[string]any{"input": map[string]any{"databaseName": database, "tableName": table}}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data.Table)
}

func boundedInt(args map[string]any, name string, def, lo, hi int) (int, error) {
	v, err := mcp.ArgInt(args, name)
	if err != nil {
		return 0, err
	}
	if raw, set := args[name]; !set || raw == nil {
		return def, nil
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("%s must be between %d and %d", name, lo, hi)
	}
	return v, nil
}

const dataLakeQueryFields = `id status message sql startedAt completedAt
    results(input: $results) {
      edges { node }
      pageInfo { hasNextPage endCursor }
      columnInfo { order types }
      stats { bytesScanned executionTime rowCount }
      warningMessage
    }`

type queryStatus struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Message     string  `json:"message"`
	SQL         string  `json:"sql"`
	StartedAt   string  `json:"startedAt"`
	CompletedAt *string `json:"completedAt"`
	Results     *struct {
		Edges []struct {
			Node json.RawMessage `json:"node"`
		} `json:"edges"`
		PageInfo struct {
			HasNextPage bool    `json:"hasNextPage"`
			EndCursor   *string `json:"endCursor"`
		} `json:"pageInfo"`
		ColumnInfo struct {
			Order []string        `json:"order"`
			Types json.RawMessage `json:"types"`
		} `json:"columnInfo"`
		Stats          json.RawMessage `json:"stats"`
		WarningMessage *string         `json:"warningMessage"`
	} `json:"results"`
}

// output flattens a query and its result page; rows stay raw.
func (q queryStatus) output() map[string]any {
	out := map[string]any{
		"queryId":     q.ID,
		"status":      q.Status,
		"message":     q.Message,
		"sql":         q.SQL,
		"startedAt":   q.StartedAt,
		"completedAt": q.CompletedAt,
	}
	switch q.Status {
	case "running":
		out["message"] = "The query is still running in Panther. Call panther_get_data_lake_query with this queryId to wait for results, or panther_cancel_data_lake_query to stop it."
	case "succeeded":
		if q.Results == nil {
			break
		}
		rows := make([]json.RawMessage, 0, len(q.Results.Edges))
		for _, edge := range q.Results.Edges {
			rows = append(rows, edge.Node)
		}
		out["rows"] = rows
		out["columns"] = q.Results.ColumnInfo.Order
		out["columnTypes"] = q.Results.ColumnInfo.Types
		out["stats"] = q.Results.Stats
		out["next"] = nil
		if q.Results.PageInfo.HasNextPage && q.Results.PageInfo.EndCursor != nil {
			out["next"] = *q.Results.PageInfo.EndCursor
		}
		if q.Results.WarningMessage != nil && *q.Results.WarningMessage != "" {
			out["warningMessage"] = *q.Results.WarningMessage
		}
	}
	return out
}

func (c *client) fetchQuery(ctx context.Context, id string, pageSize int, cursor any) (queryStatus, error) {
	var data struct {
		Query queryStatus `json:"dataLakeQuery"`
	}
	query := `query GetQuery($id: ID!, $results: DataLakeQueryResultsInput) {
  dataLakeQuery(id: $id) { ` + dataLakeQueryFields + ` }
}`
	vars := map[string]any{"id": id, "results": map[string]any{"pageSize": pageSize, "cursor": cursor}}
	err := c.graphql(ctx, "GetQuery", query, vars, &data)
	return data.Query, err
}

func queryDataLake(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	sql, err := requiredStr(args, "sql")
	if err != nil {
		return mcp.ErrResult(err)
	}
	database, err := mcp.ArgStr(args, "database_name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	timeoutSeconds, err := boundedInt(args, "timeout_seconds", int(defaultQueryTimeout/time.Second), 1, int(maxQueryTimeout/time.Second))
	if err != nil {
		return mcp.ErrResult(err)
	}
	pageSize, err := boundedInt(args, "page_size", defaultResultRows, 1, maxResultRows)
	if err != nil {
		return mcp.ErrResult(err)
	}
	input := map[string]any{"sql": sql}
	if database = strings.TrimSpace(database); database != "" {
		input["databaseName"] = database
	}
	var started struct {
		Execute struct {
			ID string `json:"id"`
		} `json:"executeDataLakeQuery"`
	}
	const mutation = `mutation ExecuteQuery($input: ExecuteDataLakeQueryInput!) { executeDataLakeQuery(input: $input) { id } }`
	if err := c.graphql(ctx, "ExecuteQuery", mutation, map[string]any{"input": input}, &started); err != nil {
		return mcp.ErrResult(err)
	}
	if started.Execute.ID == "" {
		return mcp.ErrResult(fmt.Errorf("panther: the data lake did not return a query id"))
	}
	id := started.Execute.ID
	// Status checks share the wait deadline, so a stalled request cannot
	// hold the caller past timeout_seconds; the query keeps running.
	pollCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	stillRunning := queryStatus{ID: id, Status: "running", SQL: sql}
	wait := c.pollInterval
	for {
		status, err := c.fetchQuery(pollCtx, id, pageSize, nil)
		switch {
		case err != nil && ctx.Err() != nil:
			return mcp.ErrResult(fmt.Errorf("panther: stopped waiting for query %s: %w", id, ctx.Err()))
		case err != nil && pollCtx.Err() != nil:
			return queryResult(stillRunning)
		case err != nil:
			return mcp.ErrResult(fmt.Errorf("%w (query id %s)", err, id))
		}
		if status.Status != "running" {
			return queryResult(status)
		}
		select {
		case <-ctx.Done():
			return mcp.ErrResult(fmt.Errorf("panther: stopped waiting for query %s: %w", id, ctx.Err()))
		case <-pollCtx.Done():
			return queryResult(status)
		case <-time.After(wait):
		}
		wait = min(wait*2, c.maxPollInterval)
	}
}

// queryResult reports a failed or cancelled query as a tool error, keeping
// Panther's message, which usually names the SQL problem.
func queryResult(status queryStatus) (*mcp.ToolResult, error) {
	switch status.Status {
	case "failed", "cancelled":
		return mcp.ErrResult(fmt.Errorf("panther data lake query %s %s: %s", status.ID, status.Status, status.Message))
	}
	return mcp.JSONResult(status.output())
}

func getDataLakeQuery(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	id, err := requiredStr(args, "query_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	cursor, err := optionalCursor(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	pageSize, err := boundedInt(args, "page_size", defaultResultRows, 1, maxResultRows)
	if err != nil {
		return mcp.ErrResult(err)
	}
	status, err := c.fetchQuery(ctx, id, pageSize, cursor)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return queryResult(status)
}

func listDataLakeQueries(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	contains := strings.TrimSpace(r.Str("contains"))
	statuses := r.StrSlice("status")
	after := strings.TrimSpace(r.Str("started_after"))
	before := strings.TrimSpace(r.Str("started_before"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	cursor, err := optionalCursor(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	limit, err := pageLimit(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	input := map[string]any{"pageSize": limit, "cursor": cursor}
	if contains != "" {
		input["contains"] = contains
	}
	if len(statuses) > 0 {
		lowered := make([]string, len(statuses))
		for i, s := range statuses {
			lowered[i] = strings.ToLower(strings.TrimSpace(s))
		}
		input["status"] = lowered
	}
	if raw, ok := args["is_scheduled"]; ok && raw != nil {
		scheduled, err := mcp.ArgBool(args, "is_scheduled")
		if err != nil {
			return mcp.ErrResult(err)
		}
		input["isScheduled"] = scheduled
	}
	if after != "" {
		input["startedAtAfter"] = after
	}
	if before != "" {
		input["startedAtBefore"] = before
	}
	const query = `query ListQueries($input: DataLakeQueriesInput) {
  dataLakeQueries(input: $input) {
    edges { node { id name sql status message startedAt completedAt isScheduled
      issuedBy { ... on User { id email givenName familyName } ... on APIToken { id name } } } }
    pageInfo { hasNextPage endCursor }
  }
}`
	var data struct {
		Queries connection `json:"dataLakeQueries"`
	}
	if err := c.graphql(ctx, "ListQueries", query, map[string]any{"input": input}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(data.Queries.listResult())
}

func cancelDataLakeQuery(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	id, err := requiredStr(args, "query_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	var data struct {
		Cancel struct {
			ID string `json:"id"`
		} `json:"cancelDataLakeQuery"`
	}
	const mutation = `mutation CancelQuery($input: CancelDataLakeQueryInput!) { cancelDataLakeQuery(input: $input) { id } }`
	if err := c.graphql(ctx, "CancelQuery", mutation, map[string]any{"input": map[string]any{"id": id}}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"cancelled": data.Cancel.ID})
}
