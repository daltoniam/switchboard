package ollama

import (
	"encoding/json"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"ollama_show_model": renderShowModelMD,
}

// renderedModel is the semantic type for RenderMarkdown's show_model output.
type renderedModel struct {
	Name          ModelName
	Family        ModelFamily
	ParameterSize ParameterSize
	Quantization  QuantizationLevel
	Format        ModelFormat
	Capabilities  []Capability
	Parameters    string
	Template      string
	LicenseLine   string
	ModifiedAt    string
}

func parseShowResponse(data []byte) (renderedModel, error) {
	var raw showResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return renderedModel{}, err
	}
	name := raw.Model
	if name == "" {
		name = raw.Details.ParentModel
	}
	return renderedModel{
		Name:          name,
		Family:        raw.Details.Family,
		ParameterSize: raw.Details.ParameterSize,
		Quantization:  raw.Details.QuantizationLevel,
		Format:        raw.Details.Format,
		Capabilities:  raw.Capabilities,
		Parameters:    raw.Parameters,
		Template:      raw.Template,
		LicenseLine:   firstNonEmptyLine(raw.License),
		ModifiedAt:    raw.ModifiedAt,
	}, nil
}

func renderShowModelMD(data []byte) (markdown.Markdown, bool) {
	m, err := parseShowResponse(data)
	if err != nil {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("ollama", "model", string(m.Name))
	b.Heading(1, string(m.Name))

	b.Attribution(
		"Family: "+string(m.Family),
		"Parameters: "+string(m.ParameterSize),
		"Quantization: "+string(m.Quantization),
		"Format: "+string(m.Format),
	)

	if len(m.Capabilities) > 0 {
		caps := make([]string, len(m.Capabilities))
		for i, c := range m.Capabilities {
			caps[i] = string(c)
		}
		b.Attribution("Capabilities: " + strings.Join(caps, ", "))
	}

	if m.Parameters != "" {
		b.BlankLine()
		b.Heading(2, "Parameters")
		b.Raw(m.Parameters + "\n")
	}

	if m.Template != "" {
		b.BlankLine()
		b.Heading(2, "Template")
		b.Raw(m.Template + "\n")
	}

	if m.LicenseLine != "" {
		b.BlankLine()
		b.Heading(2, "License")
		b.Raw(m.LicenseLine + "\n")
	}

	return b.Build(), true
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
