package suno

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("suno_get_generation"): {
		"data.taskId", "data.status", "data.type",
		"data.response.sunoData[].id", "data.response.sunoData[].audioUrl",
		"data.response.sunoData[].streamAudioUrl", "data.response.sunoData[].imageUrl",
		"data.response.sunoData[].title", "data.response.sunoData[].tags",
		"data.response.sunoData[].modelName", "data.response.sunoData[].duration",
		"data.response.sunoData[].createTime",
		"data.errorCode", "data.errorMessage",
	},
	mcp.ToolName("suno_get_credits"): {
		"code", "msg", "data",
	},
	mcp.ToolName("suno_get_lyrics"): {
		"data.taskId", "data.status", "data.type",
		"data.response",
		"data.errorCode", "data.errorMessage",
	},
	mcp.ToolName("suno_get_aligned_lyrics"): {
		"code", "msg", "data",
	},
	mcp.ToolName("suno_get_stem_separation"): {
		"data.taskId", "data.status", "data.type",
		"data.response",
		"data.errorCode", "data.errorMessage",
	},
	mcp.ToolName("suno_get_wav_conversion"): {
		"data.taskId", "data.status", "data.type",
		"data.response",
		"data.errorCode", "data.errorMessage",
	},
	mcp.ToolName("suno_get_video"): {
		"data.taskId", "data.status", "data.type",
		"data.response",
		"data.errorCode", "data.errorMessage",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("suno: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
