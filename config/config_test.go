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

func noEnv(string) string { return "" }

func newTestManager(t *testing.T) (*manager, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	m := &manager{filePath: path, envLookup: noEnv}
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

	assert.Len(t, m.cfg.Integrations, 23)
	for _, name := range []string{"github", "datadog", "linear", "sentry", "slack", "metabase", "aws", "posthog", "postgres", "clickhouse", "pganalyze", "rwx", "gmail", "homeassistant", "notion", "ynab", "gcp", "suno", "amazon", "jira", "confluence", "overmind", "fly"} {
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

func TestLoad_BackfillsMissingIntegrations(t *testing.T) {
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

	for name := range defaultConfig().Integrations {
		_, ok := m.cfg.Integrations[name]
		assert.True(t, ok, "missing backfilled integration: %s", name)
	}

	for _, key := range []string{"token", "client_id", "token_source"} {
		_, ok := m.cfg.Integrations["github"].Credentials[key]
		assert.True(t, ok, "missing default credential key %q for github", key)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	m, path := newTestManager(t)

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0600))

	err := m.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoad_RejectsInvalidToolGlobs(t *testing.T) {
	m, path := newTestManager(t)

	cfg := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {
				Enabled:   true,
				ToolGlobs: []string{"[unclosed"},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))

	err = m.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool glob pattern")
	assert.Contains(t, err.Error(), "github")
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
	assert.Len(t, cfg.Integrations, 23)
}

func TestGet(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	cfg := m.Get()
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.Integrations, 23)
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
	assert.Len(t, cfg.Integrations, 23)

	expected := map[string][]string{
		"github":        {"token", "client_id", "token_source"},
		"datadog":       {"api_key", "app_key"},
		"linear":        {"api_key", "mcp_access_token", "token_source"},
		"sentry":        {"auth_token", "organization", "client_id", "token_source"},
		"slack":         {"token", "cookie", "token_source"},
		"metabase":      {"api_key", "url"},
		"aws":           {"access_key_id", "secret_access_key", "session_token", "region"},
		"posthog":       {"api_key", "project_id", "base_url"},
		"postgres":      {"connection_string", "host", "user", "read_only"},
		"clickhouse":    {"host", "port", "username", "password", "database", "secure", "skip_verify"},
		"pganalyze":     {"api_key", "base_url", "organization_slug"},
		"rwx":           {"access_token"},
		"gmail":         {"access_token", "refresh_token", "client_id", "client_secret", "base_url", "token_source"},
		"homeassistant": {"token", "base_url"},
		"notion":        {"token_v2"},
		"ynab":          {"api_key"},
		"gcp":           {"project_id", "credentials_json"},
		"confluence":    {"email", "api_token", "domain"},
		"overmind":      {"base_url", "token", "agent_run_id", "flow_run_id"},
		"fly":           {"api_token", "base_url"},
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

func TestDefaultCredentialKeys(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	keys := m.DefaultCredentialKeys("pganalyze")
	assert.ElementsMatch(t, []string{"api_key", "base_url", "organization_slug"}, keys)
}

func TestDefaultCredentialKeys_Unknown(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	keys := m.DefaultCredentialKeys("nonexistent")
	assert.Nil(t, keys)
}

func TestEnvOverrides_OverridesEmptyCredentials(t *testing.T) {
	m, _ := newTestManager(t)
	m.envLookup = func(key string) string {
		switch key {
		case "GITHUB_TOKEN":
			return "gh_env_token"
		case "DD_API_KEY":
			return "dd_env_key"
		case "DD_APP_KEY":
			return "dd_env_app"
		default:
			return ""
		}
	}

	err := m.Load()
	require.NoError(t, err)

	assert.Equal(t, "gh_env_token", m.cfg.Integrations["github"].Credentials["token"])
	assert.Equal(t, "dd_env_key", m.cfg.Integrations["datadog"].Credentials["api_key"])
	assert.Equal(t, "dd_env_app", m.cfg.Integrations["datadog"].Credentials["app_key"])
}

func TestEnvOverrides_OverridesExistingValues(t *testing.T) {
	m, path := newTestManager(t)

	cfg := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {Enabled: true, Credentials: mcp.Credentials{"token": "json_token"}},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))

	m.envLookup = func(key string) string {
		if key == "GITHUB_TOKEN" {
			return "env_token"
		}
		return ""
	}

	err = m.Load()
	require.NoError(t, err)

	assert.Equal(t, "env_token", m.cfg.Integrations["github"].Credentials["token"])
}

