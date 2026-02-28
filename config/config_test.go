package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) (*manager, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	m := &manager{filePath: path}
	return m, path
}

func TestLoad_CreatesDefaultWhenMissing(t *testing.T) {
	m, path := newTestManager(t)

	err := m.Load()
	require.NoError(t, err)
	require.NotNil(t, m.cfg)

	// File should have been created.
	_, err = os.Stat(path)
	assert.NoError(t, err)

	assert.Len(t, m.cfg.Integrations, 11)
	for _, name := range []string{"github", "datadog", "linear", "sentry", "slack", "metabase", "aws", "posthog", "postgres", "clickhouse", "gmail"} {
		ic, ok := m.cfg.Integrations[name]
		assert.True(t, ok, "missing default integration: %s", name)
		assert.False(t, ic.Enabled)
	}
}

func TestLoad_ParsesExistingFile(t *testing.T) {
	m, path := newTestManager(t)

	cfg := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {Enabled: true, Credentials: mcp.Credentials{"token": "abc"}},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))

	err = m.Load()
	require.NoError(t, err)
	assert.True(t, m.cfg.Integrations["github"].Enabled)
	assert.Equal(t, "abc", m.cfg.Integrations["github"].Credentials["token"])
}

func TestLoad_InvalidJSON(t *testing.T) {
	m, path := newTestManager(t)

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0600))

	err := m.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestSave(t *testing.T) {
	m, path := newTestManager(t)
	m.cfg = defaultConfig()

	err := m.Save()
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg mcp.Config
	require.NoError(t, json.Unmarshal(data, &cfg))
	assert.Len(t, cfg.Integrations, 11)
}

func TestGet(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	cfg := m.Get()
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.Integrations, 11)
}

func TestUpdate(t *testing.T) {
	m, path := newTestManager(t)
	require.NoError(t, m.Load())

	newCfg := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"custom": {Enabled: true, Credentials: mcp.Credentials{"key": "val"}},
		},
	}
	err := m.Update(newCfg)
	require.NoError(t, err)

	assert.Equal(t, newCfg, m.Get())

	// Verify written to disk.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var diskCfg mcp.Config
	require.NoError(t, json.Unmarshal(data, &diskCfg))
	assert.True(t, diskCfg.Integrations["custom"].Enabled)
}

func TestGetIntegration(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	ic, ok := m.GetIntegration("github")
	assert.True(t, ok)
	assert.NotNil(t, ic)
	assert.False(t, ic.Enabled)

	_, ok = m.GetIntegration("nonexistent")
	assert.False(t, ok)
}

func TestSetIntegration(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	ic := &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "new_token"},
	}
	err := m.SetIntegration("github", ic)
	require.NoError(t, err)

	got, ok := m.GetIntegration("github")
	assert.True(t, ok)
	assert.True(t, got.Enabled)
	assert.Equal(t, "new_token", got.Credentials["token"])
}

func TestSetIntegration_NewIntegration(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	ic := &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"key": "value"},
	}
	err := m.SetIntegration("custom_new", ic)
	require.NoError(t, err)

	got, ok := m.GetIntegration("custom_new")
	assert.True(t, ok)
	assert.True(t, got.Enabled)
}

func TestEnabledIntegrations(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	// Default: none enabled.
	enabled := m.EnabledIntegrations()
	assert.Empty(t, enabled)

	// Enable one.
	err := m.SetIntegration("github", &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "t"},
	})
	require.NoError(t, err)

	enabled = m.EnabledIntegrations()
	assert.Equal(t, []string{"github"}, enabled)
}

func TestEnabledIntegrations_Multiple(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	for _, name := range []string{"github", "datadog"} {
		err := m.SetIntegration(name, &mcp.IntegrationConfig{
			Enabled:     true,
			Credentials: mcp.Credentials{"key": "val"},
		})
		require.NoError(t, err)
	}

	enabled := m.EnabledIntegrations()
	assert.Len(t, enabled, 2)
	assert.ElementsMatch(t, []string{"github", "datadog"}, enabled)
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Integrations, 11)

	expected := map[string][]string{
		"github":     {"token", "client_id", "token_source"},
		"datadog":    {"api_key", "app_key"},
		"linear":     {"api_key", "client_id", "client_secret", "token_source"},
		"sentry":     {"auth_token", "organization", "client_id", "token_source"},
		"slack":      {"token", "cookie"},
		"metabase":   {"api_key", "url"},
		"aws":        {"access_key_id", "secret_access_key", "session_token", "region"},
		"posthog":    {"api_key", "project_id", "base_url"},
		"postgres":   {"connection_string", "host", "user", "read_only"},
		"clickhouse": {"host", "port", "username", "password", "database", "secure", "skip_verify"},
		"gmail":      {"access_token", "base_url"},
	}

	for name, keys := range expected {
		ic, ok := cfg.Integrations[name]
		require.True(t, ok, "missing integration: %s", name)
		assert.False(t, ic.Enabled)
		for _, key := range keys {
			_, exists := ic.Credentials[key]
			assert.True(t, exists, "missing credential key %q for %s", key, name)
		}
	}
}

func TestSave_FilePermissions(t *testing.T) {
	m, path := newTestManager(t)
	m.cfg = defaultConfig()

	err := m.Save()
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	// File should be readable/writable by owner only.
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
