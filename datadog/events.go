package datadog

import (
	"context"
	"math"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
)

func listEvents(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewEventsApi(d.client)
	opts := datadogV2.NewListEventsOptionalParameters()
	if v := argStr(args, "query"); v != "" {
		opts = opts.WithFilterQuery(v)
	}

	from := parseTime(argStr(args, "from"), -time.Hour)
	to := parseTime(argStr(args, "to"), 0)
	opts = opts.WithFilterFrom(from.Format(time.RFC3339))
	opts = opts.WithFilterTo(to.Format(time.RFC3339))

	if v := argInt(args, "limit"); v > 0 && v <= math.MaxInt32 {
		opts = opts.WithPageLimit(int32(v))
	}
	if argStr(args, "sort") == "timestamp" {
		opts = opts.WithSort(datadogV2.EVENTSSORT_TIMESTAMP_ASCENDING)
	}

	resp, _, err := api.ListEvents(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func searchEvents(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewEventsApi(d.client)

	from := parseTime(argStr(args, "from"), -time.Hour)
	to := parseTime(argStr(args, "to"), 0)
	query := argStr(args, "query")
	limit := int32(min(optInt(args, "limit", 10), math.MaxInt32))

	sort := datadogV2.EVENTSSORT_TIMESTAMP_DESCENDING
	if argStr(args, "sort") == "timestamp" {
		sort = datadogV2.EVENTSSORT_TIMESTAMP_ASCENDING
	}

	opts := datadogV2.NewSearchEventsOptionalParameters().WithBody(datadogV2.EventsListRequest{
		Filter: &datadogV2.EventsQueryFilter{
			Query: &query,
			From:  datadog.PtrString(from.Format(time.RFC3339)),
			To:    datadog.PtrString(to.Format(time.RFC3339)),
		},
		Page: &datadogV2.EventsRequestPage{
			Limit: &limit,
		},
		Sort: &sort,
	})

	resp, _, err := api.SearchEvents(ctx, *opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func getEvent(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewEventsApi(d.client)
	resp, _, err := api.GetEvent(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}

func createEvent(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	api := datadogV2.NewEventsApi(d.client)

	title := argStr(args, "title")
	text := argStr(args, "text")

	payload := datadogV2.EventPayload{
		Title:    title,
		Category: datadogV2.EVENTCATEGORY_CHANGE,
		Attributes: datadogV2.ChangeEventCustomAttributesAsEventPayloadAttributes(
			datadogV2.NewChangeEventCustomAttributes(
				datadogV2.ChangeEventCustomAttributesChangedResource{
					Name: title,
					Type: datadogV2.CHANGEEVENTCUSTOMATTRIBUTESCHANGEDRESOURCETYPE_CONFIGURATION,
				},
			),
		),
	}
	if text != "" {
		payload.Message = datadog.PtrString(text)
	}
	if tags := argStrSlice(args, "tags"); len(tags) > 0 {
		payload.Tags = tags
	}
	if v := argStr(args, "aggregation_key"); v != "" {
		payload.AggregationKey = datadog.PtrString(v)
	}

	body := datadogV2.EventCreateRequestPayload{
		Data: datadogV2.EventCreateRequest{
			Attributes: payload,
			Type:       datadogV2.EVENTCREATEREQUESTTYPE_EVENT,
		},
	}

	resp, _, err := api.CreateEvent(ctx, body)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp)
}
