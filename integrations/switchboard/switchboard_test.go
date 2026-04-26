package switchboard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock config service ---

type mockConfigService struct {
	cfg *mcp.Config
}

func newMockConfigService(integrations map[string]*mcp.IntegrationConfig) *mockConfigService {
	return &mockConfigService{cfg: &mcp.Config{Integrations: integrations}}
}

func (m *mockConfigService) Load() error                  { return nil }
func (m *mockConfigService) Save() error                  { return nil }
func (m *mockConfigService) Get() *mcp.Config             { return m.cfg }
func (m *mockConfigService) Update(cfg *mcp.Config) error { m.cfg = cfg; return nil }
func (m *mockConfigService) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	ic, ok := m.cfg.Integrations[name]
	return ic, ok
}
func (m *mockConfigService) SetIntegration(name string, ic *mcp.IntegrationConfig) error {
	m.cfg.Integrations[name] = ic
	return nil
}
func (m *mockConfigService) SetWasmModules(modules []mcp.WasmModuleConfig) error {
	m.cfg.WasmModules = modules
	return nil
}
func (m *mockConfigService) EnabledIntegrations() []string {
	var names []string
	for name, ic := range m.cfg.Integrations {
		if ic.Enabled {
			names = append(names, name)
		}
	}
	return names
}
func (m *mockConfigService) DefaultCredentialKeys(name string) []string {
	defaults := map[string][]string{
		"fake":  {"api_key", "base_url"},
		"other": {"token"},
	}
	return defaults[name]
}

// --- fake integration for test setup ---

type fakeIntegration struct {
	name       string
	configured bool
	healthy    bool
	configErr  error
}

func (f *fakeIntegration) Name() string { return f.name }
func (f *fakeIntegration) Configure(_ context.Context, _ mcp.Credentials) error {
	if f.configErr != nil {
		return f.configErr
	}
	f.configured = true
	return nil
}
func (f *fakeIntegration) Tools() []mcp.ToolDefinition {
	return []mcp.ToolDefinition{
		{Name: mcp.ToolName(f.name + "_do_thing"), Description: "Does a thing"},
		{Name: mcp.ToolName(f.name + "_list_things"), Description: "Lists things"},
	}
}
func (f *fakeIntegration) Execute(_ context.Context, toolName mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: "executed " + string(toolName)}, nil
}
func (f *fakeIntegration) Healthy(_ context.Context) bool { return f.healthy }

// --- test helpers ---

func newTestServices(fakeIntegrations ...*fakeIntegration) *mcp.Services {
	reg := registry.New()
	integrations := map[string]*mcp.IntegrationConfig{}
	for _, fi := range fakeIntegrations {
		_ = reg.Register(fi)
		integrations[fi.name] = &mcp.IntegrationConfig{
			Enabled:     fi.healthy,
			Credentials: mcp.Credentials{"api_key": "test-key"},
		}
	}
	return &mcp.Services{
		Config:   newMockConfigService(integrations),
		Registry: reg,
		Metrics:  mcp.NewMetrics(),
	}
}

func newTestIntegration(services *mcp.Services) *switchboardInt {
	return &switchboardInt{services: services}
}

// --- Constructor tests ---

func TestNew(t *testing.T) {
	services := newTestServices()
	i := New(services)
	assert.Equal(t, "switchboard", i.Name())
}

func TestConfigure(t *testing.T) {
	services := newTestServices()
	i := New(services)
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.NoError(t, err)
}

func TestHealthy(t *testing.T) {
	services := newTestServices()
	i := New(services)
	assert.True(t, i.Healthy(context.Background()))
}

func TestHealthy_NilServices(t *testing.T) {
	s := &switchboardInt{}
	assert.False(t, s.Healthy(context.Background()))
}

// --- Tools metadata tests ---

func TestTools(t *testing.T) {
	services := newTestServices()
	i := New(services)
	toolList := i.Tools()
	assert.NotEmpty(t, toolList)

	seen := make(map[mcp.ToolName]bool)
	for _, tool := range toolList {
		assert.True(t, strings.HasPrefix(string(tool.Name), "switchboard_"),
			"tool %s missing switchboard_ prefix", tool.Name)
		assert.NotEmpty(t, tool.Description, "tool %s has no description", tool.Name)
		assert.False(t, seen[tool.Name], "duplicate tool: %s", tool.Name)
		seen[tool.Name] = true
	}
}

// --- Dispatch parity tests ---

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	services := newTestServices()
	i := New(services)
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	services := newTestServices()
	i := New(services)
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- Execute unknown tool ---

func TestExecute_UnknownTool(t *testing.T) {
	services := newTestServices()
	i := New(services)
	res, err := i.Execute(context.Background(), "switchboard_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "unknown tool")
}

// --- Handler tests ---

func TestListIntegrations(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	services := newTestServices(fake)
	s := newTestIntegration(services)

	res, err := listIntegrations(context.Background(), s, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &items))
	assert.NotEmpty(t, items)
	assert.Equal(t, "fake", items[0]["name"])
}

