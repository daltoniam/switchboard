package marketplace_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/marketplace"
	"github.com/daltoniam/switchboard/registry"
	wasmmod "github.com/daltoniam/switchboard/wasm"
	"github.com/daltoniam/switchboard/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func realWasmBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "wasm", "testdata", "example.wasm"))
	require.NoError(t, err, "could not read example.wasm – run from repo root")
	return data
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

type persistedConfig struct {
	mu  sync.Mutex
	cfg marketplace.Config
}

func (p *persistedConfig) save(cfg marketplace.Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = cfg
	return nil
}

func (p *persistedConfig) get() marketplace.Config {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg
}

// manifestServer starts a test HTTP server that serves both the manifest JSON
// and the real example.wasm binary. The manifest's download URL is rewritten to
// point at the test server on every request so it works with httptest.
func manifestServer(t *testing.T, wasmData []byte, manifest marketplace.Manifest) *httptest.Server {
	t.Helper()
	hash := sha256hex(wasmData)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/manifest.json":
			m := manifest
			for i := range m.Plugins {
				for j := range m.Plugins[i].Versions {
					if m.Plugins[i].Versions[j].URL == "" || strings.Contains(m.Plugins[i].Versions[j].URL, "TESTHOST") {
						m.Plugins[i].Versions[j].URL = "http://" + r.Host + "/plugins/" + m.Plugins[i].Name + ".wasm"
					}
					if m.Plugins[i].Versions[j].SHA256 == "" {
						m.Plugins[i].Versions[j].SHA256 = hash
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(m) //nolint:errcheck
		case strings.HasPrefix(r.URL.Path, "/plugins/") || r.URL.Path == "/example.wasm":
			w.Header().Set("Content-Type", "application/wasm")
			w.Write(wasmData) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newManager(t *testing.T, cfg marketplace.Config, store *persistedConfig) *marketplace.Manager {
	t.Helper()
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	return marketplace.NewManager(cfg, pluginDir, store.save)
}

// stubConfigService implements mcp.ConfigService with in-memory storage.
type stubConfigService struct {
	mu  sync.RWMutex
	cfg *mcp.Config
}

func newStubConfigService() *stubConfigService {
	return &stubConfigService{cfg: &mcp.Config{
		Integrations: map[string]*mcp.IntegrationConfig{},
	}}
}
func (s *stubConfigService) Load() error      { return nil }
func (s *stubConfigService) Save() error      { return nil }
func (s *stubConfigService) Get() *mcp.Config { s.mu.RLock(); defer s.mu.RUnlock(); return s.cfg }
func (s *stubConfigService) Update(cfg *mcp.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	return nil
}
func (s *stubConfigService) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ic, ok := s.cfg.Integrations[name]
	return ic, ok
}
func (s *stubConfigService) SetIntegration(name string, ic *mcp.IntegrationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Integrations[name] = ic
	return nil
}
func (s *stubConfigService) SetWasmModules(modules []mcp.WasmModuleConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.WasmModules = modules
	return nil
}
func (s *stubConfigService) EnabledIntegrations() []string           { return nil }
func (s *stubConfigService) DefaultCredentialKeys(_ string) []string { return nil }

// webServer spins up the real web.WebServer backed by a real marketplace
// manager and real registry, returning an httptest.Server for making requests.
func webServer(t *testing.T, mp *marketplace.Manager) *httptest.Server {
	t.Helper()
	reg := registry.New()
	cfgSvc := newStubConfigService()
	services := &mcp.Services{Config: cfgSvc, Registry: reg}
	ws := web.New(services, 0, mp, nil)
	srv := httptest.NewServer(ws.Handler())
	t.Cleanup(srv.Close)
	return srv
}

// postForm sends a POST form to the web server and returns the final response
// after following all redirects.
func postForm(t *testing.T, srvURL, path string, values url.Values) *http.Response {
	t.Helper()
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.PostForm(srvURL+path, values)
	require.NoError(t, err)
	return resp
}

func getPage(t *testing.T, srvURL, path string) string {
	t.Helper()
	resp, err := http.Get(srvURL + path)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// ─── Full lifecycle: manifest → browse → install → WASM load → update → uninstall ──

func TestE2E_FullPluginLifecycle(t *testing.T) {
	wasmData := realWasmBytes(t)
	wasmHash := sha256hex(wasmData)

	manifest := marketplace.Manifest{
		SchemaVersion: 1,
		Name:          "e2e-registry",
		Description:   "End-to-end test registry",
		Plugins: []marketplace.PluginListing{
			{
				Name:        "example",
				Description: "Example integration plugin",
				Author:      "Switchboard",
				License:     "MIT",
				Versions: []marketplace.PluginVersion{
					{Version: "1.0.0", ABIMin: 1, ABIMax: 1},
					{Version: "0.9.0", ABIMin: 1, ABIMax: 1},
				},
			},
		},
	}

	srv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}

	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: srv.URL + "/manifest.json", Name: "e2e", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)
	ctx := context.Background()

	// ── Step 1: Browse ──
	results, err := mgr.BrowsePlugins(ctx)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "example", results[0].Name)
	assert.Equal(t, "1.0.0", results[0].LatestVersion)
	assert.False(t, results[0].Installed)
	assert.Equal(t, "e2e-registry", results[0].ManifestSource)

	// ── Step 2: Install ──
	ip, err := mgr.InstallPlugin(ctx, "example")
	require.NoError(t, err)
	assert.Equal(t, "example", ip.Name)
	assert.Equal(t, "1.0.0", ip.Version)
	assert.Equal(t, wasmHash, ip.SHA256)
	assert.FileExists(t, ip.Path)

	diskData, err := os.ReadFile(ip.Path)
	require.NoError(t, err)
	assert.Equal(t, wasmData, diskData)

	persisted := store.get()
	require.Len(t, persisted.InstalledPlugins, 1)
	assert.Equal(t, "example", persisted.InstalledPlugins[0].Name)

	// ── Step 3: Load WASM with real wazero runtime ──
	wasmRT, err := wasmmod.NewRuntime(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { wasmRT.Close(ctx) }) //nolint:errcheck

	mod, err := wasmRT.LoadModule(ctx, diskData)
	require.NoError(t, err)
	t.Cleanup(func() { mod.Close(ctx) }) //nolint:errcheck

	assert.Equal(t, "example", mod.Name())
	tools := mod.Tools()
	require.GreaterOrEqual(t, len(tools), 3)

	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}
	assert.True(t, toolNames["example_echo"])
	assert.True(t, toolNames["example_http_get"])
	assert.True(t, toolNames["example_list_items"])

	err = mod.Configure(ctx, mcp.Credentials{"base_url": "https://example.com", "api_key": "k"})
	require.NoError(t, err)

	result, err := mod.Execute(ctx, "example_echo", map[string]any{"message": "e2e test"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "e2e test")

	// ── Step 4: Browse again – should show installed ──
	results2, err := mgr.BrowsePlugins(ctx)
	require.NoError(t, err)
	require.Len(t, results2, 1)
	assert.True(t, results2[0].Installed)
	assert.Equal(t, "1.0.0", results2[0].InstalledVersion)
	assert.False(t, results2[0].UpdateAvailable)

	// ── Step 5: Check for updates (none expected) ──
	updates, err := mgr.CheckForUpdates(ctx)
	require.NoError(t, err)
	assert.Len(t, updates, 0)
	assert.NotEmpty(t, store.get().LastCheck)

	// ── Step 6: Uninstall ──
	err = mgr.UninstallPlugin("example")
	require.NoError(t, err)
	assert.NoFileExists(t, ip.Path)
	assert.Len(t, store.get().InstalledPlugins, 0)
}

// ─── Install from URL with real WASM binary ──

func TestE2E_InstallFromURL(t *testing.T) {
	wasmData := realWasmBytes(t)

	wasmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(wasmData) //nolint:errcheck
	}))
	t.Cleanup(wasmSrv.Close)

	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	ip, err := mgr.InstallFromURL(context.Background(), wasmSrv.URL+"/my-plugin.wasm")
	require.NoError(t, err)
	assert.Equal(t, "my-plugin", ip.Name)
	assert.FileExists(t, ip.Path)

	diskData, err := os.ReadFile(ip.Path)
	require.NoError(t, err)
	assert.Equal(t, wasmData, diskData)

	ctx := context.Background()
	rt, err := wasmmod.NewRuntime(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { rt.Close(ctx) }) //nolint:errcheck

	mod, err := rt.LoadModule(ctx, diskData)
	require.NoError(t, err)
	t.Cleanup(func() { mod.Close(ctx) }) //nolint:errcheck
	assert.Equal(t, "example", mod.Name())
}

