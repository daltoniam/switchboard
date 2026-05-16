package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
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
func (m *mockConfigService) SetWasmModules(modules []mcp.WasmModuleConfig) error {
	m.cfg.WasmModules = modules
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
	execFn    func(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error)
	healthy   bool
	configErr error
}

func (m *mockIntegration) Name() string { return m.name }
func (m *mockIntegration) Configure(_ context.Context, _ mcp.Credentials) error {
	return m.configErr
}
func (m *mockIntegration) Tools() []mcp.ToolDefinition { return m.tools }
func (m *mockIntegration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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
func (r *mockRegistry) Unregister(name string) (mcp.Integration, bool) {
	i, ok := r.integrations[name]
	if ok {
		delete(r.integrations, name)
	}
	return i, ok
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
		Name:        mcp.ToolName("github_list_issues"),
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
		Name:        mcp.ToolName("datadog_search_logs"),
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
				Name:        mcp.ToolName("testint_list_items"),
				Description: "List test items",
				Parameters:  map[string]string{"query": "Search query"},
			},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

func TestConfigureIntegrations_DisablesPreviouslyEnabledOnFailure(t *testing.T) {
	mi := &mockIntegration{
		name:      "failint",
		configErr: fmt.Errorf("connection timeout"),
	}

	reg := newMockRegistry()
	reg.Register(mi)

	cfgService := newMockConfigService(map[string]*mcp.IntegrationConfig{
		"failint": {Enabled: true, Credentials: mcp.Credentials{"host": "unreachable"}},
	})

	services := &mcp.Services{Config: cfgService, Registry: reg}
	s := New(services)
	require.NotNil(t, s)

	ic, ok := cfgService.GetIntegration("failint")
	require.True(t, ok)
	assert.False(t, ic.Enabled, "integration should be disabled after Configure failure")
	assert.Empty(t, cfgService.EnabledIntegrations())
}

func TestHandleSearch_Integration(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_alpha"), Description: "Alpha tool"},
			{Name: mcp.ToolName("testint_beta"), Description: "Beta tool for searching"},
		},
	}

	s := setupTestServer(mi)

	// Simulate calling handleSearch by verifying matches.
	var matchedTools []string
	for _, tool := range mi.Tools() {
		if matches(tool, "testint", "alpha") {
			matchedTools = append(matchedTools, string(tool.Name))
		}
	}
	assert.Equal(t, []string{"testint_alpha"}, matchedTools)

	// Empty query matches all.
	var allTools []string
	for _, tool := range mi.Tools() {
		if matches(tool, "testint", "") {
			allTools = append(allTools, string(tool.Name))
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
// Tools is json.RawMessage because it can be either a plain array (<4 items)
// or a columnar object (4+ items with columns/rows/constants).
type searchResponse struct {
	Summary          string            `json:"summary"`
	ScriptHint       string            `json:"script_hint"`
	SharedParameters map[string]string `json:"shared_parameters"`
	Total            int               `json:"total"`
	Offset           int               `json:"offset"`
	Limit            int               `json:"limit"`
	HasMore          bool              `json:"has_more"`
	Integrations     []string          `json:"integrations"`
	Tools            json.RawMessage   `json:"tools"`
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

// searchToolCount returns the number of tools in a search response,
// handling both per-record array and columnar formats.
func searchToolCount(t *testing.T, resp searchResponse) int {
	t.Helper()
	var arr []any
	if err := json.Unmarshal(resp.Tools, &arr); err == nil {
		return len(arr)
	}
	var obj struct {
		Rows []json.RawMessage `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(resp.Tools, &obj))
	return len(obj.Rows)
}

// searchToolNames extracts tool names from a search response,
// handling both per-record array and columnar formats.
func searchToolNames(t *testing.T, resp searchResponse) []string {
	t.Helper()

	// Try per-record array first.
	var arr []struct{ Name string }
	if err := json.Unmarshal(resp.Tools, &arr); err == nil {
		names := make([]string, len(arr))
		for i, tool := range arr {
			names[i] = tool.Name
		}
		return names
	}

	// Columnar format: find "name" column index, extract from rows.
	var obj struct {
		Columns   []string            `json:"columns"`
		Constants map[string]string   `json:"constants"`
		Rows      [][]json.RawMessage `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(resp.Tools, &obj))

	nameIdx := -1
	for i, c := range obj.Columns {
		if c == "name" {
			nameIdx = i
			break
		}
	}

	var names []string
	if nameIdx >= 0 {
		for _, row := range obj.Rows {
			var name string
			require.NoError(t, json.Unmarshal(row[nameIdx], &name))
			names = append(names, name)
		}
	}
	return names
}

// extractColumnarParams parses the parameters column from columnar search tools JSON.
func extractColumnarParams(t *testing.T, toolsRaw json.RawMessage) []map[string]string {
	t.Helper()
	var obj struct {
		Columns []string            `json:"columns"`
		Rows    [][]json.RawMessage `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(toolsRaw, &obj))

	paramIdx := -1
	for i, c := range obj.Columns {
		if c == "parameters" {
			paramIdx = i
			break
		}
	}
	require.NotEqual(t, -1, paramIdx, "should have parameters column")

	var result []map[string]string
	for _, row := range obj.Rows {
		var params map[string]string
		require.NoError(t, json.Unmarshal(row[paramIdx], &params))
		result = append(result, params)
	}
	return result
}

// searchToolIntegrations extracts integration values from a search response.
func searchToolIntegrations(t *testing.T, resp searchResponse) []string {
	t.Helper()

	var arr []struct{ Integration string }
	if err := json.Unmarshal(resp.Tools, &arr); err == nil {
		integrations := make([]string, len(arr))
		for i, tool := range arr {
			integrations[i] = tool.Integration
		}
		return integrations
	}

	var obj struct {
		Columns   []string            `json:"columns"`
		Constants map[string]string   `json:"constants"`
		Rows      [][]json.RawMessage `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(resp.Tools, &obj))

	// Check constants first (integration is often lifted).
	if v, ok := obj.Constants["integration"]; ok {
		result := make([]string, len(obj.Rows))
		for i := range result {
			result[i] = v
		}
		return result
	}

	intIdx := -1
	for i, c := range obj.Columns {
		if c == "integration" {
			intIdx = i
			break
		}
	}

	var integrations []string
	if intIdx >= 0 {
		for _, row := range obj.Rows {
			var v string
			require.NoError(t, json.Unmarshal(row[intIdx], &v))
			integrations = append(integrations, v)
		}
	}
	return integrations
}

// makeManyTools generates n tool definitions for testing pagination.
// diverseMockTools returns a set of tools with diverse vocabulary so that
// IDF has meaningful contrast in small test fixtures. Without this,
// single-tool test servers produce IDF=0 for all words (every word
// appears in 100% of the corpus).
func diverseMockTools() []mcp.ToolDefinition {
	return []mcp.ToolDefinition{
		{Name: mcp.ToolName("bg_upload_photo"), Description: "Upload a photo to storage"},
		{Name: mcp.ToolName("bg_analyze_report"), Description: "Analyze a quarterly report"},
		{Name: mcp.ToolName("bg_schedule_meeting"), Description: "Schedule a calendar meeting"},
		{Name: mcp.ToolName("bg_export_csv"), Description: "Export data as CSV format"},
		{Name: mcp.ToolName("bg_validate_schema"), Description: "Validate JSON schema definitions"},
		{Name: mcp.ToolName("bg_rotate_credentials"), Description: "Rotate API keys and secrets"},
		{Name: mcp.ToolName("bg_generate_invoice"), Description: "Generate a billing invoice"},
		{Name: mcp.ToolName("bg_compress_archive"), Description: "Compress files into an archive"},
	}
}

func makeManyTools(prefix string, n int) []mcp.ToolDefinition {
	tools := make([]mcp.ToolDefinition, n)
	for i := range n {
		tools[i] = mcp.ToolDefinition{
			Name:        mcp.ToolName(fmt.Sprintf("%s_tool_%d", prefix, i)),
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
			assert.Equal(t, tt.wantTools, searchToolCount(t, resp))
			assert.Equal(t, tt.wantHasMore, resp.HasMore, "has_more")
		})
	}
}

func TestHandleSearch_QueryFiltersCombinedWithPagination(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_list_a"), Description: "List A items"},
			{Name: mcp.ToolName("testint_list_b"), Description: "List B items"},
			{Name: mcp.ToolName("testint_list_c"), Description: "List C items"},
			{Name: mcp.ToolName("testint_get_x"), Description: "Get X"},
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
	assert.Equal(t, 2, searchToolCount(t, resp))
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
			{Name: mcp.ToolName("alpha_b"), Description: "B"},
			{Name: mcp.ToolName("alpha_a"), Description: "A"},
		},
	}
	beta := &mockIntegration{
		name:    "beta",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("beta_a"), Description: "A"},
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

	names1 := searchToolNames(t, resp1)
	names2 := searchToolNames(t, resp2)
	require.Len(t, names1, 3)
	require.Len(t, names2, 3)

	// Must be sorted by integration name, then tool name.
	assert.Equal(t, "alpha_a", names1[0])
	assert.Equal(t, "alpha_b", names1[1])
	assert.Equal(t, "beta_a", names1[2])

	// Same order on second call.
	assert.Equal(t, names1, names2)
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
			"query": "zzzznonexistent zzzzxyz",
		}))
		require.NoError(t, err)

		resp := parseSearchResponse(t, result)
		assert.ElementsMatch(t, []string{"alpha", "beta"}, resp.Integrations)
		assert.Equal(t, 0, searchToolCount(t, resp))
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
	assert.Equal(t, 3, searchToolCount(t, resp))
	assert.Contains(t, resp.Summary, "15")
	assert.Contains(t, resp.ScriptHint, "script")
}

// --- markdown integration mock ---

type mockMarkdownIntegration struct {
	mockIntegration
	renderFn func(toolName mcp.ToolName, data []byte) (mcp.Markdown, bool)
}

func (m *mockMarkdownIntegration) RenderMarkdown(toolName mcp.ToolName, data []byte) (mcp.Markdown, bool) {
	return m.renderFn(toolName, data)
}

