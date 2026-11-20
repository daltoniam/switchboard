package onepassword

import (
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_EveryReadToolHasSpec(t *testing.T) {
	a := &onepassword{}
	for _, tool := range tools {
		_, ok := a.CompactSpec(tool.Name)
		if tool.Access == mcp.AccessRead {
			assert.True(t, ok, "read tool %s needs a compaction spec", tool.Name)
		} else {
			assert.False(t, ok, "write tool %s must return its full confirmation", tool.Name)
		}
	}
}

// Shape parity: each spec, applied to the handler's output, keeps the
// fields needed to pick and read an item and drops what is only noise in a
// list.
func TestFieldCompactionSpecs_Shapes(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		payload string
		keep    []string
		drop    []string
	}{
		{"onepassword_list_vaults", `[{"id":"v1","title":"Claude","description":"agents","type":"userCreated","items":84,"updated_at":"2026-11-01T00:00:00Z"}]`,
			[]string{`"v1"`, `"Claude"`, `"items":84`}, nil},
		{"onepassword_list_items", `{"items":[{"id":"i1","title":"GitHub","category":"Login","vault_id":"v1","urls":["https://github.com"],"tags":["work"],"updated_at":"2026-11-01T00:00:00Z"}],"count":1,"total":3,"next_offset":1}`,
			[]string{`"i1"`, `"GitHub"`, `"Login"`, `"https://github.com"`, `"work"`, `"total":3`, `"next_offset":1`}, []string{"vault_id"}},
		{"onepassword_get_item", `{"id":"i1","title":"GitHub","category":"Login","vault_id":"v1","vault":"Claude","version":7,"urls":["https://github.com"],"notes":"n","fields":[{"id":"password","title":"password","type":"Concealed","value":"[REDACTED]","concealed":true,"reference":"op://v1/i1/password"}],"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-11-01T00:00:00Z"}`,
			[]string{`"[REDACTED]"`, `"op://v1/i1/password"`, `"version":7`, `"concealed":true`}, []string{"created_at"}},
		{"onepassword_resolve_secret", `{"reference":"op://v1/i1/password","value":"s3cret"}`,
			[]string{`"s3cret"`}, nil},
	}
	a := &onepassword{}
	for _, tc := range tests {
		t.Run(string(tc.tool), func(t *testing.T) {
			fields, ok := a.CompactSpec(tc.tool)
			require.True(t, ok)
			var payload any
			require.NoError(t, json.Unmarshal([]byte(tc.payload), &payload))
			out, err := json.Marshal(mcp.CompactAny(payload, fields))
			require.NoError(t, err)
			for _, want := range tc.keep {
				assert.Contains(t, string(out), want)
			}
			for _, unwanted := range tc.drop {
				assert.NotContains(t, string(out), unwanted)
			}
		})
	}
}