// ─── Install from bytes (upload) with real WASM binary ──

func TestE2E_InstallFromBytes(t *testing.T) {
	wasmData := realWasmBytes(t)
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	ip, err := mgr.InstallFromBytes("uploaded-example", wasmData)
	require.NoError(t, err)
	assert.Equal(t, "uploaded-example", ip.Name)
	assert.Equal(t, "uploaded", ip.Version)
	assert.FileExists(t, ip.Path)
	assert.Equal(t, sha256hex(wasmData), ip.SHA256)

	ctx := context.Background()
	rt, err := wasmmod.NewRuntime(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { rt.Close(ctx) }) //nolint:errcheck

	mod, err := rt.LoadModule(ctx, wasmData)
	require.NoError(t, err)
	t.Cleanup(func() { mod.Close(ctx) }) //nolint:errcheck
	assert.Equal(t, "example", mod.Name())
	assert.GreaterOrEqual(t, len(mod.Tools()), 3)
}

// ─── SHA256 verification with real binary ──

func TestE2E_SHA256Verification(t *testing.T) {
	wasmData := realWasmBytes(t)
	goodHash := sha256hex(wasmData)

	manifest := marketplace.Manifest{
		SchemaVersion: 1,
		Name:          "sha-test",
		Plugins: []marketplace.PluginListing{{
			Name: "example",
			Versions: []marketplace.PluginVersion{{
				Version: "1.0.0", ABIMin: 1, ABIMax: 1, SHA256: goodHash,
			}},
		}},
	}

	srv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: srv.URL + "/manifest.json", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)

	ip, err := mgr.InstallPlugin(context.Background(), "example")
	require.NoError(t, err)
	assert.Equal(t, goodHash, ip.SHA256)

	err = mgr.UninstallPlugin("example")
	require.NoError(t, err)

	// Now tamper the manifest hash.
	badManifest := marketplace.Manifest{
		SchemaVersion: 1,
		Name:          "sha-bad",
		Plugins: []marketplace.PluginListing{{
			Name: "example",
			Versions: []marketplace.PluginVersion{{
				Version: "1.0.0", ABIMin: 1, ABIMax: 1, SHA256: "0000000000000bad",
			}},
		}},
	}
	badSrv := manifestServer(t, wasmData, badManifest)

	mgr2 := newManager(t, marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: badSrv.URL + "/manifest.json", Enabled: true},
		},
	}, store)

	_, err = mgr2.InstallPlugin(context.Background(), "example")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SHA256 mismatch")
}

