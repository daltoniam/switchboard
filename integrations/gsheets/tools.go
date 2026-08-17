package gsheets

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Spreadsheet lifecycle ───────────────────────────────────────
	{
		Name: mcp.ToolName("gsheets_get_spreadsheet"), Description: "Retrieve (get) a Google Sheets spreadsheet by ID, including its sheet tabs, their properties (titles, grid sizes, hidden state, tab colors), and optional cell data. Start here when you need to discover what sheets/tabs exist, get sheetIds for batchUpdate, or fetch metadata. For raw cell values, use gsheets_get_values (faster and lighter). Set include_grid_data=true to embed cell data — only do this for small sheets, the response can be enormous.",
		Parameters: map[string]string{
			"spreadsheet_id":    "The Google Sheets spreadsheet ID (the long string in the spreadsheet URL between /d/ and /edit)",
			"ranges":            "Optional comma-separated A1-notation ranges to limit the response to (e.g. 'Sheet1!A1:C10,Sheet2!A:B')",
			"include_grid_data": "true/false — embed full cell grid data in the response (default false; can be huge)",
			"fields":            "Optional partial-response field mask (e.g. 'sheets.properties,spreadsheetId') to trim the response",
		},
		Required: []string{"spreadsheet_id"},
	},
	{
		Name: mcp.ToolName("gsheets_create_spreadsheet"), Description: "Create a new Google Sheets spreadsheet. Returns the new spreadsheet including its ID, which you can pass to gsheets_update_values or gsheets_batch_update to add content. To save the spreadsheet into a specific Drive folder, use gdrive_update_file afterward to set parents.",
		Parameters: map[string]string{
			"title":        "Spreadsheet title (defaults to 'Untitled spreadsheet')",
			"sheet_titles": "Optional comma-separated list of initial sheet/tab titles (e.g. 'Summary,Data,Notes'). Default is one sheet named 'Sheet1'.",
		},
	},

	// ── Reading values ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("gsheets_get_values"), Description: "Read cell values from a Google Sheets range using A1 notation. The lightest-weight way to extract data — far smaller than gsheets_get_spreadsheet with grid data. Returns a 2-D array of cell values. Use 'Sheet1' or 'Sheet1!A1:D100'-style ranges. Empty trailing cells are omitted, so each row may have fewer columns than the header row.",
		Parameters: map[string]string{
			"spreadsheet_id":          "The spreadsheet ID",
			"range":                   "A1-notation range (e.g. 'Sheet1!A1:D100', 'Data', 'Summary!A:C')",
			"value_render_option":     "How values are returned: FORMATTED_VALUE (default; what's shown in the UI), UNFORMATTED_VALUE (raw values), or FORMULA (cell formulas)",
			"date_time_render_option": "How dates/times are returned: SERIAL_NUMBER (default) or FORMATTED_STRING",
			"major_dimension":         "ROWS (default) or COLUMNS — controls whether the outer array is rows or columns",
		},
		Required: []string{"spreadsheet_id", "range"},
	},
	{
		Name: mcp.ToolName("gsheets_batch_get_values"), Description: "Read cell values from multiple A1 ranges in one Google Sheets request. Use when you need to pull values from several non-contiguous ranges (e.g. 'Summary!A1:B5' and 'Data!D1:D100') — much faster than multiple gsheets_get_values calls.",
		Parameters: map[string]string{
			"spreadsheet_id":          "The spreadsheet ID",
			"ranges":                  "Comma-separated A1-notation ranges (e.g. 'Sheet1!A1:B5,Sheet2!C:C')",
			"value_render_option":     "FORMATTED_VALUE (default), UNFORMATTED_VALUE, or FORMULA",
			"date_time_render_option": "SERIAL_NUMBER (default) or FORMATTED_STRING",
			"major_dimension":         "ROWS (default) or COLUMNS",
		},
		Required: []string{"spreadsheet_id", "ranges"},
	},

	// ── Writing values ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("gsheets_update_values"), Description: "Overwrite cell values in a Google Sheets range. The values array must match the range shape; cells in the range that aren't present in values are NOT cleared. Pass values as a JSON-encoded 2-D array (rows of columns). For inserting at the bottom of a table, prefer gsheets_append_values.",
		Parameters: map[string]string{
			"spreadsheet_id":             "The spreadsheet ID",
			"range":                      "A1-notation range to write to (e.g. 'Sheet1!A2:C2')",
			"values":                     "JSON 2-D array of values (e.g. '[[\"a\",\"b\",\"c\"],[1,2,3]]') — strings, numbers, booleans, or null",
			"value_input_option":         "USER_ENTERED (default — parse like a user typed: formulas evaluate, dates parse) or RAW (store the input verbatim)",
			"major_dimension":            "ROWS (default) or COLUMNS",
			"include_values_in_response": "true/false — return the updated values in the response (default false)",
		},
		Required: []string{"spreadsheet_id", "range", "values"},
	},
	{
		Name: mcp.ToolName("gsheets_append_values"), Description: "Append rows to the end of a Google Sheets table. Finds the first empty row at or below the given range and inserts there. Use this for log-style or transactional data where you want new rows added without overwriting. Returns the actual range that was updated.",
		Parameters: map[string]string{
			"spreadsheet_id":             "The spreadsheet ID",
			"range":                      "A1-notation range identifying the table (e.g. 'Sheet1' or 'Sheet1!A1') — Sheets searches downward from the top-left of this range for the next empty row.",
			"values":                     "JSON 2-D array of values (e.g. '[[\"new\",\"row\"]]')",
			"value_input_option":         "USER_ENTERED (default) or RAW",
			"insert_data_option":         "OVERWRITE (default — write into existing cells if the table extends) or INSERT_ROWS (shift rows down)",
			"major_dimension":            "ROWS (default) or COLUMNS",
			"include_values_in_response": "true/false — return the appended values (default false)",
		},
		Required: []string{"spreadsheet_id", "range", "values"},
	},
	{
		Name: mcp.ToolName("gsheets_clear_values"), Description: "Clear the cell values in a Google Sheets range, leaving formatting and other properties intact. Use a full range like 'Sheet1!A:Z' to wipe all data on a tab. To also remove formatting/banding, use gsheets_batch_update with a 'updateCells' request and the relevant field mask.",
		Parameters: map[string]string{
			"spreadsheet_id": "The spreadsheet ID",
			"range":          "A1-notation range to clear (e.g. 'Sheet1!A2:Z')",
		},
		Required: []string{"spreadsheet_id", "range"},
	},
	{
		Name: mcp.ToolName("gsheets_batch_update_values"), Description: "Update cell values for multiple A1 ranges in one Google Sheets request — a single atomic write covering many ranges. Pass 'data' as a JSON array of {range, values, major_dimension?} objects. Much faster than multiple gsheets_update_values calls.",
		Parameters: map[string]string{
			"spreadsheet_id":             "The spreadsheet ID",
			"data":                       "JSON array of ValueRange objects (e.g. '[{\"range\":\"Sheet1!A1:B1\",\"values\":[[\"a\",\"b\"]]},{\"range\":\"Sheet2!A1\",\"values\":[[\"hello\"]]}]'). Each entry may include majorDimension.",
			"value_input_option":         "USER_ENTERED (default) or RAW",
			"include_values_in_response": "true/false — return updated values (default false)",
		},
		Required: []string{"spreadsheet_id", "data"},
	},

	// ── Spreadsheet-level batchUpdate (structural changes) ──────────
	{
		Name: mcp.ToolName("gsheets_batch_update"), Description: "Apply a batch of structural edit requests to a Google Sheets spreadsheet — the full power of the Sheets API. Use for adding/deleting/renaming sheets, formatting cells, inserting columns, freezing panes, conditional formatting, charts, banding, protected ranges, named ranges, and so on. For pure value writes, use gsheets_update_values, gsheets_append_values, or gsheets_batch_update_values instead — those are simpler and don't need sheetIds.",
		Parameters: map[string]string{
			"spreadsheet_id":                  "The spreadsheet ID",
			"requests":                        "JSON array of Sheets API Request objects (e.g. [{\"addSheet\":{\"properties\":{\"title\":\"NewTab\"}}}])",
			"include_spreadsheet_in_response": "true/false — include the full spreadsheet in the response (default false)",
			"response_include_grid_data":      "true/false — include grid data if the spreadsheet is in the response (default false)",
			"response_ranges":                 "Optional comma-separated A1-notation ranges to limit the spreadsheet response to",
		},
		Required: []string{"spreadsheet_id", "requests"},
	},
}
