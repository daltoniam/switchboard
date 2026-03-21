package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/audit"
	"github.com/daltoniam/switchboard/integrationcache"
	"github.com/daltoniam/switchboard/tenant"
)

// IntegrationCache is an optional field on Server used in hosted mode.
// When nil, the server uses the local-mode singleton path.
var _ = (*integrationcache.Cache)(nil)

// resolveIntegration returns a configured integration instance for the tenant.
// In hosted mode (tenant context present + cache set), it checks the cache,
// loads from the tenant's config on miss, and configures a new instance.
// In local mode, it delegates to the singleton registry.
func (s *Server) resolveIntegration(ctx context.Context, integrationName string) (mcp.Integration, error) {
	ti, ok := tenant.FromContext(ctx)
	if !ok || s.cache == nil {
		integration, found := s.services.Registry.Get(integrationName)
		if !found {
			return nil, fmt.Errorf("integration %q not found", integrationName)
		}
		return integration, nil
	}

	ic, err := s.services.Config.GetTenantIntegration(ctx, ti.TenantID, integrationName)
	if err != nil {
		return nil, fmt.Errorf("load config for %q: %w", integrationName, err)
	}
	if !ic.Enabled {
		return nil, fmt.Errorf("integration %q not enabled for tenant", integrationName)
	}

	credHash := integrationcache.HashCreds(ic.Credentials)
	if instance, cached := s.cache.Get(ti.TenantID, integrationName, credHash); cached {
		return instance, nil
	}

	instance, err := s.services.Registry.NewInstance(integrationName)
	if err != nil {
		return nil, fmt.Errorf("create instance of %q: %w", integrationName, err)
	}
	if err := instance.Configure(ctx, ic.Credentials); err != nil {
		return nil, fmt.Errorf("configure %q: %w", integrationName, err)
	}

	s.cache.Put(ti.TenantID, integrationName, instance, credHash)
	return instance, nil
}

// tenantEnabledIntegrations returns the list of enabled integration names,
// tenant-scoped in hosted mode or from the local config.
func (s *Server) tenantEnabledIntegrations(ctx context.Context) ([]string, error) {
	ti, ok := tenant.FromContext(ctx)
	if !ok {
		return s.services.Config.EnabledIntegrations(), nil
	}
	return s.services.Config.TenantEnabledIntegrations(ctx, ti.TenantID)
}

// tenantIntegrationConfig returns the integration config for a specific integration,
// tenant-scoped in hosted mode or from the local config.
func (s *Server) tenantIntegrationConfig(ctx context.Context, name string) (*mcp.IntegrationConfig, error) {
	ti, ok := tenant.FromContext(ctx)
	if !ok {
		ic, found := s.services.Config.GetIntegration(name)
		if !found {
			return nil, nil //nolint:nilnil
		}
		return ic, nil
	}
	return s.services.Config.GetTenantIntegration(ctx, ti.TenantID, name)
}

// tenantGetIntegrationInstance returns the integration instance for a tool name.
// In hosted mode, resolves via the cache. In local mode, uses the singleton.
func (s *Server) tenantGetIntegrationInstance(ctx context.Context, integrationName string) (mcp.Integration, bool) {
	ti, ok := tenant.FromContext(ctx)
	if !ok || s.cache == nil {
		return s.services.Registry.Get(integrationName)
	}

	instance, err := s.resolveIntegration(ctx, integrationName)
	if err != nil {
		slog.Warn("tenant integration resolution failed",
			"tenant", ti.TenantID,
			"integration", integrationName,
			"err", err,
		)
		return nil, false
	}
	return instance, true
}

// auditLogExecute records a tool execution event if an audit logger is available.
func (s *Server) auditLogExecute(ctx context.Context, toolName, integrationName string, args map[string]any, isError bool) {
	if s.services.Audit == nil {
		return
	}
	ti, ok := tenant.FromContext(ctx)
	if !ok {
		return
	}
	s.services.Audit.Log(ctx, audit.Entry{
		TenantID:    ti.TenantID,
		UserID:      ti.UserID,
		ToolName:    toolName,
		Integration: integrationName,
		ArgsHash:    audit.HashArgs(args),
		IsError:     isError,
		Timestamp:   time.Now(),
	})
}

// extractIntegrationName extracts the integration name from a tool name.
// Tool names follow the pattern "integration_action" (e.g., "github_list_issues").
func extractIntegrationName(toolName string) string {
	parts := strings.SplitN(toolName, "_", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
