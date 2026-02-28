package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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

// --- search pagination tests ---

// searchRequest is a helper to build a CallToolRequest for handleSearch.
func searchRequest(args map[string]any) *mcpsdk.CallToolRequest {
	data, _ := json.Marshal(args)
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "search",
			Arguments: json.RawMessage(data),
		},
	}
}

// searchResponse is the paginated envelope returned by handleSearch.
type searchResponse struct {
	Summary string `json:"summary"`
	Total   int    `json:"total"`
	Offset  int    `json:"offset"`
	Limit   int    `json:"limit"`
	Tools   []struct {
		Integration string `json:"integration"`
		Name        string `json:"name"`
	} `json:"tools"`
}

func parseSearchResponse(t *testing.T, result *mcpsdk.CallToolResult) searchResponse {
	t.Helper()
	require.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	var resp searchResponse
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	return resp
}

// makeManyTools generates n tool definitions for testing pagination.
func makeManyTools(prefix string, n int) []mcp.ToolDefinition {
	tools := make([]mcp.ToolDefinition, n)
	for i := range n {
		tools[i] = mcp.ToolDefinition{
			Name:        fmt.Sprintf("%s_tool_%d", prefix, i),
			Description: fmt.Sprintf("Tool %d for %s", i, prefix),
			Parameters:  map[string]string{"id": "the id"},
		}
	}
	return tools
}

func TestHandleSearch_Pagination(t *testing.T) {
	tests := []struct {
		name      string
		toolCount int
		args      map[string]any
		wantTotal int
		wantOffset int
		wantLimit int
		wantTools int
	}{
		{
			name:       "default limit caps results",
			toolCount:  50,
			args:       map[string]any{},
			wantTotal:  50, wantOffset: 0, wantLimit: 20, wantTools: 20,
		},
		{
			name:       "offset slides window",
			toolCount:  50,
			args:       map[string]any{"offset": 10, "limit": 5},
			wantTotal:  50, wantOffset: 10, wantLimit: 5, wantTools: 5,
		},
		{
			name:       "offset beyond total returns empty",
			toolCount:  5,
			args:       map[string]any{"offset": 100},
			wantTotal:  5, wantOffset: 5, wantLimit: 20, wantTools: 0,
		},
		{
			name:       "limit zero returns metadata only",
			toolCount:  30,
			args:       map[string]any{"limit": 0},
			wantTotal:  30, wantOffset: 0, wantLimit: 0, wantTools: 0,
		},
		{
			name:       "negative limit clamped to zero",
			toolCount:  10,
			args:       map[string]any{"limit": -5},
			wantTotal:  10, wantOffset: 0, wantLimit: 0, wantTools: 0,
		},
		{
			name:       "negative offset clamped to zero",
			toolCount:  10,
			args:       map[string]any{"offset": -3, "limit": 5},
			wantTotal:  10, wantOffset: 0, wantLimit: 5, wantTools: 5,
		},
		{
			name:       "limit larger than total returns all",
			toolCount:  5,
			args:       map[string]any{"limit": 1000},
			wantTotal:  5, wantOffset: 0, wantLimit: 1000, wantTools: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mockIntegration{
				name:    "testint",
				healthy: true,
				tools:   makeManyTools("testint", tt.toolCount),
			}
			s := setupTestServer(mi)

			result, err := s.handleSearch(context.Background(), searchRequest(tt.args))
			require.NoError(t, err)

			resp := parseSearchResponse(t, result)
			assert.Equal(t, tt.wantTotal, resp.Total)
			assert.Equal(t, tt.wantOffset, resp.Offset)
			assert.Equal(t, tt.wantLimit, resp.Limit)
			assert.Len(t, resp.Tools, tt.wantTools)
		})
	}
}

func TestHandleSearch_QueryFiltersCombinedWithPagination(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_list_a", Description: "List A items"},
			{Name: "testint_list_b", Description: "List B items"},
			{Name: "testint_list_c", Description: "List C items"},
			{Name: "testint_get_x", Description: "Get X"},
		},
	}
	s := setupTestServer(mi)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"query":  "list",
		"limit":  2,
		"offset": 1,
	}))
	require.NoError(t, err)

	resp := parseSearchResponse(t, result)
	assert.Equal(t, 3, resp.Total)
	assert.Len(t, resp.Tools, 2)
}

func TestHandleSearch_MalformedArgsReturnsError(t *testing.T) {
	s := setupTestServer(&mockIntegration{
		name:    "testint",
		healthy: true,
		tools:   makeManyTools("testint", 5),
	})

	req := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "search",
			Arguments: json.RawMessage(`{"limit": "not a number"}`),
		},
	}

	result, err := s.handleSearch(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestHandleSearch_MultiIntegrationSortedDeterministically(t *testing.T) {
	alpha := &mockIntegration{
		name:    "alpha",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "alpha_b", Description: "B"},
			{Name: "alpha_a", Description: "A"},
		},
	}
	beta := &mockIntegration{
		name:    "beta",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "beta_a", Description: "A"},
		},
	}
	s := setupTestServer(alpha, beta)

	// Call twice — results must be in the same order.
	result1, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"limit": 10}))
	require.NoError(t, err)
	resp1 := parseSearchResponse(t, result1)

	result2, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"limit": 10}))
	require.NoError(t, err)
	resp2 := parseSearchResponse(t, result2)

	require.Len(t, resp1.Tools, 3)
	require.Len(t, resp2.Tools, 3)

	// Must be sorted by integration name, then tool name.
	assert.Equal(t, "alpha_a", resp1.Tools[0].Name)
	assert.Equal(t, "alpha_b", resp1.Tools[1].Name)
	assert.Equal(t, "beta_a", resp1.Tools[2].Name)

	// Same order on second call.
	for i := range resp1.Tools {
		assert.Equal(t, resp1.Tools[i].Name, resp2.Tools[i].Name)
	}
}

func TestHandleSearch_ResponseIncludesSummary(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools:   makeManyTools("testint", 5),
	}
	s := setupTestServer(mi)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
	require.NoError(t, err)

	resp := parseSearchResponse(t, result)
	assert.Contains(t, resp.Summary, "5")
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