// --- compaction integration mock ---

type mockFieldCompactionIntegration struct {
	mockIntegration
	specs map[mcp.ToolName][]mcp.CompactField
}

func (m *mockFieldCompactionIntegration) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
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
				{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `[{"id":1,"name":"a","secret":"s"},{"id":2,"name":"b","secret":"s"},{"id":3,"name":"c","secret":"s"},{"id":4,"name":"d","secret":"s"},{"id":5,"name":"e","secret":"s"},{"id":6,"name":"f","secret":"s"},{"id":7,"name":"g","secret":"s"},{"id":8,"name":"h","secret":"s"}]`}, nil
			},
		},
		specs: map[mcp.ToolName][]mcp.CompactField{
			mcp.ToolName("testint_list_items"): mustParseCompactSpecs(t, []string{"id", "name"}),
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	// Re-register with the compaction-aware mock.
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	result, err := s.handleExecute(context.Background(), executeRequest("testint_list_items", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var columnar map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &columnar))

	// Array responses use columnar format: {"columns": [...], "rows": [[...], ...]}
	columns, ok := columnar["columns"].([]any)
	require.True(t, ok, "expected columnar format with 'columns' key")
	assert.Equal(t, []any{"id", "name"}, columns)

	rows, ok := columnar["rows"].([]any)
	require.True(t, ok, "expected columnar format with 'rows' key")
	assert.Len(t, rows, 8)

	row0, ok := rows[0].([]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), row0[0])
	assert.Equal(t, "a", row0[1])
}

func TestHandleExecute_CompactionSingleObjectStaysPerRecord(t *testing.T) {
	mi := &mockFieldCompactionIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_get_item"), Description: "Get item"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `{"id":1,"name":"foo","secret":"hidden"}`}, nil
			},
		},
		specs: map[mcp.ToolName][]mcp.CompactField{
			mcp.ToolName("testint_get_item"): mustParseCompactSpecs(t, []string{"id", "name"}),
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	result, err := s.handleExecute(context.Background(), executeRequest("testint_get_item", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var item map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &item))
	assert.Equal(t, float64(1), item["id"])
	assert.Equal(t, "foo", item["name"])
	assert.NotContains(t, item, "secret", "compaction should remove unlisted fields")
	assert.NotContains(t, item, "columns", "single objects should not use columnar format")
}