func TestListIntegrations_EnabledOnly(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	other := &fakeIntegration{name: "other", healthy: false}
	services := newTestServices(fake, other)
	s := newTestIntegration(services)

	res, err := listIntegrations(context.Background(), s, map[string]any{"enabled_only": true})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &items))
	assert.Len(t, items, 1)
	assert.Equal(t, "fake", items[0]["name"])
}

func TestGetIntegration_Found(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	services := newTestServices(fake)
	s := newTestIntegration(services)

	res, err := getIntegration(context.Background(), s, map[string]any{"name": "fake"})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &detail))
	assert.Equal(t, "fake", detail["name"])
	assert.Equal(t, float64(2), detail["tool_count"])
}

func TestGetIntegration_NotFound(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)

	res, err := getIntegration(context.Background(), s, map[string]any{"name": "nonexistent"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "integration not found")
}

func TestGetIntegration_MissingName(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)

	res, err := getIntegration(context.Background(), s, map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestConfigureIntegration_Success(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	services := newTestServices(fake)
	s := newTestIntegration(services)

	res, err := configureIntegration(context.Background(), s, map[string]any{
		"name":        "fake",
		"credentials": map[string]any{"api_key": "new-key"},
		"enabled":     true,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, "ok")

	ic, _ := services.Config.GetIntegration("fake")
	assert.Equal(t, "new-key", ic.Credentials["api_key"])
	assert.True(t, ic.Enabled)
}

func TestConfigureIntegration_Disable(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	services := newTestServices(fake)
	s := newTestIntegration(services)

	res, err := configureIntegration(context.Background(), s, map[string]any{
		"name":    "fake",
		"enabled": false,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Data, "disabled")

	ic, _ := services.Config.GetIntegration("fake")
	assert.False(t, ic.Enabled)
}

func TestConfigureIntegration_NotFound(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)

	res, err := configureIntegration(context.Background(), s, map[string]any{
		"name": "nonexistent",
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "integration not found")
}

func TestConfigureIntegration_ConfigureFails(t *testing.T) {
	fake := &fakeIntegration{
		name:      "fake",
		healthy:   true,
		configErr: assert.AnError,
	}
	services := newTestServices(fake)
	s := newTestIntegration(services)

	res, err := configureIntegration(context.Background(), s, map[string]any{
		"name":        "fake",
		"credentials": map[string]any{"api_key": "bad-key"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "configure failed")
}

func TestCheckHealth_SingleIntegration(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	services := newTestServices(fake)
	s := newTestIntegration(services)

	res, err := checkHealth(context.Background(), s, map[string]any{"name": "fake"})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &result))
	assert.Equal(t, "fake", result["name"])
	assert.Equal(t, true, result["healthy"])
}

func TestCheckHealth_NotFound(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)

	res, err := checkHealth(context.Background(), s, map[string]any{"name": "nonexistent"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "integration not found")
}

func TestCheckHealth_All(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	other := &fakeIntegration{name: "other", healthy: false}
	services := newTestServices(fake, other)
	s := newTestIntegration(services)

	res, err := checkHealth(context.Background(), s, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	var results []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &results))
	assert.Len(t, results, 2)
}

func TestBrowsePlugins_NilMarketplace(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)

	res, err := browsePlugins(context.Background(), s, map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "marketplace is not configured")
}

func TestInstallPlugin_NilMarketplace(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)

	res, err := installPlugin(context.Background(), s, map[string]any{"name": "foo"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "marketplace is not configured")
}

func TestInstallPlugin_MissingArgs(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)
	s.marketplace = nil

	res, err := installPlugin(context.Background(), s, map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestUninstallPlugin_NilMarketplace(t *testing.T) {
	services := newTestServices()
	s := newTestIntegration(services)

	res, err := uninstallPlugin(context.Background(), s, map[string]any{"name": "foo"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "marketplace is not configured")
}

func TestServerInfo(t *testing.T) {
	fake := &fakeIntegration{name: "fake", healthy: true}
	services := newTestServices(fake)
	s := newTestIntegration(services)

	res, err := serverInfo(context.Background(), s, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	var info map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Data), &info))
	assert.Equal(t, float64(1), info["total_integrations"])
	assert.Equal(t, float64(2), info["total_tools"])
}

// --- Compact specs tests ---

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	assert.Equal(t, len(rawFieldCompactionSpecs), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpec_ReturnsFields(t *testing.T) {
	s := &switchboardInt{}
	fields, ok := s.CompactSpec("switchboard_list_integrations")
	require.True(t, ok, "switchboard_list_integrations should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	s := &switchboardInt{}
	_, ok := s.CompactSpec("switchboard_configure_integration")
	assert.False(t, ok, "mutation tool should not have field compaction spec")
}
