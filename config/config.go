package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

const (
	configDir  = "switchboard"
	configFile = "config.json"
)

// envMapping maps integration credential keys to environment variable names.
// When an env var is set, it overrides the corresponding JSON config value.
// These use standard/conventional env var names where they exist.
var envMapping = map[string]map[string]string{
	"github": {
		"token": "GITHUB_TOKEN",
	},
	"datadog": {
		"api_key": "DD_API_KEY",
		"app_key": "DD_APP_KEY",
		"site":    "DD_SITE",
	},
	"linear": {
		"api_key": "LINEAR_API_KEY",
	},
	"sentry": {
		"auth_token":   "SENTRY_AUTH_TOKEN",
		"organization": "SENTRY_ORG",
	},
	"slack": {
		"token":   "SLACK_TOKEN",
		"cookie":  "SLACK_COOKIE",
		"team_id": "SLACK_TEAM_ID",
	},
	"metabase": {
		"api_key": "METABASE_API_KEY",
		"url":     "METABASE_URL",
	},
	"aws": {
		"access_key_id":     "AWS_ACCESS_KEY_ID",
		"secret_access_key": "AWS_SECRET_ACCESS_KEY",
		"session_token":     "AWS_SESSION_TOKEN",
		"region":            "AWS_REGION",
	},
	"posthog": {
		"api_key":    "POSTHOG_API_KEY",
		"project_id": "POSTHOG_PROJECT_ID",
		"base_url":   "POSTHOG_URL",
	},
	"jira": {
		"email":     "JIRA_EMAIL",
		"api_token": "JIRA_API_TOKEN",
		"domain":    "JIRA_DOMAIN",
	},
	"confluence": {
		"email":     "CONFLUENCE_EMAIL",
		"api_token": "CONFLUENCE_API_TOKEN",
		"domain":    "CONFLUENCE_DOMAIN",
	},
	"postgres": {
		"connection_string": "DATABASE_URL",
		"host":              "PGHOST",
		"port":              "PGPORT",
		"user":              "PGUSER",
		"password":          "PGPASSWORD",
		"database":          "PGDATABASE",
		"sslmode":           "PGSSLMODE",
	},
	"rwx": {
		"access_token": "RWX_ACCESS_TOKEN",
		"cli_path":     "RWX_CLI_PATH",
	},
	"overmind": {
		"base_url":     "OVERMIND_URL",
		"token":        "OVERMIND_TOKEN",
		"agent_run_id": "OVERMIND_AGENT_RUN_ID",
		"flow_run_id":  "OVERMIND_FLOW_RUN_ID",
	},
}

// EnvMapping returns the env var mapping table. Useful for documentation and debugging.
func EnvMapping() map[string]map[string]string {
	return envMapping
}

type manager struct {
	mu        sync.RWMutex
	cfg       *mcp.Config
	filePath  string
	envLookup func(string) string // defaults to os.Getenv; override in tests
}

// NewManager returns a ConfigService backed by a JSON file at ~/.config/switchboard/config.json.
// After loading the JSON config, environment variables are overlaid on top.
// Any env var that maps to an integration credential will override the JSON value.
func NewManager() (mcp.ConfigService, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	m := &manager{filePath: path, envLookup: os.Getenv}
	if err := m.Load(); err != nil {
		return nil, err
	}
	return m, nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".config", configDir, configFile), nil
}

