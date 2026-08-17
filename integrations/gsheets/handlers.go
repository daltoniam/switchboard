package gsheets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// ── Spreadsheet lifecycle ───────────────────────────────────────────

func getSpreadsheet(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	ranges := r.Str("ranges")
	includeGrid := r.Str("include_grid_data")
	fields := r.Str("fields")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_get_spreadsheet: spreadsheet_id is required"))
	}

	params := url.Values{}
	if ranges != "" {
		for _, rng := range strings.Split(ranges, ",") {
			if rng = strings.TrimSpace(rng); rng != "" {
				params.Add("ranges", rng)
			}
		}
	}
	if includeGrid != "" {
		params.Set("includeGridData", includeGrid)
	}
	if fields != "" {
		params.Set("fields", fields)
	}

	path := "/spreadsheets/" + url.PathEscape(spreadsheetID)
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createSpreadsheet(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	sheetTitles := r.Str("sheet_titles")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{}
	if title != "" {
		body["properties"] = map[string]any{"title": title}
	}
	if sheetTitles != "" {
		sheets := []any{}
		for _, name := range strings.Split(sheetTitles, ",") {
			if name = strings.TrimSpace(name); name != "" {
				sheets = append(sheets, map[string]any{
					"properties": map[string]any{"title": name},
				})
			}
		}
		if len(sheets) > 0 {
			body["sheets"] = sheets
		}
	}

	data, err := g.post(ctx, "/spreadsheets", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Reading values ──────────────────────────────────────────────────

func getValues(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	rangeA1 := r.Str("range")
	valueRender := r.Str("value_render_option")
	dateRender := r.Str("date_time_render_option")
	majorDim := r.Str("major_dimension")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_get_values: spreadsheet_id is required"))
	}
	if rangeA1 == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_get_values: range is required"))
	}

	params := url.Values{}
	if valueRender != "" {
		params.Set("valueRenderOption", valueRender)
	}
	if dateRender != "" {
		params.Set("dateTimeRenderOption", dateRender)
	}
	if majorDim != "" {
		params.Set("majorDimension", majorDim)
	}

	path := "/spreadsheets/" + url.PathEscape(spreadsheetID) + "/values/" + url.PathEscape(rangeA1)
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func batchGetValues(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	ranges := r.Str("ranges")
	valueRender := r.Str("value_render_option")
	dateRender := r.Str("date_time_render_option")
	majorDim := r.Str("major_dimension")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_get_values: spreadsheet_id is required"))
	}
	if ranges == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_get_values: ranges is required"))
	}

	params := url.Values{}
	for _, rng := range strings.Split(ranges, ",") {
		if rng = strings.TrimSpace(rng); rng != "" {
			params.Add("ranges", rng)
		}
	}
	if valueRender != "" {
		params.Set("valueRenderOption", valueRender)
	}
	if dateRender != "" {
		params.Set("dateTimeRenderOption", dateRender)
	}
	if majorDim != "" {
		params.Set("majorDimension", majorDim)
	}

	path := "/spreadsheets/" + url.PathEscape(spreadsheetID) + "/values:batchGet?" + params.Encode()
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Writing values ──────────────────────────────────────────────────

