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
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
)

// ── Hosts ────────────────────────────────────────────────────────────

func listHosts(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewHostsApi(d.client)
	opts := datadogV1.NewListHostsOptionalParameters()
	if v := argStr(args, "filter"); v != "" {
		opts = opts.WithFilter(v)
	}
	if v := argStr(args, "sort_field"); v != "" {
		opts = opts.WithSortField(v)
	}
	if v := argStr(args, "sort_dir"); v != "" {
		opts = opts.WithSortDir(v)
	}
	if v := argInt64(args, "count"); v > 0 {
		opts = opts.WithCount(v)
	}
	if v := argInt64(args, "from"); v > 0 {
		opts = opts.WithFrom(v)
	}
	resp, _, err := api.ListHosts(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getHostTotals(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewHostsApi(d.client)
	opts := datadogV1.NewGetHostTotalsOptionalParameters()
	if v := argInt64(args, "from"); v > 0 {
		opts = opts.WithFrom(v)
	}
	resp, _, err := api.GetHostTotals(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func muteHost(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewHostsApi(d.client)
	body := datadogV1.HostMuteSettings{}
	if v := argStr(args, "message"); v != "" {
		body.Message = datadog.PtrString(v)
	}
	if v := argInt64(args, "end"); v > 0 {
		body.End = datadog.PtrInt64(v)
	}
	if argBool(args, "override") {
		body.Override = datadog.PtrBool(true)
	}
	resp, _, err := api.MuteHost(ctx, argStr(args, "hostname"), body)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func unmuteHost(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewHostsApi(d.client)
	resp, _, err := api.UnmuteHost(ctx, argStr(args, "hostname"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

// ── Tags ─────────────────────────────────────────────────────────────

func listTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewListHostTagsOptionalParameters()
	if v := argStr(args, "source"); v != "" {
		opts = opts.WithSource(v)
	}
	resp, _, err := api.ListHostTags(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewGetHostTagsOptionalParameters()
	if v := argStr(args, "source"); v != "" {
		opts = opts.WithSource(v)
	}
	resp, _, err := api.GetHostTags(ctx, argStr(args, "hostname"), *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func createHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewTagsApi(d.client)
	tags := argStrSlice(args, "tags")
	body := datadogV1.HostTags{Tags: tags}
	opts := datadogV1.NewCreateHostTagsOptionalParameters()
	if v := argStr(args, "source"); v != "" {
		opts = opts.WithSource(v)
	}
	resp, _, err := api.CreateHostTags(ctx, argStr(args, "hostname"), body, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func updateHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewTagsApi(d.client)
	tags := argStrSlice(args, "tags")
	body := datadogV1.HostTags{Tags: tags}
	opts := datadogV1.NewUpdateHostTagsOptionalParameters()
	if v := argStr(args, "source"); v != "" {
		opts = opts.WithSource(v)
	}
	resp, _, err := api.UpdateHostTags(ctx, argStr(args, "hostname"), body, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func deleteHostTags(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewTagsApi(d.client)
	opts := datadogV1.NewDeleteHostTagsOptionalParameters()
	if v := argStr(args, "source"); v != "" {
		opts = opts.WithSource(v)
	}
	_, err := api.DeleteHostTags(ctx, argStr(args, "hostname"), *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

// ── SLOs ─────────────────────────────────────────────────────────────

func listSLOs(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	opts := datadogV1.NewListSLOsOptionalParameters()
	if v := argStr(args, "ids"); v != "" {
		opts = opts.WithIds(v)
	}
	if v := argStr(args, "query"); v != "" {
		opts = opts.WithQuery(v)
	}
	if v := argStr(args, "tags_query"); v != "" {
		opts = opts.WithTagsQuery(v)
	}
	if v := argInt64(args, "limit"); v > 0 {
		opts = opts.WithLimit(v)
	}
	if v := argInt64(args, "offset"); v > 0 {
		opts = opts.WithOffset(v)
	}
	resp, _, err := api.ListSLOs(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func searchSLOs(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	opts := datadogV1.NewSearchSLOOptionalParameters()
	if v := argStr(args, "query"); v != "" {
		opts = opts.WithQuery(v)
	}
	if v := argInt64(args, "page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := argInt64(args, "page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	resp, _, err := api.SearchSLO(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getSLO(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	resp, _, err := api.GetSLO(ctx, argStr(args, "id"), *datadogV1.NewGetSLOOptionalParameters())
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getSLOHistory(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	fromTs := argInt64(args, "from")
	toTs := argInt64(args, "to")
	resp, _, err := api.GetSLOHistory(ctx, argStr(args, "id"), fromTs, toTs, *datadogV1.NewGetSLOHistoryOptionalParameters())
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func createSLO(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)

	sloType, _ := datadogV1.NewSLOTypeFromValue(argStr(args, "type"))
	if sloType == nil {
		t := datadogV1.SLOTYPE_METRIC
		sloType = &t
	}

	target, _ := strconv.ParseFloat(argStr(args, "target"), 64)
	timeframe, _ := datadogV1.NewSLOTimeframeFromValue(argStr(args, "timeframe"))
	if timeframe == nil {
		tf := datadogV1.SLOTIMEFRAME_THIRTY_DAYS
		timeframe = &tf
	}

	body := datadogV1.ServiceLevelObjectiveRequest{
		Name: argStr(args, "name"),
		Type: *sloType,
		Thresholds: []datadogV1.SLOThreshold{
			{Target: target, Timeframe: *timeframe},
		},
	}

	if v := argStr(args, "description"); v != "" {
		body.Description = *datadog.NewNullableString(&v)
	}
	if tags := argStrSlice(args, "tags"); len(tags) > 0 {
		body.Tags = tags
	}

	if *sloType == datadogV1.SLOTYPE_MONITOR {
		if ids := argStr(args, "monitor_ids"); ids != "" {
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
		num := argStr(args, "query_numerator")
		den := argStr(args, "query_denominator")
		if num != "" && den != "" {
			body.Query = &datadogV1.ServiceLevelObjectiveQuery{
				Numerator:   num,
				Denominator: den,
			}
		}
	}

	resp, _, err := api.CreateSLO(ctx, body)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func deleteSLO(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewServiceLevelObjectivesApi(d.client)
	resp, _, err := api.DeleteSLO(ctx, argStr(args, "id"), *datadogV1.NewDeleteSLOOptionalParameters())
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

// ── Downtimes ────────────────────────────────────────────────────────

func listDowntimes(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewDowntimesApi(d.client)
	opts := datadogV2.NewListDowntimesOptionalParameters()
	if argBool(args, "current_only") {
		opts = opts.WithCurrentOnly(true)
	}
	resp, _, err := api.ListDowntimes(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getDowntime(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewDowntimesApi(d.client)
	resp, _, err := api.GetDowntime(ctx, argStr(args, "id"), *datadogV2.NewGetDowntimeOptionalParameters())
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func createDowntime(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewDowntimesApi(d.client)

	attrs := datadogV2.DowntimeCreateRequestAttributes{
		Scope: argStr(args, "scope"),
	}

	if v := argStr(args, "message"); v != "" {
		attrs.Message = *datadog.NewNullableString(&v)
	}

	idType := argStr(args, "monitor_identifier_type")
	switch idType {
	case "id":
		monID := argInt64(args, "monitor_identifier_id")
		attrs.MonitorIdentifier = datadogV2.DowntimeMonitorIdentifierIdAsDowntimeMonitorIdentifier(
			&datadogV2.DowntimeMonitorIdentifierId{MonitorId: monID},
		)
	case "tags":
		tags := argStrSlice(args, "monitor_identifier_tags")
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
		return errResult(err)
	}
	return jsonResult(resp)
}

func cancelDowntime(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewDowntimesApi(d.client)
	_, err := api.CancelDowntime(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "cancelled"})
}

// ── Incidents ────────────────────────────────────────────────────────

func listIncidents(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewIncidentsApi(d.client)
	opts := datadogV2.NewListIncidentsOptionalParameters()
	if v := argInt64(args, "page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := argInt64(args, "page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	resp, _, err := api.ListIncidents(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewIncidentsApi(d.client)
	resp, _, err := api.GetIncident(ctx, argStr(args, "id"), *datadogV2.NewGetIncidentOptionalParameters())
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func createIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewIncidentsApi(d.client)

	attrs := datadogV2.IncidentCreateAttributes{
		Title:            argStr(args, "title"),
		CustomerImpacted: argBool(args, "customer_impacted"),
	}

	if sev := argStr(args, "severity"); sev != "" {
		sevField := datadogV2.NewIncidentFieldAttributesSingleValue()
		sevField.SetValue(sev)
		sevField.SetType(datadogV2.INCIDENTFIELDATTRIBUTESSINGLEVALUETYPE_DROPDOWN)
		attrs.Fields = map[string]datadogV2.IncidentFieldAttributes{
			"severity": datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(sevField),
		}
	}

	body := datadogV2.IncidentCreateRequest{
		Data: datadogV2.IncidentCreateData{
			Attributes: attrs,
			Type:       datadogV2.INCIDENTTYPE_INCIDENTS,
		},
	}

	resp, _, err := api.CreateIncident(ctx, body)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func updateIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewIncidentsApi(d.client)

	incidentID := argStr(args, "id")
	attrs := datadogV2.IncidentUpdateAttributes{}
	if v := argStr(args, "title"); v != "" {
		attrs.Title = datadog.PtrString(v)
	}
	if _, ok := args["customer_impacted"]; ok {
		attrs.CustomerImpacted = datadog.PtrBool(argBool(args, "customer_impacted"))
	}

	fields := map[string]datadogV2.IncidentFieldAttributes{}
	if sev := argStr(args, "severity"); sev != "" {
		sevField := datadogV2.NewIncidentFieldAttributesSingleValue()
		sevField.SetValue(sev)
		sevField.SetType(datadogV2.INCIDENTFIELDATTRIBUTESSINGLEVALUETYPE_DROPDOWN)
		fields["severity"] = datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(sevField)
	}
	if status := argStr(args, "status"); status != "" {
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
		return errResult(err)
	}
	return jsonResult(resp)
}

// ── Synthetics ───────────────────────────────────────────────────────

func listSyntheticsTests(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewSyntheticsApi(d.client)
	opts := datadogV1.NewListTestsOptionalParameters()
	if v := argInt64(args, "page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := argInt64(args, "page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	resp, _, err := api.ListTests(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getSyntheticsAPITest(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewSyntheticsApi(d.client)
	resp, _, err := api.GetAPITest(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getSyntheticsTestResult(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewSyntheticsApi(d.client)
	resp, _, err := api.GetAPITestLatestResults(ctx, argStr(args, "id"), *datadogV1.NewGetAPITestLatestResultsOptionalParameters())
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func triggerSyntheticsTests(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewSyntheticsApi(d.client)
	ids := argStrSlice(args, "public_ids")
	var tests []datadogV1.SyntheticsTriggerTest
	for _, id := range ids {
		tests = append(tests, datadogV1.SyntheticsTriggerTest{PublicId: id})
	}
	resp, _, err := api.TriggerTests(ctx, datadogV1.SyntheticsTriggerBody{Tests: tests})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

// ── Notebooks ────────────────────────────────────────────────────────

func listNotebooks(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewNotebooksApi(d.client)
	opts := datadogV1.NewListNotebooksOptionalParameters()
	if v := argStr(args, "query"); v != "" {
		opts = opts.WithQuery(v)
	}
	if v := argInt64(args, "count"); v > 0 {
		opts = opts.WithCount(v)
	}
	if v := argInt64(args, "start"); v > 0 {
		opts = opts.WithStart(v)
	}
	if v := argStr(args, "sort_field"); v != "" {
		opts = opts.WithSortField(v)
	}
	if v := argStr(args, "sort_dir"); v != "" {
		opts = opts.WithSortDir(v)
	}
	resp, _, err := api.ListNotebooks(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getNotebook(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewNotebooksApi(d.client)
	resp, _, err := api.GetNotebook(ctx, argInt64(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func createNotebook(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewNotebooksApi(d.client)

	var cells []datadogV1.NotebookCellCreateRequest
	if cj := argStr(args, "cells_json"); cj != "" {
		if err := json.Unmarshal([]byte(cj), &cells); err != nil {
			return errResult(fmt.Errorf("invalid cells_json: %w", err))
		}
	}

	relTime := datadogV1.NotebookRelativeTime{
		LiveSpan: datadogV1.WIDGETLIVESPAN_PAST_ONE_HOUR,
	}
	globalTime := datadogV1.NotebookRelativeTimeAsNotebookGlobalTime(&relTime)

	body := datadogV1.NotebookCreateRequest{
		Data: datadogV1.NotebookCreateData{
			Attributes: datadogV1.NotebookCreateDataAttributes{
				Name:  argStr(args, "name"),
				Cells: cells,
				Time:  globalTime,
			},
			Type: datadogV1.NOTEBOOKRESOURCETYPE_NOTEBOOKS,
		},
	}

	resp, _, err := api.CreateNotebook(ctx, body)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func deleteNotebook(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewNotebooksApi(d.client)
	_, err := api.DeleteNotebook(ctx, argInt64(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

// ── Users ────────────────────────────────────────────────────────────

func listUsers(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewUsersApi(d.client)
	opts := datadogV2.NewListUsersOptionalParameters()
	if v := argInt64(args, "page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := argInt64(args, "page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	if v := argStr(args, "sort"); v != "" {
		opts = opts.WithSort(v)
	}
	if v := argStr(args, "filter"); v != "" {
		opts = opts.WithFilter(v)
	}
	resp, _, err := api.ListUsers(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getUser(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewUsersApi(d.client)
	resp, _, err := api.GetUser(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

// ── Spans / APM ──────────────────────────────────────────────────────

func searchSpans(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewSpansApi(d.client)
	opts := datadogV2.NewListSpansGetOptionalParameters()

	if v := argStr(args, "query"); v != "" {
		opts = opts.WithFilterQuery(v)
	}

	from := parseTime(argStr(args, "from"), -time.Hour)
	to := parseTime(argStr(args, "to"), 0)
	opts = opts.WithFilterFrom(from.Format(time.RFC3339))
	opts = opts.WithFilterTo(to.Format(time.RFC3339))

	if v := argInt(args, "limit"); v > 0 {
		opts = opts.WithPageLimit(int32(v))
	}

	sort := datadogV2.SPANSSORT_TIMESTAMP_DESCENDING
	if argStr(args, "sort") == "timestamp" {
		sort = datadogV2.SPANSSORT_TIMESTAMP_ASCENDING
	}
	opts = opts.WithSort(sort)

	resp, _, err := api.ListSpansGet(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

// ── Software Catalog ─────────────────────────────────────────────────

func listServices(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewSoftwareCatalogApi(d.client)
	opts := datadogV2.NewListCatalogEntityOptionalParameters()
	if v := argInt64(args, "page_size"); v > 0 {
		opts = opts.WithPageLimit(v)
	}
	if v := argInt64(args, "page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	resp, _, err := api.ListCatalogEntity(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

// ── IP Ranges ────────────────────────────────────────────────────────

func getIPRanges(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV1.NewIPRangesApi(d.client)
	resp, _, err := api.GetIPRanges(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}