func defaultConfig() *mcp.Config {
	return &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{
			"github": {
				Enabled:     false,
				Credentials: mcp.Credentials{"token": "", "client_id": "", "token_source": ""},
			},
			"datadog": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": "", "app_key": ""},
			},
			"linear": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": "", "mcp_access_token": "", "token_source": ""},
			},
			"sentry": {
				Enabled:     false,
				Credentials: mcp.Credentials{"auth_token": "", "organization": "", "client_id": "", "token_source": ""},
			},
			"slack": {
				Enabled:     false,
				Credentials: mcp.Credentials{"token": "", "cookie": "", "team_id": "", "token_source": ""},
			},
			"metabase": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": "", "url": ""},
			},
			"aws": {
				Enabled:     false,
				Credentials: mcp.Credentials{"access_key_id": "", "secret_access_key": "", "session_token": "", "region": ""},
			},
			"posthog": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": "", "project_id": "", "base_url": ""},
			},
			"postgres": {
				Enabled:     false,
				Credentials: mcp.Credentials{"connection_string": "", "host": "", "port": "", "user": "", "password": "", "database": "", "sslmode": "", "read_only": ""},
			},
			"clickhouse": {
				Enabled:     false,
				Credentials: mcp.Credentials{"host": "", "port": "", "username": "", "password": "", "database": "", "secure": "", "skip_verify": ""},
			},
			"pganalyze": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": "", "base_url": "", "organization_slug": ""},
			},
			"rwx": {
				Enabled:     false,
				Credentials: mcp.Credentials{"access_token": "", "cli_path": ""},
			},
			"gmail": {
				Enabled:     false,
				Credentials: mcp.Credentials{"access_token": "", "refresh_token": "", "client_id": "", "client_secret": "", "base_url": "", "token_source": ""},
			},
			"homeassistant": {
				Enabled:     false,
				Credentials: mcp.Credentials{"token": "", "base_url": ""},
			},
			"notion": {
				Enabled:     false,
				Credentials: mcp.Credentials{"token_v2": ""},
			},
			"ynab": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": ""},
			},
			"jira": {
				Enabled:     false,
				Credentials: mcp.Credentials{"email": "", "api_token": "", "domain": ""},
			},
			"confluence": {
				Enabled:     false,
				Credentials: mcp.Credentials{"email": "", "api_token": "", "domain": ""},
			},
			"gcp": {
				Enabled:     false,
				Credentials: mcp.Credentials{"project_id": "", "credentials_json": ""},
			},
			"suno": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": "", "base_url": ""},
			},
			"amazon": {
				Enabled:     false,
				Credentials: mcp.Credentials{"email": "", "password": "", "otp_secret": "", "cookies": "", "domain": ""},
			},
			"overmind": {
				Enabled:     false,
				Credentials: mcp.Credentials{"base_url": "", "token": "", "agent_run_id": "", "flow_run_id": ""},
			},
		},
	}
}

func (m *manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.cfg = defaultConfig()
			if saveErr := m.saveLocked(); saveErr != nil {
				return saveErr
			}
			m.applyEnvOverrides()
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}

	var cfg mcp.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	m.cfg = mergeWithDefaults(&cfg)
	// Validate user-supplied globs from the config file (defaults have no globs).
	for name, ic := range cfg.Integrations {
		if err := mcp.ValidateToolGlobs(ic.ToolGlobs); err != nil {
			return fmt.Errorf("config: integration %q: %w", name, err)
		}
	}
	m.applyEnvOverrides()
	return nil
}

func mergeWithDefaults(file *mcp.Config) *mcp.Config {
	cfg := defaultConfig()
	cfg.WasmModules = file.WasmModules
	if file.Integrations == nil {
		return cfg
	}
	for name, fileIC := range file.Integrations {
		defIC, ok := cfg.Integrations[name]
		if !ok {
			cfg.Integrations[name] = fileIC
			continue
		}
		defIC.Enabled = fileIC.Enabled
		defIC.ToolGlobs = fileIC.ToolGlobs
		for k, v := range fileIC.Credentials {
			defIC.Credentials[k] = v
		}
	}
	return cfg
}

func (m *manager) applyEnvOverrides() {
	if m.cfg.Integrations == nil {
		return
	}
	for integration, mapping := range envMapping {
		ic, ok := m.cfg.Integrations[integration]
		if !ok {
			ic = &mcp.IntegrationConfig{
				Credentials: mcp.Credentials{},
			}
			m.cfg.Integrations[integration] = ic
		}
		if ic.Credentials == nil {
			ic.Credentials = mcp.Credentials{}
		}
		for credKey, envVar := range mapping {
			if val := m.envLookup(envVar); val != "" {
				ic.Credentials[credKey] = val
			}
		}
	}
}

func (m *manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

func (m *manager) saveLocked() error {
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (m *manager) Get() *mcp.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *manager) Update(cfg *mcp.Config) error {
	for name, ic := range cfg.Integrations {
		if err := mcp.ValidateToolGlobs(ic.ToolGlobs); err != nil {
			return fmt.Errorf("integration %q: %w", name, err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	return m.saveLocked()
}

func (m *manager) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ic, ok := m.cfg.Integrations[name]
	return ic, ok
}

func (m *manager) SetIntegration(name string, ic *mcp.IntegrationConfig) error {
	if err := mcp.ValidateToolGlobs(ic.ToolGlobs); err != nil {
		return fmt.Errorf("integration %q: %w", name, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.Integrations[name] = ic
	return m.saveLocked()
}

func (m *manager) EnabledIntegrations() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var names []string
	for name, ic := range m.cfg.Integrations {
		if ic.Enabled {
			names = append(names, name)
		}
	}
	return names
}

func (m *manager) DefaultCredentialKeys(name string) []string {
	def := defaultConfig()
	ic, ok := def.Integrations[name]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(ic.Credentials))
	for k := range ic.Credentials {
		keys = append(keys, k)
	}
	return keys
}
