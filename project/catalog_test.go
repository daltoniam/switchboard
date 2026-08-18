package project

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

func writeProjectFile(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "projects")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".project.json"), []byte(body), 0600))
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0700))
	cmds := [][]string{
		{"git", "init", "-b", "main", dir},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
}

func TestCatalog_ResolveByIDDeterministic(t *testing.T) {
	store := newTestCatalog(t)
	_, err := store.Create(context.Background(), CreateRequest{Definition: testDefRepo("acme", "/tmp/acme", "main")})
	require.NoError(t, err)

	first, err := store.Resolve(context.Background(), ResolveRequest{ProjectID: "acme"})
	require.NoError(t, err)
	assert.True(t, first.Revision.Valid())
	assert.True(t, first.SourceRevision.Valid())
	require.NotEmpty(t, first.Sources)
	assert.Equal(t, "user", first.Sources[0].Kind)

	second, err := store.Resolve(context.Background(), ResolveRequest{ProjectID: "acme"})
	require.NoError(t, err)
	assert.Equal(t, first.Revision, second.Revision)
	assert.Equal(t, first.SourceRevision, second.SourceRevision)
}

func TestCatalog_ExplicitWorktreeOverlay(t *testing.T) {
	store := newTestCatalog(t)
	base := t.TempDir()
	initGitRepo(t, base)
	worktree := t.TempDir()
	out, err := exec.Command("git", "-C", base, "worktree", "add", "--detach", worktree).CombinedOutput()
	if err != nil {
		t.Skipf("git worktree unavailable: %s", out)
	}
	overlayBase := `{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":` + jsonString(base) + `,"branch":"base-branch"}}}`
	overlayWT := `{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":` + jsonString(worktree) + `,"branch":"wt-branch"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(base, ".project.json"), []byte(overlayBase), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".project.json"), []byte(overlayWT), 0600))

	_, err = store.Create(context.Background(), CreateRequest{Definition: testDefRepo("acme", base, "main")})
	require.NoError(t, err)

	byID, err := store.Resolve(context.Background(), ResolveRequest{ProjectID: "acme"})
	require.NoError(t, err)
	assert.Equal(t, "base-branch", byID.Definition.PrimaryBranch())

	byRoot, err := store.Resolve(context.Background(), ResolveRequest{
		ProjectID: "acme",
		RootURI:   fileURI(worktree),
	})
	require.NoError(t, err)
	assert.Equal(t, "wt-branch", byRoot.Definition.PrimaryBranch())
	assert.NotEqual(t, byID.Revision, byRoot.Revision)
	require.GreaterOrEqual(t, len(byRoot.Sources), 2)
	assert.Equal(t, "user", byRoot.Sources[0].Kind)
	assert.Equal(t, "repository", byRoot.Sources[1].Kind)
}

func TestCatalog_RejectUnsafeRoot(t *testing.T) {
	store := newTestCatalog(t)
	_, err := store.Resolve(context.Background(), ResolveRequest{RootURI: "https://example.com"})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidRoot))
}

func TestCatalog_OptimisticConflictHiddenByOverlay(t *testing.T) {
	store := newTestCatalog(t)
	repo := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".project.json"), []byte(
		`{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":`+jsonString(repo)+`,"branch":"overlay"}}}`), 0600))
	created, err := store.Create(context.Background(), CreateRequest{Definition: testDefRepo("acme", repo, "user-one")})
	require.NoError(t, err)
	s1 := created.SourceRevision

	// Hidden user-layer change: branch is still overlay in the effective snapshot.
	userPath := filepath.Join(store.ConfigDir(), "projects", "acme.project.json")
	require.NoError(t, os.WriteFile(userPath, []byte(
		`{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":`+jsonString(repo)+`,"branch":"user-two"}}}`), 0600))

	_, err = store.Patch(context.Background(), PatchRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: s1,
		Patch:                  json.RawMessage(`{"launch":{"prompt":"x"}}`),
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeRevisionConflict))
	data, err := os.ReadFile(userPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "user-two")
}