// ─── ABI filtering ──

func TestE2E_ABIFiltering(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1,
		Name:          "abi-test",
		Plugins: []marketplace.PluginListing{
			{
				Name:        "compatible",
				Description: "Works with current ABI",
				Versions:    []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 1, ABIMax: 1}},
			},
			{
				Name:        "future-only",
				Description: "Needs ABI 99",
				Versions:    []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 99, ABIMax: 99}},
			},
			{
				Name:        "past-only",
				Description: "Only ABI 0",
				Versions:    []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 0, ABIMax: 0}},
			},
		},
	}

	srv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: srv.URL + "/manifest.json", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)

	results, err := mgr.BrowsePlugins(context.Background())
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "compatible", results[0].Name)
}

// ─── Update flow: install old → detect update → update → verify ──

func TestE2E_UpdatePlugin(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1,
		Name:          "update-test",
		Plugins: []marketplace.PluginListing{{
			Name: "example",
			Versions: []marketplace.PluginVersion{
				{Version: "2.0.0", ABIMin: 1, ABIMax: 1, Changelog: "Big update"},
				{Version: "1.0.0", ABIMin: 1, ABIMax: 1},
			},
		}},
	}

	srv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	require.NoError(t, os.MkdirAll(pluginDir, 0700))

	wasmPath := filepath.Join(pluginDir, "example.wasm")
	require.NoError(t, os.WriteFile(wasmPath, wasmData, 0600))

	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: srv.URL + "/manifest.json", Enabled: true},
		},
		InstalledPlugins: []marketplace.InstalledPlugin{
			{Name: "example", Version: "1.0.0", Path: wasmPath, SHA256: sha256hex(wasmData)},
		},
	}
	mgr := marketplace.NewManager(cfg, pluginDir, store.save)

	updates, err := mgr.CheckForUpdates(context.Background())
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, "1.0.0", updates[0].CurrentVersion)
	assert.Equal(t, "2.0.0", updates[0].LatestVersion)
	assert.Equal(t, "Big update", updates[0].Changelog)

	ip, err := mgr.UpdatePlugin(context.Background(), "example")
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", ip.Version)
	assert.FileExists(t, ip.Path)

	persisted := store.get()
	require.Len(t, persisted.InstalledPlugins, 1)
	assert.Equal(t, "2.0.0", persisted.InstalledPlugins[0].Version)
}

