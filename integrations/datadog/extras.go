package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
)

// ── Hosts ────────────────────────────────────────────────────────────

func listHosts(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewHostsApi(d.client)
	opts := datadogV1.NewListHostsOptionalParameters()
	if v := r.Str("filter"); v != "" {
		opts = opts.WithFilter(v)
	}
	if v := r.Str("sort_field"); v != "" {
		opts = opts.WithSortField(v)
	}
	if v := r.Str("sort_dir"); v != "" {
		opts = opts.WithSortDir(v)
	}
	if v := r.Int64("count"); v > 0 {
		opts = opts.WithCount(v)
	}
	if v := r.Int64("from"); v > 0 {
		opts = opts.WithFrom(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListHosts(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getHostTotals(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewHostsApi(d.client)
	opts := datadogV1.NewGetHostTotalsOptionalParameters()
	if v := r.Int64("from"); v > 0 {
		opts = opts.WithFrom(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.GetHostTotals(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func muteHost(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	hostname := r.Str("hostname")
	api := datadogV1.NewHostsApi(d.client)
	body := datadogV1.HostMuteSettings{}
	if v := r.Str("message"); v != "" {
		body.Message = datadog.PtrString(v)
	}
	if v := r.Int64("end"); v > 0 {
		body.End = datadog.PtrInt64(v)
	}
	if r.Bool("override") {
		body.Override = datadog.PtrBool(true)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.MuteHost(ctx, hostname, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func unmuteHost(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	hostname := r.Str("hostname")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewHostsApi(d.client)
	resp, _, err := api.UnmuteHost(ctx, hostname)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Tags ─────────────────────────────────────────────────────────────

func listTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewListHostTagsOptionalParameters()
	if v := r.Str("source"); v != "" {
		opts = opts.WithSource(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListHostTags(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	hostname := r.Str("hostname")
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewGetHostTagsOptionalParameters()
	if v := r.Str("source"); v != "" {
		opts = opts.WithSource(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.GetHostTags(ctx, hostname, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tags := r.StrSlice("tags")
	hostname := r.Str("hostname")
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewCreateHostTagsOptionalParameters()
	if v := r.Str("source"); v != "" {
		opts = opts.WithSource(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV1.HostTags{Tags: tags}
	resp, _, err := api.CreateHostTags(ctx, hostname, body, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tags := r.StrSlice("tags")
	hostname := r.Str("hostname")
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewUpdateHostTagsOptionalParameters()
	if v := r.Str("source"); v != "" {
		opts = opts.WithSource(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV1.HostTags{Tags: tags}
	resp, _, err := api.UpdateHostTags(ctx, hostname, body, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	hostname := r.Str("hostname")
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewDeleteHostTagsOptionalParameters()
	if v := r.Str("source"); v != "" {
		opts = opts.WithSource(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := api.DeleteHostTags(ctx, hostname, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── SLOs ─────────────────────────────────────────────────────────────

func listSLOs(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	opts := datadogV1.NewListSLOsOptionalParameters()
	if v := r.Str("ids"); v != "" {
		opts = opts.WithIds(v)
	}
	if v := r.Str("query"); v != "" {
		opts = opts.WithQuery(v)
	}
	if v := r.Str("tags_query"); v != "" {
		opts = opts.WithTagsQuery(v)
	}
	if v := r.Int64("limit"); v > 0 {
		opts = opts.WithLimit(v)
	}
	if v := r.Int64("offset"); v > 0 {
		opts = opts.WithOffset(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListSLOs(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func searchSLOs(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	opts := datadogV1.NewSearchSLOOptionalParameters()
	if v := r.Str("query"); v != "" {
		opts = opts.WithQuery(v)
	}
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.SearchSLO(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getSLO(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	resp, _, err := api.GetSLO(ctx, id, *datadogV1.NewGetSLOOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getSLOHistory(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	fromTs := r.Int64("from")
	toTs := r.Int64("to")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	resp, _, err := api.GetSLOHistory(ctx, id, fromTs, toTs, *datadogV1.NewGetSLOHistoryOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createSLO(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)

	sloType, _ := datadogV1.NewSLOTypeFromValue(r.Str("type"))
	if sloType == nil {
		t := datadogV1.SLOTYPE_METRIC
		sloType = &t
	}

	target := r.Float64("target")
	timeframe, _ := datadogV1.NewSLOTimeframeFromValue(r.Str("timeframe"))
	if timeframe == nil {
		tf := datadogV1.SLOTIMEFRAME_THIRTY_DAYS
		timeframe = &tf
	}

	body := datadogV1.ServiceLevelObjectiveRequest{
		Name: r.Str("name"),
		Type: *sloType,
		Thresholds: []datadogV1.SLOThreshold{
			{Target: target, Timeframe: *timeframe},
		},
	}

	if v := r.Str("description"); v != "" {
		body.Description = *datadog.NewNullableString(&v)
	}
	if tags := r.StrSlice("tags"); len(tags) > 0 {
		body.Tags = tags
	}

	if *sloType == datadogV1.SLOTYPE_MONITOR {
		if ids := r.Str("monitor_ids"); ids != "" {
			var monitorIDs []int64
			for _, s := range strings.Split(ids, ",") {
				s = strings.TrimSpace(s)
				if n, err := strconv.ParseInt(s, 10, 64); err == nil {
					monitorIDs = append(monitorIDs, n)
				}
			}
			body.MonitorIds = monitorIDs
		}
	}

	if *sloType == datadogV1.SLOTYPE_METRIC {
		num := r.Str("query_numerator")
		den := r.Str("query_denominator")
		if num != "" && den != "" {
			body.Query = &datadogV1.ServiceLevelObjectiveQuery{
				Numerator:   num,
				Denominator: den,
			}
		}
	}

	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.CreateSLO(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteSLO(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	resp, _, err := api.DeleteSLO(ctx, id, *datadogV1.NewDeleteSLOOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Downtimes ────────────────────────────────────────────────────────

func listDowntimes(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewDowntimesApi(d.client)
	opts := datadogV2.NewListDowntimesOptionalParameters()
	if r.Bool("current_only") {
		opts = opts.WithCurrentOnly(true)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListDowntimes(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getDowntime(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewDowntimesApi(d.client)
	resp, _, err := api.GetDowntime(ctx, id, *datadogV2.NewGetDowntimeOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createDowntime(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewDowntimesApi(d.client)

	attrs := datadogV2.DowntimeCreateRequestAttributes{
		Scope: r.Str("scope"),
	}

	if v := r.Str("message"); v != "" {
		attrs.Message = *datadog.NewNullableString(&v)
	}

	idType := r.Str("monitor_identifier_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	switch idType {
	case "id":
		monID, err := mcp.ArgInt64(args, "monitor_identifier_id")
		if err != nil {
			return mcp.ErrResult(err)
		}
		attrs.MonitorIdentifier = datadogV2.DowntimeMonitorIdentifierIdAsDowntimeMonitorIdentifier(
			&datadogV2.DowntimeMonitorIdentifierId{MonitorId: monID},
		)
	case "tags":
		tags, err := mcp.ArgStrSlice(args, "monitor_identifier_tags")
		if err != nil {
			return mcp.ErrResult(err)
		}
		attrs.MonitorIdentifier = datadogV2.DowntimeMonitorIdentifierTagsAsDowntimeMonitorIdentifier(
			&datadogV2.DowntimeMonitorIdentifierTags{MonitorTags: tags},
		)
	default:
		attrs.MonitorIdentifier = datadogV2.DowntimeMonitorIdentifierTagsAsDowntimeMonitorIdentifier(
			&datadogV2.DowntimeMonitorIdentifierTags{MonitorTags: []string{"*"}},
		)
	}

	body := datadogV2.DowntimeCreateRequest{
		Data: datadogV2.DowntimeCreateRequestData{
			Attributes: attrs,
			Type:       datadogV2.DOWNTIMERESOURCETYPE_DOWNTIME,
		},
	}

	resp, _, err := api.CreateDowntime(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func cancelDowntime(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewDowntimesApi(d.client)
	_, err := api.CancelDowntime(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "cancelled"})
}

// ── Incidents ────────────────────────────────────────────────────────

func listIncidents(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewIncidentsApi(d.client)
	opts := datadogV2.NewListIncidentsOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListIncidents(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentsApi(d.client)
	resp, _, err := api.GetIncident(ctx, id, *datadogV2.NewGetIncidentOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewIncidentsApi(d.client)

	attrs := datadogV2.IncidentCreateAttributes{
		Title:            r.Str("title"),
		CustomerImpacted: r.Bool("customer_impacted"),
	}

	if sev := r.Str("severity"); sev != "" {
		sevField := datadogV2.NewIncidentFieldAttributesSingleValue()
		sevField.SetValue(sev)
		sevField.SetType(datadogV2.INCIDENTFIELDATTRIBUTESSINGLEVALUETYPE_DROPDOWN)
		attrs.Fields = map[string]datadogV2.IncidentFieldAttributes{
			"severity": datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(sevField),
		}
	}

	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := datadogV2.IncidentCreateRequest{
		Data: datadogV2.IncidentCreateData{
			Attributes: attrs,
			Type:       datadogV2.INCIDENTTYPE_INCIDENTS,
		},
	}

	resp, _, err := api.CreateIncident(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewIncidentsApi(d.client)

	incidentID := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	attrs := datadogV2.IncidentUpdateAttributes{}
	if v := r.Str("title"); v != "" {
		attrs.Title = datadog.PtrString(v)
	}
	if _, ok := args["customer_impacted"]; ok {
		attrs.CustomerImpacted = datadog.PtrBool(r.Bool("customer_impacted"))
	}

	fields := map[string]datadogV2.IncidentFieldAttributes{}
	if sev := r.Str("severity"); sev != "" {
		sevField := datadogV2.NewIncidentFieldAttributesSingleValue()
		sevField.SetValue(sev)
		sevField.SetType(datadogV2.INCIDENTFIELDATTRIBUTESSINGLEVALUETYPE_DROPDOWN)
		fields["severity"] = datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(sevField)
	}
	if status := r.Str("status"); status != "" {
		statusField := datadogV2.NewIncidentFieldAttributesSingleValue()
		statusField.SetValue(status)
		statusField.SetType(datadogV2.INCIDENTFIELDATTRIBUTESSINGLEVALUETYPE_DROPDOWN)
		fields["state"] = datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(statusField)
	}
	if len(fields) > 0 {
		attrs.Fields = fields
	}

	body := datadogV2.IncidentUpdateRequest{
		Data: datadogV2.IncidentUpdateData{
			Id:         incidentID,
			Attributes: &attrs,
			Type:       datadogV2.INCIDENTTYPE_INCIDENTS,
		},
	}

	resp, _, err := api.UpdateIncident(ctx, incidentID, body, *datadogV2.NewUpdateIncidentOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Synthetics ───────────────────────────────────────────────────────

func listSyntheticsTests(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewSyntheticsApi(d.client)
	opts := datadogV1.NewListTestsOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListTests(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getSyntheticsAPITest(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewSyntheticsApi(d.client)
	resp, _, err := api.GetAPITest(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getSyntheticsTestResult(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewSyntheticsApi(d.client)
	resp, _, err := api.GetAPITestLatestResults(ctx, id, *datadogV1.NewGetAPITestLatestResultsOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func triggerSyntheticsTests(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewSyntheticsApi(d.client)
	ids := r.StrSlice("public_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var tests []datadogV1.SyntheticsTriggerTest
	for _, id := range ids {
		tests = append(tests, datadogV1.SyntheticsTriggerTest{PublicId: id})
	}
	resp, _, err := api.TriggerTests(ctx, datadogV1.SyntheticsTriggerBody{Tests: tests})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Notebooks ────────────────────────────────────────────────────────

func listNotebooks(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV1.NewNotebooksApi(d.client)
	opts := datadogV1.NewListNotebooksOptionalParameters()
	if v := r.Str("query"); v != "" {
		opts = opts.WithQuery(v)
	}
	if v := r.Int64("count"); v > 0 {
		opts = opts.WithCount(v)
	}
	if v := r.Int64("start"); v > 0 {
		opts = opts.WithStart(v)
	}
	if v := r.Str("sort_field"); v != "" {
		opts = opts.WithSortField(v)
	}
	if v := r.Str("sort_dir"); v != "" {
		opts = opts.WithSortDir(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListNotebooks(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getNotebook(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Int64("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewNotebooksApi(d.client)
	resp, _, err := api.GetNotebook(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createNotebook(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	api := datadogV1.NewNotebooksApi(d.client)

	var cells []datadogV1.NotebookCellCreateRequest
	if cj := r.Str("cells_json"); cj != "" {
		if err := json.Unmarshal([]byte(cj), &cells); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid cells_json: %w", err))
		}
	}

	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	relTime := datadogV1.NotebookRelativeTime{
		LiveSpan: datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR,
	}
	globalTime := datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(&relTime)

	body := datadogV1.NotebookCreateRequest{
		Data: datadogV1.NotebookCreateData{
			Attributes: datadogV1.NotebookCreateDataAttributes{
				Name:  name,
				Cells: cells,
				Time:  globalTime,
			},
			Type: datadogV1.NOTEBOOKRESOURCETYPE_NOTEBOOKS,
		},
	}

	resp, _, err := api.CreateNotebook(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteNotebook(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Int64("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV1.NewNotebooksApi(d.client)
	_, err := api.DeleteNotebook(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Users ────────────────────────────────────────────────────────────

func listUsers(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewUsersApi(d.client)
	opts := datadogV2.NewListUsersOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	if v := r.Str("sort"); v != "" {
		opts = opts.WithSort(v)
	}
	if v := r.Str("filter"); v != "" {
		opts = opts.WithFilter(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListUsers(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getUser(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewUsersApi(d.client)
	resp, _, err := api.GetUser(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Spans / APM ──────────────────────────────────────────────────────

func searchSpans(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewSpansApi(d.client)
	opts := datadogV2.NewListSpansGetOptionalParameters()

	if v := r.Str("query"); v != "" {
		opts = opts.WithFilterQuery(v)
	}

	from := parseTime(r.Str("from"), -time.Hour)
	to := parseTime(r.Str("to"), 0)
	opts = opts.WithFilterFrom(from.Format(time.RFC3339))
	opts = opts.WithFilterTo(to.Format(time.RFC3339))

	if v := r.Int("limit"); v > 0 && v <= math.MaxInt32 {
		opts = opts.WithPageLimit(int32(v))
	}

	sort := datadogV2.SPANSSORT_TIMESTAMP_DESCENDING
	if r.Str("sort") == "timestamp" {
		sort = datadogV2.SPANSSORT_TIMESTAMP_ASCENDING
	}
	opts = opts.WithSort(sort)

	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListSpansGet(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Software Catalog ─────────────────────────────────────────────────

func listServices(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewSoftwareCatalogApi(d.client)
	opts := datadogV2.NewListCatalogEntityOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageLimit(v)
	}
	if v := r.Int64("page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListCatalogEntity(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── IP Ranges ────────────────────────────────────────────────────────

func getIPRanges(ctx context.Context, d *dd, _ map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewIPRangesApi(d.client)
	resp, _, err := api.GetIPRanges(ctx)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