func TestCatalog_PatchPreservesUnknownFieldsAndRepoFile(t *testing.T) {
	store := newTestCatalog(t)
	repo := t.TempDir()
	overlay := []byte(`{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":` + jsonString(repo) + `,"branch":"overlay"}},"repoOnly":true}`)
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".project.json"), overlay, 0600))
	writeProjectFile(t, store.ConfigDir(), "acme",
		`{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":`+jsonString(repo)+`,"branch":"user"}},"custom":{"keep":true}}`)
	require.NoError(t, store.Load())

	got, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	patched, err := store.Patch(context.Background(), PatchRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: got.SourceRevision,
		Patch:                  json.RawMessage(`{"launch":{"prompt":"hi"}}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "overlay", patched.Definition.PrimaryBranch())

	userRaw, err := os.ReadFile(filepath.Join(store.ConfigDir(), "projects", "acme.project.json"))
	require.NoError(t, err)
	var saved map[string]any
	require.NoError(t, json.Unmarshal(userRaw, &saved))
	resources := saved["resources"].(map[string]any)
	main := resources["main"].(map[string]any)
	assert.Equal(t, "user", main["branch"])
	assert.Equal(t, map[string]any{"keep": true}, saved["custom"])
	repoRaw, err := os.ReadFile(filepath.Join(repo, ".project.json"))
	require.NoError(t, err)
	assert.JSONEq(t, string(overlay), string(repoRaw))
}

func TestCatalog_RejectRenameThroughPatch(t *testing.T) {
	store := newTestCatalog(t)
	created, err := store.Create(context.Background(), CreateRequest{Definition: testDef("acme")})
	require.NoError(t, err)
	_, err = store.Patch(context.Background(), PatchRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: created.SourceRevision,
		Patch:                  json.RawMessage(`{"name":"renamed"}`),
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidDefinition))
	_, err = os.Stat(filepath.Join(store.ConfigDir(), "projects", "renamed.project.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestCatalog_InvalidSourceDiagnostics(t *testing.T) {
	store := newTestCatalog(t)
	writeProjectFile(t, store.ConfigDir(), "broken", `{"version":"nope","name":"broken","resources":{}}`)
	page, err := store.List(context.Background(), "")
	require.NoError(t, err)
	require.Empty(t, page.Projects)
	require.Len(t, page.InvalidProjects, 1)
	assert.Equal(t, ProjectID("broken"), page.InvalidProjects[0].ProjectID)
	assert.NotEmpty(t, page.InvalidProjects[0].RawSourceRevision)
	assert.Greater(t, page.InvalidProjects[0].DiagnosticCount, 0)

	_, err = store.Get(context.Background(), "broken")
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidDefinition))
}

func TestCatalog_RevisionsSurviveDeleteAndReconstruction(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created, err := store.Create(context.Background(), CreateRequest{Definition: testDefRepo("acme", "/tmp/acme", "one")})
	require.NoError(t, err)
	r1 := created.Revision

	patched, err := store.Patch(context.Background(), PatchRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: created.SourceRevision,
		Patch:                  json.RawMessage(`{"resources":{"main":{"type":"repo","path":"/tmp/acme","branch":"two"}}}`),
	})
	require.NoError(t, err)
	require.NoError(t, store.Delete(context.Background(), DeleteRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: patched.SourceRevision,
	}))

	again := NewStore(dir)
	got, err := again.GetRevision(context.Background(), "acme", r1)
	require.NoError(t, err)
	assert.Equal(t, r1, got.Revision)
	assert.Equal(t, "one", got.Definition.PrimaryBranch())
}

func TestCatalog_ExternalFileRefresh(t *testing.T) {
	store := newTestCatalog(t)
	_, err := store.Create(context.Background(), CreateRequest{Definition: testDefRepo("acme", "/tmp/acme", "one")})
	require.NoError(t, err)
	writeProjectFile(t, store.ConfigDir(), "acme",
		`{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":"/tmp/acme","branch":"external"}}}`)
	got, err := store.Get(context.Background(), "acme")
	require.NoError(t, err)
	assert.Equal(t, "external", got.Definition.PrimaryBranch())
}

func TestCatalog_RawCASDeleteRecreate(t *testing.T) {
	store := newTestCatalog(t)
	writeProjectFile(t, store.ConfigDir(), "broken", `{not json`)
	page, err := store.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, page.InvalidProjects, 1)
	raw := page.InvalidProjects[0].RawSourceRevision

	_, err = store.Patch(context.Background(), PatchRequest{
		ProjectID:              "broken",
		ExpectedSourceRevision: raw,
		Patch:                  json.RawMessage(`{"version":"1"}`),
	})
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeInvalidDefinition))

	require.NoError(t, store.Delete(context.Background(), DeleteRequest{
		ProjectID:                 "broken",
		ExpectedRawSourceRevision: raw,
	}))
	_, err = store.Create(context.Background(), CreateRequest{Definition: testDef("broken")})
	require.NoError(t, err)
}

func TestCatalog_ValidateJSONNoWrite(t *testing.T) {
	store := newTestCatalog(t)
	before, err := os.ReadDir(store.ConfigDir())
	require.NoError(t, err)
	diags := store.ValidateJSON(context.Background(), json.RawMessage(`{"version":"9","name":"x","resources":{}}`), nil)
	require.NotEmpty(t, diags)
	assert.Equal(t, "error", diags[0].Severity)
	after, err := os.ReadDir(store.ConfigDir())
	require.NoError(t, err)
	assert.Len(t, after, len(before))
}
