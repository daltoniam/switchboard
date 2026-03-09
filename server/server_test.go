package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

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

func (m *mockConfigService) Load() error                  { return nil }
func (m *mockConfigService) Save() error                  { return nil }
func (m *mockConfigService) Get() *mcp.Config             { return m.cfg }
func (m *mockConfigService) Update(cfg *mcp.Config) error { m.cfg = cfg; return nil }
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
func (m *mockConfigService) DefaultCredentialKeys(_ string) []string { return nil }

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
	Summary      string   `json:"summary"`
	ScriptHint   string   `json:"script_hint"`
	Total        int      `json:"total"`
	Offset       int      `json:"offset"`
	Limit        int      `json:"limit"`
	HasMore      bool     `json:"has_more"`
	Integrations []string `json:"integrations"`
	Tools        []struct {
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
		name        string
		toolCount   int
		args        map[string]any
		wantTotal   int
		wantOffset  int
		wantLimit   int
		wantTools   int
		wantHasMore bool
	}{
		{
			name:      "default limit caps results",
			toolCount: 50,
			args:      map[string]any{},
			wantTotal: 50, wantOffset: 0, wantLimit: 20, wantTools: 20, wantHasMore: true,
		},
		{
			name:      "offset slides window",
			toolCount: 50,
			args:      map[string]any{"offset": 10, "limit": 5},
			wantTotal: 50, wantOffset: 10, wantLimit: 5, wantTools: 5, wantHasMore: true,
		},
		{
			name:      "offset beyond total returns empty",
			toolCount: 5,
			args:      map[string]any{"offset": 100},
			wantTotal: 5, wantOffset: 5, wantLimit: 20, wantTools: 0, wantHasMore: false,
		},
		{
			name:      "limit zero returns metadata only",
			toolCount: 30,
			args:      map[string]any{"limit": 0},
			wantTotal: 30, wantOffset: 0, wantLimit: 0, wantTools: 0, wantHasMore: false,
		},
		{
			name:      "negative limit clamped to zero",
			toolCount: 10,
			args:      map[string]any{"limit": -5},
			wantTotal: 10, wantOffset: 0, wantLimit: 0, wantTools: 0, wantHasMore: false,
		},
		{
			name:      "negative offset clamped to zero",
			toolCount: 10,
			args:      map[string]any{"offset": -3, "limit": 5},
			wantTotal: 10, wantOffset: 0, wantLimit: 5, wantTools: 5, wantHasMore: true,
		},
		{
			name:      "limit larger than total returns all",
			toolCount: 5,
			args:      map[string]any{"limit": 1000},
			wantTotal: 5, wantOffset: 0, wantLimit: 1000, wantTools: 5, wantHasMore: false,
		},
		{
			name:      "last page has_more is false",
			toolCount: 10,
			args:      map[string]any{"offset": 5, "limit": 5},
			wantTotal: 10, wantOffset: 5, wantLimit: 5, wantTools: 5, wantHasMore: false,
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
			assert.Equal(t, tt.wantHasMore, resp.HasMore, "has_more")
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

func TestHandleSearch_ResponseIncludesIntegrations(t *testing.T) {
	alpha := &mockIntegration{
		name:    "alpha",
		healthy: true,
		tools:   makeManyTools("alpha", 3),
	}
	beta := &mockIntegration{
		name:    "beta",
		healthy: true,
		tools:   makeManyTools("beta", 2),
	}
	s := setupTestServer(alpha, beta)

	t.Run("lists all enabled integrations", func(t *testing.T) {
		result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"limit": 10}))
		require.NoError(t, err)

		resp := parseSearchResponse(t, result)
		assert.ElementsMatch(t, []string{"alpha", "beta"}, resp.Integrations)
	})

	t.Run("present even when query filters out all tools", func(t *testing.T) {
		result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
			"query": "nonexistent_tool_xyz",
		}))
		require.NoError(t, err)

		resp := parseSearchResponse(t, result)
		assert.ElementsMatch(t, []string{"alpha", "beta"}, resp.Integrations)
		assert.Empty(t, resp.Tools)
	})
}

