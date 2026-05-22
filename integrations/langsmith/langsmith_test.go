package langsmith

import (
	"context"
	"testing"

	mcp "github.com/daltoniam/switchboard"
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
func (m *mockConfigService) SetWasmModules(_ []mcp.WasmModuleConfig) error { return nil }
func (m *mockConfigService) EnabledIntegrations() []string                 { return nil }
func (m *mockConfigService) DefaultCredentialKeys(_ string) []string       { return nil }

// --- constructor & name ---

func TestNew_DefaultServer(t *testing.T) {
	cfg := newMockConfigService(nil)
	i := New(cfg)
	require.NotNil(t, i)
	assert.Equal(t, "langsmith", i.Name())
	assert.Equal(t, defaultMCPServerURL, MCPServerURL(i))
}

func TestNew_OverrideServerURL(t *testing.T) {
	cfg := newMockConfigService(nil)
	i := New(cfg, "https://custom.example.com")
	require.NotNil(t, i)
	assert.Equal(t, "https://custom.example.com", MCPServerURL(i))
}

func TestNew_EmptyOverrideUsesDefault(t *testing.T) {
	cfg := newMockConfigService(nil)
	i := New(cfg, "")
	require.NotNil(t, i)
	assert.Equal(t, defaultMCPServerURL, MCPServerURL(i))
}

func TestNew_RegionConstants(t *testing.T) {
	// Each documented region resolves to a distinct LangSmith Cloud host.
	regions := []string{RegionUS, RegionEU, RegionAPAC, RegionAWS}
	seen := map[string]bool{}
	for _, r := range regions {
		assert.NotEmpty(t, r)
		assert.NotContains(t, r, "/mcp", "region must not include /mcp suffix; remotemcp appends it")
		assert.False(t, seen[r], "duplicate region URL: %s", r)
		seen[r] = true
	}
}

func TestMCPServerURL_NonLangsmith(t *testing.T) {
	// Passing an unrelated integration yields empty — important so the web
	// OAuth-start fallback chain doesn't misattribute non-langsmith URLs.
	assert.Equal(t, "", MCPServerURL(&fakeIntegration{}))
}

// --- Configure delegation ---

func TestConfigure_RequiresAccessToken(t *testing.T) {
	cfg := newMockConfigService(nil)
	i := New(cfg, "https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

func TestConfigure_ForwardsAccessTokenOnly(t *testing.T) {
	cfg := newMockConfigService(nil)
	i := New(cfg, "https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{
		"mcp_access_token": "tok",
	})
	require.NoError(t, err)
}

func TestConfigure_ForwardsFullCredentials(t *testing.T) {
	cfg := newMockConfigService(nil)
	i := New(cfg, "https://example.com")
	err := i.Configure(context.Background(), mcp.Credentials{
		"mcp_access_token":      "acc",
		"mcp_refresh_token":     "ref",
		mcp.CredKeyClientID:     "cid",
		mcp.CredKeyClientSecret: "sec",
	})
	require.NoError(t, err)
}

// --- persistTokens callback ---

func TestPersistTokens_WritesAccessToken(t *testing.T) {
	cfg := newMockConfigService(map[string]*mcp.IntegrationConfig{
		"langsmith": {
			Enabled: true,
			Credentials: mcp.Credentials{
				"mcp_access_token":  "old",
				"mcp_refresh_token": "ref",
			},
		},
	})
	l := New(cfg).(*langsmith)

	l.persistTokens(mcp.Credentials{"access_token": "new"})

	ic, _ := cfg.GetIntegration("langsmith")
	assert.Equal(t, "new", ic.Credentials["mcp_access_token"])
	assert.Equal(t, "ref", ic.Credentials["mcp_refresh_token"], "refresh_token should be untouched when not rotated")
}

func TestPersistTokens_WritesRotatedRefresh(t *testing.T) {
	cfg := newMockConfigService(map[string]*mcp.IntegrationConfig{
		"langsmith": {
			Credentials: mcp.Credentials{
				"mcp_access_token":  "old-acc",
				"mcp_refresh_token": "old-ref",
			},
		},
	})
	l := New(cfg).(*langsmith)

	l.persistTokens(mcp.Credentials{
		"access_token":  "new-acc",
		"refresh_token": "new-ref",
	})

	ic, _ := cfg.GetIntegration("langsmith")
	assert.Equal(t, "new-acc", ic.Credentials["mcp_access_token"])
	assert.Equal(t, "new-ref", ic.Credentials["mcp_refresh_token"])
}

func TestPersistTokens_NoIntegrationEntry(t *testing.T) {
	// If config doesn't have a langsmith entry, persistTokens must not panic
	// and must not implicitly create one.
	cfg := newMockConfigService(map[string]*mcp.IntegrationConfig{})
	l := New(cfg).(*langsmith)

	l.persistTokens(mcp.Credentials{"access_token": "new"})

	_, exists := cfg.GetIntegration("langsmith")
	assert.False(t, exists)
}

func TestPersistTokens_NilCfgIsNoop(t *testing.T) {
	l := &langsmith{cfg: nil}
	// Must not panic.
	l.persistTokens(mcp.Credentials{"access_token": "x"})
}

// --- helpers ---

type fakeIntegration struct{}

func (f *fakeIntegration) Name() string                                         { return "fake" }
func (f *fakeIntegration) Configure(_ context.Context, _ mcp.Credentials) error { return nil }
func (f *fakeIntegration) Tools() []mcp.ToolDefinition                          { return nil }
func (f *fakeIntegration) Execute(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
	return nil, nil
}
func (f *fakeIntegration) Healthy(_ context.Context) bool { return false }
