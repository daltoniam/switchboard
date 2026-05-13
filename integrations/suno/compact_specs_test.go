package suno

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
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

func TestFieldCompactionSpec_ReturnsFieldsForGetTool(t *testing.T) {
	s := &suno{}
	fields, ok := s.CompactSpec("suno_get_generation")
	require.True(t, ok, "suno_get_generation should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	s := &suno{}
	_, ok := s.CompactSpec("suno_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	s := &suno{}
	_, ok := s.CompactSpec("suno_generate_music")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestFieldCompactionSpec_NoSpecsOnMutationTools(t *testing.T) {
	mutationTools := []string{
		"suno_generate_music", "suno_extend_music", "suno_generate_lyrics",
		"suno_separate_stems", "suno_convert_wav", "suno_cover_audio",
		"suno_upload_extend", "suno_add_vocals", "suno_add_instrumental",
		"suno_generate_mashup", "suno_generate_persona", "suno_generate_video",
		"suno_generate_midi",
	}
	for _, name := range mutationTools {
		_, ok := fieldCompactionSpecs[mcp.ToolName(name)]
		assert.False(t, ok, "mutation tool %q should not have compaction spec", name)
	}
}