// --- smoke test ---

// TestSmoke_SearchResponseShape verifies the full search response contract
// that LLM consumers depend on. If this test breaks, consumers will too.
func TestSmoke_SearchResponseShape(t *testing.T) {
	alpha := &mockIntegration{
		name:    "alpha",
		healthy: true,
		tools:   makeManyTools("alpha", 10),
	}
	beta := &mockIntegration{
		name:    "beta",
		healthy: true,
		tools:   makeManyTools("beta", 5),
	}
	s := setupTestServer(alpha, beta)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"limit":  3,
		"offset": 0,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Parse raw JSON to verify every expected key exists.
	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &raw))

	expectedKeys := []string{"summary", "script_hint", "total", "offset", "limit", "has_more", "integrations", "tools"}
	for _, key := range expectedKeys {
		assert.Contains(t, raw, key, "response missing key %q", key)
	}

	// Parse typed response and verify field values.
	resp := parseSearchResponse(t, result)
	assert.Equal(t, 15, resp.Total)
	assert.Equal(t, 0, resp.Offset)
	assert.Equal(t, 3, resp.Limit)
	assert.True(t, resp.HasMore)
	assert.ElementsMatch(t, []string{"alpha", "beta"}, resp.Integrations)
	assert.Len(t, resp.Tools, 3)
	assert.Contains(t, resp.Summary, "15")
	assert.Contains(t, resp.ScriptHint, "script")
}

// --- compaction integration mock ---

type mockFieldCompactionIntegration struct {
	mockIntegration
	specs map[string][]mcp.CompactField
}

func (m *mockFieldCompactionIntegration) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := m.specs[toolName]
	return fields, ok
}

// executeRequest builds a CallToolRequest for handleExecute.
func executeRequest(toolName string, args map[string]any) *mcpsdk.CallToolRequest {
	data, _ := json.Marshal(map[string]any{
		"tool_name": toolName,
		"arguments": args,
	})
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "execute",
			Arguments: json.RawMessage(data),
		},
	}
}

func mustParseCompactSpecs(t *testing.T, specs []string) []mcp.CompactField {
	t.Helper()
	fields, err := mcp.ParseCompactSpecs(specs)
	require.NoError(t, err)
	return fields
}

func TestHandleExecute_CompactionApplied(t *testing.T) {
	mi := &mockFieldCompactionIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: "testint_list_items", Description: "List items"},
			},
			execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `[{"id":1,"name":"foo","secret":"hidden"},{"id":2,"name":"bar","secret":"also hidden"}]`}, nil
			},
		},
		specs: map[string][]mcp.CompactField{
			"testint_list_items": mustParseCompactSpecs(t, []string{"id", "name"}),
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	// Re-register with the compaction-aware mock.
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	result, err := s.handleExecute(context.Background(), executeRequest("testint_list_items", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &items))
	assert.Len(t, items, 2)
	assert.Equal(t, float64(1), items[0]["id"])
	assert.Equal(t, "foo", items[0]["name"])
	assert.NotContains(t, items[0], "secret", "compaction should remove unlisted fields")
}

