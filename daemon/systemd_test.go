package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallSystemd_GeneratesUnit(t *testing.T) {
	tmp := t.TempDir()
	origFunc := systemdUnitPathFunc
	systemdUnitPathFunc = func() (string, error) {
		return filepath.Join(tmp, systemdUnitName), nil
	}
	t.Cleanup(func() { systemdUnitPathFunc = origFunc })

	origExecCommand := execCommand
	execCommand = fakeExecCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	err := InstallSystemd(3847)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmp, systemdUnitName))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "[Unit]")
	assert.Contains(t, content, "Description=Switchboard MCP Server")
	assert.Contains(t, content, "--port 3847")
	assert.Contains(t, content, "Restart=on-failure")
	assert.Contains(t, content, "[Install]")
	assert.Contains(t, content, "WantedBy=default.target")
	assert.Contains(t, content, logFileName)
}

func TestUninstallSystemd_RemovesUnit(t *testing.T) {
	tmp := t.TempDir()
	unitPath := filepath.Join(tmp, systemdUnitName)

	origFunc := systemdUnitPathFunc
	systemdUnitPathFunc = func() (string, error) { return unitPath, nil }
	t.Cleanup(func() { systemdUnitPathFunc = origFunc })

	origExecCommand := execCommand
	execCommand = fakeExecCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	err := os.WriteFile(unitPath, []byte("test"), 0644)
	require.NoError(t, err)

	err = UninstallSystemd()
	require.NoError(t, err)

	_, err = os.Stat(unitPath)
	assert.True(t, os.IsNotExist(err))
}

func TestIsSystemdInstalled(t *testing.T) {
	tmp := t.TempDir()
	unitPath := filepath.Join(tmp, systemdUnitName)

	origFunc := systemdUnitPathFunc
	systemdUnitPathFunc = func() (string, error) { return unitPath, nil }
	t.Cleanup(func() { systemdUnitPathFunc = origFunc })

	assert.False(t, IsSystemdInstalled())

	err := os.WriteFile(unitPath, []byte("test"), 0644)
	require.NoError(t, err)

	assert.True(t, IsSystemdInstalled())
}

func TestSystemdUnitTemplate_CustomPort(t *testing.T) {
	tmp := t.TempDir()
	origFunc := systemdUnitPathFunc
	systemdUnitPathFunc = func() (string, error) {
		return filepath.Join(tmp, systemdUnitName), nil
	}
	t.Cleanup(func() { systemdUnitPathFunc = origFunc })

	origExecCommand := execCommand
	execCommand = fakeExecCommand
	t.Cleanup(func() { execCommand = origExecCommand })

	err := InstallSystemd(8080)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmp, systemdUnitName))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "--port 8080")
}

func fakeExecCommand(name string, args ...string) ([]byte, error) {
	return nil, nil
}
