package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock types for testing ---

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
func (m *mockConfigService) EnabledIntegrations() []string {
	var names []string
	for name, ic := range m.cfg.Integrations {
		if ic.Enabled {
			names = append(names, name)
		}
	}
	return names
}

type mockIntegration struct {
	name    string
	tools   []mcp.ToolDefinition
	healthy bool
}

func (mi *mockIntegration) Name() string                      { return mi.name }
func (mi *mockIntegration) Configure(_ mcp.Credentials) error { return nil }
func (mi *mockIntegration) Tools() []mcp.ToolDefinition       { return mi.tools }
func (mi *mockIntegration) Execute(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: "ok"}, nil
}
func (mi *mockIntegration) Healthy(_ context.Context) bool { return mi.healthy }

type mockRegistry struct {
	integrations map[string]mcp.Integration
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{integrations: make(map[string]mcp.Integration)}
}

func (r *mockRegistry) Register(i mcp.Integration) error {
	r.integrations[i.Name()] = i
	return nil
}
func (r *mockRegistry) Get(name string) (mcp.Integration, bool) {
	i, ok := r.integrations[name]
	return i, ok
}
func (r *mockRegistry) All() []mcp.Integration {
	result := make([]mcp.Integration, 0, len(r.integrations))
	for _, i := range r.integrations {
		result = append(result, i)
	}
	return result
}
func (r *mockRegistry) Names() []string {
	names := make([]string, 0, len(r.integrations))
	for name := range r.integrations {
		names = append(names, name)
	}
	return names
}

func setupTestWeb() (*WebServer, *mockRegistry, *mockConfigService) {
	reg := newMockRegistry()
	cfgService := newMockConfigService(map[string]*mcp.IntegrationConfig{})

	reg.Register(&mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_list", Description: "List things"},
		},
	})
	cfgService.cfg.Integrations["testint"] = &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "test"},
	}

	services := &mcp.Services{Config: cfgService, Registry: reg}
	ws := New(services, 3847)
	return ws, reg, cfgService
}

// --- tests ---

func TestNew(t *testing.T) {
	ws, _, _ := setupTestWeb()
	require.NotNil(t, ws)
	assert.Equal(t, 3847, ws.port)
}

func TestHandler(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()
	assert.NotNil(t, handler)
}

func TestHealthAPI(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "healthy")
}

func TestIntegrationSave(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("enabled=true&cred_token=new_token_value")
	req := httptest.NewRequest("POST", "/integrations/testint", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "/integrations/testint")

	ic, ok := cfgService.GetIntegration("testint")
	require.True(t, ok)
	assert.True(t, ic.Enabled)
	assert.Equal(t, "new_token_value", ic.Credentials["token"])
}

func TestIntegrationSave_NotFound(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("POST", "/integrations/nonexistent", strings.NewReader("enabled=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestIntegrationDetail_NotFound(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/integrations/nonexistent", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPageData(t *testing.T) {
	ws, _, _ := setupTestWeb()

	req := httptest.NewRequest("GET", "/?success=saved&error=oops", nil)
	data := ws.pageData(req, "Test Page", "/test")

	assert.Equal(t, "Test Page", data.Title)
	assert.Equal(t, "/test", data.CurrentPath)
	assert.Equal(t, "saved", data.FlashSuccess)
	assert.Equal(t, "oops", data.FlashError)
}

func TestHealthAPI_JSON(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}