func TestScriptExecution_CompactedToolReturnsPerRecord(t *testing.T) {
	// Scripts calling compacted tools should get per-record data (not columnar),
	// enabling natural field access like items[i].name instead of row index counting.
	mi := &mockFieldCompactionIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `[{"id":1,"name":"foo","secret":"x"},{"id":2,"name":"bar","secret":"y"},{"id":3,"name":"baz","secret":"z"},{"id":4,"name":"qux","secret":"w"}]`}, nil
			},
		},
		specs: map[mcp.ToolName][]mcp.CompactField{
			mcp.ToolName("testint_list_items"): mustParseCompactSpecs(t, []string{"id", "name"}),
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	// Script accesses compacted fields by name — this would fail if api.call returned columnar.
	result, err := s.scriptEngine.Run(context.Background(), `
		var items = api.call("testint_list_items", {});
		var names = [];
		for (var i = 0; i < items.length; i++) {
			names.push(items[i].name);
		}
		names;
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.JSONEq(t, `["foo","bar","baz","qux"]`, result.Data)
}

func TestHandleScriptExecute_OutputColumnarized(t *testing.T) {
	// Script output should be columnarized at the MCP response boundary.
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `[{"id":1,"name":"a"},{"id":2,"name":"b"},{"id":3,"name":"c"},{"id":4,"name":"d"},{"id":5,"name":"e"},{"id":6,"name":"f"},{"id":7,"name":"g"},{"id":8,"name":"h"}]`}, nil
		},
	}

	s := setupTestServer(mi)
	scriptReq := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "execute",
			Arguments: json.RawMessage(`{"script":"api.call('testint_list_items', {});"}`),
		},
	}
	result, err := s.handleExecute(context.Background(), scriptReq)
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var columnar map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &columnar))

	columns, ok := columnar["columns"].([]any)
	require.True(t, ok, "script output should be columnarized at the response boundary")
	assert.Equal(t, []any{"id", "name"}, columns)
}

func TestHandleExecute_CompactionSkippedWhenNotImplemented(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_get_item"), Description: "Get item"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
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
				{Name: mcp.ToolName("testint_create_item"), Description: "Create item"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `{"id":1,"all_fields":"present"}`}, nil
			},
		},
		specs: map[mcp.ToolName][]mcp.CompactField{}, // no spec for testint_create_item
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
				{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: "API rate limit exceeded", IsError: true}, nil
			},
		},
		specs: map[mcp.ToolName][]mcp.CompactField{
			mcp.ToolName("testint_list_items"): mustParseCompactSpecs(t, []string{"id", "name"}),
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

func TestProcessResult_MarkdownRendered(t *testing.T) {
	mi := &mockMarkdownIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_get_page"), Description: "Get page"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `{"title":"Hello","body":"World"}`}, nil
			},
		},
		renderFn: func(_ mcp.ToolName, _ []byte) (mcp.Markdown, bool) {
			return mcp.Markdown("# Hello\n\nWorld"), true
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	result, err := s.handleExecute(context.Background(), executeRequest("testint_get_page", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Equal(t, "# Hello\n\nWorld", tc.Text)
}

func TestProcessResult_MarkdownSkippedWhenNotRendered(t *testing.T) {
	mi := &mockMarkdownIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `[{"id":1},{"id":2}]`}, nil
			},
		},
		renderFn: func(_ mcp.ToolName, _ []byte) (mcp.Markdown, bool) {
			return "", false
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	result, err := s.handleExecute(context.Background(), executeRequest("testint_list_items", nil))
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	// RenderMarkdown returned false, so JSON passes through the normal pipeline.
	assert.Contains(t, tc.Text, `"id"`, "non-rendered tool should return JSON, not markdown")
}

func TestResultProcessor_MarkdownOnly(t *testing.T) {
	rp := resultProcessor{
		markdown: func(_ mcp.ToolName, data []byte) (mcp.Markdown, bool) {
			return mcp.Markdown("# rendered"), true
		},
	}
	got := processResult(rp, "test_tool", compact.ViewArgs{}, `{"title":"Hello"}`, nil)
	assert.Equal(t, "# rendered", got)
}

func TestResultProcessor_CompactionOnly(t *testing.T) {
	specs := mustParseCompactSpecs(t, []string{"id", "name"})
	rp := resultProcessor{
		compact: func(_ mcp.ToolName) ([]mcp.CompactField, bool) {
			return specs, true
		},
	}
	got := processResult(rp, "test_tool", compact.ViewArgs{}, `[{"id":1,"name":"a","secret":"s"},{"id":2,"name":"b","secret":"s"}]`, nil)
	assert.Contains(t, got, `"id"`)
	assert.Contains(t, got, `"name"`)
	assert.NotContains(t, got, `"secret"`)
}

func TestResultProcessor_MarkdownTakesPriority(t *testing.T) {
	specs := mustParseCompactSpecs(t, []string{"id"})
	rp := resultProcessor{
		markdown: func(_ mcp.ToolName, _ []byte) (mcp.Markdown, bool) {
			return mcp.Markdown("# markdown wins"), true
		},
		compact: func(_ mcp.ToolName) ([]mcp.CompactField, bool) {
			return specs, true
		},
	}
	got := processResult(rp, "test_tool", compact.ViewArgs{}, `{"id":1,"secret":"s"}`, nil)
	assert.Equal(t, "# markdown wins", got, "markdown should take priority over compaction")
}

func TestResultProcessor_NoOp(t *testing.T) {
	rp := resultProcessor{}
	input := `[{"id":1},{"id":2}]`
	got := processResult(rp, "test_tool", compact.ViewArgs{}, input, nil)
	// With no processors, data passes through columnarization only.
	assert.Contains(t, got, `"id"`)
}

func TestResultProcessor_MarkdownReturnsFalse_FallsThrough(t *testing.T) {
	rp := resultProcessor{
		markdown: func(_ mcp.ToolName, _ []byte) (mcp.Markdown, bool) {
			return "", false
		},
	}
	got := processResult(rp, "test_tool", compact.ViewArgs{}, `{"id":1}`, nil)
	assert.Contains(t, got, `"id"`, "should fall through to JSON processing")
}

func TestResultProcessor_MaxBytesExceededReplacesWithEnvelope(t *testing.T) {
	rp := resultProcessor{
		maxBytes: func(_ mcp.ToolName) (int, bool) { return 50, true },
	}
	big := `{"items":["` + strings.Repeat("x", 200) + `"]}`
	got := processResult(rp, "foo", compact.ViewArgs{}, big, nil)

	// Envelope must be valid JSON the LLM can parse.
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &env), "envelope must be parseable JSON")
	assert.Equal(t, "response_too_large", env["error"])
	assert.Equal(t, "foo", env["tool"])
	assert.EqualValues(t, 50, env["limit"])
	assert.Contains(t, env, "size")
	assert.Contains(t, env, "hint")
}

func TestResultProcessor_MaxBytesUnderLimitUnchanged(t *testing.T) {
	rp := resultProcessor{
		maxBytes: func(_ mcp.ToolName) (int, bool) { return 1000, true },
	}
	small := `{"ok":true}`
	got := processResult(rp, "foo", compact.ViewArgs{}, small, nil)
	assert.NotContains(t, got, "response_too_large", "under-limit response should not be replaced")
	assert.Contains(t, got, `"ok"`)
}

func TestResultProcessor_MaxBytesUnsetNoCheck(t *testing.T) {
	rp := resultProcessor{
		maxBytes: func(_ mcp.ToolName) (int, bool) { return 0, false },
	}
	big := `{"x":"` + strings.Repeat("a", 1000) + `"}`
	got := processResult(rp, "foo", compact.ViewArgs{}, big, nil)
	assert.NotContains(t, got, "response_too_large", "no cap set: no check applied")
	assert.Contains(t, got, "aaaa")
}

func TestResultProcessor_MaxBytesZeroLimitNoCheck(t *testing.T) {
	// Defensive: limit=0 with ok=true should be treated as "no cap" (matches Result.MaxBytes
	// semantics where absence is "no cap", not zero).
	rp := resultProcessor{
		maxBytes: func(_ mcp.ToolName) (int, bool) { return 0, true },
	}
	big := `{"x":"` + strings.Repeat("a", 1000) + `"}`
	got := processResult(rp, "foo", compact.ViewArgs{}, big, nil)
	assert.NotContains(t, got, "response_too_large", "limit=0 should not trigger cap check")
}

// ── Views dispatch ──────────────────────────────────────────────────

// buildTestViewSet constructs a minimal ViewSet for testing. Two views
// (toc, full); toc supports json only, full supports json + markdown.
// Renderers are simple identity-style functions — tests assert dispatch
// logic, not rendering quality.
func buildTestViewSet(t *testing.T) compact.ViewSet {
	t.Helper()
	jsonR := func(v any) ([]byte, error) { return json.Marshal(v) }
	mdR := func(v any) ([]byte, error) { return []byte("MARKDOWN"), nil }
	return compact.ViewSet{
		Default: compact.ViewSelection{View: "toc", Format: compact.FormatJSON},
		Views: map[compact.ViewName]compact.ParsedView{
			"toc":  {Hint: "Just titles.", Formats: []compact.Format{compact.FormatJSON}},
			"full": {Hint: "Whole thing.", Formats: []compact.Format{compact.FormatJSON, compact.FormatMarkdown}},
		},
		Renderers: map[compact.ViewName]map[compact.Format]compact.Renderer{
			"toc":  {compact.FormatJSON: jsonR},
			"full": {compact.FormatJSON: jsonR, compact.FormatMarkdown: mdR},
		},
	}
}

func TestProcessViews_DefaultsApplyWhenSelectionEmpty(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	// Zero-value ViewArgs == "LLM didn't specify view/format" — defaults apply.
	got := processResult(rp, "tool", compact.ViewArgs{}, `{"id":1}`, nil)
	assert.Contains(t, got, `"id"`, "default (toc, json) marshals input back")
}

func TestProcessViews_ViewArgSelectsView(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	view := compact.ViewArgs{View: "full", Format: "markdown"}
	got := processResult(rp, "tool", view, `{"id":1}`, nil)
	assert.Equal(t, "MARKDOWN", got, "full+markdown should hit the markdown renderer")
}

func TestProcessViews_UnknownView_ReturnsErrorEnvelope(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	view := compact.ViewArgs{View: "wat"}
	got := processResult(rp, "tool", view, `{"id":1}`, nil)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &env))
	assert.Equal(t, "view_dispatch_failed", env["error"])
	assert.Contains(t, env["message"], "unknown view")
	assert.Contains(t, env["message"], "wat")
}

func TestProcessViews_UndeclaredFormatForView_ReturnsErrorEnvelope(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	view := compact.ViewArgs{View: "toc", Format: "markdown"}
	got := processResult(rp, "tool", view, `{"id":1}`, nil)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &env))
	assert.Equal(t, "view_dispatch_failed", env["error"])
	assert.Contains(t, env["message"], "does not declare format")
}

func TestProcessViews_ToolWithoutViews_FallsThrough(t *testing.T) {
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return compact.ViewSet{}, false },
	}
	// view selection present but the tool has no views — should be ignored.
	got := processResult(rp, "tool", compact.ViewArgs{View: "anything"}, `{"id":1}`, nil)
	assert.Contains(t, got, `"id"`, "tools without views should use existing path; view ignored")
}

// ViewArgs.Err() flows from request boundary (ParseViewArgs) through to the
// dispatch path. processViewsResult surfaces it as a view_dispatch_failed
// envelope without attempting to render. This pins the connection so a
// future refactor can't accidentally swallow the parse error.
func TestProcessViews_ViewArgsWithParseError_ReturnsErrorEnvelope(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	// Simulate a parse-boundary type error reaching the dispatch path.
	view := compact.ParseViewArgs(map[string]any{"view": 123})
	require.Error(t, view.Err(), "test fixture: parse should have failed")

	got := processResult(rp, "tool", view, `{"id":1}`, nil)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &env))
	assert.Equal(t, "view_dispatch_failed", env["error"])
	assert.Contains(t, env["message"], "must be string")
}

// resolveSelection is the second half of view dispatch — fills in defaults
// from ViewSet, then validates against registered renderers. Pin the three
// branches: empty selection → defaults, unknown view → error, undeclared
// format → error.
func TestResolveSelection_EmptyViewArgs_UsesDefaults(t *testing.T) {
	vs := buildTestViewSet(t)
	sel, err := resolveSelection(compact.ViewArgs{}, vs)
	require.NoError(t, err)
	assert.Equal(t, vs.Default.View, sel.View)
	assert.Equal(t, vs.Default.Format, sel.Format)
}

func TestResolveSelection_PartialViewArgs_FillsDefaultFormat(t *testing.T) {
	vs := buildTestViewSet(t)
	// View specified, format omitted → format falls back to default.
	sel, err := resolveSelection(compact.ViewArgs{View: "full"}, vs)
	require.NoError(t, err)
	assert.Equal(t, compact.ViewName("full"), sel.View)
	assert.Equal(t, vs.Default.Format, sel.Format)
}

func TestResolveSelection_UnknownView_ReturnsError(t *testing.T) {
	vs := buildTestViewSet(t)
	_, err := resolveSelection(compact.ViewArgs{View: "nonexistent"}, vs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown view")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestResolveSelection_UndeclaredFormat_ReturnsError(t *testing.T) {
	vs := buildTestViewSet(t)
	// `toc` only declares JSON; markdown is undeclared.
	_, err := resolveSelection(compact.ViewArgs{View: "toc", Format: "markdown"}, vs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not declare format")
}

// The flat compaction path records metrics only when output is smaller
// than input (genuine compression). The views path must follow the same
// contract: appendMoreHint can make output larger than input, and a
// blind RecordCompaction here would feed negative savings into
// monitoring. Pin the guard so a future refactor can't drop it.
func TestProcessViews_NoCompactionRecordedWhenOutputGrows(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	metrics := mcp.NewMetrics()

	// Tiny input + default view + _more envelope = output > input.
	input := `{"id":1}`
	got := processResult(rp, "tool", compact.ViewArgs{}, input, metrics)
	require.Greater(t, len(got), len(input), "test fixture: output must exceed input for this assertion to be meaningful")

	snap := metrics.Snapshot()
	assert.Equal(t, 0, snap.CompactionSamples,
		"RecordCompaction must not fire when output grew past input (negative savings)")
}

// Map iteration order in Go is non-deterministic. Error messages built
// by iterating viewSet.Views or viewSet.Renderers will flake any test
// that asserts substring content. listViewNames and listFormats must
// sort their output for stable, predictable errors across runs.
func TestListViewNames_SortedOutput(t *testing.T) {
	vs := compact.ViewSet{
		Views: map[compact.ViewName]compact.ParsedView{
			"zoo":    {},
			"alpha":  {},
			"middle": {},
		},
	}
	assert.Equal(t, "alpha, middle, zoo", listViewNames(vs),
		"view names must be sorted for deterministic error messages")
}

func TestListFormats_SortedOutput(t *testing.T) {
	vs := compact.ViewSet{
		Renderers: map[compact.ViewName]map[compact.Format]compact.Renderer{
			"v": {
				"yaml":     nil,
				"json":     nil,
				"markdown": nil,
			},
		},
	}
	assert.Equal(t, "json, markdown, yaml", listFormats(vs, "v"),
		"formats must be sorted for deterministic error messages")
}

func TestProcessViews_MoreEnvelopeOnObjectRoot(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	got := processResult(rp, "tool", compact.ViewArgs{}, `{"id":1}`, nil)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &env))

	more, ok := env["_more"].(map[string]any)
	require.True(t, ok, "_more envelope missing")
	views, ok := more["views"].(map[string]any)
	require.True(t, ok, "_more.views missing")
	assert.Contains(t, views, "full", "alternates should list `full` (not the current `toc`)")
	assert.NotContains(t, views, "toc", "current view should not appear in alternates")
}

func TestProcessViews_MoreEnvelopeWrapsArrayRoot(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	got := processResult(rp, "tool", compact.ViewArgs{}, `[{"id":1},{"id":2}]`, nil)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &env))

	data, ok := env["data"].([]any)
	require.True(t, ok, "array root should be wrapped under `data` key")
	assert.Len(t, data, 2)
	_, hasMore := env["_more"]
	assert.True(t, hasMore, "_more envelope missing on array-root response")
}

func TestProcessViews_MarkdownFormatSkipsMore(t *testing.T) {
	vs := buildTestViewSet(t)
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	view := compact.ViewArgs{View: "full", Format: "markdown"}
	got := processResult(rp, "tool", view, `{"id":1}`, nil)
	// Markdown renderer returns "MARKDOWN" — no JSON _more should be appended.
	assert.Equal(t, "MARKDOWN", got)
}

func TestProcessViews_SingleViewSkipsMore(t *testing.T) {
	// Only one view declared → no alternates → no _more envelope.
	jsonR := func(v any) ([]byte, error) { return json.Marshal(v) }
	vs := compact.ViewSet{
		Default: compact.ViewSelection{View: "only", Format: compact.FormatJSON},
		Views: map[compact.ViewName]compact.ParsedView{
			"only": {Formats: []compact.Format{compact.FormatJSON}},
		},
		Renderers: map[compact.ViewName]map[compact.Format]compact.Renderer{
			"only": {compact.FormatJSON: jsonR},
		},
	}
	rp := resultProcessor{
		views: func(_ mcp.ToolName) (compact.ViewSet, bool) { return vs, true },
	}
	got := processResult(rp, "tool", compact.ViewArgs{}, `{"id":1}`, nil)
	assert.NotContains(t, got, "_more", "single-view tool should emit no _more envelope")
}

// mockViewsIntegration wraps mockIntegration and adds the ToolViewsIntegration
// affordance so the validator-tolerance path can be exercised through the real
// type-assertion the production code uses.
type mockViewsIntegration struct {
	*mockIntegration
	viewsByTool map[mcp.ToolName]compact.ViewSet
}

func (m *mockViewsIntegration) Views(toolName mcp.ToolName) (compact.ViewSet, bool) {
	vs, ok := m.viewsByTool[toolName]
	return vs, ok
}

// reservedArgsFor is the gate between the validator and the views dispatch.
// It MUST return compact.ReservedArgs() only when the integration both
// implements ToolViewsIntegration and reports views for the named tool.
// Otherwise the validator would let view/format slip past for tools that
// can't actually dispatch them — silent misroute, the failure the view
// contract was designed to eliminate.
func TestReservedArgsFor_NonViewIntegrationReturnsNil(t *testing.T) {
	mi := &mockIntegration{name: "plain"}
	got := reservedArgsFor(mi, "plain_tool")
	assert.Nil(t, got, "plain mockIntegration does not implement ToolViewsIntegration")
}

func TestReservedArgsFor_ViewIntegrationWithoutMatchingToolReturnsNil(t *testing.T) {
	mvi := &mockViewsIntegration{
		mockIntegration: &mockIntegration{name: "vw"},
		viewsByTool: map[mcp.ToolName]compact.ViewSet{
			"vw_has_views": buildTestViewSet(t),
		},
	}
	got := reservedArgsFor(mvi, "vw_no_views")
	assert.Nil(t, got, "tool not in viewsByTool returns nil even on view-aware integration")
}

func TestReservedArgsFor_ViewIntegrationWithMatchingToolReturnsReservedArgs(t *testing.T) {
	mvi := &mockViewsIntegration{
		mockIntegration: &mockIntegration{name: "vw"},
		viewsByTool: map[mcp.ToolName]compact.ViewSet{
			"vw_tool": buildTestViewSet(t),
		},
	}
	got := reservedArgsFor(mvi, "vw_tool")
	assert.Equal(t, compact.ReservedArgs(), got)
}

// End-to-end check that the validator no longer rejects view/format on
// view-aware tools, and that non-view tools still reject them. Without
// this guard the views feature appears registered but is unreachable
// through MCP — the failure mode discovered when smoke-testing notion
// against the running binary.
func TestHandleExecute_ViewArgsPassValidatorOnViewAwareTool(t *testing.T) {
	mi := &mockIntegration{
		name:    "vw",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        mcp.ToolName("vw_get"),
				Description: "Returns shapes",
				Parameters:  map[string]string{"id": "Record ID"},
				Required:    []string{"id"},
			},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":"x"}`}, nil
		},
	}
	mvi := &mockViewsIntegration{
		mockIntegration: mi,
		viewsByTool: map[mcp.ToolName]compact.ViewSet{
			"vw_get": buildTestViewSet(t),
		},
	}

	s := setupTestServerWithIntegration(mvi)
	result, err := s.handleExecute(context.Background(), executeRequest("vw_get", map[string]any{
		"id":   "x",
		"view": "full",
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError, "view arg on view-aware tool must pass validation")
}

func TestHandleExecute_ViewArgRejectedOnNonViewTool(t *testing.T) {
	mi := &mockIntegration{
		name:    "plain",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        mcp.ToolName("plain_get"),
				Description: "Returns shapes",
				Parameters:  map[string]string{"id": "Record ID"},
				Required:    []string{"id"},
			},
		},
	}

	s := setupTestServerWithIntegration(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("plain_get", map[string]any{
		"id":   "x",
		"view": "full",
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError, "view arg on non-view tool must still fail validation")

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Contains(t, tc.Text, `unknown parameter "view"`)
}

func TestHandleExecute_ByteCapEnforced(t *testing.T) {
	// Generate a response over 50KB.
	bigData := `{"data":"` + string(make([]byte, 60*1024)) + `"}`
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_big"), Description: "Returns huge data"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: bigData}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("testint_big", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError, "over-cap response should return error")

	tc := result.Content[0].(*mcpsdk.TextContent)
	capKB := fmt.Sprintf("%dKB", defaultMaxResponseBytes/1024)
	assert.Contains(t, tc.Text, capKB)
}

func TestHandleExecute_ByteCapSkippedOnError(t *testing.T) {
	bigErr := string(make([]byte, 60*1024))
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_big_err"), Description: "Returns huge error"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: bigErr, IsError: true}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("testint_big_err", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	capKB := fmt.Sprintf("%dKB", defaultMaxResponseBytes/1024)
	assert.NotContains(t, tc.Text, capKB, "error results should skip byte cap")
}

// mockIntegrationWithCap is a mockIntegration that also implements
// mcp.MaxResponseBytesIntegration, declaring a higher per-integration cap.
type mockIntegrationWithCap struct {
	*mockIntegration
	maxBytes int
}

func (m *mockIntegrationWithCap) MaxResponseBytes() int { return m.maxBytes }

// setupTestServerWithIntegration builds a Server around a single arbitrary
// mcp.Integration — unlike setupTestServer which only accepts *mockIntegration.
// Needed for tests that register wrapper types like mockIntegrationWithCap.
func setupTestServerWithIntegration(i mcp.Integration) *Server {
	reg := newMockRegistry()
	reg.Register(i)
	services := &mcp.Services{
		Config: newMockConfigService(map[string]*mcp.IntegrationConfig{
			i.Name(): {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
		}),
		Registry: reg,
	}
	return New(services)
}

func TestHandleExecute_PerIntegrationCapHonored(t *testing.T) {
	// Payload is above the default (50KB) but under the integration's 256KB cap.
	// A default-capped integration would reject this; the override must allow it.
	bigData := `{"data":"` + strings.Repeat("x", 100*1024) + `"}`
	mi := &mockIntegrationWithCap{
		mockIntegration: &mockIntegration{
			name:    "bigint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: "bigint_get_page", Description: "Returns rich page content"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: bigData}, nil
			},
		},
		maxBytes: 256 * 1024,
	}

	s := setupTestServerWithIntegration(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("bigint_get_page", nil))
	require.NoError(t, err)
	assert.False(t, result.IsError, "response within per-integration cap should succeed")

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Equal(t, bigData, tc.Text)
}

func TestHandleExecute_PerIntegrationCapStillEnforced(t *testing.T) {
	// Payload exceeds even the integration's raised cap — must still be rejected,
	// and the error must report the integration's cap, not the default.
	bigData := `{"data":"` + strings.Repeat("x", 300*1024) + `"}`
	mi := &mockIntegrationWithCap{
		mockIntegration: &mockIntegration{
			name:    "bigint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: "bigint_get_page", Description: "Returns rich page content"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: bigData}, nil
			},
		},
		maxBytes: 256 * 1024,
	}

	s := setupTestServerWithIntegration(mi)
	result, err := s.handleExecute(context.Background(), executeRequest("bigint_get_page", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError, "response above per-integration cap should be rejected")

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Contains(t, tc.Text, "256KB", "error should report the integration's own cap")
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
			{Name: mcp.ToolName("testint_get_item"), Description: "Get an item"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"id":"123","name":"widget"}`}, nil
		},
	}

	s := setupTestServer(mi)
	integration, result, err := s.executeTool(context.Background(), "testint_get_item", map[string]any{"id": "123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "widget")
	require.NotNil(t, integration, "executeTool should return the owning integration on success")
	assert.Equal(t, "testint", integration.Name())
}

func TestExecuteTool_NotFound(t *testing.T) {
	s := setupTestServer()
	integration, result, err := s.executeTool(context.Background(), "nonexistent_tool", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
	assert.Nil(t, integration, "executeTool should return nil integration when tool not found")
}

func TestScriptExecution_SingleCall(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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
			{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
			{Name: mcp.ToolName("testint_get_detail"), Description: "Get item detail"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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
			{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

func TestScriptExecution_MetaToolReturnsSpecificError(t *testing.T) {
	s := setupTestServer()
	for _, toolName := range []string{"search", "execute"} {
		t.Run(toolName, func(t *testing.T) {
			result, err := s.scriptEngine.Run(context.Background(),
				`api.call("`+toolName+`", {});`,
			)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, "meta-tool")
			assert.Contains(t, result.Data, "cannot")
		})
	}
}

func TestHandleExecute_EmptyArgs(t *testing.T) {
	s := setupTestServer()
	_, result, err := s.executeTool(context.Background(), "", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
}

func TestExecuteTool_RetriesOnRetryableError(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_flaky"), Description: "flaky tool"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			if calls < 3 {
				return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
			}
			return &mcp.ToolResult{Data: "ok"}, nil
		},
		healthy: true,
	})
	s.retryBackoff = 0
	_, result, err := s.executeTool(context.Background(), "test_flaky", map[string]any{})
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
			{Name: mcp.ToolName("test_down"), Description: "always 503"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	integration, result, err := s.executeTool(context.Background(), "test_down", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "service unavailable")
	assert.Equal(t, 3, calls, "should attempt exactly 3 times")
	assert.Nil(t, integration, "executeTool should return nil integration when retries exhausted")
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
			{Name: mcp.ToolName("test_bad"), Description: "permanent error"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return nil, fmt.Errorf("json marshal failed")
		},
		healthy: true,
	})
	integration, _, err := s.executeTool(context.Background(), "test_bad", map[string]any{})
	require.Error(t, err)
	assert.Equal(t, 1, calls, "should not retry non-retryable errors")
	assert.Nil(t, integration, "executeTool should return nil integration on non-retryable error")
}

func TestExecuteTool_DoesNotRetryToolResultErrors(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_4xx"), Description: "client error"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return &mcp.ToolResult{Data: "not found", IsError: true}, nil
		},
		healthy: true,
	})
	_, result, err := s.executeTool(context.Background(), "test_4xx", map[string]any{})
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
			{Name: mcp.ToolName("test_slow"), Description: "fails then cancelled"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			if calls == 1 {
				cancel() // cancel during first failure — should abort before retry
			}
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("unavailable")}
		},
		healthy: true,
	})
	s.retryBackoff = time.Second // long backoff so cancellation wins the select race

	_, _, err := s.executeTool(ctx, "test_slow", map[string]any{})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls, "should not retry after context cancellation")
}

func TestExecuteTool_UsesRetryAfterWhenProvided(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_ratelimit"), Description: "429 with retry-after"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
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
	_, result, err := s.executeTool(context.Background(), "test_ratelimit", map[string]any{})
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
			{Name: mcp.ToolName("test_breaker"), Description: "always 503"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 5
	s.breakerCooldown = time.Minute

	// Each executeTool call records 1 failure (per-call, not per-attempt).
	// After 5 calls (5 failures = threshold), breaker should be open.
	for i := 0; i < 5; i++ {
		s.executeTool(context.Background(), "test_breaker", map[string]any{})
	}

	callsBefore := calls
	integration, result, err := s.executeTool(context.Background(), "test_breaker", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "circuit breaker open")
	assert.Equal(t, callsBefore, calls, "should not call integration when breaker is open")
	assert.Nil(t, integration, "executeTool should return nil integration when breaker is open")
}

func TestExecuteTool_CircuitBreakerResetsOnSuccess(t *testing.T) {
	callCount := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_recover"), Description: "fails then recovers"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			callCount++
			if callCount <= 15 {
				return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
			}
			return &mcp.ToolResult{Data: "ok"}, nil
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 5
	s.breakerCooldown = 50 * time.Millisecond

	// Trip the breaker (5 per-call failures across 5 calls).
	for i := 0; i < 5; i++ {
		s.executeTool(context.Background(), "test_recover", map[string]any{})
	}

	// Wait for cooldown.
	time.Sleep(60 * time.Millisecond)

	// Half-open: allows one probe. Mock now returns success.
	_, result, err := s.executeTool(context.Background(), "test_recover", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "ok", result.Data)
}

func TestExecuteTool_BudgetExhaustionRecordsFailure(t *testing.T) {
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_budget_breaker"), Description: "always 503"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 2
	s.breakerCooldown = time.Minute

	// With per-call counting: each executeTool call records 1 failure regardless of
	// how many retry attempts it made. Need 2 calls to trip threshold=2.
	ctx := withRetryBudget(context.Background(), 1)
	s.executeTool(ctx, "test_budget_breaker", map[string]any{})
	s.executeTool(context.Background(), "test_budget_breaker", map[string]any{})

	// If failures were recorded correctly, breaker should now be open.
	_, result, err := s.executeTool(context.Background(), "test_budget_breaker", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "circuit breaker open")
}

func TestScriptExecution_RetriesShareBudget(t *testing.T) {
	callCounts := map[string]int{}
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_flaky_a"), Description: "flaky a"},
			{Name: mcp.ToolName("test_flaky_b"), Description: "flaky b"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			callCounts[string(toolName)]++
			// Each tool fails twice then succeeds on 3rd attempt (needs 2 retries).
			if callCounts[string(toolName)] <= 2 {
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
			{Name: mcp.ToolName("github_get_pull"), Description: "Get a pull request"},
			{Name: mcp.ToolName("github_get_pull_diff"), Description: "Get the raw diff"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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
			{Name: mcp.ToolName("linear_create_issue"), Description: "Create a Linear issue"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
			title, _ := args["title"].(string)
			return &mcp.ToolResult{Data: fmt.Sprintf(`{"identifier":"ENG-42","title":"%s","url":"https://linear.app/team/issue/ENG-42"}`, title)}, nil
		},
	}

	gh := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_create_pull"), Description: "Create a pull request"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

func TestScriptExecution_OutputByteCapEnforced(t *testing.T) {
	chunkData := `{"data":"` + strings.Repeat("x", 20*1024) + `"}`
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_chunk"), Description: "Returns a chunk of data"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: chunkData}, nil
		},
	}

	s := setupTestServer(mi)
	script := `var a = api.call("testint_chunk", {}); var b = api.call("testint_chunk", {}); var c = api.call("testint_chunk", {}); ({a: a, b: b, c: c});`
	data, _ := json.Marshal(map[string]any{"script": script})
	req := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "execute",
			Arguments: json.RawMessage(data),
		},
	}
	result, err := s.handleExecute(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError, "over-cap script output should return error")

	tc := result.Content[0].(*mcpsdk.TextContent)
	capKB := fmt.Sprintf("%dKB", defaultMaxResponseBytes/1024)
	assert.Contains(t, tc.Text, capKB)
	assert.Contains(t, tc.Text, "Script output exceeded")
}

func TestScriptExecution_OutputByteCapSkippedOnError(t *testing.T) {
	s := setupTestServer()
	data, _ := json.Marshal(map[string]any{
		"script": `api.call("nonexistent_tool", {});`,
	})
	req := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "execute",
			Arguments: json.RawMessage(data),
		},
	}
	result, err := s.handleExecute(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.NotContains(t, tc.Text, "Script output exceeded", "error results should skip byte cap")
}

func TestScriptExecution_FieldProjection(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `[{"id":1,"name":"alpha","secret":"hidden","meta":{"tag":"v1"}},{"id":2,"name":"beta","secret":"also hidden","meta":{"tag":"v2"}}]`}, nil
		},
	}

	s := setupTestServer(mi)
	result, err := s.scriptEngine.Run(context.Background(), `
		var items = api.call("testint_list_items", {}, {fields: ["id", "name"]});
		items;
	`)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var parsed []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	require.Len(t, parsed, 2)
	assert.Equal(t, float64(1), parsed[0]["id"])
	assert.Equal(t, "alpha", parsed[0]["name"])
	assert.Nil(t, parsed[0]["secret"], "secret should be projected out")
	assert.Nil(t, parsed[0]["meta"], "meta should be projected out")
}

func TestSearch_ScriptHint_MultipleIntegrations(t *testing.T) {
	alpha := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("github_list_issues"), Description: "List issues"}},
	}
	beta := &mockIntegration{
		name:    "linear",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("linear_list_issues"), Description: "List issues"}},
	}
	// Background tools give IDF contrast so "list" and "issues" have nonzero weight.
	bg := &mockIntegration{
		name:    "slack",
		healthy: true,
		tools:   diverseMockTools(),
	}
	s := setupTestServer(alpha, beta, bg)

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
			{Name: mcp.ToolName("github_get_pull"), Description: "Get a pull request"},
			{Name: mcp.ToolName("github_get_pull_diff"), Description: "Get diff"},
		},
	}
	bg := &mockIntegration{
		name:    "slack",
		healthy: true,
		tools:   diverseMockTools(),
	}
	s := setupTestServer(mi, bg)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"query": "pull"}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)
	assert.Contains(t, resp.ScriptHint, "multiple tool calls")
}

func TestExecuteTool_BreakerCountsPerCallNotPerAttempt(t *testing.T) {
	calls := 0
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_counter"), Description: "always 503"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			calls++
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 3
	s.breakerCooldown = time.Minute

	// Each executeTool call exhausts all 3 retry attempts but should record only 1 failure.
	// With threshold=3, need exactly 3 calls (not 1 call with 3 attempts) to trip.
	s.executeTool(context.Background(), "test_counter", map[string]any{})
	s.executeTool(context.Background(), "test_counter", map[string]any{})

	// After 2 calls (2 failures), breaker should still be closed (threshold=3).
	callsBefore := calls
	_, _, err := s.executeTool(context.Background(), "test_counter", map[string]any{})
	require.NoError(t, err)
	assert.True(t, calls > callsBefore, "breaker should still allow calls after 2 failures with threshold=3")

	// After 3rd call (3 failures), breaker should be open.
	_, result, err := s.executeTool(context.Background(), "test_counter", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "circuit breaker open")
}

func TestExecuteTool_BreakerErrorIncludesCooldownDuration(t *testing.T) {
	s := setupTestServer(&mockIntegration{
		name: "test",
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("test_cooldown"), Description: "always 503"},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return nil, &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("down")}
		},
		healthy: true,
	})
	s.retryBackoff = 0
	s.breakerThreshold = 1
	s.breakerCooldown = 30 * time.Second

	// Trip the breaker with 1 call.
	s.executeTool(context.Background(), "test_cooldown", map[string]any{})

	// Next call should hit the breaker with a helpful message.
	_, result, err := s.executeTool(context.Background(), "test_cooldown", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "30s", "breaker error should include cooldown duration")
	assert.Contains(t, result.Data, "Other integrations still work", "breaker error should hint at alternatives")
}

func TestSearch_IntegrationFilter(t *testing.T) {
	gh := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_list_pulls"), Description: "List pull requests"},
		},
	}
	slack := &mockIntegration{
		name:    "slack",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("slack_send_message"), Description: "Send a message"},
			{Name: mcp.ToolName("slack_list_conversations"), Description: "List conversations"},
		},
	}
	s := setupTestServer(gh, slack)

	t.Run("filter returns only matching integration", func(t *testing.T) {
		result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
			"integration": "github",
		}))
		require.NoError(t, err)
		resp := parseSearchResponse(t, result)
		assert.Equal(t, 2, resp.Total)
		for _, integration := range searchToolIntegrations(t, resp) {
			assert.Equal(t, "github", integration)
		}
	})

	t.Run("filter combined with query", func(t *testing.T) {
		result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
			"integration": "slack",
			"query":       "send",
		}))
		require.NoError(t, err)
		resp := parseSearchResponse(t, result)
		assert.Equal(t, 1, resp.Total)
		names := searchToolNames(t, resp)
		require.Len(t, names, 1)
		assert.Equal(t, "slack_send_message", names[0])
	})

	t.Run("empty filter returns all", func(t *testing.T) {
		result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
		require.NoError(t, err)
		resp := parseSearchResponse(t, result)
		assert.Equal(t, 4, resp.Total)
	})
}

func TestSearch_SynonymMatching(t *testing.T) {
	slack := &mockIntegration{
		name:    "slack",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("slack_send_message"), Description: "Send (post) a message to a channel or DM"},
		},
	}
	bg := &mockIntegration{
		name:    "testbg",
		healthy: true,
		tools:   diverseMockTools(),
	}
	s := setupTestServer(slack, bg)

	// "post message" should match because "post" is now in the description.
	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"query": "slack post message",
	}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)
	assert.Equal(t, 1, resp.Total, "search for 'slack post message' should find slack_send_message")
	names := searchToolNames(t, resp)
	require.Len(t, names, 1)
	assert.Equal(t, "slack_send_message", names[0])
}

func TestFindTool_ReturnsToolDefinition(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        mcp.ToolName("testint_get_item"),
				Description: "Get an item",
				Parameters:  map[string]string{"id": "Item ID"},
				Required:    []string{"id"},
			},
			{
				Name:        mcp.ToolName("testint_list_items"),
				Description: "List items",
				Parameters:  map[string]string{"query": "Search query"},
			},
		},
	}
	s := setupTestServer(mi)

	t.Run("returns matching tool definition", func(t *testing.T) {
		integration, toolDef, err := s.findTool("testint_get_item")
		require.NoError(t, err)
		assert.Equal(t, "testint", integration.Name())
		assert.Equal(t, mcp.ToolName("testint_get_item"), toolDef.Name)
		assert.Equal(t, []string{"id"}, toolDef.Required)
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		_, _, err := s.findTool("nonexistent_tool")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestValidateArgs(t *testing.T) {
	tool := mcp.ToolDefinition{
		Name:       mcp.ToolName("github_get_issue"),
		Parameters: map[string]string{"owner": "Repo owner", "repo": "Repo name", "number": "Issue number"},
		Required:   []string{"owner", "repo", "number"},
	}

	tests := []struct {
		name    string
		tool    mcp.ToolDefinition
		args    map[string]any
		extras  []string
		wantErr string
	}{
		{
			name: "all required present, no unknowns",
			tool: tool,
			args: map[string]any{"owner": "foo", "repo": "bar", "number": 42},
		},
		{
			name:    "missing required param",
			tool:    tool,
			args:    map[string]any{"owner": "foo", "repo": "bar"},
			wantErr: `missing required parameter "number"`,
		},
		{
			name:    "missing multiple required params",
			tool:    tool,
			args:    map[string]any{"owner": "foo"},
			wantErr: "missing required parameter",
		},
		{
			name:    "unknown param",
			tool:    tool,
			args:    map[string]any{"owner": "foo", "repo": "bar", "number": 42, "bogus": "val"},
			wantErr: `unknown parameter "bogus"`,
		},
		{
			name:    "unknown param similar to valid — suggests correction",
			tool:    tool,
			args:    map[string]any{"owner": "foo", "repo": "bar", "number": 42, "issue_number": 99},
			wantErr: `did you mean "number"`,
		},
		{
			name: "optional param present",
			tool: mcp.ToolDefinition{
				Name:       mcp.ToolName("testint_list"),
				Parameters: map[string]string{"query": "Search", "page": "Page number"},
				Required:   []string{"query"},
			},
			args: map[string]any{"query": "test", "page": 2},
		},
		{
			name: "empty args with no required",
			tool: mcp.ToolDefinition{
				Name:       mcp.ToolName("testint_list"),
				Parameters: map[string]string{"query": "Search"},
			},
			args: map[string]any{},
		},
		{
			name: "nil args with no required",
			tool: mcp.ToolDefinition{
				Name:       mcp.ToolName("testint_list"),
				Parameters: map[string]string{"query": "Search"},
			},
			args: nil,
		},
		{
			name:    "nil args with required params",
			tool:    tool,
			args:    nil,
			wantErr: `missing required parameter "owner"`,
		},
		{
			name:   "allowedExtras tolerated alongside declared params",
			tool:   tool,
			args:   map[string]any{"owner": "foo", "repo": "bar", "number": 42, "view": "full", "format": "markdown"},
			extras: []string{"view", "format"},
		},
		{
			name:    "extras not in allowedExtras still rejected",
			tool:    tool,
			args:    map[string]any{"owner": "foo", "repo": "bar", "number": 42, "view": "full", "bogus": "x"},
			extras:  []string{"view", "format"},
			wantErr: `unknown parameter "bogus"`,
		},
		{
			name:    "empty allowedExtras same as nil",
			tool:    tool,
			args:    map[string]any{"owner": "foo", "repo": "bar", "number": 42, "view": "full"},
			extras:  []string{},
			wantErr: `unknown parameter "view"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgs(tt.tool, tt.args, tt.extras)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestExecuteTool_ValidationRejectsMissingRequired(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        mcp.ToolName("testint_get_item"),
				Description: "Get an item",
				Parameters:  map[string]string{"id": "Item ID", "format": "Output format"},
				Required:    []string{"id"},
			},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			t.Fatal("handler should not be called when validation fails")
			return nil, nil
		},
	}
	s := setupTestServer(mi)

	integration, result, err := s.executeTool(context.Background(), "testint_get_item", map[string]any{"format": "json"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, `missing required parameter "id"`)
	assert.Nil(t, integration, "executeTool should return nil integration on validation failure")
}

