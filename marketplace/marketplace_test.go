package marketplace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testManifest() Manifest {
	return Manifest{
		SchemaVersion: 1,
		Name:          "test-registry",
		Description:   "Test plugin registry",
		Plugins: []PluginListing{
			{
				Name:        "example",
				Description: "An example plugin",
				Author:      "test",
				Versions: []PluginVersion{
					{
						Version: "1.0.0",
						ABIMin:  1,
						ABIMax:  1,
						URL:     "", // set per test
						SHA256:  "",
					},
					{
						Version: "0.9.0",
						ABIMin:  1,
						ABIMax:  1,
						URL:     "",
						SHA256:  "",
					},
				},
			},
			{
				Name:        "future-plugin",
				Description: "Only works with ABI 99",
				Versions: []PluginVersion{
					{
						Version: "1.0.0",
						ABIMin:  99,
						ABIMax:  99,
						URL:     "",
						SHA256:  "",
					},
				},
			},
		},
	}
}

func TestBestCompatibleVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []PluginVersion
		abi      int
		want     string
	}{
		{
			name: "exact match",
			versions: []PluginVersion{
				{Version: "1.0.0", ABIMin: 1, ABIMax: 1},
			},
			abi:  1,
			want: "1.0.0",
		},
		{
			name: "picks latest compatible",
			versions: []PluginVersion{
				{Version: "0.9.0", ABIMin: 1, ABIMax: 1},
				{Version: "1.0.0", ABIMin: 1, ABIMax: 1},
			},
			abi:  1,
			want: "1.0.0",
		},
		{
			name: "no compatible version",
			versions: []PluginVersion{
				{Version: "1.0.0", ABIMin: 2, ABIMax: 3},
			},
			abi:  1,
			want: "",
		},
		{
			name: "range match",
			versions: []PluginVersion{
				{Version: "1.0.0", ABIMin: 1, ABIMax: 5},
			},
			abi:  3,
			want: "1.0.0",
		},
		{
			name:     "empty versions",
			versions: nil,
			abi:      1,
			want:     "",
		},
		{
			name: "semver: double-digit minor beats single-digit major",
			versions: []PluginVersion{
				{Version: "2.0.0", ABIMin: 1, ABIMax: 1},
				{Version: "1.10.0", ABIMin: 1, ABIMax: 1},
			},
			abi:  1,
			want: "2.0.0",
		},
		{
			name: "semver: 1.10.0 > 1.9.0",
			versions: []PluginVersion{
				{Version: "1.9.0", ABIMin: 1, ABIMax: 1},
				{Version: "1.10.0", ABIMin: 1, ABIMax: 1},
			},
			abi:  1,
			want: "1.10.0",
		},
		{
			name: "semver: patch version ordering",
			versions: []PluginVersion{
				{Version: "1.0.2", ABIMin: 1, ABIMax: 1},
				{Version: "1.0.10", ABIMin: 1, ABIMax: 1},
			},
			abi:  1,
			want: "1.0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bestCompatibleVersion(tt.versions, tt.abi)
			if tt.want == "" {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.want, got.Version)
			}
		})
	}
}

func TestFetchManifest(t *testing.T) {
	manifest := testManifest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()

	m := NewManager(Config{}, t.TempDir(), nil)
	got, err := m.FetchManifest(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "test-registry", got.Name)
	assert.Len(t, got.Plugins, 2)
}

func TestFetchManifest_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := NewManager(Config{}, t.TempDir(), nil)
	_, err := m.FetchManifest(context.Background(), srv.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestFetchManifest_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	m := NewManager(Config{}, t.TempDir(), nil)
	_, err := m.FetchManifest(context.Background(), srv.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse manifest")
}

func TestBrowsePlugins(t *testing.T) {
	manifest := testManifest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()

	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: srv.URL, Name: "test", Enabled: true},
		},
	}

	m := NewManager(cfg, t.TempDir(), nil)
	results, err := m.BrowsePlugins(context.Background())
	require.NoError(t, err)

	// "future-plugin" is ABI 99, should be filtered out
	assert.Len(t, results, 1)
	assert.Equal(t, "example", results[0].Name)
	assert.Equal(t, "1.0.0", results[0].LatestVersion)
	assert.False(t, results[0].Installed)
}

func TestBrowsePlugins_ShowsInstalledState(t *testing.T) {
	manifest := testManifest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()

	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: srv.URL, Name: "test", Enabled: true},
		},
		InstalledPlugins: []InstalledPlugin{
			{Name: "example", Version: "0.9.0", Path: "/tmp/example.wasm"},
		},
	}

	m := NewManager(cfg, t.TempDir(), nil)
	results, err := m.BrowsePlugins(context.Background())
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.True(t, results[0].Installed)
	assert.Equal(t, "0.9.0", results[0].InstalledVersion)
	assert.True(t, results[0].UpdateAvailable)
}

