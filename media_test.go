package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaResult(t *testing.T) {
	result, err := MediaResult(`{"document_id":7}`, []byte("image"), "image/webp", "document-7.webp")
	require.NoError(t, err)
	assert.Equal(t, `{"document_id":7}`, result.Data)
	require.Len(t, result.Media, 1)
	assert.Equal(t, []byte("image"), result.Media[0].Data)
	assert.Equal(t, "image/webp", result.Media[0].MIMEType)
	assert.Equal(t, "document-7.webp", result.Media[0].Name)
}
