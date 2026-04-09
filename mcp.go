package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"time"
)

// ErrNotConfigured is returned when an integration is used before being configured.
var ErrNotConfigured = errors.New("integration not configured")

// ErrUnhealthy is returned when an integration cannot reach its upstream API.
var ErrUnhealthy = errors.New("integration unhealthy")

// RetryableError signals that an operation failed with a transient error (5xx, 429)
// and should be retried. The server layer retries these automatically with backoff.
// Adapters return this from their HTTP helpers; non-retryable errors (4xx) use plain errors.
type RetryableError struct {
	StatusCode int
	Err        error
	RetryAfter time.Duration // server-suggested wait; 0 means use default backoff
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable (%d): %s", e.StatusCode, e.Err)
}

func (e *RetryableError) Unwrap() error { return e.Err }

// IsRetryable reports whether err (or any error in its chain) is a RetryableError.
func IsRetryable(err error) bool {
	var re *RetryableError
	return errors.As(err, &re)
}

const maxRetryAfter = 60 * time.Second

// ParseRetryAfter parses a Retry-After header value (integer seconds) into a Duration.
// Returns 0 for empty, non-numeric, or non-positive values. Caps at 60s.
// Does not handle HTTP-date format (RFC 7231 §7.1.3) — all known upstream APIs use
// integer seconds.
func ParseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	secs, err := strconv.Atoi(header)
	if err != nil || secs <= 0 {
		return 0
	}
	d := time.Duration(secs) * time.Second
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}

// Credentials holds key-value credential pairs for an integration.
type Credentials map[string]string

// IntegrationConfig stores the enabled state and credentials for a single integration.
type IntegrationConfig struct {
	Enabled     bool        `json:"enabled"`
	Credentials Credentials `json:"credentials"`
	ToolGlobs   []string    `json:"tool_globs,omitempty"`
}

// ToolAllowed reports whether toolName is permitted by the integration's tool glob
// restrictions. An empty ToolGlobs slice means all tools are permitted.
// Multiple globs are OR'd: the tool is allowed if any glob matches.
// Invalid patterns are skipped (use ValidateToolGlobs to catch them at config time).
func (ic *IntegrationConfig) ToolAllowed(toolName string) bool {
	if len(ic.ToolGlobs) == 0 {
		return true
	}
	for _, pattern := range ic.ToolGlobs {
		matched, err := path.Match(pattern, toolName)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// ValidateToolGlobs checks that all tool glob patterns are syntactically valid.
// Returns an error naming the first invalid pattern.
func ValidateToolGlobs(globs []string) error {
	for _, pattern := range globs {
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("invalid tool glob pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// WasmModuleConfig describes a WASM module to load as an integration.
type WasmModuleConfig struct {
	Path        string      `json:"path"`
	Credentials Credentials `json:"credentials,omitempty"`
}

// Config is the top-level configuration containing all integrations.
type Config struct {
	Integrations map[string]*IntegrationConfig `json:"integrations"`
	WasmModules  []WasmModuleConfig            `json:"wasm_modules,omitempty"`
}

// ToolDefinition describes an API operation an integration exposes.
// These are used by the search tool to let the AI discover available operations.
type ToolDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"` // param name -> description
	Required    []string          `json:"required,omitempty"`
}

// ToolResult is the output of executing a tool.
type ToolResult struct {
	Data    string `json:"data,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// JSONResult marshals v to JSON and returns it as a ToolResult.
func JSONResult(v any) (*ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return &ToolResult{Data: err.Error(), IsError: true}, nil
	}
	return &ToolResult{Data: string(data)}, nil
}

// RawResult wraps already-serialized JSON bytes as a ToolResult.
// Passing nil is equivalent to passing an empty slice — returns an empty, non-error result.
func RawResult(data []byte) (*ToolResult, error) {
	return &ToolResult{Data: string(data)}, nil
}

// ErrResult converts an error to a ToolResult.
// Retryable errors are propagated as Go errors for the server retry loop.
// Non-retryable errors become ToolResult with IsError=true.
func ErrResult(err error) (*ToolResult, error) {
	if err == nil {
		return nil, nil
	}
	if IsRetryable(err) {
		return nil, err
	}
	return &ToolResult{Data: err.Error(), IsError: true}, nil
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
	Configure(ctx context.Context, creds Credentials) error

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

// PlaceholderHints is an optional interface that integrations can implement
// to provide custom placeholder text for credential input fields in the web UI.
type PlaceholderHints interface {
	Placeholders() map[string]string
}

// OptionalCredentials is an optional interface that integrations can implement
// to declare which credential keys are not required, so the web UI can label them.
type OptionalCredentials interface {
	OptionalKeys() []string
}

// ConfigService manages loading and saving configuration.
type ConfigService interface {
	Load() error
	Save() error
	Get() *Config
	Update(cfg *Config) error
	GetIntegration(name string) (*IntegrationConfig, bool)
	SetIntegration(name string, ic *IntegrationConfig) error
	SetWasmModules(modules []WasmModuleConfig) error
	EnabledIntegrations() []string
	DefaultCredentialKeys(name string) []string
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
	Browser  BrowserService // nil if playwright driver is not installed
	Metrics  *Metrics       // nil until initialized; callers must nil-check
}

// BrowserService manages browser lifecycle for web automation.
// Pass via integration constructors — never exposed as MCP tools.
type BrowserService interface {
	NewSession(ctx context.Context) (BrowserSession, error)
	Close() error
}

// BrowserCookie represents a browser cookie for injection into a BrowserSession.
type BrowserCookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Secure   bool
	HTTPOnly bool
	Expires  *time.Time
}

// BrowserSession is an isolated browser context (own cookies, local storage).
// One session per integration; pages within the same session share auth state.
// AddCookies should be called before navigating any pages — cookies injected
// after the first navigation may not apply to already-loaded page contexts.
type BrowserSession interface {
	AddCookies(ctx context.Context, cookies []BrowserCookie) error
	NewPage(ctx context.Context) (BrowserPage, error)
	Close() error
}

// BrowserPage is a single browser tab.
// Note: context.Context parameters are accepted for API consistency and future-proofing,
// but the underlying playwright-go driver does not support context cancellation.
// Long-running calls (Navigate, WaitForSelector) will not be interrupted by ctx.Done().
type BrowserPage interface {
	Navigate(ctx context.Context, url string) error
	Fill(ctx context.Context, selector, value string) error
	Click(ctx context.Context, selector string) error
	SelectOption(ctx context.Context, selector, value string) error
	InnerText(ctx context.Context, selector string) (string, error)
	InnerHTML(ctx context.Context, selector string) (string, error)
	Content(ctx context.Context) (string, error)
	WaitForSelector(ctx context.Context, selector string) error
	Screenshot(ctx context.Context) ([]byte, error)
	Evaluate(ctx context.Context, expression string, args ...any) (any, error)
	Close() error
}
