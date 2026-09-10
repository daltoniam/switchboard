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

	assert.Len(t, m.cfg.Integrations, 64)
	for _, name := range []string{"github", "forgejo", "datadog", "linear", "sentry", "slack", "slackmcp", "likec4excalidraw", "figma", "metabase", "paperless", "recoll", "aws", "posthog", "postgres", "clickhouse", "elasticsearch", "pganalyze", "rwx", "projectinterop", "gmail", "gcal", "gdrive", "gdocs", "gsheets", "gslides", "gforms", "gchat", "gmeet", "gtasks", "gpeople", "notion", "ollama", "ynab", "stripe", "gcp", "suno", "amazon", "jira", "confluence", "salesforce", "servicenow", "cloudflare", "digitalocean", "fly", "kubernetes", "vercel", "snowflake", "acp", "web", "botidentity", "x", "signoz", "nomad", "agents", "switchboard", "netsuite", "ramp", "gong", "zendesk", "hubspot", "intercom", "okta", "microsoft365"} {
		ic, ok := m.cfg.Integrations[name]
		require.True(t, ok, "missing default integration: %s", name)
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

func TestProjectCatalogConfig_DefaultEnabled(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())
	assert.True(t, mcp.ProjectCatalogEnabled(m.cfg.ProjectCatalog))
	assert.True(t, mcp.ProjectCatalogWritesEnabled(m.cfg.ProjectCatalog))
}

func TestProjectCatalogConfig_ExplicitDisable(t *testing.T) {
	m, path := newTestManager(t)
	off := false
	cfg := &mcp.Config{ProjectCatalog: mcp.ProjectCatalogConfig{Enabled: &off, WritesEnabled: &off}}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))
	require.NoError(t, m.Load())
	assert.False(t, mcp.ProjectCatalogEnabled(m.cfg.ProjectCatalog))
	assert.False(t, mcp.ProjectCatalogWritesEnabled(m.cfg.ProjectCatalog))
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
	assert.Len(t, cfg.Integrations, 64)
}

