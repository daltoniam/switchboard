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

func TestGetRevision_ReadFailureIsInternalNotNotFound(t *testing.T) {
	store := newTestCatalog(t)
	ctx := context.Background()
	snap, err := store.Create(ctx, CreateRequest{Definition: Definition{Version: "1", Name: "pin"}})
	require.NoError(t, err)

	path := filepath.Join(store.ConfigDir(), "revisions", "pin", snap.Revision.DigestHex()+".json")
	require.FileExists(t, path)
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err = store.GetRevision(ctx, "pin", snap.Revision)
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInternalError), "got %v", err)
	assert.False(t, IsCode(err, CodeProjectNotFound), "I/O must not look like missing pin")
}

func TestGet_ArchivesLiveRevision(t *testing.T) {
	store := newTestCatalog(t)
	ctx := context.Background()
	// Seed via external file so Create/Resolve do not pre-archive.
	proj := filepath.Join(store.ConfigDir(), "projects")
	require.NoError(t, os.MkdirAll(proj, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(proj, "live.project.json"), []byte(`{"version":"1","name":"live","description":"d"}`), 0o600))
	require.NoError(t, store.Load())

	got, err := store.Get(ctx, "live")
	require.NoError(t, err)
	require.NotEmpty(t, got.Revision)
	path := filepath.Join(store.ConfigDir(), "revisions", "live", got.Revision.DigestHex()+".json")
	require.FileExists(t, path)

	// Concurrent list must still succeed (Get no longer holds mu across archive I/O).
	page, err := store.List(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, page.Projects)
}

func TestCatalog_KnownResourceIDsRoundTripAndCAS(t *testing.T) {
	store := newTestCatalog(t)
	ctx := context.Background()
	created, err := store.Create(ctx, CreateRequest{Definition: Definition{
		Version: "1", Name: "obs", Description: "before",
		KnownResourceIDs: []string{"repo", "worktree-root"},
		Additional:       map[string]json.RawMessage{"legacy": json.RawMessage(`{"keep":true}`)},
	}})
	require.NoError(t, err)
	assert.Equal(t, []string{"repo", "worktree-root"}, created.Definition.KnownResourceIDs)

	got, err := store.Get(ctx, "obs")
	require.NoError(t, err)
	assert.Equal(t, []string{"repo", "worktree-root"}, got.Definition.KnownResourceIDs)

	listed, err := store.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, listed.Projects, 1)

	replaced, err := store.Replace(ctx, ReplaceRequest{
		ProjectID: "obs", ExpectedSourceRevision: created.SourceRevision,
		Definition: Definition{Version: "1", Name: "obs", Description: "replaced", KnownResourceIDs: []string{"repo"}},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"repo"}, replaced.Definition.KnownResourceIDs)
	assert.JSONEq(t, `{"keep":true}`, string(replaced.Definition.Additional["legacy"]))

	_, err = store.Replace(ctx, ReplaceRequest{
		ProjectID: "obs", ExpectedSourceRevision: created.SourceRevision,
		Definition: Definition{Version: "1", Name: "obs", KnownResourceIDs: []string{"stale"}},
	})
	assert.True(t, IsCode(err, CodeRevisionConflict))

	rev, err := store.GetRevision(ctx, "obs", replaced.Revision)
	require.NoError(t, err)
	assert.Equal(t, []string{"repo"}, rev.Definition.KnownResourceIDs)

	empty := ""
	patched, err := store.PatchDefinition(ctx, TypedPatchRequest{
		ProjectID: "obs", ExpectedSourceRevision: replaced.SourceRevision,
		Patch: DefinitionPatch{Description: &empty},
	})
	require.NoError(t, err)
	assert.Empty(t, patched.Definition.Description)
	assert.Equal(t, []string{"repo"}, patched.Definition.KnownResourceIDs)

	cleared, err := store.Patch(ctx, PatchRequest{
		ProjectID: "obs", ExpectedSourceRevision: patched.SourceRevision,
		Patch: json.RawMessage(`{"known_resource_ids":[]}`),
	})
	require.NoError(t, err)
	assert.Empty(t, cleared.Definition.KnownResourceIDs)
}

