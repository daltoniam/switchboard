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

type manager struct {
	mu       sync.RWMutex
	cfg      *mcp.Config
	filePath string
}

// NewManager returns a ConfigService backed by a JSON file at ~/.config/switchboard/config.json.
func NewManager() (mcp.ConfigService, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	m := &manager{filePath: path}
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
				Credentials: mcp.Credentials{"api_key": "", "client_id": "", "client_secret": "", "token_source": ""},
			},
			"sentry": {
				Enabled:     false,
				Credentials: mcp.Credentials{"auth_token": "", "organization": "", "client_id": "", "token_source": ""},
			},
			"slack": {
				Enabled:     false,
				Credentials: mcp.Credentials{"token": "", "cookie": "", "client_id": "", "client_secret": "", "token_source": ""},
			},
			"metabase": {
				Enabled:     false,
				Credentials: mcp.Credentials{"api_key": "", "url": ""},
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
			return m.saveLocked()
		}
		return fmt.Errorf("read config: %w", err)
	}

	var cfg mcp.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	m.cfg = &cfg
	return nil
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
