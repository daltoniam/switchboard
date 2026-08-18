package project

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCatalog(t *testing.T) *Store {
	t.Helper()
	store := NewStore(t.TempDir())
	require.NoError(t, store.Load())
	return store
}

func TestCatalog_ResolveByID(t *testing.T) {
	store := newTestCatalog(t)
	_, err := store.Create(context.Background(), CreateRequest{Definition: Definition{Version: "1", Name: "acme", Description: "x"}})
	require.NoError(t, err)
	first, err := store.Resolve(context.Background(), ResolveRequest{ProjectID: "acme"})
	require.NoError(t, err)
	second, err := store.Resolve(context.Background(), ResolveRequest{ProjectID: "acme"})
	require.NoError(t, err)
	assert.Equal(t, first.Revision, second.Revision)
	assert.Equal(t, "x", first.Definition.Description)
}

func TestCatalog_ExternalFileRefresh(t *testing.T) {
	store := newTestCatalog(t)
	root := store.ConfigDir()
	proj := filepath.Join(root, "projects")
	require.NoError(t, os.MkdirAll(proj, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(proj, "ext.project.json"), []byte(`{"version":"1","name":"ext","description":"a"}`), 0600))
	page, err := store.List(context.Background(), "")
	require.NoError(t, err)
	require.NotEmpty(t, page.Projects)
	assert.Equal(t, "a", page.Projects[0].Description)
}

func TestCatalog_PatchConflict(t *testing.T) {
	store := newTestCatalog(t)
	created, err := store.Create(context.Background(), CreateRequest{Definition: Definition{Version: "1", Name: "c"}})
	require.NoError(t, err)
	_, err = store.Patch(context.Background(), PatchRequest{
		ProjectID: "c", ExpectedSourceRevision: created.SourceRevision,
		Patch: json.RawMessage(`{"description":"n"}`),
	})
	require.NoError(t, err)
	_, err = store.Patch(context.Background(), PatchRequest{
		ProjectID: "c", ExpectedSourceRevision: created.SourceRevision,
		Patch: json.RawMessage(`{"description":"stale"}`),
	})
	assert.True(t, IsCode(err, CodeRevisionConflict))
}

func TestGetRevision_RejectsInvalidProjectID(t *testing.T) {
	store := NewStore(t.TempDir())
	require.NoError(t, store.Load())
	_, err := store.GetRevision(context.Background(), ProjectID(".."), Revision("sha256:"+strings.Repeat("a", 64)))
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidDefinition) || IsCode(err, CodeProjectNotFound))
}
