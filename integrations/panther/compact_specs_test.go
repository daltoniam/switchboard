package panther

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// rawReadTools return records that are already small or are the content the
// caller asked for, so a spec would only hide data.
var rawReadTools = map[mcp.ToolName]string{
	"panther_list_databases":      "names and descriptions only",
	"panther_get_table_schema":    "the column list is the content",
	"panther_get_log_source":      "the GraphQL query selects only routing fields",
	"panther_get_data_model":      "field mappings are the content",
	"panther_get_global_helper":   "the Python body is the content",
	"panther_get_scheduled_query": "SQL and schedule are the content",
	"panther_get_user":            "a handful of fields",
	"panther_get_role":            "permissions are the content",
	"panther_get_permissions":     "the handler already keeps only identity and permissions",
}

// handlerOutputs are representative handler outputs (Panther records as
// documented in the REST OpenAPI spec and GraphQL schema, in the shapes the
// handlers emit) for every tool with a spec.
var handlerOutputs = map[mcp.ToolName]string{
	"panther_list_alerts": `{"results":[{"id":"a1","title":"Root login","severity":"CRITICAL","status":"OPEN","type":"ALERT","detection":{"id":"AWS.Root.Login","type":"RULE"},"eventCount":3,
		"createdAt":"2026-01-01T00:00:00Z","firstEventOccurredAt":"2026-01-01T00:00:00Z","lastReceivedEventAt":"2026-01-01T01:00:00Z","deliveries":[{"outputId":"slack","success":true}],"runbook":"Page oncall"}],"next":"n2"}`,
	"panther_get_alert":           `{"id":"a1","title":"Root login","deliveries":[{"outputId":"slack","success":true,"statusCode":200}],"runbook":"Page oncall"}`,
	"panther_list_alert_events":   `{"results":[{"p_event_time":"2026-01-01T00:00:00Z","eventName":"ConsoleLogin"}],"next":null}`,
	"panther_list_alert_comments": `{"results":[{"id":"c1","alertId":"a1","body":"Benign","format":"PLAIN_TEXT","createdAt":"2026-01-01T00:00:00Z","createdBy":{"id":"u1","type":"user"},"updatedBy":{"id":"u1"}}],"next":null}`,
	"panther_list_rules": `{"results":[{"id":"AWS.Root.Login","displayName":"Root login","enabled":true,"severity":"HIGH","logTypes":["AWS.CloudTrail"],"tags":["MITRE"],"managed":true,"lastModified":"2026-01-01T00:00:00Z",
		"body":"def rule(event): return True","tests":[{"name":"t","resource":"{}"}],"description":"Root user logged in"}],"next":null}`,
	"panther_get_rule":               `{"id":"AWS.Root.Login","body":"def rule(event): return True","tests":[{"name":"t","resource":"{}"}],"runbook":"Page oncall"}`,
	"panther_list_scheduled_rules":   `{"results":[{"id":"S1","displayName":"Brute force","enabled":true,"severity":"MEDIUM","scheduledQueries":["q1"],"tags":["Initial Access"],"managed":false,"lastModified":"2026-01-01T00:00:00Z","body":"def rule(e): return True"}],"next":null}`,
	"panther_get_scheduled_rule":     `{"id":"S1","body":"def rule(e): return True","tests":[{"name":"t"}]}`,
	"panther_list_simple_rules":      `{"results":[{"id":"Y1","displayName":"Okta MFA","enabled":false,"severity":"LOW","logTypes":["Okta.SystemLog"],"tags":["Initial Access"],"managed":false,"lastModified":"2026-01-01T00:00:00Z","detection":"Key: x"}],"next":null}`,
	"panther_get_simple_rule":        `{"id":"Y1","detection":"Key: x","tests":[{"name":"t"}]}`,
	"panther_list_policies":          `{"results":[{"id":"P1","displayName":"S3 public","enabled":true,"severity":"HIGH","resourceTypes":["AWS.S3.Bucket"],"tags":["Initial Access"],"managed":true,"lastModified":"2026-01-01T00:00:00Z","body":"def policy(r): return True"}],"next":null}`,
	"panther_get_policy":             `{"id":"P1","body":"def policy(r): return True","tests":[{"name":"t"}]}`,
	"panther_list_correlation_rules": `{"results":[{"id":"C1","displayName":"Login then exfil","enabled":true,"severity":"CRITICAL","tags":["Initial Access"],"managed":false,"lastModified":"2026-01-01T00:00:00Z","detection":"Sequence: []"}],"next":null}`,
	"panther_get_correlation_rule":   `{"id":"C1","detection":"Sequence: []","tests":[{"name":"t"}]}`,
	"panther_list_database_tables":   `{"results":[{"name":"aws_cloudtrail","displayName":"AWS CloudTrail","description":"CloudTrail logs","logType":"AWS.CloudTrail"}],"next":"t2"}`,
	"panther_query_data_lake":        `{"queryId":"q1","status":"succeeded","message":"ok","sql":"select 1","startedAt":"t0","completedAt":"t1","rows":[{"n":1}],"columns":["n"],"columnTypes":{"n":"number"},"stats":{"rowCount":1},"next":null}`,
	"panther_get_data_lake_query":    `{"queryId":"q1","status":"succeeded","message":"ok","sql":"select 1","startedAt":"t0","completedAt":"t1","rows":[{"n":1}],"columns":["n"],"columnTypes":{"n":"number"},"stats":{"rowCount":1},"next":"p2"}`,
	"panther_list_data_lake_queries": `{"results":[{"id":"q1","name":null,"sql":"select 1","status":"succeeded","message":"ok","startedAt":"t0","completedAt":"t1","isScheduled":false,"issuedBy":{"id":"u1","email":"a@example.com"}}],"next":null}`,
	"panther_list_log_sources":       `{"results":[{"integrationId":"s1","integrationLabel":"CloudTrail","integrationType":"aws-s3","isEditable":true,"isHealthy":true,"logTypes":["AWS.CloudTrail"],"lastEventReceivedAtTime":"t1","lastEventProcessedAtTime":"t2","lastModified":"t0","createdAtTime":"t0"}],"next":null}`,
	"panther_list_schemas":           `{"results":[{"name":"AWS.CloudTrail","description":"CloudTrail","revision":3,"isArchived":false,"isManaged":true,"referenceURL":"https://docs","createdAt":"t0","updatedAt":"t1"}],"next":null}`,
	"panther_get_schema":             `{"name":"Custom.App","spec":"fields: []","discoveredSpec":"fields: [{name: x}]","version":2}`,
	"panther_list_data_models":       `{"results":[{"id":"Standard.AWS.CloudTrail","displayName":"CloudTrail","enabled":true,"logTypes":["AWS.CloudTrail"],"lastModified":"t1","mappings":[{"name":"actor_user","path":"$.userIdentity.arn"}],"body":""}],"next":null}`,
	"panther_list_global_helpers":    `{"results":[{"id":"panther_base_helpers","description":"Shared helpers","tags":["Initial Access"],"lastModified":"t1","body":"def deep_get(): pass"}],"next":null}`,
	"panther_list_scheduled_queries": `{"results":[{"id":"q1","name":"Hourly logins","sql":"select 1","schedule":{"cron":"0 * * * *","disabled":false,"timeoutMinutes":5},"updatedAt":"t1","createdAt":"t0","managed":false,"emailConfig":{"recipients":["a@example.com"]}}],"next":null}`,
	"panther_list_users":             `{"results":[{"id":"u1","email":"a@example.com","givenName":"Ada","familyName":"Lovelace","role":{"id":"r1","name":"Admin"},"status":"CONFIRMED","enabled":true,"lastLoggedInAt":"t1","createdAt":"t0"}],"next":null}`,
	"panther_list_roles":             `{"results":[{"id":"r1","name":"Analyst","permissions":["AlertRead"],"logTypeAccessKind":"ALLOW_ALL","logTypeAccess":[],"createdAt":"t0"}],"next":null}`,
	"panther_get_alert_metrics":      `{"fromDate":"2026-01-01T00:00:00Z","toDate":"2026-01-08T00:00:00Z","metrics":{"totalAlerts":5,"alertsPerSeverity":[{"label":"HIGH","value":5,"breakdown":{"2026-01-01T00:00:00Z":5}}],"alertsPerRule":[{"entityId":"AWS.Root.Login","label":"Root login","value":5}]}}`,
	"panther_get_data_metrics": `{"fromDate":"2026-01-01T00:00:00Z","toDate":"2026-01-08T00:00:00Z","metrics":{"totalEventsProcessed":100,"totalBytesProcessed":2048,"totalBytesQueried":10,
		"eventsProcessedPerLogType":[{"label":"AWS.CloudTrail","value":100,"breakdown":{"t":100}}],"bytesProcessedPerSource":[{"label":"CloudTrail","value":2048,"breakdown":{"t":2048}}],"bytesQueriedPerSource":[{"label":"CloudTrail","value":10,"breakdown":{"t":10}}]}}`,
}

