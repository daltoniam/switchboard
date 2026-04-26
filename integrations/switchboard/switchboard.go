package switchboard

import (
	"context"
	"sort"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/marketplace"
)

var (
	_ mcp.Integration                = (*switchboardInt)(nil)
	_ mcp.FieldCompactionIntegration = (*switchboardInt)(nil)
)

type switchboardInt struct {
	services    *mcp.Services
	marketplace *marketplace.Manager
}

// New creates a switchboard self-management integration.
// Call SetMarketplace after construction if marketplace is available.
func New(services *mcp.Services) mcp.Integration {
	return &switchboardInt{
		services: services,
	}
}

// SetMarketplace attaches the marketplace manager for plugin tools.
// Must be called on the concrete type returned by New.
func SetMarketplace(i mcp.Integration, mp *marketplace.Manager) {
	if s, ok := i.(*switchboardInt); ok {
		s.marketplace = mp
	}
}

func (s *switchboardInt) Name() string { return "switchboard" }

func (s *switchboardInt) Configure(_ context.Context, _ mcp.Credentials) error {
	return nil
}

func (s *switchboardInt) Tools() []mcp.ToolDefinition { return tools }

func (s *switchboardInt) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: "unknown tool: " + string(toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *switchboardInt) Healthy(_ context.Context) bool {
	return s.services != nil && s.services.Config != nil && s.services.Registry != nil
}

func (s *switchboardInt) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

type handlerFunc func(ctx context.Context, s *switchboardInt, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"switchboard_list_integrations":     listIntegrations,
	"switchboard_get_integration":       getIntegration,
	"switchboard_configure_integration": configureIntegration,
	"switchboard_check_health":          checkHealth,
	"switchboard_browse_plugins":        browsePlugins,
	"switchboard_install_plugin":        installPlugin,
	"switchboard_uninstall_plugin":      uninstallPlugin,
	"switchboard_server_info":           serverInfo,
}

