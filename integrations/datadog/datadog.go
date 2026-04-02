package datadog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*dd)(nil)
	_ mcp.FieldCompactionIntegration = (*dd)(nil)
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

func (d *dd) Configure(_ context.Context, creds mcp.Credentials) error {
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
	cfg.SetUnstableOperationEnabled("v2.SearchIncidents", true)
	cfg.SetUnstableOperationEnabled("v2.ListIncidentAttachments", true)
	cfg.SetUnstableOperationEnabled("v2.ListIncidentTodos", true)
	cfg.SetUnstableOperationEnabled("v2.ListIncidentServices", true)
	cfg.SetUnstableOperationEnabled("v2.GetIncidentService", true)
	cfg.SetUnstableOperationEnabled("v2.CreateIncidentService", true)
	cfg.SetUnstableOperationEnabled("v2.UpdateIncidentService", true)
	cfg.SetUnstableOperationEnabled("v2.DeleteIncidentService", true)
	cfg.SetUnstableOperationEnabled("v2.ListIncidentTeams", true)
	cfg.SetUnstableOperationEnabled("v2.GetIncidentTeam", true)
	cfg.SetUnstableOperationEnabled("v2.CreateIncidentTeam", true)
	cfg.SetUnstableOperationEnabled("v2.UpdateIncidentTeam", true)
	cfg.SetUnstableOperationEnabled("v2.DeleteIncidentTeam", true)
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

func (d *dd) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (d *dd) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(d.ctx(ctx), d, args)
}

// --- helpers ---

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
	"datadog_query_metrics":       queryMetrics,
	"datadog_list_active_metrics": listActiveMetrics,
	"datadog_search_metrics":      searchMetrics,
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
	"datadog_list_incidents":            listIncidents,
	"datadog_search_incidents":          searchIncidents,
	"datadog_get_incident":              getIncident,
	"datadog_create_incident":           createIncident,
	"datadog_update_incident":           updateIncident,
	"datadog_list_incident_attachments": listIncidentAttachments,
	"datadog_list_incident_todos":       listIncidentTodos,

	// Incident Services
	"datadog_list_incident_services":  listIncidentServices,
	"datadog_get_incident_service":    getIncidentService,
	"datadog_create_incident_service": createIncidentService,
	"datadog_update_incident_service": updateIncidentService,
	"datadog_delete_incident_service": deleteIncidentService,

	// Incident Teams
	"datadog_list_incident_teams":  listIncidentTeams,
	"datadog_get_incident_team":    getIncidentTeam,
	"datadog_create_incident_team": createIncidentTeam,
	"datadog_update_incident_team": updateIncidentTeam,
	"datadog_delete_incident_team": deleteIncidentTeam,

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

	// Teams
	"datadog_list_teams":                     listTeams,
	"datadog_get_team":                       getTeam,
	"datadog_create_team":                    createTeam,
	"datadog_update_team":                    updateTeam,
	"datadog_delete_team":                    deleteTeam,
	"datadog_list_team_members":              listTeamMembers,
	"datadog_add_team_member":                addTeamMember,
	"datadog_update_team_member":             updateTeamMember,
	"datadog_remove_team_member":             removeTeamMember,
	"datadog_get_user_team_memberships":      getUserTeamMemberships,
	"datadog_list_team_links":                listTeamLinks,
	"datadog_get_team_link":                  getTeamLink,
	"datadog_create_team_link":               createTeamLink,
	"datadog_update_team_link":               updateTeamLink,
	"datadog_delete_team_link":               deleteTeamLink,
	"datadog_get_team_permission_settings":   getTeamPermissionSettings,
	"datadog_update_team_permission_setting": updateTeamPermissionSetting,

	// Spans / APM
	"datadog_search_spans": searchSpans,

	// Service Definition / Software Catalog
	"datadog_list_services": listServices,

	// On-Call
	"datadog_get_oncall_schedule":             getOnCallSchedule,
	"datadog_create_oncall_schedule":          createOnCallSchedule,
	"datadog_update_oncall_schedule":          updateOnCallSchedule,
	"datadog_delete_oncall_schedule":          deleteOnCallSchedule,
	"datadog_get_schedule_oncall_user":        getScheduleOnCallUser,
	"datadog_get_oncall_escalation_policy":    getOnCallEscalationPolicy,
	"datadog_create_oncall_escalation_policy": createOnCallEscalationPolicy,
	"datadog_update_oncall_escalation_policy": updateOnCallEscalationPolicy,
	"datadog_delete_oncall_escalation_policy": deleteOnCallEscalationPolicy,
	"datadog_get_oncall_team_routing_rules":   getOnCallTeamRoutingRules,
	"datadog_set_oncall_team_routing_rules":   setOnCallTeamRoutingRules,
	"datadog_get_team_oncall_users":           getTeamOnCallUsers,

	// On-Call Paging
	"datadog_create_oncall_page":      createOnCallPage,
	"datadog_acknowledge_oncall_page": acknowledgeOnCallPage,
	"datadog_escalate_oncall_page":    escalateOnCallPage,
	"datadog_resolve_oncall_page":     resolveOnCallPage,

	// IP Ranges
	"datadog_get_ip_ranges": getIPRanges,
}
