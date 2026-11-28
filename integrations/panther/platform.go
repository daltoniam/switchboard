package panther

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const defaultMetricsWindow = 7 * 24 * time.Hour

// roleFilters are the only /roles arguments; roles are not paginated.
var roleFilters = []param{
	str("name_contains", "name-contains", "Only roles whose name contains this text."),
	str("name", "name", "Only the role with this exact name."),
}

var platformSpecs = []spec{
	{
		name:   "panther_list_log_sources",
		desc:   "List Panther log sources (data ingestion integrations such as S3, HTTP, SQS, and SaaS pullers): id, label, type, health, log types, and last event received and processed times. Start here to check log ingestion health and which logs are onboarded.",
		access: mcp.AccessRead,
		params: []param{str("cursor", "", "Opaque next cursor from the previous page; omit for the first page.")},
		run:    listLogSources,
	},
	{
		name:   "panther_get_log_source",
		desc:   "Get one Panther log source by id: type, health, log types, last event times, and for S3 sources the bucket, prefix, and role. Use after panther_list_log_sources.",
		access: mcp.AccessRead,
		params: []param{str("source_id", "", "Log source (integration) id from panther_list_log_sources.").req()},
		run:    getLogSource,
	},
	{
		name:   "panther_list_schemas",
		desc:   "List Panther log type schemas (log schema definitions for parsing ingested logs): name, description, revision, archived and managed flags. Use to find log types before writing detections or SQL.",
		access: mcp.AccessRead,
		params: []param{
			str("contains", "", "Only schemas whose name or field names contain this text."),
			boolean("is_archived", "", "true for archived schemas only, false for active only."),
			boolean("is_in_use", "", "true for schemas a log source uses, false for unused."),
			boolean("is_managed", "", "true for Panther-managed (pack) schemas, false for custom schemas."),
		},
		run: listSchemas,
	},
	{
		name:   "panther_get_schema",
		desc:   "Get a Panther log type schema in full by exact name: YAML spec, discovered spec, version, and field discovery setting. Use after panther_list_schemas.",
		access: mcp.AccessRead,
		params: []param{str("name", "", "Exact schema (log type) name, such as AWS.CloudTrail or Custom.MyApp.").req()},
		run:    getSchema,
	},
	restList("panther_list_data_models", "List Panther data models (field mappings that normalize log types for detections, such as udm or p_udm fields): id, name, enabled state, and log types.", "/data-models"),
	restGet("panther_get_data_model", "Get a Panther data model in full: field mappings, Python body, and log types. Use after panther_list_data_models.", "/data-models", "data_model_id", "Data model id."),
	restList("panther_list_global_helpers", "List Panther global helpers (shared Python modules imported by rules and policies): id, description, and tags.", "/globals",
		str("name_contains", "name-contains", "Only globals whose name contains this text."),
		str("created_by", "created-by", "Only globals created by this user or API token id."),
		str("last_modified_by", "last-modified-by", "Only globals last modified by this user or API token id."),
	),
	restGet("panther_get_global_helper", "Get a Panther global helper in full, including its Python body. Use after panther_list_global_helpers.", "/globals", "global_id", "Global helper id."),
	restList("panther_list_scheduled_queries", "List Panther scheduled queries (saved SQL searches that run on a cron or interval, often feeding scheduled rules): id, name, SQL, schedule, and email settings.", "/queries"),
	restGet("panther_get_scheduled_query", "Get a Panther saved or scheduled query in full: SQL, schedule (cron or rate, timeout, disabled), and description. Use after panther_list_scheduled_queries.", "/queries", "query_id", "Saved query id."),
	restList("panther_list_users", "List Panther users (people with Console access): id, email, name, role, status, and last login. Use to find a user id for panther_assign_alerts.", "/users",
		str("contains", "contains", "Only users whose name or email contains this text."),
		str("email", "email", "Only the user with this exact email."),
		str("status", "status", "Only users with this status, such as CONFIRMED or FORCE_CHANGE_PASSWORD."),
		boolean("include_deactivated", "include-deactivated", "true to include deactivated users."),
	),
	restGet("panther_get_user", "Get one Panther user by id: email, name, role, status, and last login. Use after panther_list_users.", "/users", "user_id", "User id."),
	{
		name:   "panther_list_roles",
		desc:   "List Panther roles (RBAC permission sets assigned to users and API tokens): id, name, permissions, and log type access.",
		access: mcp.AccessRead,
		params: roleFilters,
		run:    listRoles,
	},
	restGet("panther_get_role", "Get one Panther role by id: permissions and log type access. Use after panther_list_roles.", "/roles", "role_id", "Role id."),
	{
		name:   "panther_get_permissions",
		desc:   "Get the permissions of the Panther API token Switchboard uses (name, permissions, expiry, allowed CIDR blocks). Use to diagnose permission denied errors before asking for a new token.",
		access: mcp.AccessRead,
		run:    getPermissions,
	},
	{
		name:   "panther_get_alert_metrics",
		desc:   "Get Panther alert metrics for a time range: total alerts, alert counts per severity, and the rules that alerted most, for security reporting and noisy detection tuning. Defaults to the last 7 days.",
		access: mcp.AccessRead,
		params: metricsParams(),
		run:    getAlertMetrics,
	},
	{
		name:   "panther_get_data_metrics",
		desc:   "Get Panther data ingestion metrics for a time range: total events and bytes processed, events processed per log type, and bytes processed and queried per log source, for ingestion volume and cost monitoring. Defaults to the last 7 days.",
		access: mcp.AccessRead,
		params: metricsParams(),
		run:    getDataMetrics,
	},
}