func TestExecuteTool_ValidationRejectsUnknownParam(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{
				Name:        mcp.ToolName("testint_get_item"),
				Description: "Get an item",
				Parameters:  map[string]string{"id": "Item ID"},
				Required:    []string{"id"},
			},
		},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			t.Fatal("handler should not be called when validation fails")
			return nil, nil
		},
	}
	s := setupTestServer(mi)

	_, result, err := s.executeTool(context.Background(), "testint_get_item", map[string]any{"id": "123", "item_id": "456"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, `unknown parameter "item_id"`)
}

func TestSearch_ScriptHint_SingleResult(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("github_get_pull"), Description: "Get a pull request"}},
	}
	s := setupTestServer(mi)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{"query": "get pull"}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)
	assert.Empty(t, resp.ScriptHint)
}

func TestSearch_RecordsCatalogAvoidance(t *testing.T) {
	alpha := &mockIntegration{name: "alpha", healthy: true, tools: makeManyTools("alpha", 10)}
	beta := &mockIntegration{name: "beta", healthy: true, tools: makeManyTools("beta", 8)}
	s := setupTestServer(alpha, beta)
	s.services.Metrics = mcp.NewMetrics()
	// Re-build the index so catalogBytes is computed under the new metrics.
	s.buildSearchIndex()
	require.Positive(t, s.catalogBytes, "test setup: catalog must have nonzero baseline")

	_, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"query": "alpha_tool_0",
		"limit": 3,
	}))
	require.NoError(t, err)

	snap := s.services.Metrics.Snapshot()
	assert.Equal(t, int64(1), snap.CatalogAvoidedCount)
	assert.Positive(t, snap.CatalogBytesAvoided, "narrow search must yield positive avoidance")
	assert.Less(t, snap.CatalogBytesAvoided, s.catalogBytes,
		"avoidance must be less than the full catalog (since we shipped some bytes back)")
}

