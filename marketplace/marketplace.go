package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
)

// ABIVersion is the current plugin ABI version that this build of Switchboard supports.
// Plugins declare a min/max ABI range; the host only loads plugins whose range includes this value.
const ABIVersion = 1

// Manifest describes a collection of available plugins from a single source.
type Manifest struct {
	// SchemaVersion allows future manifest format changes.
	SchemaVersion int             `json:"schema_version"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	Plugins       []PluginListing `json:"plugins"`
}

// PluginListing describes a single plugin available for installation.
type PluginListing struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Author      string          `json:"author,omitempty"`
	Homepage    string          `json:"homepage,omitempty"`
	License     string          `json:"license,omitempty"`
	Versions    []PluginVersion `json:"versions"`
}

// PluginVersion describes a specific version of a plugin.
type PluginVersion struct {
	Version    string   `json:"version"`
	ABIMin     int      `json:"abi_min"`
	ABIMax     int      `json:"abi_max"`
	URL        string   `json:"url"`
	SHA256     string   `json:"sha256"`
	Size       int64    `json:"size,omitempty"`
	ReleasedAt string   `json:"released_at,omitempty"`
	Changelog  string   `json:"changelog,omitempty"`
	Platforms  []string `json:"platforms,omitempty"` // empty = all platforms
}

// PluginMetadata is embedded in the WASM binary and returned by the `metadata()` export.
type PluginMetadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	ABIVersion   int      `json:"abi_version"`
	Description  string   `json:"description,omitempty"`
	Author       string   `json:"author,omitempty"`
	Homepage     string   `json:"homepage,omitempty"`
	License      string   `json:"license,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// InstalledPlugin tracks a plugin installed via the marketplace.
type InstalledPlugin struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	ManifestURL   string `json:"manifest_url,omitempty"`
	InstalledAt   string `json:"installed_at"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	AutoUpdate    bool   `json:"auto_update"`
	LatestVersion string `json:"latest_version,omitempty"`
}

// ManifestSource is a configured manifest URL.
type ManifestSource struct {
	URL     string `json:"url"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
}

// Config is the marketplace section of the switchboard config.
type Config struct {
	ManifestSources  []ManifestSource  `json:"manifest_sources,omitempty"`
	InstalledPlugins []InstalledPlugin `json:"installed_plugins,omitempty"`
	AutoUpdate       bool              `json:"auto_update"`
	CheckInterval    string            `json:"check_interval,omitempty"` // duration string, default "6h"
	PluginDir        string            `json:"plugin_dir,omitempty"`     // default ~/.config/switchboard/plugins/
	LastCheck        string            `json:"last_check,omitempty"`
}

// Manager handles plugin discovery, installation, and updates.
type Manager struct {
	mu        sync.RWMutex
	cfg       Config
	pluginDir string
	client    *http.Client
	saveFn    func(Config) error
	manifests map[string]*Manifest // URL -> manifest cache
}

// NewManager creates a marketplace manager.
func NewManager(cfg Config, pluginDir string, saveFn func(Config) error) *Manager {
	if pluginDir == "" {
		home, _ := os.UserHomeDir()
		pluginDir = filepath.Join(home, ".config", "switchboard", "plugins")
	}
	if cfg.PluginDir != "" {
		pluginDir = cfg.PluginDir
	}
	return &Manager{
		cfg:       cfg,
		pluginDir: pluginDir,
		client:    &http.Client{Timeout: 60 * time.Second},
		saveFn:    saveFn,
		manifests: make(map[string]*Manifest),
	}
}

// PluginDir returns the directory where plugins are stored.
func (m *Manager) PluginDir() string {
	return m.pluginDir
}

// Config returns the current marketplace config.
func (m *Manager) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// FetchManifest downloads and parses a manifest from a URL.
func (m *Manager) FetchManifest(ctx context.Context, url string) (*Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Switchboard-Plugin-Manager/1.0")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = 1
	}

	m.mu.Lock()
	m.manifests[url] = &manifest
	m.mu.Unlock()

	return &manifest, nil
}

