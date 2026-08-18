package server

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestProjectCatalog_E2ECreateReadDelete(t *testing.T) {
	httpSrv, store := newCatalogTestServer(t, true)
	client := newCatalogClient(t, httpSrv.URL+"/project-catalog/mcp")

	created, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_create",
		Arguments: map[string]any{
			"definition": map[string]any{"version": "1", "name": "lifecycle", "resources": map[string]any{}},
		},
	})
	require.NoError(t, err)
	require.False(t, created.IsError)

	listed, err := client.ListResources(context.Background(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, listed.Resources)

	got, err := store.Get(context.Background(), "lifecycle")
	require.NoError(t, err)

	_, err = client.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "project_delete",
		Arguments: map[string]any{
			"projectId":              "lifecycle",
			"expectedSourceRevision": string(got.SourceRevision),
		},
	})
	require.NoError(t, err)

	_, err = store.GetRevision(context.Background(), "lifecycle", got.Revision)
	require.NoError(t, err)
}