func TestSearch_ResponseColumnarized(t *testing.T) {
	tests := []struct {
		name         string
		toolCount    int
		wantColumnar bool
		wantConstant string // expected constant integration value, empty = no constants check
	}{
		{
			name:         "columnarizes tools array when 8+ results from one integration",
			toolCount:    10,
			wantColumnar: true,
			wantConstant: "testint",
		},
		{
			name:         "keeps per-record format when fewer than 8 results",
			toolCount:    5,
			wantColumnar: false,
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

			result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
			require.NoError(t, err)
			require.False(t, result.IsError)

			tc, ok := result.Content[0].(*mcpsdk.TextContent)
			require.True(t, ok)

			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(tc.Text), &raw))

			var tools any
			require.NoError(t, json.Unmarshal(raw["tools"], &tools))

			if tt.wantColumnar {
				// tools should be {"columns":[...],"rows":[...],...}
				toolsMap, ok := tools.(map[string]any)
				require.True(t, ok, "expected columnar object, got %T", tools)
				assert.Contains(t, toolsMap, "columns")
				assert.Contains(t, toolsMap, "rows")

				if tt.wantConstant != "" {
					constants, ok := toolsMap["constants"].(map[string]any)
					require.True(t, ok, "expected constants map")
					assert.Equal(t, tt.wantConstant, constants["integration"])
				}
			} else {
				// tools should be a plain array
				_, ok := tools.([]any)
				assert.True(t, ok, "expected array, got %T", tools)
			}
		})
	}
}