func TestGet(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	cfg := m.Get()
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.Integrations, 64)
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
	assert.Len(t, cfg.Integrations, 64)

	expected := map[string][]string{
		"github":           {"token", "client_id", "token_source"},
		"forgejo":          {"base_url", "token"},
		"datadog":          {"api_key", "app_key"},
		"linear":           {"api_key", "mcp_access_token", "token_source"},
		"sentry":           {"auth_token", "organization", "client_id", "token_source"},
		"slack":            {"token", "cookie", "token_source"},
		"slackmcp":         {"base_url"},
		"likec4excalidraw": {"base_url", "mcp_token"},
		"figma":            {"mcp_access_token", "base_url", "token_source"},
		"metabase":         {"api_key", "url"},
		"paperless":        {"token", "url"},
		"recoll":           {"base_url"},
		"aws":              {"access_key_id", "secret_access_key", "session_token", "region"},
		"posthog":          {"api_key", "project_id", "base_url"},
		"postgres":         {"connection_string", "host", "user", "read_only"},
		"clickhouse":       {"host", "port", "username", "password", "database", "secure", "skip_verify", "connections"},
		"pganalyze":        {"api_key", "base_url"},
		"rwx":              {"access_token", "org"},
		"projectinterop":   {"config_root"},
		"gmail":            {"access_token", "refresh_token", "client_id", "client_secret", "base_url", "token_source"},
		"notion":           {"token_v2"},
		"ollama":           {"base_url", "api_key"},
		"ynab":             {"api_key"},
		"gong":             {"access_key", "access_key_secret", "base_url"},
		"hubspot":          {"access_token", "base_url"},
		"intercom":         {"access_token", "base_url"},
		"ramp":             {"access_token", "base_url"},
		"zendesk":          {"subdomain", "email", "api_token", "access_token", "base_url"},
		"okta":             {"api_token", "org_url"},
		"microsoft365":     {"access_token", "refresh_token", "client_id", "client_secret", "tenant_id", "base_url", "token_source"},
		"gcp":              {"project_id", "credentials_json"},
		"confluence":       {"email", "api_token", "domain"},
		"elasticsearch":    {"base_url", "api_key", "username", "password"},
		"salesforce":       {"access_token", "instance_url", "api_version"},
		"servicenow":       {"instance_url", "username", "password", "access_token"},
		"netsuite":         {"account_id", "consumer_key", "consumer_secret", "token_id", "token_secret", "access_token", "base_url"},
		"cloudflare":       {"api_token", "account_id"},
		"digitalocean":     {"api_token"},
		"fly":              {"api_token", "base_url"},
		"kubernetes":       {"kubeconfig", "kubeconfig_path", "context", "namespace", "api_server", "token", "ca_cert", "insecure_skip_tls_verify", "in_cluster", "clusters", "allow_mutations"},
		"vercel":           {"api_token", "team_id", "team_slug", "base_url"},
		"web":              {},
		"signoz":           {"api_key", "base_url", "skip_verify"},
		"nomad":            {"address", "token"},
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

func TestDefaultConfig_Forgejo(t *testing.T) {
	ic, ok := defaultConfig().Integrations["forgejo"]
	require.True(t, ok, "missing default integration: forgejo")
	assert.False(t, ic.Enabled)
	assert.Equal(t, mcp.Credentials{"base_url": "", "token": ""}, ic.Credentials)
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
	assert.ElementsMatch(t, []string{"api_key", "base_url"}, keys)
}

func TestDefaultCredentialKeys_Forgejo(t *testing.T) {
	m, _ := newTestManager(t)
	assert.ElementsMatch(t, []string{"base_url", "token"}, m.DefaultCredentialKeys("forgejo"))
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

func TestEnvOverrides_Forgejo(t *testing.T) {
	tests := []struct {
		name     string
		stored   *mcp.IntegrationConfig
		env      map[string]string
		expected mcp.Credentials
	}{
		{
			name:     "backfills existing config",
			expected: mcp.Credentials{"base_url": "", "token": ""},
		},
		{
			name: "environment credentials remain disabled",
			env: map[string]string{
				"FORGEJO_BASE_URL": "https://git.example.com/forgejo/",
				"FORGEJO_TOKEN":    "env-token",
			},
			expected: mcp.Credentials{"base_url": "https://git.example.com/forgejo/", "token": "env-token"},
		},
		{
			name: "environment overrides stored credentials",
			stored: &mcp.IntegrationConfig{
				Enabled:     true,
				Credentials: mcp.Credentials{"base_url": "https://old.example.com", "token": "disk-token"},
			},
			env: map[string]string{
				"FORGEJO_BASE_URL": "https://git.example.com/forgejo/",
				"FORGEJO_TOKEN":    "env-token",
			},
			expected: mcp.Credentials{"base_url": "https://git.example.com/forgejo/", "token": "env-token"},
		},
		{
			name: "empty environment preserves stored credentials",
			stored: &mcp.IntegrationConfig{
				Enabled:     true,
				Credentials: mcp.Credentials{"base_url": "https://git.example.com/forgejo/", "token": "disk-token"},
			},
			env:      map[string]string{"FORGEJO_BASE_URL": "", "FORGEJO_TOKEN": ""},
			expected: mcp.Credentials{"base_url": "https://git.example.com/forgejo/", "token": "disk-token"},
		},
		{
			name: "token override preserves deployment subpath",
			stored: &mcp.IntegrationConfig{
				Credentials: mcp.Credentials{"base_url": "https://git.example.com/forgejo/", "token": "disk-token"},
			},
			env:      map[string]string{"FORGEJO_TOKEN": "env-token"},
			expected: mcp.Credentials{"base_url": "https://git.example.com/forgejo/", "token": "env-token"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, path := newTestManager(t)
			cfg := &mcp.Config{Integrations: map[string]*mcp.IntegrationConfig{}}
			if tt.stored != nil {
				cfg.Integrations["forgejo"] = tt.stored
			}
			data, err := json.Marshal(cfg)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, data, 0600))
			m.envLookup = func(key string) string { return tt.env[key] }
			require.NoError(t, m.Load())

			ic, ok := m.GetIntegration("forgejo")
			require.True(t, ok, "missing integration: forgejo")
			assert.Equal(t, tt.stored != nil && tt.stored.Enabled, ic.Enabled)
			assert.Equal(t, tt.expected, ic.Credentials)

			require.NoError(t, m.Save())
			onDisk := &manager{filePath: path, envLookup: noEnv}
			require.NoError(t, onDisk.Load())
			persisted, ok := onDisk.GetIntegration("forgejo")
			require.True(t, ok)
			if tt.stored != nil {
				assert.Equal(t, tt.stored, persisted)
			} else {
				assert.False(t, persisted.Enabled)
				assert.Equal(t, mcp.Credentials{"base_url": "", "token": ""}, persisted.Credentials)
			}
		})
	}
}