// FetchAllManifests fetches all enabled manifest sources.
func (m *Manager) FetchAllManifests(ctx context.Context) ([]*Manifest, error) {
	m.mu.RLock()
	sources := make([]ManifestSource, len(m.cfg.ManifestSources))
	copy(sources, m.cfg.ManifestSources)
	m.mu.RUnlock()

	var results []*Manifest
	var errs []string
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		manifest, err := m.FetchManifest(ctx, src.URL)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", src.URL, err))
			continue
		}
		results = append(results, manifest)
	}
	if len(errs) > 0 && len(results) == 0 {
		return nil, fmt.Errorf("all manifest fetches failed: %s", strings.Join(errs, "; "))
	}
	return results, nil
}

// BrowsePlugins returns all available plugins across all manifests,
// filtered to only those compatible with the current ABI version.
func (m *Manager) BrowsePlugins(ctx context.Context) ([]BrowseResult, error) {
	manifests, err := m.FetchAllManifests(ctx)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	installed := make(map[string]InstalledPlugin)
	for _, ip := range m.cfg.InstalledPlugins {
		installed[ip.Name] = ip
	}
	m.mu.RUnlock()

	var results []BrowseResult
	for _, mf := range manifests {
		for _, pl := range mf.Plugins {
			best := bestCompatibleVersion(pl.Versions, ABIVersion)
			if best == nil {
				continue
			}
			br := BrowseResult{
				Name:           pl.Name,
				Description:    pl.Description,
				Author:         pl.Author,
				Homepage:       pl.Homepage,
				License:        pl.License,
				LatestVersion:  best.Version,
				ManifestSource: mf.Name,
			}
			if ip, ok := installed[pl.Name]; ok {
				br.Installed = true
				br.InstalledVersion = ip.Version
				br.UpdateAvailable = ip.Version != best.Version
			}
			results = append(results, br)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

// BrowseResult is a plugin available in the marketplace.
type BrowseResult struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Author           string `json:"author,omitempty"`
	Homepage         string `json:"homepage,omitempty"`
	License          string `json:"license,omitempty"`
	LatestVersion    string `json:"latest_version"`
	ManifestSource   string `json:"manifest_source"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version,omitempty"`
	UpdateAvailable  bool   `json:"update_available,omitempty"`
}

// InstallPlugin downloads and installs a plugin by name from available manifests.
func (m *Manager) InstallPlugin(ctx context.Context, name string) (*InstalledPlugin, error) {
	if err := os.MkdirAll(m.pluginDir, 0700); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	m.mu.RLock()
	sources := make([]ManifestSource, len(m.cfg.ManifestSources))
	copy(sources, m.cfg.ManifestSources)
	m.mu.RUnlock()

	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		manifest, err := m.FetchManifest(ctx, src.URL)
		if err != nil {
			continue
		}
		for _, pl := range manifest.Plugins {
			if pl.Name != name {
				continue
			}
			ver := bestCompatibleVersion(pl.Versions, ABIVersion)
			if ver == nil {
				return nil, fmt.Errorf("no compatible version of %q for ABI %d", name, ABIVersion)
			}
			return m.downloadAndInstall(ctx, pl.Name, ver, src.URL)
		}
	}
	return nil, fmt.Errorf("plugin %q not found in any manifest", name)
}

// InstallFromURL downloads a WASM file from a URL and installs it.
func (m *Manager) InstallFromURL(ctx context.Context, url string) (*InstalledPlugin, error) {
	if err := os.MkdirAll(m.pluginDir, 0700); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	data, err := m.downloadWasm(ctx, url)
	if err != nil {
		return nil, err
	}

	hash := sha256sum(data)
	name := nameFromURL(url)
	filename := name + ".wasm"
	destPath := filepath.Join(m.pluginDir, filename)

	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return nil, fmt.Errorf("write plugin: %w", err)
	}

	ip := InstalledPlugin{
		Name:        name,
		Version:     "unknown",
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		Path:        destPath,
		SHA256:      hash,
		AutoUpdate:  false,
	}

	m.mu.Lock()
	m.cfg.InstalledPlugins = append(m.cfg.InstalledPlugins, ip)
	m.mu.Unlock()

	if err := m.save(); err != nil {
		return nil, err
	}
	return &ip, nil
}

