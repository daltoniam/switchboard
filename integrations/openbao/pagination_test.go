package openbao

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListFinalPageHasExplicitContinuation(t *testing.T) {
	adapter := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"data":{"keys":["final"]}}`)
	})
	result := execute(t, adapter, "openbao_list_secret_keys", map[string]any{"limit": 1})
	require.False(t, result.IsError, result.Data)
	require.Contains(t, result.Data, `"has_more":false`)
	require.Contains(t, result.Data, `"next_after":""`)
}