// ─── UpdateAll with auto-update flags ──

func TestE2E_UpdateAllRespectsAutoUpdate(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1,
		Name:          "auto-test",
		Plugins: []marketplace.PluginListing{
			{Name: "plugin-a", Versions: []marketplace.PluginVersion{{Version: "2.0.0", ABIMin: 1, ABIMax: 1}}},
			{Name: "plugin-b", Versions: []marketplace.PluginVersion{{Version: "2.0.0", ABIMin: 1, ABIMax: 1}}},
		},
	}

	srv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	require.NoError(t, os.MkdirAll(pluginDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin-a.wasm"), wasmData, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin-b.wasm"), wasmData, 0600))

	cfg := marketplace.Config{
		AutoUpdate: false,
		ManifestSources: []marketplace.ManifestSource{
			{URL: srv.URL + "/manifest.json", Enabled: true},
		},
		InstalledPlugins: []marketplace.InstalledPlugin{
			{Name: "plugin-a", Version: "1.0.0", Path: filepath.Join(pluginDir, "plugin-a.wasm"), AutoUpdate: true},
			{Name: "plugin-b", Version: "1.0.0", Path: filepath.Join(pluginDir, "plugin-b.wasm"), AutoUpdate: false},
		},
	}
	mgr := marketplace.NewManager(cfg, pluginDir, store.save)

	updated, err := mgr.UpdateAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, updated, 1)
	assert.Equal(t, "plugin-a", updated[0].Name)
	assert.Equal(t, "2.0.0", updated[0].Version)
}

// ─── Multiple manifest sources ──

func TestE2E_MultipleManifestSources(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest1 := marketplace.Manifest{
		SchemaVersion: 1, Name: "official",
		Plugins: []marketplace.PluginListing{{
			Name: "plugin-alpha", Description: "From official",
			Versions: []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 1, ABIMax: 1}},
		}},
	}
	manifest2 := marketplace.Manifest{
		SchemaVersion: 1, Name: "community",
		Plugins: []marketplace.PluginListing{{
			Name: "plugin-beta", Description: "From community",
			Versions: []marketplace.PluginVersion{{Version: "0.5.0", ABIMin: 1, ABIMax: 1}},
		}},
	}

	srv1 := manifestServer(t, wasmData, manifest1)
	srv2 := manifestServer(t, wasmData, manifest2)
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: srv1.URL + "/manifest.json", Name: "official", Enabled: true},
			{URL: srv2.URL + "/manifest.json", Name: "community", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)

	results, err := mgr.BrowsePlugins(context.Background())
	require.NoError(t, err)
	assert.Len(t, results, 2)

	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	assert.True(t, names["plugin-alpha"])
	assert.True(t, names["plugin-beta"])
}

// ─── Disabled manifest source is skipped ──

func TestE2E_DisabledManifestSkipped(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1, Name: "disabled-test",
		Plugins: []marketplace.PluginListing{{
			Name:     "should-not-appear",
			Versions: []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 1, ABIMax: 1}},
		}},
	}
	srv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: srv.URL + "/manifest.json", Enabled: false},
		},
	}
	mgr := newManager(t, cfg, store)

	// All sources disabled → FetchAllManifests returns nil results, BrowsePlugins
	// propagates that as an error. Either way, no plugins should be returned.
	results, err := mgr.BrowsePlugins(context.Background())
	if err == nil {
		assert.Empty(t, results)
	}
}

// ─── Manifest source management ──

func TestE2E_ManifestSourceManagement(t *testing.T) {
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	err := mgr.AddManifestSource(marketplace.ManifestSource{URL: "https://a.com/m.json", Name: "A", Enabled: true})
	require.NoError(t, err)
	err = mgr.AddManifestSource(marketplace.ManifestSource{URL: "https://b.com/m.json", Name: "B", Enabled: true})
	require.NoError(t, err)

	cfg := store.get()
	assert.Len(t, cfg.ManifestSources, 2)

	err = mgr.AddManifestSource(marketplace.ManifestSource{URL: "https://a.com/m.json"})
	assert.Error(t, err)

	err = mgr.RemoveManifestSource("https://a.com/m.json")
	require.NoError(t, err)
	cfg = store.get()
	assert.Len(t, cfg.ManifestSources, 1)
	assert.Equal(t, "https://b.com/m.json", cfg.ManifestSources[0].URL)
}

