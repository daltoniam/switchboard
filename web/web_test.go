package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/marketplace"
	wasmmod "github.com/daltoniam/switchboard/wasm"
	"github.com/daltoniam/switchboard/web/templates/pages"
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
	name         string
	tools        []mcp.ToolDefinition
	healthy      bool
	lastCreds    mcp.Credentials
	configureErr error
}

func (mi *mockIntegration) Name() string { return mi.name }
func (mi *mockIntegration) Configure(_ context.Context, creds mcp.Credentials) error {
	if mi.configureErr != nil {
		return mi.configureErr
	}
	mi.lastCreds = creds
	return nil
}
func (mi *mockIntegration) Tools() []mcp.ToolDefinition { return mi.tools }
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
func (r *mockRegistry) Unregister(name string) (mcp.Integration, bool) {
	i, ok := r.integrations[name]
	if ok {
		delete(r.integrations, name)
	}
	return i, ok
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
	ws := New(services, 3847, nil, nil)
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

func TestIntegrationSave_NotifiesConfigChange(t *testing.T) {
	ws, _, _ := setupTestWeb()
	called := false
	ws.onConfigChange = func() { called = true }
	handler := ws.Handler()

	form := strings.NewReader("enabled=true&cred_token=new_token_value")
	req := httptest.NewRequest("POST", "/integrations/testint", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.True(t, called)
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

func TestPluginLoadPath_LoadError(t *testing.T) {
	ws, _, cfgService := setupTestWeb()
	ws.wasmLoader = wasmmod.NewLoader(nil, nil, cfgService)
	handler := ws.Handler()

	form := strings.NewReader("path=/nonexistent/path/plugin.wasm")
	req := httptest.NewRequest("POST", "/plugins/load-path", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error")
	assert.Contains(t, rr.Header().Get("Location"), "Load+failed")
}

func TestPluginLoadPath_EmptyPath(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("path=")
	req := httptest.NewRequest("POST", "/plugins/load-path", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error")
}

func TestPluginLoadPath_NilLoader(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("path=/tmp/plugin.wasm")
	req := httptest.NewRequest("POST", "/plugins/load-path", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error")
	assert.Contains(t, rr.Header().Get("Location"), "not+configured")
}

func TestPluginLoadPath_InvalidExtension(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	form := strings.NewReader("path=/etc/passwd")
	req := httptest.NewRequest("POST", "/plugins/load-path", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "error")
	assert.Contains(t, rr.Header().Get("Location"), ".wasm")
}

// errLoader is a pluginLoader stub that returns the configured error from
// LoadPlugin. Used to exercise the live-load failure branch in marketplace
// install/upload/update handlers without spinning up a real wazero runtime.
type errLoader struct {
	loadErr error
}

func (e *errLoader) LoadPlugin(_ context.Context, _, _ string) error { return e.loadErr }
func (e *errLoader) UnloadPlugin(_ context.Context, _ string) error  { return nil }

// uploadPlugin builds a multipart upload request for POST /plugins/upload.
func uploadPlugin(t *testing.T, name string, body []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("name", name))
	part, err := writer.CreateFormFile("wasm", "plugin.wasm")
	require.NoError(t, err)
	_, err = part.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/plugins/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// TestPluginUpload_LiveLoadError verifies the upload handler surfaces a
// live-load failure as a flash error instead of a misleading success.
func TestPluginUpload_LiveLoadError(t *testing.T) {
	ws, _, _ := setupTestWeb()
	ws.marketplace = marketplace.NewManager(marketplace.Config{}, t.TempDir(), func(_ marketplace.Config) error { return nil })
	ws.wasmLoader = &errLoader{loadErr: errors.New("wasm: module does not export 'metadata'")}

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, uploadPlugin(t, "here", []byte("fake wasm")))

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	loc := rr.Header().Get("Location")
	assert.Contains(t, loc, "/plugins?error=")
	assert.Contains(t, loc, "Uploaded+here+but+load+failed")
	assert.Contains(t, loc, "module+does+not+export+%27metadata%27")
	assert.NotContains(t, loc, "success=")
}

// TestPluginUpload_LiveLoadSuccess verifies the happy path still redirects to
// the success flash when the loader returns nil.
func TestPluginUpload_LiveLoadSuccess(t *testing.T) {
	ws, _, _ := setupTestWeb()
	ws.marketplace = marketplace.NewManager(marketplace.Config{}, t.TempDir(), func(_ marketplace.Config) error { return nil })
	ws.wasmLoader = &errLoader{loadErr: nil}

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, uploadPlugin(t, "here", []byte("fake wasm")))

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	loc := rr.Header().Get("Location")
	assert.Contains(t, loc, "/plugins?success=Uploaded+and+loaded+here.")
}

// TestPluginUpload_NameEscaping verifies that a plugin name containing query
// metacharacters (&, =) is URL-encoded so it can't smuggle in extra params
// or override the flash state. Regression test for the case where an
// attacker-controlled name like "foo&success=Installed" would split into two
// query parameters in the redirect URL.
func TestPluginUpload_NameEscaping(t *testing.T) {
	ws, _, _ := setupTestWeb()
	ws.marketplace = marketplace.NewManager(marketplace.Config{}, t.TempDir(), func(_ marketplace.Config) error { return nil })
	ws.wasmLoader = &errLoader{loadErr: errors.New("boom")}

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, uploadPlugin(t, "evil&success=pwned", []byte("fake wasm")))

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	loc := rr.Header().Get("Location")
	assert.Contains(t, loc, "evil%26success%3Dpwned")
	assert.NotContains(t, loc, "&success=pwned")
}

func TestDashboard_IntegrationCounts(t *testing.T) {
	ws, reg, cfgService := setupTestWeb()

	// Add a disabled integration
	reg.Register(&mockIntegration{
		name: "disabled_int", healthy: false,
		tools: []mcp.ToolDefinition{{Name: "disabled_int_do", Description: "Do"}},
	})
	cfgService.cfg.Integrations["disabled_int"] = &mcp.IntegrationConfig{
		Enabled: false, Credentials: mcp.Credentials{},
	}

	// Add an errored integration (enabled but not healthy)
	reg.Register(&mockIntegration{
		name: "errored_int", healthy: false,
		tools: []mcp.ToolDefinition{{Name: "errored_int_do", Description: "Do"}},
	})
	cfgService.cfg.Integrations["errored_int"] = &mcp.IntegrationConfig{
		Enabled: true, Credentials: mcp.Credentials{"token": "bad"},
	}

	// Refresh health cache
	ws.health.refreshAll(context.Background())

	handler := ws.Handler()
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	// Should have the summary cards with correct counts
	assert.Contains(t, body, "Connected")
	assert.Contains(t, body, "Disabled")
	assert.Contains(t, body, "Errored")

	// Verify stat card values: 1 connected (testint), 1 disabled, 1 errored
	assert.Contains(t, body, `stat-card-green"><div class="stat-value">1</div>`)
	assert.Contains(t, body, `stat-card-muted"><div class="stat-value">1</div>`)
	assert.Contains(t, body, `stat-card-yellow"><div class="stat-value">1</div>`)

	// Should show errored integration in "Needs Attention" section
	assert.Contains(t, body, "Needs Attention")
	assert.Contains(t, body, "errored_int")
}

func TestDashboard_NoErroredHidesSection(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	// "testint" is healthy, so no "Needs Attention" section
	assert.NotContains(t, body, "Needs Attention")
}

func TestIntegrationsList_CategorizedSections(t *testing.T) {
	ws, reg, cfgService := setupTestWeb()

	// Add disabled integration
	reg.Register(&mockIntegration{
		name: "alpha_disabled", healthy: false,
		tools: []mcp.ToolDefinition{{Name: "alpha_disabled_do", Description: "Do"}},
	})
	cfgService.cfg.Integrations["alpha_disabled"] = &mcp.IntegrationConfig{
		Enabled: false, Credentials: mcp.Credentials{},
	}

	// Add errored integration (enabled but not healthy)
	reg.Register(&mockIntegration{
		name: "beta_errored", healthy: false,
		tools: []mcp.ToolDefinition{{Name: "beta_errored_do", Description: "Do"}},
	})
	cfgService.cfg.Integrations["beta_errored"] = &mcp.IntegrationConfig{
		Enabled: true, Credentials: mcp.Credentials{"token": "bad"},
	}

	ws.health.refreshAll(context.Background())

	handler := ws.Handler()
	req := httptest.NewRequest("GET", "/integrations", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	// Should have all 3 sections
	assert.Contains(t, body, "Needs Attention")
	assert.Contains(t, body, "Connected")
	assert.Contains(t, body, "Disabled")

	// Should show the integrations
	assert.Contains(t, body, "beta_errored")
	assert.Contains(t, body, "testint")
	assert.Contains(t, body, "alpha_disabled")

	// Should use card grid
	assert.Contains(t, body, "integration-grid")
	assert.Contains(t, body, "integration-card")
}

func TestIntegrationsList_NoErroredHidesSection(t *testing.T) {
	ws, _, _ := setupTestWeb()
	handler := ws.Handler()

	req := httptest.NewRequest("GET", "/integrations", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	// testint is healthy, no errored integrations
	assert.NotContains(t, body, "Needs Attention")
	assert.Contains(t, body, "Connected")
}

func TestUpdateCredentials(t *testing.T) {
	t.Run("happy path — updates token and reconfigures", func(t *testing.T) {
		ws, reg, cfgService := setupTestWeb()
		handler := ws.Handler()

		body := `{"token": "new-token-value"}`
		req := httptest.NewRequest("PUT", "/api/integrations/testint/credentials", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]string
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "true", resp["ok"])

		// Integration was reconfigured with merged creds.
		mi := reg.integrations["testint"].(*mockIntegration)
		assert.Equal(t, "new-token-value", mi.lastCreds["token"])

		// Config was persisted.
		ic, ok := cfgService.GetIntegration("testint")
		require.True(t, ok)
		assert.Equal(t, "new-token-value", ic.Credentials["token"])
		assert.True(t, ic.Enabled)
	})

	t.Run("merges with existing credentials", func(t *testing.T) {
		ws, reg, cfgService := setupTestWeb()
		cfgService.cfg.Integrations["testint"].Credentials = mcp.Credentials{
			"token":     "old-token",
			"client_id": "my-client",
		}
		handler := ws.Handler()

		body := `{"token": "refreshed-token"}`
		req := httptest.NewRequest("PUT", "/api/integrations/testint/credentials", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		mi := reg.integrations["testint"].(*mockIntegration)
		assert.Equal(t, "refreshed-token", mi.lastCreds["token"])
		assert.Equal(t, "my-client", mi.lastCreds["client_id"])
	})

	t.Run("notifies config change after successful update", func(t *testing.T) {
		ws, _, _ := setupTestWeb()
		called := false
		ws.onConfigChange = func() { called = true }
		handler := ws.Handler()

		req := httptest.NewRequest("PUT", "/api/integrations/testint/credentials", strings.NewReader(`{"token":"new"}`))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, called)
	})

	t.Run("unknown integration returns 404", func(t *testing.T) {
		ws, _, _ := setupTestWeb()
		handler := ws.Handler()

		req := httptest.NewRequest("PUT", "/api/integrations/nonexistent/credentials", strings.NewReader(`{}`))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		ws, _, _ := setupTestWeb()
		handler := ws.Handler()

		req := httptest.NewRequest("PUT", "/api/integrations/testint/credentials", strings.NewReader(`not json`))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("configure failure returns 500", func(t *testing.T) {
		ws, reg, _ := setupTestWeb()
		mi := reg.integrations["testint"].(*mockIntegration)
		mi.configureErr = fmt.Errorf("bad token")
		handler := ws.Handler()

		req := httptest.NewRequest("PUT", "/api/integrations/testint/credentials", strings.NewReader(`{"token":"bad"}`))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "bad token")
	})
}

// TestPostgresConnection_JSONRoundTrip verifies that the PostgresConnection
// struct used by the postgres setup page can round-trip through JSON without
// losing any field. The web UI serializes additional connections to JSON on
// save and unmarshals them on the next page load — without snake_case JSON
// tags, fields like connection_string and read_only would silently come back
// blank, and the user would lose the connection data on the next save.
func TestPostgresConnection_JSONRoundTrip(t *testing.T) {
	original := pages.PostgresConnection{
		Alias:            "analytics",
		ConnectionString: "postgres://user:pass@host:5432/db",
		Host:             "analytics.example.com",
		Port:             "5432",
		User:             "readonly",
		Password:         "secret",
		Database:         "analytics",
		SSLMode:          "require",
		ReadOnly:         "true",
	}

	// Round-trip as a slice — this matches how postgres_setup.templ JS gathers
	// connections and how handlePostgresSetup unmarshals them.
	encoded, err := json.Marshal([]pages.PostgresConnection{original})
	require.NoError(t, err)

	// JS produces snake_case keys; the encoded form must match that contract.
	assert.Contains(t, string(encoded), `"connection_string"`)
	assert.Contains(t, string(encoded), `"read_only"`)

	var decoded []pages.PostgresConnection
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Len(t, decoded, 1)
	assert.Equal(t, original, decoded[0])
}

// TestPostgresConnection_DecodesSnakeCase guards the specific failure mode
// the latest PR 109 review caught: without JSON tags, snake_case keys from
// the browser-side JSON do not bind to CamelCase struct fields and the
// fields come back blank.
func TestPostgresConnection_DecodesSnakeCase(t *testing.T) {
	raw := `[{
		"alias": "warehouse",
		"connection_string": "postgres://x",
		"host": "warehouse.example.com",
		"port": "5432",
		"user": "u",
		"password": "p",
		"database": "wh",
		"sslmode": "require",
		"read_only": "false"
	}]`

	var conns []pages.PostgresConnection
	require.NoError(t, json.Unmarshal([]byte(raw), &conns))
	require.Len(t, conns, 1)
	assert.Equal(t, "warehouse", conns[0].Alias)
	assert.Equal(t, "postgres://x", conns[0].ConnectionString)
	assert.Equal(t, "warehouse.example.com", conns[0].Host)
	assert.Equal(t, "false", conns[0].ReadOnly)
	assert.Equal(t, "require", conns[0].SSLMode)
}

func TestClickHouseConnection_JSONRoundTrip(t *testing.T) {
	original := pages.ClickHouseConnection{
		Alias:      "analytics",
		Host:       "analytics.example.com",
		Port:       "9440",
		Username:   "default",
		Password:   "secret",
		Database:   "analytics",
		Secure:     "true",
		SkipVerify: "false",
	}

	encoded, err := json.Marshal([]pages.ClickHouseConnection{original})
	require.NoError(t, err)

	assert.Contains(t, string(encoded), `"skip_verify"`)

	var decoded []pages.ClickHouseConnection
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Len(t, decoded, 1)
	assert.Equal(t, original, decoded[0])
}

func TestClickHouseConnection_DecodesSnakeCase(t *testing.T) {
	raw := `[{ 
		"alias": "warehouse",
		"host": "warehouse.example.com",
		"port": "9440",
		"username": "default",
		"password": "p",
		"database": "wh",
		"secure": "true",
		"skip_verify": "false"
	}]`

	var conns []pages.ClickHouseConnection
	require.NoError(t, json.Unmarshal([]byte(raw), &conns))
	require.Len(t, conns, 1)
	assert.Equal(t, "warehouse", conns[0].Alias)
	assert.Equal(t, "warehouse.example.com", conns[0].Host)
	assert.Equal(t, "false", conns[0].SkipVerify)
}