// InstallFromBytes installs a WASM binary uploaded directly (e.g., from browser).
func (m *Manager) InstallFromBytes(name string, data []byte) (*InstalledPlugin, error) {
	if err := os.MkdirAll(m.pluginDir, 0700); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	hash := sha256sum(data)
	if name == "" {
		name = "uploaded-" + hash[:8]
	}
	filename := sanitizeFilename(name) + ".wasm"
	destPath := filepath.Join(m.pluginDir, filename)

	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return nil, fmt.Errorf("write plugin: %w", err)
	}

	ip := InstalledPlugin{
		Name:        name,
		Version:     "uploaded",
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		Path:        destPath,
		SHA256:      hash,
		AutoUpdate:  false,
	}

	m.mu.Lock()
	m.cfg.InstalledPlugins = append(m.cfg.InstalledPlugins, ip)
	m.mu.Unlock()

	if err := m.save(); err != nil {
		return nil, err
	}
	return &ip, nil
}

// UninstallPlugin removes an installed plugin.
func (m *Manager) UninstallPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var remaining []InstalledPlugin
	var found bool
	for _, ip := range m.cfg.InstalledPlugins {
		if ip.Name == name {
			found = true
			_ = os.Remove(ip.Path)
			continue
		}
		remaining = append(remaining, ip)
	}
	if !found {
		return fmt.Errorf("plugin %q not installed", name)
	}
	m.cfg.InstalledPlugins = remaining
	return m.saveLocked()
}

// CheckForUpdates checks all installed plugins for available updates.
func (m *Manager) CheckForUpdates(ctx context.Context) ([]UpdateResult, error) {
	manifests, err := m.FetchAllManifests(ctx)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	installed := make([]InstalledPlugin, len(m.cfg.InstalledPlugins))
	copy(installed, m.cfg.InstalledPlugins)
	m.mu.RUnlock()

	m.mu.RLock()
	sources := make([]ManifestSource, len(m.cfg.ManifestSources))
	copy(sources, m.cfg.ManifestSources)
	manifestCache := make(map[string]*Manifest, len(m.manifests))
	for k, v := range m.manifests {
		manifestCache[k] = v
	}
	m.mu.RUnlock()

	lookup := make(map[string]*PluginVersion)
	lookupURL := make(map[string]string) // name -> manifest URL
	for _, mf := range manifests {
		for _, pl := range mf.Plugins {
			if best := bestCompatibleVersion(pl.Versions, ABIVersion); best != nil {
				lookup[pl.Name] = best
				for _, src := range sources {
					if cached, ok := manifestCache[src.URL]; ok && cached == mf {
						lookupURL[pl.Name] = src.URL
						break
					}
				}
			}
		}
	}

	var results []UpdateResult
	for _, ip := range installed {
		if latest, ok := lookup[ip.Name]; ok {
			if latest.Version != ip.Version {
				results = append(results, UpdateResult{
					Name:           ip.Name,
					CurrentVersion: ip.Version,
					LatestVersion:  latest.Version,
					Changelog:      latest.Changelog,
					ManifestURL:    lookupURL[ip.Name],
				})
			}
		}
	}

	// Update last check timestamp.
	m.mu.Lock()
	m.cfg.LastCheck = time.Now().UTC().Format(time.RFC3339)
	for i, ip := range m.cfg.InstalledPlugins {
		if latest, ok := lookup[ip.Name]; ok {
			m.cfg.InstalledPlugins[i].LatestVersion = latest.Version
		}
	}
	m.mu.Unlock()

	if err := m.save(); err != nil {
		log.Printf("WARN: failed to persist update-check state: %v", err)
	}
	return results, nil
}