func TestInstallFromBytes(t *testing.T) {
	dir := t.TempDir()
	var savedCfg Config
	saveFn := func(cfg Config) error {
		savedCfg = cfg
		return nil
	}

	m := NewManager(Config{}, dir, saveFn)
	ip, err := m.InstallFromBytes("test-plugin", []byte("fake wasm data"))
	require.NoError(t, err)

	assert.Equal(t, "test-plugin", ip.Name)
	assert.Equal(t, "uploaded", ip.Version)
	assert.FileExists(t, ip.Path)
	assert.NotEmpty(t, ip.SHA256)

	// Verify saved config
	assert.Len(t, savedCfg.InstalledPlugins, 1)
	assert.Equal(t, "test-plugin", savedCfg.InstalledPlugins[0].Name)
}

func TestInstallFromBytes_EmptyName(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Config{}, dir, func(_ Config) error { return nil })
	ip, err := m.InstallFromBytes("", []byte("data"))
	require.NoError(t, err)
	assert.Contains(t, ip.Name, "uploaded-")
}

func TestUninstallPlugin(t *testing.T) {
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "test.wasm")
	require.NoError(t, os.WriteFile(wasmPath, []byte("data"), 0600))

	var savedCfg Config
	cfg := Config{
		InstalledPlugins: []InstalledPlugin{
			{Name: "test", Path: wasmPath},
		},
	}
	m := NewManager(cfg, dir, func(c Config) error {
		savedCfg = c
		return nil
	})

	err := m.UninstallPlugin("test")
	require.NoError(t, err)
	assert.Len(t, savedCfg.InstalledPlugins, 0)
	assert.NoFileExists(t, wasmPath)
}

func TestUninstallPlugin_NotFound(t *testing.T) {
	m := NewManager(Config{}, t.TempDir(), nil)
	err := m.UninstallPlugin("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

func TestAddManifestSource(t *testing.T) {
	var saved Config
	m := NewManager(Config{}, t.TempDir(), func(c Config) error {
		saved = c
		return nil
	})

	err := m.AddManifestSource(ManifestSource{URL: "https://example.com/manifest.json", Name: "test", Enabled: true})
	require.NoError(t, err)
	assert.Len(t, saved.ManifestSources, 1)
}

func TestAddManifestSource_Duplicate(t *testing.T) {
	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: "https://example.com/manifest.json", Name: "test", Enabled: true},
		},
	}
	m := NewManager(cfg, t.TempDir(), func(_ Config) error { return nil })
	err := m.AddManifestSource(ManifestSource{URL: "https://example.com/manifest.json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRemoveManifestSource(t *testing.T) {
	var saved Config
	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: "https://example.com/a.json", Enabled: true},
			{URL: "https://example.com/b.json", Enabled: true},
		},
	}
	m := NewManager(cfg, t.TempDir(), func(c Config) error {
		saved = c
		return nil
	})

	err := m.RemoveManifestSource("https://example.com/a.json")
	require.NoError(t, err)
	assert.Len(t, saved.ManifestSources, 1)
	assert.Equal(t, "https://example.com/b.json", saved.ManifestSources[0].URL)
}

func TestRemoveManifestSource_NotFound(t *testing.T) {
	m := NewManager(Config{}, t.TempDir(), func(_ Config) error { return nil })
	err := m.RemoveManifestSource("https://example.com/nonexistent.json")
	assert.Error(t, err)
}

func TestSetAutoUpdate(t *testing.T) {
	var saved Config
	m := NewManager(Config{}, t.TempDir(), func(c Config) error {
		saved = c
		return nil
	})

	err := m.SetAutoUpdate(true)
	require.NoError(t, err)
	assert.True(t, saved.AutoUpdate)
}

func TestSetPluginAutoUpdate(t *testing.T) {
	var saved Config
	cfg := Config{
		InstalledPlugins: []InstalledPlugin{
			{Name: "test", AutoUpdate: false},
		},
	}
	m := NewManager(cfg, t.TempDir(), func(c Config) error {
		saved = c
		return nil
	})

	err := m.SetPluginAutoUpdate("test", true)
	require.NoError(t, err)
	assert.True(t, saved.InstalledPlugins[0].AutoUpdate)
}

func TestSetPluginAutoUpdate_NotFound(t *testing.T) {
	m := NewManager(Config{}, t.TempDir(), func(_ Config) error { return nil })
	err := m.SetPluginAutoUpdate("nonexistent", true)
	assert.Error(t, err)
}

func TestCheckForUpdates(t *testing.T) {
	manifest := testManifest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()

	var saved Config
	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: srv.URL, Enabled: true},
		},
		InstalledPlugins: []InstalledPlugin{
			{Name: "example", Version: "0.9.0", Path: "/tmp/example.wasm"},
		},
	}
	m := NewManager(cfg, t.TempDir(), func(c Config) error {
		saved = c
		return nil
	})

	updates, err := m.CheckForUpdates(context.Background())
	require.NoError(t, err)
	assert.Len(t, updates, 1)
	assert.Equal(t, "example", updates[0].Name)
	assert.Equal(t, "0.9.0", updates[0].CurrentVersion)
	assert.Equal(t, "1.0.0", updates[0].LatestVersion)
	assert.NotEmpty(t, saved.LastCheck)
}

