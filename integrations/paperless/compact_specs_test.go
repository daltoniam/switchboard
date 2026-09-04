package paperless

import (
	"testing"

	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs(t *testing.T) {
	var specFile compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &specFile))
	require.NotEmpty(t, fieldCompactionSpecs)
	assert.Equal(t, len(specFile.Tools), len(fieldCompactionSpecs))

	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecExcludesMutations(t *testing.T) {
	p := &paperless{}
	fields, ok := p.CompactSpec("paperless_list_documents")
	assert.True(t, ok)
	assert.NotEmpty(t, fields)
	_, ok = p.CompactSpec("paperless_update_document")
	assert.False(t, ok)
}