func TestCatalog_KnownResourceIDsRejectMalformed(t *testing.T) {
	store := newTestCatalog(t)
	ctx := context.Background()
	tests := []struct {
		name string
		ids  []string
	}{
		{name: "empty id", ids: []string{""}},
		{name: "bad pattern", ids: []string{"-bad"}},
		{name: "duplicate", ids: []string{"repo", "repo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Create(ctx, CreateRequest{Definition: Definition{
				Version: "1", Name: "bad-" + tt.name, KnownResourceIDs: tt.ids,
			}})
			require.Error(t, err)
			assert.True(t, IsCode(err, CodeInvalidDefinition), "%v", err)
		})
	}
}

func TestCatalog_KnownResourceIDsSurviveResourceDelete(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	require.NoError(t, store.Load())
	work := awmPresence{root: root}
	store.SetResourcePresence(work)
	ctx := context.Background()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "awm", "resources"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "awm", "resources", "repo.json"), []byte(`{"version":"1","resource_id":"repo","uri":"file:///tmp/r","kind":"git-repository"}`), 0o600))

	created, err := store.Create(ctx, CreateRequest{Definition: Definition{
		Version: "1", Name: "obs", KnownResourceIDs: []string{"repo"},
	}})
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(root, "awm", "resources", "repo.json")))

	got, err := store.Get(ctx, "obs")
	require.NoError(t, err)
	assert.Equal(t, []string{"repo"}, got.Definition.KnownResourceIDs)
	rev, err := store.GetRevision(ctx, "obs", created.Revision)
	require.NoError(t, err)
	assert.Equal(t, []string{"repo"}, rev.Definition.KnownResourceIDs)
}

type awmPresence struct{ root string }

func (p awmPresence) ResourceExists(_ context.Context, id string) (bool, error) {
	_, err := os.Stat(filepath.Join(p.root, "awm", "resources", id+".json"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func TestCatalog_KnownResourceIDsPresenceIsAdvisory(t *testing.T) {
	store := newTestCatalog(t)
	ctx := context.Background()
	store.SetResourcePresence(resourcePresenceFunc(func(_ context.Context, id string) (bool, error) {
		return id == "repo", nil
	}))

	_, err := store.Create(ctx, CreateRequest{Definition: Definition{
		Version: "1", Name: "missing-ref", KnownResourceIDs: []string{"missing"},
	}})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidReference), "%v", err)

	created, err := store.Create(ctx, CreateRequest{Definition: Definition{
		Version: "1", Name: "ok-ref", KnownResourceIDs: []string{"repo"},
	}})
	require.NoError(t, err)
	assert.Equal(t, []string{"repo"}, created.Definition.KnownResourceIDs)
}

type resourcePresenceFunc func(context.Context, string) (bool, error)

func (f resourcePresenceFunc) ResourceExists(ctx context.Context, id string) (bool, error) {
	return f(ctx, id)
}

func TestCatalog_TypedReplaceAndPatchPreserveCompatibilityFields(t *testing.T) {
	store := newTestCatalog(t)
	ctx := context.Background()
	created, err := store.Create(ctx, CreateRequest{Definition: Definition{
		Version: "1", Name: "typed", Description: "before",
		Additional: map[string]json.RawMessage{"resources": json.RawMessage(`{"main":{"type":"repo"}}`)},
	}})
	require.NoError(t, err)

	replaced, err := store.Replace(ctx, ReplaceRequest{
		ProjectID: "typed", ExpectedSourceRevision: created.SourceRevision,
		Definition: Definition{Version: "1", Name: "typed", Description: "replaced"},
	})
	require.NoError(t, err)
	assert.Equal(t, "replaced", replaced.Definition.Description)
	assert.JSONEq(t, `{"main":{"type":"repo"}}`, string(replaced.Definition.Additional["resources"]))

	empty := ""
	patched, err := store.PatchDefinition(ctx, TypedPatchRequest{
		ProjectID: "typed", ExpectedSourceRevision: replaced.SourceRevision,
		Patch: DefinitionPatch{Description: &empty},
	})
	require.NoError(t, err)
	assert.Empty(t, patched.Definition.Description)
	assert.JSONEq(t, `{"main":{"type":"repo"}}`, string(patched.Definition.Additional["resources"]))

	_, err = store.Replace(ctx, ReplaceRequest{
		ProjectID: "typed", ExpectedSourceRevision: created.SourceRevision,
		Definition: Definition{Version: "1", Name: "typed"},
	})
	assert.True(t, IsCode(err, CodeRevisionConflict))
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
