package project

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestList_InvalidProjectsOnlyOnFirstPage(t *testing.T) {
	store := newTestCatalog(t)
	ctx := context.Background()
	// Seed past page size with valid projects, plus a couple of invalid sources.
	for i := 0; i < pageSizeDefault+5; i++ {
		name := fmt.Sprintf("valid-%03d", i)
		_, err := store.Create(ctx, CreateRequest{Definition: Definition{Version: "1", Name: name}})
		require.NoError(t, err)
	}
	proj := filepath.Join(store.ConfigDir(), "projects")
	require.NoError(t, os.WriteFile(filepath.Join(proj, "zzz-bad.project.json"), []byte(`{not json`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(proj, "aaa-bad.project.json"), []byte(`{"version":"9","name":"aaa-bad"}`), 0600))
	require.NoError(t, store.Load())

	first, err := store.List(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, first.NextCursor)
	require.NotEmpty(t, first.InvalidProjects, "invalids should appear on first page")
	invalidIDs := map[string]int{}
	for _, inv := range first.InvalidProjects {
		invalidIDs[string(inv.ProjectID)]++
	}

	// Walk remaining pages; invalids must not reappear.
	cursor := first.NextCursor
	for cursor != "" {
		page, err := store.List(ctx, cursor)
		require.NoError(t, err)
		assert.Empty(t, page.InvalidProjects, "invalids must only ride the first page")
		if page.NextCursor == "" || page.NextCursor == cursor {
			break
		}
		cursor = page.NextCursor
	}
	assert.GreaterOrEqual(t, invalidIDs["zzz-bad"]+invalidIDs["aaa-bad"], 1)
	for id, n := range invalidIDs {
		assert.Equal(t, 1, n, "invalid id %s duplicated", id)
	}
}
