package wasm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loaderConfig struct {
	cfg      *mcp.Config
	setCalls int
	setErr   error
}

func newLoaderConfig(integrations map[string]*mcp.IntegrationConfig) *loaderConfig {
	return &loaderConfig{cfg: &mcp.Config{Integrations: integrations}}
}

func (c *loaderConfig) Load() error      { return nil }
func (c *loaderConfig) Save() error      { return nil }
func (c *loaderConfig) Get() *mcp.Config { return c.cfg }
func (c *loaderConfig) Update(cfg *mcp.Config) error {
	c.cfg = cfg
	return nil
}
func (c *loaderConfig) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	ic, ok := c.cfg.Integrations[name]
	return ic, ok
}
func (c *loaderConfig) SetIntegration(name string, ic *mcp.IntegrationConfig) error {
	c.setCalls++
	if c.setErr != nil {
		return c.setErr
	}
	c.cfg.Integrations[name] = ic
	return nil
}
func (c *loaderConfig) SetWasmModules(modules []mcp.WasmModuleConfig) error {
	c.cfg.WasmModules = modules
	return nil
}
func (c *loaderConfig) EnabledIntegrations() []string {
	var names []string
	for name, ic := range c.cfg.Integrations {
		if ic.Enabled {
			names = append(names, name)
		}
	}
	return names
}
func (c *loaderConfig) DefaultCredentialKeys(string) []string { return nil }

type loaderRegistry struct {
	integrations map[string]mcp.Integration
}

func newLoaderRegistry() *loaderRegistry {
	return &loaderRegistry{integrations: map[string]mcp.Integration{}}
}
func (r *loaderRegistry) Register(i mcp.Integration) error {
	r.integrations[i.Name()] = i
	return nil
}
func (r *loaderRegistry) Unregister(name string) (mcp.Integration, bool) {
	i, ok := r.integrations[name]
	delete(r.integrations, name)
	return i, ok
}
func (r *loaderRegistry) Get(name string) (mcp.Integration, bool) {
	i, ok := r.integrations[name]
	return i, ok
}
func (r *loaderRegistry) All() []mcp.Integration {
	result := make([]mcp.Integration, 0, len(r.integrations))
	for _, integration := range r.integrations {
		result = append(result, integration)
	}
	return result
}
func (r *loaderRegistry) Names() []string {
	result := make([]string, 0, len(r.integrations))
	for name := range r.integrations {
		result = append(result, name)
	}
	return result
}

func newLoaderForTest(t *testing.T, cfg *loaderConfig) (*Loader, string) {
	t.Helper()
	ctx := context.Background()
	rt, err := NewRuntime(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	path := filepath.Join(t.TempDir(), "example.wasm")
	require.NoError(t, os.WriteFile(path, exampleWasm, 0600))
	return NewLoader(rt, newLoaderRegistry(), cfg), path
}

func TestLoadPlugin_PreservesExistingConfiguration(t *testing.T) {
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{
		"example": {
			Enabled: false,
			Credentials: mcp.Credentials{
				"base_url": "https://example.com",
				"api_key":  "test-key",
			},
			ToolGlobs: []string{"example_echo"},
			Identities: map[string]mcp.IntegrationIdentity{
				"work": {Credentials: mcp.Credentials{"token": "identity-token"}},
			},
		},
	})
	loader, path := newLoaderForTest(t, cfg)

	require.NoError(t, loader.LoadPlugin(context.Background(), path, ""))

	got, ok := cfg.GetIntegration("example")
	require.True(t, ok)
	assert.False(t, got.Enabled)
	assert.Equal(t, []string{"example_echo"}, got.ToolGlobs)
	assert.Equal(t, "identity-token", got.Identities["work"].Credentials["token"])
	assert.Zero(t, cfg.setCalls, "nil identity fields must not cause a startup rewrite")
}

func TestLoadPlugin_NewPluginDefaultsDisabled(t *testing.T) {
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{})
	loader, path := newLoaderForTest(t, cfg)

	require.NoError(t, loader.LoadPlugin(context.Background(), path, ""))

	got, ok := cfg.GetIntegration("example")
	require.True(t, ok)
	assert.False(t, got.Enabled)
	assert.ElementsMatch(t, []string{"api_key", "base_url"}, mapKeysForLoaderTest(got.Credentials))
}

func TestLoadPlugin_DisabledPluginAllowsPartialCredentials(t *testing.T) {
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{
		"example": {
			Enabled:     false,
			Credentials: mcp.Credentials{"base_url": "https://example.com", "api_key": ""},
		},
	})
	loader, path := newLoaderForTest(t, cfg)

	require.NoError(t, loader.LoadPlugin(context.Background(), path, ""))
	got, ok := cfg.GetIntegration("example")
	require.True(t, ok)
	assert.False(t, got.Enabled)
}

func TestLoadPlugin_DoesNotRewriteUnchangedConfiguration(t *testing.T) {
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{
		"example": {
			Enabled: true,
			Credentials: mcp.Credentials{
				"base_url": "https://example.com",
				"api_key":  "test-key",
			},
		},
	})
	loader, path := newLoaderForTest(t, cfg)

	require.NoError(t, loader.LoadPlugin(context.Background(), path, ""))
	assert.Zero(t, cfg.setCalls, "loading an unchanged plugin must not rewrite the whole config file")
}

func TestLoadPlugin_FailsWhenConfigurationCannotPersist(t *testing.T) {
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{})
	cfg.setErr = errors.New("disk full")
	loader, path := newLoaderForTest(t, cfg)

	err := loader.LoadPlugin(context.Background(), path, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "persist WASM module")
	assert.NotContains(t, cfg.cfg.Integrations, "example")
}

func mapKeysForLoaderTest(values mcp.Credentials) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