func TestHandleExecute_CompactionSkippedWhenNotImplemented(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_get_item", Description: "Get item"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":1,"secret":"visible"}`}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("testint_get_item", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Contains(t, tc.Text, "secret", "non-compact integration should return full response")
}

func TestHandleExecute_CompactionSkippedOnNilSpec(t *testing.T) {
	mi := &mockFieldCompactionIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: "testint_create_item", Description: "Create item"},
			},
			execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `{"id":1,"all_fields":"present"}`}, nil
			},
		},
		specs: map[string][]mcp.CompactField{}, // no spec for testint_create_item
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	result, err := s.handleExecute(context.Background(), executeRequest("testint_create_item", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Contains(t, tc.Text, "all_fields", "nil spec should pass response through unchanged")
}

func TestHandleExecute_CompactionSkippedOnErrorResult(t *testing.T) {
	mi := &mockFieldCompactionIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: "testint_list_items", Description: "List items"},
			},
			execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: "API rate limit exceeded", IsError: true}, nil
			},
		},
		specs: map[string][]mcp.CompactField{
			"testint_list_items": mustParseCompactSpecs(t, []string{"id", "name"}),
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	result, err := s.handleExecute(context.Background(), executeRequest("testint_list_items", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Equal(t, "API rate limit exceeded", tc.Text, "error results should not be compacted")
}

func TestHandleExecute_ByteCapEnforced(t *testing.T) {
	// Generate a response over 50KB.
	bigData := `{"data":"` + string(make([]byte, 60*1024)) + `"}`
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_big", Description: "Returns huge data"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: bigData}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("testint_big", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError, "over-cap response should return error")

	tc := result.Content[0].(*mcpsdk.TextContent)
	capKB := fmt.Sprintf("%dKB", maxResponseBytes/1024)
	assert.Contains(t, tc.Text, capKB)
}

func TestHandleExecute_ByteCapSkippedOnError(t *testing.T) {
	bigErr := string(make([]byte, 60*1024))
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_big_err", Description: "Returns huge error"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: bigErr, IsError: true}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("testint_big_err", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	capKB := fmt.Sprintf("%dKB", maxResponseBytes/1024)
	assert.NotContains(t, tc.Text, capKB, "error results should skip byte cap")
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

func TestExecuteTool_SingleTool(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_get_item", Description: "Get an item"},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":"123","name":"widget"}`}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.executeTool(context.Background(), "testint_get_item", map[string]any{"id": "123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "widget")
}

func TestExecuteTool_NotFound(t *testing.T) {
	s := setupTestServer()
	result, err := s.executeTool(context.Background(), "nonexistent_tool", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
}

func TestScriptExecution_SingleCall(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_list_items", Description: "List items"},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `[{"id":1,"name":"alpha"},{"id":2,"name":"beta"}]`}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.scriptEngine.Run(context.Background(), `
		var items = api.call("testint_list_items", {});
		items.length;
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "2", result.Data)
}

func TestScriptExecution_ChainedCalls(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_list_items", Description: "List items"},
			{Name: "testint_get_detail", Description: "Get item detail"},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			switch toolName {
			case "testint_list_items":
				return &mcp.ToolResult{Data: `[{"id":"abc"},{"id":"def"}]`}, nil
			case "testint_get_detail":
				id, _ := args["id"].(string)
				return &mcp.ToolResult{Data: fmt.Sprintf(`{"id":"%s","detail":"info for %s"}`, id, id)}, nil
			}
			return &mcp.ToolResult{Data: "unknown", IsError: true}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.scriptEngine.Run(context.Background(), `
		var items = api.call("testint_list_items", {});
		var details = [];
		for (var i = 0; i < items.length; i++) {
			details.push(api.call("testint_get_detail", {id: items[i].id}));
		}
		({count: details.length, first: details[0].detail});
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, float64(2), parsed["count"])
	assert.Equal(t, "info for abc", parsed["first"])
}

func TestScriptExecution_FilterResults(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "testint_list_items", Description: "List items"},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `[{"name":"a","active":true},{"name":"b","active":false},{"name":"c","active":true}]`}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.scriptEngine.Run(context.Background(), `
		var items = api.call("testint_list_items", {});
		var active = items.filter(function(i) { return i.active; });
		active.map(function(i) { return i.name; });
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var names []string
	require.NoError(t, json.Unmarshal([]byte(result.Data), &names))
	assert.Equal(t, []string{"a", "c"}, names)
}

func TestScriptExecution_ToolNotFound(t *testing.T) {
	s := setupTestServer()
	result, err := s.scriptEngine.Run(context.Background(), `
		api.call("nonexistent_tool", {});
	`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
}

func TestHandleExecute_EmptyArgs(t *testing.T) {
	s := setupTestServer()
	result, err := s.executeTool(context.Background(), "", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
}

func TestExecuteTool_RetriesOnRetryableError(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_flaky", Description: "flaky tool"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			if calls < 3 {
				return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
			}
			return &mcp.ToolResult{Data: "ok"}, nil
		},
		healthy: true,
	})
	s.retryBackoff = 0
	result, err := s.executeTool(context.Background(), "test_flaky", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "ok", result.Data)
	assert.Equal(t, 3, calls, "should have retried until success")
}