func TestEnvOverrides_EmptyEnvDoesNotOverride(t *testing.T) {
	m, path := newTestManager(t)

	cfg := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {Enabled: true, Credentials: mcp.Credentials{"token": "json_token"}},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))

	err = m.Load()
	require.NoError(t, err)

	assert.Equal(t, "json_token", m.cfg.Integrations["github"].Credentials["token"])
}

func TestEnvOverrides_AllIntegrations(t *testing.T) {
	m, _ := newTestManager(t)

	envVars := map[string]string{
		"GITHUB_TOKEN":          "gh_tok",
		"DD_API_KEY":            "dd_api",
		"DD_APP_KEY":            "dd_app",
		"DD_SITE":               "datadoghq.eu",
		"LINEAR_API_KEY":        "lin_key",
		"SENTRY_AUTH_TOKEN":     "sentry_tok",
		"SENTRY_ORG":            "my-org",
		"SLACK_TOKEN":           "xoxc-tok",
		"SLACK_COOKIE":          "xoxd-cookie",
		"METABASE_API_KEY":      "mb_key",
		"METABASE_URL":          "https://mb.example.com",
		"AWS_ACCESS_KEY_ID":     "AKIA123",
		"AWS_SECRET_ACCESS_KEY": "secret123",
		"AWS_SESSION_TOKEN":     "sess123",
		"AWS_REGION":            "eu-west-1",
		"POSTHOG_API_KEY":       "phx_key",
		"POSTHOG_PROJECT_ID":    "12345",
		"POSTHOG_URL":           "https://eu.posthog.com",
		"DATABASE_URL":          "postgres://user:pass@host:5432/db",
		"PGHOST":                "db.example.com",
		"PGPORT":                "5433",
		"PGUSER":                "admin",
		"PGPASSWORD":            "secret",
		"PGDATABASE":            "mydb",
		"PGSSLMODE":             "require",
	}

	m.envLookup = func(key string) string {
		return envVars[key]
	}

	err := m.Load()
	require.NoError(t, err)

	assert.Equal(t, "gh_tok", m.cfg.Integrations["github"].Credentials["token"])
	assert.Equal(t, "dd_api", m.cfg.Integrations["datadog"].Credentials["api_key"])
	assert.Equal(t, "dd_app", m.cfg.Integrations["datadog"].Credentials["app_key"])
	assert.Equal(t, "datadoghq.eu", m.cfg.Integrations["datadog"].Credentials["site"])
	assert.Equal(t, "lin_key", m.cfg.Integrations["linear"].Credentials["api_key"])
	assert.Equal(t, "sentry_tok", m.cfg.Integrations["sentry"].Credentials["auth_token"])
	assert.Equal(t, "my-org", m.cfg.Integrations["sentry"].Credentials["organization"])
	assert.Equal(t, "xoxc-tok", m.cfg.Integrations["slack"].Credentials["token"])
	assert.Equal(t, "xoxd-cookie", m.cfg.Integrations["slack"].Credentials["cookie"])
	assert.Equal(t, "mb_key", m.cfg.Integrations["metabase"].Credentials["api_key"])
	assert.Equal(t, "https://mb.example.com", m.cfg.Integrations["metabase"].Credentials["url"])
	assert.Equal(t, "AKIA123", m.cfg.Integrations["aws"].Credentials["access_key_id"])
	assert.Equal(t, "secret123", m.cfg.Integrations["aws"].Credentials["secret_access_key"])
	assert.Equal(t, "sess123", m.cfg.Integrations["aws"].Credentials["session_token"])
	assert.Equal(t, "eu-west-1", m.cfg.Integrations["aws"].Credentials["region"])
	assert.Equal(t, "phx_key", m.cfg.Integrations["posthog"].Credentials["api_key"])
	assert.Equal(t, "12345", m.cfg.Integrations["posthog"].Credentials["project_id"])
	assert.Equal(t, "https://eu.posthog.com", m.cfg.Integrations["posthog"].Credentials["base_url"])
	assert.Equal(t, "postgres://user:pass@host:5432/db", m.cfg.Integrations["postgres"].Credentials["connection_string"])
	assert.Equal(t, "db.example.com", m.cfg.Integrations["postgres"].Credentials["host"])
	assert.Equal(t, "5433", m.cfg.Integrations["postgres"].Credentials["port"])
	assert.Equal(t, "admin", m.cfg.Integrations["postgres"].Credentials["user"])
	assert.Equal(t, "secret", m.cfg.Integrations["postgres"].Credentials["password"])
	assert.Equal(t, "mydb", m.cfg.Integrations["postgres"].Credentials["database"])
	assert.Equal(t, "require", m.cfg.Integrations["postgres"].Credentials["sslmode"])
}