func TestEnvOverrides_GoogleSharedClientFansOut(t *testing.T) {
	m, _ := newTestManager(t)
	m.envLookup = func(key string) string {
		switch key {
		case "GOOGLE_OAUTH_CLIENT_ID":
			return "shared-client-id"
		case "GOOGLE_OAUTH_CLIENT_SECRET":
			return "shared-secret"
		default:
			return ""
		}
	}

	require.NoError(t, m.Load())

	for _, name := range googleWorkspaceIntegrations {
		ic := m.cfg.Integrations[name]
		require.NotNil(t, ic, "integration %q missing", name)
		assert.Equal(t, "shared-client-id", ic.Credentials[mcp.CredKeyClientID], "client_id for %q", name)
		assert.Equal(t, "shared-secret", ic.Credentials[mcp.CredKeyClientSecret], "client_secret for %q", name)
	}
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
		"GITHUB_TOKEN":                "gh_tok",
		"DD_API_KEY":                  "dd_api",
		"DD_APP_KEY":                  "dd_app",
		"DD_SITE":                     "datadoghq.eu",
		"LINEAR_API_KEY":              "lin_key",
		"SENTRY_AUTH_TOKEN":           "sentry_tok",
		"SENTRY_ORG":                  "my-org",
		"SLACK_TOKEN":                 "xoxc-tok",
		"SLACK_COOKIE":                "xoxd-cookie",
		"METABASE_API_KEY":            "mb_key",
		"METABASE_URL":                "https://mb.example.com",
		"AWS_ACCESS_KEY_ID":           "AKIA123",
		"AWS_SECRET_ACCESS_KEY":       "secret123",
		"AWS_SESSION_TOKEN":           "sess123",
		"AWS_REGION":                  "eu-west-1",
		"POSTHOG_API_KEY":             "phx_key",
		"POSTHOG_PROJECT_ID":          "12345",
		"POSTHOG_URL":                 "https://eu.posthog.com",
		"DATABASE_URL":                "postgres://user:pass@host:5432/db",
		"PGHOST":                      "db.example.com",
		"PGPORT":                      "5433",
		"PGUSER":                      "admin",
		"PGPASSWORD":                  "secret",
		"PGDATABASE":                  "mydb",
		"PGSSLMODE":                   "require",
		"VERCEL_API_TOKEN":            "vc_token",
		"VERCEL_TEAM_ID":              "team_123",
		"VERCEL_TEAM_SLUG":            "acme",
		"VERCEL_BASE_URL":             "https://api.vercel.test",
		"LIKEC4_EXCALIDRAW_BASE_URL":  "http://127.0.0.1:4242",
		"LIKEC4_EXCALIDRAW_MCP_TOKEN": "likec4-secret",
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
	assert.Equal(t, "vc_token", m.cfg.Integrations["vercel"].Credentials["api_token"])
	assert.Equal(t, "team_123", m.cfg.Integrations["vercel"].Credentials["team_id"])
	assert.Equal(t, "acme", m.cfg.Integrations["vercel"].Credentials["team_slug"])
	assert.Equal(t, "https://api.vercel.test", m.cfg.Integrations["vercel"].Credentials["base_url"])
	assert.Equal(t, "http://127.0.0.1:4242", m.cfg.Integrations["likec4excalidraw"].Credentials["base_url"])
	assert.Equal(t, "likec4-secret", m.cfg.Integrations["likec4excalidraw"].Credentials["mcp_token"])
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
	assert.Equal(t, map[string]string{"base_url": "FORGEJO_BASE_URL", "token": "FORGEJO_TOKEN"}, m["forgejo"])
	assert.Equal(t, "DD_API_KEY", m["datadog"]["api_key"])
	assert.Equal(t, "DATABASE_URL", m["postgres"]["connection_string"])
	assert.Equal(t, "RWX_ACCESS_TOKEN", m["rwx"]["access_token"])
	assert.Equal(t, "RWX_ORG", m["rwx"]["org"])
	assert.Equal(t, "RWX_CLI_PATH", m["rwx"]["cli_path"])
	assert.Equal(t, "OLLAMA_HOST", m["ollama"]["base_url"])
	assert.Equal(t, "OLLAMA_API_KEY", m["ollama"]["api_key"])
	assert.Equal(t, "JIRA_EMAIL", m["jira"]["email"])
	assert.Equal(t, "JIRA_API_TOKEN", m["jira"]["api_token"])
	assert.Equal(t, "JIRA_DOMAIN", m["jira"]["domain"])
	assert.Equal(t, "CONFLUENCE_EMAIL", m["confluence"]["email"])
	assert.Equal(t, "CONFLUENCE_API_TOKEN", m["confluence"]["api_token"])
	assert.Equal(t, "CONFLUENCE_DOMAIN", m["confluence"]["domain"])
	assert.Equal(t, "BOTIDENTITY_GITHUB_TOKEN", m["botidentity"]["github_token"])
	assert.Equal(t, "BOTIDENTITY_SLACK_CONFIG_TOKEN", m["botidentity"]["slack_config_token"])
	assert.Equal(t, "BOTIDENTITY_SLACK_REFRESH_TOKEN", m["botidentity"]["slack_refresh_token"])
	assert.Equal(t, "X_BEARER_TOKEN", m["x"]["bearer_token"])
	assert.Equal(t, "SIGNOZ_API_KEY", m["signoz"]["api_key"])
	assert.Equal(t, "NOMAD_ADDR", m["nomad"]["address"])
	assert.Equal(t, "NOMAD_TOKEN", m["nomad"]["token"])
	assert.Equal(t, "STRIPE_API_KEY", m["stripe"]["api_key"])
	assert.Equal(t, "STRIPE_ACCOUNT", m["stripe"]["account"])
	assert.Equal(t, "STRIPE_BASE_URL", m["stripe"]["base_url"])
	assert.Equal(t, "GONG_ACCESS_KEY", m["gong"]["access_key"])
	assert.Equal(t, "GONG_ACCESS_KEY_SECRET", m["gong"]["access_key_secret"])
	assert.Equal(t, "HUBSPOT_ACCESS_TOKEN", m["hubspot"]["access_token"])
	assert.Equal(t, "HUBSPOT_BASE_URL", m["hubspot"]["base_url"])
	assert.Equal(t, "INTERCOM_ACCESS_TOKEN", m["intercom"]["access_token"])
	assert.Equal(t, "INTERCOM_BASE_URL", m["intercom"]["base_url"])
	assert.Equal(t, "RAMP_ACCESS_TOKEN", m["ramp"]["access_token"])
	assert.Equal(t, "RAMP_BASE_URL", m["ramp"]["base_url"])
	assert.Equal(t, "ZENDESK_SUBDOMAIN", m["zendesk"]["subdomain"])
	assert.Equal(t, "ZENDESK_EMAIL", m["zendesk"]["email"])
	assert.Equal(t, "ZENDESK_API_TOKEN", m["zendesk"]["api_token"])
	assert.Equal(t, "ZENDESK_ACCESS_TOKEN", m["zendesk"]["access_token"])
	assert.Equal(t, "ZENDESK_BASE_URL", m["zendesk"]["base_url"])
	assert.Equal(t, "OKTA_API_TOKEN", m["okta"]["api_token"])
	assert.Equal(t, "OKTA_ORG_URL", m["okta"]["org_url"])
	assert.Equal(t, "NETSUITE_ACCOUNT_ID", m["netsuite"]["account_id"])
	assert.Equal(t, "NETSUITE_ACCESS_TOKEN", m["netsuite"]["access_token"])
	assert.Equal(t, "MICROSOFT365_ACCESS_TOKEN", m["microsoft365"]["access_token"])
	assert.Equal(t, "MICROSOFT365_CLIENT_ID", m["microsoft365"]["client_id"])
	assert.Equal(t, "MICROSOFT365_TENANT_ID", m["microsoft365"]["tenant_id"])
	assert.Equal(t, "FIGMA_MCP_ACCESS_TOKEN", m["figma"]["mcp_access_token"])
	assert.Equal(t, "FIGMA_MCP_BASE_URL", m["figma"]["base_url"])
	assert.Equal(t, "SERVICENOW_INSTANCE_URL", m["servicenow"]["instance_url"])
	assert.Equal(t, "SERVICENOW_USERNAME", m["servicenow"]["username"])
	assert.Equal(t, "SERVICENOW_PASSWORD", m["servicenow"]["password"])
	assert.Equal(t, "SERVICENOW_ACCESS_TOKEN", m["servicenow"]["access_token"])
	assert.Equal(t, "KUBECONFIG_CONTENT", m["kubernetes"]["kubeconfig"])
	assert.Equal(t, "KUBECONFIG", m["kubernetes"]["kubeconfig_path"])
	assert.Equal(t, "KUBECONTEXT", m["kubernetes"]["context"])
	assert.Equal(t, "KUBENAMESPACE", m["kubernetes"]["namespace"])
	assert.Equal(t, "KUBERNETES_API_SERVER", m["kubernetes"]["api_server"])
	assert.Equal(t, "KUBERNETES_TOKEN", m["kubernetes"]["token"])
	assert.Equal(t, "KUBERNETES_CA_CERT", m["kubernetes"]["ca_cert"])
	assert.Equal(t, "KUBERNETES_INSECURE_SKIP_TLS_VERIFY", m["kubernetes"]["insecure_skip_tls_verify"])
	assert.Equal(t, "KUBERNETES_IN_CLUSTER", m["kubernetes"]["in_cluster"])
	assert.Equal(t, "KUBERNETES_CLUSTERS", m["kubernetes"]["clusters"])
	assert.Equal(t, "KUBERNETES_ALLOW_MUTATIONS", m["kubernetes"]["allow_mutations"])
	assert.Equal(t, "PAPERLESS_TOKEN", m["paperless"]["token"])
	assert.Equal(t, "PAPERLESS_URL", m["paperless"]["url"])
	assert.Equal(t, "RECOLL_URL", m["recoll"]["base_url"])
	assert.Equal(t, "VERCEL_API_TOKEN", m["vercel"]["api_token"])
	assert.Equal(t, "VERCEL_TEAM_ID", m["vercel"]["team_id"])
	assert.Equal(t, "VERCEL_TEAM_SLUG", m["vercel"]["team_slug"])
	assert.Equal(t, "VERCEL_BASE_URL", m["vercel"]["base_url"])
	assert.Equal(t, "LIKEC4_EXCALIDRAW_BASE_URL", m["likec4excalidraw"]["base_url"])
	assert.Equal(t, "LIKEC4_EXCALIDRAW_MCP_TOKEN", m["likec4excalidraw"]["mcp_token"])
	assert.Len(t, m, 53)
	for _, name := range googleWorkspaceIntegrations {
		assert.Equal(t, "GOOGLE_OAUTH_CLIENT_ID", m[name][mcp.CredKeyClientID])
		assert.Equal(t, "GOOGLE_OAUTH_CLIENT_SECRET", m[name][mcp.CredKeyClientSecret])
	}
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
	assert.True(t, ghIC.ToolAllowed(mcp.ToolName("github_get_issue")))
	assert.True(t, ghIC.ToolAllowed(mcp.ToolName("github_list_pulls")))
	assert.False(t, ghIC.ToolAllowed(mcp.ToolName("github_delete_repo")))

	ddIC := m.cfg.Integrations["datadog"]
	assert.Empty(t, ddIC.ToolGlobs)
	assert.True(t, ddIC.ToolAllowed(mcp.ToolName("datadog_anything")))
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

func TestSetWasmModules(t *testing.T) {
	m, path := newTestManager(t)
	require.NoError(t, m.Load())

	modules := []mcp.WasmModuleConfig{
		{Path: "/tmp/mod1.wasm", Credentials: mcp.Credentials{"key": "val"}},
		{Path: "/tmp/mod2.wasm"},
	}
	err := m.SetWasmModules(modules)
	require.NoError(t, err)

	got := m.Get()
	require.Len(t, got.WasmModules, 2)
	assert.Equal(t, "/tmp/mod1.wasm", got.WasmModules[0].Path)
	assert.Equal(t, "val", got.WasmModules[0].Credentials["key"])
	assert.Equal(t, "/tmp/mod2.wasm", got.WasmModules[1].Path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var persisted mcp.Config
	require.NoError(t, json.Unmarshal(data, &persisted))
	require.Len(t, persisted.WasmModules, 2)
	assert.Equal(t, "/tmp/mod1.wasm", persisted.WasmModules[0].Path)
}

func TestSetWasmModules_EmptySlice(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	modules := []mcp.WasmModuleConfig{
		{Path: "/tmp/mod.wasm"},
	}
	require.NoError(t, m.SetWasmModules(modules))
	require.Len(t, m.Get().WasmModules, 1)

	require.NoError(t, m.SetWasmModules(nil))
	assert.Empty(t, m.Get().WasmModules)
}

func TestSetWasmModules_PreservedOnReload(t *testing.T) {
	m, path := newTestManager(t)
	require.NoError(t, m.Load())

	modules := []mcp.WasmModuleConfig{
		{Path: "/tmp/persist.wasm", Credentials: mcp.Credentials{"token": "abc"}},
	}
	require.NoError(t, m.SetWasmModules(modules))

	m2 := &manager{filePath: path, envLookup: noEnv}
	require.NoError(t, m2.Load())
	require.Len(t, m2.Get().WasmModules, 1)
	assert.Equal(t, "/tmp/persist.wasm", m2.Get().WasmModules[0].Path)
	assert.Equal(t, "abc", m2.Get().WasmModules[0].Credentials["token"])
}

func TestMergeWithDefaults_PreservesIdentities(t *testing.T) {
	file := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"slack": {
				Enabled: true,
				Credentials: mcp.Credentials{
					"token": "xoxc",
				},
				Identities: map[string]mcp.IntegrationIdentity{
					"work": {
						Credentials: mcp.Credentials{"access_token": "xoxp-work"},
						Metadata:    map[string]string{"label": "Work"},
					},
				},
			},
		},
	}
	cfg := mergeWithDefaults(file)
	require.NotNil(t, cfg.Integrations["slack"])
	assert.True(t, cfg.Integrations["slack"].Enabled)
	assert.Equal(t, "xoxc", cfg.Integrations["slack"].Credentials["token"])
	require.Contains(t, cfg.Integrations["slack"].Identities, "work")
	assert.Equal(t, "xoxp-work", cfg.Integrations["slack"].Identities["work"].Credentials["access_token"])
	assert.Equal(t, "Work", cfg.Integrations["slack"].Identities["work"].Metadata["label"])
}

