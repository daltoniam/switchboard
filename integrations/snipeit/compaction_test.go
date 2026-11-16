package snipeit

import (
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestCompactionPreservesNullableReferences(t *testing.T) {
	adapter := New().(*snipeit)
	for _, tool := range []mcp.ToolName{"snipeit_list_assets", "snipeit_get_asset_by_tag"} {
		t.Run(string(tool), func(t *testing.T) {
			fields, ok := adapter.CompactSpec(tool)
			require.True(t, ok)
			var raw any
			require.NoError(t, json.Unmarshal([]byte(`{"total":1,"rows":[{"id":7,"name":"Ready asset","asset_tag":"IT-7","serial":"SERIAL-7","model":{"id":2,"name":"Laptop"},"status_label":{"id":3,"name":"Ready"},"location":null,"assigned_to":null}],"next_offset":null}`), &raw))
			projected := mcp.CompactAny(raw, fields).(map[string]any)
			require.Equal(t, raw.(map[string]any)["rows"], projected["rows"])
		})
	}
}
