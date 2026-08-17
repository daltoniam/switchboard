package ollama

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// Lists
	"ollama_list_models": {
		"models[].name", "models[].size", "models[].modified_at",
		"models[].details.parameter_size", "models[].details.quantization_level", "models[].details.family",
	},
	"ollama_list_running": {
		"models[].name", "models[].size", "models[].size_vram",
		"models[].context_length", "models[].expires_at", "models[].details.parameter_size",
	},
	// Inference — strip timing stats, keep content
	"ollama_chat": {
		"model", "message", "done_reason",
	},
	"ollama_generate": {
		"model", "response", "thinking", "done_reason",
	},
	"ollama_embed": {
		"model", "embeddings",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("ollama: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