func TestFieldCompactionSpecs_Parse(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	require.NotEmpty(t, fieldCompactionSpecs)
	assert.Len(t, fieldCompactionSpecs, len(sf.Tools), "the loader dropped an entry")
	_, err := compact.Load(compactYAML, compact.Options{Strict: true})
	require.NoError(t, err)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec for %s has no tool", name)
	}
}

func TestFieldCompactionSpecs_ReadToolsOnly(t *testing.T) {
	for _, tool := range nativeTools {
		_, hasSpec := fieldCompactionSpecs[tool.Name]
		_, raw := rawReadTools[tool.Name]
		switch {
		case tool.Access != mcp.AccessRead:
			assert.False(t, hasSpec, "mutation tool %s must return its full confirmation", tool.Name)
		case raw:
			assert.False(t, hasSpec, "%s is listed as raw but has a spec", tool.Name)
		default:
			assert.True(t, hasSpec, "read tool %s has no compaction spec", tool.Name)
		}
	}
	for name := range rawReadTools {
		tool, ok := findTool(nativeTools, name)
		require.True(t, ok, "raw tool %s does not exist", name)
		assert.Equal(t, mcp.AccessRead, tool.Access)
	}
}

// Inside an array, a nested path keeps only its leaf name, so a nested path
// would overwrite a record's own field of the same name.
func TestFieldCompactionSpecs_NoNestedPathsInArrays(t *testing.T) {
	for name, fields := range fieldCompactionSpecs {
		for _, field := range fields {
			if _, rest, ok := strings.Cut(field.Spec(), "[]."); ok {
				assert.NotContains(t, rest, ".", "%s: %q projects a nested path inside an array", name, field.Spec())
			}
		}
	}
}

