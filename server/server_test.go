package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test helpers ---

type mockConfigService struct {
	cfg *mcp.Config
}

func newMockConfigService(integrations map[string]*mcp.IntegrationConfig) *mockConfigService {
	return &mockConfigService{cfg: &mcp.Config{Integrations: integrations}}
}

func (m *mockConfigService) Load() error                                          { return nil }
func (m *mockConfigService) Save() error                                          { return nil }
func (m *mockConfigService) Get() *mcp.Config                                     { return m.cfg }
func (m *mockConfigService) Update(cfg *mcp.Config) error                         { m.cfg = cfg; return nil }
func (m *mockConfigService) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	ic, ok := m.cfg.Integrations[name]
	return ic, ok
}
func (m *mockConfigService) SetIntegration(name string, ic *mcp.IntegrationConfig) error {
	m.cfg.Integrations[name] = ic
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

type mockIntegration struct {
	name      string
	tools     []mcp.ToolDefinition
	execFn    func(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error)
	healthy   bool
	configErr error
}

func (m *mockIntegration) Name() string { return m.name }
func (m *mockIntegration) Configure(_ mcp.Credentials) error {
	return m.configErr
}
func (m *mockIntegration) Tools() []mcp.ToolDefinition { return m.tools }
func (m *mockIntegration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	if m.execFn != nil {
		return m.execFn(ctx, toolName, args)
	}
	return &mcp.ToolResult{Data: fmt.Sprintf("executed %s", toolName)}, nil
}
func (m *mockIntegration) Healthy(_ context.Context) bool { return m.healthy }

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

func setupTestServer(integrations ...*mockIntegration) *Server {
	reg := newMockRegistry()
	cfgIntegrations := make(map[string]*mcp.IntegrationConfig)

	for _, i := range integrations {
		reg.Register(i)
		cfgIntegrations[i.name] = &mcp.IntegrationConfig{
			Enabled:     true,
			Credentials: mcp.Credentials{"token": "test"},
		}
	}

	services := &mcp.Services{
		Config:   newMockConfigService(cfgIntegrations),
		Registry: reg,
	}
	return New(services)
}

// --- tests ---

func TestNew(t *testing.T) {
	s := setupTestServer()
	require.NotNil(t, s)
	assert.NotNil(t, s.mcpServer)
	assert.NotNil(t, s.services)
}

func TestMatches(t *testing.T) {
	tool := mcp.ToolDefinition{
		Name:        "github_list_issues",
		Description: "List issues in a repository",
	}

	tests := []struct {
		query string
		match bool
	}{
		{"github", true},
		{"list issues", true},
		{"github_list_issues", true},
		{"issues", true},
		{"repository", true},
		{"datadog", false},
		{"metrics", false},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			assert.Equal(t, tt.match, matches(tool, "github", tt.query))
		})
	}
}

func TestMatches_MultiWord(t *testing.T) {
	tool := mcp.ToolDefinition{
		Name:        "datadog_search_logs",
		Description: "Search Datadog logs",
	}

	assert.True(t, matches(tool, "datadog", "search logs"))
	assert.True(t, matches(tool, "datadog", "datadog logs"))
	assert.False(t, matches(tool, "datadog", "github logs"))
}

func TestObjectSchema(t *testing.T) {
	props := map[string]any{
		"query": map[string]any{"type": "string"},
	}

	t.Run("without required", func(t *testing.T) {
		schema := objectSchema(props, nil)
		assert.Equal(t, "object", schema["type"])
		assert.Equal(t, props, schema["properties"])
		_, hasRequired := schema["required"]
		assert.False(t, hasRequired)
	})

	t.Run("with required", func(t *testing.T) {
		schema := objectSchema(props, []string{"query"})
		assert.Equal(t, []string{"query"}, schema["required"])
	})
}

func TestErrorResult(t *testing.T) {
	r := errorResult("something went wrong")
	require.NotNil(t, r)
	assert.True(t, r.IsError)
	require.Len(t, r.Content, 1)
}

func TestHandler(t *testing.T) {
	s := setupTestServer()
	handler := s.Handler()
	assert.NotNil(t, handler)
}

func TestServerWithIntegration(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        "testint_list_items",
				Description: "List test items",
				Parameters:  map[string]string{"query": "Search query"},
			},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"items":["a","b"]}`}, nil
		},
	}

	s := setupTestServer(mi)
	require.NotNil(t, s)
}

func TestConfigureIntegrations_SkipsMissingAdapter(t *testing.T) {
	cfgService := newMockConfigService(map[string]*mcp.IntegrationConfig{
		"nonexistent": {Enabled: true, Credentials: mcp.Credentials{"key": "val"}},
	})
	reg := newMockRegistry()

	services := &mcp.Services{Config: cfgService, Registry: reg}
	s := New(services)
	require.NotNil(t, s)
}

func TestConfigureIntegrations_SkipsFailedConfigure(t *testing.T) {
	mi := &mockIntegration{
		name:      "badint",
		configErr: fmt.Errorf("bad credentials"),
	}

	reg := newMockRegistry()
	reg.Register(mi)

	cfgService := newMockConfigService(map[string]*mcp.IntegrationConfig{
		"badint": {Enabled: true, Credentials: mcp.Credentials{}},
	})

	services := &mcp.Services{Config: cfgService, Registry: reg}
	s := New(services)
	require.NotNil(t, s)
}

func TestHandleSearch_Integration(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_alpha", Description: "Alpha tool"},
			{Name: "testint_beta", Description: "Beta tool for searching"},
		},
	}

	s := setupTestServer(mi)

	// Simulate calling handleSearch by verifying matches.
	var matchedTools []string
	for _, tool := range mi.Tools() {
		if matches(tool, "testint", "alpha") {
			matchedTools = append(matchedTools, tool.Name)
		}
	}
	assert.Equal(t, []string{"testint_alpha"}, matchedTools)

	// Empty query matches all.
	var allTools []string
	for _, tool := range mi.Tools() {
		if matches(tool, "testint", "") {
			allTools = append(allTools, tool.Name)
		}
	}
	assert.Len(t, allTools, 2)

	_ = s // ensure server was created successfully
}

func TestToolResultJSON(t *testing.T) {
	result := &mcp.ToolResult{Data: `{"count":5}`, IsError: false}
	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded mcp.ToolResult
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, `{"count":5}`, decoded.Data)
	assert.False(t, decoded.IsError)
}
