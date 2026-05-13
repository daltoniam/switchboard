package datadog

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compactyaml"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compactyaml.MustLoadWithOverlay("datadog", compactYAML, compactyaml.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*dd)(nil)
	_ mcp.FieldCompactionIntegration = (*dd)(nil)
	_ mcp.PlainTextCredentials       = (*dd)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*dd)(nil)
)

func (d *dd) PlainTextKeys() []string {
	return []string{"site"}
}

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

func (d *dd) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (d *dd) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (d *dd) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

var dispatch = map[mcp.ToolName]handlerFunc{
	// Logs
	mcp.ToolName("datadog_search_logs"):    searchLogs,
	mcp.ToolName("datadog_aggregate_logs"): aggregateLogs,

	// Metrics
	mcp.ToolName("datadog_query_metrics"):       queryMetrics,
	mcp.ToolName("datadog_list_active_metrics"): listActiveMetrics,
	mcp.ToolName("datadog_search_metrics"):      searchMetrics,
	mcp.ToolName("datadog_get_metric_metadata"): getMetricMetadata,

	// Monitors
	mcp.ToolName("datadog_list_monitors"):   listMonitors,
	mcp.ToolName("datadog_search_monitors"): searchMonitors,
	mcp.ToolName("datadog_get_monitor"):     getMonitor,
	mcp.ToolName("datadog_create_monitor"):  createMonitor,
	mcp.ToolName("datadog_update_monitor"):  updateMonitor,
	mcp.ToolName("datadog_delete_monitor"):  deleteMonitor,
	mcp.ToolName("datadog_mute_monitor"):    muteMonitor,

	// Dashboards
	mcp.ToolName("datadog_list_dashboards"):  listDashboards,
	mcp.ToolName("datadog_get_dashboard"):    getDashboard,
	mcp.ToolName("datadog_create_dashboard"): createDashboard,
	mcp.ToolName("datadog_delete_dashboard"): deleteDashboard,

	// Events
	mcp.ToolName("datadog_list_events"):   listEvents,
	mcp.ToolName("datadog_search_events"): searchEvents,
	mcp.ToolName("datadog_get_event"):     getEvent,
	mcp.ToolName("datadog_create_event"):  createEvent,

	// Hosts
	mcp.ToolName("datadog_list_hosts"):      listHosts,
	mcp.ToolName("datadog_get_host_totals"): getHostTotals,
	mcp.ToolName("datadog_mute_host"):       muteHost,
	mcp.ToolName("datadog_unmute_host"):     unmuteHost,

	// Tags
	mcp.ToolName("datadog_list_tags"):        listTags,
	mcp.ToolName("datadog_get_host_tags"):    getHostTags,
	mcp.ToolName("datadog_create_host_tags"): createHostTags,
	mcp.ToolName("datadog_update_host_tags"): updateHostTags,
	mcp.ToolName("datadog_delete_host_tags"): deleteHostTags,

	// SLOs
	mcp.ToolName("datadog_list_slos"):       listSLOs,
	mcp.ToolName("datadog_search_slos"):     searchSLOs,
	mcp.ToolName("datadog_get_slo"):         getSLO,
	mcp.ToolName("datadog_get_slo_history"): getSLOHistory,
	mcp.ToolName("datadog_create_slo"):      createSLO,
	mcp.ToolName("datadog_delete_slo"):      deleteSLO,

	// Downtimes
	mcp.ToolName("datadog_list_downtimes"):  listDowntimes,
	mcp.ToolName("datadog_get_downtime"):    getDowntime,
	mcp.ToolName("datadog_create_downtime"): createDowntime,
	mcp.ToolName("datadog_cancel_downtime"): cancelDowntime,

	// Incidents
	mcp.ToolName("datadog_list_incidents"):            listIncidents,
	mcp.ToolName("datadog_search_incidents"):          searchIncidents,
	mcp.ToolName("datadog_get_incident"):              getIncident,
	mcp.ToolName("datadog_create_incident"):           createIncident,
	mcp.ToolName("datadog_update_incident"):           updateIncident,
	mcp.ToolName("datadog_list_incident_attachments"): listIncidentAttachments,
	mcp.ToolName("datadog_list_incident_todos"):       listIncidentTodos,

	// Incident Services
	mcp.ToolName("datadog_list_incident_services"):  listIncidentServices,
	mcp.ToolName("datadog_get_incident_service"):    getIncidentService,
	mcp.ToolName("datadog_create_incident_service"): createIncidentService,
	mcp.ToolName("datadog_update_incident_service"): updateIncidentService,
	mcp.ToolName("datadog_delete_incident_service"): deleteIncidentService,

	// Incident Teams
	mcp.ToolName("datadog_list_incident_teams"):  listIncidentTeams,
	mcp.ToolName("datadog_get_incident_team"):    getIncidentTeam,
	mcp.ToolName("datadog_create_incident_team"): createIncidentTeam,
	mcp.ToolName("datadog_update_incident_team"): updateIncidentTeam,
	mcp.ToolName("datadog_delete_incident_team"): deleteIncidentTeam,

	// Synthetics
	mcp.ToolName("datadog_list_synthetics_tests"):      listSyntheticsTests,
	mcp.ToolName("datadog_get_synthetics_api_test"):    getSyntheticsAPITest,
	mcp.ToolName("datadog_get_synthetics_test_result"): getSyntheticsTestResult,
	mcp.ToolName("datadog_trigger_synthetics_tests"):   triggerSyntheticsTests,

	// Notebooks
	mcp.ToolName("datadog_list_notebooks"):  listNotebooks,
	mcp.ToolName("datadog_get_notebook"):    getNotebook,
	mcp.ToolName("datadog_create_notebook"): createNotebook,
	mcp.ToolName("datadog_delete_notebook"): deleteNotebook,

	// Users
	mcp.ToolName("datadog_list_users"): listUsers,
	mcp.ToolName("datadog_get_user"):   getUser,

	// Teams
	mcp.ToolName("datadog_list_teams"):                     listTeams,
	mcp.ToolName("datadog_get_team"):                       getTeam,
	mcp.ToolName("datadog_create_team"):                    createTeam,
	mcp.ToolName("datadog_update_team"):                    updateTeam,
	mcp.ToolName("datadog_delete_team"):                    deleteTeam,
	mcp.ToolName("datadog_list_team_members"):              listTeamMembers,
	mcp.ToolName("datadog_add_team_member"):                addTeamMember,
	mcp.ToolName("datadog_update_team_member"):             updateTeamMember,
	mcp.ToolName("datadog_remove_team_member"):             removeTeamMember,
	mcp.ToolName("datadog_get_user_team_memberships"):      getUserTeamMemberships,
	mcp.ToolName("datadog_list_team_links"):                listTeamLinks,
	mcp.ToolName("datadog_get_team_link"):                  getTeamLink,
	mcp.ToolName("datadog_create_team_link"):               createTeamLink,
	mcp.ToolName("datadog_update_team_link"):               updateTeamLink,
	mcp.ToolName("datadog_delete_team_link"):               deleteTeamLink,
	mcp.ToolName("datadog_get_team_permission_settings"):   getTeamPermissionSettings,
	mcp.ToolName("datadog_update_team_permission_setting"): updateTeamPermissionSetting,

	// Spans / APM
	mcp.ToolName("datadog_search_spans"): searchSpans,

	// Service Definition / Software Catalog
	mcp.ToolName("datadog_list_services"): listServices,

	// On-Call
	mcp.ToolName("datadog_list_oncall_schedules"):           listOnCallSchedules,
	mcp.ToolName("datadog_get_oncall_schedule"):             getOnCallSchedule,
	mcp.ToolName("datadog_create_oncall_schedule"):          createOnCallSchedule,
	mcp.ToolName("datadog_update_oncall_schedule"):          updateOnCallSchedule,
	mcp.ToolName("datadog_delete_oncall_schedule"):          deleteOnCallSchedule,
	mcp.ToolName("datadog_get_schedule_oncall_user"):        getScheduleOnCallUser,
	mcp.ToolName("datadog_list_oncall_escalation_policies"): listOnCallEscalationPolicies,
	mcp.ToolName("datadog_get_oncall_escalation_policy"):    getOnCallEscalationPolicy,
	mcp.ToolName("datadog_create_oncall_escalation_policy"): createOnCallEscalationPolicy,
	mcp.ToolName("datadog_update_oncall_escalation_policy"): updateOnCallEscalationPolicy,
	mcp.ToolName("datadog_delete_oncall_escalation_policy"): deleteOnCallEscalationPolicy,
	mcp.ToolName("datadog_get_oncall_team_routing_rules"):   getOnCallTeamRoutingRules,
	mcp.ToolName("datadog_set_oncall_team_routing_rules"):   setOnCallTeamRoutingRules,
	mcp.ToolName("datadog_get_team_oncall_users"):           getTeamOnCallUsers,

	// On-Call Paging
	mcp.ToolName("datadog_list_oncall_pages"):       listOnCallPages,
	mcp.ToolName("datadog_get_oncall_page"):         getOnCallPage,
	mcp.ToolName("datadog_create_oncall_page"):      createOnCallPage,
	mcp.ToolName("datadog_acknowledge_oncall_page"): acknowledgeOnCallPage,
	mcp.ToolName("datadog_escalate_oncall_page"):    escalateOnCallPage,
	mcp.ToolName("datadog_resolve_oncall_page"):     resolveOnCallPage,

	// On-Call Notification Channels
	mcp.ToolName("datadog_list_user_notification_channels"):  listUserNotificationChannels,
	mcp.ToolName("datadog_create_user_notification_channel"): createUserNotificationChannel,
	mcp.ToolName("datadog_get_user_notification_channel"):    getUserNotificationChannel,
	mcp.ToolName("datadog_delete_user_notification_channel"): deleteUserNotificationChannel,

	// On-Call Notification Rules
	mcp.ToolName("datadog_list_user_notification_rules"):  listUserNotificationRules,
	mcp.ToolName("datadog_create_user_notification_rule"): createUserNotificationRule,
	mcp.ToolName("datadog_get_user_notification_rule"):    getUserNotificationRule,
	mcp.ToolName("datadog_update_user_notification_rule"): updateUserNotificationRule,
	mcp.ToolName("datadog_delete_user_notification_rule"): deleteUserNotificationRule,

	// IP Ranges
	mcp.ToolName("datadog_get_ip_ranges"): getIPRanges,
}