func TestLoad_PreservesIdentities(t *testing.T) {
	m, path := newTestManager(t)
	cfg := &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {
				Enabled:     true,
				Credentials: mcp.Credentials{"token": "abc"},
				Identities: map[string]mcp.IntegrationIdentity{
					"alt": {Credentials: mcp.Credentials{"token": "alt-tok"}, Metadata: map[string]string{"label": "Alt"}},
				},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, data, 0600))

	require.NoError(t, m.Load())
	ic := m.cfg.Integrations["github"]
	require.Contains(t, ic.Identities, "alt")
	assert.Equal(t, "alt-tok", ic.Identities["alt"].Credentials["token"])
	assert.Equal(t, "Alt", ic.Identities["alt"].Metadata["label"])
}

func TestDefaultConfig_SlackMCP(t *testing.T) {
	cfg := defaultConfig()
	ic, ok := cfg.Integrations["slackmcp"]
	require.True(t, ok)
	assert.False(t, ic.Enabled)
	_, hasBase := ic.Credentials["base_url"]
	assert.True(t, hasBase)
	require.NotNil(t, ic.Identities)
	assert.Empty(t, ic.Identities)
}

func TestSetIntegration_RollsBackMemoryWhenSaveFails(t *testing.T) {
	m, path := newTestManager(t)
	m.cfg = defaultConfig()
	require.NoError(t, os.Mkdir(path, 0700))

	original := m.cfg.Integrations["github"]
	err := m.SetIntegration("github", &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "new-token"},
	})

	require.Error(t, err)
	assert.Same(t, original, m.cfg.Integrations["github"])
	assert.False(t, m.cfg.Integrations["github"].Enabled)
	assert.Empty(t, m.cfg.Integrations["github"].Credentials["token"])
}

