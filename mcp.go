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
	Instructions *InstructionsConfig           `json:"instructions,omitempty"`
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

// FieldCompactionIntegration is an optional interface that integrations can implement
// to declare field compaction specs for tool responses. The server applies
// field compaction automatically after Execute, reducing context usage for LLM consumers.
type FieldCompactionIntegration interface {
	// CompactSpec returns pre-parsed field compaction specs for a tool.
	// Returns false if the tool has no specs (skip compaction).
	// Adapters should parse specs once at init time via ParseCompactSpecs.
	CompactSpec(toolName string) ([]CompactField, bool)
}

// PlainTextCredentials is an optional interface that integrations can implement
// to declare which credential keys should be rendered as plain text inputs
// instead of password fields in the web UI.
type PlainTextCredentials interface {
	PlainTextKeys() []string
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
	DefaultCredentialKeys(name string) []string
	// Instructions management
	GetInstructions() *InstructionsConfig
	SetInstructions(ic *InstructionsConfig) error
	GetInstruction(id string) (*Instruction, bool)
	SetInstruction(inst *Instruction) error
	DeleteInstruction(id string) error
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

// Instruction represents a system instruction/prompt template that can be
// dynamically rendered with context (model info, environment, custom variables).
// Inspired by the Helmsman Ruby gem's approach to managing AI assistant instructions.
type Instruction struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Template    string            `json:"template"`
	Enabled     bool              `json:"enabled"`
	Variables   map[string]string `json:"variables,omitempty"`
}

// InstructionsConfig holds all instruction templates.
type InstructionsConfig struct {
	DefaultTier  string            `json:"default_tier,omitempty"`
	ModelTiers   map[string]string `json:"model_tiers,omitempty"`
	Instructions []*Instruction    `json:"instructions,omitempty"`
}

// ModelContext provides information about the AI model requesting instructions.
type ModelContext struct {
	ID   string // Model identifier (e.g., "claude-opus-4-5-20251101")
	Tier string // Model tier: "large" or "small"
}

// EnvContext provides information about the execution environment.
type EnvContext struct {
	OS       string // Operating system: "macos", "linux", "windows"
	Shell    string // Shell: "zsh", "bash", "fish", "sh", "powershell"
	InDocker bool   // Running inside Docker container
	InSSH    bool   // Running in SSH session
	HasMise  bool   // mise (formerly rtx) available
	HasBrew  bool   // Homebrew available
	HasApt   bool   // apt available
	HasGh    bool   // GitHub CLI available
	HasGit   bool   // git available
}

// InstructionRenderContext is the data passed to instruction templates.
type InstructionRenderContext struct {
	Model ModelContext
	Env   EnvContext
	Vars  map[string]string // Custom variables from instruction config
}
