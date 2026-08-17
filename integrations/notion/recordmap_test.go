package notion

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- extractRecord ---

func TestExtractRecord_ReturnsValueFromRecordMap(t *testing.T) {
	data := json.RawMessage(`{
		"block": {
			"abc-123": {
				"value": {"id": "abc-123", "type": "page", "properties": {"title": [["Hello"]]}}
			}
		}
	}`)

	record, err := extractRecord(data, "block", "abc-123")
	require.NoError(t, err)
	assert.Equal(t, "abc-123", record["id"])
	assert.Equal(t, "page", record["type"])
}

func TestExtractRecord_UnwrapsRecordMapWrapper(t *testing.T) {
	data := json.RawMessage(`{
		"recordMap": {
			"collection": {
				"col-1": {
					"value": {"id": "col-1", "name": [["My DB"]]}
				}
			}
		}
	}`)

	record, err := extractRecord(data, "collection", "col-1")
	require.NoError(t, err)
	assert.Equal(t, "col-1", record["id"])
}

func TestExtractRecord_ReturnsErrorWhenTableMissing(t *testing.T) {
	data := json.RawMessage(`{"block": {}}`)

	_, err := extractRecord(data, "collection", "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `table "collection" not found`)
}

func TestExtractRecord_ReturnsErrorWhenIDMissing(t *testing.T) {
	data := json.RawMessage(`{"block": {"other-id": {"value": {"id": "other"}}}}`)

	_, err := extractRecord(data, "block", "abc-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `record "abc-123" not found`)
}

func TestExtractRecord_ReturnsErrorWhenValueIsNull(t *testing.T) {
	data := json.RawMessage(`{"block": {"abc": {"value": null}}}`)

	_, err := extractRecord(data, "block", "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil value")
}

// --- extractAllRecords ---

func TestExtractAllRecords_ReturnsAllValuesFromTable(t *testing.T) {
	data := json.RawMessage(`{
		"notion_user": {
			"u1": {"value": {"id": "u1", "name": "Alice"}},
			"u2": {"value": {"id": "u2", "name": "Bob"}}
		}
	}`)

	records, err := extractAllRecords(data, "notion_user")
	require.NoError(t, err)
	assert.Len(t, records, 2)

	ids := map[string]bool{}
	for _, r := range records {
		ids[r["id"].(string)] = true
	}
	assert.True(t, ids["u1"])
	assert.True(t, ids["u2"])
}

func TestExtractAllRecords_UnwrapsRecordMapWrapper(t *testing.T) {
	data := json.RawMessage(`{
		"recordMap": {
			"block": {
				"b1": {"value": {"id": "b1"}}
			}
		}
	}`)

	records, err := extractAllRecords(data, "block")
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

func TestExtractAllRecords_SkipsRecordsWithNilValue(t *testing.T) {
	data := json.RawMessage(`{
		"block": {
			"b1": {"value": {"id": "b1"}},
			"b2": {"value": null}
		}
	}`)

	records, err := extractAllRecords(data, "block")
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

// --- recordMap v3 double-nested value ---

func TestExtractRecord_UnwrapsDoubleNestedValue(t *testing.T) {
	// Notion recordMap version 3: {value: {value: {id, type, ...}}}
	data := json.RawMessage(`{
		"recordMap": {
			"block": {
				"abc-123": {
					"spaceId": "space-1",
					"value": {"value": {"id": "abc-123", "type": "page", "properties": {"title": [["Hello"]]}}}
				}
			}
		}
	}`)

	record, err := extractRecord(data, "block", "abc-123")
	require.NoError(t, err)
	assert.Equal(t, "abc-123", record["id"])
	assert.Equal(t, "page", record["type"])
}

func TestExtractAllRecords_UnwrapsDoubleNestedValues(t *testing.T) {
	// Notion recordMap version 3 with double-nested values
	data := json.RawMessage(`{
		"recordMap": {
			"__version__": 3,
			"block": {
				"page-1": {"spaceId": "s1", "value": {"value": {"id": "page-1", "type": "page", "properties": {"title": [["My Page"]]}}}},
				"blk-1": {"spaceId": "s1", "value": {"value": {"id": "blk-1", "type": "text", "properties": {"title": [["Hello"]]}}}}
			}
		}
	}`)

	records, err := extractAllRecords(data, "block")
	require.NoError(t, err)
	assert.Len(t, records, 2)

	ids := map[string]bool{}
	for _, r := range records {
		ids[r["id"].(string)] = true
		// Verify the inner fields are directly accessible (not double-nested)
		assert.NotNil(t, r["type"], "type should be accessible at top level")
	}
	assert.True(t, ids["page-1"])
	assert.True(t, ids["blk-1"])
}

func TestExtractAllRecords_HandlesOldAndNewFormatMixed(t *testing.T) {
	// Edge case: mix of old {value: {id, ...}} and new {value: {value: {id, ...}}}
	data := json.RawMessage(`{
		"block": {
			"old-format": {"value": {"id": "old-format", "type": "text"}},
			"new-format": {"value": {"value": {"id": "new-format", "type": "page"}}}
		}
	}`)

	records, err := extractAllRecords(data, "block")
	require.NoError(t, err)
	assert.Len(t, records, 2)

	ids := map[string]bool{}
	for _, r := range records {
		ids[r["id"].(string)] = true
	}
	assert.True(t, ids["old-format"])
	assert.True(t, ids["new-format"])
}

func TestExtractAllRecords_ReturnsErrorWhenTableMissing(t *testing.T) {
	data := json.RawMessage(`{"block": {}}`)

	_, err := extractAllRecords(data, "notion_user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `table "notion_user" not found`)
}

// --- recordMapResult ---

func TestRecordMapResult_ReturnsToolResultWithRecordData(t *testing.T) {
	data := json.RawMessage(`{
		"block": {
			"page-1": {
				"value": {"id": "page-1", "type": "page"}
			}
		}
	}`)

	result, err := recordMapResult(data, "block", "page-1")
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "page-1")
	assert.Contains(t, result.Data, "page")
}

func TestRecordMapResult_ReturnsErrorResultWhenRecordMissing(t *testing.T) {
	data := json.RawMessage(`{"block": {}}`)

	result, err := recordMapResult(data, "block", "missing")
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
}