// UpdateResult describes an available update for an installed plugin.
type UpdateResult struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Changelog      string `json:"changelog,omitempty"`
	ManifestURL    string `json:"manifest_url,omitempty"`
}

// UpdatePlugin updates a specific plugin to its latest version.
func (m *Manager) UpdatePlugin(ctx context.Context, name string) (*InstalledPlugin, error) {
	manifests, err := m.FetchAllManifests(ctx)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	sources := make([]ManifestSource, len(m.cfg.ManifestSources))
	copy(sources, m.cfg.ManifestSources)
	manifestCache := make(map[string]*Manifest, len(m.manifests))
	for k, v := range m.manifests {
		manifestCache[k] = v
	}
	m.mu.RUnlock()

	for _, mf := range manifests {
		for _, pl := range mf.Plugins {
			if pl.Name != name {
				continue
			}
			ver := bestCompatibleVersion(pl.Versions, ABIVersion)
			if ver == nil {
				return nil, fmt.Errorf("no compatible version for %q", name)
			}

			manifestURL := ""
			for _, src := range sources {
				if cached, ok := manifestCache[src.URL]; ok && cached == mf {
					manifestURL = src.URL
					break
				}
			}

			ip, err := m.downloadAndInstall(ctx, name, ver, manifestURL)
			if err != nil {
				return nil, err
			}
			return ip, nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found in any manifest", name)
}

// UpdateAll updates all installed plugins that have auto_update enabled.
func (m *Manager) UpdateAll(ctx context.Context) ([]InstalledPlugin, error) {
	updates, err := m.CheckForUpdates(ctx)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	autoUpdate := make(map[string]bool)
	for _, ip := range m.cfg.InstalledPlugins {
		autoUpdate[ip.Name] = ip.AutoUpdate
	}
	globalAutoUpdate := m.cfg.AutoUpdate
	m.mu.RUnlock()

	var updated []InstalledPlugin
	for _, u := range updates {
		if globalAutoUpdate || autoUpdate[u.Name] {
			ip, err := m.UpdatePlugin(ctx, u.Name)
			if err != nil {
				log.Printf("WARN: failed to update plugin %q: %v", u.Name, err)
				continue
			}
			updated = append(updated, *ip)
		}
	}
	return updated, nil
}

// AddManifestSource adds a new manifest URL.
func (m *Manager) AddManifestSource(src ManifestSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.cfg.ManifestSources {
		if existing.URL == src.URL {
			return fmt.Errorf("manifest source %q already exists", src.URL)
		}
	}
	m.cfg.ManifestSources = append(m.cfg.ManifestSources, src)
	return m.saveLocked()
}

// RemoveManifestSource removes a manifest URL.
func (m *Manager) RemoveManifestSource(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var remaining []ManifestSource
	var found bool
	for _, src := range m.cfg.ManifestSources {
		if src.URL == url {
			found = true
			continue
		}
		remaining = append(remaining, src)
	}
	if !found {
		return fmt.Errorf("manifest source %q not found", url)
	}
	m.cfg.ManifestSources = remaining
	return m.saveLocked()
}

// SetAutoUpdate sets the global auto-update flag.
func (m *Manager) SetAutoUpdate(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.AutoUpdate = enabled
	return m.saveLocked()
}

// SetPluginAutoUpdate sets auto-update for a specific plugin.
func (m *Manager) SetPluginAutoUpdate(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, ip := range m.cfg.InstalledPlugins {
		if ip.Name == name {
			m.cfg.InstalledPlugins[i].AutoUpdate = enabled
			return m.saveLocked()
		}
	}
	return fmt.Errorf("plugin %q not installed", name)
}

// InstalledPlugins returns the list of installed plugins.
func (m *Manager) InstalledPlugins() []InstalledPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]InstalledPlugin, len(m.cfg.InstalledPlugins))
	copy(result, m.cfg.InstalledPlugins)
	return result
}

