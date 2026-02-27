package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallLaunchd_GeneratesPlist(t *testing.T) {
	tmp := t.TempDir()
	origFunc := launchdPlistPathFunc
	launchdPlistPathFunc = func() (string, error) {
		return filepath.Join(tmp, "com.daltoniam.switchboard.plist"), nil
	}
	t.Cleanup(func() { launchdPlistPathFunc = origFunc })

	err := InstallLaunchd(3847)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmp, "com.daltoniam.switchboard.plist"))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "<key>Label</key>")
	assert.Contains(t, content, launchdLabel)
	assert.Contains(t, content, "<string>--port</string>")
	assert.Contains(t, content, "<string>3847</string>")
	assert.Contains(t, content, "<key>RunAtLoad</key>")
	assert.Contains(t, content, "<key>KeepAlive</key>")
	assert.Contains(t, content, logFileName)
}

func TestUninstallLaunchd_RemovesPlist(t *testing.T) {
	tmp := t.TempDir()
	plistPath := filepath.Join(tmp, "com.daltoniam.switchboard.plist")

	origFunc := launchdPlistPathFunc
	launchdPlistPathFunc = func() (string, error) { return plistPath, nil }
	t.Cleanup(func() { launchdPlistPathFunc = origFunc })

	err := os.WriteFile(plistPath, []byte("test"), 0644)
	require.NoError(t, err)

	err = UninstallLaunchd()
	require.NoError(t, err)

	_, err = os.Stat(plistPath)
	assert.True(t, os.IsNotExist(err))
}

func TestIsLaunchdInstalled(t *testing.T) {
	tmp := t.TempDir()
	plistPath := filepath.Join(tmp, "com.daltoniam.switchboard.plist")

	origFunc := launchdPlistPathFunc
	launchdPlistPathFunc = func() (string, error) { return plistPath, nil }
	t.Cleanup(func() { launchdPlistPathFunc = origFunc })

	assert.False(t, IsLaunchdInstalled())

	err := os.WriteFile(plistPath, []byte("test"), 0644)
	require.NoError(t, err)

	assert.True(t, IsLaunchdInstalled())
}

func TestLaunchdPlistTemplate_CustomPort(t *testing.T) {
	tmp := t.TempDir()
	origFunc := launchdPlistPathFunc
	launchdPlistPathFunc = func() (string, error) {
		return filepath.Join(tmp, "com.daltoniam.switchboard.plist"), nil
	}
	t.Cleanup(func() { launchdPlistPathFunc = origFunc })

	err := InstallLaunchd(9999)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmp, "com.daltoniam.switchboard.plist"))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "<string>9999</string>")
	assert.True(t, strings.Contains(content, "<?xml version"))
}