func TestCheckForUpdates_NoUpdates(t *testing.T) {
	manifest := testManifest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()

	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: srv.URL, Enabled: true},
		},
		InstalledPlugins: []InstalledPlugin{
			{Name: "example", Version: "1.0.0", Path: "/tmp/example.wasm"},
		},
	}
	m := NewManager(cfg, t.TempDir(), func(_ Config) error { return nil })

	updates, err := m.CheckForUpdates(context.Background())
	require.NoError(t, err)
	assert.Len(t, updates, 0)
}

func TestInstallPlugin_Download(t *testing.T) {
	wasmData := []byte("fake wasm binary data")
	wasmSHA := sha256sum(wasmData)

	manifest := Manifest{
		SchemaVersion: 1,
		Name:          "test",
		Plugins: []PluginListing{
			{
				Name:        "example",
				Description: "test",
				Versions: []PluginVersion{
					{
						Version: "1.0.0",
						ABIMin:  1,
						ABIMax:  1,
						SHA256:  wasmSHA,
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			manifest.Plugins[0].Versions[0].URL = "http://" + r.Host + "/example.wasm"
			json.NewEncoder(w).Encode(manifest)
		case "/example.wasm":
			w.Write(wasmData)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	var saved Config
	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: srv.URL + "/manifest.json", Enabled: true},
		},
	}
	m := NewManager(cfg, dir, func(c Config) error {
		saved = c
		return nil
	})

	ip, err := m.InstallPlugin(context.Background(), "example")
	require.NoError(t, err)
	assert.Equal(t, "example", ip.Name)
	assert.Equal(t, "1.0.0", ip.Version)
	assert.Equal(t, wasmSHA, ip.SHA256)
	assert.FileExists(t, ip.Path)

	data, _ := os.ReadFile(ip.Path)
	assert.Equal(t, wasmData, data)
	assert.Len(t, saved.InstalledPlugins, 1)
}

func TestInstallPlugin_SHA256Mismatch(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: 1,
		Name:          "test",
		Plugins: []PluginListing{
			{
				Name: "example",
				Versions: []PluginVersion{
					{
						Version: "1.0.0",
						ABIMin:  1,
						ABIMax:  1,
						SHA256:  "badbadbad",
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest.json" {
			manifest.Plugins[0].Versions[0].URL = "http://" + r.Host + "/example.wasm"
			json.NewEncoder(w).Encode(manifest)
		} else {
			w.Write([]byte("data"))
		}
	}))
	defer srv.Close()

	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: srv.URL + "/manifest.json", Enabled: true},
		},
	}
	m := NewManager(cfg, t.TempDir(), func(_ Config) error { return nil })

	_, err := m.InstallPlugin(context.Background(), "example")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SHA256 mismatch")
}

func TestInstallPlugin_NotFound(t *testing.T) {
	manifest := testManifest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()

	cfg := Config{
		ManifestSources: []ManifestSource{
			{URL: srv.URL, Enabled: true},
		},
	}
	m := NewManager(cfg, t.TempDir(), func(_ Config) error { return nil })

	_, err := m.InstallPlugin(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCheckInterval(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		m := NewManager(Config{}, t.TempDir(), nil)
		assert.Equal(t, 6*60, int(m.CheckInterval().Minutes()))
	})
	t.Run("custom", func(t *testing.T) {
		m := NewManager(Config{CheckInterval: "1h"}, t.TempDir(), nil)
		assert.Equal(t, 60, int(m.CheckInterval().Minutes()))
	})
	t.Run("invalid falls back to default", func(t *testing.T) {
		m := NewManager(Config{CheckInterval: "invalid"}, t.TempDir(), nil)
		assert.Equal(t, 6*60, int(m.CheckInterval().Minutes()))
	})
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"has spaces", "has_spaces"},
		{"path/sep", "path_sep"},
		{"UPPERCASE", "uppercase"},
		{"../traversal", "__traversal"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, sanitizeFilename(tt.input))
	}
}

func TestNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/plugins/my-plugin.wasm", "my-plugin"},
		{"https://example.com/plugin.wasm", "plugin"},
		{"https://example.com/", "plugin"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, nameFromURL(tt.url))
	}
}

func TestManifestSchemaVersion(t *testing.T) {
	manifest := Manifest{
		Name:    "test",
		Plugins: []PluginListing{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()

	m := NewManager(Config{}, t.TempDir(), nil)
	got, err := m.FetchManifest(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, 1, got.SchemaVersion)
}

func TestInstallFromURL(t *testing.T) {
	wasmData := []byte("url-fetched wasm")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(wasmData)
	}))
	defer srv.Close()

	dir := t.TempDir()
	var saved Config
	m := NewManager(Config{}, dir, func(c Config) error {
		saved = c
		return nil
	})

	ip, err := m.InstallFromURL(context.Background(), srv.URL+"/my-plugin.wasm")
	require.NoError(t, err)
	assert.Equal(t, "my-plugin", ip.Name)
	assert.FileExists(t, ip.Path)
	assert.Len(t, saved.InstalledPlugins, 1)
}