func TestSearch_SharedParametersExtracted(t *testing.T) {
	tests := []struct {
		name             string
		tools            []mcp.ToolDefinition
		wantSharedParams map[string]string // params expected in shared_parameters
		wantKeptPerTool  []string          // param names that should stay per-tool
	}{
		{
			name: "extracts params with identical description across 3+ tools",
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("t_list_issues"), Description: "List issues", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "open or closed"}},
				{Name: mcp.ToolName("t_get_issue"), Description: "Get issue", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "issue_number": "Issue number"}},
				{Name: mcp.ToolName("t_list_pulls"), Description: "List pulls", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "base": "Base branch"}},
				{Name: mcp.ToolName("t_get_pull"), Description: "Get pull", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull number"}},
				{Name: mcp.ToolName("t_list_commits"), Description: "List commits", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Branch or SHA"}},
				{Name: mcp.ToolName("t_list_branches"), Description: "List branches", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"}},
				{Name: mcp.ToolName("t_list_releases"), Description: "List releases", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"}},
				{Name: mcp.ToolName("t_list_tags"), Description: "List tags", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"}},
			},
			wantSharedParams: map[string]string{
				"owner": "Repository owner",
				"repo":  "Repository name",
			},
			wantKeptPerTool: []string{"state", "issue_number", "base", "pull_number", "sha"},
		},
		{
			name: "keeps params with same name but different description per-tool",
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("t_one"), Description: "One", Parameters: map[string]string{"id": "The issue ID"}},
				{Name: mcp.ToolName("t_two"), Description: "Two", Parameters: map[string]string{"id": "The pull request ID"}},
				{Name: mcp.ToolName("t_three"), Description: "Three", Parameters: map[string]string{"id": "The commit SHA"}},
				{Name: mcp.ToolName("t_four"), Description: "Four", Parameters: map[string]string{"id": "The release ID"}},
				{Name: mcp.ToolName("t_five"), Description: "Five", Parameters: map[string]string{"id": "The tag name"}},
				{Name: mcp.ToolName("t_six"), Description: "Six", Parameters: map[string]string{"id": "The branch name"}},
				{Name: mcp.ToolName("t_seven"), Description: "Seven", Parameters: map[string]string{"id": "The deploy ID"}},
				{Name: mcp.ToolName("t_eight"), Description: "Eight", Parameters: map[string]string{"id": "The run ID"}},
			},
			wantSharedParams: nil, // all different descriptions
			wantKeptPerTool:  []string{"id"},
		},
		{
			name: "preserves tool-specific value hints even when name matches",
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("t_a"), Description: "A", Parameters: map[string]string{"event": "APPROVE, REQUEST_CHANGES, COMMENT", "owner": "Repo owner"}},
				{Name: mcp.ToolName("t_b"), Description: "B", Parameters: map[string]string{"event": "push, pull_request", "owner": "Repo owner"}},
				{Name: mcp.ToolName("t_c"), Description: "C", Parameters: map[string]string{"event": "issues, created", "owner": "Repo owner"}},
				{Name: mcp.ToolName("t_d"), Description: "D", Parameters: map[string]string{"owner": "Repo owner"}},
				{Name: mcp.ToolName("t_e"), Description: "E", Parameters: map[string]string{"event": "deployment", "owner": "Repo owner"}},
				{Name: mcp.ToolName("t_f"), Description: "F", Parameters: map[string]string{"event": "release", "owner": "Repo owner"}},
				{Name: mcp.ToolName("t_g"), Description: "G", Parameters: map[string]string{"owner": "Repo owner"}},
				{Name: mcp.ToolName("t_h"), Description: "H", Parameters: map[string]string{"owner": "Repo owner"}},
			},
			wantSharedParams: map[string]string{"owner": "Repo owner"},
			wantKeptPerTool:  []string{"event"}, // different descriptions → stays per-tool
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mockIntegration{
				name:    "testint",
				healthy: true,
				tools:   tt.tools,
			}
			s := setupTestServer(mi)

			result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
			require.NoError(t, err)
			require.False(t, result.IsError)

			tc, ok := result.Content[0].(*mcpsdk.TextContent)
			require.True(t, ok)

			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(tc.Text), &raw))

			if tt.wantSharedParams != nil {
				var shared map[string]string
				require.Contains(t, raw, "shared_parameters", "response should have shared_parameters")
				require.NoError(t, json.Unmarshal(raw["shared_parameters"], &shared))
				assert.Equal(t, tt.wantSharedParams, shared)

				// Verify shared params are removed from per-tool parameters.
				allParams := extractColumnarParams(t, raw["tools"])
				for _, params := range allParams {
					for sharedKey := range tt.wantSharedParams {
						assert.NotContains(t, params, sharedKey, "shared param %q should be removed from per-tool params", sharedKey)
					}
				}
			} else {
				_, hasShared := raw["shared_parameters"]
				assert.False(t, hasShared, "should not have shared_parameters when no common params")
			}

			// Verify kept-per-tool params are still in tool parameters.
			if len(tt.wantKeptPerTool) > 0 {
				allParams := extractColumnarParams(t, raw["tools"])
				for _, keptParam := range tt.wantKeptPerTool {
					found := false
					for _, params := range allParams {
						if _, ok := params[keptParam]; ok {
							found = true
							break
						}
					}
					assert.True(t, found, "param %q should be kept per-tool in at least one tool", keptParam)
				}
			}
		})
	}
}

