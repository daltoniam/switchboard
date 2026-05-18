package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadToolsYAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        []ToolDefinition
		wantErr     bool
		errContains string // optional substring check on error message
	}{
		{
			name: "valid minimal",
			input: `
version: 1
tools:
  my_tool:
    description: "Does something useful."
    parameters:
      query:
        description: "The search query."
`,
			want: []ToolDefinition{
				{
					Name:        ToolName("my_tool"),
					Description: "Does something useful.",
					Parameters: []Parameter{
						{Name: ParamName("query"), Description: "The search query."},
					},
				},
			},
		},
		{
			name: "valid with required propagation",
			input: `
version: 1
tools:
  create_item:
    description: "Creates an item."
    parameters:
      name:
        description: "Item name."
        required: true
      color:
        description: "Item color."
`,
			want: []ToolDefinition{
				{
					Name:        ToolName("create_item"),
					Description: "Creates an item.",
					Parameters: []Parameter{
						{Name: ParamName("name"), Description: "Item name.", Required: true},
						{Name: ParamName("color"), Description: "Item color."},
					},
				},
			},
		},
		{
			name: "missing version",
			input: `
tools:
  my_tool:
    description: "Does something."
`,
			wantErr: true,
		},
		{
			name: "unsupported version",
			input: `
version: 2
tools:
  my_tool:
    description: "Does something."
`,
			wantErr: true,
		},
		{
			name: "unknown top-level key",
			input: `
version: 1
toools:
  my_tool:
    description: "Does something."
`,
			wantErr: true,
		},
		{
			name: "unknown nested key",
			input: `
version: 1
tools:
  my_tool:
    descripton: "Typo in key."
`,
			wantErr: true,
		},
		{
			name: "parameter declaration order preserved",
			input: `
version: 1
tools:
  ordered_tool:
    description: "Order matters."
    parameters:
      zebra:
        description: "Last alphabetically."
      mango:
        description: "Middle alphabetically."
      apple:
        description: "First alphabetically."
`,
			want: []ToolDefinition{
				{
					Name:        ToolName("ordered_tool"),
					Description: "Order matters.",
					Parameters: []Parameter{
						{Name: ParamName("zebra"), Description: "Last alphabetically."},
						{Name: ParamName("mango"), Description: "Middle alphabetically."},
						{Name: ParamName("apple"), Description: "First alphabetically."},
					},
				},
			},
		},
		{
			name: "empty parameters",
			input: `
version: 1
tools:
  no_params:
    description: "No parameters needed."
    parameters: {}
`,
			// parameters: {} decodes as an empty MappingNode, so we get an empty non-nil slice.
			want: []ToolDefinition{
				{
					Name:        ToolName("no_params"),
					Description: "No parameters needed.",
					Parameters:  []Parameter{},
				},
			},
		},
		{
			name: "parameters null",
			input: `
version: 1
tools:
  nullparams:
    description: "Null parameters."
    parameters: ~
`,
			want: []ToolDefinition{
				{
					Name:        ToolName("nullparams"),
					Description: "Null parameters.",
					Parameters:  nil,
				},
			},
		},
		{
			name: "parameters missing",
			input: `
version: 1
tools:
  noparams:
    description: "No parameters key."
`,
			want: []ToolDefinition{
				{
					Name:        ToolName("noparams"),
					Description: "No parameters key.",
					Parameters:  nil,
				},
			},
		},
		{
			name: "duplicate tool name rejected",
			input: `
version: 1
tools:
  my_tool:
    description: "First."
  my_tool:
    description: "Second."
`,
			wantErr:     true,
			errContains: "duplicate",
		},
		{
			name: "empty tools rejected",
			input: `
version: 1
tools: {}
`,
			wantErr:     true,
			errContains: "non-empty",
		},
		{
			name: "tools null rejected",
			input: `
version: 1
tools: ~
`,
			wantErr:     true,
			errContains: "non-empty",
		},
		{
			name: "missing tools rejected",
			input: `
version: 1
`,
			wantErr:     true,
			errContains: "non-empty",
		},
		{
			name: "parameter-level typo rejected",
			input: `
version: 1
tools:
  my_tool:
    description: "Does something."
    parameters:
      query:
        description: "A query."
        requird: true
`,
			wantErr:     true,
			errContains: "requird",
		},
		{
			name: "parameter-level unknown key rejected",
			input: `
version: 1
tools:
  my_tool:
    description: "Does something."
    parameters:
      query:
        description: "A query."
        frobnicate: foo
`,
			wantErr:     true,
			errContains: "frobnicate",
		},
		{
			name: "tool-level unknown key rejected",
			input: `
version: 1
tools:
  my_tool:
    description_x: "Typo at tool level."
`,
			wantErr:     true,
			errContains: "description_x",
		},
		{
			name: "required false rejected",
			input: `
version: 1
tools:
  my_tool:
    description: "Does something."
    parameters:
      query:
        description: "A query."
        required: false
`,
			wantErr:     true,
			errContains: "required: false is not allowed",
		},
		{
			name:        "empty input rejected",
			input:       "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name: "multi-document rejected",
			input: `
version: 1
tools:
  my_tool:
    description: "First doc."
---
version: 1
tools:
  other_tool:
    description: "Second doc."
`,
			wantErr:     true,
			errContains: "multi-document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadToolsYAML([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadToolsYAML_MissingVersionIsZero(t *testing.T) {
	// Version field absent decodes as 0, which is != 1 and must return an error.
	input := `
tools:
  my_tool:
    description: "Does something."
`
	_, err := LoadToolsYAML([]byte(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version 0")
}

func TestMustLoadToolsYAML_PanicsOnBadYAML(t *testing.T) {
	require.Panics(t, func() {
		MustLoadToolsYAML([]byte("version: 99\ntools: {}"))
	})
}

func TestLoadToolsYAML_MultiToolDeclarationOrderPreserved(t *testing.T) {
	input := `
version: 1
tools:
  zebra:
    description: "Third alphabetically."
  mango:
    description: "Second alphabetically."
  apple:
    description: "First alphabetically."
`
	got, err := LoadToolsYAML([]byte(input))
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, ToolName("zebra"), got[0].Name)
	assert.Equal(t, ToolName("mango"), got[1].Name)
	assert.Equal(t, ToolName("apple"), got[2].Name)
}