// ─── Auto-update toggle ──

func TestE2E_AutoUpdateToggle(t *testing.T) {
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	assert.False(t, store.get().AutoUpdate)
	require.NoError(t, mgr.SetAutoUpdate(true))
	assert.True(t, store.get().AutoUpdate)
	require.NoError(t, mgr.SetAutoUpdate(false))
	assert.False(t, store.get().AutoUpdate)
}

// ─── Per-plugin auto-update ──

func TestE2E_PerPluginAutoUpdate(t *testing.T) {
	wasmData := realWasmBytes(t)
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	_, err := mgr.InstallFromBytes("test-plugin", wasmData)
	require.NoError(t, err)

	require.NoError(t, mgr.SetPluginAutoUpdate("test-plugin", true))
	cfg := store.get()
	require.Len(t, cfg.InstalledPlugins, 1)
	assert.True(t, cfg.InstalledPlugins[0].AutoUpdate)

	require.NoError(t, mgr.SetPluginAutoUpdate("test-plugin", false))
	cfg = store.get()
	assert.False(t, cfg.InstalledPlugins[0].AutoUpdate)

	err = mgr.SetPluginAutoUpdate("nonexistent", true)
	assert.Error(t, err)
}

// ─── Uninstall removes file from disk ──

func TestE2E_UninstallRemovesFile(t *testing.T) {
	wasmData := realWasmBytes(t)
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	ip, err := mgr.InstallFromBytes("to-delete", wasmData)
	require.NoError(t, err)
	assert.FileExists(t, ip.Path)

	require.NoError(t, mgr.UninstallPlugin("to-delete"))
	assert.NoFileExists(t, ip.Path)
	assert.Empty(t, mgr.InstalledPlugins())
}

// ─── Context cancellation ──

func TestE2E_ContextCancellation(t *testing.T) {
	// Use a server that hangs but respects connection close so cleanup is fast.
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(30 * time.Second):
		}
		w.WriteHeader(200)
	}))
	t.Cleanup(slowSrv.Close)

	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: slowSrv.URL + "/manifest.json", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := mgr.BrowsePlugins(ctx)
	require.Error(t, err)
}

// ─── Web UI: GET /plugins renders page ──

func TestE2E_WebUI_PluginsPage(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1, Name: "web-test",
		Plugins: []marketplace.PluginListing{{
			Name: "example", Description: "For web UI test",
			Author:   "E2E",
			Versions: []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 1, ABIMax: 1}},
		}},
	}
	mSrv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: mSrv.URL + "/manifest.json", Name: "web-test", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)
	ws := webServer(t, mgr)

	body := getPage(t, ws.URL, "/plugins")
	assert.Contains(t, body, "Plugin Marketplace")
	assert.Contains(t, body, "example")
	assert.Contains(t, body, "For web UI test")
	assert.Contains(t, body, "E2E")
	assert.Contains(t, body, "Install")
}

// ─── Web UI: install from manifest ──

func TestE2E_WebUI_InstallPlugin(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1, Name: "install-web",
		Plugins: []marketplace.PluginListing{{
			Name:     "example",
			Versions: []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 1, ABIMax: 1}},
		}},
	}
	mSrv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: mSrv.URL + "/manifest.json", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)
	ws := webServer(t, mgr)

	resp := postForm(t, ws.URL, "/plugins/install", url.Values{"name": {"example"}})
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	loc := resp.Header.Get("Location")
	assert.Contains(t, loc, "success=")
	assert.Contains(t, loc, "example")

	assert.Len(t, mgr.InstalledPlugins(), 1)
	assert.Equal(t, "1.0.0", mgr.InstalledPlugins()[0].Version)
}

// ─── Web UI: install from URL ──

func TestE2E_WebUI_InstallFromURL(t *testing.T) {
	wasmData := realWasmBytes(t)

	wasmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(wasmData) //nolint:errcheck
	}))
	t.Cleanup(wasmSrv.Close)

	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)
	ws := webServer(t, mgr)

	resp := postForm(t, ws.URL, "/plugins/install-url", url.Values{"url": {wasmSrv.URL + "/custom.wasm"}})
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "success=")
	assert.Len(t, mgr.InstalledPlugins(), 1)
}