func TestExecuteTool_ReturnsErrorAfterMaxRetries(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_down", Description: "always 503"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	result, err := s.executeTool(context.Background(), "test_down", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "service unavailable")
	assert.Equal(t, 3, calls, "should attempt exactly 3 times")
}

func TestComputeBackoff_ReturnsValueWithinJitterBounds(t *testing.T) {
	s := &Server{retryBackoff: 100 * time.Millisecond}

	for attempt := range 3 {
		base := s.retryBackoff << attempt
		half := base / 2

		// Run enough times to verify bounds and variation.
		var vals []time.Duration
		for range 50 {
			d := s.computeBackoff(attempt)
			assert.GreaterOrEqual(t, d, half, "attempt %d: backoff must be ≥ base/2", attempt)
			assert.LessOrEqual(t, d, base, "attempt %d: backoff must be ≤ base", attempt)
			vals = append(vals, d)
		}

		// At least 2 distinct values in 50 samples (deterministic would produce 1).
		distinct := map[time.Duration]bool{}
		for _, v := range vals {
			distinct[v] = true
		}
		assert.Greater(t, len(distinct), 1, "attempt %d: jitter should produce varying values", attempt)
	}
}

func TestExecuteTool_DoesNotRetryNonRetryableErrors(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_bad", Description: "permanent error"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return nil, fmt.Errorf("json marshal failed")
		},
		healthy: true,
	})
	_, err := s.executeTool(context.Background(), "test_bad", map[string]any{})
	require.Error(t, err)
	assert.Equal(t, 1, calls, "should not retry non-retryable errors")
}

func TestExecuteTool_DoesNotRetryToolResultErrors(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_4xx", Description: "client error"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return &mcp.ToolResult{Data: "not found", IsError: true}, nil
		},
		healthy: true,
	})
	result, err := s.executeTool(context.Background(), "test_4xx", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 1, calls, "should not retry ToolResult errors")
}

func TestExecuteTool_RespectsContextCancellationDuringBackoff(t *testing.T) {
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_slow", Description: "fails then cancelled"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			if calls == 1 {
				cancel() // cancel during first failure — should abort before retry
			}
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("unavailable")}
		},
		healthy: true,
	})
	s.retryBackoff = time.Second // long backoff so cancellation wins the select race

	_, err := s.executeTool(ctx, "test_slow", map[string]any{})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls, "should not retry after context cancellation")
}

func TestExecuteTool_UsesRetryAfterWhenProvided(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_ratelimit", Description: "429 with retry-after"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			if calls == 1 {
				return nil, &mcp.RetryableError{StatusCode: 429, Err: fmt.Errorf("rate limited"), RetryAfter: 15 * time.Millisecond}
			}
			return &mcp.ToolResult{Data: "ok"}, nil
		},
		healthy: true,
	})
	s.retryBackoff = time.Second // default backoff is very long

	start := time.Now()
	result, err := s.executeTool(context.Background(), "test_ratelimit", map[string]any{})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.False(t, result.IsError)
	// If using RetryAfter (15ms), elapsed should be much less than default backoff (1s).
	assert.Less(t, elapsed, 500*time.Millisecond, "should use RetryAfter delay, not default backoff")
	assert.Equal(t, 2, calls)
}

