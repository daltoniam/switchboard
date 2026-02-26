package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	mcp "github.com/daltoniam/switchboard"
)

type dd struct {
	apiKey string
	appKey string
	site   string
	client *datadog.APIClient
}

func New() mcp.Integration {
	return &dd{}
}

func (d *dd) Name() string { return "datadog" }

func (d *dd) Configure(creds mcp.Credentials) error {
	d.apiKey = creds["api_key"]
	d.appKey = creds["app_key"]
	if d.apiKey == "" || d.appKey == "" {
		return fmt.Errorf("datadog: api_key and app_key are required")
	}
	d.site = creds["site"]

	cfg := datadog.NewConfiguration()
	cfg.SetUnstableOperationEnabled("v2.ListIncidents", true)
	cfg.SetUnstableOperationEnabled("v2.GetIncident", true)
	cfg.SetUnstableOperationEnabled("v2.CreateIncident", true)
	cfg.SetUnstableOperationEnabled("v2.UpdateIncident", true)
	d.client = datadog.NewAPIClient(cfg)
	return nil
}

// ctx returns a context with Datadog API keys and site injected.
func (d *dd) ctx(parent context.Context) context.Context {
	ctx := context.WithValue(parent, datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {Key: d.apiKey},
		"appKeyAuth": {Key: d.appKey},
	})
	if d.site != "" {
		ctx = context.WithValue(ctx, datadog.ContextServerVariables, map[string]string{
			"site": d.site,
		})
	}
	return ctx
}

func (d *dd) Healthy(ctx context.Context) bool {
	api := datadogV1.NewAuthenticationApi(d.client)
	_, _, err := api.Validate(d.ctx(ctx))
	return err == nil
}

func (d *dd) Tools() []mcp.ToolDefinition {
	return tools
}

func (d *dd) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(d.ctx(ctx), d, args)
}

// --- helpers ---

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errResult(err)
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argInt64(args map[string]any, key string) int64 {
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return nil
}

func optInt(args map[string]any, key string, def int) int {
	if v := argInt(args, key); v > 0 {
		return v
	}
	return def
}

func optInt64(args map[string]any, key string, def int64) int64 {
	if v := argInt64(args, key); v > 0 {
		return v
	}
	return def
}

// parseTime parses a time string like "now-1h", epoch seconds, or ISO 8601.
func parseTime(s string, fallback time.Duration) time.Time {
	if s == "" {
		return time.Now().Add(fallback)
	}
	if s == "now" {
		return time.Now()
	}
	if strings.HasPrefix(s, "now-") {
		dur, err := time.ParseDuration(s[4:])
		if err == nil {
			return time.Now().Add(-dur)
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0)
	}
	return time.Now().Add(fallback)
}

type handlerFunc func(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	// Logs
	"datadog_search_logs":    searchLogs,
	"datadog_aggregate_logs": aggregateLogs,

	// Metrics
	"datadog_query_metrics":      queryMetrics,
	"datadog_list_active_metrics": listActiveMetrics,
	"datadog_search_metrics":     searchMetrics,
	"datadog_get_metric_metadata": getMetricMetadata,

	// Monitors
	"datadog_list_monitors":   listMonitors,
	"datadog_search_monitors": searchMonitors,
	"datadog_get_monitor":     getMonitor,
	"datadog_create_monitor":  createMonitor,
	"datadog_update_monitor":  updateMonitor,
	"datadog_delete_monitor":  deleteMonitor,
	"datadog_mute_monitor":    muteMonitor,

	// Dashboards
	"datadog_list_dashboards":  listDashboards,
	"datadog_get_dashboard":    getDashboard,
	"datadog_create_dashboard": createDashboard,
	"datadog_delete_dashboard": deleteDashboard,

	// Events
	"datadog_list_events":   listEvents,
	"datadog_search_events": searchEvents,
	"datadog_get_event":     getEvent,
	"datadog_create_event":  createEvent,

	// Hosts
	"datadog_list_hosts":      listHosts,
	"datadog_get_host_totals": getHostTotals,
	"datadog_mute_host":       muteHost,
	"datadog_unmute_host":     unmuteHost,

	// Tags
	"datadog_list_tags":        listTags,
	"datadog_get_host_tags":    getHostTags,
	"datadog_create_host_tags": createHostTags,
	"datadog_update_host_tags": updateHostTags,
	"datadog_delete_host_tags": deleteHostTags,

	// SLOs
	"datadog_list_slos":       listSLOs,
	"datadog_search_slos":     searchSLOs,
	"datadog_get_slo":         getSLO,
	"datadog_get_slo_history": getSLOHistory,
	"datadog_create_slo":      createSLO,
	"datadog_delete_slo":      deleteSLO,

	// Downtimes
	"datadog_list_downtimes":  listDowntimes,
	"datadog_get_downtime":    getDowntime,
	"datadog_create_downtime": createDowntime,
	"datadog_cancel_downtime": cancelDowntime,

	// Incidents
	"datadog_list_incidents":  listIncidents,
	"datadog_get_incident":    getIncident,
	"datadog_create_incident": createIncident,
	"datadog_update_incident": updateIncident,

	// Synthetics
	"datadog_list_synthetics_tests":      listSyntheticsTests,
	"datadog_get_synthetics_api_test":    getSyntheticsAPITest,
	"datadog_get_synthetics_test_result": getSyntheticsTestResult,
	"datadog_trigger_synthetics_tests":   triggerSyntheticsTests,

	// Notebooks
	"datadog_list_notebooks":  listNotebooks,
	"datadog_get_notebook":    getNotebook,
	"datadog_create_notebook": createNotebook,
	"datadog_delete_notebook": deleteNotebook,

	// Users
	"datadog_list_users": listUsers,
	"datadog_get_user":   getUser,

	// Spans / APM
	"datadog_search_spans": searchSpans,

	// Service Definition / Software Catalog
	"datadog_list_services": listServices,

	// IP Ranges
	"datadog_get_ip_ranges": getIPRanges,
}