// listIntegrations returns all registered integrations with their status.
func listIntegrations(_ context.Context, s *switchboardInt, args map[string]any) (*mcp.ToolResult, error) {
	enabledOnly, _ := mcp.ArgBool(args, "enabled_only")

	type integrationSummary struct {
		Name           string   `json:"name"`
		Enabled        bool     `json:"enabled"`
		Healthy        bool     `json:"healthy,omitempty"`
		ToolCount      int      `json:"tool_count"`
		CredentialKeys []string `json:"credential_keys"`
	}

	var results []integrationSummary
	for _, a := range s.services.Registry.All() {
		ic, exists := s.services.Config.GetIntegration(a.Name())
		enabled := exists && ic.Enabled

		if enabledOnly && !enabled {
			continue
		}

		credKeys := s.services.Config.DefaultCredentialKeys(a.Name())
		sort.Strings(credKeys)

		results = append(results, integrationSummary{
			Name:           a.Name(),
			Enabled:        enabled,
			ToolCount:      len(a.Tools()),
			CredentialKeys: credKeys,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return mcp.JSONResult(results)
}

// getIntegration returns detailed info about a specific integration.
func getIntegration(ctx context.Context, s *switchboardInt, args map[string]any) (*mcp.ToolResult, error) {
	name, err := mcp.ArgStr(args, "name")
	if err != nil {
		return mcp.ErrResult(err)
	}

	a, ok := s.services.Registry.Get(name)
	if !ok {
		return &mcp.ToolResult{
			Data:    "integration not found: " + name,
			IsError: true,
		}, nil
	}

	ic, exists := s.services.Config.GetIntegration(name)
	enabled := exists && ic.Enabled

	var healthy bool
	if enabled {
		healthy = a.Healthy(ctx)
	}

	type toolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	var toolList []toolInfo
	for _, t := range a.Tools() {
		toolList = append(toolList, toolInfo{
			Name:        string(t.Name),
			Description: t.Description,
		})
	}

	credKeys := s.services.Config.DefaultCredentialKeys(name)
	sort.Strings(credKeys)

	// Show which credential keys have values set (without showing the values).
	configuredKeys := []string{}
	if exists {
		for _, k := range credKeys {
			if ic.Credentials[k] != "" {
				configuredKeys = append(configuredKeys, k)
			}
		}
	}

	type integrationDetail struct {
		Name           string     `json:"name"`
		Enabled        bool       `json:"enabled"`
		Healthy        bool       `json:"healthy"`
		ToolCount      int        `json:"tool_count"`
		CredentialKeys []string   `json:"credential_keys"`
		ConfiguredKeys []string   `json:"configured_keys"`
		Tools          []toolInfo `json:"tools"`
	}

	return mcp.JSONResult(integrationDetail{
		Name:           name,
		Enabled:        enabled,
		Healthy:        healthy,
		ToolCount:      len(a.Tools()),
		CredentialKeys: credKeys,
		ConfiguredKeys: configuredKeys,
		Tools:          toolList,
	})
}

// configureIntegration sets credentials and enables/disables an integration.
func configureIntegration(ctx context.Context, s *switchboardInt, args map[string]any) (*mcp.ToolResult, error) {
	name, err := mcp.ArgStr(args, "name")
	if err != nil {
		return mcp.ErrResult(err)
	}

	a, ok := s.services.Registry.Get(name)
	if !ok {
		return &mcp.ToolResult{
			Data:    "integration not found: " + name,
			IsError: true,
		}, nil
	}

	// Get existing config or create a new one.
	ic, exists := s.services.Config.GetIntegration(name)
	if !exists || ic == nil {
		ic = &mcp.IntegrationConfig{
			Credentials: mcp.Credentials{},
		}
	}

	// Merge credentials if provided.
	if credsRaw, ok := args["credentials"]; ok {
		if credsMap, ok := credsRaw.(map[string]any); ok {
			for k, v := range credsMap {
				if vs, ok := v.(string); ok {
					ic.Credentials[k] = vs
				}
			}
		}
	}

	// Set enabled (default true).
	enabled := true
	if v, ok := args["enabled"]; ok {
		if b, ok := v.(bool); ok {
			enabled = b
		}
	}
	ic.Enabled = enabled

	// Attempt to configure the integration to validate credentials.
	if enabled {
		if err := a.Configure(ctx, ic.Credentials); err != nil {
			return &mcp.ToolResult{
				Data:    "configure failed: " + err.Error(),
				IsError: true,
			}, nil
		}
	}

	if err := s.services.Config.SetIntegration(name, ic); err != nil {
		return &mcp.ToolResult{
			Data:    "save config failed: " + err.Error(),
			IsError: true,
		}, nil
	}

	status := "enabled"
	if !enabled {
		status = "disabled"
	}

	return mcp.JSONResult(map[string]string{
		"status":      "ok",
		"integration": name,
		"state":       status,
	})
}

// checkHealth checks connectivity for one or all enabled integrations.
func checkHealth(ctx context.Context, s *switchboardInt, args map[string]any) (*mcp.ToolResult, error) {
	name, _ := mcp.ArgStr(args, "name")

	type healthResult struct {
		Name    string `json:"name"`
		Healthy bool   `json:"healthy"`
		Enabled bool   `json:"enabled"`
	}

	if name != "" {
		a, ok := s.services.Registry.Get(name)
		if !ok {
			return &mcp.ToolResult{
				Data:    "integration not found: " + name,
				IsError: true,
			}, nil
		}

		ic, exists := s.services.Config.GetIntegration(name)
		enabled := exists && ic.Enabled

		var healthy bool
		if enabled {
			healthy = a.Healthy(ctx)
		}

		return mcp.JSONResult(healthResult{
			Name:    name,
			Healthy: healthy,
			Enabled: enabled,
		})
	}

	var results []healthResult
	for _, a := range s.services.Registry.All() {
		ic, exists := s.services.Config.GetIntegration(a.Name())
		enabled := exists && ic.Enabled

		var healthy bool
		if enabled {
			healthy = a.Healthy(ctx)
		}

		results = append(results, healthResult{
			Name:    a.Name(),
			Healthy: healthy,
			Enabled: enabled,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return mcp.JSONResult(results)
}

// browsePlugins returns available plugins from configured manifest sources.
func browsePlugins(ctx context.Context, s *switchboardInt, _ map[string]any) (*mcp.ToolResult, error) {
	if s.marketplace == nil {
		return &mcp.ToolResult{
			Data:    "marketplace is not configured",
			IsError: true,
		}, nil
	}

	results, err := s.marketplace.BrowsePlugins(ctx)
	if err != nil {
		return mcp.ErrResult(err)
	}

	if len(results) == 0 {
		return mcp.JSONResult(map[string]string{
			"message": "No plugins available. Add manifest sources via the web UI or config file.",
		})
	}

	return mcp.JSONResult(results)
}

// installPlugin installs a plugin from the marketplace by name or URL.
func installPlugin(ctx context.Context, s *switchboardInt, args map[string]any) (*mcp.ToolResult, error) {
	if s.marketplace == nil {
		return &mcp.ToolResult{
			Data:    "marketplace is not configured",
			IsError: true,
		}, nil
	}

	name, _ := mcp.ArgStr(args, "name")
	url, _ := mcp.ArgStr(args, "url")

	if name == "" && url == "" {
		return &mcp.ToolResult{
			Data:    "either name or url is required",
			IsError: true,
		}, nil
	}

	var ip *marketplace.InstalledPlugin
	var err error

	if url != "" {
		ip, err = s.marketplace.InstallFromURL(ctx, url)
	} else {
		ip, err = s.marketplace.InstallPlugin(ctx, name)
	}

	if err != nil {
		return mcp.ErrResult(err)
	}

	return mcp.JSONResult(map[string]any{
		"status":  "installed",
		"name":    ip.Name,
		"version": ip.Version,
		"path":    ip.Path,
		"note":    "Restart Switchboard to load the plugin.",
	})
}

// uninstallPlugin removes an installed marketplace plugin.
func uninstallPlugin(_ context.Context, s *switchboardInt, args map[string]any) (*mcp.ToolResult, error) {
	if s.marketplace == nil {
		return &mcp.ToolResult{
			Data:    "marketplace is not configured",
			IsError: true,
		}, nil
	}

	name, err := mcp.ArgStr(args, "name")
	if err != nil {
		return mcp.ErrResult(err)
	}

	if err := s.marketplace.UninstallPlugin(name); err != nil {
		return mcp.ErrResult(err)
	}

	return mcp.JSONResult(map[string]string{
		"status": "uninstalled",
		"name":   name,
		"note":   "Restart Switchboard to fully remove the plugin.",
	})
}

// serverInfo returns server version, metrics, and configuration summary.
func serverInfo(_ context.Context, s *switchboardInt, _ map[string]any) (*mcp.ToolResult, error) {
	allIntegrations := s.services.Registry.All()
	enabledCount := 0
	totalTools := 0
	for _, a := range allIntegrations {
		totalTools += len(a.Tools())
		ic, exists := s.services.Config.GetIntegration(a.Name())
		if exists && ic.Enabled {
			enabledCount++
		}
	}

	info := map[string]any{
		"total_integrations":   len(allIntegrations),
		"enabled_integrations": enabledCount,
		"total_tools":          totalTools,
	}

	if s.services.Metrics != nil {
		snap := s.services.Metrics.Snapshot()
		info["metrics"] = snap
	}

	if s.marketplace != nil {
		installed := s.marketplace.InstalledPlugins()
		cfg := s.marketplace.Config()
		info["marketplace"] = map[string]any{
			"installed_plugins": len(installed),
			"manifest_sources":  len(cfg.ManifestSources),
			"auto_update":       cfg.AutoUpdate,
		}
	}

	return mcp.JSONResult(info)
}