// CheckInterval returns the update check interval as a duration.
func (m *Manager) CheckInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg.CheckInterval == "" {
		return 6 * time.Hour
	}
	d, err := time.ParseDuration(m.cfg.CheckInterval)
	if err != nil {
		return 6 * time.Hour
	}
	return d
}

// StartAutoUpdateLoop runs a background goroutine that checks for updates periodically.
// Returns a cancel function.
func (m *Manager) StartAutoUpdateLoop(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx) // #nosec G118 -- cancel is returned to caller
	go func() {
		ticker := time.NewTicker(m.CheckInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.RLock()
				autoUpdate := m.cfg.AutoUpdate
				m.mu.RUnlock()

				if !autoUpdate {
					continue
				}

				updateCtx, updateCancel := context.WithTimeout(ctx, 5*time.Minute)
				updated, err := m.UpdateAll(updateCtx)
				updateCancel()

				if err != nil {
					log.Printf("WARN: auto-update check failed: %v", err)
					continue
				}
				if len(updated) > 0 {
					names := make([]string, len(updated))
					for i, u := range updated {
						names[i] = u.Name + "@" + u.Version
					}
					log.Printf("Auto-updated plugins: %s", strings.Join(names, ", "))
				}
			}
		}
	}()
	return cancel
}

// --- internal helpers ---

func (m *Manager) downloadAndInstall(ctx context.Context, name string, ver *PluginVersion, manifestURL string) (*InstalledPlugin, error) {
	data, err := m.downloadWasm(ctx, ver.URL)
	if err != nil {
		return nil, err
	}

	hash := sha256sum(data)
	if ver.SHA256 != "" && hash != ver.SHA256 {
		return nil, fmt.Errorf("SHA256 mismatch for %s@%s: got %s, want %s", name, ver.Version, hash, ver.SHA256)
	}

	filename := sanitizeFilename(name) + ".wasm"
	destPath := filepath.Join(m.pluginDir, filename)

	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return nil, fmt.Errorf("write plugin: %w", err)
	}

	ip := InstalledPlugin{
		Name:        name,
		Version:     ver.Version,
		ManifestURL: manifestURL,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		Path:        destPath,
		SHA256:      hash,
		AutoUpdate:  m.cfg.AutoUpdate,
	}

	m.mu.Lock()
	var updated bool
	for i, existing := range m.cfg.InstalledPlugins {
		if existing.Name == name {
			m.cfg.InstalledPlugins[i] = ip
			updated = true
			break
		}
	}
	if !updated {
		m.cfg.InstalledPlugins = append(m.cfg.InstalledPlugins, ip)
	}
	m.mu.Unlock()

	if err := m.save(); err != nil {
		return nil, err
	}
	return &ip, nil
}

func (m *Manager) downloadWasm(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Switchboard-Plugin-Manager/1.0")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download plugin: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20)) // 100MB limit
	if err != nil {
		return nil, fmt.Errorf("read plugin: %w", err)
	}
	return data, nil
}

func (m *Manager) save() error {
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()

	if m.saveFn != nil {
		return m.saveFn(cfg)
	}
	return nil
}

func (m *Manager) saveLocked() error {
	if m.saveFn != nil {
		return m.saveFn(m.cfg)
	}
	return nil
}

func sha256sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return strings.ToLower(name)
}

func nameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return "plugin"
	}
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".wasm")
	if name == "" {
		return "plugin"
	}
	return sanitizeFilename(name)
}

func bestCompatibleVersion(versions []PluginVersion, abi int) *PluginVersion {
	var best *PluginVersion
	var bestSV *semver.Version
	for i := range versions {
		v := &versions[i]
		if v.ABIMin <= abi && v.ABIMax >= abi {
			sv, err := semver.NewVersion(v.Version)
			if err != nil {
				if best == nil {
					best = v
				}
				continue
			}
			if bestSV == nil || sv.GreaterThan(bestSV) {
				best = v
				bestSV = sv
			}
		}
	}
	return best
}