func TestSearch_SharedParametersDoNotMutateOriginalTools(t *testing.T) {
	mi := &mockIntegration{
		name:    "testint",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("t_list_issues"), Description: "List issues", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "open or closed"}},
			{Name: mcp.ToolName("t_get_issue"), Description: "Get issue", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "issue_number": "Issue number"}},
			{Name: mcp.ToolName("t_list_pulls"), Description: "List pulls", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "base": "Base branch"}},
			{Name: mcp.ToolName("t_get_pull"), Description: "Get pull", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull number"}},
			{Name: mcp.ToolName("t_list_commits"), Description: "List commits", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Branch or SHA"}},
			{Name: mcp.ToolName("t_list_branches"), Description: "List branches", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"}},
			{Name: mcp.ToolName("t_list_releases"), Description: "List releases", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"}},
			{Name: mcp.ToolName("t_list_tags"), Description: "List tags", Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"}},
		},
	}
	s := setupTestServer(mi)

	// First search triggers shared parameter extraction — must not corrupt originals.
	_, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
	require.NoError(t, err)

	// Verify the original ToolDefinition maps are untouched.
	for _, tool := range mi.tools {
		assert.Contains(t, tool.Parameters, "owner",
			"tool %q should still have 'owner' after search (was deleted from original)", tool.Name)
		assert.Contains(t, tool.Parameters, "repo",
			"tool %q should still have 'repo' after search (was deleted from original)", tool.Name)
	}

	// Second search should produce identical shared_parameters as the first.
	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
	require.NoError(t, err)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &raw))

	var shared map[string]string
	require.Contains(t, raw, "shared_parameters")
	require.NoError(t, json.Unmarshal(raw["shared_parameters"], &shared))
	assert.Equal(t, map[string]string{
		"owner": "Repository owner",
		"repo":  "Repository name",
	}, shared, "second search should produce the same shared_parameters")
}

// --- ABAC tool glob filtering tests ---

func setupTestServerWithGlobs(integrations map[string]*mockIntegration, globs map[string][]string) *Server {
	reg := newMockRegistry()
	cfgIntegrations := make(map[string]*mcp.IntegrationConfig)

	for name, i := range integrations {
		reg.Register(i)
		cfgIntegrations[name] = &mcp.IntegrationConfig{
			Enabled:     true,
			Credentials: mcp.Credentials{"token": "test"},
			ToolGlobs:   globs[name],
		}
	}

	services := &mcp.Services{
		Config:   newMockConfigService(cfgIntegrations),
		Registry: reg,
	}
	return New(services)
}

func TestABAC_SearchFiltersToolsByGlob(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_get_issue"), Description: "Get an issue"},
			{Name: mcp.ToolName("github_list_pulls"), Description: "List pulls"},
			{Name: mcp.ToolName("github_get_pull"), Description: "Get a pull"},
		},
	}

	tests := []struct {
		name      string
		globs     []string
		wantTools []string
	}{
		{
			name:      "empty globs allows all tools",
			globs:     nil,
			wantTools: []string{"github_get_issue", "github_get_pull", "github_list_issues", "github_list_pulls"},
		},
		{
			name:      "glob restricts to matching tools",
			globs:     []string{"github_get_*"},
			wantTools: []string{"github_get_issue", "github_get_pull"},
		},
		{
			name:      "multiple globs ORd",
			globs:     []string{"github_list_issues", "github_get_pull"},
			wantTools: []string{"github_get_pull", "github_list_issues"},
		},
		{
			name:      "no matching globs hides all tools",
			globs:     []string{"datadog_*"},
			wantTools: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupTestServerWithGlobs(
				map[string]*mockIntegration{"github": mi},
				map[string][]string{"github": tt.globs},
			)

			result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
			require.NoError(t, err)

			resp := parseSearchResponse(t, result)
			names := searchToolNames(t, resp)
			if tt.wantTools == nil {
				assert.Empty(t, names)
			} else {
				assert.Equal(t, tt.wantTools, names)
			}
		})
	}
}

func TestABAC_ScoredSearchFiltersToolsByGlob(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues for a repository"},
			{Name: mcp.ToolName("github_delete_repo"), Description: "Delete a repository"},
		},
	}
	bg := &mockIntegration{
		name:    "testbg",
		healthy: true,
		tools:   diverseMockTools(),
	}

	reg := newMockRegistry()
	reg.Register(mi)
	reg.Register(bg)

	cfgIntegrations := map[string]*mcp.IntegrationConfig{
		"github": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}, ToolGlobs: []string{"github_list_*"}},
		"testbg": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
	}
	services := &mcp.Services{Config: newMockConfigService(cfgIntegrations), Registry: reg}
	s := New(services)

	// Scored search with a query should respect ABAC globs.
	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"query": "github",
	}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)
	names := searchToolNames(t, resp)

	assert.Contains(t, names, "github_list_issues", "allowed tool should appear in scored search")
	assert.NotContains(t, names, "github_delete_repo", "blocked tool should NOT appear in scored search")
}

func TestABAC_ExecuteRejectsBlockedTool(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_delete_repo"), Description: "Delete a repository"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"ok":true}`}, nil
		},
	}

	s := setupTestServerWithGlobs(
		map[string]*mockIntegration{"github": mi},
		map[string][]string{"github": {"github_list_*"}},
	)

	t.Run("allowed tool executes", func(t *testing.T) {
		_, result, err := s.executeTool(context.Background(), "github_list_issues", map[string]any{})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("blocked tool returns not found", func(t *testing.T) {
		_, result, err := s.executeTool(context.Background(), "github_delete_repo", map[string]any{})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Data, "not found")
	})
}

func TestABAC_ScriptCannotCallBlockedTool(t *testing.T) {
	mi := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_delete_repo"), Description: "Delete a repository"},
		},
		execFn: func(_ context.Context, toolName mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `{"ok":true}`}, nil
		},
	}

	s := setupTestServerWithGlobs(
		map[string]*mockIntegration{"github": mi},
		map[string][]string{"github": {"github_list_*"}},
	)

	result, err := s.scriptEngine.Run(context.Background(), `api.call("github_delete_repo", {});`)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "not found")
}

func TestABAC_MultiIntegrationGlobIsolation(t *testing.T) {
	gh := &mockIntegration{
		name:    "github",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("github_list_issues"), Description: "List issues"},
			{Name: mcp.ToolName("github_get_pull"), Description: "Get a pull"},
		},
	}
	dd := &mockIntegration{
		name:    "datadog",
		healthy: true,
		tools: []mcp.ToolDefinition{
			{Name: mcp.ToolName("datadog_search_logs"), Description: "Search logs"},
			{Name: mcp.ToolName("datadog_get_metric"), Description: "Get metric"},
		},
	}

	s := setupTestServerWithGlobs(
		map[string]*mockIntegration{"github": gh, "datadog": dd},
		map[string][]string{
			"github":  {"github_list_*"},
			"datadog": nil,
		},
	)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{}))
	require.NoError(t, err)

	resp := parseSearchResponse(t, result)
	names := searchToolNames(t, resp)
	assert.Contains(t, names, "github_list_issues")
	assert.NotContains(t, names, "github_get_pull")
	assert.Contains(t, names, "datadog_search_logs")
	assert.Contains(t, names, "datadog_get_metric")
}

