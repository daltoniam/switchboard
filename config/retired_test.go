package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateRetiredIntegrations(t *testing.T) {
	tests := []struct {
		name         string
		file         map[string]*mcp.IntegrationConfig
		want         map[string]*mcp.IntegrationConfig
		wantWarnings int
	}{
		{
			name: "slackmcp identity moves into empty slack",
			file: map[string]*mcp.IntegrationConfig{
				"slackmcp": {Enabled: true, Credentials: mcp.Credentials{"base_url": ""}, Identities: map[string]mcp.IntegrationIdentity{
					"work":     {Credentials: mcp.Credentials{"access_token": "xoxp-work"}, Metadata: map[string]string{"team_id": "T1"}},
					"personal": {Credentials: mcp.Credentials{"access_token": "xoxp-personal"}},
				}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"slack": {Enabled: true, Credentials: mcp.Credentials{"token": "xoxp-personal"}},
			},
			wantWarnings: 2,
		},
		{
			name: "slackmcp team id is carried over",
			file: map[string]*mcp.IntegrationConfig{
				"slack": {Credentials: mcp.Credentials{"token": "", "cookie": ""}},
				"slackmcp": {Identities: map[string]mcp.IntegrationIdentity{
					"work": {Credentials: mcp.Credentials{"access_token": "xoxp-work"}, Metadata: map[string]string{"team_id": "T1"}},
				}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"slack": {Credentials: mcp.Credentials{"token": "xoxp-work", "cookie": "", "team_id": "T1"}},
			},
			wantWarnings: 1,
		},
		{
			name: "slackmcp dropped when slack already has a token",
			file: map[string]*mcp.IntegrationConfig{
				"slack": {Enabled: true, Credentials: mcp.Credentials{"token": "xoxc-existing"}},
				"slackmcp": {Enabled: true, Identities: map[string]mcp.IntegrationIdentity{
					"work": {Credentials: mcp.Credentials{"access_token": "xoxp-work"}},
				}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"slack": {Enabled: true, Credentials: mcp.Credentials{"token": "xoxc-existing"}},
			},
			wantWarnings: 1,
		},
		{
			name: "slackmcp without identities is dropped",
			file: map[string]*mcp.IntegrationConfig{
				"slackmcp": {Enabled: true, Credentials: mcp.Credentials{"base_url": ""}},
			},
			want:         map[string]*mcp.IntegrationConfig{},
			wantWarnings: 1,
		},
		{
			name: "notion-mcp sign-in moves into empty notion",
			file: map[string]*mcp.IntegrationConfig{
				"notion":     {Credentials: mcp.Credentials{"token_v2": ""}},
				"notion-mcp": {Enabled: true, Credentials: mcp.Credentials{"mcp_access_token": "ntn-mcp", "base_url": "https://proxy.test", mcp.CredKeyTokenSource: "oauth"}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"notion": {Enabled: true, Credentials: mcp.Credentials{"token_v2": "", "mcp_access_token": "ntn-mcp", "mcp_base_url": "https://proxy.test", mcp.CredKeyTokenSource: "oauth"}},
			},
			wantWarnings: 1,
		},
		{
			name: "notionmcp alias without notion entry",
			file: map[string]*mcp.IntegrationConfig{
				"notionmcp": {Credentials: mcp.Credentials{"mcp_access_token": "ntn-mcp"}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"notion": {Credentials: mcp.Credentials{"mcp_access_token": "ntn-mcp"}},
			},
			wantWarnings: 1,
		},
		{
			name: "notion-mcp dropped when notion has a browser token",
			file: map[string]*mcp.IntegrationConfig{
				"notion":     {Enabled: true, Credentials: mcp.Credentials{"token_v2": "v03:cookie"}},
				"notion-mcp": {Enabled: true, Credentials: mcp.Credentials{"mcp_access_token": "ntn-mcp"}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"notion": {Enabled: true, Credentials: mcp.Credentials{"token_v2": "v03:cookie"}},
			},
			wantWarnings: 1,
		},
		{
			name: "notion-mcp dropped when notion has a public API token",
			file: map[string]*mcp.IntegrationConfig{
				"notion":     {Credentials: mcp.Credentials{"access_token": "ntn_public"}},
				"notion-mcp": {Credentials: mcp.Credentials{"mcp_access_token": "ntn-mcp"}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"notion": {Credentials: mcp.Credentials{"access_token": "ntn_public"}},
			},
			wantWarnings: 1,
		},
		{
			name: "unconfigured notion-mcp default is dropped",
			file: map[string]*mcp.IntegrationConfig{
				"notion-mcp": {Credentials: mcp.Credentials{"mcp_access_token": "", "base_url": ""}},
			},
			want:         map[string]*mcp.IntegrationConfig{},
			wantWarnings: 1,
		},
		{
			name: "nothing retired",
			file: map[string]*mcp.IntegrationConfig{
				"slack": {Credentials: mcp.Credentials{"token": "xoxb"}},
			},
			want: map[string]*mcp.IntegrationConfig{
				"slack": {Credentials: mcp.Credentials{"token": "xoxb"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &mcp.Config{Integrations: tt.file}
			warnings := migrateRetiredIntegrations(cfg)
			assert.Equal(t, tt.want, cfg.Integrations)
			assert.Len(t, warnings, tt.wantWarnings)
			for _, warning := range warnings {
				assert.NotContains(t, warning, "xoxp-")
				assert.NotContains(t, warning, "ntn-mcp")
			}
			assert.Empty(t, migrateRetiredIntegrations(cfg), "migration must be idempotent")
		})
	}
}

func TestMigrateRetiredIntegrations_NilIntegrations(t *testing.T) {
	assert.Empty(t, migrateRetiredIntegrations(&mcp.Config{}))
}

func writeConfigFile(t *testing.T, path string, cfg *mcp.Config) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))
}

func readConfigFile(t *testing.T, path string) *mcp.Config {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var cfg mcp.Config
	require.NoError(t, json.Unmarshal(data, &cfg))
	return &cfg
}

func TestLoad_MigratesRetiredIntegrations(t *testing.T) {
	m, path := newTestManager(t)
	var logged []string
	previous := logRetired
	logRetired = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	t.Cleanup(func() { logRetired = previous })

	writeConfigFile(t, path, &mcp.Config{Integrations: map[string]*mcp.IntegrationConfig{
		"slackmcp": {Enabled: true, Credentials: mcp.Credentials{"base_url": ""}, Identities: map[string]mcp.IntegrationIdentity{
			"work": {Credentials: mcp.Credentials{"access_token": "xoxp-work"}},
		}},
		"notion-mcp": {Enabled: true, Credentials: mcp.Credentials{"mcp_access_token": "ntn-mcp", "base_url": ""}},
	}})

	require.NoError(t, m.Load())
	assert.Len(t, logged, 2)
	assert.NotContains(t, m.cfg.Integrations, "slackmcp")
	assert.NotContains(t, m.cfg.Integrations, "notion-mcp")
	assert.True(t, m.cfg.Integrations["slack"].Enabled)
	assert.Equal(t, "xoxp-work", m.cfg.Integrations["slack"].Credentials["token"])
	assert.True(t, m.cfg.Integrations["notion"].Enabled)
	assert.Equal(t, "ntn-mcp", m.cfg.Integrations["notion"].Credentials["mcp_access_token"])
	assert.NotContains(t, m.EnabledIntegrations(), "slackmcp")
	assert.NotContains(t, m.EnabledIntegrations(), "notion-mcp")

	saved := readConfigFile(t, path)
	assert.NotContains(t, saved.Integrations, "slackmcp")
	assert.NotContains(t, saved.Integrations, "notion-mcp")
	assert.Equal(t, "xoxp-work", saved.Integrations["slack"].Credentials["token"])
	assert.Equal(t, "ntn-mcp", saved.Integrations["notion"].Credentials["mcp_access_token"])

	require.NoError(t, m.SetIntegration("github", &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"token": "gh"}}))
	saved = readConfigFile(t, path)
	assert.Equal(t, "xoxp-work", saved.Integrations["slack"].Credentials["token"])
	assert.Equal(t, "ntn-mcp", saved.Integrations["notion"].Credentials["mcp_access_token"])

	logged = nil
	reloaded := &manager{filePath: path, envLookup: noEnv}
	require.NoError(t, reloaded.Load())
	assert.Empty(t, logged, "a migrated file must not warn again")
}

func TestLoad_RetiredIntegrationWhenSaveFails(t *testing.T) {
	m, path := newTestManager(t)
	var logged []string
	previous := logRetired
	logRetired = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	t.Cleanup(func() { logRetired = previous })
	target := filepath.Join(t.TempDir(), "real-config.json")
	writeConfigFile(t, target, &mcp.Config{Integrations: map[string]*mcp.IntegrationConfig{
		"notion-mcp": {Enabled: true, Credentials: mcp.Credentials{"mcp_access_token": "ntn-mcp"}},
	}})
	// Saves refuse a non-regular target, so a symlinked config fails to save even as root.
	require.NoError(t, os.Symlink(target, path))

	require.NoError(t, m.Load())
	assert.NotContains(t, m.cfg.Integrations, "notion-mcp")
	assert.Equal(t, "ntn-mcp", m.cfg.Integrations["notion"].Credentials["mcp_access_token"])
	require.Len(t, logged, 2)
	assert.Contains(t, logged[1], "could not save migrated config")
	assert.Contains(t, readConfigFile(t, path).Integrations, "notion-mcp", "the file is left untouched when the save fails")
}