func TestMaxBytes_NoCapConfigured(t *testing.T) {
	p, _ := newNativeAdapter(t, func(_ http.ResponseWriter, _ recorded) {})
	for name := range fieldCompactionSpecs {
		_, ok := p.MaxBytes(name)
		assert.False(t, ok, "%s declares a max_bytes cap; update this test deliberately", name)
	}
}

// Every positive spec field must match something in the handler output,
// the same check the live acceptance run makes against real responses.
func TestFieldCompactionSpecs_MatchHandlerOutputs(t *testing.T) {
	for name, fields := range fieldCompactionSpecs {
		t.Run(string(name), func(t *testing.T) {
			output, ok := handlerOutputs[name]
			require.True(t, ok, "add a representative handler output for %s", name)
			var raw any
			require.NoError(t, json.Unmarshal([]byte(output), &raw))
			for _, field := range fields {
				if field.Exclusion() {
					continue
				}
				assert.True(t, fieldPresent(raw, field), "%q matches nothing in the %s output", field.Spec(), name)
			}
		})
	}
}

func fieldPresent(raw any, field mcp.CompactField) bool {
	source, _, _ := strings.Cut(field.Spec(), ":")
	if !strings.ContainsAny(source, ".[*") {
		object, ok := raw.(map[string]any)
		if !ok {
			return false
		}
		_, found := object[source]
		return found
	}
	return !emptyValue(mcp.CompactAny(raw, []mcp.CompactField{field}))
}

// emptyValue reports whether compaction kept no values, only empty
// containers.
func emptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case map[string]any:
		for _, item := range typed {
			if !emptyValue(item) {
				return false
			}
		}
		return true
	case []any:
		for _, item := range typed {
			if !emptyValue(item) {
				return false
			}
		}
		return true
	}
	return false
}

func TestCompaction_Shape(t *testing.T) {
	tests := []struct {
		tool mcp.ToolName
		keep []string
		drop []string
	}{
		{tool: "panther_list_alerts", keep: []string{`"id":"a1"`, "CRITICAL", "OPEN", `"detection":{"id":"AWS.Root.Login","type":"RULE"}`, `"next":"n2"`}, drop: []string{"deliveries", "Page oncall"}},
		{tool: "panther_get_alert", keep: []string{"Root login", "Page oncall"}, drop: []string{"deliveries", "statusCode"}},
		{tool: "panther_list_rules", keep: []string{"AWS.Root.Login", "AWS.CloudTrail", "MITRE"}, drop: []string{"def rule", "tests"}},
		{tool: "panther_get_rule", keep: []string{"def rule", "Page oncall"}, drop: []string{"tests"}},
		{tool: "panther_query_data_lake", keep: []string{`"rows":[{"n":1}]`, `"columns":["n"]`, `"queryId":"q1"`}, drop: []string{"columnTypes", "select 1"}},
		{tool: "panther_list_scheduled_queries", keep: []string{"Hourly logins", "0 * * * *"}, drop: []string{"recipients"}},
		{tool: "panther_list_users", keep: []string{"a@example.com", `"role":{"id":"r1","name":"Admin"}`}, drop: []string{"createdAt"}},
		{tool: "panther_get_alert_metrics", keep: []string{`"totalAlerts":5`, `"entityId":"AWS.Root.Login"`, `"label":"HIGH"`}, drop: []string{"breakdown"}},
		{tool: "panther_get_data_metrics", keep: []string{`"totalEventsProcessed":100`, "AWS.CloudTrail"}, drop: []string{"breakdown"}},
		{tool: "panther_get_schema", keep: []string{"fields: []"}, drop: []string{"discoveredSpec"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			var raw any
			require.NoError(t, json.Unmarshal([]byte(handlerOutputs[tt.tool]), &raw))
			data, err := json.Marshal(mcp.CompactAny(raw, fieldCompactionSpecs[tt.tool]))
			require.NoError(t, err)
			for _, want := range tt.keep {
				assert.Contains(t, string(data), want)
			}
			for _, unwanted := range tt.drop {
				assert.NotContains(t, string(data), unwanted)
			}
		})
	}
}
