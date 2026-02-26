package mcp

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned when an integration is used before being configured.
var ErrNotConfigured = errors.New("integration not configured")

// ErrUnhealthy is returned when an integration cannot reach its upstream API.
var ErrUnhealthy = errors.New("integration unhealthy")

// Credentials holds key-value credential pairs for an integration.
type Credentials map[string]string

// IntegrationConfig stores the enabled state and credentials for a single integration.
type IntegrationConfig struct {
	Enabled     bool        `json:"enabled"`
	Credentials Credentials `json:"credentials"`
}

// Config is the top-level configuration containing all integrations.
type Config struct {
	Integrations map[string]*IntegrationConfig `json:"integrations"`
}

// ToolDefinition describes an API operation an integration exposes.
// These are used by the search tool to let the AI discover available operations.
type ToolDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`           // param name -> description
	Required    []string          `json:"required,omitempty"`
}

// ToolResult is the output of executing a tool.
type ToolResult struct {
	Data    string `json:"data,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// HealthStatus represents the health of an integration.
type HealthStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

// --- Port Interfaces (the hexagonal boundaries) ---

// Integration is the primary port that all integration adapters implement.
// The domain defines what it needs; adapter packages satisfy it.
type Integration interface {
	// Name returns the lowercase integration identifier (e.g., "github", "datadog").
	Name() string

	// Configure initializes the integration with credentials.
	Configure(creds Credentials) error

	// Tools returns the tool definitions this integration provides.
	// Used by the search tool for progressive discovery.
	Tools() []ToolDefinition

	// Execute runs a named tool with the given arguments and returns the result.
	Execute(ctx context.Context, toolName string, args map[string]any) (*ToolResult, error)

	// Healthy returns true if the integration can reach its upstream API.
	Healthy(ctx context.Context) bool
}

// ConfigService manages loading and saving configuration.
type ConfigService interface {
	Load() error
	Save() error
	Get() *Config
	Update(cfg *Config) error
	GetIntegration(name string) (*IntegrationConfig, bool)
	SetIntegration(name string, ic *IntegrationConfig) error
	EnabledIntegrations() []string
}

// Registry holds all registered integrations and provides lookup.
type Registry interface {
	Register(i Integration) error
	Get(name string) (Integration, bool)
	All() []Integration
	Names() []string
}

// Services aggregates all port interfaces — the dependency injection container.
type Services struct {
	Config   ConfigService
	Registry Registry
}