func listLogSources(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	cursor, err := optionalCursor(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	const query = `query ListSources($input: SourcesInput) {
  sources(input: $input) {
    edges { node { integrationId integrationLabel integrationType isEditable isHealthy lastEventProcessedAtTime lastEventReceivedAtTime lastModified createdAtTime logTypes } }
    pageInfo { hasNextPage endCursor }
  }
}`
	var data struct {
		Sources connection `json:"sources"`
	}
	if err := c.graphql(ctx, "ListSources", query, map[string]any{"input": map[string]any{"cursor": cursor}}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(data.Sources.listResult())
}

func getLogSource(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	id, err := requiredStr(args, "source_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	const query = `query GetSource($id: ID!) {
  source(id: $id) {
    integrationId integrationLabel integrationType isEditable isHealthy lastEventProcessedAtTime lastEventReceivedAtTime lastModified createdAtTime logTypes
    ... on S3LogIntegration { awsAccountId s3Bucket s3Prefix kmsKey logProcessingRole logStreamType managedBucketNotifications stackName s3PrefixLogTypes { prefix logTypes excludedPrefixes } }
  }
}`
	var data struct {
		Source json.RawMessage `json:"source"`
	}
	if err := c.graphql(ctx, "GetSource", query, map[string]any{"id": id}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	if len(data.Source) == 0 || string(data.Source) == "null" {
		return mcp.ErrResult(fmt.Errorf("panther: no log source with id %s", id))
	}
	return mcp.RawResult(data.Source)
}

const schemaSummaryFields = `name description revision isArchived isManaged referenceURL createdAt updatedAt`

func listSchemas(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	input := map[string]any{}
	contains, err := mcp.ArgStr(args, "contains")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if contains = strings.TrimSpace(contains); contains != "" {
		input["contains"] = contains
	}
	for arg, field := range map[string]string{"is_archived": "isArchived", "is_in_use": "isInUse", "is_managed": "isManaged"} {
		if raw, ok := args[arg]; ok && raw != nil {
			v, err := mcp.ArgBool(args, arg)
			if err != nil {
				return mcp.ErrResult(err)
			}
			input[field] = v
		}
	}
	query := `query ListSchemas($input: SchemasInput!) { schemas(input: $input) { edges { node { ` + schemaSummaryFields + ` } } } }`
	var data struct {
		Schemas connection `json:"schemas"`
	}
	if err := c.graphql(ctx, "ListSchemas", query, map[string]any{"input": input}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(data.Schemas.listResult())
}

func getSchema(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	name, err := requiredStr(args, "name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	// Panther has no get-by-name query; contains narrows the list and the
	// exact name is picked here.
	query := `query GetSchema($input: SchemasInput!) { schemas(input: $input) { edges { node { ` + schemaSummaryFields + ` spec version isFieldDiscoveryEnabled discoveredSpec } } } }`
	var data struct {
		Schemas connection `json:"schemas"`
	}
	if err := c.graphql(ctx, "GetSchema", query, map[string]any{"input": map[string]any{"contains": name}}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	for _, edge := range data.Schemas.Edges {
		var node struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(edge.Node, &node) == nil && node.Name == name {
			return mcp.RawResult(edge.Node)
		}
	}
	return mcp.ErrResult(fmt.Errorf("panther: no schema named %q; use panther_list_schemas to find the exact name", name))
}

func listRoles(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	q, err := queryValues(roleFilters, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return getList(ctx, c, "/roles", q)
}

// getPermissions reads the token's own record and drops everything but its
// identity and permissions, so no token value can reach a response.
func getPermissions(ctx context.Context, c *client, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.rest(ctx, http.MethodGet, "/api-tokens/self", nil, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var token struct {
		ID                string   `json:"id"`
		Name              string   `json:"name"`
		Permissions       []string `json:"permissions"`
		AllowedCIDRBlocks []string `json:"allowedCIDRBlocks"`
		CreatedAt         string   `json:"createdAt"`
		ExpiresAt         *string  `json:"expiresAt"`
		LastUsedAt        *string  `json:"lastUsedAt"`
	}
	if err := json.Unmarshal(data, &token); err != nil {
		return mcp.ErrResult(fmt.Errorf("panther: unexpected API token response: %w", err))
	}
	return mcp.JSONResult(token)
}

func metricsParams() []param {
	return []param{
		str("from", "", "Start of the range, ISO-8601 (default 7 days before to)."),
		str("to", "", "End of the range, ISO-8601 (default now)."),
		integer("interval_minutes", "", "Bucket size for the per-interval breakdowns; omit to let Panther choose."),
	}
}

func metricsInput(args map[string]any) (map[string]any, error) {
	r := mcp.NewArgs(args)
	from := strings.TrimSpace(r.Str("from"))
	to := strings.TrimSpace(r.Str("to"))
	interval := r.Int("interval_minutes")
	if err := r.Err(); err != nil {
		return nil, err
	}
	end := time.Now().UTC()
	if to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return nil, fmt.Errorf("to must be an ISO-8601 time such as 2026-01-31T00:00:00Z")
		}
		end = parsed
	}
	start := end.Add(-defaultMetricsWindow)
	if from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return nil, fmt.Errorf("from must be an ISO-8601 time such as 2026-01-01T00:00:00Z")
		}
		start = parsed
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("from must be before to")
	}
	input := map[string]any{"fromDate": start.Format(time.RFC3339), "toDate": end.Format(time.RFC3339)}
	if interval > 0 {
		input["intervalInMinutes"] = interval
	}
	return input, nil
}

func getMetrics(ctx context.Context, c *client, args map[string]any, operation, fields string) (*mcp.ToolResult, error) {
	input, err := metricsInput(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	query := `query ` + operation + `($input: MetricsInput!) { metrics(input: $input) { ` + fields + ` } }`
	var data struct {
		Metrics json.RawMessage `json:"metrics"`
	}
	if err := c.graphql(ctx, operation, query, map[string]any{"input": input}, &data); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"fromDate": input["fromDate"], "toDate": input["toDate"], "metrics": data.Metrics})
}

func getAlertMetrics(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	return getMetrics(ctx, c, args, "AlertMetrics", `totalAlerts alertsPerSeverity { label value breakdown } alertsPerRule { entityId label value }`)
}

func getDataMetrics(ctx context.Context, c *client, args map[string]any) (*mcp.ToolResult, error) {
	return getMetrics(ctx, c, args, "DataMetrics", `totalEventsProcessed totalBytesProcessed totalBytesQueried eventsProcessedPerLogType { label value breakdown } bytesProcessedPerSource { label value breakdown } bytesQueriedPerSource { label value breakdown }`)
}