func TestSearch_DiscoverAll_IncludesUnconfiguredTools(t *testing.T) {
	// "github" is configured (enabled), "linear" is registered but NOT in config.
	configured := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("github_list_repos"), Description: "List repositories"}},
	}
	unconfigured := &mockIntegration{
		name:    "linear",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("linear_list_issues"), Description: "List issues"}},
	}
	bg := &mockIntegration{
		name:    "testbg",
		healthy: true,
		tools:   diverseMockTools(),
	}

	reg := newMockRegistry()
	reg.Register(configured)
	reg.Register(unconfigured)
	reg.Register(bg)

	// Only github and testbg are in the config; linear is not.
	cfgIntegrations := map[string]*mcp.IntegrationConfig{
		"github": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
		"testbg": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
	}

	services := &mcp.Services{
		Config:   newMockConfigService(cfgIntegrations),
		Registry: reg,
	}
	s := New(services, WithDiscoverAll(true))

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"query": "list",
	}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)

	// Should find tools from both configured AND unconfigured integrations.
	names := searchToolNames(t, resp)
	assert.Contains(t, names, "github_list_repos", "configured tool should appear")
	assert.Contains(t, names, "linear_list_issues", "unconfigured tool should appear with discoverAll")
}

func TestSearch_DiscoverAll_MarksUnconfiguredTools(t *testing.T) {
	configured := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("github_list_repos"), Description: "List repositories"}},
	}
	unconfigured := &mockIntegration{
		name:    "linear",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("linear_list_issues"), Description: "List issues"}},
	}
	bg := &mockIntegration{
		name:    "testbg",
		healthy: true,
		tools:   diverseMockTools(),
	}

	reg := newMockRegistry()
	reg.Register(configured)
	reg.Register(unconfigured)
	reg.Register(bg)

	cfgIntegrations := map[string]*mcp.IntegrationConfig{
		"github": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
		"testbg": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
	}

	services := &mcp.Services{
		Config:   newMockConfigService(cfgIntegrations),
		Registry: reg,
	}
	s := New(services, WithDiscoverAll(true))

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"query": "list issues",
	}))
	require.NoError(t, err)

	// Parse raw JSON to check for "configured" field on tools.
	tc := result.Content[0].(*mcpsdk.TextContent)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &raw))

	var tools []struct {
		Name       string `json:"name"`
		Configured *bool  `json:"configured"`
	}
	require.NoError(t, json.Unmarshal(raw["tools"], &tools))

	for _, tool := range tools {
		if tool.Name == "github_list_repos" {
			// Configured tools should either have configured=true or omit the field.
			if tool.Configured != nil {
				assert.True(t, *tool.Configured, "configured tool should have configured=true")
			}
		}
		if tool.Name == "linear_list_issues" {
			require.NotNil(t, tool.Configured, "unconfigured tool must have configured field")
			assert.False(t, *tool.Configured, "unconfigured tool should have configured=false")
		}
	}
}

func TestSearch_DiscoverAllOff_ExcludesUnconfiguredTools(t *testing.T) {
	configured := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("github_list_repos"), Description: "List repositories"}},
	}
	unconfigured := &mockIntegration{
		name:    "linear",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("linear_list_issues"), Description: "List issues"}},
	}
	bg := &mockIntegration{
		name:    "testbg",
		healthy: true,
		tools:   diverseMockTools(),
	}

	reg := newMockRegistry()
	reg.Register(configured)
	reg.Register(unconfigured)
	reg.Register(bg)

	cfgIntegrations := map[string]*mcp.IntegrationConfig{
		"github": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
		"testbg": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
	}

	services := &mcp.Services{
		Config:   newMockConfigService(cfgIntegrations),
		Registry: reg,
	}
	// discoverAll defaults to false — unconfigured tools should NOT appear.
	s := New(services)

	result, err := s.handleSearch(context.Background(), searchRequest(map[string]any{
		"query": "list",
	}))
	require.NoError(t, err)
	resp := parseSearchResponse(t, result)

	names := searchToolNames(t, resp)
	assert.Contains(t, names, "github_list_repos", "configured tool should appear")
	assert.NotContains(t, names, "linear_list_issues", "unconfigured tool should NOT appear without discoverAll")
}

func TestExecute_UnconfiguredTool_ReturnsConfigurationError(t *testing.T) {
	configured := &mockIntegration{
		name:    "github",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("github_list_repos"), Description: "List repositories"}},
		execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
			return &mcp.ToolResult{Data: `[{"name":"test"}]`}, nil
		},
	}
	unconfigured := &mockIntegration{
		name:    "linear",
		healthy: true,
		tools:   []mcp.ToolDefinition{{Name: mcp.ToolName("linear_list_issues"), Description: "List issues"}},
	}

	reg := newMockRegistry()
	reg.Register(configured)
	reg.Register(unconfigured)

	// Only github is configured; linear is registered but not in config.
	cfgIntegrations := map[string]*mcp.IntegrationConfig{
		"github": {Enabled: true, Credentials: mcp.Credentials{"token": "test"}},
	}

	services := &mcp.Services{
		Config:   newMockConfigService(cfgIntegrations),
		Registry: reg,
	}
	s := New(services, WithDiscoverAll(true))

	// Execute a configured tool — should succeed.
	result, err := s.handleExecute(context.Background(), executeRequest("github_list_repos", nil))
	require.NoError(t, err)
	assert.False(t, result.IsError, "configured tool should execute successfully")

	// Execute an unconfigured tool — should get a specific "not configured" error,
	// NOT the generic "tool not found" message.
	result, err = s.handleExecute(context.Background(), executeRequest("linear_list_issues", nil))
	require.NoError(t, err)
	assert.True(t, result.IsError, "unconfigured tool should return an error")

	tc := result.Content[0].(*mcpsdk.TextContent)
	assert.Contains(t, tc.Text, "not configured",
		"error should mention 'not configured', not 'not found'")
	assert.Contains(t, tc.Text, "linear",
		"error should name the unconfigured integration")
}

// --- ExecuteRendered (script api.callRendered) ---

func TestToolExecutor_ExecuteRendered_AppliesMarkdown(t *testing.T) {
	mi := &mockMarkdownIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_get_page"), Description: "Get page"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `{"title":"Hello","body":"World"}`}, nil
			},
		},
		renderFn: func(_ mcp.ToolName, _ []byte) (mcp.Markdown, bool) {
			return "# Hello\nWorld\n", true
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	te := &toolExecutor{server: s}
	result, err := te.ExecuteRendered(context.Background(), "testint_get_page", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "# Hello\nWorld\n", result.Data, "ExecuteRendered should return markdown")
}

func TestToolExecutor_ExecuteRendered_FallsBackToCompaction(t *testing.T) {
	// Integration without MarkdownIntegration — should still compact
	mi := &mockFieldCompactionIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_list_items"), Description: "List items"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `{"id":1,"name":"kept","secret":"dropped"}`}, nil
			},
		},
		specs: map[mcp.ToolName][]mcp.CompactField{
			"testint_list_items": mustParseSpecs([]string{"id", "name"}),
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	te := &toolExecutor{server: s}
	result, err := te.ExecuteRendered(context.Background(), "testint_list_items", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "kept")
	assert.NotContains(t, result.Data, "secret", "compaction should strip unspecified fields")
}

func TestToolExecutor_ExecuteRendered_SkipsProcessResultOnError(t *testing.T) {
	mi := &mockMarkdownIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_get_page"), Description: "Get page"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: "page not found", IsError: true}, nil
			},
		},
		renderFn: func(_ mcp.ToolName, _ []byte) (mcp.Markdown, bool) {
			return "# Should not render", true
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	te := &toolExecutor{server: s}
	result, err := te.ExecuteRendered(context.Background(), "testint_get_page", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "page not found", result.Data, "error results should not be rendered")
}

func TestToolExecutor_ExecuteRendered_BlocksMetaTools(t *testing.T) {
	s := setupTestServer()
	te := &toolExecutor{server: s}

	result, err := te.ExecuteRendered(context.Background(), "search", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "meta-tool")

	result, err = te.ExecuteRendered(context.Background(), "execute", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "meta-tool")
}

func TestScript_CallRendered_EndToEnd(t *testing.T) {
	mi := &mockMarkdownIntegration{
		mockIntegration: mockIntegration{
			name:    "testint",
			healthy: true,
			tools: []mcp.ToolDefinition{
				{Name: mcp.ToolName("testint_get_doc"), Description: "Get doc"},
			},
			execFn: func(_ context.Context, _ mcp.ToolName, _ map[string]any) (*mcp.ToolResult, error) {
				return &mcp.ToolResult{Data: `{"raw":"json","noise":"dropped"}`}, nil
			},
		},
		renderFn: func(_ mcp.ToolName, _ []byte) (mcp.Markdown, bool) {
			return "# Rendered Document\nClean content.\n", true
		},
	}

	s := setupTestServer(&mi.mockIntegration)
	s.services.Registry.(*mockRegistry).integrations["testint"] = mi

	script := `({text: api.callRendered('testint_get_doc', {id: '123'})})`
	scriptReq := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "execute",
			Arguments: mustMarshal(map[string]any{"script": script}),
		},
	}
	result, err := s.handleExecute(context.Background(), scriptReq)
	require.NoError(t, err)
	require.False(t, result.IsError)

	tc := result.Content[0].(*mcpsdk.TextContent)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &parsed))
	assert.Equal(t, "# Rendered Document\nClean content.\n", parsed["text"])
}

func mustParseSpecs(specs []string) []mcp.CompactField {
	fields, err := mcp.ParseCompactSpecs(specs)
	if err != nil {
		panic(err)
	}
	return fields
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