func TestExecuteTool_CircuitBreakerTripsAfterRepeatedFailures(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_breaker", Description: "always 503"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 5
	s.breakerCooldown = time.Minute

	// Each executeTool call with maxRetries=3 records 3 failures.
	// After 2 calls (6 failures ≥ threshold of 5), breaker should be open.
	s.executeTool(context.Background(), "test_breaker", map[string]any{})
	s.executeTool(context.Background(), "test_breaker", map[string]any{})

	callsBefore := calls
	result, err := s.executeTool(context.Background(), "test_breaker", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "circuit breaker open")
	assert.Equal(t, callsBefore, calls, "should not call integration when breaker is open")
}

func TestExecuteTool_CircuitBreakerResetsOnSuccess(t *testing.T) {
	callCount := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_recover", Description: "fails then recovers"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			callCount++
			if callCount <= 6 {
				return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
			}
			return &mcp.ToolResult{Data: "ok"}, nil
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 5
	s.breakerCooldown = 50 * time.Millisecond

	// Trip the breaker (6 failures across 2 calls).
	s.executeTool(context.Background(), "test_recover", map[string]any{})
	s.executeTool(context.Background(), "test_recover", map[string]any{})

	// Wait for cooldown.
	time.Sleep(60 * time.Millisecond)

	// Half-open: allows one probe. Mock now returns success.
	result, err := s.executeTool(context.Background(), "test_recover", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "ok", result.Data)
}

func TestExecuteTool_BudgetExhaustionRecordsFailure(t *testing.T) {
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_budget_breaker", Description: "always 503"},
		},
		execFn: func(_ context.Context, _ string, _ map[string]any) (*mcp.ToolResult, error) {
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 2
	s.breakerCooldown = time.Minute

	// Budget of 1: attempt 0 fails → record failure, consume retry (budget=0) →
	// attempt 1 fails → record failure, budget exhausted → break.
	// Two failures should trip the breaker (threshold=2).
	ctx := withRetryBudget(context.Background(), 1)
	s.executeTool(ctx, "test_budget_breaker", map[string]any{})

	// If failures were recorded correctly, breaker should now be open.
	result, err := s.executeTool(context.Background(), "test_budget_breaker", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "circuit breaker open")
}

func TestScriptExecution_RetriesShareBudget(t *testing.T) {
	callCounts := map[string]int{}
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: "test_flaky_a", Description: "flaky a"},
			{Name: "test_flaky_b", Description: "flaky b"},
		},
		execFn: func(_ context.Context, toolName string, _ map[string]any) (*mcp.ToolResult, error) {
			callCounts[toolName]++
			// Each tool fails twice then succeeds on 3rd attempt (needs 2 retries).
			if callCounts[toolName] <= 2 {
				return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("unavailable")}
			}
			return &mcp.ToolResult{Data: fmt.Sprintf(`{"tool":"%s"}`, toolName)}, nil
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 100 // disable breaker for this test

	// Budget of 3 retries total. Tool A uses 2, leaving 1 for tool B (needs 2).
	// Without budget wiring: both tools succeed (each has 3 attempts via maxRetries).
	// With budget wiring: tool B fails because budget only allows 1 more retry.
	result, err := s.scriptEngine.Run(
		withRetryBudget(context.Background(), 3),
		`var a = api.call("test_flaky_a", {}); var b = api.call("test_flaky_b", {}); ({a: a, b: b});`,
	)
	require.NoError(t, err)
	assert.True(t, result.IsError, "second tool should fail — retry budget exhausted")
}

func TestHandleExecute_NeitherToolNameNorScript(t *testing.T) {
	s := setupTestServer()
	req := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "execute",
			Arguments: json.RawMessage(`{}`),
		},
	}
	result, err := s.handleExecute(context.Background(), req)
	require.NoError(t, err)
	require.True(t, result.IsError)
	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Equal(t, "either tool_name or script is required", tc.Text)
}