// ─── Web UI: upload WASM file ──

func TestE2E_WebUI_UploadPlugin(t *testing.T) {
	wasmData := realWasmBytes(t)
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)
	ws := webServer(t, mgr)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("name", "uploaded-test"))
	part, err := writer.CreateFormFile("wasm", "test.wasm")
	require.NoError(t, err)
	_, err = part.Write(wasmData)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Post(ws.URL+"/plugins/upload", writer.FormDataContentType(), &buf)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "success=")

	installed := mgr.InstalledPlugins()
	require.Len(t, installed, 1)
	assert.Equal(t, "uploaded-test", installed[0].Name)
	assert.FileExists(t, installed[0].Path)
}

// ─── Web UI: uninstall ──

func TestE2E_WebUI_UninstallPlugin(t *testing.T) {
	wasmData := realWasmBytes(t)
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	_, err := mgr.InstallFromBytes("to-remove", wasmData)
	require.NoError(t, err)
	require.Len(t, mgr.InstalledPlugins(), 1)

	ws := webServer(t, mgr)
	resp := postForm(t, ws.URL, "/plugins/uninstall", url.Values{"name": {"to-remove"}})
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "success=")
	assert.Empty(t, mgr.InstalledPlugins())
}

// ─── Web UI: add / remove manifest source ──

func TestE2E_WebUI_ManifestSourceCRUD(t *testing.T) {
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)
	ws := webServer(t, mgr)

	resp := postForm(t, ws.URL, "/plugins/add-manifest", url.Values{
		"url":  {"https://example.com/plugins.json"},
		"name": {"My Plugins"},
	})
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "success=")

	cfg := mgr.Config()
	require.Len(t, cfg.ManifestSources, 1)
	assert.Equal(t, "https://example.com/plugins.json", cfg.ManifestSources[0].URL)
	assert.Equal(t, "My Plugins", cfg.ManifestSources[0].Name)
	assert.True(t, cfg.ManifestSources[0].Enabled)

	resp2 := postForm(t, ws.URL, "/plugins/remove-manifest", url.Values{
		"url": {"https://example.com/plugins.json"},
	})
	defer resp2.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp2.StatusCode)
	assert.Empty(t, mgr.Config().ManifestSources)
}

// ─── Web UI: auto-update toggle ──

func TestE2E_WebUI_AutoUpdateToggle(t *testing.T) {
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)
	ws := webServer(t, mgr)

	resp := postForm(t, ws.URL, "/plugins/auto-update", url.Values{"enabled": {"true"}})
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.True(t, mgr.Config().AutoUpdate)

	resp2 := postForm(t, ws.URL, "/plugins/auto-update", url.Values{"enabled": {"false"}})
	defer resp2.Body.Close() //nolint:errcheck
	assert.False(t, mgr.Config().AutoUpdate)
}

// ─── Web UI: check-updates ──

func TestE2E_WebUI_CheckUpdates(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1, Name: "check-web",
		Plugins: []marketplace.PluginListing{{
			Name:     "example",
			Versions: []marketplace.PluginVersion{{Version: "2.0.0", ABIMin: 1, ABIMax: 1}},
		}},
	}
	mSrv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	require.NoError(t, os.MkdirAll(pluginDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "example.wasm"), wasmData, 0600))

	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: mSrv.URL + "/manifest.json", Enabled: true},
		},
		InstalledPlugins: []marketplace.InstalledPlugin{
			{Name: "example", Version: "1.0.0", Path: filepath.Join(pluginDir, "example.wasm")},
		},
	}
	mgr := marketplace.NewManager(cfg, pluginDir, store.save)
	ws := webServer(t, mgr)

	resp := postForm(t, ws.URL, "/plugins/check-updates", url.Values{})
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	loc := resp.Header.Get("Location")
	assert.Contains(t, loc, "success=")
	assert.Contains(t, loc, "example")
}

// ─── Web UI: update plugin ──

