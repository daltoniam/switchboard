package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveReplacesFileAtomicallyAndRestrictsPermissions(t *testing.T) {
	m, path := newTestManager(t)
	require.NoError(t, m.Load())
	old, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = old.Close() }()
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(path, 0644))
	require.NoError(t, m.SetIntegration("primer", &mcp.IntegrationConfig{Credentials: mcp.Credentials{"marker": "updated"}}))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	oldInfo, err := old.Stat()
	require.NoError(t, err)
	assert.False(t, os.SameFile(info, oldInfo))
	data := make([]byte, len(before))
	_, err = old.ReadAt(data, 0)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(before, data))
}

func TestSaveRejectsSymlinkAndRollsBack(t *testing.T) {
	m, path := newTestManager(t)
	require.NoError(t, m.Load())
	before := m.Get()
	target := filepath.Join(filepath.Dir(path), "target.json")
	require.NoError(t, os.Rename(path, target))
	require.NoError(t, os.Symlink(target, path))
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Error(t, m.SetIntegration("primer", &mcp.IntegrationConfig{Credentials: mcp.Credentials{"marker": "updated"}}))
	assert.Equal(t, before, m.Get())
	after, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, data, after)
	files, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestAtomicIntegrationUpdateUsesCurrentCopyAndRollsBack(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())
	updater, ok := any(m).(interface {
		UpdateIntegration(string, func(*mcp.IntegrationConfig) error) error
	})
	require.True(t, ok, "configuration must support an in-lock update")
	require.NoError(t, m.SetIntegration("primer", &mcp.IntegrationConfig{Credentials: mcp.Credentials{"rotation": "current"}}))
	require.NoError(t, updater.UpdateIntegration("primer", func(ic *mcp.IntegrationConfig) error {
		assert.Equal(t, "current", ic.Credentials["rotation"])
		ic.Credentials["unrelated"] = "updated"
		return nil
	}))
	before := m.Get()
	require.Error(t, updater.UpdateIntegration("primer", func(ic *mcp.IntegrationConfig) error {
		ic.Credentials["rotation"] = "discard"
		return errors.New("abort")
	}))
	assert.Equal(t, before, m.Get())
}