func TestEnvOverrides_DoesNotPersistToFile(t *testing.T) {
	m, path := newTestManager(t)
	m.envLookup = func(key string) string {
		if key == "GITHUB_TOKEN" {
			return "env_secret"
		}
		return ""
	}

	err := m.Load()
	require.NoError(t, err)

	assert.Equal(t, "env_secret", m.cfg.Integrations["github"].Credentials["token"])

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var diskCfg mcp.Config
	require.NoError(t, json.Unmarshal(data, &diskCfg))
	assert.Equal(t, "", diskCfg.Integrations["github"].Credentials["token"])
}

func TestEnvMapping_ReturnsMapping(t *testing.T) {
	m := EnvMapping()
	require.NotNil(t, m)

	assert.Equal(t, "GITHUB_TOKEN", m["github"]["token"])
	assert.Equal(t, "DD_API_KEY", m["datadog"]["api_key"])
	assert.Equal(t, "DATABASE_URL", m["postgres"]["connection_string"])
	assert.Equal(t, "RWX_ACCESS_TOKEN", m["rwx"]["access_token"])
	assert.Equal(t, "RWX_CLI_PATH", m["rwx"]["cli_path"])
	assert.Len(t, m, 14)
	assert.Equal(t, "JIRA_EMAIL", m["jira"]["email"])
	assert.Equal(t, "JIRA_API_TOKEN", m["jira"]["api_token"])
	assert.Equal(t, "JIRA_DOMAIN", m["jira"]["domain"])
	assert.Equal(t, "CONFLUENCE_EMAIL", m["confluence"]["email"])
	assert.Equal(t, "CONFLUENCE_API_TOKEN", m["confluence"]["api_token"])
	assert.Equal(t, "CONFLUENCE_DOMAIN", m["confluence"]["domain"])
	assert.Equal(t, "OVERMIND_URL", m["overmind"]["base_url"])
	assert.Equal(t, "OVERMIND_TOKEN", m["overmind"]["token"])
	assert.Equal(t, "OVERMIND_AGENT_RUN_ID", m["overmind"]["agent_run_id"])
	assert.Equal(t, "OVERMIND_FLOW_RUN_ID", m["overmind"]["flow_run_id"])
}

func TestToolGlobs_PersistThroughSaveLoad(t *testing.T) {
	m, path := newTestManager(t)

	cfg := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {
				Enabled:     true,
				Credentials: mcp.Credentials{"token": "abc"},
				ToolGlobs:   []string{"github_get_*", "github_list_*"},
			},
			"datadog": {
				Enabled:     true,
				Credentials: mcp.Credentials{"api_key": "key"},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))

	err = m.Load()
	require.NoError(t, err)

	ghIC := m.cfg.Integrations["github"]
	assert.Equal(t, []string{"github_get_*", "github_list_*"}, ghIC.ToolGlobs)
	assert.True(t, ghIC.ToolAllowed("github_get_issue"))
	assert.True(t, ghIC.ToolAllowed("github_list_pulls"))
	assert.False(t, ghIC.ToolAllowed("github_delete_repo"))

	ddIC := m.cfg.Integrations["datadog"]
	assert.Empty(t, ddIC.ToolGlobs)
	assert.True(t, ddIC.ToolAllowed("datadog_anything"))
}

func TestToolGlobs_OmittedFromJSONWhenEmpty(t *testing.T) {
	ic := &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "abc"},
	}
	data, err := json.Marshal(ic)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "tool_globs")

	icWithGlobs := &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "abc"},
		ToolGlobs:   []string{"github_*"},
	}
	data, err = json.Marshal(icWithGlobs)
	require.NoError(t, err)
	assert.Contains(t, string(data), "tool_globs")
}

func TestSetIntegration_RejectsInvalidGlob(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	err := m.SetIntegration("github", &mcp.IntegrationConfig{
		Enabled:   true,
		ToolGlobs: []string{"github_[unclosed"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool glob pattern")
}

func TestSetIntegration_AcceptsValidGlobs(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	err := m.SetIntegration("github", &mcp.IntegrationConfig{
		Enabled:   true,
		ToolGlobs: []string{"github_*", "github_get_?"},
	})
	require.NoError(t, err)
}

func TestUpdate_RejectsInvalidGlob(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	err := m.Update(&mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {Enabled: true, ToolGlobs: []string{"[bad"}},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool glob pattern")
}