func TestE2E_WebUI_UpdatePlugin(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1, Name: "update-web",
		Plugins: []marketplace.PluginListing{{
			Name:     "example",
			Versions: []marketplace.PluginVersion{{Version: "3.0.0", ABIMin: 1, ABIMax: 1}},
		}},
	}
	mSrv := manifestServer(t, wasmData, manifest)
	store := &persistedConfig{}
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	require.NoError(t, os.MkdirAll(pluginDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "example.wasm"), wasmData, 0600))

	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: mSrv.URL + "/manifest.json", Enabled: true},
		},
		InstalledPlugins: []marketplace.InstalledPlugin{
			{Name: "example", Version: "1.0.0", Path: filepath.Join(pluginDir, "example.wasm")},
		},
	}
	mgr := marketplace.NewManager(cfg, pluginDir, store.save)
	ws := webServer(t, mgr)

	resp := postForm(t, ws.URL, "/plugins/update", url.Values{"name": {"example"}})
	defer resp.Body.Close() //nolint:errcheck
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	loc := resp.Header.Get("Location")
	assert.Contains(t, loc, "success=")
	assert.Contains(t, loc, "3.0.0")

	installed := mgr.InstalledPlugins()
	require.Len(t, installed, 1)
	assert.Equal(t, "3.0.0", installed[0].Version)
}

// ─── Web UI: page shows installed plugin info ──

func TestE2E_WebUI_PageShowsInstalledPlugins(t *testing.T) {
	wasmData := realWasmBytes(t)
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	_, err := mgr.InstallFromBytes("my-plugin", wasmData)
	require.NoError(t, err)

	ws := webServer(t, mgr)
	body := getPage(t, ws.URL, "/plugins")
	assert.Contains(t, body, "my-plugin")
	assert.Contains(t, body, "uploaded")
	assert.Contains(t, body, "Uninstall")
}

// ─── Web UI: error cases ──

func TestE2E_WebUI_ErrorCases(t *testing.T) {
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)
	ws := webServer(t, mgr)

	t.Run("install missing name", func(t *testing.T) {
		resp := postForm(t, ws.URL, "/plugins/install", url.Values{"name": {""}})
		defer resp.Body.Close() //nolint:errcheck
		assert.Contains(t, resp.Header.Get("Location"), "error=")
	})

	t.Run("install-url missing url", func(t *testing.T) {
		resp := postForm(t, ws.URL, "/plugins/install-url", url.Values{"url": {""}})
		defer resp.Body.Close() //nolint:errcheck
		assert.Contains(t, resp.Header.Get("Location"), "error=")
	})

	t.Run("uninstall nonexistent", func(t *testing.T) {
		resp := postForm(t, ws.URL, "/plugins/uninstall", url.Values{"name": {"nope"}})
		defer resp.Body.Close() //nolint:errcheck
		assert.Contains(t, resp.Header.Get("Location"), "error=")
	})

	t.Run("update nonexistent", func(t *testing.T) {
		resp := postForm(t, ws.URL, "/plugins/update", url.Values{"name": {"nope"}})
		defer resp.Body.Close() //nolint:errcheck
		assert.Contains(t, resp.Header.Get("Location"), "error=")
	})

	t.Run("remove nonexistent manifest", func(t *testing.T) {
		resp := postForm(t, ws.URL, "/plugins/remove-manifest", url.Values{"url": {"https://nope.com"}})
		defer resp.Body.Close() //nolint:errcheck
		assert.Contains(t, resp.Header.Get("Location"), "error=")
	})
}

// ─── Web UI: nil marketplace returns page without crash ──

func TestE2E_WebUI_NilMarketplace(t *testing.T) {
	ws := webServer(t, nil)
	body := getPage(t, ws.URL, "/plugins")
	assert.Contains(t, body, "Plugin Marketplace")
}

// ─── Concurrency: parallel installs don't corrupt state ──

func TestE2E_ConcurrentInstalls(t *testing.T) {
	wasmData := realWasmBytes(t)
	store := &persistedConfig{}
	mgr := newManager(t, marketplace.Config{}, store)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", n)
			_, err := mgr.InstallFromBytes(name, wasmData)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	installed := mgr.InstalledPlugins()
	assert.Len(t, installed, 5)

	names := map[string]bool{}
	for _, ip := range installed {
		names[ip.Name] = true
		assert.FileExists(t, ip.Path)
	}
	for i := 0; i < 5; i++ {
		assert.True(t, names[fmt.Sprintf("plugin-%d", i)])
	}
}

// ─── Manifest server down ──

func TestE2E_ManifestServerDown(t *testing.T) {
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: "http://127.0.0.1:1/manifest.json", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := mgr.BrowsePlugins(ctx)
	assert.Error(t, err)
}

// ─── Web UI: GET /plugins with fetch error shows error message ──

