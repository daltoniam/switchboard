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
func (m *mockConfigService) DefaultCredentialKeys(_ string) []string { return nil }

type mockIntegration struct {
	name    string
	tools   []mcp.ToolDefinition
	healthy bool
}

func (mi *mockIntegration) Name() string                                         { return mi.name }
func (mi *mockIntegration) Configure(_ context.Context, _ mcp.Credentials) error { return nil }
func (mi *mockIntegration) Tools() []mcp.ToolDefinition                          { return mi.tools }
func (mi *mockIntegration) Execute(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
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
			{Name: mcp.ToolName("testint_list"), Description: "List things"},
		},
	})
	cfgService.cfg.Integrations["testint"] = &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "test"},
	}

	services := &mcp.Services{Config: cfgService, Registry: reg}
	ws := New(services, 3847, nil)
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

func TestIntegrationSave_PreservesToolGlobs(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.Integrations["testint"].ToolGlobs = []string{"testint_list_*", "testint_get_*"}
	handler := ws.Handler()

	form := strings.NewReader("enabled=true&cred_token=updated_token")
	req := httptest.NewRequest("POST", "/integrations/testint", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)

	ic, ok := cfgService.GetIntegration("testint")
	require.True(t, ok)
	assert.Equal(t, "updated_token", ic.Credentials["token"])
	assert.Equal(t, []string{"testint_list_*", "testint_get_*"}, ic.ToolGlobs)
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

func TestHealthRefresh(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("POST", "/api/health/refresh", nil)
	req.Header.Set("Referer", "/integrations")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations", rr.Header().Get("Location"))

	entry, ok := ws.health.get("testint")
	require.True(t, ok)
	assert.True(t, entry.Healthy)
}

func TestHealthRefresh_NoReferer(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("POST", "/api/health/refresh", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
}

func TestIntegrationSummaries_UsesCache(t *testing.T) {
	ws, _, _ := setupTestWeb()

	summaries := ws.integrationSummaries(context.Background())
	require.Len(t, summaries, 1)

	s := summaries[0]
	assert.Equal(t, "testint", s.Name)
	assert.True(t, s.Healthy)
	assert.True(t, s.Enabled)
	assert.False(t, s.LastCheck.IsZero())
}

func TestMetricsAPI_NilMetrics(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/metrics", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "metrics not initialized")
}

func TestMetricsAPI(t *testing.T) {
	ws, _, _ := setupTestWeb()
	ws.services.Metrics = mcp.NewMetrics()
	handler := ws.Handler()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/metrics", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body, "uptime_seconds")
	assert.Contains(t, body, "total_executions")
	assert.Contains(t, body, "tools")
	assert.Contains(t, body, "integrations")
}

func TestWasmModulesPage_Empty(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/wasm", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "WASM Modules")
	assert.Contains(t, rr.Body.String(), "No WASM modules configured")
}

func TestWasmModulesPage_WithModules(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/test.wasm"},
		{Path: "/tmp/other.wasm", Credentials: mcp.Credentials{"key": "val"}},
	}
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/wasm", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "/tmp/test.wasm")
	assert.Contains(t, rr.Body.String(), "/tmp/other.wasm")
	assert.Contains(t, rr.Body.String(), "1 credential(s) configured")
}

func TestWasmModuleAdd(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("path=/tmp/new.wasm&cred_key_0=api_key&cred_val_0=secret")
	req := httptest.NewRequest("POST", "/wasm", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "/wasm")
	assert.Contains(t, rr.Header().Get("Location"), "success")

	require.Len(t, cfgService.cfg.WasmModules, 1)
	assert.Equal(t, "/tmp/new.wasm", cfgService.cfg.WasmModules[0].Path)
	assert.Equal(t, "secret", cfgService.cfg.WasmModules[0].Credentials["api_key"])
}

func TestWasmModuleAdd_EmptyPath(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("path=")
	req := httptest.NewRequest("POST", "/wasm", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error")
}

func TestWasmModuleAdd_NoCreds(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("path=/tmp/plain.wasm")
	req := httptest.NewRequest("POST", "/wasm", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	require.Len(t, cfgService.cfg.WasmModules, 1)
	assert.Nil(t, cfgService.cfg.WasmModules[0].Credentials)
}

func TestWasmModuleDelete(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/a.wasm"},
		{Path: "/tmp/b.wasm"},
		{Path: "/tmp/c.wasm"},
	}
	handler := ws.Handler()

	form := strings.NewReader("index=1")
	req := httptest.NewRequest("POST", "/wasm/delete", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "success")

	require.Len(t, cfgService.cfg.WasmModules, 2)
	assert.Equal(t, "/tmp/a.wasm", cfgService.cfg.WasmModules[0].Path)
	assert.Equal(t, "/tmp/c.wasm", cfgService.cfg.WasmModules[1].Path)
}

func TestWasmModuleDelete_InvalidIndex(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/a.wasm"},
	}
	handler := ws.Handler()

	form := strings.NewReader("index=5")
	req := httptest.NewRequest("POST", "/wasm/delete", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error")
	require.Len(t, cfgService.cfg.WasmModules, 1)
}

func TestWasmModuleDelete_BadIndex(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	for _, idx := range []string{"abc", "1abc", ""} {
		form := strings.NewReader("index=" + idx)
		req := httptest.NewRequest("POST", "/wasm/delete", form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusSeeOther, rr.Code, "index=%q", idx)
		assert.Contains(t, rr.Header().Get("Location"), "error", "index=%q", idx)
	}
}

func TestWasmModuleEdit(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/test.wasm", Credentials: mcp.Credentials{"key": "val"}},
	}
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/wasm/0", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Edit WASM Module")
	assert.Contains(t, rr.Body.String(), "/tmp/test.wasm")
	assert.Contains(t, rr.Body.String(), "key")
	assert.Contains(t, rr.Body.String(), "val")
}