func TestSetWasmModules_DoesNotPersistEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	initial := &mcp.Config{Integrations: map[string]*mcp.IntegrationConfig{
		"github": {Enabled: true, Credentials: mcp.Credentials{"token": "disk-token"}},
	}}
	data, err := json.Marshal(initial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	m := &manager{filePath: path, envLookup: func(name string) string {
		if name == "GITHUB_TOKEN" {
			return "environment-token"
		}
		return ""
	}}
	require.NoError(t, m.Load())
	require.NoError(t, m.SetWasmModules([]mcp.WasmModuleConfig{{Path: "/tmp/plugin.wasm"}}))

	var persisted mcp.Config
	persistedData, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(persistedData, &persisted))
	assert.Equal(t, "disk-token", persisted.Integrations["github"].Credentials["token"])
}

func TestConfigReads_ReturnCopies(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())

	cfg := m.Get()
	cfg.Integrations["github"].Enabled = true
	cfg.Integrations["github"].Credentials["token"] = "mutated"
	ic, ok := m.GetIntegration("github")
	require.True(t, ok)
	ic.Enabled = true
	ic.Credentials["token"] = "mutated-again"

	stored, ok := m.GetIntegration("github")
	require.True(t, ok)
	assert.False(t, stored.Enabled)
	assert.Empty(t, stored.Credentials["token"])
}
