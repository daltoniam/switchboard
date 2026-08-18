package project

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefinition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		def     Definition
		wantErr bool
	}{
		{name: "valid", def: Definition{Version: "1", Name: "my-project"}},
		{name: "with description", def: Definition{Version: "1", Name: "my-project", Description: "hello"}},
		{name: "empty version", def: Definition{Version: "", Name: "my-project"}, wantErr: true},
		{name: "bad version", def: Definition{Version: "2", Name: "my-project"}, wantErr: true},
		{name: "empty name", def: Definition{Version: "1", Name: ""}, wantErr: true},
		{name: "bad name", def: Definition{Version: "1", Name: "-bad"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfigDir_SwitchboardOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	dir := DefaultConfigDir()
	assert.Contains(t, dir, "switchboard")
	assert.NotContains(t, dir, "project-interop")
}

func TestStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	require.NoError(t, store.CreateDefinition(&Definition{Version: "1", Name: "acme", Description: "A project"}))
	got, ok := store.Definition("acme")
	require.True(t, ok)
	assert.Equal(t, "acme", got.Name)
	assert.Equal(t, "A project", got.Description)

	snap, err := store.Create(context.Background(), CreateRequest{Definition: Definition{Version: "1", Name: "beta"}})
	require.NoError(t, err)
	assert.Equal(t, ProjectID("beta"), snap.ProjectID)

	page, err := store.Search(context.Background(), SearchRequest{Query: "acme"})
	require.NoError(t, err)
	require.Len(t, page.Projects, 1)
	assert.Equal(t, "A project", page.Projects[0].Description)

	updated, err := store.Patch(context.Background(), PatchRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: page.Projects[0].SourceRevision,
		Patch:                  json.RawMessage(`{"description":"updated"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Definition.Description)

	require.NoError(t, store.Delete(context.Background(), DeleteRequest{
		ProjectID:              "acme",
		ExpectedSourceRevision: updated.SourceRevision,
	}))
	_, ok = store.Definition("acme")
	assert.False(t, ok)
}

func TestStore_LoadFromDisk(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "projects")
	require.NoError(t, os.MkdirAll(projDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "loaded.project.json"), []byte(`{"version":"1","name":"loaded","description":"from disk"}`), 0600))
	store := NewStore(dir)
	require.NoError(t, store.Load())
	got, ok := store.Definition("loaded")
	require.True(t, ok)
	assert.Equal(t, "from disk", got.Description)
}

func TestJsonMergePatch(t *testing.T) {
	base := map[string]any{"a": "1", "b": "2", "c": map[string]any{"d": "3"}}
	patch := map[string]any{"b": nil, "c": map[string]any{"e": "4"}}
	result := jsonMergePatch(base, patch)
	assert.Equal(t, "1", result["a"])
	assert.Nil(t, result["b"])
	inner := result["c"].(map[string]any)
	assert.Equal(t, "3", inner["d"])
	assert.Equal(t, "4", inner["e"])
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "x"), ExpandHome("~/x"))
	assert.Equal(t, "/abs", ExpandHome("/abs"))
}