func TestWasmModuleEdit_InvalidIndex(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/test.wasm"},
	}
	handler := ws.Handler()

	for _, idx := range []string{"5", "-1", "abc"} {
		req := httptest.NewRequest("GET", "/wasm/"+idx, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusSeeOther, rr.Code, "index=%q", idx)
		assert.Contains(t, rr.Header().Get("Location"), "error", "index=%q", idx)
	}
}

func TestWasmModuleUpdate(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/old.wasm", Credentials: mcp.Credentials{"old_key": "old_val"}},
		{Path: "/tmp/other.wasm"},
	}
	handler := ws.Handler()

	form := strings.NewReader("path=/tmp/new.wasm&cred_key_0=new_key&cred_val_0=new_val")
	req := httptest.NewRequest("POST", "/wasm/0", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "success")

	require.Len(t, cfgService.cfg.WasmModules, 2)
	assert.Equal(t, "/tmp/new.wasm", cfgService.cfg.WasmModules[0].Path)
	assert.Equal(t, "new_val", cfgService.cfg.WasmModules[0].Credentials["new_key"])
	assert.Equal(t, "/tmp/other.wasm", cfgService.cfg.WasmModules[1].Path)
}

func TestWasmModuleUpdate_EmptyPath(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/old.wasm"},
	}
	handler := ws.Handler()

	form := strings.NewReader("path=")
	req := httptest.NewRequest("POST", "/wasm/0", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error")
	assert.Equal(t, "/tmp/old.wasm", cfgService.cfg.WasmModules[0].Path)
}

func TestWasmModuleUpdate_InvalidIndex(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/test.wasm"},
	}
	handler := ws.Handler()

	for _, idx := range []string{"5", "-1", "abc"} {
		form := strings.NewReader("path=/tmp/new.wasm")
		req := httptest.NewRequest("POST", "/wasm/"+idx, form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusSeeOther, rr.Code, "index=%q", idx)
		assert.Contains(t, rr.Header().Get("Location"), "error", "index=%q", idx)
	}
	assert.Equal(t, "/tmp/test.wasm", cfgService.cfg.WasmModules[0].Path)
}

func TestWasmModuleUpdate_NoCreds(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/old.wasm", Credentials: mcp.Credentials{"key": "val"}},
	}
	handler := ws.Handler()

	form := strings.NewReader("path=/tmp/updated.wasm")
	req := httptest.NewRequest("POST", "/wasm/0", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "success")
	assert.Equal(t, "/tmp/updated.wasm", cfgService.cfg.WasmModules[0].Path)
	assert.Nil(t, cfgService.cfg.WasmModules[0].Credentials)
}

func TestWasmModuleAdd_WithName(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("path=/tmp/new.wasm&name=custom")
	req := httptest.NewRequest("POST", "/wasm", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "success")

	require.Len(t, cfgService.cfg.WasmModules, 1)
	assert.Equal(t, "/tmp/new.wasm", cfgService.cfg.WasmModules[0].Path)
	assert.Equal(t, "custom", cfgService.cfg.WasmModules[0].Name)
}

func TestWasmModuleEdit_ShowsName(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/test.wasm", Name: "mymod"},
	}
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/wasm/0", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "mymod")
}

func TestWasmModuleUpdate_WithName(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/old.wasm"},
	}
	handler := ws.Handler()

	form := strings.NewReader("path=/tmp/old.wasm&name=renamed")
	req := httptest.NewRequest("POST", "/wasm/0", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "success")
	assert.Equal(t, "renamed", cfgService.cfg.WasmModules[0].Name)
}

func TestWasmModulesPage_ShowsName(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	cfgService.cfg.WasmModules = []mcp.WasmModuleConfig{
		{Path: "/tmp/test.wasm", Name: "custom_name"},
	}
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/wasm", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "custom_name")
}