func updateValues(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	rangeA1 := r.Str("range")
	valuesRaw := r.Str("values")
	valueInput := r.Str("value_input_option")
	majorDim := r.Str("major_dimension")
	includeValues := r.Str("include_values_in_response")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_update_values: spreadsheet_id is required"))
	}
	if rangeA1 == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_update_values: range is required"))
	}
	if valuesRaw == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_update_values: values is required"))
	}

	var values [][]any
	if err := json.Unmarshal([]byte(valuesRaw), &values); err != nil {
		return mcp.ErrResult(fmt.Errorf("gsheets_update_values: values must be a JSON 2-D array: %w", err))
	}
	if valueInput == "" {
		valueInput = "USER_ENTERED"
	}

	body := map[string]any{
		"range":  rangeA1,
		"values": values,
	}
	if majorDim != "" {
		body["majorDimension"] = majorDim
	}

	params := url.Values{}
	params.Set("valueInputOption", valueInput)
	if includeValues == "true" {
		params.Set("includeValuesInResponse", "true")
	}

	path := "/spreadsheets/" + url.PathEscape(spreadsheetID) + "/values/" + url.PathEscape(rangeA1) + "?" + params.Encode()
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func appendValues(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	rangeA1 := r.Str("range")
	valuesRaw := r.Str("values")
	valueInput := r.Str("value_input_option")
	insertOption := r.Str("insert_data_option")
	majorDim := r.Str("major_dimension")
	includeValues := r.Str("include_values_in_response")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_append_values: spreadsheet_id is required"))
	}
	if rangeA1 == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_append_values: range is required"))
	}
	if valuesRaw == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_append_values: values is required"))
	}

	var values [][]any
	if err := json.Unmarshal([]byte(valuesRaw), &values); err != nil {
		return mcp.ErrResult(fmt.Errorf("gsheets_append_values: values must be a JSON 2-D array: %w", err))
	}
	if valueInput == "" {
		valueInput = "USER_ENTERED"
	}

	body := map[string]any{
		"range":  rangeA1,
		"values": values,
	}
	if majorDim != "" {
		body["majorDimension"] = majorDim
	}

	params := url.Values{}
	params.Set("valueInputOption", valueInput)
	if insertOption != "" {
		params.Set("insertDataOption", insertOption)
	}
	if includeValues == "true" {
		params.Set("includeValuesInResponse", "true")
	}

	path := "/spreadsheets/" + url.PathEscape(spreadsheetID) + "/values/" + url.PathEscape(rangeA1) + ":append?" + params.Encode()
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func clearValues(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	rangeA1 := r.Str("range")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_clear_values: spreadsheet_id is required"))
	}
	if rangeA1 == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_clear_values: range is required"))
	}

	path := "/spreadsheets/" + url.PathEscape(spreadsheetID) + "/values/" + url.PathEscape(rangeA1) + ":clear"
	data, err := g.post(ctx, path, map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func batchUpdateValues(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	dataRaw := r.Str("data")
	valueInput := r.Str("value_input_option")
	includeValues := r.Str("include_values_in_response")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_update_values: spreadsheet_id is required"))
	}
	if dataRaw == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_update_values: data is required"))
	}

	var data []any
	if err := json.Unmarshal([]byte(dataRaw), &data); err != nil {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_update_values: data must be a JSON array: %w", err))
	}
	if valueInput == "" {
		valueInput = "USER_ENTERED"
	}

	body := map[string]any{
		"data":             data,
		"valueInputOption": valueInput,
	}
	if includeValues == "true" {
		body["includeValuesInResponse"] = true
	}

	path := "/spreadsheets/" + url.PathEscape(spreadsheetID) + "/values:batchUpdate"
	respData, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(respData)
}

// ── Spreadsheet-level batchUpdate ──────────────────────────────────

// batchUpdate accepts a JSON-encoded `requests` array and forwards it to
// spreadsheets.batchUpdate. Same shape as the gdocs batch_update tool.
func batchUpdate(ctx context.Context, g *gsheets, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spreadsheetID := r.Str("spreadsheet_id")
	requestsRaw := r.Str("requests")
	includeSpreadsheet := r.Str("include_spreadsheet_in_response")
	includeGrid := r.Str("response_include_grid_data")
	responseRanges := r.Str("response_ranges")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spreadsheetID == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_update: spreadsheet_id is required"))
	}
	if requestsRaw == "" {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_update: requests is required"))
	}

	var requests []any
	if err := json.Unmarshal([]byte(requestsRaw), &requests); err != nil {
		return mcp.ErrResult(fmt.Errorf("gsheets_batch_update: requests must be a JSON array: %w", err))
	}

	body := map[string]any{"requests": requests}
	if includeSpreadsheet == "true" {
		body["includeSpreadsheetInResponse"] = true
	}
	if includeGrid == "true" {
		body["responseIncludeGridData"] = true
	}
	if responseRanges != "" {
		ranges := []any{}
		for _, rng := range strings.Split(responseRanges, ",") {
			if rng = strings.TrimSpace(rng); rng != "" {
				ranges = append(ranges, rng)
			}
		}
		if len(ranges) > 0 {
			body["responseRanges"] = ranges
		}
	}

	data, err := g.post(ctx, "/spreadsheets/"+url.PathEscape(spreadsheetID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
