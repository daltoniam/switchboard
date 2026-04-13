package ollama

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
)

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	for name := range markdownRenderers {
		_, ok := dispatch[name]
		assert.True(t, ok, "markdown renderer %s has no dispatch handler", name)
	}
}

func TestRenderMarkdown_ShowModel(t *testing.T) {
	data := []byte(`{
		"model":"gemma4:e2b",
		"details":{"family":"gemma4","parameter_size":"5.1B","quantization_level":"Q8_0","format":"gguf","parent_model":"gemma4:e2b-it-q8_0"},
		"capabilities":["completion","vision","tools"],
		"parameters":"temperature 1\ntop_k 64",
		"template":"{{ .Prompt }}",
		"license":"                                Apache License\n                           Version 2.0, January 2004",
		"modified_at":"2026-04-11"
	}`)

	o := New().(*ollama)
	md, ok := o.RenderMarkdown("ollama_show_model", data)
	assert.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "<!-- ollama:model=gemma4:e2b -->")
	assert.Contains(t, s, "# gemma4:e2b")
	assert.Contains(t, s, "Family: gemma4")
	assert.Contains(t, s, "Parameters: 5.1B")
	assert.Contains(t, s, "Quantization: Q8_0")
	assert.Contains(t, s, "Format: gguf")
	assert.Contains(t, s, "completion, vision, tools")
	assert.Contains(t, s, "temperature 1")
	assert.Contains(t, s, "top_k 64")
	assert.Contains(t, s, "{{ .Prompt }}")
	assert.Contains(t, s, "Apache License")
	assert.NotContains(t, s, "Version 2.0, January 2004", "should only include first non-empty line of license")
}

func TestRenderMarkdown_ShowModel_MinimalFields(t *testing.T) {
	data := []byte(`{
		"details":{"family":"llama","parameter_size":"7B","quantization_level":"Q4_0","format":"gguf"},
		"capabilities":[],
		"parameters":"",
		"template":"",
		"license":"",
		"modified_at":""
	}`)

	o := New().(*ollama)
	md, ok := o.RenderMarkdown("ollama_show_model", data)
	assert.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Family: llama")
	assert.NotContains(t, s, "## Parameters", "empty parameters should be omitted")
	assert.NotContains(t, s, "## Template", "empty template should be omitted")
	assert.NotContains(t, s, "## License", "empty license should be omitted")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	o := New().(*ollama)
	_, ok := o.RenderMarkdown("ollama_list_models", nil)
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	o := New().(*ollama)
	_, ok := o.RenderMarkdown("ollama_show_model", []byte(`{invalid`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InterfaceCompliance(t *testing.T) {
	var _ mcp.MarkdownIntegration = New().(*ollama)
}

func TestFirstNonEmptyLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal", "Apache License\nVersion 2.0", "Apache License"},
		{"leading whitespace", "   \n   Apache License", "Apache License"},
		{"leading blank lines", "\n\n\nMIT License", "MIT License"},
		{"only whitespace", "   \n   \n   ", ""},
		{"empty", "", ""},
		{"indented first line", "                                Apache License", "Apache License"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, firstNonEmptyLine(tt.input))
		})
	}
}
