package notion

import (
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// extractRecord pulls a single record from a v3 recordMap response.
// The response shape is: { "<table>": { "<id>": { "value": { ... } } } }
func extractRecord(data json.RawMessage, table, id string) (map[string]any, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	tableData, ok := top[table]
	if !ok {
		// Check inside recordMap wrapper
		if rm, exists := top["recordMap"]; exists {
			return extractRecord(rm, table, id)
		}
		return nil, fmt.Errorf("table %q not found in response", table)
	}

	var records map[string]json.RawMessage
	if err := json.Unmarshal(tableData, &records); err != nil {
		return nil, fmt.Errorf("parse table %q: %w", table, err)
	}

	recData, ok := records[id]
	if !ok {
		return nil, fmt.Errorf("record %q not found in table %q", id, table)
	}

	var wrapper struct {
		Value map[string]any `json:"value"`
	}
	if err := json.Unmarshal(recData, &wrapper); err != nil {
		return nil, fmt.Errorf("parse record %q: %w", id, err)
	}
	if wrapper.Value == nil {
		return nil, fmt.Errorf("record %q has nil value", id)
	}
	return wrapper.Value, nil
}

// extractAllRecords pulls all records from a table in a v3 recordMap response.
// Returns them as a slice in iteration order.
func extractAllRecords(data json.RawMessage, table string) ([]map[string]any, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	tableData, ok := top[table]
	if !ok {
		if rm, exists := top["recordMap"]; exists {
			return extractAllRecords(rm, table)
		}
		return nil, fmt.Errorf("table %q not found in response", table)
	}

	var records map[string]json.RawMessage
	if err := json.Unmarshal(tableData, &records); err != nil {
		return nil, fmt.Errorf("parse table %q: %w", table, err)
	}

	result := make([]map[string]any, 0, len(records))
	for _, raw := range records {
		var wrapper struct {
			Value map[string]any `json:"value"`
		}
		if err := json.Unmarshal(raw, &wrapper); err != nil {
			continue
		}
		if wrapper.Value == nil {
			continue
		}
		result = append(result, wrapper.Value)
	}
	return result, nil
}

// recordMapResult extracts a single record and returns it as a ToolResult.
func recordMapResult(data json.RawMessage, table, id string) (*mcp.ToolResult, error) {
	record, err := extractRecord(data, table, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(record)
}
