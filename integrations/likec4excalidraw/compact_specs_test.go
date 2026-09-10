package likec4excalidraw

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
	var specFile compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &specFile))
	assert.Equal(t, len(specFile.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_OnlyReadTools(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		assert.NotContains(t, toolName, "_create_")
		assert.NotContains(t, toolName, "_update_")
		assert.NotContains(t, toolName, "_delete_")
	}
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	tests := []struct {
		name     string
		toolName mcp.ToolName
		payload  string
		want     string
	}{
		{
			name:     "architecture",
			toolName: "likec4excalidraw_get_architecture",
			payload:  `{"specification":{"elementKinds":["service"],"relationshipKinds":["uses"],"tags":["core"]},"elements":[{"fqn":"shop.api","kind":"service","title":"API","description":"Backend","technology":"Go","parent":"shop","tags":["core"],"metadata":{"noise":"drop"}}],"relationships":[{"id":"rel_1","source":"shop.web","target":"shop.api","kind":"uses","title":"Calls","technology":"HTTP","tags":["core"],"metadata":{"noise":"drop"}}],"views":[{"id":"index","title":"Landscape","includes":["shop.*"]}],"diagnostics":[{"severity":"warning","message":"Example","source":"index.c4"}]}`,
			want:     "shop.api",
		},
		{
			name:     "scene",
			toolName: "likec4excalidraw_get_scene",
			payload:  `{"version":1,"activeViewId":"index","elements":[{"id":"element-1","type":"rectangle","x":80,"y":80,"width":180,"height":100,"frameId":"view-index","isDeleted":false,"customData":{"likec4":{"version":1,"role":"element","fqn":"shop.api","kind":"service","title":"API"},"noise":true}}],"appState":{"noise":true},"files":{"noise":true}}`,
			want:     "shop.api",
		},
		{
			name:     "diagnostics",
			toolName: "likec4excalidraw_get_diagnostics",
			payload:  `[{"severity":"error","message":"Broken relation","source":"index.c4","noise":true}]`,
			want:     "Broken relation",
		},
		{
			name:     "validate",
			toolName: "likec4excalidraw_validate",
			payload:  `{"diagnostics":[{"severity":"info","message":"Valid","source":"generated.c4"}],"dsl":"model {}","noise":true}`,
			want:     "model {}",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[test.toolName]
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(test.payload), fields)
			require.NoError(t, err)
			assert.Contains(t, string(compacted), test.want)
			assert.NotContains(t, string(compacted), "noise")
		})
	}
}