func TestScriptExecution_PRReviewScript(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_pull", Description: "Get a pull request"},
			{Name: "github_get_pull_diff", Description: "Get the raw diff"},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			switch toolName {
			case "github_get_pull":
				return &mcp.ToolResult{Data: `{"title":"Fix bug","state":"open","body":"Fixes issue #1","base":{"ref":"main"},"head":{"ref":"fix-branch"}}`}, nil
			case "github_get_pull_diff":
				return &mcp.ToolResult{Data: "diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,4 @@\n package main\n+import \"fmt\"\n func main() {}"}, nil
			}
			return &mcp.ToolResult{Data: "unknown", IsError: true}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.scriptEngine.Run(context.Background(), `
		var pr = api.call("github_get_pull", {owner: "o", repo: "r", pull_number: 37});
		var diff = api.call("github_get_pull_diff", {owner: "o", repo: "r", pull_number: 37});
		({title: pr.title, state: pr.state, body: pr.body, base: pr.base.ref, head: pr.head.ref, diff: diff});
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "Fix bug", parsed["title"])
	assert.Equal(t, "open", parsed["state"])
	assert.Equal(t, "main", parsed["base"])
	assert.Equal(t, "fix-branch", parsed["head"])
	assert.Contains(t, parsed["diff"], "diff --git")
}

func TestScriptExecution_CrossIntegration(t *testing.T) {
	linear := &mockIntegration{
		name:    "linear",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "linear_create_issue", Description: "Create a Linear issue"},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			title, _ := args["title"].(string)
			return &mcp.ToolResult{Data: fmt.Sprintf(`{"identifier":"ENG-42","title":"%s","url":"https://linear.app/team/issue/ENG-42"}`, title)}, nil
		},
	}

	gh := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_create_pull", Description: "Create a pull request"},
		},
		execFn: func(_ context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
			title, _ := args["title"].(string)
			body, _ := args["body"].(string)
			return &mcp.ToolResult{Data: fmt.Sprintf(`{"html_url":"https://github.com/o/r/pull/99","title":"%s","body":"%s"}`, title, body)}, nil
		},
	}

	s := setupTestServer(linear, gh)
	result, err := s.scriptEngine.Run(context.Background(), `
		var issue = api.call("linear_create_issue", {team_id: "TEAM", title: "Fix auth bug"});
		var pr = api.call("github_create_pull", {
			owner: "o", repo: "r",
			title: issue.identifier + ": " + issue.title,
			head: "fix-auth", base: "main",
			body: "Resolves " + issue.url
		});
		({issue: issue.identifier, pr_url: pr.html_url, pr_title: pr.title});
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	assert.Equal(t, "ENG-42", parsed["issue"])
	assert.Equal(t, "https://github.com/o/r/pull/99", parsed["pr_url"])
	assert.Equal(t, "ENG-42: Fix auth bug", parsed["pr_title"])
}

func TestSearch_ScriptHint_MultipleIntegrations(t *testing.T) {
	alpha := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: "github_list_issues", Description: "List issues"}},
	}
	beta := &mockIntegration{
		name:    "linear",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: "linear_list_issues", Description: "List issues"}},
	}
	s := setupTestServer(alpha, beta)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"query": "list issues"}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)
	assert.Contains(t, resp.ScriptHint, "multiple integrations")
}

func TestSearch_ScriptHint_SingleIntegrationMultipleTools(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: "github_get_pull", Description: "Get a pull request"},
			{Name: "github_get_pull_diff", Description: "Get diff"},
		},
	}
	s := setupTestServer(mi)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"query": "pull"}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)
	assert.Contains(t, resp.ScriptHint, "multiple tool calls")
}

func TestSearch_ScriptHint_SingleResult(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: "github_get_pull", Description: "Get a pull request"}},
	}
	s := setupTestServer(mi)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"query": "get pull"}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)
	assert.Empty(t, resp.ScriptHint)
}