func TestE2E_WebUI_ManifestFetchError(t *testing.T) {
	store := &persistedConfig{}
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: "http://127.0.0.1:1/bad.json", Enabled: true},
		},
	}
	mgr := newManager(t, cfg, store)
	ws := webServer(t, mgr)

	body := getPage(t, ws.URL, "/plugins")
	assert.Contains(t, body, "Plugin Marketplace")
	assert.Contains(t, body, "Failed to fetch manifests")
}

// ─── WASM metadata export (optional) ──

func TestE2E_WASMMetadataRequired(t *testing.T) {
	wasmData := realWasmBytes(t)
	ctx := context.Background()
	rt, err := wasmmod.NewRuntime(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { rt.Close(ctx) }) //nolint:errcheck

	mod, err := rt.LoadModule(ctx, wasmData)
	require.NoError(t, err)
	t.Cleanup(func() { mod.Close(ctx) }) //nolint:errcheck

	meta := mod.Metadata()
	require.NotNil(t, meta)
	assert.Equal(t, "example", meta.Name)
	assert.Equal(t, "0.1.0", meta.Version)
	assert.Equal(t, 1, meta.ABIVersion)
	assert.NotEmpty(t, meta.Description)
	assert.Equal(t, "Switchboard", meta.Author)
	assert.Equal(t, "MIT", meta.License)
	assert.Contains(t, meta.Capabilities, "http")
	assert.Equal(t, []string{"base_url", "api_key"}, meta.CredentialKeys)
	assert.Equal(t, []string{"base_url"}, meta.PlainTextKeys)
	assert.Equal(t, "https://api.example.com", meta.Placeholders["base_url"])
	assert.Equal(t, "your-api-key", meta.Placeholders["api_key"])
}

// ─── WASM credential interfaces ──

func TestE2E_WASMCredentialInterfaces(t *testing.T) {
	wasmData := realWasmBytes(t)
	ctx := context.Background()
	rt, err := wasmmod.NewRuntime(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { rt.Close(ctx) }) //nolint:errcheck

	mod, err := rt.LoadModule(ctx, wasmData)
	require.NoError(t, err)
	t.Cleanup(func() { mod.Close(ctx) }) //nolint:errcheck

	assert.Equal(t, []string{"base_url"}, mod.PlainTextKeys())
	assert.Equal(t, "https://api.example.com", mod.Placeholders()["base_url"])
	assert.Empty(t, mod.OptionalKeys())
	assert.Equal(t, []string{"base_url", "api_key"}, mod.CredentialKeys())
}

// ─── Config persistence round-trip ──

func TestE2E_ConfigPersistenceRoundTrip(t *testing.T) {
	wasmData := realWasmBytes(t)

	manifest := marketplace.Manifest{
		SchemaVersion: 1, Name: "persist-test",
		Plugins: []marketplace.PluginListing{{
			Name:     "example",
			Versions: []marketplace.PluginVersion{{Version: "1.0.0", ABIMin: 1, ABIMax: 1}},
		}},
	}
	mSrv := manifestServer(t, wasmData, manifest)

	configFile := filepath.Join(t.TempDir(), "marketplace.json")
	saveFn := func(cfg marketplace.Config) error {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(configFile, data, 0600)
	}

	pluginDir := filepath.Join(t.TempDir(), "plugins")
	cfg := marketplace.Config{
		ManifestSources: []marketplace.ManifestSource{
			{URL: mSrv.URL + "/manifest.json", Enabled: true},
		},
	}
	mgr1 := marketplace.NewManager(cfg, pluginDir, saveFn)

	_, err := mgr1.InstallPlugin(context.Background(), "example")
	require.NoError(t, err)
	require.NoError(t, mgr1.SetAutoUpdate(true))

	// Read the persisted JSON and recreate a manager from it.
	raw, err := os.ReadFile(configFile)
	require.NoError(t, err)

	var loaded marketplace.Config
	require.NoError(t, json.Unmarshal(raw, &loaded))
	assert.True(t, loaded.AutoUpdate)
	require.Len(t, loaded.InstalledPlugins, 1)
	assert.Equal(t, "example", loaded.InstalledPlugins[0].Name)
	assert.Equal(t, "1.0.0", loaded.InstalledPlugins[0].Version)
	assert.FileExists(t, loaded.InstalledPlugins[0].Path)

	mgr2 := marketplace.NewManager(loaded, pluginDir, saveFn)
	assert.Len(t, mgr2.InstalledPlugins(), 1)
	assert.True(t, mgr2.Config().AutoUpdate)
}
