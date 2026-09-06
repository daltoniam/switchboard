package paperless

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveDocumentVision(t *testing.T) {
	if os.Getenv("PAPERLESS_LIVE_TEST") != "1" {
		t.Skip("set PAPERLESS_LIVE_TEST=1 to run against a configured Paperless instance")
	}
	url := os.Getenv("PAPERLESS_URL")
	token := os.Getenv("PAPERLESS_TOKEN")
	require.NotEmpty(t, url)
	require.NotEmpty(t, token)

	integration := New()
	require.NoError(t, integration.Configure(context.Background(), mcp.Credentials{"url": url, "token": token}))

	list, err := integration.Execute(context.Background(), "paperless_list_documents", map[string]any{"page_size": 1})
	require.NoError(t, err)
	require.False(t, list.IsError, list.Data)
	var documents struct {
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal([]byte(list.Data), &documents))
	require.NotEmpty(t, documents.Results)
	documentID := documents.Results[0].ID

	thumbnail, err := integration.Execute(context.Background(), "paperless_get_document_thumbnail", map[string]any{"document_id": documentID})
	require.NoError(t, err)
	require.False(t, thumbnail.IsError, thumbnail.Data)
	require.Len(t, thumbnail.Media, 1)
	assert.Equal(t, "image/webp", thumbnail.Media[0].MIMEType)
	assert.NotEmpty(t, thumbnail.Media[0].Data)

	preview, err := integration.Execute(context.Background(), "paperless_get_document_preview", map[string]any{"document_id": documentID})
	require.NoError(t, err)
	require.False(t, preview.IsError, preview.Data)
	require.Len(t, preview.Media, 1)
	assert.NotEmpty(t, preview.Media[0].MIMEType)
	assert.NotEmpty(t, preview.Media[0].Data)

	ocr, err := integration.Execute(context.Background(), "paperless_get_document_ocr_text", map[string]any{"document_id": documentID, "limit": 1000})
	require.NoError(t, err)
	require.False(t, ocr.IsError, ocr.Data)
	assert.Contains(t, ocr.Data, `"document_id"`)
}
